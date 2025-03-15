import OpenAI from "openai";
import {
  InvalidInputError,
  InvariantError,
  RefusalError,
  UnsupportedError,
} from "../errors.ts";
import type {
  LanguageModel,
  LanguageModelMetadata,
} from "../language-model.ts";
import { traceLanguageModel } from "../opentelemetry.ts";
import { getCompatiblePartsWithoutSourceParts } from "../source-part.utils.ts";
import type {
  ContentDelta,
  ImagePartDelta,
  LanguageModelInput,
  Message,
  ModelResponse,
  ModelUsage,
  Part,
  PartialModelResponse,
  ReasoningPartDelta,
  ResponseFormatOption,
  TextPartDelta,
  Tool,
  ToolCallPartDelta,
  ToolChoiceOption,
} from "../types.ts";
import { calculateCost } from "../usage.utils.ts";
import type { OpenAIModelOptions } from "./options.ts";
import type {
  OpenAIPatchedImageGenerationCallPartialImage,
  OpenAIPatchedResponsesImageGenerationCall,
} from "./types.ts";

const PROVIDER = "openai";

export class OpenAIModel implements LanguageModel {
  provider: string;
  modelId: string;
  metadata?: LanguageModelMetadata;

  #openai: OpenAI;

  constructor(options: OpenAIModelOptions, metadata?: LanguageModelMetadata) {
    this.provider = PROVIDER;
    this.modelId = options.modelId;
    if (metadata) this.metadata = metadata;
    this.#openai = new OpenAI({
      baseURL: options.baseURL,
      apiKey: options.apiKey,
    });

    traceLanguageModel(this);
  }

  async generate(input: LanguageModelInput): Promise<ModelResponse> {
    const createParams = convertToOpenAICreateParams(input, this.modelId);

    const response = await this.#openai.responses.create({
      ...createParams,
      stream: false,
    });

    const content = mapOpenAIOutputItems(response.output);

    const result: ModelResponse = {
      content,
    };

    if (response.usage) {
      result.usage = mapOpenAIUsage(response.usage);
      if (this.metadata?.pricing) {
        result.cost = calculateCost(result.usage, this.metadata.pricing);
      }
    }

    return result;
  }

  async *stream(
    input: LanguageModelInput,
  ): AsyncGenerator<PartialModelResponse> {
    const createParams = convertToOpenAICreateParams(input, this.modelId);
    const stream = await this.#openai.responses.create({
      ...createParams,
      stream: true,
    });

    let refusal = "";

    for await (const event of stream) {
      if (event.type === "response.refusal.delta") {
        refusal += event.delta;
      }

      const partDelta = mapOpenAIStreamEvent(event);
      if (partDelta) {
        const partial: PartialModelResponse = { delta: partDelta };
        yield partial;
      }

      if (event.type === "response.completed") {
        if (event.response.usage) {
          const usage = mapOpenAIUsage(event.response.usage);
          const partial: PartialModelResponse = { usage };
          yield partial;
        }
      }
    }

    if (refusal) {
      throw new RefusalError(refusal);
    }
  }
}

function convertToOpenAICreateParams(
  input: LanguageModelInput,
  modelId: string,
): Omit<OpenAI.Responses.ResponseCreateParams, "stream"> {
  const {
    messages,
    system_prompt,
    max_tokens,
    temperature,
    top_p,
    response_format,
    tools,
    tool_choice,
    extra,
    modalities,
  } = input;

  const params: Omit<OpenAI.Responses.ResponseCreateParams, "stream"> = {
    store: false,
    model: modelId,
    input: convertToOpenAIInputs(messages),
    ...(system_prompt && { instructions: system_prompt }),
    max_output_tokens: max_tokens ?? null,
    temperature: temperature ?? null,
    top_p: top_p ?? null,
    ...(tools && {
      tools: tools.map(convertToOpenAITool),
    }),
    ...(tool_choice && {
      tool_choice: convertToOpenAIToolChoice(tool_choice),
    }),
    ...(response_format && {
      text: convertToOpenAIResponseTextConfig(response_format),
    }),
    reasoning: {
      summary: "auto",
    },
  };

  if (modalities?.includes("image")) {
    params.tools = params.tools ?? [];
    params.tools.push({
      type: "image_generation",
    });
  }

  return { ...params, ...extra };
}

// MARK: To Provider Messages

