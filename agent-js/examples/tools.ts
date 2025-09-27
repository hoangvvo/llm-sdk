import { Agent, getContentText, tool } from "@hoangvvo/llm-agent";
import { typeboxTool } from "@hoangvvo/llm-agent/typebox";
import { zodTool } from "@hoangvvo/llm-agent/zod";
import { Type } from "typebox";
import z from "zod";
import { getModel } from "./get-model.ts";

/**
 * Shared context used by every tool invocation. Tools mutate this object so the agent can
 * keep track of the case without needing per-turn toolkits.
 */
interface LostAndFoundContext {
  manifestId: string;
  archivistOnDuty: string;
  // Items waiting for confirmation before we issue a receipt.
  intakeLedger: Map<
    string,
    { description: string; priority: "standard" | "rush" }
  >;
  // Items that must be escalated for contraband review.
  flaggedContraband: Set<string>;
  // Notes that should appear on the final receipt.
  receiptNotes: string[];
}

function createContext(): LostAndFoundContext {
  return {
    manifestId: "aurora-shift",
    archivistOnDuty: "Quill",
    intakeLedger: new Map(),
    flaggedContraband: new Set(),
    receiptNotes: [],
  };
}

/**
 * Basic `tool` helper which provides convenient type completion for args and context.
 */
const intakeItemTool = tool<
  LostAndFoundContext,
  {
    item_id: string;
    description: string;
    priority?: "standard" | "rush";
  }
>({
  name: "intake_item",
  description:
    "Register an item reported by the traveller. Records a note for later receipt generation.",
  parameters: {
    type: "object",
    properties: {
      item_id: {
        type: "string",
        description: "Identifier used on the manifest ledger.",
      },
      description: {
        type: "string",
        description: "What the traveller says it looks like.",
      },
      priority: {
        type: "string",
        description: "Optional rush flag. Defaults to standard intake.",
        enum: ["standard", "rush"],
      },
    },
    required: ["item_id", "description", "priority"],
    additionalProperties: false,
  },
  execute(args, ctx) {
    const normalizedId = args.item_id.trim().toLowerCase();
    if (ctx.intakeLedger.has(normalizedId)) {
      return {
        content: [
          {
            type: "text",
            text: `Item ${args.item_id} is already on the ledger; confirm the manifest number before adding duplicates.`,
          },
        ],
        is_error: true,
      };
    }

    const priority =
      args.priority?.trim() === "" ? "standard" : (args.priority ?? "standard");
    ctx.intakeLedger.set(normalizedId, {
      description: args.description,
      priority,
    });
    ctx.receiptNotes.push(
      `${args.item_id}: ${args.description}${priority === "rush" ? " (rush intake)" : ""}`,
    );

    return {
      content: [
        {
          type: "text",
          text: `Logged ${args.description} as ${args.item_id}. Intake queue now holds ${ctx.intakeLedger.size} item(s).`,
        },
      ],
      is_error: false,
    };
  },
});

/**
 * zodTool helper to demonstrate schema definitions using Zod.
 * Requires:
 *
 * npm install zod zod-to-json-schema
 */
const flagContrabandTool = zodTool({
  name: "flag_contraband",
  description:
    "Escalate a manifest item for contraband review. Prevents it from appearing on the standard receipt.",
  parameters: z.object({
    item_id: z.string().describe("Item identifier within the manifest."),
    reason: z
      .string()
      .min(3)
      .describe("Why the item requires additional screening."),
  }),
  execute(args, ctx: LostAndFoundContext) {
    const key = args.item_id.trim().toLowerCase();
    if (!ctx.intakeLedger.has(key)) {
      return {
        content: [
          {
            type: "text",
            text: `Cannot flag ${args.item_id}; it has not been logged yet. Intake the item first.`,
          },
        ],
        is_error: true,
      };
    }

    ctx.flaggedContraband.add(key);
    ctx.receiptNotes.push(`⚠️ ${args.item_id} held for review: ${args.reason}`);

    return {
      content: [
        {
          type: "text",
          text: `${args.item_id} marked for contraband inspection. Inform security before release.`,
        },
      ],
      is_error: false,
    };
  },
});

/**
 * Another standard tool using TypeBox that demonstrates returning a final summary and clearing state.
 * Requires:
 *
 * npm install typebox
 */
const issueReceiptTool = typeboxTool({
  name: "issue_receipt",
  description:
    "Publish a receipt for the traveller: lists cleared items, highlights contraband reminders, and clears the ledger.",
  parameters: Type.Object(
    {
      traveller: Type.String({ description: "Name to print on the receipt." }),
    },
    { additionalProperties: false },
  ),
  execute(args, ctx: LostAndFoundContext) {
    if (ctx.intakeLedger.size === 0) {
      return {
        content: [
          {
            type: "text",
            text: `No items pending on manifest ${ctx.manifestId}. Intake something before issuing a receipt.`,
          },
        ],
        is_error: true,
      };
    }

    const cleared = Array.from(ctx.intakeLedger.entries())
      .filter(([id]) => !ctx.flaggedContraband.has(id))
      .map(([id, entry]) => `${id} (${entry.description})`);

    const contraband = ctx.flaggedContraband.size;
    const summaryLines: string[] = [
      `Receipt for ${args.traveller} on manifest ${ctx.manifestId}:`,
      cleared.length > 0
        ? `Cleared items: ${cleared.join(", ")}`
        : "No items cleared; everything is held for review.",
    ];
    if (ctx.receiptNotes.length > 0) {
      summaryLines.push("Notes:");
      summaryLines.push(...ctx.receiptNotes);
    }
    summaryLines.push(
      contraband > 0
        ? `${contraband} item(s) require contraband follow-up.`
        : "No contraband flags recorded.",
    );

    ctx.intakeLedger.clear();
    ctx.flaggedContraband.clear();
    ctx.receiptNotes.length = 0;

    return {
      content: [
        {
          type: "text",
          text: summaryLines.join("\n"),
        },
      ],
      is_error: false,
    };
  },
});

const model = getModel("openai", "gpt-4o");

const lostAndFoundAgent = new Agent<LostAndFoundContext>({
  name: "WaypointClerk",
  instructions: [
    "You are the archivist completing intake for Waypoint Seven's Interdimensional Lost & Found desk.",
    "When travellers report belongings, call the available tools to mutate the manifest and then summarise your actions.",
    "If a tool reports an error, acknowledge the issue and guide the traveller appropriately.",
  ],
  model,
  tools: [intakeItemTool, flagContrabandTool, issueReceiptTool],
});

// Successful run: exercise multiple tools and show evolving context state.
const successContext = createContext();
const successResponse = await lostAndFoundAgent.run({
  context: successContext,
  input: [
    {
      type: "message",
      role: "user",
      content: [
        {
          type: "text",
          text: `Log the Chrono Locket as rush, mark the "Folded star chart" for contraband, then issue a receipt for Captain Lyra Moreno.`,
        },
      ],
    },
  ],
});

console.log("\n=== SUCCESS RUN ===");
console.dir(successResponse, { depth: null });
console.log(getContentText(successResponse));

// Failure case: demonstrate tool error handling in the same scenario.
const failureContext = createContext();
const failureResponse = await lostAndFoundAgent.run({
  context: failureContext,
  input: [
    {
      type: "message",
      role: "user",
      content: [
        {
          type: "text",
          text: `Issue a receipt immediately without logging anything.`,
        },
      ],
    },
  ],
});

console.log("\n=== FAILURE RUN ===");
console.dir(failureResponse, { depth: null });
console.log(getContentText(failureResponse));
