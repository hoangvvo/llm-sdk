import Anthropic from "@anthropic-ai/sdk";
import { RefusalError, UnsupportedError } from "../errors.ts";
import type {
  LanguageModel,
  LanguageModelCallOptions,
  LanguageModelMetadata,
} from "../language-model.ts";
import { traceLanguageModel } from "../opentelemetry.ts";
import { looselyConvertPartToPartDelta } from "../stream.utils.ts";
import { CANCELLED_TOOL_RESULT_FALLBACK_CONTENT } from "../tool-result.utils.ts";
import type {
  Citation,
  CitationDelta,
  ContentDelta,
  ImagePart,
  LanguageModelInput,
  Message,
  ModelResponse,
  ModelTokensDetails,
  ModelUsage,
  Part,
  PartDelta,
  PartialModelResponse,
  ReasoningOptions,
  ReasoningPart,
  ResponseFormatOption,
  SourcePart,
  TextPart,
  Tool,
  ToolCallPart,
  ToolCallPartDelta,
  ToolChoiceOption,
  ToolResultPart,
  ToolSearchToolResult,
} from "../types.ts";
import { calculateCost, sumModelUsage } from "../usage.utils.ts";

export interface AnthropicModelOptions {
  baseURL?: string;
  apiKey: string;
  modelId: string;
  dangerouslyAllowBrowser?: boolean;
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
      dangerouslyAllowBrowser: options.dangerouslyAllowBrowser,
    });

    traceLanguageModel(this);
  }

  async generate(
    input: LanguageModelInput,
    options?: LanguageModelCallOptions,
  ): Promise<ModelResponse> {
    const createParams = convertToAnthropicCreateParams(input, this.modelId);

    const response = await this.#anthropic.messages.create(createParams, {
      signal: options?.signal,
    });

    if (response.stop_reason === "refusal") {
      throw new RefusalError(anthropicRefusalMessage(response.stop_details));
    }

    const content = mapAnthropicMessage(response.content);
    const usage = mapAnthropicUsage(response.usage);

    const result: ModelResponse = { content, usage };

    if (this.metadata?.pricing) {
      result.cost = calculateCost(usage, this.metadata.pricing, {
        input_cache_tokens_are_additional: true,
        output_reasoning_tokens_are_additional: false,
      });
    }

    return result;
  }

  async *stream(
    input: LanguageModelInput,
    options?: LanguageModelCallOptions,
  ): AsyncGenerator<PartialModelResponse> {
    const createParams = convertToAnthropicCreateParams(input, this.modelId);

    // Hosted search can return multiple messages in one stream.
    const stream = await this.#anthropic.messages.create(
      { ...createParams, stream: true },
      { signal: options?.signal },
    );
    // Hosted search arguments are only usable once complete.
    const serverToolBlocks = new Map<
      number,
      { id: string; name: string; input: string }
    >();
    const serverToolCallIndexes = new Map<string, number>();
    // Hosted search can produce multiple billed messages with restarting block indexes.
    let indexOffset = 0;
    let maxIndex = -1;
    const completedUsages: ModelUsage[] = [];
    let messageUsage: AnthropicUsageLike | undefined;

    for await (const chunk of stream) {
      switch (chunk.type) {
        case "message_start": {
          if (messageUsage) {
            completedUsages.push(mapAnthropicUsage(messageUsage));
            indexOffset = maxIndex + 1;
          }
          messageUsage = { ...chunk.message.usage };
          if (chunk.message.stop_reason === "refusal") {
            throw new RefusalError(
              anthropicRefusalMessage(chunk.message.stop_details),
            );
          }
          break;
        }
        case "message_delta": {
          messageUsage = mergeAnthropicUsage(messageUsage ?? {}, chunk.usage);
          if (chunk.delta.stop_reason === "refusal") {
            throw new RefusalError(
              anthropicRefusalMessage(chunk.delta.stop_details),
            );
          }
          break;
        }
        case "content_block_start": {
          const index = chunk.index + indexOffset;
          maxIndex = Math.max(maxIndex, index);
          const block = chunk.content_block;
          if (block.type === "server_tool_use") {
            serverToolBlocks.set(index, {
              id: block.id,
              name: block.name,
              input: "",
            });
            serverToolCallIndexes.set(block.id, index);
            const call = mapAnthropicServerToolUseStart(block);
            if (call) {
              yield {
                delta: {
                  index,
                  part: { type: "tool-call", tool_call_id: block.id, call },
                },
              };
            }
            break;
          }
          if (
            block.type === "web_search_tool_result" ||
            block.type === "tool_search_tool_result"
          ) {
            const callIndex = serverToolCallIndexes.get(block.tool_use_id);
            const status = anthropicServerToolResultStatus(block);
            if (callIndex !== undefined) {
              yield {
                delta: {
                  index: callIndex,
                  part: {
                    type: "tool-call",
                    tool_call_id: block.tool_use_id,
                    call:
                      block.type === "web_search_tool_result"
                        ? { type: "web_search", status }
                        : { type: "tool_search", status },
                  },
                },
              };
            }
          }
          const incomingContentDeltas = mapAnthropicRawContentBlockStartEvent({
            ...chunk,
            index,
          });
          for (const delta of incomingContentDeltas) {
            const event: PartialModelResponse = { delta };
            yield event;
          }
          break;
        }
        case "content_block_delta": {
          const index = chunk.index + indexOffset;
          const serverToolBlock = serverToolBlocks.get(index);
          if (serverToolBlock) {
            if (chunk.delta.type === "input_json_delta") {
              serverToolBlock.input += chunk.delta.partial_json;
            }
            break;
          }
          const incomingContentDeltas = mapAnthropicRawContentBlockDeltaEvent({
            ...chunk,
            index,
          });
          for (const delta of incomingContentDeltas) {
            const event: PartialModelResponse = { delta };
            yield event;
          }
          break;
        }
        case "content_block_stop": {
          const index = chunk.index + indexOffset;
          const serverToolBlock = serverToolBlocks.get(index);
          if (!serverToolBlock) break;
          serverToolBlocks.delete(index);
          const call = mapAnthropicServerToolUseStop(
            serverToolBlock.name,
            serverToolBlock.input,
          );
          if (call) {
            yield {
              delta: {
                index,
                part: {
                  type: "tool-call",
                  tool_call_id: serverToolBlock.id,
                  call,
                },
              },
            };
          }
          break;
        }
      }
    }

    // The raw SDK stream swallows abort errors when its iterator ends.
    if (stream.controller.signal.aborted) {
      throw new Anthropic.APIUserAbortError();
    }

    if (messageUsage) {
      completedUsages.push(mapAnthropicUsage(messageUsage));
    }
    if (completedUsages.length > 0) {
      const usage = sumModelUsage(completedUsages);
      const event: PartialModelResponse = { usage };
      if (this.metadata?.pricing) {
        event.cost = calculateCost(usage, this.metadata.pricing, {
          input_cache_tokens_are_additional: true,
          output_reasoning_tokens_are_additional: false,
        });
      }
      yield event;
    }
  }
}