function convertToOpenAIInputs(
  messages: Message[],
): OpenAI.Responses.ResponseInputItem[] {
  return messages
    .map((message): OpenAI.Responses.ResponseInputItem[] => {
      switch (message.role) {
        case "user": {
          const messageParts = getCompatiblePartsWithoutSourceParts(
            message.content,
          );

          return [
            {
              type: "message",
              role: "user",
              content: messageParts.map(convertToOpenAIResponseInputContent),
            },
          ];
        }
        case "assistant": {
          const messageParts = getCompatiblePartsWithoutSourceParts(
            message.content,
          );

          return messageParts.map(
            (part): OpenAI.Responses.ResponseInputItem => {
              switch (part.type) {
                case "text":
                  return {
                    // Response output item requires an ID.
                    // This usually applies if we enable OpenAI "store".
                    // or that we propogate the message ID in output.
                    // For compatibility, we want to avoid doing that, so we use a generated ID
                    // to avoid the API from returning an error.
                    id: "msg_" + genidForMessageId(),
                    type: "message",
                    role: "assistant",
                    status: "completed",
                    content: [
                      {
                        type: "output_text",
                        text: part.text,
                        annotations: [],
                      },
                    ],
                  };
                case "reasoning":
                  return {
                    type: "reasoning",
                    // Similar to assistant message parts, we generate a unique ID for each reasoning part.
                    id: `reasoning_${genidForMessageId()}`,
                    summary: [
                      {
                        type: "summary_text",
                        text: part.summary ?? part.text,
                      },
                    ],
                    content: [
                      {
                        type: "reasoning_text",
                        text: part.text,
                      },
                    ],
                    ...(part.signature && {
                      encrypted_content: part.signature,
                    }),
                    status: "completed",
                  };
                case "image":
                  return {
                    id: "",
                    type: "image_generation_call",
                    status: "completed",
                    result: `data:${part.mime_type};base64,${part.image_data}`,
                  };
                case "tool-call":
                  return {
                    type: "function_call",
                    call_id: part.tool_call_id,
                    name: part.tool_name,
                    arguments: JSON.stringify(part.args),
                  };
                default:
                  throw new UnsupportedError(
                    PROVIDER,
                    `Cannot convert part to OpenAI ResponseInputItem for type ${part.type} of assistant message`,
                  );
              }
            },
          );
        }
        case "tool": {
          return message.content
            .map((part): OpenAI.Responses.ResponseInputItem[] => {
              if (part.type !== "tool-result") {
                throw new InvalidInputError(
                  "Tool messages must contain only tool result parts",
                );
              }

              const toolResultPartContent =
                getCompatiblePartsWithoutSourceParts(part.content);

              return toolResultPartContent.map(
                (
                  toolResultPartPart,
                ): OpenAI.Responses.ResponseInputItem.FunctionCallOutput => {
                  switch (toolResultPartPart.type) {
                    case "text":
                      return {
                        type: "function_call_output",
                        call_id: part.tool_call_id,
                        output: toolResultPartPart.text,
                      };
                    default:
                      throw new UnsupportedError(
                        PROVIDER,
                        `Cannot convert tool result part to OpenAI ResponseInputItem.FunctionCallOutput for type ${toolResultPartPart.type}`,
                      );
                  }
                },
              );
            })
            .flat();
        }
      }
    })
    .flat();
}

function convertToOpenAIResponseInputContent(
  part: Part,
): OpenAI.Responses.ResponseInputContent {
  switch (part.type) {
    case "text":
      return { type: "input_text", text: part.text };
    case "image":
      return {
        type: "input_image",
        image_url: `data:${part.mime_type};base64,${part.image_data}`,
        detail: "auto",
      };
    case "audio": {
      let format: OpenAI.Responses.ResponseInputAudio.InputAudio["format"];
      switch (part.format) {
        case "mp3":
          format = "mp3";
          break;
        case "wav":
          format = "wav";
          break;
        default:
          throw new UnsupportedError(
            PROVIDER,
            `Cannot convert audio format to OpenAI InputAudio format for format ${part.format}`,
          );
      }
      return {
        type: "input_audio",
        input_audio: {
          data: part.audio_data,
          format,
        },
      };
    }
    default:
      throw new UnsupportedError(
        PROVIDER,
        `Cannot convert part to OpenAI content part for type ${part.type}`,
      );
  }
}

// MARK: To Provider Tools

function convertToOpenAITool(tool: Tool): OpenAI.Responses.Tool {
  return {
    type: "function",
    name: tool.name,
    description: tool.description,
    parameters: tool.parameters,
    strict: true,
  };
}

function convertToOpenAIToolChoice(
  toolChoice: ToolChoiceOption,
): NonNullable<OpenAI.Responses.ResponseCreateParams["tool_choice"]> {
  switch (toolChoice.type) {
    case "tool": {
      return {
        type: "function",
        name: toolChoice.tool_name,
      };
    }
    case "auto": {
      return "auto";
    }
    case "none": {
      return "none";
    }
    case "required": {
      return "required";
    }
  }
}

// MARK: To Provider Response Format

function convertToOpenAIResponseTextConfig(
  responseFormat: ResponseFormatOption,
): OpenAI.Responses.ResponseTextConfig {
  switch (responseFormat.type) {
    case "json": {
      if (responseFormat.schema) {
        return {
          format: {
            type: "json_schema",
            name: responseFormat.name,
            ...(responseFormat.description && {
              description: responseFormat.description,
            }),
            schema: responseFormat.schema,
            strict: true,
          },
        };
      }
      return {
        format: {
          type: "json_object",
        },
      };
    }
    case "text": {
      return {
        format: {
          type: "text",
        },
      };
    }
  }
}

// MARK: To SDK Message

