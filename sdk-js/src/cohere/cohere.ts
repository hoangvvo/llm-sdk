import type { Cohere } from "cohere-ai";
import { CohereClientV2 } from "cohere-ai";
import {
  InvalidValueError,
  ModelUnsupportedMessagePart,
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
  Tool,
  ToolCallPart,
  ToolCallPartDelta,
} from "../types.js";
import { convertAudioPartsToTextParts } from "../utils/message.utils.js";
import { ContentDeltaAccumulator } from "../utils/stream.utils.js";
import { calculateCost } from "../utils/usage.utils.js";
import type { CohereModelOptions } from "./types.js";

export type CohereLanguageModelInput = LanguageModelInput & {
  extra?: Partial<Cohere.V2ChatRequest>;
};

export class CohereModel extends LanguageModel {
  public provider: string;
  public modelId: string;
  public metadata?: LanguageModelMetadata;

  private cohere: CohereClientV2;

  constructor(
    public options: CohereModelOptions,
    metadata?: LanguageModelMetadata,
  ) {
    super();
    this.provider = "cohere";
    this.modelId = options.modelId;
    if (metadata) this.metadata = metadata;

    this.cohere = new CohereClientV2({
      token: options.apiKey,
    });
  }

  async generate(input: LanguageModelInput): Promise<ModelResponse> {
    const response = await this.cohere.chat(
      convertToCohereParams(input, this.options),
    );

    const usage = response.usage ? mapCohereUsage(response.usage) : undefined;

    const result: ModelResponse = {
      content: mapCohereMessage(response.message).content,
    };
    if (usage) {
      result.usage = usage;
      if (this.metadata?.pricing) {
        result.cost = calculateCost(usage, this.metadata.pricing);
      }
    }

    return result;
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

    const result: ModelResponse = {
      content: accumulator.computeContent(),
    };
    if (usage) {
      result.usage = usage;
      if (this.metadata?.pricing) {
        result.cost = calculateCost(usage, this.metadata.pricing);
      }
    }

    return result;
  }
}

export function convertToCohereParams(
  input: LanguageModelInput,
  options: CohereModelOptions,
): Cohere.V2ChatRequest {
  const response_format = convertToCohereResponseFormat(input.response_format);
  const samplingParams = convertToCohereSamplingParams(input);

  const params: Cohere.V2ChatRequest = {
    model: options.modelId,
    messages: convertToCohereMessages(input, options),
    ...samplingParams,
    ...input.extra,
  };
  if (input.tools) {
    params.tools = input.tools.map(convertToCohereTool);
  }
  if (response_format) {
    params.responseFormat = response_format;
  }
  return params;
}

