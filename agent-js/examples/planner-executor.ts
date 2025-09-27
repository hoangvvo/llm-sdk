import { Agent, type AgentItem } from "@hoangvvo/llm-agent";
import { zodTool } from "@hoangvvo/llm-agent/zod";
import { z } from "zod";
import { getModel } from "./get-model.ts";

interface PlanItem {
  status: "pending" | "in_progress" | "complete";
  step: string;
}

class PlanStore {
  #plan: PlanItem[] = [];
  #explanation = "";
  list(): PlanItem[] {
    return this.#plan.slice();
  }
  set(plan: PlanItem[], explanation: string) {
    this.#plan = plan.slice();
    this.#explanation = explanation;
  }
  explanation(): string {
    return this.#explanation;
  }
}

const planStore = new PlanStore();

function formatPlan(): string {
  const list = planStore.list();
  const lines: string[] = [];
  lines.push(
    `\n─ PLAN (internal) · ${list.length} item${list.length === 1 ? "" : "s"}`,
  );
  const expl = planStore.explanation();
  if (expl) lines.push(`Explanation: ${expl}`);
  if (list.length === 0) {
    lines.push("(empty)");
  } else {
    const symbol = (s: PlanItem["status"]) =>
      s === "complete" ? "✓" : s === "in_progress" ? "▸" : "○";
    for (const t of list) {
      lines.push(`${symbol(t.status)} ${t.step}`);
    }
  }
  return lines.join("\n");
}

function clearAndRenderScreen(messages: string[]) {
  // Clear the console for a clean redraw
  try {
    console.clear();
  } catch {
    process.stdout.write("\x1b[2J\x1b[H");
  }
  // Print assistant messages back-to-back
  if (messages.length > 0) {
    process.stdout.write(messages.join("\n\n") + "\n\n");
  }
  // Always render internal plan at the bottom
  process.stdout.write(formatPlan() + "\n");
}

const updatePlanTool = zodTool({
  name: "update_plan",
  description:
    "Replace internal plan with a new list of steps (status + step) and optional explanation.",
  parameters: z.object({
    explanation: z.string(),
    plan: z
      .array(
        z
          .object({
            status: z.enum(["pending", "in_progress", "complete"]),
            step: z.string(),
          })
          .strict(),
      )
      .nonempty(),
  }),
  async execute({ explanation, plan }) {
    planStore.set(plan, explanation);
    return {
      content: [
        { type: "text", text: JSON.stringify({ ok: true, explanation, plan }) },
      ],
      is_error: false,
    };
  },
});

const model = getModel("openai", "gpt-4o");

const agent = new Agent<void>({
  name: "planner-executor",
  model,
  instructions: [
    `You are a planner–executor assistant.
Break the user's goal into clear, actionable steps using the tool update_plan (explanation, plan: [{status, step}]).
Use the TODO tools strictly as your internal plan: NEVER reveal or enumerate TODO items to the user. Do not mention the words TODO, task list, or the names of tools.
Keep user-visible replies concise and focused on results and next-step confirmations.
Work iteratively: plan an initial set of high-level steps, then refine/execute one major step per turn, marking completed items along the way via tools.
When the work is complete, respond with the final deliverable and a brief one-paragraph summary of what you did.`,
    () => {
      const rows = planStore
        .list()
        .map((p, i) => `${i + 1}. [${p.status}] ${p.step}`)
        .join("\n");
      const expl = planStore.explanation();
      return `INTERNAL PLAN:\n${rows}\nExplanation: ${expl}`;
    },
  ],
  tools: [updatePlanTool],
  max_turns: 20,
});

// Demo: multi-turn execution for a complex task
const items: AgentItem[] = [
  {
    type: "message",
    role: "user",
    content: [
      {
        type: "text",
        text:
          "You are hired to produce a concise PRD (Product Requirements Document) for a travel booking app. " +
          "Do high-level planning and execution across turns: outline the PRD structure, then draft sections " +
          "(Overview, Target Users, Core Features, MVP Scope, Non-Goals, Success Metrics, Risks), and finally " +
          "produce the final PRD in markdown. Keep replies brief and focused on progress/results only.",
      },
    ],
  },
];

const messages: string[] = [];
clearAndRenderScreen(messages);

for (let turn = 1; ; turn += 1) {
  const res = await agent.run({ input: items, context: undefined });

  // Capture only assistant-visible text to display back-to-back
  const visibleText = res.content
    .filter((p) => p.type === "text")
    .map((p) => p.text)
    .join("\n");
  if (visibleText.trim()) messages.push(visibleText.trim());

  clearAndRenderScreen(messages);

  // Append agent output items to the conversation
  items.push(...res.output);

  // Stop when plan exists and all steps have status DONE
  const plan = planStore.list();
  const havePlan = plan.length > 0;
  const allDone = havePlan && plan.every((p) => p.status === "complete");
  if (allDone) break;

  // Otherwise continue to next turn
  items.push({
    type: "message",
    role: "user",
    content: [{ type: "text", text: "NEXT" }],
  });
}

// Final render to ensure the last state persists on screen
clearAndRenderScreen(messages);