function mapOpenAIOutputItems(
  items: OpenAI.Responses.ResponseOutputItem[],
): Part[] {
  return items
    .map((item): Part[] => {
      switch (item.type) {
        case "message": {
          return item.content.map((item): Part => {
            switch (item.type) {
              case "output_text":
                return {
                  type: "text",
                  text: item.text,
                };
              case "refusal":
                throw new RefusalError(item.refusal);
            }
          });
        }
        case "function_call": {
          return [
            {
              type: "tool-call",
              args: JSON.parse(item.arguments) as Record<string, unknown>,
              tool_call_id: item.call_id,
              tool_name: item.name,
            },
          ];
        }
        case "image_generation_call": {
          const patchedItem = item as OpenAIPatchedResponsesImageGenerationCall;

          if (!patchedItem.result) {
            throw new InvariantError(
              PROVIDER,
              "Image generation call did not return a result",
            );
          }

          let width: number | undefined;
          let height: number | undefined;
          if (patchedItem.size) {
            [width, height] = patchedItem.size.split("x").map(Number);
          }

          return [
            {
              type: "image",
              image_data: patchedItem.result,
              mime_type: `image/${patchedItem.output_format}`,
              ...(width && { width }),
              ...(height && { height }),
            },
          ];
        }
        case "reasoning": {
          const summary = item.summary.map((s) => s.text).join("\n");
          return [
            {
              type: "reasoning",
              text: item.content?.map((c) => c.text).join("\n") ?? summary,
              summary,
              ...(item.encrypted_content && {
                signature: item.encrypted_content,
              }),
            },
          ];
        }
        default: {
          return [];
        }
      }
    })
    .flat();
}

// MARK: To SDK Delta

function mapOpenAIStreamEvent(
  event: OpenAI.Responses.ResponseStreamEvent,
): ContentDelta | null {
  // OpenAI event outputs include "output_index" and "content_index".
  // Each item is indexed by "output_index". If an item contains multiple pieces of content (only applicable for text and refusal cases), it will also have a "content_index".
  // Text and refusal always belong to "ResponseOutputMessage", but refusal is already filtered out in the stream function. Therefore, the index will consistently be "output_index".
  // Function tool calls are always represented as separate "ResponseFunctionToolCall" items, so we can directly use "output_index" as the index.
  // The same applies to ImageGenerationCall and other similar cases.

  switch (event.type) {
    case "response.failed": {
      const errorMessage = event.response.error?.message;
      throw new InvariantError(
        PROVIDER,
        "OpenAI Response Stream failed" +
          (errorMessage ? `: ${errorMessage}` : ""),
      );
    }
    case "response.output_item.added": {
      if (event.item.type === "function_call") {
        const part: ToolCallPartDelta = {
          type: "tool-call",
          args: event.item.arguments,
          tool_name: event.item.name,
          tool_call_id: event.item.call_id,
        };
        const index = event.output_index;
        return {
          index,
          part,
        };
      }

      if (event.item.type === "reasoning") {
        if (event.item.encrypted_content) {
          const part: ReasoningPartDelta = {
            type: "reasoning",
            signature: event.item.encrypted_content,
          };

          const index = event.output_index;
          return {
            index,
            part,
          };
        }
      }
      break;
    }
    case "response.output_text.delta": {
      const part: TextPartDelta = {
        type: "text",
        text: event.delta,
      };
      const index = event.output_index;
      return {
        index,
        part,
      };
    }

    case "response.function_call_arguments.delta": {
      // Note: function name is added in "response.output_item.added"
      const part: ToolCallPartDelta = {
        type: "tool-call",
        args: event.delta,
      };
      const index = event.output_index;
      return {
        index,
        part,
      };
    }

    case "response.image_generation_call.partial_image": {
      const patchedEvent =
        event as OpenAIPatchedImageGenerationCallPartialImage;

      let width: number | undefined;
      let height: number | undefined;

      if (patchedEvent.size) {
        [width, height] = patchedEvent.size.split("x").map(Number);
      }

      const part: ImagePartDelta = {
        type: "image",
        image_data: patchedEvent.partial_image_b64,
        ...(patchedEvent.output_format && {
          mime_type: `image/${patchedEvent.output_format}`,
        }),
        ...(width && { width }),
        ...(height && { height }),
      };

      const index = event.output_index;
      return {
        index,
        part,
      };
    }

    case "response.reasoning_text.delta": {
      const part: ReasoningPartDelta = {
        type: "reasoning",
        text: event.delta,
      };
      const index = event.output_index;
      return {
        index,
        part,
      };
    }

    case "response.reasoning_summary_text.delta": {
      const part: ReasoningPartDelta = {
        type: "reasoning",
        summary: event.delta,
      };
      const index = event.output_index;
      return {
        index,
        part,
      };
    }
  }

  return null;
}

// MARK: To SDK Usage

function mapOpenAIUsage(usage: OpenAI.Responses.ResponseUsage): ModelUsage {
  const result: ModelUsage = {
    input_tokens: usage.input_tokens,
    output_tokens: usage.output_tokens,
  };
  return result;
}

function genidForMessageId() {
  return Math.random().toString(36).substring(2, 15);
}