export function convertToCohereMessages(
  input: Pick<CohereLanguageModelInput, "messages" | "system_prompt">,
  options: CohereModelOptions,
): Cohere.ChatMessageV2[] {
  const cohereMessages: Cohere.ChatMessageV2[] = [];

  let messages = input.messages;
  if (options.convertAudioPartsToTextParts) {
    messages = messages.map(convertAudioPartsToTextParts);
  }

  if (input.system_prompt) {
    cohereMessages.push({
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
    const { content, toolCalls, toolResults } =
      convertToCohereMessageParam(message);
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
        const assistantMessage: Cohere.ChatMessageV2 = { role: "assistant" };
        if (content) {
          assistantMessage.content = content;
        }
        if (toolCalls) {
          assistantMessage.toolCalls = toolCalls;
        }
        cohereMessages.push(assistantMessage);
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

export function convertToCohereMessageParam(message: Message): {
  content?: Cohere.Content[];
  toolCalls?: Cohere.ToolCallV2[];
  toolResults?: Cohere.ToolMessageV2[];
} {
  let content: Cohere.Content[] | undefined;
  let toolCalls: Cohere.ToolCallV2[] | undefined;
  let toolResults: Cohere.ToolMessageV2[] | undefined;

  message.content.forEach((part) => {
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
        throw new ModelUnsupportedMessagePart("cohere", message, part);
      case "tool-call": {
        toolCalls = toolCalls ?? [];
        toolCalls.push({
          id: part.tool_call_id,
          type: "function",
          function: {
            name: part.tool_name,
            ...(part.args ? { arguments: JSON.stringify(part.args) } : {}),
          },
        });
        break;
      }
      case "tool-result": {
        toolResults = toolResults ?? [];
        toolResults.push({
          toolCallId: part.tool_call_id,
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

  const result: {
    content?: Cohere.Content[];
    toolCalls?: Cohere.ToolCallV2[];
    toolResults?: Cohere.ToolMessageV2[];
  } = {};
  if (content) {
    result.content = content;
  }
  if (toolCalls) {
    result.toolCalls = toolCalls;
  }
  if (toolResults) {
    result.toolResults = toolResults;
  }
  return result;
}

export function convertToCohereSamplingParams(
  input: Partial<LanguageModelInput>,
): Partial<Cohere.V2ChatRequest> {
  const params: Partial<Cohere.V2ChatRequest> = {};
  if (typeof input.max_tokens === "number") {
    params.maxTokens = input.max_tokens;
  }
  if (typeof input.temperature === "number") {
    params.temperature = input.temperature;
  }
  if (typeof input.top_p === "number") {
    params.p = input.top_p;
  }
  if (typeof input.presence_penalty === "number") {
    params.presencePenalty = input.presence_penalty;
  }
  if (typeof input.frequency_penalty === "number") {
    params.frequencyPenalty = input.frequency_penalty;
  }
  if (typeof input.seed === "number") {
    params.seed = input.seed;
  }
  return params;
}

export function convertToCohereTool(tool: Tool): Cohere.ToolV2 {
  const toolParams: Cohere.ToolV2 = {
    type: "function",
    function: {
      name: tool.name,
      description: tool.description,
    },
  };

  if (tool.parameters && toolParams.function) {
    toolParams.function.parameters = convertToCohereSchema(tool.parameters);
  }
  return toolParams;
}

export function convertToCohereSchema(
  schema: Record<string, unknown>,
): Record<string, unknown> {
  const newSchema: Record<string, unknown> = { ...schema };
  if (newSchema["properties"]) {
    newSchema["properties"] = convertToCohereSchema(
      newSchema["properties"] as Record<string, unknown>,
    );
  }
  if (newSchema["items"]) {
    newSchema["items"] = convertToCohereSchema(
      newSchema["items"] as Record<string, unknown>,
    );
  }
  if (newSchema["anyOf"]) {
    newSchema["anyOf"] = (newSchema["anyOf"] as Record<string, unknown>[]).map(
      convertToCohereSchema,
    );
  }
  if (newSchema["allOf"]) {
    newSchema["allOf"] = (newSchema["allOf"] as Record<string, unknown>[]).map(
      convertToCohereSchema,
    );
  }
  if (newSchema["oneOf"]) {
    newSchema["oneOf"] = (newSchema["oneOf"] as Record<string, unknown>[]).map(
      convertToCohereSchema,
    );
  }
  if ("additionalProperties" in newSchema) {
    delete newSchema["additionalProperties"];
  }
  return newSchema;
}

export function convertToCohereResponseFormat(
  responseFormat: LanguageModelInput["response_format"],
): Cohere.ResponseFormatV2 | undefined {
  if (!responseFormat) return undefined;

  switch (responseFormat.type) {
    case "json": {
      const format: Cohere.ResponseFormatV2 = {
        type: "json_object",
      };
      if (responseFormat.schema) {
        format.jsonSchema = convertToCohereSchema(responseFormat.schema);
      }
      return format;
    }
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
    tool_call_id: toolCall.id,
    tool_name: functionName,
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
      const toolCall = event.delta?.message?.toolCalls;
      const part: ToolCallPartDelta = {
        type: "tool-call",
      };
      if (toolCall?.id) {
        part.tool_call_id = toolCall.id;
      }
      if (toolCall?.function?.name) {
        part.tool_name = toolCall.function.name;
      }
      if (toolCall?.function?.arguments) {
        part.args = toolCall.function.arguments;
      }
      return {
        index: event.index,
        part,
      };
    }
    case "tool-call-delta": {
      const toolCall = event.delta?.message?.toolCalls;
      const part: ToolCallPartDelta = { type: "tool-call" };
      if (toolCall?.function?.arguments) {
        part.args = toolCall.function.arguments;
      }
      return {
        index: event.index,
        part,
      };
    }
  }
}

export function mapCohereUsage(usage: Cohere.Usage): ModelUsage | undefined {
  if (!usage.tokens?.inputTokens && !usage.tokens?.outputTokens) {
    return undefined;
  }
  return {
    input_tokens: usage.tokens.inputTokens || 0,
    output_tokens: usage.tokens.outputTokens || 0,
  };
}
