import { Mistral } from "@mistralai/mistralai";
import * as MistralComponents from "@mistralai/mistralai/models/components/index.js";
import {
  InvalidValueError,
  ModelUnsupportedMessagePart,
  NotImplementedError,
} from "../errors/errors.js";
import {
  LanguageModel,
  LanguageModelMetadata,
} from "../models/language-model.js";
import {
  AssistantMessage,
  ContentDelta,
  LanguageModelInput,
  ModelResponse,
  ModelUsage,
  PartialModelResponse,
  Tool,
  ToolCallPart,
} from "../schema/index.js";
import { convertAudioPartsToTextParts } from "../utils/message.utils.js";
import {
  ContentDeltaAccumulator,
  InternalContentDelta,
} from "../utils/stream.utils.js";
import { calculateCost } from "../utils/usage.utils.js";
import { MistralModelOptions } from "./types.js";

export type MistralLanguageModelInput = LanguageModelInput & {
  extra?: Partial<MistralComponents.ChatCompletionRequest>;
};

export class MistralModel implements LanguageModel {
  public provider: string;
  public modelId: string;
  public metadata?: LanguageModelMetadata;

  private client: Mistral;

  constructor(
    public options: MistralModelOptions,
    metadata?: LanguageModelMetadata,
  ) {
    this.provider = "mistral";
    this.modelId = options.modelId;
    if (metadata) this.metadata = metadata;

    this.client = new Mistral({
      apiKey: options.apiKey,
      ...(options.baseURL && { serverURL: options.baseURL }),
    });
  }

  async generate(input: LanguageModelInput): Promise<ModelResponse> {
    const response = await this.client.chat.complete(
      convertToMistralParams(input, this.options),
    );

    if (!response.choices?.[0]) {
      throw new Error("no choices in response");
    }

    const choice = response.choices[0];

    const usage = mapMistralUsage(response.usage);

    return {
      content: mapMistralMessage(choice.message).content,
      usage,
      ...(this.metadata?.pricing && {
        cost: calculateCost(usage, this.metadata.pricing),
      }),
    };
  }

