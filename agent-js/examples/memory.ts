import {
  Agent,
  type AgentItem,
  type AgentItemMessage,
} from "@hoangvvo/llm-agent";
import { zodTool } from "@hoangvvo/llm-agent/zod";
import type { TextPart } from "@hoangvvo/llm-sdk";
import { z } from "zod";
import { getModel } from "./get-model.ts";

// Memory pattern example: provide tools + instructions for core/archival memory.

interface MemoryBlock {
  id: string;
  content: string;
}

class InMemoryStore {
  core = new Map<string, string>();
  archival = new Map<string, string>();

  fetchCore(): MemoryBlock[] {
    return [...this.core.entries()].map(([id, content]) => ({ id, content }));
  }
  updateCore(block: MemoryBlock): MemoryBlock[] {
    const { id, content } = block;
    if (!content.trim()) this.core.delete(id);
    else this.core.set(id, content);
    return this.fetchCore();
  }
  searchArchival(query: string): MemoryBlock[] {
    // TODO: Replace with semantic vector search using embeddings.
    const q = query.toLowerCase();
    return [...this.archival.entries()]
      .filter(
        ([id, c]) =>
          id.toLowerCase().includes(q) || c.toLowerCase().includes(q),
      )
      .map(([id, content]) => ({ id, content }));
  }
  updateArchival(block: MemoryBlock): void {
    const { id, content } = block;
    if (!content.trim()) this.archival.delete(id);
    else this.archival.set(id, content);
  }
}

const MEMORY_PROMPT = `You can remember information learned from interactions with the user in two types of memory called core memory and archival memory.
Core memory is always available in your conversation context, providing essential, foundational context for keeping track of key details about the user.
As core memory is limited in size, it is important to only store the most important information. For other less important details, use archival memory.
Archival memory is infinite size, but is held outside of your immediate context, so you must explicitly run a search operation to see data inside it.
Archival memory is used to remember less significant details about the user or information found during the conversation. When the user mentions a name, topic, or details you don't know, search your archival memory to see if you have any information about it.`;

const coreMemoryPrompt = (memories: MemoryBlock[]) =>
  `Core memories (JSON list):\n${JSON.stringify(memories)}`;

const store = new InMemoryStore();
const model = getModel("openai", "gpt-4o");

const memoryAgent = new Agent<void>({
  name: "memory",
  model,
  instructions: [
    MEMORY_PROMPT,
    `You cannot see prior conversation turns beyond what is provided in the current input. When a user shares a durable preference or profile detail, call core_memory_update to store it.
When asked to recall such facts and it's not present in the current input, rely on the core memories in this prompt.
For less important or long-tail info, use archival_memory_search before answering.`,
    async () => coreMemoryPrompt(store.fetchCore()),
  ],
  tools: [
    zodTool({
      name: "core_memory_update",
      description:
        "Update or add a core memory block. Returns all core memories after the update.",
      parameters: z.object({ id: z.string(), content: z.string() }),
      async execute({ id, content }) {
        console.log(
          `[memory.core_memory_update] id=${id} len=${content.length}`,
        );
        const memoryId = id?.trim()
          ? id
          : Math.random().toString(36).slice(2, 11);
        const updated = store.updateCore({ id: memoryId, content });
        return {
          content: [
            { type: "text", text: JSON.stringify({ core_memories: updated }) },
          ],
          is_error: false,
        };
      },
    }),
    zodTool({
      name: "archival_memory_search",
      description: "Search for memories in the archival memory",
      parameters: z.object({ query: z.string() }),
      async execute({ query }) {
        console.log(`[memory.archival_memory_search] query="${query}"`);
        // TODO: Replace with semantic vector search using embeddings.
        const results = store.searchArchival(query);
        return {
          content: [{ type: "text", text: JSON.stringify({ results }) }],
          is_error: false,
        };
      },
    }),
    zodTool({
      name: "archival_memory_update",
      description: "Update or add a memory block in the archival memory",
      parameters: z.object({ id: z.string(), content: z.string() }),
      async execute({ id, content }) {
        console.log(
          `[memory.archival_memory_update] id=${id} len=${content.length}`,
        );
        // TODO: store vector embedding for semantic search
        const memoryId = id?.trim()
          ? id
          : Math.random().toString(36).slice(2, 11);
        store.updateArchival({ id: memoryId, content });
        const result =
          content.trim() === ""
            ? { success: true, action: "deleted" }
            : {
                success: true,
                action: "updated",
                memory: { id: memoryId, content },
              };
        return {
          content: [{ type: "text", text: JSON.stringify(result) }],
          is_error: false,
        };
      },
    }),
  ],
});

// Demo: four independent turns to show core + archival memory
// Turn 1 — store a core memory
const items1: AgentItem[] = [
  {
    type: "message",
    role: "user",
    content: [
      { type: "text", text: "Remember that my favorite color is blue." },
    ],
  },
];
console.log(
  `[user] ${((items1[0] as AgentItemMessage).content[0] as TextPart).text}`,
);
const res1 = await memoryAgent.run({ context: undefined, input: items1 });
console.dir(res1.content, { depth: null });

// Turn 2 — recall using core memory (no prior messages)
const items2: AgentItem[] = [
  {
    type: "message",
    role: "user",
    content: [{ type: "text", text: "What's my favorite color?" }],
  },
];
console.log(
  `[user] ${((items2[0] as AgentItemMessage).content[0] as TextPart).text}`,
);
const res2 = await memoryAgent.run({ context: undefined, input: items2 });
console.dir(res2.content, { depth: null });

// Turn 3 — store less-important info in archival memory
const items3: AgentItem[] = [
  {
    type: "message",
    role: "user",
    content: [
      {
        type: "text",
        text:
          "I captured some background notes titled 'q3-report-research' for future reference: " +
          "Key data sources for the Q3 report include Salesforce pipeline exports, Google Analytics weekly sessions, and the paid ads spend spreadsheet. " +
          "Please tuck this away so you can look it up later.",
      },
    ],
  },
];
console.log(
  `[user] ${((items3[0] as AgentItemMessage).content[0] as TextPart).text}`,
);
const res3 = await memoryAgent.run({ context: undefined, input: items3 });
console.dir(res3.content, { depth: null });

// Turn 4 — recall via archival search (no prior messages)
const items4: AgentItem[] = [
  {
    type: "message",
    role: "user",
    content: [
      {
        type: "text",
        text: "Can you pull up what we have under 'q3-report-research'?",
      },
    ],
  },
];
console.log(
  `[user] ${((items4[0] as AgentItemMessage).content[0] as TextPart).text}`,
);
const res4 = await memoryAgent.run({ context: undefined, input: items4 });
console.dir(res4.content, { depth: null });
