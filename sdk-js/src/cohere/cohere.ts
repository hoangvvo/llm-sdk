import { Cohere, CohereClientV2 } from "cohere-ai";
import {
  InvalidInputError,
  InvariantError,
  UnsupportedError,
} from "../errors.ts";
import type { LanguageModelMetadata } from "../language-model.ts";
import { LanguageModel } from "../language-model.ts";
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
  Tool,
  ToolCallPart,
  ToolCallPartDelta,
  ToolChoiceOption,
} from "../types.ts";
import { calculateCost } from "../usage.utils.ts";

const PROVIDER = "cohere";

export interface CohereModelOptions {
  apiKey: string;
  modelId: string;
}

export class CohereModel extends LanguageModel {
  public provider: string;
  public modelId: string;
  public metadata?: LanguageModelMetadata;

  private cohere: CohereClientV2;

  constructor(options: CohereModelOptions, metadata?: LanguageModelMetadata) {
    super();
    this.provider = PROVIDER;
    this.modelId = options.modelId;
    if (metadata) this.metadata = metadata;
    this.cohere = new CohereClientV2({
      token: options.apiKey,
    });
  }

  async generate(input: LanguageModelInput): Promise<ModelResponse> {
    const request = convertToCohereChatRequest(input, this.modelId);
    const response = await this.cohere.chat(request);

    const content = mapCohereMessageResponse(response.message);
    const result: ModelResponse = { content };

    if (response.usage) {
      result.usage = mapCohereUsage(response.usage);
      if (this.metadata?.pricing) {
        result.cost = calculateCost(result.usage, this.metadata.pricing);
      }
    }

    return result;
  }

  async *stream(
    input: LanguageModelInput,
  ): AsyncGenerator<PartialModelResponse> {
    const request = convertToCohereChatRequest(input, this.modelId);
    const stream = await this.cohere.chatStream(request);

    for await (const event of stream) {
      switch (event.type) {
        case "content-start":
        case "content-delta": {
          const incomingContentDelta = mapCohereStreamedContent(event);
          if (incomingContentDelta) {
            yield { delta: incomingContentDelta };
          }
          break;
        }
        case "tool-call-start": {
          const incomingContentDelta = mapCohereToolCallStartEvent(event);
          if (incomingContentDelta) {
            yield { delta: incomingContentDelta };
          }
          break;
        }
        case "tool-call-delta": {
          const incomingContentDelta = mapCohereToolCallDeltaEvent(event);
          if (incomingContentDelta) {
            yield { delta: incomingContentDelta };
          }
        }
      }
    }
  }
}

function convertToCohereChatRequest(
  input: LanguageModelInput,
  modelId: string,
): Cohere.V2ChatRequest {
  const {
    messages,
    system_prompt,
    max_tokens,
    temperature,
    top_p,
    top_k,
    presence_penalty,
    frequency_penalty,
    seed,
    tools,
    tool_choice,
    response_format,
    extra,
  } = input;

  const toolChoice = tool_choice
    ? convertToCohereToolChoice(tool_choice)
    : undefined;

  return {
    model: modelId,
    messages: convertToCohereMessages(messages, system_prompt),
    ...(typeof max_tokens === "number" && { maxTokens: max_tokens }),
    ...(typeof temperature === "number" && { temperature }),
    ...(typeof top_p === "number" && { p: top_p }),
    ...(typeof top_k === "number" && { k: top_k }),
    ...(typeof presence_penalty === "number" && {
      presencePenalty: presence_penalty,
    }),
    ...(typeof frequency_penalty === "number" && {
      frequencyPenalty: frequency_penalty,
    }),
    ...(typeof seed === "number" && { seed }),
    ...(tools && {
      tools: tools.map(convertToCohereTool),
      strictTools: true,
    }),
    ...(toolChoice && { toolChoice }),

    ...(response_format && {
      responseFormat: convertToCohereResponseFormat(response_format),
    }),
    ...extra,
  };
}

