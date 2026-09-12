/**
 * Loosely describe audio format. Some values (e.g., 'wav') denote containers; others (e.g., 'linear16') specify encoding only; cannot describe containers that can contain different audio encodings.
 */
export type AudioFormat =
  "wav" | "mp3" | "linear16" | "flac" | "mulaw" | "alaw" | "aac" | "opus";
/**
 * A part of the message.
 */
export type Part =
  | TextPart
  | ImagePart
  | AudioPart
  | FilePart
  | SourcePart
  | ToolCallPart
  | ToolResultPart
  | ReasoningPart;
/**
 * Delta parts used in partial updates.
 */
export type PartDelta =
  | TextPartDelta
  | ToolCallPartDelta
  | ToolResultPartDelta
  | ImagePartDelta
  | AudioPartDelta
  | ReasoningPartDelta;
/**
 * A message in an LLM conversation history.
 */
export type Message = UserMessage | AssistantMessage | ToolMessage;
/**
 * Defines the modality of content (e.g., text or audio) in LLM responses.
 */
export type Modality = "text" | "image" | "audio";
/**
 * The terminal status of a tool call.
 */
export type ToolResultStatus = "completed" | "failed" | "cancelled";
/**
 * Determines how the model should choose which tool to use:
 * - "auto": The model will automatically choose the tool to use or not use any tools.
 * - "none": The model will not use any tools.
 * - "required": The model will be forced to use a tool.
 * - { type: "tool", toolName: "toolName" }: The model will use the specified tool.
 */
export type ToolChoiceOption =
  ToolChoiceAuto | ToolChoiceNone | ToolChoiceRequired | ToolChoiceTool;
/**
 * The format that the model must output.
 */
export type ResponseFormatOption = ResponseFormatText | ResponseFormatJson;
/**
 * The capabilities supported by the model.
 */
export interface LanguageModelCapabilities {
  text_input: boolean;
  text_output: boolean;
  image_input: boolean;
  image_output: boolean;
  audio_input: boolean;
  audio_output: boolean;
  function_calling: boolean;
  structured_output: boolean;
  citation: boolean;
  reasoning: boolean;
}

/**
 * A part of the message that contains text.
 */
export interface TextPart {
  type: "text";
  text: string;
  citations?: Citation[];
  /**
   * An opaque provider signature used to preserve text-part continuity when returning the part to the same provider.
   */
  signature?: string;
}
/**
 * A part of the message that contains an image.
 */
export interface ImagePart {
  type: "image";
  /**
   * The MIME type of the image. E.g. "image/jpeg", "image/png".
   */
  mime_type: string;
  /**
   * Base64 content; either `data` or `url` must be provided.
   */
  data?: string;
  /**
   * Fetched by the provider; URL support depends on the model.
   */
  url?: string;
  /**
   * The width of the image in pixels.
   */
  width?: number;
  /**
   * The height of the image in pixels.
   */
  height?: number;
  /**
   * ID of the image part, if applicable
   */
  id?: string;
}
/**
 * A part of the message that contains an audio.
 */
export interface AudioPart {
  type: "audio";
  /**
   * Base64 content; either `data` or `url` must be provided.
   */
  data?: string;
  /**
   * Fetched by the provider; URL support depends on the model.
   */
  url?: string;
  format: AudioFormat;
  /**
   * The sample rate of the audio. E.g. 44100, 48000.
   */
  sample_rate?: number;
  /**
   * The number of channels of the audio. E.g. 1, 2.
   */
  channels?: number;
  /**
   * The transcript of the audio.
   */
  transcript?: string;
  /**
   * ID of the audio part, if applicable
   */
  id?: string;
}
/**
 * Document or video input; accepted MIME types depend on the model.
 */
export interface FilePart {
  type: "file";
  /**
   * The MIME type of the file. E.g. "application/pdf", "text/plain", "video/mp4".
   */
  mime_type: string;
  /**
   * The file contents in the format accepted by the model.
   * Either `data` or `url` must be provided.
   */
  data?: string;
  /**
   * Fetched by the provider; URL support depends on the model.
   */
  url?: string;
  /**
   * Document name; some models require it for inline files.
   */
  filename?: string;
}
/**
 * A part of the message that contains a source with structured content.
 * It will be used for citation for supported models.
 */
export interface SourcePart {
  type: "source";
  /**
   * The URL or identifier of the document.
   */
  source: string;
  /**
   * The title of the document.
   */
  title: string;
  /**
   * The content of the document.
   */
  content: Part[];
}
/**
 * A part of the message that represents a call to a tool the model wants to use.
 */
