import Anthropic from "@anthropic-ai/sdk";
import { UnsupportedError } from "../errors.ts";
import type {
  LanguageModel,
  LanguageModelMetadata,
} from "../language-model.ts";
import { traceLanguageModel } from "../opentelemetry.ts";
import { looselyConvertPartToPartDelta } from "../stream.utils.ts";
import type {
  Citation,
  CitationDelta,
  ContentDelta,
  ImagePart,
  LanguageModelInput,
  Message,
  ModelResponse,
  ModelUsage,
  Part,
  PartDelta,
  PartialModelResponse,
  ReasoningOptions,
  ReasoningPart,
  SourcePart,
  TextPart,
  Tool,
  ToolCallPart,
  ToolChoiceOption,
  ToolResultPart,
} from "../types.ts";
import { calculateCost } from "../usage.utils.ts";

export interface AnthropicModelOptions {
  baseURL?: string;
  apiKey: string;
  modelId: string;
}

const PROVIDER = "anthropic";

export class AnthropicModel implements LanguageModel {
  provider: string;
  modelId: string;
  metadata?: LanguageModelMetadata;

  #anthropic: Anthropic;

  constructor(
    options: AnthropicModelOptions,
    metadata?: LanguageModelMetadata,
  ) {
    this.provider = PROVIDER;
    this.modelId = options.modelId;
    if (metadata) this.metadata = metadata;
    this.#anthropic = new Anthropic({
      baseURL: options.baseURL,
      apiKey: options.apiKey,
    });

    traceLanguageModel(this);
  }

  async generate(input: LanguageModelInput): Promise<ModelResponse> {
    const createParams = convertToAnthropicCreateParams(input, this.modelId);

    const response = await this.#anthropic.messages.create(createParams);

    const content = mapAnthropicMessage(response.content);
    const usage = mapAnthropicUsage(response.usage);

    const result: ModelResponse = { content, usage };

    if (this.metadata?.pricing) {
      result.cost = calculateCost(usage, this.metadata.pricing);
    }

    return result;
  }

  async *stream(
    input: LanguageModelInput,
  ): AsyncGenerator<PartialModelResponse> {
    const createParams = convertToAnthropicCreateParams(input, this.modelId);

    const stream = this.#anthropic.messages.stream(createParams);

    for await (const chunk of stream) {
      switch (chunk.type) {
        case "message_start": {
          const usage = mapAnthropicUsage(chunk.message.usage);
          const event: PartialModelResponse = { usage };
          if (this.metadata?.pricing) {
            event.cost = calculateCost(usage, this.metadata.pricing);
          }
          yield event;
          break;
        }
        case "message_delta": {
          const usage = mapAnthropicMessageDeltaUsage(chunk.usage);
          const event: PartialModelResponse = { usage };
          if (this.metadata?.pricing) {
            event.cost = calculateCost(usage, this.metadata.pricing);
          }
          yield event;
          break;
        }
        case "content_block_start": {
          const incomingContentDeltas =
            mapAnthropicRawContentBlockStartEvent(chunk);
          for (const delta of incomingContentDeltas) {
            const event: PartialModelResponse = { delta };
            yield event;
          }
          break;
        }
        case "content_block_delta": {
          const incomingContentDeltas =
            mapAnthropicRawContentBlockDeltaEvent(chunk);
          for (const delta of incomingContentDeltas) {
            const event: PartialModelResponse = { delta };
            yield event;
          }
          break;
        }
      }
    }
  }
}

function convertToAnthropicCreateParams(
  input: LanguageModelInput,
  modelId: string,
): Omit<Anthropic.Messages.MessageCreateParams, "stream"> {
  const {
    messages,
    system_prompt,
    max_tokens,
    temperature,
    top_p,
    top_k,
    tools,
    tool_choice,
    reasoning,
  } = input;

  const maxTokens = max_tokens ?? 4096;

  const params: Omit<Anthropic.Messages.MessageCreateParams, "stream"> = {
    model: modelId,
    messages: convertToAnthropicMessages(messages),
    max_tokens: maxTokens,
  };
  if (system_prompt) {
    params.system = system_prompt;
  }
  if (typeof temperature === "number") {
    params.temperature = temperature;
  }
  if (typeof top_p === "number") {
    params.top_p = top_p;
  }
  if (typeof top_k === "number") {
    params.top_k = top_k;
  }
  if (tools) {
    params.tools = tools.map(convertToAnthropicTool);
  }
  if (tool_choice) {
    params.tool_choice = convertToAnthropicToolChoice(tool_choice);
  }
  if (reasoning) {
    params.thinking = convertToAnthropicThinkingConfigParam(
      reasoning,
      maxTokens,
    );
  }

  return params;
}

