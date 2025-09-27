import { Agent, type AgentItem } from "@hoangvvo/llm-agent";
import { zodTool } from "@hoangvvo/llm-agent/zod";
import { diffLines } from "diff";
import { z } from "zod";
import { getModel } from "./get-model.ts";

// Artifacts/Canvas feature example: the agent maintains named deliverables
// (documents/code/specs) separate from the chat by using tools to create,
// update, retrieve, list, and delete artifacts.

type ArtifactKind = "markdown" | "text" | "code";

interface Artifact {
  id: string;
  title: string;
  kind: ArtifactKind;
  content: string;
  version: number;
  updated_at: string;
}

class InMemoryArtifactStore {
  #map = new Map<string, Artifact>();

  create(a: {
    id?: string;
    title: string;
    kind: ArtifactKind;
    content: string;
  }): Artifact {
    const id = (a.id?.trim() ? a.id : Math.random().toString(36).slice(2, 11))!;
    const now = new Date().toISOString();
    const artifact: Artifact = {
      id,
      title: a.title,
      kind: a.kind,
      content: a.content,
      version: 1,
      updated_at: now,
    };
    this.#map.set(id, artifact);
    return artifact;
  }

  update(a: { id: string; content: string }): Artifact {
    const existing = this.#map.get(a.id);
    if (!existing) throw new Error(`Artifact not found: ${a.id}`);
    const next: Artifact = {
      ...existing,
      content: a.content,
      version: existing.version + 1,
      updated_at: new Date().toISOString(),
    };
    this.#map.set(a.id, next);
    return next;
  }

  get(id: string): Artifact {
    const a = this.#map.get(id);
    if (!a) throw new Error(`Artifact not found: ${id}`);
    return a;
  }

  list(): Artifact[] {
    return [...this.#map.values()].sort((a, b) => a.id.localeCompare(b.id));
  }

  delete(id: string): { success: boolean } {
    const existed = this.#map.delete(id);
    return { success: existed };
  }
}

const store = new InMemoryArtifactStore();
const model = getModel("openai", "gpt-4o");

const overviewPrompt = `Use artifacts (documents/canvases) for substantive deliverables like documents, plans, specs, or code. Keep chat replies brief and status-oriented; put the full content into an artifact via the tools. Always reference artifacts by id.`;

const rulesPrompt = `
- Prefer creating/updating artifacts instead of pasting large content into chat
- When asked to revise or extend prior work, read/update the relevant artifact
- Keep the chat response short: what changed, where it lives (artifact id), and next steps
`;

// Minimal colored diff rendering (single dep: diff)
const color = (s: string, code: number) => `\x1b[${code}m${s}\x1b[0m`;
const green = (s: string) => color(s, 32);
const red = (s: string) => color(s, 31);
const dim = (s: string) => color(s, 2);
function renderDiff(oldText: string, newText: string): string {
  const parts = diffLines(oldText, newText);
  const lines: string[] = [];
  for (const p of parts) {
    const valLines = p.value.replace(/\n$/, "").split("\n");
    for (const ln of valLines) {
      if (p.added) lines.push(green(`+ ${ln}`));
      else if (p.removed) lines.push(red(`- ${ln}`));
      else lines.push(dim(`  ${ln}`));
    }
  }
  return lines.join("\n");
}

const artifactsAgent = new Agent<void>({
  name: "artifacts",
  model,
  instructions: [overviewPrompt, rulesPrompt],
  tools: [
    zodTool({
      name: "artifact_create",
      description:
        "Create a new artifact (document/canvas) and return the created artifact",
      parameters: z.object({
        title: z.string(),
        kind: z.enum(["markdown", "text", "code"]),
        content: z.string(),
      }),
      async execute(args) {
        console.log(
          `[artifacts.create] id=(auto) title=${args.title} kind=${args.kind}`,
        );
        const artifact = store.create(args);
        return {
          content: [{ type: "text", text: JSON.stringify({ artifact }) }],
          is_error: false,
        };
      },
    }),
    zodTool({
      name: "artifact_update",
      description: "Replace the content of an existing artifact and return it",
      parameters: z.object({ id: z.string(), content: z.string() }),
      async execute({ id, content }) {
        const before = store.get(id).content;
        console.log(`[artifacts.update] id=${id} len=${content.length}`);
        const artifact = store.update({ id, content });
        console.log(
          "\n=== Diff (old → new) ===\n" +
            renderDiff(before, artifact.content) +
            "\n========================\n",
        );
        return {
          content: [{ type: "text", text: JSON.stringify({ artifact }) }],
          is_error: false,
        };
      },
    }),
    zodTool({
      name: "artifact_get",
      description: "Fetch a single artifact by id",
      parameters: z.object({ id: z.string() }),
      async execute({ id }) {
        console.log(`[artifacts.get] id=${id}`);
        const artifact = store.get(id);
        return {
          content: [{ type: "text", text: JSON.stringify({ artifact }) }],
          is_error: false,
        };
      },
    }),
    zodTool({
      name: "artifact_list",
      description: "List all artifacts",
      parameters: z.object({}),
      async execute() {
        console.log(`[artifacts.list]`);
        const artifacts = store.list();
        return {
          content: [{ type: "text", text: JSON.stringify({ artifacts }) }],
          is_error: false,
        };
      },
    }),
    zodTool({
      name: "artifact_delete",
      description: "Delete an artifact by id",
      parameters: z.object({ id: z.string() }),
      async execute({ id }) {
        console.log(`[artifacts.delete] id=${id}`);
        const result = store.delete(id);
        return {
          content: [{ type: "text", text: JSON.stringify(result) }],
          is_error: false,
        };
      },
    }),
  ],
});

// Demo: ask the agent to create an artifact, then revise it.
const items1: AgentItem[] = [
  {
    type: "message",
    role: "user",
    content: [
      {
        type: "text",
        text: `We need a product requirements document for a new Todo app.
Please draft it in markdown with sections: Overview, Goals, Non-Goals, Requirements.
Keep your chat reply short and save the full document to a separate document we can keep iterating on.`,
      },
    ],
  },
];

const res1 = await artifactsAgent.run({ context: undefined, input: items1 });
console.dir(res1.content, { depth: null });
console.log("Artifacts after creation:");
console.dir(store.list(), { depth: null });

const items2: AgentItem[] = [
  {
    type: "message",
    role: "user",
    content: [
      {
        type: "text",
        text: `Please revise the document: expand the Goals section with 3 concrete goals and add a Milestones section. Keep your chat reply brief.`,
      },
    ],
  },
];

const res2 = await artifactsAgent.run({ context: undefined, input: items2 });
console.dir(res2.content, { depth: null });
console.log("Artifacts after update:");
console.dir(store.list(), { depth: null });
