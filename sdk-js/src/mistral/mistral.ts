import { Mistral } from "@mistralai/mistralai";
import type * as MistralComponents from "@mistralai/mistralai/models/components/index.ts";
import {
  InvalidInputError,
  InvariantError,
  NotImplementedError,
  UnsupportedError,
} from "../errors.ts";
import type {
  LanguageModel,
  LanguageModelMetadata,
} from "../language-model.ts";
import { getCompatiblePartsWithoutSourceParts } from "../source-part.utils.ts";
import {
  guessDeltaIndex,
  looselyConvertPartToPartDelta,
} from "../stream.utils.ts";
import type {
  ContentDelta,
  ImagePart,
  LanguageModelInput,
  Message,
  ModelResponse,
  ModelUsage,
  Part,
  PartialModelResponse,
  ResponseFormatOption,
  TextPart,
  TextPartDelta,
  Tool,
  ToolCallPart,
  ToolChoiceOption,
} from "../types.ts";
import { calculateCost } from "../usage.utils.ts";

const PROVIDER = "mistral";

export interface MistralModelOptions {
  baseURL?: string;
  apiKey: string;
  modelId: string;
}

type MistralChatCompletionRequestMessage =
  MistralComponents.ChatCompletionRequest["messages"][number];

export class MistralModel implements LanguageModel {
  provider: string;
  modelId: string;
  metadata?: LanguageModelMetadata;

  #client: Mistral;

  constructor(options: MistralModelOptions, metadata?: LanguageModelMetadata) {
    this.provider = "mistral";
    this.modelId = options.modelId;
    if (metadata) this.metadata = metadata;

    this.#client = new Mistral({
      apiKey: options.apiKey,
      ...(options.baseURL && { serverURL: options.baseURL }),
    });
  }

  async generate(input: LanguageModelInput): Promise<ModelResponse> {
    const request = convertToMistralRequest(input, this.modelId);
    const response = await this.#client.chat.complete(request);

    const choice = response.choices?.[0];
    if (!choice) {
      throw new InvariantError(
        PROVIDER,
        "Response does not contain a valid choice",
      );
    }

    const content = mapMistralMessage(choice.message);
    const usage = mapMistralUsageInfo(response.usage);

    const result: ModelResponse = { content, usage };
    if (this.metadata?.pricing) {
      result.cost = calculateCost(usage, this.metadata.pricing);
    }
    return result;
  }

  async *stream(
    input: LanguageModelInput,
  ): AsyncGenerator<PartialModelResponse> {
    const request = convertToMistralRequest(input, this.modelId);
    const stream = await this.#client.chat.stream(request);

    const allContentDeltas: ContentDelta[] = [];

    for await (const chunk of stream) {
      const choice = chunk.data.choices[0];

      if (choice?.delta) {
        const incomingContentDeltas = mapMistralDelta(
          choice.delta,
          allContentDeltas,
        );

        allContentDeltas.push(...incomingContentDeltas);

        for (const delta of incomingContentDeltas) {
          yield { delta };
        }
      }

      if (chunk.data.usage) {
        const usage = mapMistralUsageInfo(chunk.data.usage);
        yield { usage };
      }
    }
  }
}

function convertToMistralRequest(
  input: LanguageModelInput,
  modelId: string,
): MistralComponents.ChatCompletionRequest {
  const {
    messages,
    system_prompt,
    max_tokens,
    temperature,
    top_p,
    presence_penalty,
    frequency_penalty,
    seed,
    response_format,
    tools,
    tool_choice,
    extra,
  } = input;

  return {
    model: modelId,
    messages: convertToMistralMessages(messages, system_prompt),
    ...(typeof max_tokens === "number" && { maxTokens: max_tokens }),
    ...(typeof temperature === "number" && { temperature }),
    ...(typeof top_p === "number" && { topP: top_p }),
    ...(typeof presence_penalty === "number" && {
      presencePenalty: presence_penalty,
    }),
    ...(typeof frequency_penalty === "number" && {
      frequencyPenalty: frequency_penalty,
    }),
    ...(typeof seed === "number" && { randomSeed: seed }),
    ...(tools && { tools: tools.map(convertToMistralTool) }),
    ...(tool_choice && { toolChoice: convertToMistralToolChoice(tool_choice) }),
    ...(response_format && {
      responseFormat: convertToMistralResponseFormat(response_format),
    }),
    ...extra,
  };
}