// MARK: To Provider Messages

function convertToAnthropicMessages(
  messages: Message[],
): Anthropic.Messages.MessageParam[] {
  return messages.map((message): Anthropic.Messages.MessageParam => {
    switch (message.role) {
      case "assistant":
        return {
          role: "assistant",
          content: message.content.map(convertToAnthropicContentBlockParam),
        };
      case "user":
      case "tool": {
        return {
          role: "user",
          content: message.content.map(convertToAnthropicContentBlockParam),
        };
      }
    }
  });
}

function convertToAnthropicContentBlockParam(
  part: Part,
): Anthropic.ContentBlockParam {
  switch (part.type) {
    case "text":
      return convertToAnthropicTextBlockParam(part);
    case "image":
      return convertToAnthropicImageBlockParam(part);
    case "source":
      return convertToAnthropicSearchResultBlockParam(part);
    case "tool-call":
      return convertToAnthropicToolUseBlockParam(part);
    case "tool-result":
      return convertToAnthropicToolResultBlockParam(part);
    case "reasoning":
      return convertToAnthropicThinkingBlockParam(part);
    default:
      throw new UnsupportedError(
        PROVIDER,
        `Cannot convert part to Anthropic content for type ${part.type}`,
      );
  }
}

function convertToAnthropicTextBlockParam(
  part: TextPart,
): Anthropic.TextBlockParam {
  return {
    type: "text",
    text: part.text,
  };
}

function convertToAnthropicImageBlockParam(
  part: ImagePart,
): Anthropic.ImageBlockParam {
  return {
    type: "image",
    source: {
      data: part.data,
      type: "base64",
      media_type:
        part.mime_type as Anthropic.Messages.Base64ImageSource["media_type"],
    },
  };
}

function convertToAnthropicSearchResultBlockParam(
  part: SourcePart,
): Anthropic.SearchResultBlockParam {
  return {
    type: "search_result",
    source: part.source,
    title: part.title,
    content: part.content.map((part) => {
      // only text blocks are allowed inside source blocks
      if (part.type !== "text") {
        throw new UnsupportedError(
          PROVIDER,
          `Cannot convert source part to Anthropic SearchResultBlockParam content for type ${part.type}`,
        );
      }
      return convertToAnthropicTextBlockParam(part);
    }),
    citations: {
      enabled: true,
    },
  };
}

function convertToAnthropicToolUseBlockParam(
  part: ToolCallPart,
): Anthropic.ToolUseBlockParam {
  return {
    type: "tool_use",
    id: part.tool_call_id,
    name: part.tool_name,
    input: part.args,
  };
}

function convertToAnthropicToolResultBlockParam(
  part: ToolResultPart,
): Anthropic.ToolResultBlockParam {
  return {
    type: "tool_result",
    tool_use_id: part.tool_call_id,
    content: part.content.map((part) => {
      const blockParam = convertToAnthropicContentBlockParam(part);
      if (
        blockParam.type !== "text" &&
        blockParam.type !== "image" &&
        blockParam.type !== "search_result"
      ) {
        throw new UnsupportedError(
          PROVIDER,
          `Cannot convert tool result part to Anthropic ToolResultBlockParam content for type ${blockParam.type}`,
        );
      }
      return blockParam;
    }),
    is_error: part.is_error ?? false,
  };
}

export function convertToAnthropicThinkingBlockParam(
  part: ReasoningPart,
): Anthropic.ThinkingBlockParam | Anthropic.RedactedThinkingBlockParam {
  // redacted block will have data field of base64. we put that in signature instead of text
  if (part.text === "" && part.signature) {
    return {
      type: "redacted_thinking",
      data: part.signature,
    };
  }

  return {
    type: "thinking",
    thinking: part.text,
    signature: part.signature ?? "",
  };
}

// MARK: To Provider Tools

function convertToAnthropicTool(tool: Tool): Anthropic.Tool {
  return {
    name: tool.name,
    description: tool.description,
    input_schema: tool.parameters as Anthropic.Tool.InputSchema,
  };
}