export interface ToolCallPart {
  type: "tool-call";
  /**
   * The ID of the tool call, used to match the tool result with the tool call.
   */
  tool_call_id: string;
  call: ToolCall;
  /**
   * The provider-specific signature used to preserve reasoning/tool continuity.
   */
  signature?: string;
  /**
   * The ID of the tool call part, if applicable.
   * This is different from tool_call_id which is used to match tool results.
   */
  id?: string;
}
export type ToolCall =
  FunctionToolCall | WebSearchToolCall | ToolSearchToolCall;
export interface FunctionToolCall {
  type: "function";
  name: string;
  args: Record<string, unknown>;
}
export type WebSearchToolCallStatus =
  "in_progress" | "searching" | "completed" | "failed";
export type WebSearchAction =
  | { type: "search"; queries: string[] }
  | { type: "open_page"; url: string }
  | { type: "find_in_page"; url: string; pattern: string };
export interface WebSearchToolCall {
  type: "web_search";
  action?: WebSearchAction;
  status?: WebSearchToolCallStatus;
}
export type ToolSearchToolCallStatus = "in_progress" | "completed" | "failed";
/**
 * A provider-hosted search over the deferred tools of the request.
 */
export interface ToolSearchToolCall {
  type: "tool_search";
  /**
   * Opaque search arguments; preserve for conversation replay.
   */
  args: Record<string, unknown>;
  status?: ToolSearchToolCallStatus;
}
/**
 * A part of the message that represents the result of a tool call.
 */
export interface ToolResultPart {
  type: "tool-result";
  /**
   * The ID of the tool call from previous assistant message.
   */
  tool_call_id: string;
  result: ToolResult;
  /**
   * The terminal status of the tool call.
   */
  status: ToolResultStatus;
}
export type ToolResult =
  FunctionToolResult | WebSearchToolResult | ToolSearchToolResult;
export interface FunctionToolResult {
  type: "function";
  name: string;
  content: Part[];
}
export interface WebSearchSource {
  url: string;
  title?: string;
  page_age?: string;
  /** Opaque provider data required to replay this source to the same provider. */
  signature?: string;
}
export interface WebSearchToolResult {
  type: "web_search";
  sources: WebSearchSource[];
  /** Provider error code required to replay a failed hosted-search result. */
  error_code?: string;
}
/**
 * Discovered tools made available to the model.
 */
export interface ToolSearchToolResult {
  type: "tool_search";
  /**
   * Discovered tool names; each must be declared in the request's `tools`.
   */
  tool_names: string[];
  /** Provider error code required to replay a failed hosted tool search. */
  error_code?: string;
}
/**
 * A part of the message that represents the model reasoning.
 */
export interface ReasoningPart {
  type: "reasoning";
  /**
   * The reasoning text content
   */
  text: string;
  /**
   * The reasoning internal signature
   */
  signature?: string;
  /**
   * The ID of the reasoning part, if applicable
   */
  id?: string;
}

/**
 * Represents a citation for a part.
 */
export interface Citation {
  /**
   * The URL or identifier of the document being cited.
   */
  source: string;

  /**
   * The title of the document being cited.
   */
  title?: string;

  /**
   * The text snippet from the document being cited.
   */
  cited_text?: string;

  /**
   * The start index of the document content part being cited.
   */
  start_index?: number;
  /**
   * The end index of the document content part being cited.
   */
  end_index?: number;

  /**
   * An opaque provider signature used to preserve citation continuity when returning it to the same provider.
   */
  signature?: string;
}
/**
 * Represents a message sent by the user.
 */
export interface UserMessage {
  role: "user";
  content: Part[];
}
/**
 * Represents a message generated by the model.
 */
export interface AssistantMessage {
  role: "assistant";
  content: Part[];
}
/**
 * A delta update for a text part, used in streaming or incremental updates of a message.
 */
export interface TextPartDelta {
  type: "text";
  text: string;
  citation?: CitationDelta;
  /**
   * An opaque provider signature used to preserve text-part continuity when returning the part to the same provider.
   */
  signature?: string;
}
/**
 * A delta update for a tool call part, used in streaming of a tool invocation.
 */
