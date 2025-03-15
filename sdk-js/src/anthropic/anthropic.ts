import Anthropic from "@anthropic-ai/sdk";
import { NotImplementedError, UnsupportedError } from "../errors.ts";
import type {
  LanguageModel,
  LanguageModelMetadata,
} from "../language-model.ts";
import { traceLanguageModel } from "../opentelemetry.ts";
import { getCompatiblePartsWithoutSourceParts } from "../source-part.utils.ts";
import { looselyConvertPartToPartDelta } from "../stream.utils.ts";
import type {
  ContentDelta,
  ImagePart,
  LanguageModelInput,
  Message,
  ModelResponse,
  ModelUsage,
  Part,
  PartDelta,
  PartialModelResponse,
  ReasoningPart,
  ReasoningPartDelta,
  SourcePart,
  TextPart,
  TextPartDelta,
  Tool,
  ToolCallPart,
  ToolCallPartDelta,
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
          yield event;
          break;
        }
        case "message_delta": {
          const usage = mapAnthropicMessageDeltaUsage(chunk.usage);
          const event: PartialModelResponse = { usage };
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
    extra,
  } = input;

  return {
    model: modelId,
    messages: convertToAnthropicMessages(messages),
    ...(system_prompt && { system: system_prompt }),
    max_tokens: max_tokens ?? 4096,
    ...(typeof temperature === "number" && {
      temperature,
    }),
    ...(typeof top_p === "number" && { top_p }),
    ...(typeof top_k === "number" && { top_k }),
    ...(tools && { tools: tools.map(convertToAnthropicTool) }),
    ...(tool_choice && {
      tool_choice: convertToAnthropicToolChoice(tool_choice),
    }),
    ...extra,
  };
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
      return convertToAnthropicDocumentBlockParam(part);
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
      data: part.image_data,
      type: "base64",
      media_type:
        part.mime_type as Anthropic.Messages.Base64ImageSource["media_type"],
    },
  };
}

function convertToAnthropicDocumentBlockParam(
  part: SourcePart,
): Anthropic.DocumentBlockParam {
  return {
    type: "document",
    title: part.title,
    source: {
      type: "content",
      content: part.content.map(convertToAnthropicContentBlockSourceContent),
    },
    citations: {
      enabled: true,
    },
  };
}

function convertToAnthropicContentBlockSourceContent(
  part: Part,
): Anthropic.ContentBlockSourceContent {
  switch (part.type) {
    case "text":
      return convertToAnthropicTextBlockParam(part);
    case "image":
      return convertToAnthropicImageBlockParam(part);
    default:
      throw new UnsupportedError(
        PROVIDER,
        `Cannot convert part to Anthropic ContentBlockSourceContent for type ${part.type}`,
      );
  }
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
  const toolResultPartContent = getCompatiblePartsWithoutSourceParts(
    part.content,
  );
  return {
    type: "tool_result",
    tool_use_id: part.tool_call_id,
    content: toolResultPartContent.map((part) => {
      const blockParam = convertToAnthropicContentBlockParam(part);
      if (blockParam.type !== "text" && blockParam.type !== "image") {
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

// MARK: To SDK Message

function mapAnthropicMessage(
  contentBlocks: Anthropic.Messages.ContentBlock[],
): Part[] {
  return contentBlocks.map(mapAnthropicBlock);
}

function mapAnthropicBlock(block: Anthropic.Messages.ContentBlock): Part {
  switch (block.type) {
    case "text":
      return mapAnthropicTextBlock(block);
    case "tool_use":
      return mapAnthropicToolUseBlock(block);
    case "thinking":
      return mapAnthropicThinkingBlock(block);
    case "redacted_thinking":
      return mapAnthropicRedactedThinkingBlock(block);
    default:
      throw new NotImplementedError(
        PROVIDER,
        `Cannot map Anthropic content block for type ${block.type}`,
      );
  }
}

function mapAnthropicTextBlock(block: Anthropic.Messages.TextBlock): TextPart {
  return {
    type: "text",
    text: block.text,
  };
}

function mapAnthropicToolUseBlock(
  block: Anthropic.Messages.ToolUseBlock,
): ToolCallPart {
  return {
    type: "tool-call",
    tool_call_id: block.id,
    tool_name: block.name,
    args: block.input as Record<string, unknown>,
  };
}

function mapAnthropicThinkingBlock(
  block: Anthropic.Messages.ThinkingBlock,
): ReasoningPart {
  return {
    type: "reasoning",
    text: block.thinking,
    signature: block.signature,
  };
}

function mapAnthropicRedactedThinkingBlock(
  block: Anthropic.Messages.RedactedThinkingBlock,
): ReasoningPart {
  return {
    type: "reasoning",
    text: "",
    signature: block.data,
  };
}

// MARK: To SDK Delta

function mapAnthropicRawContentBlockStartEvent(
  event: Anthropic.RawContentBlockStartEvent,
): ContentDelta[] {
  const part = looselyConvertPartToPartDelta(
    mapAnthropicBlock(event.content_block),
  );
  if (part.type === "tool-call") {
    // Start event for tool call should not have content
    part.args = "";
  }

  return [
    {
      index: event.index,
      part,
    },
  ];
}

function mapAnthropicRawContentBlockDeltaEvent(
  event: Anthropic.RawContentBlockDeltaEvent,
): ContentDelta[] {
  const partDelta = mapAnthropicRawContentBlockDelta(event.delta);
  return [
    {
      index: event.index,
      part: partDelta,
    },
  ];
}

function mapAnthropicRawContentBlockDelta(
  delta: Anthropic.RawContentBlockDelta,
): PartDelta {
  switch (delta.type) {
    case "text_delta":
      return mapAnthropicTextDelta(delta);
    case "input_json_delta":
      return mapAnthropicInputJSONDelta(delta);
    case "thinking_delta":
      return mapAnthropicThinkingDelta(delta);
    case "signature_delta":
      return {
        type: "reasoning",
        signature: delta.signature,
      };
    default: {
      throw new NotImplementedError(
        PROVIDER,
        `Cannot map Anthropic raw content block delta for type ${delta.type}`,
      );
    }
  }
}

function mapAnthropicTextDelta(delta: Anthropic.TextDelta): TextPartDelta {
  return {
    type: "text",
    text: delta.text,
  };
}

function mapAnthropicInputJSONDelta(
  delta: Anthropic.InputJSONDelta,
): ToolCallPartDelta {
  return {
    type: "tool-call",
    args: delta.partial_json,
  };
}

function mapAnthropicThinkingDelta(
  delta: Anthropic.ThinkingDelta,
): ReasoningPartDelta {
  return {
    type: "reasoning",
    text: delta.thinking,
  };
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
