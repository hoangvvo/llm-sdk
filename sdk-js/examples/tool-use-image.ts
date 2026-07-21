import type {
  Message,
  ModelResponse,
  Tool,
  ToolMessage,
} from "@hoangvvo/llm-sdk";
import { getModel } from "./get-model.ts";

const RED_PIXEL_PNG_BASE64 =
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFBQIAX8jx0gAAAABJRU5ErkJggg==";

function getColorSample() {
  console.log("[TOOLS getColorSample()] Returning a red sample image");

  return {
    mime_type: "image/png",
    data: RED_PIXEL_PNG_BASE64,
  };
}

let maxTurnLeft = 10;

const provider = process.env["PROVIDER"] ?? "openai";
const modelId = process.env["MODEL"] ?? "gpt-5.6-terra";
const model = getModel(provider, modelId);

const tools: Tool[] = [
  {
    type: "function",
    name: "get_color_sample",
    description: "Get a color sample image",
    parameters: {
      type: "object",
      properties: {},
      additionalProperties: false,
    },
  },
];

const messages: Message[] = [
  {
    role: "user",
    content: [
      {
        type: "text",
        text: "What color is the image returned by the tool? Answer with one word.",
      },
    ],
  },
];

let response: ModelResponse;

do {
  response = await model.generate({
    messages,
    tools,
  });

  messages.push({
    role: "assistant",
    content: response.content,
  });

  const toolCallParts = response.content.filter((c) => c.type === "tool-call");

  if (toolCallParts.length === 0) {
    break;
  }

  let toolMessage: ToolMessage | undefined;

  for (const toolCallPart of toolCallParts) {
    if (toolCallPart.call.type !== "function") continue;
    const { tool_call_id, call } = toolCallPart;

    const toolResult = (() => {
      switch (call.name) {
        case "get_color_sample":
          return getColorSample();
        default:
          throw new Error(`Tool ${call.name} not found`);
      }
    })();

    toolMessage = toolMessage ?? {
      role: "tool",
      content: [],
    };

    toolMessage.content.push({
      type: "tool-result",
      status: "completed",
      tool_call_id,
      result: {
        type: "function",
        name: call.name,
        content: [
          {
            type: "image",
            mime_type: toolResult.mime_type,
            data: toolResult.data,
          },
        ],
      },
    });
  }

  if (toolMessage) messages.push(toolMessage);
} while (maxTurnLeft-- > 0);

console.dir(response, { depth: null });