// MARK: To Provider Messages

function convertToCohereMessages(
  messages: Message[],
  systemPrompt: string | undefined,
): Cohere.ChatMessageV2[] {
  const cohereMessages: Cohere.ChatMessageV2[] = [];

  if (systemPrompt) {
    cohereMessages.push({
      role: "system",
      content: systemPrompt,
    });
  }

  messages.forEach((message) => {
    switch (message.role) {
      case "user": {
        cohereMessages.push({
          role: "user",
          content: message.content.map(convertToCohereContent),
        });
        break;
      }

      case "assistant": {
        const cohereAssistantMessage: Omit<
          Cohere.ChatMessageV2.Assistant,
          "content"
        > & {
          content?: Cohere.AssistantMessageContentItem[];
        } = {
          role: "assistant",
        };
        message.content.forEach((part) => {
          switch (part.type) {
            case "text": {
              cohereAssistantMessage.content =
                cohereAssistantMessage.content ?? [];
              cohereAssistantMessage.content.push(
                convertToCohereTextContent(part),
              );
              break;
            }
            case "tool-call": {
              cohereAssistantMessage.toolCalls =
                cohereAssistantMessage.toolCalls ?? [];
              cohereAssistantMessage.toolCalls.push(
                convertToCohereToolCall(part),
              );
              break;
            }
            default:
              throw new UnsupportedError(
                PROVIDER,
                `Cannot convert Part to Cohere assistant message for type ${part.type}`,
              );
          }
        });
        cohereMessages.push(cohereAssistantMessage);
        break;
      }

      case "tool": {
        message.content.forEach((part) => {
          if (part.type !== "tool-result") {
            throw new InvalidInputError(
              "Tool messages must contain only tool result parts",
            );
          }

          cohereMessages.push({
            role: "tool",
            toolCallId: part.tool_call_id,
            content: part.content.map(convertToCohereToolMessageContent),
          });
        });
        break;
      }
    }
  });

  return cohereMessages;
}

function convertToCohereContent(part: Part): Cohere.Content {
  switch (part.type) {
    case "text":
      return convertToCohereTextContent(part);
    case "image":
      return convertToCohereImageContent(part);
    default:
      throw new UnsupportedError(
        PROVIDER,
        `Cannot convert part to Cohere content for type ${part.type}`,
      );
  }
}

function convertToCohereTextContent(textPart: TextPart): Cohere.Content.Text {
  return {
    type: "text",
    text: textPart.text,
  };
}

function convertToCohereImageContent(
  imagePart: ImagePart,
): Cohere.Content.ImageUrl {
  return {
    type: "image_url",
    imageUrl: {
      url: `data:${imagePart.mime_type};base64,${imagePart.image_data}`,
    },
  };
}

function convertToCohereToolCall(
  toolCallPart: ToolCallPart,
): Cohere.ToolCallV2 {
  return {
    type: "function",
    id: toolCallPart.tool_call_id,
    function: {
      name: toolCallPart.tool_name,
      arguments: JSON.stringify(toolCallPart.args),
    },
  };
}

function convertToCohereToolMessageContent(part: Part): Cohere.ToolContent {
  switch (part.type) {
    case "text":
      return convertToCohereTextContent(part);
    default:
      throw new UnsupportedError(
        PROVIDER,
        `Cannot convert part to Cohere ToolContent for type ${part.type}`,
      );
  }
}

// MARK: To Provider Tools

function convertToCohereTool(tool: Tool): Cohere.ToolV2 {
  return {
    type: "function",
    function: {
      name: tool.name,
      description: tool.description,
      parameters: tool.parameters,
    },
  };
}

function convertToCohereToolChoice(
  toolChoice: ToolChoiceOption,
): Cohere.V2ChatRequestToolChoice | undefined {
  switch (toolChoice.type) {
    case "auto": {
      return undefined;
    }
    case "none": {
      return Cohere.V2ChatRequestToolChoice.None;
    }
    case "required": {
      return Cohere.V2ChatRequestToolChoice.Required;
    }
    default:
      throw new UnsupportedError(
        PROVIDER,
        `Cannot convert tool choice to Cohere tool choice for type ${toolChoice.type}`,
      );
  }
}

