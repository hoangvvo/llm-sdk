import { Mistral } from "@mistralai/mistralai";
import type * as MistralComponents from "@mistralai/mistralai/models/components/index.js";
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
  ModelResponse,
  ModelUsage,
  PartialModelResponse,
  Tool,
  ToolCallPart,
  ToolCallPartDelta,
} from "../types.js";
import { convertAudioPartsToTextParts } from "../utils/message.utils.js";
import type { InternalContentDelta } from "../utils/stream.utils.js";
import { ContentDeltaAccumulator } from "../utils/stream.utils.js";
import { calculateCost } from "../utils/usage.utils.js";
import type { MistralModelOptions } from "./types.js";

export type MistralLanguageModelInput = LanguageModelInput & {
  extra?: Partial<MistralComponents.ChatCompletionRequest>;
};

export class MistralModel extends LanguageModel {
  public provider: string;
  public modelId: string;
  public metadata?: LanguageModelMetadata;

  private client: Mistral;

  constructor(
    public options: MistralModelOptions,
    metadata?: LanguageModelMetadata,
  ) {
    super();
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

    const result: ModelResponse = {
      content: mapMistralMessage(choice.message).content,
      usage,
    };
    if (this.metadata?.pricing) {
      result.cost = calculateCost(usage, this.metadata.pricing);
    }
    return result;
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

    const finalResult: ModelResponse = {
      content: accumulator.computeContent(),
    };
    if (usage) {
      finalResult.usage = usage;
      if (this.metadata?.pricing) {
        finalResult.cost = calculateCost(usage, this.metadata.pricing);
      }
    }

    return finalResult;
  }
}

export function convertToMistralParams(
  input: LanguageModelInput,
  options: MistralModelOptions,
): MistralComponents.ChatCompletionRequest {
  const params: MistralComponents.ChatCompletionRequest = {
    model: options.modelId,
    messages: convertToMistralMessages(input, options),
    ...convertToMistralSamplingParams(input),
    ...input.extra,
  };
  if (input.tools) {
    params.tools = input.tools.map(convertToMistralTool);
  }
  if (input.tool_choice) {
    params.toolChoice = convertToMistralToolChoice(input.tool_choice);
  }
  if (input.response_format) {
    params.responseFormat = convertToMistralResponseFormat(
      input.response_format,
    );
  }
  return params;
}

export function convertToMistralMessages(
  input: Pick<MistralLanguageModelInput, "messages" | "system_prompt">,
  options: MistralModelOptions,
): MistralComponents.ChatCompletionRequest["messages"] {
  const mistralMessages: MistralComponents.ChatCompletionRequest["messages"] =
    [];

  let messages = input.messages;

  if (options.convertAudioPartsToTextParts) {
    messages = messages.map(convertAudioPartsToTextParts);
  }

  if (input.system_prompt) {
    mistralMessages.push({
      role: "system",
      content: [
        {
          type: "text",
          text: input.system_prompt,
        },
      ],
    });
  }

  messages.forEach((message) => {
    switch (message.role) {
      case "assistant": {
        const mistralMessageParam: Omit<
          MistralComponents.AssistantMessage,
          "content"
        > & {
          content: MistralComponents.ContentChunk[] | null;
        } = {
          role: "assistant",
          content: null,
        };
        message.content.forEach((part) => {
          switch (part.type) {
            case "text": {
              mistralMessageParam.content = mistralMessageParam.content || [];
              mistralMessageParam.content.push({
                type: "text",
                text: part.text,
              });
              break;
            }
            case "tool-call": {
              mistralMessageParam.toolCalls =
                mistralMessageParam.toolCalls || [];
              mistralMessageParam.toolCalls.push({
                type: "function",
                id: part.tool_call_id,
                function: {
                  name: part.tool_name,
                  arguments: JSON.stringify(part.args),
                },
              });
              break;
            }
            case "image":
            case "tool-result":
            case "audio": {
              throw new ModelUnsupportedMessagePart("mistral", message, part);
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
            toolCallId: toolResult.tool_call_id,
            name: toolResult.tool_name,
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
                    url: `data:${part.mime_type};base64,${part.image_data}`,
                  },
                };
              }
              case "tool-call":
              case "tool-result":
              case "audio": {
                throw new ModelUnsupportedMessagePart("mistral", message, part);
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
): Partial<MistralComponents.ChatCompletionRequest> {
  const sampling: Partial<MistralComponents.ChatCompletionRequest> = {};
  if (typeof input.max_tokens === "number") {
    sampling.maxTokens = input.max_tokens;
  }
  if (typeof input.temperature === "number") {
    sampling.temperature = input.temperature;
  }
  if (typeof input.top_p === "number") {
    sampling.topP = input.top_p;
  }
  if (typeof input.presence_penalty === "number") {
    sampling.presencePenalty = input.presence_penalty;
  }
  if (typeof input.frequency_penalty === "number") {
    sampling.frequencyPenalty = input.frequency_penalty;
  }
  if (typeof input.seed === "number") {
    sampling.randomSeed = input.seed;
  }
  return sampling;
}

export function convertToMistralToolChoice(
  toolChoice: NonNullable<LanguageModelInput["tool_choice"]>,
): MistralComponents.ToolChoice | MistralComponents.ToolChoiceEnum {
  switch (toolChoice.type) {
    case "tool": {
      return {
        type: "function",
        function: {
          name: toolChoice.tool_name,
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
  responseFormat: NonNullable<LanguageModelInput["response_format"]>,
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
        tool_call_id: toolCall.id,
        tool_name: toolCall.function.name,
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
      const toolCallPart: ToolCallPartDelta = {
        type: "tool-call",
        tool_name: toolCall.function.name,
        args,
      };
      if (toolCall.id) {
        toolCallPart.tool_call_id = toolCall.id;
      }
      contentDeltas.push({
        index: contentDeltas.length,
        part: toolCallPart,
      });
    });
  }
  return contentDeltas;
}

export function mapMistralUsage(
  usage: MistralComponents.UsageInfo,
): ModelUsage {
  return {
    input_tokens: usage.promptTokens,
    output_tokens: usage.completionTokens,
  };
}
