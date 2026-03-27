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

const model = getModel("openai", "gpt-4o");

const tools: Tool[] = [
  {
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
    const { tool_call_id, tool_name } = toolCallPart;

    const toolResult = (() => {
      switch (tool_name) {
        case "get_color_sample":
          return getColorSample();
        default:
          throw new Error(`Tool ${tool_name} not found`);
      }
    })();

    toolMessage = toolMessage ?? {
      role: "tool",
      content: [],
    };

    toolMessage.content.push({
      type: "tool-result",
      tool_name,
      tool_call_id,
      content: [
        {
          type: "image",
          mime_type: toolResult.mime_type,
          data: toolResult.data,
        },
      ],
    });
  }

  if (toolMessage) messages.push(toolMessage);
} while (maxTurnLeft-- > 0);

console.dir(response, { depth: null });
