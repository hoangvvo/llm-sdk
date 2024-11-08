import Anthropic from "@anthropic-ai/sdk";
import type {
  LanguageModel,
  LanguageModelMetadata,
} from "../models/language-model.js";
import type {
  AssistantMessage,
  ContentDelta,
  LanguageModelInput,
  Message,
  ModelResponse,
  ModelUsage,
  PartialModelResponse,
  TextPart,
  Tool,
  ToolCallPart,
} from "../schemas/index.js";
import { convertAudioPartsToTextParts } from "../utils/message.utils.js";
import { ContentDeltaAccumulator } from "../utils/stream.utils.js";
import { calculateCost } from "../utils/usage.utils.js";
import type { AnthropicModelOptions } from "./types.js";

export type AnthropicLanguageModelInput = LanguageModelInput & {
  extra?: Partial<Anthropic.Messages.MessageCreateParams>;
};

export class AnthropicModel implements LanguageModel {
  provider: string;
  modelId: string;
  private anthropic: Anthropic;
  public metadata?: LanguageModelMetadata;

  constructor(
    public options: AnthropicModelOptions,
    metadata?: LanguageModelMetadata,
  ) {
    this.provider = "anthropic";
    this.modelId = options.modelId;
    if (metadata) this.metadata = metadata;

    this.anthropic = new Anthropic({
      baseURL: options.baseURL,
      apiKey: options.apiKey,
    });
  }

  async generate(input: AnthropicLanguageModelInput): Promise<ModelResponse> {
    const response = await this.anthropic.messages.create({
      ...convertToAnthropicParams(input, this.options),
      stream: false,
    });

    const usage: ModelUsage = {
      inputTokens: response.usage.input_tokens,
      outputTokens: response.usage.output_tokens,
    };

    return {
      content: mapAnthropicMessage(response.content).content,
      usage,
      ...(this.metadata?.pricing && {
        cost: calculateCost(usage, this.metadata.pricing),
      }),
    };
  }

  async *stream(
    input: AnthropicLanguageModelInput,
  ): AsyncGenerator<PartialModelResponse, ModelResponse> {
    const stream = this.anthropic.messages.stream({
      ...convertToAnthropicParams(input, this.options),
      stream: true,
    });

    const usage: ModelUsage = {
      inputTokens: 0,
      outputTokens: 0,
    };

    const accumulator = new ContentDeltaAccumulator();

    for await (const chunk of stream) {
      // https://docs.anthropic.com/claude/reference/messages-streaming#raw-http-stream-response

      switch (chunk.type) {
        case "message_start":
          usage.inputTokens += chunk.message.usage.input_tokens;
          usage.outputTokens += chunk.message.usage.output_tokens;
          break;
        case "message_delta":
          usage.outputTokens += chunk.usage.output_tokens;
          break;
        case "content_block_start":
        case "content_block_delta": {
          const incomingContentDeltas = mapAnthropicStreamEvent(chunk);
          accumulator.addChunks(incomingContentDeltas);

          for (const delta of incomingContentDeltas) {
            yield { delta };
          }
          break;
        }
      }
    }

    return {
      content: accumulator.computeContent(),
      usage,
      ...(this.metadata?.pricing && {
        cost: calculateCost(usage, this.metadata.pricing),
      }),
    };
  }
}

export function convertToAnthropicParams(
  input: LanguageModelInput,
  options: AnthropicModelOptions,
): Anthropic.Messages.MessageCreateParams {
  const tool_choice = convertToAnthropicToolChoice(input.toolChoice);

  const sampleParams = convertToAnthropicSamplingParams(input);

  return {
    model: options.modelId,
    messages: convertToAnthropicMessages(input.messages, options),
    ...(input.systemPrompt && { system: input.systemPrompt }),
    ...(input.tools &&
      input.toolChoice?.type !== "none" && {
        tools: input.tools.map(convertToAnthropicTool),
      }),
    ...(tool_choice && { tool_choice }),
    ...sampleParams,
    max_tokens: sampleParams.max_tokens || 4096,
    ...input.extra,
  };
}

