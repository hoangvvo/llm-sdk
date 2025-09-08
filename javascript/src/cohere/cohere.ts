import { Cohere, CohereClientV2 } from "cohere-ai";
import {
  InvalidValueError,
  ModelUnsupportedMessagePart,
} from "../errors/errors.js";
import {
  LanguageModel,
  LanguageModelMetadata,
} from "../models/language-model.js";
import { traceLanguageModel } from "../models/opentelemetry.js";
import {
  AssistantMessage,
  ContentDelta,
  LanguageModelInput,
  ModelResponse,
  ModelUsage,
  Part,
  PartialModelResponse,
  Tool,
  ToolCallPart,
} from "../schema/index.js";
import { convertAudioPartsToTextParts } from "../utils/message.utils.js";
import { ContentDeltaAccumulator } from "../utils/stream.utils.js";
import { calculateCost } from "../utils/usage.utils.js";
import { CohereModelOptions } from "./types.js";

export type CohereLanguageModelInput = LanguageModelInput & {
  extra?: Partial<Cohere.V2ChatRequest>;
};

export class CohereModel implements LanguageModel {
  public provider: string;
  public modelId: string;
  public metadata?: LanguageModelMetadata;

  private cohere: CohereClientV2;

  constructor(
    public options: CohereModelOptions,
    metadata?: LanguageModelMetadata,
  ) {
    this.provider = "cohere";
    this.modelId = options.modelId;
    if (metadata) this.metadata = metadata;

    this.cohere = new CohereClientV2({
      token: options.apiKey,
    });

    traceLanguageModel(this);
  }

  async generate(input: LanguageModelInput): Promise<ModelResponse> {
    const response = await this.cohere.chat(
      convertToCohereParams(input, this.options),
    );

    const usage = response.usage ? mapCohereUsage(response.usage) : undefined;

    return {
      content: mapCohereMessage(response.message).content,
      ...(usage && { usage }),
      ...(this.metadata?.pricing &&
        usage && {
          cost: calculateCost(usage, this.metadata.pricing),
        }),
    };
  }

  async *stream(
    input: LanguageModelInput,
  ): AsyncGenerator<PartialModelResponse, ModelResponse> {
    const stream = await this.cohere.chatStream(
      convertToCohereParams(input, this.options),
    );

    let usage: ModelUsage | undefined;

    const accumulator = new ContentDeltaAccumulator();

    for await (const event of stream) {
      switch (event.type) {
        case "content-start":
        case "content-delta":
        case "tool-call-start":
        case "tool-call-delta": {
          const delta = mapCohereContentDelta(event);
          if (delta) {
            yield { delta };
            accumulator.addChunks([delta]);
          }
          break;
        }
        case "message-end": {
          if (event.delta?.usage) {
            usage = mapCohereUsage(event.delta.usage);
          }
          break;
        }
      }
    }

    return {
      content: accumulator.computeContent(),
      ...(usage && { usage }),
      ...(this.metadata?.pricing &&
        usage && {
          cost: calculateCost(usage, this.metadata.pricing),
        }),
    };
  }
}

export function convertToCohereParams(
  input: LanguageModelInput,
  options: CohereModelOptions,
): Cohere.V2ChatRequest {
  const response_format = convertToCohereResponseFormat(input.responseFormat);
  const samplingParams = convertToCohereSamplingParams(input);

  return {
    model: options.modelId,
    messages: convertToCohereMessages(input, options),
    ...(input.tools && { tools: input.tools.map(convertToCohereTool) }),
    ...samplingParams,
    ...(response_format && {
      responseFormat: response_format,
    }),
    ...input.extra,
  };
}

