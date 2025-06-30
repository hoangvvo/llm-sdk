import {
  Agent,
  AgentToolExecutionError,
  type AgentItem,
  type AgentStreamResponseEvent,
} from "@hoangvvo/llm-agent";
import { zodTool } from "@hoangvvo/llm-agent/zod";
import { stdin as input, stdout as output } from "node:process";
import { createInterface } from "node:readline/promises";
import { z } from "zod";
import { getContentText } from "../src/utils.ts";
import { getModel } from "./get-model.ts";

// Human-in-the-loop outline with agent primitives:
// 1. Seed the run with a user `AgentItem` and call `Agent#runStream` so we capture
//    every emitted `AgentStreamEvent` (model messages, tool results, etc.).
// 2. When the tool throws our user-land `RequireApprovalError`, collect the human
//    decision and persist it on the shared RunSession context.
// 3. Repeat step (1) with the accumulated items and mutated context until the tool
//    succeeds or returns an error result that reflects the denial.

class RequireApprovalError extends Error {
  readonly artifact: string;

  constructor(message: string, artifact: string) {
    super(message);
    this.name = "RequireApprovalError";
    this.artifact = artifact;
  }
}

type ApprovalStatus = "approved" | "denied";

interface VaultContext {
  approvals: Map<string, ApprovalStatus>;
}

const vaultContext: VaultContext = {
  approvals: new Map(),
};

// Single AgentTool that inspects the context map and interrupts the run.
// Thrown errors become AgentToolExecutionError.
const unlockArtifact = zodTool({
  name: "unlock_artifact",
  description:
    "Unlock an artifact for release once a human supervisor has recorded their approval.",
  parameters: z.object({
    artifact: z.string().min(1).describe("Name of the artifact to release."),
  }),
  async execute({ artifact }, ctx: VaultContext) {
    const artifactKey = artifact.trim().toLowerCase();
    const status = ctx.approvals.get(artifactKey);

    if (!status) {
      throw new RequireApprovalError(
        `Release of ${artifact} requires human approval before it can proceed.`,
        artifact,
      );
    }

    if (status === "denied") {
      return {
        content: [
          {
            type: "text",
            text: `Release of ${artifact} remains blocked until a supervisor approves it.`,
          },
        ],
        is_error: true,
      };
    }

    return {
      content: [
        {
          type: "text",
          text: `${artifact} unlocked. Proceed with standard vault handling protocols.`,
        },
      ],
      is_error: false,
    };
  },
});

const sentinel = new Agent<VaultContext>({
  name: "VaultSentinel",
  model: getModel("openai", "gpt-4o"),
  instructions: [
    "You supervise the Eon Vault, safeguarding experimental expedition technology.",
  ],
  tools: [unlockArtifact],
});

const initialText =
  "We have an emergency launch window in four hours. Please unlock the Starlight Compass for the Horizon survey team.";

const allItems: AgentItem[] = [
  {
    type: "message",
    role: "user",
    content: [
      {
        type: "text",
        text: initialText,
      },
    ],
  },
];

console.log(`[user] ${initialText}`);

async function run(context: VaultContext): Promise<AgentStreamResponseEvent> {
  const stream = sentinel.runStream({
    context,
    input: [...allItems],
  });

  for await (const event of stream) {
    if (event.event === "partial") {
      continue;
    }

    if (event.event === "item") {
      // Persist generated items so later iterations operate on the full history.
      allItems.push(event.item);
      logItem(event.item);
    }

    if (event.event === "response") {
      return event;
    }
  }

  throw new Error("Agent stream completed without emitting a response.");
}

function logItem(item: AgentItem) {
  switch (item.type) {
    case "message": {
      const text = getContentText(item);
      if (text !== "") {
        console.log(`\n[${item.role}] ${text}`);
      }
      break;
    }
    case "model": {
      const text = getContentText(item);
      if (text !== "") {
        console.log(`\n[assistant]\n${text}`);
      }
      break;
    }
    case "tool": {
      const toolOutput = getContentText({ content: item.output });
      console.log(
        `\n[tool:${item.tool_name}]
  input=${JSON.stringify(item.input)}
  output=${JSON.stringify(toolOutput)}`,
      );
      break;
    }
  }
}

async function promptForApproval(artifact: string): Promise<ApprovalStatus> {
  const rl = createInterface({ input, output });
  try {
    const decision = (
      await rl.question(`Grant approval to unlock ${artifact}? (y/N) `)
    )
      .trim()
      .toLowerCase();

    if (/^y(es)?$/.test(decision)) {
      return "approved";
    }

    if (/^n(o)?$/.test(decision) || decision === "") {
      return "denied";
    }

    console.log("Unrecognized response, treating as denied.");
    return "denied";
  } finally {
    rl.close();
  }
}

for (;;) {
  try {
    const response = await run(vaultContext);

    console.log("\nCompleted run.");
    console.dir(response.content, { depth: null });
    break;
  } catch (err) {
    if (
      err instanceof AgentToolExecutionError &&
      err.cause instanceof RequireApprovalError
    ) {
      console.log(`\n[agent halted] err = ${err.cause.message}`);

      const haltedArtifact = err.cause.artifact;
      const normalized = haltedArtifact.trim().toLowerCase();
      // Store the decision so the tool sees the new approval status on retry.
      const decision = await promptForApproval(haltedArtifact);

      vaultContext.approvals.set(normalized, decision);

      continue;
    }

    throw err;
  }
}