function convertToAnthropicToolChoice(
  toolChoice: ToolChoiceOption,
): Anthropic.ToolChoice {
  switch (toolChoice.type) {
    case "auto":
      return {
        type: "auto",
      };
    case "required":
      return {
        type: "any",
      };
    case "tool": {
      return {
        type: "tool",
        name: toolChoice.tool_name,
      };
    }
    case "none": {
      return {
        type: "none",
      };
    }
  }
}

function convertToAnthropicThinkingConfigParam(
  reasoning: ReasoningOptions,
  maxTokens: number,
): Anthropic.ThinkingConfigParam {
  if (!reasoning.enabled) {
    return {
      type: "disabled",
    };
  }
  return {
    type: "enabled",
    budget_tokens: reasoning.budget_tokens ?? maxTokens - 1,
  };
}

// MARK: To SDK Message

function mapAnthropicMessage(
  contentBlocks: Anthropic.Messages.ContentBlock[],
): Part[] {
  return contentBlocks.map(mapAnthropicBlock).filter((b) => !!b);
}

function mapAnthropicBlock(
  block: Anthropic.Messages.ContentBlock,
): Part | null {
  switch (block.type) {
    case "text":
      return mapAnthropicTextBlock(block);
    case "tool_use":
      return {
        type: "tool-call",
        tool_call_id: block.id,
        tool_name: block.name,
        args: block.input as Record<string, unknown>,
      };
    case "thinking":
      return {
        type: "reasoning",
        text: block.thinking,
        signature: block.signature,
      };
    case "redacted_thinking":
      return {
        type: "reasoning",
        text: "",
        signature: block.data,
      };
    default:
      return null;
  }
}

function mapAnthropicTextBlock(block: Anthropic.Messages.TextBlock): TextPart {
  const textPart: TextPart = {
    type: "text",
    text: block.text,
  };

  if (block.citations) {
    textPart.citations = block.citations
      .map((textCitation): Citation | null => {
        if (textCitation.type === "search_result_location") {
          const citation: Citation = {
            source: textCitation.source,
            cited_text: textCitation.cited_text,
            start_index: textCitation.start_block_index,
            end_index: textCitation.end_block_index,
          };
          if (textCitation.title) {
            citation.title = textCitation.title;
          }
          return citation;
        }
        return null;
      })
      .filter((c) => !!c);
  }

  return textPart;
}

// MARK: To SDK Delta

function mapAnthropicRawContentBlockStartEvent(
  event: Anthropic.RawContentBlockStartEvent,
): ContentDelta[] {
  const part = mapAnthropicBlock(event.content_block);
  if (!part) return [];
  const partDelta = looselyConvertPartToPartDelta(part);

  if (partDelta.type === "tool-call") {
    // Start event for tool call should not have content
    partDelta.args = "";
  }

  return [
    {
      index: event.index,
      part: partDelta,
    },
  ];
}

function mapAnthropicRawContentBlockDeltaEvent(
  event: Anthropic.RawContentBlockDeltaEvent,
): ContentDelta[] {
  const partDelta = mapAnthropicRawContentBlockDelta(event.delta);
  if (!partDelta) return [];
  return [
    {
      index: event.index,
      part: partDelta,
    },
  ];
}

function mapAnthropicRawContentBlockDelta(
  delta: Anthropic.RawContentBlockDelta,
): PartDelta | null {
  switch (delta.type) {
    case "text_delta":
      return {
        type: "text",
        text: delta.text,
      };
    case "input_json_delta":
      return {
        type: "tool-call",
        args: delta.partial_json,
      };
    case "thinking_delta":
      return {
        type: "reasoning",
        text: delta.thinking,
      };
    case "signature_delta":
      return {
        type: "reasoning",
        signature: delta.signature,
        text: "",
      };
    case "citations_delta":
      if (delta.citation.type === "search_result_location") {
        const citation: CitationDelta = {
          type: "citation",
          start_index: delta.citation.start_block_index,
          end_index: delta.citation.end_block_index,
          source: delta.citation.source,
          cited_text: delta.citation.cited_text,
        };
        if (delta.citation.title) {
          citation.title = delta.citation.title;
        }
        return {
          type: "text",
          text: "",
          citation,
        };
      }
      return null;
    default:
      return null;
  }
}

// MARK: To SDK Usage

function mapAnthropicUsage(usage: Anthropic.Usage): ModelUsage {
  return {
    input_tokens: usage.input_tokens,
    output_tokens: usage.output_tokens,
  };
}

function mapAnthropicMessageDeltaUsage(usage: Anthropic.MessageDeltaUsage) {
  return {
    input_tokens: 0,
    output_tokens: usage.output_tokens,
  };
}
