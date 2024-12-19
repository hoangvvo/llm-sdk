import Anthropic from "@anthropic-ai/sdk";
import {
  InvalidValueError,
  ModelUnsupportedMessagePart,
  NotImplementedError,
} from "../errors/errors.js";
import {
  LanguageModel,
  type LanguageModelMetadata,
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
} from "../types.js";
import { convertAudioPartsToTextParts } from "../utils/message.utils.js";
import { ContentDeltaAccumulator } from "../utils/stream.utils.js";
import { calculateCost } from "../utils/usage.utils.js";
import type { AnthropicModelOptions } from "./types.js";

export type AnthropicLanguageModelInput = LanguageModelInput & {
  extra?: Partial<Anthropic.Messages.MessageCreateParams>;
};

export class AnthropicModel extends LanguageModel {
  public provider: string;
  public modelId: string;
  public metadata?: LanguageModelMetadata;

  private anthropic: Anthropic;

  constructor(
    public options: AnthropicModelOptions,
    metadata?: LanguageModelMetadata,
  ) {
    super();
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
      input_tokens: response.usage.input_tokens,
      output_tokens: response.usage.output_tokens,
    };

    const result: ModelResponse = {
      content: mapAnthropicMessage(response.content).content,
      usage,
    };
    if (this.metadata?.pricing) {
      result.cost = calculateCost(usage, this.metadata.pricing);
    }
    return result;
  }

  async *stream(
    input: AnthropicLanguageModelInput,
  ): AsyncGenerator<PartialModelResponse, ModelResponse> {
    const stream = this.anthropic.messages.stream({
      ...convertToAnthropicParams(input, this.options),
      stream: true,
    });

    const usage: ModelUsage = {
      input_tokens: 0,
      output_tokens: 0,
    };

    const accumulator = new ContentDeltaAccumulator();

    for await (const _chunk of stream) {
      // TODO: type error from library
      const chunk = _chunk as Anthropic.Messages.MessageStreamEvent;

      // https://docs.anthropic.com/claude/reference/messages-streaming#raw-http-stream-response
      switch (chunk.type) {
        case "message_start":
          usage.input_tokens += chunk.message.usage.input_tokens;
          usage.output_tokens += chunk.message.usage.output_tokens;
          break;
        case "message_delta":
          usage.output_tokens += chunk.usage.output_tokens;
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

    const result: ModelResponse = {
      content: accumulator.computeContent(),
      usage,
    };
    if (this.metadata?.pricing) {
      result.cost = calculateCost(usage, this.metadata.pricing);
    }
    return result;
  }
}

export function convertToAnthropicParams(
  input: LanguageModelInput,
  options: AnthropicModelOptions,
): Anthropic.Messages.MessageCreateParams {
  const tool_choice = convertToAnthropicToolChoice(input.tool_choice);

  const sampleParams = convertToAnthropicSamplingParams(input);

  const params: Anthropic.Messages.MessageCreateParams = {
    model: options.modelId,
    messages: convertToAnthropicMessages(input.messages, options),
    ...sampleParams,
    max_tokens: sampleParams.max_tokens || 4096,
    ...input.extra,
  };
  if (input.system_prompt) {
    params.system = input.system_prompt;
  }
  if (input.tools && input.tool_choice?.type !== "none") {
    params.tools = input.tools.map(convertToAnthropicTool);
  }
  if (tool_choice) {
    params.tool_choice = tool_choice;
  }
  return params;
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
                    id: part.tool_call_id,
                    name: part.tool_name,
                    input: part.args,
                  };
                case "audio":
                  throw new ModelUnsupportedMessagePart("anthropic", part.type);
                default: {
                  const exhaustiveCheck: never = part;
                  throw new InvalidValueError(
                    "part.type",
                    (exhaustiveCheck as { type: string }).type,
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
              tool_use_id: part.tool_call_id,
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
                      data: part.image_data,
                      type: "base64",
                      media_type:
                        part.mime_type as Anthropic.Messages.ImageBlockParam["source"]["media_type"],
                    },
                  };
                case "audio":
                  throw new ModelUnsupportedMessagePart("anthropic", part.type);
                default: {
                  const exhaustiveCheck: never = part;
                  throw new InvalidValueError(
                    "part.type",
                    (exhaustiveCheck as { type: string }).type,
                  );
                }
              }
            },
          ),
        };
      }

      default: {
        const exhaustiveCheck: never = message;
        throw new InvalidValueError(
          "message.role",
          (exhaustiveCheck as { role: string }).role,
        );
      }
    }
  });
}

export function convertToAnthropicSamplingParams(
  input: Partial<LanguageModelInput>,
): Partial<Anthropic.Messages.MessageCreateParams> {
  const params: Partial<Anthropic.Messages.MessageCreateParams> = {};
  if (input.max_tokens) {
    params.max_tokens = input.max_tokens || 4096;
  }
  if (typeof input.temperature === "number") {
    params.temperature = input.temperature;
  }
  if (typeof input.top_p === "number") {
    params.top_p = input.top_p;
  }
  if (typeof input.top_k === "number") {
    params.top_k = input.top_k;
  }
  return params;
}

export function convertToAnthropicToolChoice(
  toolChoice: LanguageModelInput["tool_choice"],
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
        name: toolChoice.tool_name,
      };
    case "none": {
      // already handled in convertToAnthropicParams
      return undefined;
    }
    default: {
      const exhaustiveCheck: never = toolChoice;
      throw new InvalidValueError(
        "toolChoice.type",
        (exhaustiveCheck as { type: string }).type,
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
        tool_call_id: block.id,
        tool_name: block.name,
        args: block.input as Record<string, unknown>,
      } satisfies ToolCallPart;
    default: {
      const exhaustiveCheck: never = block;
      throw new NotImplementedError(
        "block.type",
        (exhaustiveCheck as { type: string }).type,
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
                tool_call_id: chunk.content_block.id,
                tool_name: chunk.content_block.name,
              },
            },
          ];
        }
        default: {
          const exhaustiveCheck: never = chunk.content_block;
          throw new NotImplementedError(
            "content_block.type",
            (exhaustiveCheck as { type: string }).type,
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
        case "citations_delta":
          return [];
        default: {
          const exhaustiveCheck: never = chunk.delta;
          throw new NotImplementedError(
            "delta.type",
            (exhaustiveCheck as { type: string }).type,
          );
        }
      }
    default:
      return [];
  }
}