export interface ToolCallPartDelta {
  type: "tool-call";
  /**
   * The ID of the tool call, used to match the tool result with the tool call.
   */
  tool_call_id?: string;
  call: ToolCallDelta;
  /**
   * The provider-specific signature used to preserve reasoning/tool continuity.
   */
  signature?: string;
  /**
   * The ID of the tool call part, if applicable.
   * This is different from tool_call_id which is used to match tool results.
   */
  id?: string;
}
export type ToolCallDelta =
  FunctionToolCallDelta | WebSearchToolCallDelta | ToolSearchToolCallDelta;
export interface FunctionToolCallDelta {
  type: "function";
  name?: string;
  /** The partial JSON string of the arguments to pass to the tool. */
  args?: string;
}
export interface WebSearchToolCallDelta {
  type: "web_search";
  action?: WebSearchAction;
  status?: WebSearchToolCallStatus;
}
export interface ToolSearchToolCallDelta {
  type: "tool_search";
  /** The partial JSON string of the search arguments. */
  args?: string;
  status?: ToolSearchToolCallStatus;
}
/** An atomic delta containing a hosted or client tool result. */
export interface ToolResultPartDelta {
  type: "tool-result";
  tool_call_id: string;
  result: ToolResult;
  status: ToolResultStatus;
}
/**
 * A delta update for an image part, used in streaming of an image message.
 */
export interface ImagePartDelta {
  type: "image";
  /**
   * The MIME type of the image. E.g. "image/jpeg", "image/png".
   */
  mime_type?: string;
  /**
   * The base64-encoded image data.
   */
  data?: string;
  /**
   * The width of the image in pixels.
   */
  width?: number;
  /**
   * The height of the image in pixels.
   */
  height?: number;
  /**
   * ID of the image part, if applicable
   */
  id?: string;
}
/**
 * A delta update for an audio part, used in streaming of an audio message.
 */
export interface AudioPartDelta {
  type: "audio";
  /**
   * The base64-encoded audio data.
   */
  data?: string;
  format?: AudioFormat;
  /**
   * The sample rate of the audio. E.g. 44100, 48000.
   */
  sample_rate?: number;
  /**
   * The number of channels of the audio. E.g. 1, 2.
   */
  channels?: number;
  /**
   * The transcript of the audio.
   */
  transcript?: string;
  /**
   * The ID of the audio part, if applicable
   */
  id?: string;
}
/**
 * A delta update for a reasoning part, used in streaming of reasoning messages.
 */
export interface ReasoningPartDelta {
  type: "reasoning";
  /**
   * The reasoning text content
   */
  text: string;
  /**
   * The reasoning internal signature
   */
  signature?: string;
  /**
   * The ID of the reasoning part, if applicable
   */
  id?: string;
}
/**
 * A delta update for a citation part, used in streaming of citation messages.
 */
export interface CitationDelta {
  type: "citation";
  /**
   * The URL or identifier of the document being cited.
   */
  source?: string;

  /**
   * The title of the document being cited.
   */
  title?: string;

  /**
   * The text snippet from the document being cited.
   */
  cited_text?: string;
  /**
   * The start index of the document content part being cited.
   */
  start_index?: number;
  /**
   * The end index of the document content part being cited.
   */
  end_index?: number;
  /**
   * An opaque provider signature used to preserve citation continuity when returning it to the same provider.
   */
  signature?: string;
}
/**
 * Represents a delta update in a message's content, enabling partial streaming updates in LLM responses.
 */
export interface ContentDelta {
  index: number;
  part: PartDelta;
}
/**
 * Represents a JSON schema.
 */
export type JSONSchema = Record<string, unknown>;
/**
 * Represents a tool that can be used by the model.
 */
export type Tool = FunctionTool | WebSearchTool | ToolSearchTool;
/**
 * Represents a client-executed function tool that can be used by the model.
 */
export interface FunctionTool {
  type: "function";
  /**
   * The name of the tool.
   */
  name: string;
  /**
   * A description of the tool.
   */
  description: string;
  /**
   * The JSON schema of the parameters that the tool accepts. The type must be "object".
   */
  parameters: JSONSchema;
  /**
   * Hide the tool from the model until a `tool_search` tool discovers it.
   * Providers without tool search ignore this flag and load the tool eagerly.
   */
  defer_loading?: boolean;
}
/**
 * Represents a provider-hosted web search tool.
 */
