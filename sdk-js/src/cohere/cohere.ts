import { Cohere, CohereClientV2 } from "cohere-ai";
import {
  InvalidInputError,
  InvariantError,
  UnsupportedError,
} from "../errors.ts";
import type {
  LanguageModel,
  LanguageModelMetadata,
} from "../language-model.ts";
import { traceLanguageModel } from "../opentelemetry.ts";
import type {
  ContentDelta,
  ImagePart,
  LanguageModelInput,
  Message,
  ModelResponse,
  ModelUsage,
  Part,
  PartialModelResponse,
  ReasoningOptions,
  ResponseFormatOption,
  SourcePart,
  TextPart,
  Tool,
  ToolCallPart,
  ToolCallPartDelta,
  ToolChoiceOption,
} from "../types.ts";
import { calculateCost } from "../usage.utils.ts";
import type { PatchedAssistantMessageV2ContentItem } from "./types.ts";

const PROVIDER = "cohere";

export interface CohereModelOptions {
  apiKey: string;
  modelId: string;
}

export class CohereModel implements LanguageModel {
  provider: string;
  modelId: string;
  metadata?: LanguageModelMetadata;

  #cohere: CohereClientV2;

  constructor(options: CohereModelOptions, metadata?: LanguageModelMetadata) {
    this.provider = PROVIDER;
    this.modelId = options.modelId;
    if (metadata) this.metadata = metadata;
    this.#cohere = new CohereClientV2({
      token: options.apiKey,
    });

    traceLanguageModel(this);
  }

  async generate(input: LanguageModelInput): Promise<ModelResponse> {
    const request = convertToCohereChatRequest(input, this.modelId);
    const response = await this.#cohere.chat(request);

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
    const stream = await this.#cohere.chatStream(request);

    for await (const event of stream) {
      switch (event.type) {
        case "content-delta": {
          const incomingContentDelta = mapCohereStreamedContent(event);
          if (incomingContentDelta) {
            const event: PartialModelResponse = {
              delta: incomingContentDelta,
            };
            yield event;
          }
          break;
        }
        case "tool-call-start": {
          const incomingContentDelta = mapCohereToolCallStartEvent(event);
          if (incomingContentDelta) {
            const event: PartialModelResponse = {
              delta: incomingContentDelta,
            };
            yield event;
          }
          break;
        }
        case "tool-call-delta": {
          const incomingContentDelta = mapCohereToolCallDeltaEvent(event);
          if (incomingContentDelta) {
            const event: PartialModelResponse = {
              delta: incomingContentDelta,
            };
            yield event;
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
    reasoning,
  } = input;

  const { messages: cohereMessages, documents: cohereDocuments } =
    convertToCohereMessages(messages, system_prompt);

  const request: Cohere.V2ChatRequest = {
    model: modelId,
    messages: cohereMessages,
    documents: cohereDocuments,
    ...extra,
  };

  if (typeof max_tokens === "number") {
    request.maxTokens = max_tokens;
  }
  if (typeof temperature === "number") {
    request.temperature = temperature;
  }
  if (typeof top_p === "number") {
    request.p = top_p;
  }
  if (typeof top_k === "number") {
    request.k = top_k;
  }
  if (typeof presence_penalty === "number") {
    request.presencePenalty = presence_penalty;
  }
  if (typeof frequency_penalty === "number") {
    request.frequencyPenalty = frequency_penalty;
  }
  if (typeof seed === "number") {
    request.seed = seed;
  }
  if (tools) {
    request.tools = tools.map(convertToCohereTool);
    request.strictTools = true;
  }
  if (tool_choice) {
    const toolChoice = convertToCohereToolChoice(tool_choice);
    if (toolChoice) {
      request.toolChoice = toolChoice;
    }
  }
  if (response_format) {
    request.responseFormat = convertToCohereResponseFormat(response_format);
  }
  if (reasoning) {
    request.thinking = convertToCohereThinking(reasoning);
  }

  return request;
}

// MARK: To Provider Messages

function convertToCohereMessages(
  messages: Message[],
  systemPrompt: string | undefined,
): {
  messages: Cohere.ChatMessageV2[];
  documents: Cohere.V2ChatRequestDocumentsItem[];
} {
  const cohereMessages: Cohere.ChatMessageV2[] = [];
  const cohereDocuments: Cohere.V2ChatRequestDocumentsItem[] = [];

  if (systemPrompt) {
    cohereMessages.push({
      role: "system",
      content: systemPrompt,
    });
  }

  messages.forEach((message) => {
    const { parts: messageContent, documents } = separatePartsAndDocuments(
      message.content,
    );
    cohereDocuments.push(...documents);

    switch (message.role) {
      case "user": {
        cohereMessages.push({
          role: "user",
          content: messageContent.map(convertToCohereContent),
        });
        break;
      }

      case "assistant": {
        const cohereAssistantMessage: Omit<
          Cohere.ChatMessageV2.Assistant,
          "content"
        > & {
          content?: PatchedAssistantMessageV2ContentItem[];
        } = {
          role: "assistant",
        };
        messageContent.forEach((part) => {
          switch (part.type) {
            case "text": {
              cohereAssistantMessage.content =
                cohereAssistantMessage.content ?? [];
              cohereAssistantMessage.content.push(
                convertToCohereTextContent(part),
              );
              break;
            }
            case "reasoning": {
              cohereAssistantMessage.content =
                cohereAssistantMessage.content ?? [];
              cohereAssistantMessage.content.push({
                type: "thinking",
                thinking: part.text,
              });
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
        messageContent.forEach((part) => {
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

  return {
    messages: cohereMessages,
    documents: cohereDocuments,
  };
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

// source parts are not supported as regular Cohere Content.
// Instead they are provided in a separate property called documents
// with type V2ChatRequestDocumentsItem.
// Return those documents and remaining parts that are not documents
function separatePartsAndDocuments(parts: Part[]): {
  parts: Part[];
  documents: Cohere.V2ChatRequestDocumentsItem[];
} {
  const remainingParts: Part[] = [];
  const documents: Cohere.V2ChatRequestDocumentsItem[] = [];
  for (const part of parts) {
    if (part.type === "source") {
      documents.push({
        data: convertToCohereDocumentData(part),
      });
    } else {
      remainingParts.push(part);
    }
  }

  return {
    parts: remainingParts,
    documents,
  };
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

function convertToCohereDocumentContent(
  sourcePart: SourcePart,
): Cohere.ToolContent.Document {
  return {
    type: "document",
    document: {
      data: convertToCohereDocumentData(sourcePart),
    },
  };
}

function convertToCohereDocumentData(
  sourcePart: SourcePart,
): Record<string, unknown> {
  function partToRecord(part: Part): Record<string, unknown> {
    switch (part.type) {
      case "text":
        return { text: part.text };
    }
    return {};
  }

  const firstPart = sourcePart.content[0];
  if (!firstPart) return {};

  if (sourcePart.content.length === 1) {
    return partToRecord(firstPart);
  }

  return {
    content: sourcePart.content,
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
    case "source":
      return convertToCohereDocumentContent(part);
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

function convertToCohereThinking(reasoning: ReasoningOptions): Cohere.Thinking {
  return {
    type: reasoning.enabled ? "enabled" : "disabled",
    ...(reasoning.budget_tokens && { tokenBudget: reasoning.budget_tokens }),
  };
}

// MARK: To SDK Message

function mapCohereMessageResponse(
  messageResponse: Cohere.AssistantMessageResponse,
): Part[] {
  const parts: Part[] = [];

  if (messageResponse.content) {
    messageResponse.content.forEach((contentItem) => {
      parts.push(mapCohereResponseContentItem(contentItem));
    });
  }

  if (messageResponse.toolCalls) {
    messageResponse.toolCalls.forEach((toolCall) => {
      parts.push(mapCohereToolCall(toolCall));
    });
  }

  return parts;
}

function mapCohereResponseContentItem(
  content: Cohere.AssistantMessageResponseContentItem,
): Part {
  switch (content.type) {
    case "text": {
      return {
        type: "text",
        text: content.text,
      };
    }
    case "thinking": {
      return {
        type: "reasoning",
        text: content.thinking,
      };
    }
  }
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
  event: Cohere.V2ChatStreamResponse.ContentDelta,
): ContentDelta | null {
  const text = event.delta?.message?.content?.text;
  const thinking = event.delta?.message?.content?.thinking;
  const index = event.index;
  if (typeof index !== "number") {
    return null;
  }
  if (text) {
    return {
      index,
      part: {
        type: "text",
        text,
      },
    };
  }
  if (thinking) {
    return {
      index,
      part: {
        type: "reasoning",
        text: thinking,
      },
    };
  }
  return null;
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