export function convertToAnthropicMessages(
  messages: Message[],
  options: AnthropicModelOptions,
): Anthropic.Messages.MessageParam[] {
  if (options.convertAudioPartsToTextParts) {
    messages = messages.map(convertAudioPartsToTextParts);
  }

  return messages.map((message): Anthropic.Messages.MessageParam => {
    switch (message.role) {
      case "assistant": {
        return {
          role: "assistant",
          content: message.content.map(
            (
              part,
            ):
              | Anthropic.Messages.TextBlockParam
              | Anthropic.Messages.ToolUseBlockParam => {
              switch (part.type) {
                case "text":
                  return {
                    type: "text",
                    text: part.text,
                  };
                case "tool-call":
                  return {
                    type: "tool_use",
                    id: part.toolCallId,
                    name: part.toolName,
                    input: part.args,
                  };
                case "audio":
                  throw new Error("Audio parts are not supported");
                default: {
                  const exhaustiveCheck: never = part;
                  throw new Error(
                    `Unsupported message part type: ${(exhaustiveCheck as { type: string }).type}`,
                  );
                }
              }
            },
          ),
        };
      }

      case "tool": {
        // anthropic does not have a dedicated tool message type
        return {
          role: "user",
          content: message.content.map(
            (part): Anthropic.Messages.ToolResultBlockParam => ({
              type: "tool_result",
              tool_use_id: part.toolCallId,
              content: [
                {
                  type: "text",
                  text: JSON.stringify(part.result),
                },
              ],
            }),
          ),
        };
      }

      case "user": {
        return {
          role: "user",
          content: message.content.map(
            (
              part,
            ):
              | Anthropic.Messages.TextBlockParam
              | Anthropic.Messages.ImageBlockParam => {
              switch (part.type) {
                case "text":
                  return {
                    type: "text",
                    text: part.text,
                  };
                case "image":
                  return {
                    type: "image",
                    source: {
                      data: part.imageData,
                      type: "base64",
                      media_type:
                        part.mimeType as Anthropic.Messages.ImageBlockParam["source"]["media_type"],
                    },
                  };
                case "audio":
                  throw new Error("Audio parts are not supported");
                default: {
                  const exhaustiveCheck: never = part;
                  throw new Error(
                    `Unsupported message part type: ${(exhaustiveCheck as { type: string }).type}`,
                  );
                }
              }
            },
          ),
        };
      }

      default: {
        const exhaustiveCheck: never = message;
        throw new Error(
          `Unsupported message role: ${(exhaustiveCheck as { role: string }).role}`,
        );
      }
    }
  });
}

export function convertToAnthropicSamplingParams(
  input: Partial<LanguageModelInput>,
) {
  return {
    ...(input.maxTokens && { max_tokens: input.maxTokens || 4096 }),
    ...(typeof input.temperature === "number" && {
      temperature: input.temperature,
    }),
    ...(typeof input.topP === "number" && { top_p: input.topP }),
    ...(typeof input.topK === "number" && { top_k: input.topK }),
  } satisfies Partial<Anthropic.Messages.MessageCreateParams>;
}

export function convertToAnthropicToolChoice(
  toolChoice: LanguageModelInput["toolChoice"],
): Anthropic.Messages.MessageCreateParams["tool_choice"] {
  if (!toolChoice) {
    return undefined;
  }

  switch (toolChoice.type) {
    case "auto":
      return { type: "auto" };
    case "required":
      return { type: "any" };
    case "tool":
      return {
        type: "tool",
        name: toolChoice.toolName,
      };
    case "none": {
      // already handled in convertToAnthropicParams
      return undefined;
    }
    default: {
      const exhaustiveCheck: never = toolChoice;
      throw new Error(
        `Unsupported tool choice type: ${(exhaustiveCheck as { type: string }).type}`,
      );
    }
  }
}

export function convertToAnthropicTool(tool: Tool): Anthropic.Tool {
  return {
    name: tool.name,
    description: tool.description,
    // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
    input_schema: (tool.parameters as Anthropic.Tool.InputSchema) || {
      type: "object",
      // anthropic tool call parameters are required
      // so if no parameters, we define it as null
      properties: null,
    },
  };
}

export function mapAnthropicMessage(
  content: Array<Anthropic.Messages.ContentBlock>,
): AssistantMessage {
  return {
    role: "assistant",
    content: content.map(mapAnthropicBlock),
  };
}

export function mapAnthropicBlock(block: Anthropic.Messages.ContentBlock) {
  switch (block.type) {
    case "text":
      return {
        type: "text",
        text: block.text,
      } satisfies TextPart;
    case "tool_use":
      return {
        type: "tool-call",
        toolCallId: block.id,
        toolName: block.name,
        args: block.input as Record<string, unknown>,
      } satisfies ToolCallPart;
    default: {
      const exhaustiveCheck: never = block;
      throw new Error(
        `Unsupported block type: ${(exhaustiveCheck as { type: string }).type}`,
      );
    }
  }
}

export function mapAnthropicStreamEvent(
  chunk: Anthropic.Messages.RawMessageStreamEvent,
): ContentDelta[] {
  switch (chunk.type) {
    case "content_block_start":
      switch (chunk.content_block.type) {
        case "text":
          return [
            {
              index: chunk.index,
              part: {
                type: "text",
                text: chunk.content_block.text,
              },
            },
          ];
        case "tool_use": {
          return [
            {
              index: chunk.index,
              part: {
                type: "tool-call",
                toolCallId: chunk.content_block.id,
                toolName: chunk.content_block.name,
              },
            },
          ];
        }
        default: {
          const exhaustiveCheck: never = chunk.content_block;
          throw new Error(
            `Unsupported content block type: ${(exhaustiveCheck as { type: string }).type}`,
          );
        }
      }
    case "content_block_delta":
      switch (chunk.delta.type) {
        case "text_delta":
          return [
            {
              index: chunk.index,
              part: {
                type: "text",
                text: chunk.delta.text,
              },
            },
          ];
        case "input_json_delta":
          return [
            {
              index: chunk.index,
              part: {
                type: "tool-call",
                args: chunk.delta.partial_json,
              },
            },
          ];
        default: {
          const exhaustiveCheck: never = chunk.delta;
          throw new Error(
            `Unsupported delta type: ${(exhaustiveCheck as { type: string }).type}`,
          );
        }
      }
    default:
      return [];
  }
}