// MARK: To Provider Messages

function convertToMistralMessages(
  messages: Message[],
  systemPrompt: string | undefined,
): MistralComponents.ChatCompletionRequest["messages"] {
  const mistralMessages: MistralChatCompletionRequestMessage[] = [];

  if (systemPrompt) {
    mistralMessages.push({
      role: "system",
      content: [
        {
          type: "text",
          text: systemPrompt,
        },
      ],
    });
  }

  const convertedMistralMessages = messages.map(
    (
      message,
    ):
      | MistralChatCompletionRequestMessage
      | MistralChatCompletionRequestMessage[] => {
      switch (message.role) {
        case "user": {
          const messageParts = getCompatiblePartsWithoutSourceParts(
            message.content,
          );

          return {
            role: "user",
            content: messageParts.map((part) =>
              convertToMistralContentChunk(part),
            ),
          };
        }
        case "assistant": {
          const mistralAssistantMessage: Omit<
            MistralComponents.AssistantMessage,
            "content"
          > & {
            content: MistralComponents.ContentChunk[] | null;
          } = {
            role: "assistant",
            content: null,
          };

          const messageParts = getCompatiblePartsWithoutSourceParts(
            message.content,
          );

          messageParts.forEach((part) => {
            switch (part.type) {
              case "text":
              case "image": {
                mistralAssistantMessage.content =
                  mistralAssistantMessage.content ?? [];
                mistralAssistantMessage.content.push(
                  convertToMistralContentChunk(part),
                );
                break;
              }
              case "tool-call": {
                mistralAssistantMessage.toolCalls =
                  mistralAssistantMessage.toolCalls ?? [];
                mistralAssistantMessage.toolCalls.push(
                  convertToMistralToolCall(part),
                );
                break;
              }
              default: {
                throw new UnsupportedError(
                  PROVIDER,
                  `Cannot convert Part to Mistral assistant message for type ${part.type}`,
                );
              }
            }
          });
          return {
            ...mistralAssistantMessage,
            role: "assistant",
          };
        }
        case "tool": {
          return message.content.map((part) => {
            if (part.type !== "tool-result") {
              throw new InvalidInputError(
                "Tool messages must contain only tool result parts",
              );
            }
            const toolResultContent = getCompatiblePartsWithoutSourceParts(
              part.content,
            );
            return {
              role: "tool",
              toolCallId: part.tool_call_id,
              name: part.tool_name,
              content: toolResultContent.map(convertToMistralContentChunk),
            };
          });
        }
      }
    },
  );

  mistralMessages.push(...convertedMistralMessages.flat());

  return mistralMessages;
}

function convertToMistralContentChunk(
  part: Part,
): MistralComponents.ContentChunk {
  switch (part.type) {
    case "text":
      return convertToMistralTextChunk(part);
    case "image":
      return convertToMistralImageURLChunk(part);
    default:
      throw new UnsupportedError(
        PROVIDER,
        `Cannot convert Part to Mistral ContentChunk for type ${part.type}`,
      );
  }
}

function convertToMistralTextChunk(
  part: TextPart,
): MistralComponents.TextChunk & { type: "text" } {
  return {
    type: "text",
    text: part.text,
  };
}

function convertToMistralImageURLChunk(
  part: ImagePart,
): MistralComponents.ImageURLChunk & { type: "image_url" } {
  return {
    type: "image_url",
    imageUrl: {
      url: `data:${part.mime_type};base64,${part.image_data}`,
    },
  };
}

function convertToMistralToolCall(
  part: ToolCallPart,
): MistralComponents.ToolCall {
  return {
    type: "function",
    id: part.tool_call_id,
    function: {
      name: part.tool_name,
      arguments: JSON.stringify(part.args),
    },
  };
}

