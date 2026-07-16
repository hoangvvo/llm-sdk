import { Agent, getContentText, tool } from "@hoangvvo/llm-agent";
import { setTimeout as sleep } from "node:timers/promises";
import { getModel } from "./get-model.ts";

const waitTool = tool<undefined, { seconds: number }>({
  name: "wait",
  description: "Wait for a requested number of seconds",
  parameters: {
    type: "object",
    properties: {
      seconds: { type: "number", minimum: 1 },
    },
    required: ["seconds"],
    additionalProperties: false,
  },
  async execute({ seconds }, _context, state) {
    // This timer is only for demonstration. Production APIs such as fetch
    // usually accept the signal directly.
    if (state.signal) {
      await sleep(seconds * 1_000, undefined, { signal: state.signal });
    } else {
      await sleep(seconds * 1_000);
    }

    return {
      content: [{ type: "text", text: "Finished waiting" }],
      is_error: false,
    };
  },
});

const provider = process.env["PROVIDER"] ?? "openai";
const modelId = process.env["MODEL"] ?? "gpt-5.6-terra";
const model = getModel(provider, modelId);

const agent = new Agent<undefined>({
  name: "CancellableAssistant",
  model,
  tools: [waitTool],
});

const controller = new AbortController();

// A Stop button or client disconnect would call abort().
const cancellationTimer = setTimeout(() => controller.abort(), 2_000);

try {
  const response = await agent.run(
    {
      context: undefined,
      input: [
        {
          type: "message",
          role: "user",
          content: [
            { type: "text", text: "Use the wait tool to wait for 30 seconds." },
          ],
        },
      ],
    },
    { signal: controller.signal },
  );

  if (response.status === "cancelled") {
    console.log("Run cancelled safely.");
  } else {
    console.log(getContentText(response));
  }
} finally {
  clearTimeout(cancellationTimer);
}