// MARK: To Provider Response Format

function convertToCohereResponseFormat(
  responseFormat: ResponseFormatOption,
): Cohere.ResponseFormatV2 {
  switch (responseFormat.type) {
    case "json": {
      return {
        type: "json_object",
        ...(responseFormat.schema && {
          jsonSchema: responseFormat.schema,
        }),
      };
    }
    case "text": {
      return { type: "text" };
    }
  }
}

// MARK: To SDK Message

function mapCohereMessageResponse(
  messageResponse: Cohere.AssistantMessageResponse,
): Part[] {
  const parts: Part[] = [];

  if (messageResponse.content) {
    messageResponse.content.forEach((contentItem) => {
      parts.push(mapCohereTextContent(contentItem));
    });
  }

  if (messageResponse.toolCalls) {
    messageResponse.toolCalls.forEach((toolCall) => {
      parts.push(mapCohereToolCall(toolCall));
    });
  }

  return parts;
}

function mapCohereTextContent(content: Cohere.Content.Text): Part {
  return {
    type: "text",
    text: content.text,
  };
}

function mapCohereToolCall(toolCall: Cohere.ToolCallV2): ToolCallPart {
  if (!toolCall.id) {
    throw new InvariantError(PROVIDER, "Cohere tool call is missing an ID");
  }
  const functionName = toolCall.function?.name;
  if (!functionName) {
    throw new InvariantError(
      PROVIDER,
      "Cohere tool call is missing a function name",
    );
  }
  const functionArguments = toolCall.function?.arguments;
  return {
    type: "tool-call",
    tool_call_id: toolCall.id,
    tool_name: functionName,
    args: functionArguments
      ? (JSON.parse(functionArguments) as Record<string, unknown>)
      : {},
  };
}

// MARK: To SDK Delta

function mapCohereStreamedContent(
  event:
    | Cohere.StreamedChatResponseV2.ContentDelta
    | Cohere.StreamedChatResponseV2.ContentStart,
): ContentDelta | null {
  const text = event.delta?.message?.content?.text;
  const index = event.index;
  if (!text || typeof index !== "number") {
    return null;
  }
  return {
    index: index,
    part: {
      type: "text",
      text,
    },
  };
}

function mapCohereToolCallStartEvent(
  event: Cohere.ChatToolCallStartEvent,
): ContentDelta | null {
  const toolCall = event.delta?.message?.toolCalls;
  const index = event.index;
  if (!toolCall || typeof index !== "number") return null;
  const part: ToolCallPartDelta = {
    type: "tool-call",
  };
  if (toolCall.id) {
    part.tool_call_id = toolCall.id;
  }
  if (toolCall.function?.name) {
    part.tool_name = toolCall.function.name;
  }
  if (toolCall.function?.arguments) {
    part.args = toolCall.function.arguments;
  }
  return {
    index,
    part,
  };
}

function mapCohereToolCallDeltaEvent(
  event: Cohere.ChatToolCallDeltaEvent,
): ContentDelta | null {
  const toolCall = event.delta?.message?.toolCalls;
  const index = event.index;
  if (!toolCall || typeof index !== "number") return null;

  const part: ToolCallPartDelta = { type: "tool-call" };
  if (toolCall.function?.arguments) {
    part.args = toolCall.function.arguments;
  }
  return {
    index,
    part,
  };
}

// MARK: To SDK Usage

function mapCohereUsage(usage: Cohere.Usage): ModelUsage {
  return {
    input_tokens:
      usage.billedUnits?.inputTokens ?? usage.tokens?.inputTokens ?? 0,
    output_tokens:
      usage.billedUnits?.outputTokens ?? usage.tokens?.outputTokens ?? 0,
  };
}