  async *stream(
    input: LanguageModelInput,
  ): AsyncGenerator<PartialModelResponse, ModelResponse> {
    const result = await this.client.chat.stream(
      convertToMistralParams(input, this.options),
    );

    let usage: ModelUsage | undefined;
    const accumulator = new ContentDeltaAccumulator();

    for await (const chunk of result) {
      const choice = chunk.data.choices[0];
      if (choice?.delta) {
        const incomingContentDeltas = mapMistralDelta(
          choice.delta,
          accumulator.deltas,
        );

        accumulator.addChunks(incomingContentDeltas);

        for (const delta of incomingContentDeltas) {
          yield { delta };
        }
      }

      if (chunk.data.usage) {
        usage = mapMistralUsage(chunk.data.usage);
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

export function convertToMistralParams(
  input: LanguageModelInput,
  options: MistralModelOptions,
): MistralComponents.ChatCompletionRequest {
  return {
    model: options.modelId,
    messages: convertToMistralMessages(input, options),
    ...(input.tools && {
      tools: input.tools.map(convertToMistralTool),
    }),
    ...(input.toolChoice && {
      toolChoice: convertToMistralToolChoice(input.toolChoice),
    }),
    ...(input.responseFormat && {
      responseFormat: convertToMistralResponseFormat(input.responseFormat),
    }),
    ...convertToMistralSamplingParams(input),
    ...input.extra,
  };
}

export function convertToMistralMessages(
  input: Pick<MistralLanguageModelInput, "messages" | "systemPrompt">,
  options: MistralModelOptions,
): MistralComponents.ChatCompletionRequest["messages"] {
  const mistralMessages: MistralComponents.ChatCompletionRequest["messages"] =
    [];

  let messages = input.messages;

  if (options.convertAudioPartsToTextParts) {
    messages = messages.map(convertAudioPartsToTextParts);
  }

  if (input.systemPrompt) {
    mistralMessages.push({
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
    switch (message.role) {
      case "assistant": {
        const mistralMessageParam: MistralComponents.AssistantMessage = {
          role: "assistant",
          content: null,
        };
        message.content.forEach((part) => {
          switch (part.type) {
            case "text": {
              mistralMessageParam.content = [
                ...(mistralMessageParam.content || []),
                {
                  type: "text",
                  text: part.text,
                },
              ] as Array<MistralComponents.ContentChunk>;
              break;
            }
            case "tool-call": {
              mistralMessageParam.toolCalls =
                mistralMessageParam.toolCalls || [];
              mistralMessageParam.toolCalls.push({
                type: "function",
                id: part.toolCallId,
                function: {
                  name: part.toolName,
                  arguments: JSON.stringify(part.args),
                },
              });
              break;
            }
            case "audio": {
              throw new ModelUnsupportedMessagePart("mistral", "audio");
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
        mistralMessages.push({
          ...mistralMessageParam,
          role: "assistant",
        });
        break;
      }
      case "tool": {
        message.content.forEach((toolResult) => {
          mistralMessages.push({
            role: "tool",
            content: JSON.stringify(toolResult.result),
            toolCallId: toolResult.toolCallId,
            name: toolResult.toolName,
          });
        });
        break;
      }
      case "user": {
        const contentParts = message.content;
        mistralMessages.push({
          role: "user",
          content: contentParts.map((part): MistralComponents.ContentChunk => {
            switch (part.type) {
              case "text": {
                return {
                  type: "text",
                  text: part.text,
                };
              }
              case "image": {
                return {
                  type: "image_url",
                  imageUrl: {
                    url: `data:${part.mimeType};base64,${part.imageData}`,
                  },
                };
              }
              case "audio": {
                throw new ModelUnsupportedMessagePart("mistral", "audio");
              }
              default: {
                const exhaustiveCheck: never = part;
                throw new InvalidValueError(
                  "part.type",
                  (exhaustiveCheck as { type: string }).type,
                );
              }
            }
          }),
        });
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

  return mistralMessages;
}

export function convertToMistralSamplingParams(
  input: Partial<LanguageModelInput>,
) {
  return {
    ...(typeof input.maxTokens === "number" && { maxTokens: input.maxTokens }),
    ...(typeof input.temperature === "number" && {
      temperature: input.temperature,
    }),
    ...(typeof input.topP === "number" && { topP: input.topP }),
    ...(typeof input.presencePenalty === "number" && {
      presencePenalty: input.presencePenalty,
    }),
    ...(typeof input.frequencyPenalty === "number" && {
      frequencyPenalty: input.frequencyPenalty,
    }),
    ...(typeof input.seed === "number" && { randomSeed: input.seed }),
  } satisfies Partial<MistralComponents.ChatCompletionRequest>;
}

export function convertToMistralToolChoice(
  toolChoice: NonNullable<LanguageModelInput["toolChoice"]>,
): MistralComponents.ToolChoice | MistralComponents.ToolChoiceEnum {
  switch (toolChoice.type) {
    case "tool": {
      return {
        type: "function",
        function: {
          name: toolChoice.toolName,
        },
      };
    }
    default:
      return toolChoice.type;
  }
}

export function convertToMistralTool(tool: Tool): MistralComponents.Tool {
  return {
    type: "function",
    function: {
      name: tool.name,
      description: tool.description,
      parameters: tool.parameters || {},
    },
  };
}

export function convertToMistralResponseFormat(
  responseFormat: NonNullable<LanguageModelInput["responseFormat"]>,
): MistralComponents.ResponseFormat {
  switch (responseFormat.type) {
    case "json":
      return { type: "json_object" };
    case "text":
      return { type: "text" };
    default: {
      const exhaustiveCheck: never = responseFormat;
      throw new InvalidValueError(
        "responseFormat.type",
        (exhaustiveCheck as { type: string }).type,
      );
    }
  }
}

export function mapMistralMessage(
  message: MistralComponents.AssistantMessage,
): AssistantMessage {
  const content: AssistantMessage["content"] = [];

  if (typeof message.content === "string") {
    content.push({
      type: "text",
      text: message.content,
    });
  }
  if (Array.isArray(message.content)) {
    message.content.forEach((chunk) => {
      switch (chunk.type) {
        case "text":
          content.push({
            type: "text",
            text: chunk.text,
          });
          break;
        case "image_url":
        case "reference":
          throw new NotImplementedError("message.part", chunk.type);
        default: {
          const exhaustiveCheck: never = chunk;
          throw new NotImplementedError(
            "message.part",
            (exhaustiveCheck as { type: string }).type,
          );
        }
      }
    });
  }

  if (message.toolCalls) {
    message.toolCalls.forEach((toolCall) => {
      if (!toolCall.id) {
        throw new Error("toolCall.id is missing");
      }
      let args: ToolCallPart["args"] = null;

      if (typeof toolCall.function.arguments === "string") {
        args = JSON.parse(toolCall.function.arguments) as {
          [key: string]: unknown;
        };
      }
      if (typeof toolCall.function.arguments === "object") {
        args = toolCall.function.arguments;
      }

      content.push({
        type: "tool-call",
        toolCallId: toolCall.id,
        toolName: toolCall.function.name,
        args,
      });
    });
  }

  return {
    role: "assistant",
    content,
  };
}

export function mapMistralDelta(
  delta: MistralComponents.DeltaMessage,
  existingContentDeltas: InternalContentDelta[],
): ContentDelta[] {
  const contentDeltas: ContentDelta[] = [];
  if (delta.content && typeof delta.content === "string") {
    const existingDelta = existingContentDeltas.find(
      (delta) => delta.part.type === "text",
    );
    contentDeltas.push({
      index: existingDelta ? existingDelta.index : contentDeltas.length,
      part: { type: "text", text: delta.content },
    });
  }
  if (Array.isArray(delta.content)) {
    delta.content.forEach((chunk) => {
      switch (chunk.type) {
        case "text": {
          const existingDelta = existingContentDeltas.find(
            (delta) => delta.part.type === "text",
          );
          contentDeltas.push({
            index: existingDelta ? existingDelta.index : contentDeltas.length,
            part: { type: "text", text: chunk.text },
          });
          break;
        }
        case "image_url":
        case "reference":
          throw new NotImplementedError("message.part", chunk.type);
        default: {
          const exhaustiveCheck: never = chunk;
          throw new NotImplementedError(
            "message.part",
            (exhaustiveCheck as { type: string }).type,
          );
        }
      }
    });
  }
  if (delta.toolCalls) {
    delta.toolCalls.forEach((toolCall) => {
      // This is unsafe because it leads to mismatched tool calls
      // but from the Mistral API, it seems like the tool calls are
      // always streamed at once
      let args: string;
      if (typeof toolCall.function.arguments === "string") {
        args = toolCall.function.arguments;
      } else {
        args = JSON.stringify(toolCall.function.arguments);
      }
      contentDeltas.push({
        index: contentDeltas.length,
        part: {
          type: "tool-call",
          ...(toolCall.id && { toolCallId: toolCall.id }),
          ...(toolCall.function.name && { toolName: toolCall.function.name }),
          ...(args && { args }),
        },
      });
    });
  }
  return contentDeltas;
}

export function mapMistralUsage(
  usage: MistralComponents.UsageInfo,
): ModelUsage {
  return {
    inputTokens: usage.promptTokens,
    outputTokens: usage.completionTokens,
  };
}