export interface WebSearchTool {
  type: "web_search";
  /**
   * Restricts search results to these domains when supported by the provider.
   */
  allowed_domains?: string[];
  /**
   * Limits the number of searches the provider may perform when supported.
   */
  max_uses?: number;
  /**
   * An approximate user location used to localize web search results.
   */
  user_location?: WebSearchUserLocation;
}
/**
 * Loads deferred function tools on demand through hosted search.
 */
export interface ToolSearchTool {
  type: "tool_search";
  /**
   * The search algorithm, when the provider offers a choice. Defaults to "bm25".
   */
  strategy?: ToolSearchStrategy;
}
export type ToolSearchStrategy = "regex" | "bm25";
/**
 * An approximate user location used to localize web search results.
 */
export interface WebSearchUserLocation {
  /**
   * The city of the user.
   */
  city?: string;
  /**
   * The region or state of the user.
   */
  region?: string;
  /**
   * The two-letter ISO 3166-1 country code of the user.
   */
  country?: string;
  /**
   * The IANA timezone of the user.
   */
  timezone?: string;
}
/**
 * Represents tool result in the message history.
 * Only ToolResultPart should be included in the content.
 */
export interface ToolMessage {
  role: "tool";
  content: Part[];
}
/**
 * A breakdown of `input_tokens` or `output_tokens`, using the provider's own counting.
 */
export interface ModelTokensDetails {
  text_tokens?: number;
  cached_text_tokens?: number;
  audio_tokens?: number;
  cached_audio_tokens?: number;
  image_tokens?: number;
  cached_image_tokens?: number;
  /**
   * Cache reads, billed at the cached-input rate.
   */
  cached_tokens?: number;
  /**
   * Cache writes, billed separately from cache reads.
   */
  cache_write_tokens?: number;
  /**
   * The subset of `cache_write_tokens` stored with extended retention (see `cache_retention`), which some providers bill at a higher rate.
   */
  extended_cache_write_tokens?: number;
  /**
   * The tokens spent on reasoning.
   */
  reasoning_tokens?: number;
}
/**
 * Hosted tool usage billed per request, separately from tokens.
 */
export interface ModelServerToolUsage {
  web_search_requests?: number;
}
/**
 * Represents the token usage of the model.
 */
export interface ModelUsage {
  /**
   * The input tokens as reported by the provider. Whether cached and
   * cache-write tokens are included depends on the provider; see `ModelUsageCostOptions`.
   */
  input_tokens: number;
  /**
   * The output tokens as reported by the provider. Whether reasoning tokens
   * are included depends on the provider; see `ModelUsageCostOptions`.
   */
  output_tokens: number;
  input_tokens_details?: ModelTokensDetails;
  output_tokens_details?: ModelTokensDetails;
  server_tool_use?: ModelServerToolUsage;
}
/**
 * Represents the response generated by the model.
 */
export interface ModelResponse {
  content: Part[];
  usage?: ModelUsage;
  /**
   * The cost of the response.
   */
  cost?: number;
}
/**
 * Represents a partial response from the language model, useful for streaming output via async generator.
 */
export interface PartialModelResponse {
  delta?: ContentDelta;
  usage?: ModelUsage;
  cost?: number;
}
/**
 * The model will automatically choose the tool to use or not use any tools.
 */
export interface ToolChoiceAuto {
  type: "auto";
}
/**
 * The model will not use any tools.
 */
export interface ToolChoiceNone {
  type: "none";
}
/**
 * The model will be forced to use a tool.
 */
export interface ToolChoiceRequired {
  type: "required";
}
/**
 * The model will use the specified tool.
 */
export interface ToolChoiceTool {
  type: "tool";
  tool_name: string;
}
/**
 * Specifies that the model response should be in plain text format.
 */
export interface ResponseFormatText {
  type: "text";
}
/**
 * Specifies that the model response should be in JSON format adhering to a specified schema.
 */
export interface ResponseFormatJson {
  type: "json";
  /**
   * The name of the schema.
   */
  name: string;
  /**
   * The description of the schema.
   */
  description?: string;
  schema?: JSONSchema;
}
/**
 * Options for audio generation.
 */
export interface AudioOptions {
  /**
   * The desired audio format.
   */
  format?: AudioFormat;
  /**
   * The provider-specific voice ID to use for audio generation.
   */
  voice?: string;
  /**
   * The language code for the audio generation.
   */
  language?: string;
}
/**
 * Options for reasoning generation.
 */
export interface ReasoningOptions {
  /**
   * Whether to enable reasoning output.
   */
  enabled: boolean;
  /**
   * Specify the budget tokens for reasoning generation.
   */
  budget_tokens?: number;
}
/**
 * Defines the input parameters for the language model completion.
 */