// MARK: To Provider Tools

function convertToMistralTool(tool: Tool): MistralComponents.Tool {
  return {
    type: "function",
    function: {
      name: tool.name,
      description: tool.description,
      parameters: tool.parameters,
      strict: true,
    },
  };
}

function convertToMistralToolChoice(
  toolChoice: ToolChoiceOption,
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

function convertToMistralResponseFormat(
  responseFormat: ResponseFormatOption,
): MistralComponents.ResponseFormat {
  switch (responseFormat.type) {
    case "json":
      if (responseFormat.schema) {
        return {
          type: "json_schema",
          jsonSchema: {
            name: responseFormat.name,
            description: responseFormat.description ?? null,
            schemaDefinition: responseFormat.schema,
            strict: true,
          },
        };
      }
      return { type: "json_object" };
    case "text":
      return { type: "text" };
  }
}

// MARK: To SDK Message

function mapMistralMessage(
  message: MistralComponents.AssistantMessage,
): Part[] {
  const parts: Part[] = [];

  if (typeof message.content === "string" && !!message.content) {
    parts.push({
      type: "text",
      text: message.content,
    });
  }

  if (Array.isArray(message.content)) {
    parts.push(...message.content.map(mapMistralContentChunk));
  }

  if (message.toolCalls) {
    parts.push(...message.toolCalls.map(mapMistralToolCall));
  }

  return parts;
}

function mapMistralContentChunk(chunk: MistralComponents.ContentChunk): Part {
  switch (chunk.type) {
    case "text": {
      return {
        type: "text",
        text: chunk.text,
      };
    }
    default:
      throw new NotImplementedError(
        PROVIDER,
        `Cannot map Mistral ContentChunk to Part for type ${chunk.type}`,
      );
  }
}

function mapMistralToolCall(
  toolCall: MistralComponents.ToolCall,
): ToolCallPart {
  if (!toolCall.id) {
    throw new InvariantError(
      PROVIDER,
      "Mistral ToolCall does not contain an id",
    );
  }
  const args =
    typeof toolCall.function.arguments === "string"
      ? (JSON.parse(toolCall.function.arguments) as Record<string, unknown>)
      : toolCall.function.arguments;

  return {
    type: "tool-call",
    tool_call_id: toolCall.id,
    tool_name: toolCall.function.name,
    args,
  };
}

// MARK: To SDK Delta

export function mapMistralDelta(
  deltaMessage: MistralComponents.DeltaMessage,
  existingContentDeltas: ContentDelta[],
): ContentDelta[] {
  const contentDeltas: ContentDelta[] = [];

  if (deltaMessage.content) {
    if (typeof deltaMessage.content === "string") {
      const part: TextPartDelta = {
        type: "text",
        text: deltaMessage.content,
      };
      const index = guessDeltaIndex(part, [
        ...existingContentDeltas,
        ...contentDeltas,
      ]);

      contentDeltas.push({ index, part });
    } else if (Array.isArray(deltaMessage.content)) {
      deltaMessage.content.forEach((chunk) => {
        const part = looselyConvertPartToPartDelta(
          mapMistralContentChunk(chunk),
        );
        const index = guessDeltaIndex(part, [
          ...existingContentDeltas,
          ...contentDeltas,
        ]);
        contentDeltas.push({ index, part });
      });
    }
  }

  if (deltaMessage.toolCalls) {
    deltaMessage.toolCalls.forEach((toolCall) => {
      const part = looselyConvertPartToPartDelta(mapMistralToolCall(toolCall));

      contentDeltas.push({
        index: guessDeltaIndex(
          part,
          [...existingContentDeltas, ...contentDeltas],
          toolCall.index,
        ),
        part,
      });
    });
  }

  return contentDeltas;
}

// MARK: To SDK Usage

function mapMistralUsageInfo(usage: MistralComponents.UsageInfo): ModelUsage {
  return {
    input_tokens: usage.promptTokens,
    output_tokens: usage.completionTokens,
  };
}