/**
 * Hosted calls exposed in conversation history:
 * - web search
 * - regex or BM25 tool search
 * - other hosted tools are omitted
 */
function mapAnthropicServerToolUseStart(
  block: Anthropic.Messages.ServerToolUseBlock,
): ToolCallPartDelta["call"] | null {
  switch (block.name) {
    case "web_search":
      return { type: "web_search", status: "in_progress" };
    case "tool_search_tool_regex":
    case "tool_search_tool_bm25":
      return { type: "tool_search", status: "in_progress" };
    default:
      return null;
  }
}

function mapAnthropicServerToolUseStop(
  name: string,
  input: string,
): ToolCallPartDelta["call"] | null {
  let parsedInput: unknown;
  try {
    parsedInput = JSON.parse(input || "{}");
  } catch {
    parsedInput = undefined;
  }
  switch (name) {
    case "web_search": {
      const query = (parsedInput as { query?: unknown } | undefined)?.query;
      return typeof query === "string"
        ? { type: "web_search", action: { type: "search", queries: [query] } }
        : null;
    }
    case "tool_search_tool_regex":
    case "tool_search_tool_bm25":
      return {
        type: "tool_search",
        args: JSON.stringify(isRecord(parsedInput) ? parsedInput : {}),
      };
    default:
      return null;
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function anthropicRefusalMessage(
  details: Anthropic.Messages.RefusalStopDetails | null | undefined,
): string {
  return (
    details?.explanation ??
    (details?.category
      ? `Anthropic policy category: ${details.category}`
      : "Anthropic refused the request")
  );
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
    response_format,
    tools,
    tool_choice,
    reasoning,
    cache_retention,
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
    // Kept for Anthropic models that still support this common generation option.
    // eslint-disable-next-line @typescript-eslint/no-deprecated
    params.temperature = temperature;
  }
  if (typeof top_p === "number") {
    // Kept for Anthropic models that still support this common generation option.
    // eslint-disable-next-line @typescript-eslint/no-deprecated
    params.top_p = top_p;
  }
  if (typeof top_k === "number") {
    // Kept for Anthropic models that still support this common generation option.
    // eslint-disable-next-line @typescript-eslint/no-deprecated
    params.top_k = top_k;
  }
  if (response_format) {
    const outputConfig = convertToAnthropicOutputConfig(response_format);
    if (outputConfig) {
      params.output_config = outputConfig;
    }
  }
  if (tools) {
    params.tools = tools.map(convertToAnthropicTool);
  }
  if (tool_choice) {
    params.tool_choice = convertToAnthropicToolChoice(tool_choice);
  }
  if (reasoning) {
    params.thinking = convertToAnthropicThinkingConfigParam(reasoning);
  }
  if (cache_retention) {
    // Top-level cache_control caches through the last cacheable block.
    params.cache_control = {
      type: "ephemeral",
      ...(cache_retention === "extended" ? { ttl: "1h" } : {}),
    };
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
  const block: Anthropic.TextBlockParam = {
    type: "text",
    text: part.text,
  };
  const citations = part.citations?.flatMap(
    (citation): Anthropic.Messages.CitationWebSearchResultLocationParam[] =>
      citation.signature
        ? [
            {
              type: "web_search_result_location",
              cited_text: citation.cited_text ?? "",
              encrypted_index: citation.signature,
              title: citation.title ?? null,
              url: citation.source,
            },
          ]
        : [],
  );
  // encrypted_index is the provider state Anthropic accepts when a web-search
  // citation is returned in a later assistant message.
  if (citations && citations.length > 0) {
    block.citations = citations;
  }
  return block;
}

function convertToAnthropicImageBlockParam(
  part: ImagePart,
): Anthropic.ImageBlockParam {
  return {
    type: "image",
    source: convertToAnthropicImageSource(part),
  };
}

function convertToAnthropicImageSource(
  part: ImagePart,
): Anthropic.Messages.Base64ImageSource {
  switch (part.mime_type) {
    case "image/jpeg":
    case "image/png":
    case "image/gif":
    case "image/webp":
      return {
        data: part.data,
        type: "base64",
        media_type: part.mime_type,
      };
    default:
      throw new UnsupportedError(
        PROVIDER,
        `Cannot convert image MIME type ${part.mime_type} to Anthropic image source`,
      );
  }
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
): Anthropic.ToolUseBlockParam | Anthropic.ServerToolUseBlockParam {
  if (part.call.type === "tool_search") {
    return {
      type: "server_tool_use",
      id: part.tool_call_id,
      // Regex searches carry a pattern; anything else replays as a BM25 query.
      name:
        "pattern" in part.call.args
          ? "tool_search_tool_regex"
          : "tool_search_tool_bm25",
      input: part.call.args,
    };
  }
  if (part.call.type === "web_search") {
    return {
      type: "server_tool_use",
      id: part.tool_call_id,
      name: "web_search",
      input:
        part.call.action?.type === "search"
          ? { query: part.call.action.queries[0] ?? "" }
          : {},
    };
  }
  return {
    type: "tool_use",
    id: part.tool_call_id,
    name: part.call.name,
    input: part.call.args,
  };
}

function convertToAnthropicToolResultBlockParam(
  part: ToolResultPart,
):
  | Anthropic.ToolResultBlockParam
  | Anthropic.WebSearchToolResultBlockParam
  | Anthropic.Messages.ToolSearchToolResultBlockParam {
  if (part.result.type === "tool_search") {
    return {
      type: "tool_search_tool_result",
      tool_use_id: part.tool_call_id,
      content: convertToAnthropicToolSearchResultContent(part.result),
    };
  }
  if (part.result.type === "web_search") {
    return {
      type: "web_search_tool_result",
      tool_use_id: part.tool_call_id,
      content: part.result.error_code
        ? {
            type: "web_search_tool_result_error",
            error_code: part.result
              .error_code as Anthropic.WebSearchToolResultError["error_code"],
          }
        : part.result.sources.map((source) => ({
            type: "web_search_result" as const,
            url: source.url,
            title: source.title ?? "",
            encrypted_content: source.signature ?? "",
            ...(source.page_age ? { page_age: source.page_age } : {}),
          })),
    };
  }
  return {
    type: "tool_result",
    tool_use_id: part.tool_call_id,
    content:
      part.result.content.length === 0 && part.status === "cancelled"
        ? CANCELLED_TOOL_RESULT_FALLBACK_CONTENT
        : part.result.content.map((part) => {
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
    is_error: part.status !== "completed",
  };
}

function convertToAnthropicToolSearchResultContent(
  result: ToolSearchToolResult,
): Anthropic.Messages.ToolSearchToolResultBlockParam["content"] {
  if (result.error_code) {
    return {
      type: "tool_search_tool_result_error",
      error_code:
        result.error_code as Anthropic.Messages.ToolSearchToolResultErrorCode,
    };
  }
  return {
    type: "tool_search_tool_search_result",
    tool_references: result.tool_names.map((toolName) => ({
      type: "tool_reference",
      tool_name: toolName,
    })),
  };
}

function convertToAnthropicThinkingBlockParam(
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

function convertToAnthropicTool(tool: Tool): Anthropic.Messages.ToolUnion {
  if (tool.type === "web_search") {
    // Use Anthropic's basic hosted-search version: it supports both common
    // options without enabling the newer code-execution filtering flow.
    const webSearchTool: Anthropic.Messages.WebSearchTool20250305 = {
      type: "web_search_20250305",
      name: "web_search",
    };
    if (tool.allowed_domains) {
      webSearchTool.allowed_domains = tool.allowed_domains;
    }
    if (tool.max_uses !== undefined) {
      webSearchTool.max_uses = tool.max_uses;
    }
    if (tool.user_location) {
      webSearchTool.user_location = {
        type: "approximate",
        ...tool.user_location,
      };
    }
    return webSearchTool;
  }

  if (tool.type === "tool_search") {
    return tool.strategy === "regex"
      ? {
          type: "tool_search_tool_regex_20251119",
          name: "tool_search_tool_regex",
        }
      : {
          type: "tool_search_tool_bm25_20251119",
          name: "tool_search_tool_bm25",
        };
  }

  const functionTool: Anthropic.Messages.Tool = {
    name: tool.name,
    description: tool.description,
    input_schema: tool.parameters as Anthropic.Tool.InputSchema,
    strict: true,
  };
  if (tool.defer_loading) {
    functionTool.defer_loading = true;
  }
  return functionTool;
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

function convertToAnthropicOutputConfig(
  responseFormat: ResponseFormatOption,
): Anthropic.Messages.OutputConfig | null {
  switch (responseFormat.type) {
    case "text":
      return null;
    case "json":
      if (!responseFormat.schema) {
        return null;
      }
      return {
        format: {
          type: "json_schema",
          schema: responseFormat.schema,
        },
      };
  }
}

function convertToAnthropicThinkingConfigParam(
  reasoning: ReasoningOptions,
): Anthropic.ThinkingConfigParam {
  if (!reasoning.enabled) {
    return {
      type: "disabled",
    };
  }
  // Without an explicit token budget, let Anthropic choose the thinking depth.
  if (reasoning.budget_tokens === undefined) {
    return {
      type: "adaptive",
    };
  }
  return {
    type: "enabled",
    budget_tokens: reasoning.budget_tokens,
  };
}

// MARK: To SDK Message

function mapAnthropicMessage(
  contentBlocks: Anthropic.Messages.ContentBlock[],
): Part[] {
  const callStatuses = new Map(
    contentBlocks.flatMap((block) =>
      block.type === "web_search_tool_result" ||
      block.type === "tool_search_tool_result"
        ? [[block.tool_use_id, anthropicServerToolResultStatus(block)] as const]
        : [],
    ),
  );
  return contentBlocks
    .map((block) => mapAnthropicBlock(block, callStatuses))
    .filter((b) => !!b);
}

function mapAnthropicBlock(
  block: Anthropic.Messages.ContentBlock,
  callStatuses = new Map<string, "completed" | "failed">(),
): Part | null {
  switch (block.type) {
    case "text":
      return mapAnthropicTextBlock(block);
    case "tool_use":
      return {
        type: "tool-call",
        tool_call_id: block.id,
        call: {
          type: "function",
          name: block.name,
          args: block.input as Record<string, unknown>,
        },
      };
    case "server_tool_use": {
      if (
        block.name === "tool_search_tool_regex" ||
        block.name === "tool_search_tool_bm25"
      ) {
        return {
          type: "tool-call",
          tool_call_id: block.id,
          call: {
            type: "tool_search",
            args: isRecord(block.input) ? block.input : {},
            status: callStatuses.get(block.id) ?? "in_progress",
          },
        };
      }
      if (block.name !== "web_search") return null;
      const input = block.input as { query?: unknown };
      return {
        type: "tool-call",
        tool_call_id: block.id,
        call: {
          type: "web_search",
          status: callStatuses.get(block.id) ?? "in_progress",
          ...(typeof input.query === "string"
            ? { action: { type: "search" as const, queries: [input.query] } }
            : {}),
        },
      };
    }
    case "tool_search_tool_result": {
      const isError = block.content.type === "tool_search_tool_result_error";
      return {
        type: "tool-result",
        tool_call_id: block.tool_use_id,
        result: {
          type: "tool_search",
          tool_names:
            block.content.type === "tool_search_tool_search_result"
              ? block.content.tool_references.map(
                  (reference) => reference.tool_name,
                )
              : [],
          ...(block.content.type === "tool_search_tool_result_error"
            ? { error_code: block.content.error_code }
            : {}),
        },
        status: isError ? "failed" : "completed",
      };
    }
    case "web_search_tool_result": {
      const isError = !Array.isArray(block.content);
      return {
        type: "tool-result",
        tool_call_id: block.tool_use_id,
        result: {
          type: "web_search",
          sources: Array.isArray(block.content)
            ? block.content.map((source) => ({
                url: source.url,
                title: source.title,
                signature: source.encrypted_content,
                ...(source.page_age ? { page_age: source.page_age } : {}),
              }))
            : [],
          ...(!Array.isArray(block.content)
            ? { error_code: block.content.error_code }
            : {}),
        },
        status: isError ? "failed" : "completed",
      };
    }
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

function anthropicServerToolResultStatus(
  block:
    | Anthropic.Messages.WebSearchToolResultBlock
    | Anthropic.Messages.ToolSearchToolResultBlock,
): "completed" | "failed" {
  if (block.type === "tool_search_tool_result") {
    return block.content.type === "tool_search_tool_result_error"
      ? "failed"
      : "completed";
  }
  return Array.isArray(block.content) ? "completed" : "failed";
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
        if (textCitation.type === "web_search_result_location") {
          const citation: Citation = {
            source: textCitation.url,
            cited_text: textCitation.cited_text,
            signature: textCitation.encrypted_index,
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
    if (partDelta.call.type === "function") partDelta.call.args = "";
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
        call: { type: "function", args: delta.partial_json },
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
      if (delta.citation.type === "web_search_result_location") {
        const citation: CitationDelta = {
          type: "citation",
          source: delta.citation.url,
          cited_text: delta.citation.cited_text,
          signature: delta.citation.encrypted_index,
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

/**
 * The usage fields shared by `message_start` and the cumulative `message_delta`
 * events. Fields of later events overwrite earlier values when present.
 */
type AnthropicUsageLike = Partial<{
  input_tokens: number | null;
  output_tokens: number | null;
  cache_read_input_tokens: number | null;
  cache_creation_input_tokens: number | null;
  cache_creation: Anthropic.Messages.CacheCreation | null;
  output_tokens_details: Anthropic.Messages.OutputTokensDetails | null;
  server_tool_use: Anthropic.Messages.ServerToolUsage | null;
}>;

function mergeAnthropicUsage(
  current: AnthropicUsageLike,
  incoming: AnthropicUsageLike,
): AnthropicUsageLike {
  const result: AnthropicUsageLike = { ...current };
  for (const key of Object.keys(incoming) as (keyof AnthropicUsageLike)[]) {
    const value = incoming[key];
    if (value !== undefined && value !== null) {
      (result as Record<string, unknown>)[key] = value;
    }
  }
  return result;
}

/**
 * Anthropic reports `input_tokens` without the cached and cache-write tokens.
 * The numbers are kept as reported; the cost calculation accounts for it.
 */
function mapAnthropicUsage(usage: AnthropicUsageLike): ModelUsage {
  const result: ModelUsage = {
    input_tokens: usage.input_tokens ?? 0,
    output_tokens: usage.output_tokens ?? 0,
  };
  const inputDetails: ModelTokensDetails = {};
  if (typeof usage.cache_read_input_tokens === "number") {
    inputDetails.cached_tokens = usage.cache_read_input_tokens;
  }
  if (typeof usage.cache_creation_input_tokens === "number") {
    inputDetails.cache_write_tokens = usage.cache_creation_input_tokens;
  }
  if (
    usage.cache_creation &&
    usage.cache_creation.ephemeral_1h_input_tokens > 0
  ) {
    inputDetails.extended_cache_write_tokens =
      usage.cache_creation.ephemeral_1h_input_tokens;
  }
  if (Object.keys(inputDetails).length > 0) {
    result.input_tokens_details = inputDetails;
  }
  if (typeof usage.output_tokens_details?.thinking_tokens === "number") {
    result.output_tokens_details = {
      reasoning_tokens: usage.output_tokens_details.thinking_tokens,
    };
  }
  if (usage.server_tool_use && usage.server_tool_use.web_search_requests > 0) {
    result.server_tool_use = {
      web_search_requests: usage.server_tool_use.web_search_requests,
    };
  }
  return result;
}