export interface LanguageModelInput {
  /**
   * A system prompt is a way of providing context and instructions to the model
   */
  system_prompt?: string;
  /**
   * A list of messages comprising the conversation so far.
   */
  messages: Message[];
  /**
   * Definitions of tools that the model may use.
   */
  tools?: Tool[];
  tool_choice?: ToolChoiceOption;
  response_format?: ResponseFormatOption;
  /**
   * The maximum number of tokens that can be generated in the chat completion.
   */
  max_tokens?: number;
  /**
   * Amount of randomness injected into the response. Ranges from 0.0 to 1.0
   */
  temperature?: number;
  /**
   * An alternative to sampling with temperature, called nucleus sampling, where the model considers the results of the tokens with top_p probability mass. Ranges from 0.0 to 1.0
   */
  top_p?: number;
  /**
   * Only sample from the top K options for each subsequent token. Used to remove 'long tail' low probability responses. Must be a non-negative integer.
   */
  top_k?: number;
  /**
   * Positive values penalize new tokens based on whether they appear in the text so far, increasing the model's likelihood to talk about new topics.
   */
  presence_penalty?: number;
  /**
   * Positive values penalize new tokens based on their existing frequency in the text so far, decreasing the model's likelihood to repeat the same line verbatim.
   */
  frequency_penalty?: number;
  /**
   * The seed (integer), if set and supported by the model, to enable deterministic results.
   */
  seed?: number;
  /**
   * The modalities that the model should support.
   */
  modalities?: Modality[];
  /**
   * Options for audio generation.
   */
  audio?: AudioOptions;
  /**
   * Options for reasoning generation.
   */
  reasoning?: ReasoningOptions;
  /**
   * Opts into the provider's prompt cache and chooses how long entries are kept.
   * "standard" uses the provider's default retention and "extended" the longest it offers.
   * Providers without a retention setting ignore this option.
   */
  cache_retention?: CacheRetention;
  /**
   * A set of key/value pairs that store additional information about the request. This is forwarded to the model provider if supported.
   */
  metadata?: Record<string, string>;
}
/**
 * How long prompt cache entries are kept.
 */
export type CacheRetention = "standard" | "extended";
/**
 * A metadata property that describes the pricing of the model.
 */
export interface LanguageModelPricing {
  /**
   * The cost in USD per single text token for input.
   */
  input_cost_per_text_token?: number;
  /**
   * The cost in USD per single cached input token.
   */
  input_cost_per_cached_token?: number;
  /**
   * The cost in USD per single cache-write input token.
   */
  input_cost_per_cache_write_token?: number;
  /**
   * The cost in USD per single cache-write input token stored with extended retention. Defaults to `input_cost_per_cache_write_token`.
   */
  input_cost_per_extended_cache_write_token?: number;
  /**
   * The cost in USD per single cached text token for input.
   */
  input_cost_per_cached_text_token?: number;
  /**
   * The cost in USD per single text token for output.
   */
  output_cost_per_text_token?: number;
  /**
   * The cost in USD per single audio token for input.
   */
  input_cost_per_audio_token?: number;
  /**
   * The cost in USD per single cached audio token for input.
   */
  input_cost_per_cached_audio_token?: number;
  /**
   * The cost in USD per single audio token for output.
   */
  output_cost_per_audio_token?: number;
  /**
   * The cost in USD per single image token for input.
   */
  input_cost_per_image_token?: number;
  /**
   * The cost in USD per single cached image token for input.
   */
  input_cost_per_cached_image_token?: number;
  /**
   * The cost in USD per single image token for output.
   */
  output_cost_per_image_token?: number;
  /**
   * The cost in USD per provider-hosted web search request.
   */
  cost_per_web_search_request?: number;
  /**
   * Rate multipliers applied to requests whose input exceeds a token threshold.
   */
  long_context?: LanguageModelLongContextPricing;
}
/**
 * Rate multipliers applied to requests whose input exceeds a token threshold.
 */
export interface LanguageModelLongContextPricing {
  /**
   * The request is priced with the multipliers when `input_tokens` exceeds this value.
   */
  threshold_tokens: number;
  /**
   * The multiplier applied to every input rate. Defaults to 1.
   */
  input_cost_multiplier?: number;
  /**
   * The multiplier applied to every output rate. Defaults to 1.
   */
  output_cost_multiplier?: number;
}