export function convertToCohereMessages(
  input: Pick<CohereLanguageModelInput, "messages" | "systemPrompt">,
  options: CohereModelOptions,
): Cohere.ChatMessageV2[] {
  const cohereMessages: Cohere.ChatMessageV2[] = [];

  let messages = input.messages;
  if (options.convertAudioPartsToTextParts) {
    messages = messages.map(convertAudioPartsToTextParts);
  }

  if (input.systemPrompt) {
    cohereMessages.push({
      role: "system",
      content: [
        {
          type: "text",
          text: input.systemPrompt,
        },
      ],
    });
  }

  messages.forEach((message) => {
    const { content, toolCalls, toolResults } = convertToCohereMessageParam(
      message.content,
    );
    switch (message.role) {
      case "user": {
        if (!content) {
          throw new Error("User message must have contents");
        }
        cohereMessages.push({
          role: "user",
          content,
        });
        break;
      }
      case "assistant": {
        cohereMessages.push({
          role: "assistant",
          ...(content && { content }),
          ...(toolCalls && { toolCalls }),
        });
        break;
      }
      case "tool": {
        if (toolResults) {
          for (const toolResult of toolResults) {
            cohereMessages.push({
              role: "tool",
              ...toolResult,
            });
          }
        } else {
          throw new Error("Tool message must have tool results");
        }
        break;
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

  return cohereMessages;
}

export function convertToCohereMessageParam(parts: Part[]): {
  content?: Cohere.Content[];
  toolCalls?: Cohere.ToolCallV2[];
  toolResults?: Cohere.ToolMessageV2[];
} {
  let content: Cohere.Content[] | undefined;
  let toolCalls: Cohere.ToolCallV2[] | undefined;
  let toolResults: Cohere.ToolMessageV2[] | undefined;

  parts.forEach((part) => {
    switch (part.type) {
      case "text": {
        content = content ?? [];
        content.push({
          type: "text",
          text: part.text,
        });
        break;
      }
      case "audio":
      case "image":
        throw new ModelUnsupportedMessagePart("cohere", part.type);
      case "tool-call": {
        toolCalls = toolCalls ?? [];
        toolCalls.push({
          id: part.toolCallId,
          type: "function",
          function: {
            name: part.toolName,
            ...(part.args && {
              arguments: JSON.stringify(part.args),
            }),
          },
        });
        break;
      }
      case "tool-result": {
        toolResults = toolResults ?? [];
        toolResults.push({
          toolCallId: part.toolCallId,
          content: [
            {
              type: "text",
              text: JSON.stringify(part.result),
            },
          ],
        });
        break;
      }
      default: {
        const exhaustiveCheck: never = part;
        throw new InvalidValueError(
          "part.type",
          (exhaustiveCheck as { type: string }).type,
        );
      }
    }
  });

  return {
    ...(content && { content }),
    ...(toolCalls && { toolCalls }),
    ...(toolResults && { toolResults }),
  };
}

export function convertToCohereSamplingParams(
  input: Partial<LanguageModelInput>,
) {
  return {
    ...(typeof input.maxTokens === "number" && { maxTokens: input.maxTokens }),
    ...(typeof input.temperature === "number" && {
      temperature: input.temperature,
    }),
    ...(typeof input.topP === "number" && { p: input.topP }),
    ...(typeof input.presencePenalty === "number" && {
      presencePenalty: input.presencePenalty,
    }),
    ...(typeof input.frequencyPenalty === "number" && {
      frequencyPenalty: input.frequencyPenalty,
    }),
    ...(typeof input.seed === "number" && { seed: input.seed }),
  } satisfies Partial<Cohere.V2ChatRequest>;
}

export function convertToCohereTool(tool: Tool): Cohere.ToolV2 {
  return {
    type: "function",
    function: {
      name: tool.name,
      description: tool.description,
      ...(tool.parameters && {
        parameters: convertToCohereSchema(tool.parameters),
      }),
    },
  };
}

export function convertToCohereSchema(
  schema: Record<string, unknown>,
): Record<string, unknown> {
  const newSchema = {
    ...schema,
    ...(!!schema["properties"] && {
      properties: convertToCohereSchema(
        schema["properties"] as Record<string, unknown>,
      ),
    }),
    ...(!!schema["items"] && {
      items: convertToCohereSchema(schema["items"] as Record<string, unknown>),
    }),
    ...(!!schema["anyOf"] && {
      anyOf: (schema["anyOf"] as Record<string, unknown>[]).map(
        convertToCohereSchema,
      ),
    }),
    ...(!!schema["allOf"] && {
      allOf: (schema["allOf"] as Record<string, unknown>[]).map(
        convertToCohereSchema,
      ),
    }),
    ...(!!schema["oneOf"] && {
      oneOf: (schema["oneOf"] as Record<string, unknown>[]).map(
        convertToCohereSchema,
      ),
    }),
  } as Record<string, unknown>;
  // additionalProperties is known to not be supported by Cohere
  if ("additionalProperties" in newSchema) {
    delete newSchema["additionalProperties"];
  }
  return newSchema;
}

export function convertToCohereResponseFormat(
  responseFormat: LanguageModelInput["responseFormat"],
): Cohere.ResponseFormatV2 | undefined {
  if (!responseFormat) return undefined;

  switch (responseFormat.type) {
    case "json":
      return {
        type: "json_object",
        ...(responseFormat.schema && {
          jsonSchema: convertToCohereSchema(responseFormat.schema),
        }),
      };
    case "text":
      return {
        type: "text",
      };
    default: {
      const exhaustiveCheck: never = responseFormat;
      throw new InvalidValueError(
        "responseFormat.type",
        (exhaustiveCheck as { type: string }).type,
      );
    }
  }
}

export function mapCohereMessage(
  messageResponse: Cohere.AssistantMessageResponse,
): AssistantMessage {
  const content: AssistantMessage["content"] = [];

  if (messageResponse.content) {
    messageResponse.content.forEach((contentBlock) => {
      content.push({
        type: "text",
        text: contentBlock.text,
      });
    });
  }

  if (messageResponse.toolCalls) {
    messageResponse.toolCalls.forEach((toolCall) => {
      content.push(mapCohereToolCall(toolCall));
    });
  }

  return {
    role: "assistant",
    content,
  };
}

export function mapCohereToolCall(toolCall: Cohere.ToolCallV2): ToolCallPart {
  if (!toolCall.id) {
    throw new Error(`Tool call is missing an ID`);
  }
  const functionName = toolCall.function?.name;
  if (!functionName) {
    throw new Error(`Tool call is missing a function name`);
  }
  const functionArguments = toolCall.function?.arguments;
  return {
    type: "tool-call",
    toolCallId: toolCall.id,
    toolName: functionName,
    args: functionArguments
      ? (JSON.parse(functionArguments) as Record<string, unknown>)
      : null,
  };
}

export function mapCohereContentDelta(
  event:
    | Cohere.StreamedChatResponseV2.ContentDelta
    | Cohere.StreamedChatResponseV2.ContentStart
    | Cohere.StreamedChatResponseV2.ToolCallDelta
    | Cohere.StreamedChatResponseV2.ToolCallStart,
): ContentDelta | undefined {
  if (typeof event.index !== "number") {
    throw new Error("Delta event is missing index");
  }
  switch (event.type) {
    case "content-start":
    case "content-delta": {
      const text = event.delta?.message?.content?.text;
      if (!text) {
        return undefined;
      }
      return {
        index: event.index,
        part: {
          type: "text",
          text,
        },
      };
    }
    case "tool-call-start": {
      const toolCall =
        event.delta?.toolCall ||
        // sdk has wrong type
        (
          event.delta as
            | {
                message?: {
                  tool_calls: Cohere.ChatToolCallStartEventDeltaToolCall;
                };
              }
            | undefined
        )?.message?.tool_calls;
      return {
        index: event.index,
        part: {
          type: "tool-call",
          ...(toolCall?.id && {
            toolCallId: toolCall.id,
          }),
          ...(toolCall?.function?.name && {
            toolName: toolCall.function.name,
          }),
          ...(toolCall?.function?.arguments && {
            args: toolCall.function.arguments,
          }),
        },
      };
    }
    case "tool-call-delta": {
      const toolCall =
        event.delta?.toolCall ||
        (
          event.delta as
            | {
                message?: {
                  tool_calls: Cohere.ChatToolCallDeltaEventDeltaToolCall;
                };
              }
            | undefined
        )?.message?.tool_calls;

      return {
        index: event.index,
        part: {
          type: "tool-call",
          ...(toolCall?.function?.arguments && {
            args: toolCall.function.arguments,
          }),
        },
      };
    }
  }
}

export function mapCohereUsage(usage: Cohere.Usage): ModelUsage | undefined {
  if (!usage.tokens?.inputTokens && !usage.tokens?.outputTokens) {
    return undefined;
  }
  return {
    inputTokens: usage.tokens.inputTokens || 0,
    outputTokens: usage.tokens.outputTokens || 0,
  };
}
