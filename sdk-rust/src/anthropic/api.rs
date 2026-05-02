#![allow(clippy::enum_variant_names)]
#![allow(clippy::struct_field_names)]
#![allow(clippy::doc_markdown)]
#![allow(clippy::too_many_lines)]

use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::HashMap;

#[derive(Serialize, Deserialize)]
pub struct CreateMessageParams {
    /// Top-level cache control automatically applies a cache_control marker to
    /// the last cacheable block in the request.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<CreateMessageParamsCacheControl>,
    /// Container identifier for reuse across requests.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub container: Option<String>,
    /// Specifies the geographic region for inference processing. If not
    /// specified, the workspace's `default_inference_geo` is used.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub inference_geo: Option<String>,
    /// The maximum number of tokens to generate before stopping.
    ///
    /// Note that our models may stop _before_ reaching this maximum. This
    /// parameter only specifies the absolute maximum number of tokens to
    /// generate.
    ///
    /// Different models have different maximum values for this parameter.  See [models](https://docs.claude.com/en/docs/models-overview) for details.
    pub max_tokens: i64,
    /// Input messages.
    ///
    /// Our models are trained to operate on alternating `user` and `assistant`
    /// conversational turns. When creating a new `Message`, you specify the
    /// prior conversational turns with the `messages` parameter, and the model
    /// then generates the next `Message` in the conversation. Consecutive
    /// `user` or `assistant` turns in your request will be combined into a
    /// single turn.
    ///
    /// Each input message must be an object with a `role` and `content`. You
    /// can specify a single `user`-role message, or you can include multiple
    /// `user` and `assistant` messages.
    ///
    /// If the final message uses the `assistant` role, the response content
    /// will continue immediately from the content in that message. This can be
    /// used to constrain part of the model's response.
    ///
    /// Example with a single `user` message:
    ///
    /// ```json
    /// [{"role": "user", "content": "Hello, Claude"}]
    /// ```
    ///
    /// Example with multiple conversational turns:
    ///
    /// ```json
    /// [
    ///   {"role": "user", "content": "Hello there."},
    ///   {"role": "assistant", "content": "Hi, I'm Claude. How can I help you?"},
    ///   {"role": "user", "content": "Can you explain LLMs in plain English?"},
    /// ]
    /// ```
    ///
    /// Example with a partially-filled response from Claude:
    ///
    /// ```json
    /// [
    ///   {"role": "user", "content": "What's the Greek name for Sun? (A) Sol (B) Helios (C) Sun"},
    ///   {"role": "assistant", "content": "The best answer is ("},
    /// ]
    /// ```
    ///
    /// Each input message `content` may be either a single `string` or an array
    /// of content blocks, where each block has a specific `type`. Using a
    /// `string` for `content` is shorthand for an array of one content block of
    /// type `"text"`. The following input messages are equivalent:
    ///
    /// ```json
    /// {"role": "user", "content": "Hello, Claude"}
    /// ```
    ///
    /// ```json
    /// {"role": "user", "content": [{"type": "text", "text": "Hello, Claude"}]}
    /// ```
    ///
    /// See [input examples](https://docs.claude.com/en/api/messages-examples).
    ///
    /// Note that if you want to include a [system prompt](https://docs.claude.com/en/docs/system-prompts), you can use the top-level `system` parameter — there is no `"system"` role for input messages in the Messages API.
    ///
    /// There is a limit of 100,000 messages in a single request.
    pub messages: Vec<InputMessage>,
    /// An object describing metadata about the request.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub metadata: Option<Metadata>,
    pub model: Model,
    /// Configuration options for the model's output, such as the output format.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output_config: Option<OutputConfig>,
    /// Determines whether to use priority capacity (if available) or standard
    /// capacity for this request.
    ///
    /// Anthropic offers different levels of service for your API requests. See [service-tiers](https://docs.claude.com/en/api/service-tiers) for details.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub service_tier: Option<CreateMessageParamsServiceTier>,
    /// Custom text sequences that will cause the model to stop generating.
    ///
    /// Our models will normally stop when they have naturally completed their
    /// turn, which will result in a response `stop_reason` of `"end_turn"`.
    ///
    /// If you want the model to stop generating when it encounters custom
    /// strings of text, you can use the `stop_sequences` parameter. If the
    /// model encounters one of the custom sequences, the response `stop_reason`
    /// value will be `"stop_sequence"` and the response `stop_sequence` value
    /// will contain the matched stop sequence.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub stop_sequences: Option<Vec<String>>,
    /// Whether to incrementally stream the response using server-sent events.
    ///
    /// See [streaming](https://docs.claude.com/en/api/messages-streaming) for details.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub stream: Option<bool>,
    /// System prompt.
    ///
    /// A system prompt is a way of providing context and instructions to Claude, such as specifying a particular goal or role. See our [guide to system prompts](https://docs.claude.com/en/docs/system-prompts).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub system: Option<CreateMessageParamsSystem>,
    /// Amount of randomness injected into the response.
    ///
    /// Defaults to `1.0`. Ranges from `0.0` to `1.0`. Use `temperature` closer
    /// to `0.0` for analytical / multiple choice, and closer to `1.0` for
    /// creative and generative tasks.
    ///
    /// Note that even with `temperature` of `0.0`, the results will not be
    /// fully deterministic.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub temperature: Option<f64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub thinking: Option<ThinkingConfigParam>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_choice: Option<ToolChoice>,
    /// Definitions of tools that the model may use.
    ///
    /// If you include `tools` in your API request, the model may return
    /// `tool_use` content blocks that represent the model's use of those tools.
    /// You can then run those tools using the tool input generated by the model
    /// and then optionally return results back to the model using `tool_result`
    /// content blocks.
    ///
    /// There are two types of tools: **client tools** and **server tools**. The behavior described below applies to client tools. For [server tools](https://docs.claude.com/en/docs/agents-and-tools/tool-use/overview\#server-tools), see their individual documentation as each has its own behavior (e.g., the [web search tool](https://docs.claude.com/en/docs/agents-and-tools/tool-use/web-search-tool)).
    ///
    /// Each tool definition includes:
    ///
    /// * `name`: Name of the tool.
    /// * `description`: Optional, but strongly-recommended description of the
    ///   tool.
    /// * `input_schema`: [JSON schema](https://json-schema.org/draft/2020-12)
    ///   for the tool `input` shape that the model will produce in `tool_use`
    ///   output content blocks.
    ///
    /// For example, if you defined `tools` as:
    ///
    /// ```json
    /// [
    ///   {
    ///     "name": "get_stock_price",
    ///     "description": "Get the current stock price for a given ticker symbol.",
    ///     "input_schema": {
    ///       "type": "object",
    ///       "properties": {
    ///         "ticker": {
    ///           "type": "string",
    ///           "description": "The stock ticker symbol, e.g. AAPL for Apple Inc."
    ///         }
    ///       },
    ///       "required": ["ticker"]
    ///     }
    ///   }
    /// ]
    /// ```
    ///
    /// And then asked the model "What's the S&P 500 at today?", the model might
    /// produce `tool_use` content blocks in the response like this:
    ///
    /// ```json
    /// [
    ///   {
    ///     "type": "tool_use",
    ///     "id": "toolu_01D7FLrfh4GYq7yT1ULFeyMV",
    ///     "name": "get_stock_price",
    ///     "input": { "ticker": "^GSPC" }
    ///   }
    /// ]
    /// ```
    ///
    /// You might then run your `get_stock_price` tool with `{"ticker":
    /// "^GSPC"}` as an input, and return the following back to the model in a
    /// subsequent `user` message:
    ///
    /// ```json
    /// [
    ///   {
    ///     "type": "tool_result",
    ///     "tool_use_id": "toolu_01D7FLrfh4GYq7yT1ULFeyMV",
    ///     "content": "259.75 USD"
    ///   }
    /// ]
    /// ```
    ///
    /// Tools can be used for workflows that include running client-side tools
    /// and functions, or more generally whenever you want the model to produce
    /// a particular JSON structure of output.
    ///
    /// See our [guide](https://docs.claude.com/en/docs/tool-use) for more details.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tools: Option<Vec<CreateMessageParamsToolsItem>>,
    /// Only sample from the top K options for each subsequent token.
    ///
    /// Used to remove "long tail" low probability responses. [Learn more technical details here](https://towardsdatascience.com/how-to-sample-from-language-models-682bceb97277).
    ///
    /// Recommended for advanced use cases only. You usually only need to use
    /// `temperature`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub top_k: Option<i64>,
    /// Use nucleus sampling.
    ///
    /// In nucleus sampling, we compute the cumulative distribution over all the
    /// options for each subsequent token in decreasing probability order and
    /// cut it off once it reaches a particular probability specified by
    /// `top_p`. You should either alter `temperature` or `top_p`, but not both.
    ///
    /// Recommended for advanced use cases only. You usually only need to use
    /// `temperature`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub top_p: Option<f64>,
}

/// Top-level cache control automatically applies a cache_control marker to the
/// last cacheable block in the request.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum CreateMessageParamsCacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

/// Determines whether to use priority capacity (if available) or standard
/// capacity for this request.
///
/// Anthropic offers different levels of service for your API requests. See [service-tiers](https://docs.claude.com/en/api/service-tiers) for details.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
pub enum CreateMessageParamsServiceTier {
    #[serde(rename = "auto")]
    Auto,
    #[serde(rename = "standard_only")]
    StandardOnly,
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

pub type CreateMessageParamsSystemString = Option<String>;

pub type CreateMessageParamsSystemArray = Option<Vec<RequestTextBlock>>;

/// System prompt.
///
/// A system prompt is a way of providing context and instructions to Claude, such as specifying a particular goal or role. See our [guide to system prompts](https://docs.claude.com/en/docs/system-prompts).
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(untagged)]
pub enum CreateMessageParamsSystem {
    CreateMessageParamsSystemString(CreateMessageParamsSystemString),
    CreateMessageParamsSystemArray(CreateMessageParamsSystemArray),
    #[allow(dead_code)]
    #[serde(skip_serializing)]
    Unknown(Value),
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(untagged)]
pub enum CreateMessageParamsToolsItem {
    Tool(Tool),
    BashTool20250124(BashTool20250124),
    CodeExecutionTool20250522(CodeExecutionTool20250522),
    CodeExecutionTool20250825(CodeExecutionTool20250825),
    CodeExecutionTool20260120(CodeExecutionTool20260120),
    MemoryTool20250818(MemoryTool20250818),
    TextEditor20250124(TextEditor20250124),
    TextEditor20250429(TextEditor20250429),
    TextEditor20250728(TextEditor20250728),
    WebSearchTool20250305(WebSearchTool20250305),
    WebFetchTool20250910(WebFetchTool20250910),
    WebSearchTool20260209(WebSearchTool20260209),
    WebFetchTool20260209(WebFetchTool20260209),
    WebFetchTool20260309(WebFetchTool20260309),
    ToolSearchToolBM2520251119(ToolSearchToolBM2520251119),
    ToolSearchToolRegex20251119(ToolSearchToolRegex20251119),
    #[allow(dead_code)]
    #[serde(skip_serializing)]
    Unknown(Value),
}

#[derive(Serialize, Deserialize)]
pub struct Message {
    /// Information about the container used in this request.
    ///
    /// This will be non-null if a container tool (e.g. code execution) was
    /// used.
    pub container: Option<Container>,
    /// Content generated by the model.
    ///
    /// This is an array of content blocks, each of which has a `type` that
    /// determines its shape.
    ///
    /// Example:
    ///
    /// ```json
    /// [{"type": "text", "text": "Hi, I'm Claude."}]
    /// ```
    ///
    /// If the request input `messages` ended with an `assistant` turn, then the
    /// response `content` will continue directly from that last turn. You can
    /// use this to constrain the model's output.
    ///
    /// For example, if the input `messages` were:
    /// ```json
    /// [
    ///   {"role": "user", "content": "What's the Greek name for Sun? (A) Sol (B) Helios (C) Sun"},
    ///   {"role": "assistant", "content": "The best answer is ("}
    /// ]
    /// ```
    ///
    /// Then the response `content` might be:
    ///
    /// ```json
    /// [{"type": "text", "text": "B)"}]
    /// ```
    pub content: Vec<ContentBlock>,
    /// Unique object identifier.
    ///
    /// The format and length of IDs may change over time.
    pub id: String,
    pub model: Model,
    /// Conversational role of the generated message.
    ///
    /// This will always be `"assistant"`.
    pub role: String,
    /// Structured information about why model output stopped.
    ///
    /// This is `null` when the `stop_reason` has no additional detail to
    /// report.
    pub stop_details: Option<RefusalStopDetails>,
    /// The reason that we stopped.
    ///
    /// This may be one the following values:
    /// * `"end_turn"`: the model reached a natural stopping point
    /// * `"max_tokens"`: we exceeded the requested `max_tokens` or the model's
    ///   maximum
    /// * `"stop_sequence"`: one of your provided custom `stop_sequences` was
    ///   generated
    /// * `"tool_use"`: the model invoked one or more tools
    /// * `"pause_turn"`: we paused a long-running turn. You may provide the
    ///   response back as-is in a subsequent request to let the model continue.
    /// * `"refusal"`: when streaming classifiers intervene to handle potential
    ///   policy violations
    ///
    /// In non-streaming mode this value is always non-null. In streaming mode,
    /// it is null in the `message_start` event and non-null otherwise.
    pub stop_reason: Option<StopReason>,
    /// Which custom stop sequence was generated, if any.
    ///
    /// This value will be a non-null string if one of your custom stop
    /// sequences was generated.
    pub stop_sequence: Option<String>,
    /// Object type.
    ///
    /// For Messages, this is always `"message"`.
    pub r#type: String,
    /// Billing and rate-limit usage.
    ///
    /// Anthropic's API bills and rate-limits by token counts, as tokens
    /// represent the underlying cost to our systems.
    ///
    /// Under the hood, the API transforms requests into a format suitable for
    /// the model. The model's output then goes through a parsing stage before
    /// becoming an API response. As a result, the token counts in `usage` will
    /// not match one-to-one with the exact visible content of an API request or
    /// response.
    ///
    /// For example, `output_tokens` will be non-zero, even for an empty string
    /// response from Claude.
    ///
    /// Total input tokens in a request is the summation of `input_tokens`,
    /// `cache_creation_input_tokens`, and `cache_read_input_tokens`.
    pub usage: Usage,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum MessageStreamEvent {
    #[serde(rename = "message_start")]
    MessageStart(MessageStartEvent),
    #[serde(rename = "message_delta")]
    MessageDelta(MessageDeltaEvent),
    #[serde(rename = "message_stop")]
    MessageStop(MessageStopEvent),
    #[serde(rename = "content_block_start")]
    ContentBlockStart(ContentBlockStartEvent),
    #[serde(rename = "content_block_delta")]
    ContentBlockDelta(ContentBlockDeltaEvent),
    #[serde(rename = "content_block_stop")]
    ContentBlockStop(ContentBlockStopEvent),
    #[serde(rename = "ping")]
    Ping(PingEvent),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct PingEvent {}

#[derive(Serialize, Deserialize)]
pub struct CacheControlEphemeral {
    /// The time-to-live for the cache control breakpoint.
    ///
    /// This may be one the following values:
    /// - `5m`: 5 minutes
    /// - `1h`: 1 hour
    ///
    /// Defaults to `5m`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub ttl: Option<CacheControlEphemeralTtl>,
}

/// The time-to-live for the cache control breakpoint.
///
/// This may be one the following values:
/// - `5m`: 5 minutes
/// - `1h`: 1 hour
///
/// Defaults to `5m`.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
pub enum CacheControlEphemeralTtl {
    #[serde(rename = "5m")]
    N5M,
    #[serde(rename = "1h")]
    N1H,
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct InputMessage {
    pub content: InputMessageContent,
    pub role: InputMessageRole,
}

pub type InputMessageContentString = Option<String>;

pub type InputMessageContentArray = Option<Vec<InputContentBlock>>;

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(untagged)]
pub enum InputMessageContent {
    InputMessageContentString(InputMessageContentString),
    InputMessageContentArray(InputMessageContentArray),
    #[allow(dead_code)]
    #[serde(skip_serializing)]
    Unknown(Value),
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
pub enum InputMessageRole {
    #[serde(rename = "user")]
    User,
    #[serde(rename = "assistant")]
    Assistant,
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct Metadata {
    /// An external identifier for the user who is associated with the request.
    ///
    /// This should be a uuid, hash value, or other opaque identifier. Anthropic
    /// may use this id to help detect abuse. Do not include any identifying
    /// information such as name, email address, or phone number.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub user_id: Option<String>,
}

/// The model that will complete your prompt.\n\nSee [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.
pub type Model = Option<String>;

#[derive(Serialize, Deserialize)]
pub struct OutputConfig {
    /// How much effort the model should put into its response. Higher effort
    /// levels may result in more thorough analysis but take longer.
    ///
    /// Valid values are `low`, `medium`, `high`, or `max`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub effort: Option<EffortLevel>,
    /// A schema to specify Claude's output format in responses. See [structured outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub format: Option<JsonOutputFormat>,
}

#[derive(Serialize, Deserialize)]
pub struct RequestTextBlock {
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<RequestTextBlockCacheControl>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub citations: Option<Vec<RequestTextBlockCitationsItem>>,
    pub text: String,
    pub r#type: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestTextBlockCacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestTextBlockCitationsItem {
    #[serde(rename = "char_location")]
    CharLocation(RequestCharLocationCitation),
    #[serde(rename = "page_location")]
    PageLocation(RequestPageLocationCitation),
    #[serde(rename = "content_block_location")]
    ContentBlockLocation(RequestContentBlockLocationCitation),
    #[serde(rename = "web_search_result_location")]
    WebSearchResultLocation(RequestWebSearchResultLocationCitation),
    #[serde(rename = "search_result_location")]
    SearchResultLocation(RequestSearchResultLocationCitation),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

/// Configuration for enabling Claude's extended thinking.
///
/// When enabled, responses include `thinking` content blocks showing Claude's
/// thinking process before the final answer. Requires a minimum budget of 1,024
/// tokens and counts towards your `max_tokens` limit.
///
/// See [extended thinking](https://docs.claude.com/en/docs/build-with-claude/extended-thinking) for details.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum ThinkingConfigParam {
    #[serde(rename = "enabled")]
    Enabled(ThinkingConfigEnabled),
    #[serde(rename = "disabled")]
    Disabled(ThinkingConfigDisabled),
    #[serde(rename = "adaptive")]
    Adaptive(ThinkingConfigAdaptive),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

/// How the model should use the provided tools. The model can use a specific
/// tool, any available tool, decide by itself, or not use tools at all.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum ToolChoice {
    #[serde(rename = "auto")]
    Auto(ToolChoiceAuto),
    #[serde(rename = "any")]
    Any(ToolChoiceAny),
    #[serde(rename = "tool")]
    Tool(ToolChoiceTool),
    #[serde(rename = "none")]
    None(ToolChoiceNone),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct Tool {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_callers: Option<Vec<AllowedCaller>>,
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<ToolCacheControl>,
    /// If true, tool will not be included in initial system prompt. Only loaded
    /// when returned via tool_reference from tool search.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub defer_loading: Option<bool>,
    /// Description of what this tool does.
    ///
    /// Tool descriptions should be as detailed as possible. The more
    /// information that the model has about what the tool is and how to use it,
    /// the better it will perform. You can use natural language descriptions to
    /// reinforce important aspects of the tool input JSON schema.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    /// Enable eager input streaming for this tool. When true, tool input
    /// parameters will be streamed incrementally as they are generated, and
    /// types will be inferred on-the-fly rather than buffering the full JSON
    /// output. When false, streaming is disabled for this tool even if the
    /// fine-grained-tool-streaming beta is active. When null (default), uses
    /// the default behavior based on beta headers.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub eager_input_streaming: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_examples: Option<Vec<HashMap<String, JsonValue>>>,
    /// [JSON schema](https://json-schema.org/draft/2020-12) for this tool's input.
    ///
    /// This defines the shape of the `input` that your tool accepts and that
    /// the model will produce.
    pub input_schema: InputSchema,
    /// Name of the tool.
    ///
    /// This is how the tool will be called by the model and in `tool_use`
    /// blocks.
    pub name: String,
    /// When true, guarantees schema validation on tool names and inputs
    #[serde(skip_serializing_if = "Option::is_none")]
    pub strict: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub r#type: Option<String>,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum ToolCacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct BashTool20250124 {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_callers: Option<Vec<AllowedCaller>>,
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<BashTool20250124CacheControl>,
    /// If true, tool will not be included in initial system prompt. Only loaded
    /// when returned via tool_reference from tool search.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub defer_loading: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_examples: Option<Vec<HashMap<String, JsonValue>>>,
    /// Name of the tool.
    ///
    /// This is how the tool will be called by the model and in `tool_use`
    /// blocks.
    pub name: String,
    /// When true, guarantees schema validation on tool names and inputs
    #[serde(skip_serializing_if = "Option::is_none")]
    pub strict: Option<bool>,
    pub r#type: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum BashTool20250124CacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct CodeExecutionTool20250522 {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_callers: Option<Vec<AllowedCaller>>,
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<CodeExecutionTool20250522CacheControl>,
    /// If true, tool will not be included in initial system prompt. Only loaded
    /// when returned via tool_reference from tool search.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub defer_loading: Option<bool>,
    /// Name of the tool.
    ///
    /// This is how the tool will be called by the model and in `tool_use`
    /// blocks.
    pub name: String,
    /// When true, guarantees schema validation on tool names and inputs
    #[serde(skip_serializing_if = "Option::is_none")]
    pub strict: Option<bool>,
    pub r#type: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum CodeExecutionTool20250522CacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct CodeExecutionTool20250825 {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_callers: Option<Vec<AllowedCaller>>,
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<CodeExecutionTool20250825CacheControl>,
    /// If true, tool will not be included in initial system prompt. Only loaded
    /// when returned via tool_reference from tool search.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub defer_loading: Option<bool>,
    /// Name of the tool.
    ///
    /// This is how the tool will be called by the model and in `tool_use`
    /// blocks.
    pub name: String,
    /// When true, guarantees schema validation on tool names and inputs
    #[serde(skip_serializing_if = "Option::is_none")]
    pub strict: Option<bool>,
    pub r#type: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum CodeExecutionTool20250825CacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

/// Code execution tool with REPL state persistence (daemon mode + gVisor
/// checkpoint).
#[derive(Serialize, Deserialize)]
pub struct CodeExecutionTool20260120 {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_callers: Option<Vec<AllowedCaller>>,
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<CodeExecutionTool20260120CacheControl>,
    /// If true, tool will not be included in initial system prompt. Only loaded
    /// when returned via tool_reference from tool search.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub defer_loading: Option<bool>,
    /// Name of the tool.
    ///
    /// This is how the tool will be called by the model and in `tool_use`
    /// blocks.
    pub name: String,
    /// When true, guarantees schema validation on tool names and inputs
    #[serde(skip_serializing_if = "Option::is_none")]
    pub strict: Option<bool>,
    pub r#type: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum CodeExecutionTool20260120CacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct MemoryTool20250818 {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_callers: Option<Vec<AllowedCaller>>,
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<MemoryTool20250818CacheControl>,
    /// If true, tool will not be included in initial system prompt. Only loaded
    /// when returned via tool_reference from tool search.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub defer_loading: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_examples: Option<Vec<HashMap<String, JsonValue>>>,
    /// Name of the tool.
    ///
    /// This is how the tool will be called by the model and in `tool_use`
    /// blocks.
    pub name: String,
    /// When true, guarantees schema validation on tool names and inputs
    #[serde(skip_serializing_if = "Option::is_none")]
    pub strict: Option<bool>,
    pub r#type: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum MemoryTool20250818CacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct TextEditor20250124 {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_callers: Option<Vec<AllowedCaller>>,
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<TextEditor20250124CacheControl>,
    /// If true, tool will not be included in initial system prompt. Only loaded
    /// when returned via tool_reference from tool search.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub defer_loading: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_examples: Option<Vec<HashMap<String, JsonValue>>>,
    /// Name of the tool.
    ///
    /// This is how the tool will be called by the model and in `tool_use`
    /// blocks.
    pub name: String,
    /// When true, guarantees schema validation on tool names and inputs
    #[serde(skip_serializing_if = "Option::is_none")]
    pub strict: Option<bool>,
    pub r#type: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum TextEditor20250124CacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct TextEditor20250429 {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_callers: Option<Vec<AllowedCaller>>,
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<TextEditor20250429CacheControl>,
    /// If true, tool will not be included in initial system prompt. Only loaded
    /// when returned via tool_reference from tool search.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub defer_loading: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_examples: Option<Vec<HashMap<String, JsonValue>>>,
    /// Name of the tool.
    ///
    /// This is how the tool will be called by the model and in `tool_use`
    /// blocks.
    pub name: String,
    /// When true, guarantees schema validation on tool names and inputs
    #[serde(skip_serializing_if = "Option::is_none")]
    pub strict: Option<bool>,
    pub r#type: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum TextEditor20250429CacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct TextEditor20250728 {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_callers: Option<Vec<AllowedCaller>>,
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<TextEditor20250728CacheControl>,
    /// If true, tool will not be included in initial system prompt. Only loaded
    /// when returned via tool_reference from tool search.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub defer_loading: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_examples: Option<Vec<HashMap<String, JsonValue>>>,
    /// Maximum number of characters to display when viewing a file. If not
    /// specified, defaults to displaying the full file.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_characters: Option<i64>,
    /// Name of the tool.
    ///
    /// This is how the tool will be called by the model and in `tool_use`
    /// blocks.
    pub name: String,
    /// When true, guarantees schema validation on tool names and inputs
    #[serde(skip_serializing_if = "Option::is_none")]
    pub strict: Option<bool>,
    pub r#type: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum TextEditor20250728CacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct WebSearchTool20250305 {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_callers: Option<Vec<AllowedCaller>>,
    /// If provided, only these domains will be included in results. Cannot be
    /// used alongside `blocked_domains`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_domains: Option<Vec<String>>,
    /// If provided, these domains will never appear in results. Cannot be used
    /// alongside `allowed_domains`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub blocked_domains: Option<Vec<String>>,
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<WebSearchTool20250305CacheControl>,
    /// If true, tool will not be included in initial system prompt. Only loaded
    /// when returned via tool_reference from tool search.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub defer_loading: Option<bool>,
    /// Maximum number of times the tool can be used in the API request.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_uses: Option<i64>,
    /// Name of the tool.
    ///
    /// This is how the tool will be called by the model and in `tool_use`
    /// blocks.
    pub name: String,
    /// When true, guarantees schema validation on tool names and inputs
    #[serde(skip_serializing_if = "Option::is_none")]
    pub strict: Option<bool>,
    pub r#type: String,
    /// Parameters for the user's location. Used to provide more relevant search
    /// results.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub user_location: Option<UserLocation>,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum WebSearchTool20250305CacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct WebFetchTool20250910 {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_callers: Option<Vec<AllowedCaller>>,
    /// List of domains to allow fetching from
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_domains: Option<Vec<String>>,
    /// List of domains to block fetching from
    #[serde(skip_serializing_if = "Option::is_none")]
    pub blocked_domains: Option<Vec<String>>,
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<WebFetchTool20250910CacheControl>,
    /// Citations configuration for fetched documents. Citations are disabled by
    /// default.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub citations: Option<RequestCitationsConfig>,
    /// If true, tool will not be included in initial system prompt. Only loaded
    /// when returned via tool_reference from tool search.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub defer_loading: Option<bool>,
    /// Maximum number of tokens used by including web page text content in the
    /// context. The limit is approximate and does not apply to binary content
    /// such as PDFs.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_content_tokens: Option<i64>,
    /// Maximum number of times the tool can be used in the API request.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_uses: Option<i64>,
    /// Name of the tool.
    ///
    /// This is how the tool will be called by the model and in `tool_use`
    /// blocks.
    pub name: String,
    /// When true, guarantees schema validation on tool names and inputs
    #[serde(skip_serializing_if = "Option::is_none")]
    pub strict: Option<bool>,
    pub r#type: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum WebFetchTool20250910CacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct WebSearchTool20260209 {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_callers: Option<Vec<AllowedCaller>>,
    /// If provided, only these domains will be included in results. Cannot be
    /// used alongside `blocked_domains`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_domains: Option<Vec<String>>,
    /// If provided, these domains will never appear in results. Cannot be used
    /// alongside `allowed_domains`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub blocked_domains: Option<Vec<String>>,
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<WebSearchTool20260209CacheControl>,
    /// If true, tool will not be included in initial system prompt. Only loaded
    /// when returned via tool_reference from tool search.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub defer_loading: Option<bool>,
    /// Maximum number of times the tool can be used in the API request.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_uses: Option<i64>,
    /// Name of the tool.
    ///
    /// This is how the tool will be called by the model and in `tool_use`
    /// blocks.
    pub name: String,
    /// When true, guarantees schema validation on tool names and inputs
    #[serde(skip_serializing_if = "Option::is_none")]
    pub strict: Option<bool>,
    pub r#type: String,
    /// Parameters for the user's location. Used to provide more relevant search
    /// results.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub user_location: Option<UserLocation>,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum WebSearchTool20260209CacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct WebFetchTool20260209 {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_callers: Option<Vec<AllowedCaller>>,
    /// List of domains to allow fetching from
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_domains: Option<Vec<String>>,
    /// List of domains to block fetching from
    #[serde(skip_serializing_if = "Option::is_none")]
    pub blocked_domains: Option<Vec<String>>,
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<WebFetchTool20260209CacheControl>,
    /// Citations configuration for fetched documents. Citations are disabled by
    /// default.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub citations: Option<RequestCitationsConfig>,
    /// If true, tool will not be included in initial system prompt. Only loaded
    /// when returned via tool_reference from tool search.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub defer_loading: Option<bool>,
    /// Maximum number of tokens used by including web page text content in the
    /// context. The limit is approximate and does not apply to binary content
    /// such as PDFs.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_content_tokens: Option<i64>,
    /// Maximum number of times the tool can be used in the API request.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_uses: Option<i64>,
    /// Name of the tool.
    ///
    /// This is how the tool will be called by the model and in `tool_use`
    /// blocks.
    pub name: String,
    /// When true, guarantees schema validation on tool names and inputs
    #[serde(skip_serializing_if = "Option::is_none")]
    pub strict: Option<bool>,
    pub r#type: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum WebFetchTool20260209CacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

/// Web fetch tool with use_cache parameter for bypassing cached content.
#[derive(Serialize, Deserialize)]
pub struct WebFetchTool20260309 {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_callers: Option<Vec<AllowedCaller>>,
    /// List of domains to allow fetching from
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_domains: Option<Vec<String>>,
    /// List of domains to block fetching from
    #[serde(skip_serializing_if = "Option::is_none")]
    pub blocked_domains: Option<Vec<String>>,
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<WebFetchTool20260309CacheControl>,
    /// Citations configuration for fetched documents. Citations are disabled by
    /// default.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub citations: Option<RequestCitationsConfig>,
    /// If true, tool will not be included in initial system prompt. Only loaded
    /// when returned via tool_reference from tool search.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub defer_loading: Option<bool>,
    /// Maximum number of tokens used by including web page text content in the
    /// context. The limit is approximate and does not apply to binary content
    /// such as PDFs.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_content_tokens: Option<i64>,
    /// Maximum number of times the tool can be used in the API request.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_uses: Option<i64>,
    /// Name of the tool.
    ///
    /// This is how the tool will be called by the model and in `tool_use`
    /// blocks.
    pub name: String,
    /// When true, guarantees schema validation on tool names and inputs
    #[serde(skip_serializing_if = "Option::is_none")]
    pub strict: Option<bool>,
    pub r#type: String,
    /// Whether to use cached content. Set to false to bypass the cache and
    /// fetch fresh content. Only set to false when the user explicitly requests
    /// fresh content or when fetching rapidly-changing sources.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub use_cache: Option<bool>,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum WebFetchTool20260309CacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct ToolSearchToolBM2520251119 {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_callers: Option<Vec<AllowedCaller>>,
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<ToolSearchToolBM2520251119CacheControl>,
    /// If true, tool will not be included in initial system prompt. Only loaded
    /// when returned via tool_reference from tool search.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub defer_loading: Option<bool>,
    /// Name of the tool.
    ///
    /// This is how the tool will be called by the model and in `tool_use`
    /// blocks.
    pub name: String,
    /// When true, guarantees schema validation on tool names and inputs
    #[serde(skip_serializing_if = "Option::is_none")]
    pub strict: Option<bool>,
    pub r#type: ToolSearchToolBM2520251119Type,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum ToolSearchToolBM2520251119CacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
pub enum ToolSearchToolBM2520251119Type {
    #[serde(rename = "tool_search_tool_bm25_20251119")]
    ToolSearchToolBm2520251119,
    #[serde(rename = "tool_search_tool_bm25")]
    ToolSearchToolBm25,
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct ToolSearchToolRegex20251119 {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_callers: Option<Vec<AllowedCaller>>,
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<ToolSearchToolRegex20251119CacheControl>,
    /// If true, tool will not be included in initial system prompt. Only loaded
    /// when returned via tool_reference from tool search.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub defer_loading: Option<bool>,
    /// Name of the tool.
    ///
    /// This is how the tool will be called by the model and in `tool_use`
    /// blocks.
    pub name: String,
    /// When true, guarantees schema validation on tool names and inputs
    #[serde(skip_serializing_if = "Option::is_none")]
    pub strict: Option<bool>,
    pub r#type: ToolSearchToolRegex20251119Type,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum ToolSearchToolRegex20251119CacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
pub enum ToolSearchToolRegex20251119Type {
    #[serde(rename = "tool_search_tool_regex_20251119")]
    ToolSearchToolRegex20251119,
    #[serde(rename = "tool_search_tool_regex")]
    ToolSearchToolRegex,
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

/// Information about the container used in the request (for the code execution
/// tool)
#[derive(Serialize, Deserialize)]
pub struct Container {
    /// The time at which the container will expire.
    pub expires_at: String,
    /// Identifier for the container used in this request
    pub id: String,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum ContentBlock {
    #[serde(rename = "text")]
    Text(ResponseTextBlock),
    #[serde(rename = "thinking")]
    Thinking(ResponseThinkingBlock),
    #[serde(rename = "redacted_thinking")]
    RedactedThinking(ResponseRedactedThinkingBlock),
    #[serde(rename = "tool_use")]
    ToolUse(ResponseToolUseBlock),
    #[serde(rename = "server_tool_use")]
    ServerToolUse(ResponseServerToolUseBlock),
    #[serde(rename = "web_search_tool_result")]
    WebSearchToolResult(ResponseWebSearchToolResultBlock),
    #[serde(rename = "web_fetch_tool_result")]
    WebFetchToolResult(ResponseWebFetchToolResultBlock),
    #[serde(rename = "code_execution_tool_result")]
    CodeExecutionToolResult(ResponseCodeExecutionToolResultBlock),
    #[serde(rename = "bash_code_execution_tool_result")]
    BashCodeExecutionToolResult(ResponseBashCodeExecutionToolResultBlock),
    #[serde(rename = "text_editor_code_execution_tool_result")]
    TextEditorCodeExecutionToolResult(ResponseTextEditorCodeExecutionToolResultBlock),
    #[serde(rename = "tool_search_tool_result")]
    ToolSearchToolResult(ResponseToolSearchToolResultBlock),
    #[serde(rename = "container_upload")]
    ContainerUpload(ResponseContainerUploadBlock),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

/// Structured information about a refusal.
#[derive(Serialize, Deserialize)]
pub struct RefusalStopDetails {
    /// The policy category that triggered the refusal.
    ///
    /// `null` when the refusal doesn't map to a named category.
    pub category: Option<RefusalStopDetailsCategory>,
    /// Human-readable explanation of the refusal.
    ///
    /// This text is not guaranteed to be stable. `null` when no explanation is
    /// available for the category.
    pub explanation: Option<String>,
    pub r#type: String,
}

/// The policy category that triggered the refusal.
///
/// `null` when the refusal doesn't map to a named category.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
pub enum RefusalStopDetailsCategory {
    #[serde(rename = "cyber")]
    Cyber,
    #[serde(rename = "bio")]
    Bio,
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
pub enum StopReason {
    #[serde(rename = "end_turn")]
    EndTurn,
    #[serde(rename = "max_tokens")]
    MaxTokens,
    #[serde(rename = "stop_sequence")]
    StopSequence,
    #[serde(rename = "tool_use")]
    ToolUse,
    #[serde(rename = "pause_turn")]
    PauseTurn,
    #[serde(rename = "refusal")]
    Refusal,
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct Usage {
    /// Breakdown of cached tokens by TTL
    pub cache_creation: Option<CacheCreation>,
    /// The number of input tokens used to create the cache entry.
    pub cache_creation_input_tokens: Option<i64>,
    /// The number of input tokens read from the cache.
    pub cache_read_input_tokens: Option<i64>,
    /// The geographic region where inference was performed for this request.
    pub inference_geo: Option<String>,
    /// The number of input tokens which were used.
    pub input_tokens: i64,
    /// The number of output tokens which were used.
    pub output_tokens: i64,
    /// The number of server tool requests.
    pub server_tool_use: Option<ServerToolUsage>,
    /// If the request used the priority, standard, or batch tier.
    pub service_tier: Option<UsageServiceTier>,
}

/// If the request used the priority, standard, or batch tier.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
pub enum UsageServiceTier {
    #[serde(rename = "standard")]
    Standard,
    #[serde(rename = "priority")]
    Priority,
    #[serde(rename = "batch")]
    Batch,
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct ContentBlockDeltaEvent {
    pub delta: ContentBlockDeltaEventDelta,
    pub index: i64,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum ContentBlockDeltaEventDelta {
    #[serde(rename = "text_delta")]
    TextDelta(TextContentBlockDelta),
    #[serde(rename = "input_json_delta")]
    InputJsonDelta(InputJsonContentBlockDelta),
    #[serde(rename = "citations_delta")]
    CitationsDelta(CitationsDelta),
    #[serde(rename = "thinking_delta")]
    ThinkingDelta(ThinkingContentBlockDelta),
    #[serde(rename = "signature_delta")]
    SignatureDelta(SignatureContentBlockDelta),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct ContentBlockStartEvent {
    pub content_block: ContentBlockStartEventContentBlock,
    pub index: i64,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum ContentBlockStartEventContentBlock {
    #[serde(rename = "text")]
    Text(ResponseTextBlock),
    #[serde(rename = "thinking")]
    Thinking(ResponseThinkingBlock),
    #[serde(rename = "redacted_thinking")]
    RedactedThinking(ResponseRedactedThinkingBlock),
    #[serde(rename = "tool_use")]
    ToolUse(ResponseToolUseBlock),
    #[serde(rename = "server_tool_use")]
    ServerToolUse(ResponseServerToolUseBlock),
    #[serde(rename = "web_search_tool_result")]
    WebSearchToolResult(ResponseWebSearchToolResultBlock),
    #[serde(rename = "web_fetch_tool_result")]
    WebFetchToolResult(ResponseWebFetchToolResultBlock),
    #[serde(rename = "code_execution_tool_result")]
    CodeExecutionToolResult(ResponseCodeExecutionToolResultBlock),
    #[serde(rename = "bash_code_execution_tool_result")]
    BashCodeExecutionToolResult(ResponseBashCodeExecutionToolResultBlock),
    #[serde(rename = "text_editor_code_execution_tool_result")]
    TextEditorCodeExecutionToolResult(ResponseTextEditorCodeExecutionToolResultBlock),
    #[serde(rename = "tool_search_tool_result")]
    ToolSearchToolResult(ResponseToolSearchToolResultBlock),
    #[serde(rename = "container_upload")]
    ContainerUpload(ResponseContainerUploadBlock),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct ContentBlockStopEvent {
    pub index: i64,
}

#[derive(Serialize, Deserialize)]
pub struct MessageDeltaEvent {
    pub delta: MessageDelta,
    /// Billing and rate-limit usage.
    ///
    /// Anthropic's API bills and rate-limits by token counts, as tokens
    /// represent the underlying cost to our systems.
    ///
    /// Under the hood, the API transforms requests into a format suitable for
    /// the model. The model's output then goes through a parsing stage before
    /// becoming an API response. As a result, the token counts in `usage` will
    /// not match one-to-one with the exact visible content of an API request or
    /// response.
    ///
    /// For example, `output_tokens` will be non-zero, even for an empty string
    /// response from Claude.
    ///
    /// Total input tokens in a request is the summation of `input_tokens`,
    /// `cache_creation_input_tokens`, and `cache_read_input_tokens`.
    pub usage: MessageDeltaUsage,
}

#[derive(Serialize, Deserialize)]
pub struct MessageStartEvent {
    pub message: Message,
}

#[derive(Serialize, Deserialize)]
pub struct MessageStopEvent {}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum InputContentBlock {
    #[serde(rename = "text")]
    Text(RequestTextBlock),
    #[serde(rename = "image")]
    Image(RequestImageBlock),
    #[serde(rename = "document")]
    Document(RequestDocumentBlock),
    #[serde(rename = "search_result")]
    SearchResult(RequestSearchResultBlock),
    #[serde(rename = "thinking")]
    Thinking(RequestThinkingBlock),
    #[serde(rename = "redacted_thinking")]
    RedactedThinking(RequestRedactedThinkingBlock),
    #[serde(rename = "tool_use")]
    ToolUse(RequestToolUseBlock),
    #[serde(rename = "tool_result")]
    ToolResult(RequestToolResultBlock),
    #[serde(rename = "server_tool_use")]
    ServerToolUse(RequestServerToolUseBlock),
    #[serde(rename = "web_search_tool_result")]
    WebSearchToolResult(RequestWebSearchToolResultBlock),
    #[serde(rename = "web_fetch_tool_result")]
    WebFetchToolResult(RequestWebFetchToolResultBlock),
    #[serde(rename = "code_execution_tool_result")]
    CodeExecutionToolResult(RequestCodeExecutionToolResultBlock),
    #[serde(rename = "bash_code_execution_tool_result")]
    BashCodeExecutionToolResult(RequestBashCodeExecutionToolResultBlock),
    #[serde(rename = "text_editor_code_execution_tool_result")]
    TextEditorCodeExecutionToolResult(RequestTextEditorCodeExecutionToolResultBlock),
    #[serde(rename = "tool_search_tool_result")]
    ToolSearchToolResult(RequestToolSearchToolResultBlock),
    #[serde(rename = "container_upload")]
    ContainerUpload(RequestContainerUploadBlock),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

/// All possible effort levels.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
pub enum EffortLevel {
    #[serde(rename = "low")]
    Low,
    #[serde(rename = "medium")]
    Medium,
    #[serde(rename = "high")]
    High,
    #[serde(rename = "xhigh")]
    Xhigh,
    #[serde(rename = "max")]
    Max,
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct JsonOutputFormat {
    /// The JSON schema of the format
    pub schema: Value,
    pub r#type: String,
}

#[derive(Serialize, Deserialize)]
pub struct RequestCharLocationCitation {
    pub cited_text: String,
    pub document_index: i64,
    pub document_title: Option<String>,
    pub end_char_index: i64,
    pub start_char_index: i64,
}

#[derive(Serialize, Deserialize)]
pub struct RequestContentBlockLocationCitation {
    pub cited_text: String,
    pub document_index: i64,
    pub document_title: Option<String>,
    pub end_block_index: i64,
    pub start_block_index: i64,
}

#[derive(Serialize, Deserialize)]
pub struct RequestPageLocationCitation {
    pub cited_text: String,
    pub document_index: i64,
    pub document_title: Option<String>,
    pub end_page_number: i64,
    pub start_page_number: i64,
}

#[derive(Serialize, Deserialize)]
pub struct RequestSearchResultLocationCitation {
    pub cited_text: String,
    pub end_block_index: i64,
    pub search_result_index: i64,
    pub source: String,
    pub start_block_index: i64,
    pub title: Option<String>,
}

#[derive(Serialize, Deserialize)]
pub struct RequestWebSearchResultLocationCitation {
    pub cited_text: String,
    pub encrypted_index: String,
    pub title: Option<String>,
    pub url: String,
}

#[derive(Serialize, Deserialize)]
pub struct ThinkingConfigAdaptive {
    /// Controls how thinking content appears in the response. When set to
    /// `summarized`, thinking is returned normally. When set to `omitted`,
    /// thinking content is redacted but a signature is returned for multi-turn
    /// continuity. Defaults to `summarized`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub display: Option<ThinkingDisplayMode>,
}

#[derive(Serialize, Deserialize)]
pub struct ThinkingConfigDisabled {}

#[derive(Serialize, Deserialize)]
pub struct ThinkingConfigEnabled {
    /// Determines how many tokens Claude can use for its internal reasoning
    /// process. Larger budgets can enable more thorough analysis for complex
    /// problems, improving response quality.
    ///
    /// Must be ≥1024 and less than `max_tokens`.
    ///
    /// See [extended thinking](https://docs.claude.com/en/docs/build-with-claude/extended-thinking) for details.
    pub budget_tokens: i64,
    /// Controls how thinking content appears in the response. When set to
    /// `summarized`, thinking is returned normally. When set to `omitted`,
    /// thinking content is redacted but a signature is returned for multi-turn
    /// continuity. Defaults to `summarized`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub display: Option<ThinkingDisplayMode>,
}

/// The model will use any available tools.
#[derive(Serialize, Deserialize)]
pub struct ToolChoiceAny {
    /// Whether to disable parallel tool use.
    ///
    /// Defaults to `false`. If set to `true`, the model will output exactly one
    /// tool use.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub disable_parallel_tool_use: Option<bool>,
}

/// The model will automatically decide whether to use tools.
#[derive(Serialize, Deserialize)]
pub struct ToolChoiceAuto {
    /// Whether to disable parallel tool use.
    ///
    /// Defaults to `false`. If set to `true`, the model will output at most one
    /// tool use.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub disable_parallel_tool_use: Option<bool>,
}

/// The model will not be allowed to use tools.
#[derive(Serialize, Deserialize)]
pub struct ToolChoiceNone {}

/// The model will use the specified tool with `tool_choice.name`.
#[derive(Serialize, Deserialize)]
pub struct ToolChoiceTool {
    /// Whether to disable parallel tool use.
    ///
    /// Defaults to `false`. If set to `true`, the model will output exactly one
    /// tool use.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub disable_parallel_tool_use: Option<bool>,
    /// The name of the tool to use.
    pub name: String,
}

/// Specifies who can invoke a tool.
///
/// Values:
///     direct: The model can call this tool directly.
///     code_execution_20250825: The tool can be called from the code execution
/// environment (v1).     code_execution_20260120: The tool can be called from
/// the code execution environment (v2 with persistence).
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
pub enum AllowedCaller {
    #[serde(rename = "direct")]
    Direct,
    #[serde(rename = "code_execution_20250825")]
    CodeExecution20250825,
    #[serde(rename = "code_execution_20260120")]
    CodeExecution20260120,
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

pub type JsonValue = Option<Value>;

pub type InputSchema = Option<Value>;

#[derive(Serialize, Deserialize)]
pub struct UserLocation {
    /// The city of the user.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub city: Option<String>,
    /// The two letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2) of the user.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub country: Option<String>,
    /// The region of the user.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub region: Option<String>,
    /// The [IANA timezone](https://nodatime.org/TimeZones) of the user.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub timezone: Option<String>,
    pub r#type: String,
}

#[derive(Serialize, Deserialize)]
pub struct RequestCitationsConfig {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseBashCodeExecutionToolResultBlock {
    pub content: ResponseBashCodeExecutionToolResultBlockContent,
    pub tool_use_id: String,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum ResponseBashCodeExecutionToolResultBlockContent {
    #[serde(rename = "bash_code_execution_tool_result_error")]
    BashCodeExecutionToolResultError(ResponseBashCodeExecutionToolResultError),
    #[serde(rename = "bash_code_execution_result")]
    BashCodeExecutionResult(ResponseBashCodeExecutionResultBlock),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseCodeExecutionToolResultBlock {
    pub content: ResponseCodeExecutionToolResultBlockContent,
    pub tool_use_id: String,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum ResponseCodeExecutionToolResultBlockContent {
    #[serde(rename = "code_execution_tool_result_error")]
    CodeExecutionToolResultError(ResponseCodeExecutionToolResultError),
    #[serde(rename = "code_execution_result")]
    CodeExecutionResult(ResponseCodeExecutionResultBlock),
    #[serde(rename = "encrypted_code_execution_result")]
    EncryptedCodeExecutionResult(ResponseEncryptedCodeExecutionResultBlock),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

/// Response model for a file uploaded to the container.
#[derive(Serialize, Deserialize)]
pub struct ResponseContainerUploadBlock {
    pub file_id: String,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseRedactedThinkingBlock {
    pub data: String,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseServerToolUseBlock {
    pub caller: ResponseServerToolUseBlockCaller,
    pub id: String,
    pub input: Value,
    pub name: ResponseServerToolUseBlockName,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum ResponseServerToolUseBlockCaller {
    #[serde(rename = "direct")]
    Direct(DirectCaller),
    #[serde(rename = "code_execution_20250825")]
    CodeExecution20250825(ServerToolCaller),
    #[serde(rename = "code_execution_20260120")]
    CodeExecution20260120(ServerToolCaller20260120),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
pub enum ResponseServerToolUseBlockName {
    #[serde(rename = "web_search")]
    WebSearch,
    #[serde(rename = "web_fetch")]
    WebFetch,
    #[serde(rename = "code_execution")]
    CodeExecution,
    #[serde(rename = "bash_code_execution")]
    BashCodeExecution,
    #[serde(rename = "text_editor_code_execution")]
    TextEditorCodeExecution,
    #[serde(rename = "tool_search_tool_regex")]
    ToolSearchToolRegex,
    #[serde(rename = "tool_search_tool_bm25")]
    ToolSearchToolBm25,
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseTextBlock {
    /// Citations supporting the text block.
    ///
    /// The type of citation returned will depend on the type of document being
    /// cited. Citing a PDF results in `page_location`, plain text results in
    /// `char_location`, and content document results in
    /// `content_block_location`.
    pub citations: Option<Vec<ResponseTextBlockCitationsItem>>,
    pub text: String,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum ResponseTextBlockCitationsItem {
    #[serde(rename = "char_location")]
    CharLocation(ResponseCharLocationCitation),
    #[serde(rename = "page_location")]
    PageLocation(ResponsePageLocationCitation),
    #[serde(rename = "content_block_location")]
    ContentBlockLocation(ResponseContentBlockLocationCitation),
    #[serde(rename = "web_search_result_location")]
    WebSearchResultLocation(ResponseWebSearchResultLocationCitation),
    #[serde(rename = "search_result_location")]
    SearchResultLocation(ResponseSearchResultLocationCitation),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseTextEditorCodeExecutionToolResultBlock {
    pub content: ResponseTextEditorCodeExecutionToolResultBlockContent,
    pub tool_use_id: String,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum ResponseTextEditorCodeExecutionToolResultBlockContent {
    #[serde(rename = "text_editor_code_execution_tool_result_error")]
    TextEditorCodeExecutionToolResultError(ResponseTextEditorCodeExecutionToolResultError),
    #[serde(rename = "text_editor_code_execution_view_result")]
    TextEditorCodeExecutionViewResult(ResponseTextEditorCodeExecutionViewResultBlock),
    #[serde(rename = "text_editor_code_execution_create_result")]
    TextEditorCodeExecutionCreateResult(ResponseTextEditorCodeExecutionCreateResultBlock),
    #[serde(rename = "text_editor_code_execution_str_replace_result")]
    TextEditorCodeExecutionStrReplaceResult(ResponseTextEditorCodeExecutionStrReplaceResultBlock),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseThinkingBlock {
    pub signature: String,
    pub thinking: String,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseToolSearchToolResultBlock {
    pub content: ResponseToolSearchToolResultBlockContent,
    pub tool_use_id: String,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum ResponseToolSearchToolResultBlockContent {
    #[serde(rename = "tool_search_tool_result_error")]
    ToolSearchToolResultError(ResponseToolSearchToolResultError),
    #[serde(rename = "tool_search_tool_search_result")]
    ToolSearchToolSearchResult(ResponseToolSearchToolSearchResultBlock),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseToolUseBlock {
    pub caller: ResponseToolUseBlockCaller,
    pub id: String,
    pub input: Value,
    pub name: String,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum ResponseToolUseBlockCaller {
    #[serde(rename = "direct")]
    Direct(DirectCaller),
    #[serde(rename = "code_execution_20250825")]
    CodeExecution20250825(ServerToolCaller),
    #[serde(rename = "code_execution_20260120")]
    CodeExecution20260120(ServerToolCaller20260120),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseWebFetchToolResultBlock {
    pub caller: ResponseWebFetchToolResultBlockCaller,
    pub content: ResponseWebFetchToolResultBlockContent,
    pub tool_use_id: String,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum ResponseWebFetchToolResultBlockCaller {
    #[serde(rename = "direct")]
    Direct(DirectCaller),
    #[serde(rename = "code_execution_20250825")]
    CodeExecution20250825(ServerToolCaller),
    #[serde(rename = "code_execution_20260120")]
    CodeExecution20260120(ServerToolCaller20260120),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum ResponseWebFetchToolResultBlockContent {
    #[serde(rename = "web_fetch_tool_result_error")]
    WebFetchToolResultError(ResponseWebFetchToolResultError),
    #[serde(rename = "web_fetch_result")]
    WebFetchResult(ResponseWebFetchResultBlock),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseWebSearchToolResultBlock {
    pub caller: ResponseWebSearchToolResultBlockCaller,
    pub content: ResponseWebSearchToolResultBlockContent,
    pub tool_use_id: String,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum ResponseWebSearchToolResultBlockCaller {
    #[serde(rename = "direct")]
    Direct(DirectCaller),
    #[serde(rename = "code_execution_20250825")]
    CodeExecution20250825(ServerToolCaller),
    #[serde(rename = "code_execution_20260120")]
    CodeExecution20260120(ServerToolCaller20260120),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

pub type ResponseWebSearchToolResultBlockContentArray = Option<Vec<ResponseWebSearchResultBlock>>;

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(untagged)]
pub enum ResponseWebSearchToolResultBlockContent {
    ResponseWebSearchToolResultError(ResponseWebSearchToolResultError),
    ResponseWebSearchToolResultBlockContentArray(ResponseWebSearchToolResultBlockContentArray),
    #[allow(dead_code)]
    #[serde(skip_serializing)]
    Unknown(Value),
}

#[derive(Serialize, Deserialize)]
pub struct CacheCreation {
    /// The number of input tokens used to create the 1 hour cache entry.
    #[serde(rename = "ephemeral_1h_input_tokens")]
    pub ephemeral_1_h_input_tokens: i64,
    /// The number of input tokens used to create the 5 minute cache entry.
    #[serde(rename = "ephemeral_5m_input_tokens")]
    pub ephemeral_5_m_input_tokens: i64,
}

#[derive(Serialize, Deserialize)]
pub struct ServerToolUsage {
    /// The number of web fetch tool requests.
    pub web_fetch_requests: i64,
    /// The number of web search tool requests.
    pub web_search_requests: i64,
}

#[derive(Serialize, Deserialize)]
pub struct CitationsDelta {
    pub citation: CitationsDeltaCitation,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum CitationsDeltaCitation {
    #[serde(rename = "char_location")]
    CharLocation(ResponseCharLocationCitation),
    #[serde(rename = "page_location")]
    PageLocation(ResponsePageLocationCitation),
    #[serde(rename = "content_block_location")]
    ContentBlockLocation(ResponseContentBlockLocationCitation),
    #[serde(rename = "web_search_result_location")]
    WebSearchResultLocation(ResponseWebSearchResultLocationCitation),
    #[serde(rename = "search_result_location")]
    SearchResultLocation(ResponseSearchResultLocationCitation),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct InputJsonContentBlockDelta {
    pub partial_json: String,
}

#[derive(Serialize, Deserialize)]
pub struct SignatureContentBlockDelta {
    pub signature: String,
}

#[derive(Serialize, Deserialize)]
pub struct TextContentBlockDelta {
    pub text: String,
}

#[derive(Serialize, Deserialize)]
pub struct ThinkingContentBlockDelta {
    pub thinking: String,
}

#[derive(Serialize, Deserialize)]
pub struct MessageDelta {
    /// Information about the container used in this request.
    ///
    /// This will be non-null if a container tool (e.g. code execution) was
    /// used.
    pub container: Option<Container>,
    /// Structured information about why model output stopped.
    ///
    /// This is `null` when the `stop_reason` has no additional detail to
    /// report.
    pub stop_details: Option<RefusalStopDetails>,
    pub stop_reason: Option<StopReason>,
    pub stop_sequence: Option<String>,
}

#[derive(Serialize, Deserialize)]
pub struct MessageDeltaUsage {
    /// The cumulative number of input tokens used to create the cache entry.
    pub cache_creation_input_tokens: Option<i64>,
    /// The cumulative number of input tokens read from the cache.
    pub cache_read_input_tokens: Option<i64>,
    /// The cumulative number of input tokens which were used.
    pub input_tokens: Option<i64>,
    /// The cumulative number of output tokens which were used.
    pub output_tokens: i64,
    /// The number of server tool requests.
    pub server_tool_use: Option<ServerToolUsage>,
}

#[derive(Serialize, Deserialize)]
pub struct RequestBashCodeExecutionToolResultBlock {
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<RequestBashCodeExecutionToolResultBlockCacheControl>,
    pub content: RequestBashCodeExecutionToolResultBlockContent,
    pub tool_use_id: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestBashCodeExecutionToolResultBlockCacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestBashCodeExecutionToolResultBlockContent {
    #[serde(rename = "bash_code_execution_tool_result_error")]
    BashCodeExecutionToolResultError(RequestBashCodeExecutionToolResultError),
    #[serde(rename = "bash_code_execution_result")]
    BashCodeExecutionResult(RequestBashCodeExecutionResultBlock),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct RequestCodeExecutionToolResultBlock {
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<RequestCodeExecutionToolResultBlockCacheControl>,
    pub content: RequestCodeExecutionToolResultBlockContent,
    pub tool_use_id: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestCodeExecutionToolResultBlockCacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestCodeExecutionToolResultBlockContent {
    #[serde(rename = "code_execution_tool_result_error")]
    CodeExecutionToolResultError(RequestCodeExecutionToolResultError),
    #[serde(rename = "code_execution_result")]
    CodeExecutionResult(RequestCodeExecutionResultBlock),
    #[serde(rename = "encrypted_code_execution_result")]
    EncryptedCodeExecutionResult(RequestEncryptedCodeExecutionResultBlock),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

/// A content block that represents a file to be uploaded to the container
/// Files uploaded via this block will be available in the container's input
/// directory.
#[derive(Serialize, Deserialize)]
pub struct RequestContainerUploadBlock {
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<RequestContainerUploadBlockCacheControl>,
    pub file_id: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestContainerUploadBlockCacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct RequestDocumentBlock {
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<RequestDocumentBlockCacheControl>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub citations: Option<RequestCitationsConfig>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub context: Option<String>,
    pub source: RequestDocumentBlockSource,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub title: Option<String>,
    pub r#type: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestDocumentBlockCacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestDocumentBlockSource {
    #[serde(rename = "base64")]
    Base64(Base64PDFSource),
    #[serde(rename = "text")]
    Text(PlainTextSource),
    #[serde(rename = "content")]
    Content(ContentBlockSource),
    #[serde(rename = "url")]
    Url(URLPDFSource),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct RequestImageBlock {
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<RequestImageBlockCacheControl>,
    pub source: RequestImageBlockSource,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestImageBlockCacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestImageBlockSource {
    #[serde(rename = "base64")]
    Base64(Base64ImageSource),
    #[serde(rename = "url")]
    Url(URLImageSource),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct RequestRedactedThinkingBlock {
    pub data: String,
}

#[derive(Serialize, Deserialize)]
pub struct RequestSearchResultBlock {
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<RequestSearchResultBlockCacheControl>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub citations: Option<RequestCitationsConfig>,
    pub content: Vec<RequestTextBlock>,
    pub source: String,
    pub title: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestSearchResultBlockCacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct RequestServerToolUseBlock {
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<RequestServerToolUseBlockCacheControl>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub caller: Option<RequestServerToolUseBlockCaller>,
    pub id: String,
    pub input: Value,
    pub name: RequestServerToolUseBlockName,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestServerToolUseBlockCacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestServerToolUseBlockCaller {
    #[serde(rename = "direct")]
    Direct(DirectCaller),
    #[serde(rename = "code_execution_20250825")]
    CodeExecution20250825(ServerToolCaller),
    #[serde(rename = "code_execution_20260120")]
    CodeExecution20260120(ServerToolCaller20260120),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
pub enum RequestServerToolUseBlockName {
    #[serde(rename = "web_search")]
    WebSearch,
    #[serde(rename = "web_fetch")]
    WebFetch,
    #[serde(rename = "code_execution")]
    CodeExecution,
    #[serde(rename = "bash_code_execution")]
    BashCodeExecution,
    #[serde(rename = "text_editor_code_execution")]
    TextEditorCodeExecution,
    #[serde(rename = "tool_search_tool_regex")]
    ToolSearchToolRegex,
    #[serde(rename = "tool_search_tool_bm25")]
    ToolSearchToolBm25,
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct RequestTextEditorCodeExecutionToolResultBlock {
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<RequestTextEditorCodeExecutionToolResultBlockCacheControl>,
    pub content: RequestTextEditorCodeExecutionToolResultBlockContent,
    pub tool_use_id: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestTextEditorCodeExecutionToolResultBlockCacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestTextEditorCodeExecutionToolResultBlockContent {
    #[serde(rename = "text_editor_code_execution_tool_result_error")]
    TextEditorCodeExecutionToolResultError(RequestTextEditorCodeExecutionToolResultError),
    #[serde(rename = "text_editor_code_execution_view_result")]
    TextEditorCodeExecutionViewResult(RequestTextEditorCodeExecutionViewResultBlock),
    #[serde(rename = "text_editor_code_execution_create_result")]
    TextEditorCodeExecutionCreateResult(RequestTextEditorCodeExecutionCreateResultBlock),
    #[serde(rename = "text_editor_code_execution_str_replace_result")]
    TextEditorCodeExecutionStrReplaceResult(RequestTextEditorCodeExecutionStrReplaceResultBlock),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct RequestThinkingBlock {
    pub signature: String,
    pub thinking: String,
}

#[derive(Serialize, Deserialize)]
pub struct RequestToolResultBlock {
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<RequestToolResultBlockCacheControl>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub content: Option<RequestToolResultBlockContent>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub is_error: Option<bool>,
    pub tool_use_id: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestToolResultBlockCacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

pub type RequestToolResultBlockContentString = Option<String>;

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestToolResultBlockContentArrayItem {
    #[serde(rename = "text")]
    Text(RequestTextBlock),
    #[serde(rename = "image")]
    Image(RequestImageBlock),
    #[serde(rename = "search_result")]
    SearchResult(RequestSearchResultBlock),
    #[serde(rename = "document")]
    Document(RequestDocumentBlock),
    #[serde(rename = "tool_reference")]
    ToolReference(RequestToolReferenceBlock),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

pub type RequestToolResultBlockContentArray = Option<Vec<RequestToolResultBlockContentArrayItem>>;

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(untagged)]
pub enum RequestToolResultBlockContent {
    RequestToolResultBlockContentString(RequestToolResultBlockContentString),
    RequestToolResultBlockContentArray(RequestToolResultBlockContentArray),
    #[allow(dead_code)]
    #[serde(skip_serializing)]
    Unknown(Value),
}

#[derive(Serialize, Deserialize)]
pub struct RequestToolSearchToolResultBlock {
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<RequestToolSearchToolResultBlockCacheControl>,
    pub content: RequestToolSearchToolResultBlockContent,
    pub tool_use_id: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestToolSearchToolResultBlockCacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestToolSearchToolResultBlockContent {
    #[serde(rename = "tool_search_tool_result_error")]
    ToolSearchToolResultError(RequestToolSearchToolResultError),
    #[serde(rename = "tool_search_tool_search_result")]
    ToolSearchToolSearchResult(RequestToolSearchToolSearchResultBlock),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct RequestToolUseBlock {
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<RequestToolUseBlockCacheControl>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub caller: Option<RequestToolUseBlockCaller>,
    pub id: String,
    pub input: Value,
    pub name: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestToolUseBlockCacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestToolUseBlockCaller {
    #[serde(rename = "direct")]
    Direct(DirectCaller),
    #[serde(rename = "code_execution_20250825")]
    CodeExecution20250825(ServerToolCaller),
    #[serde(rename = "code_execution_20260120")]
    CodeExecution20260120(ServerToolCaller20260120),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct RequestWebFetchToolResultBlock {
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<RequestWebFetchToolResultBlockCacheControl>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub caller: Option<RequestWebFetchToolResultBlockCaller>,
    pub content: RequestWebFetchToolResultBlockContent,
    pub tool_use_id: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestWebFetchToolResultBlockCacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestWebFetchToolResultBlockCaller {
    #[serde(rename = "direct")]
    Direct(DirectCaller),
    #[serde(rename = "code_execution_20250825")]
    CodeExecution20250825(ServerToolCaller),
    #[serde(rename = "code_execution_20260120")]
    CodeExecution20260120(ServerToolCaller20260120),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestWebFetchToolResultBlockContent {
    #[serde(rename = "web_fetch_tool_result_error")]
    WebFetchToolResultError(RequestWebFetchToolResultError),
    #[serde(rename = "web_fetch_result")]
    WebFetchResult(RequestWebFetchResultBlock),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct RequestWebSearchToolResultBlock {
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<RequestWebSearchToolResultBlockCacheControl>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub caller: Option<RequestWebSearchToolResultBlockCaller>,
    pub content: RequestWebSearchToolResultBlockContent,
    pub tool_use_id: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestWebSearchToolResultBlockCacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestWebSearchToolResultBlockCaller {
    #[serde(rename = "direct")]
    Direct(DirectCaller),
    #[serde(rename = "code_execution_20250825")]
    CodeExecution20250825(ServerToolCaller),
    #[serde(rename = "code_execution_20260120")]
    CodeExecution20260120(ServerToolCaller20260120),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

pub type RequestWebSearchToolResultBlockContentArray = Option<Vec<RequestWebSearchResultBlock>>;

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(untagged)]
pub enum RequestWebSearchToolResultBlockContent {
    RequestWebSearchToolResultBlockContentArray(RequestWebSearchToolResultBlockContentArray),
    RequestWebSearchToolResultError(RequestWebSearchToolResultError),
    #[allow(dead_code)]
    #[serde(skip_serializing)]
    Unknown(Value),
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
pub enum ThinkingDisplayMode {
    #[serde(rename = "summarized")]
    Summarized,
    #[serde(rename = "omitted")]
    Omitted,
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseBashCodeExecutionToolResultError {
    pub error_code: BashCodeExecutionToolResultErrorCode,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseBashCodeExecutionResultBlock {
    pub content: Vec<ResponseBashCodeExecutionOutputBlock>,
    pub return_code: i64,
    pub stderr: String,
    pub stdout: String,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseCodeExecutionToolResultError {
    pub error_code: CodeExecutionToolResultErrorCode,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseCodeExecutionResultBlock {
    pub content: Vec<ResponseCodeExecutionOutputBlock>,
    pub return_code: i64,
    pub stderr: String,
    pub stdout: String,
}

/// Code execution result with encrypted stdout for PFC + web_search results.
#[derive(Serialize, Deserialize)]
pub struct ResponseEncryptedCodeExecutionResultBlock {
    pub content: Vec<ResponseCodeExecutionOutputBlock>,
    pub encrypted_stdout: String,
    pub return_code: i64,
    pub stderr: String,
}

/// Tool invocation generated by a server-side tool.
#[derive(Serialize, Deserialize)]
pub struct ServerToolCaller {
    pub tool_id: String,
}

#[derive(Serialize, Deserialize)]
pub struct ServerToolCaller20260120 {
    pub tool_id: String,
}

/// Tool invocation directly from the model.
#[derive(Serialize, Deserialize)]
pub struct DirectCaller {}

#[derive(Serialize, Deserialize)]
pub struct ResponseCharLocationCitation {
    pub cited_text: String,
    pub document_index: i64,
    pub document_title: Option<String>,
    pub end_char_index: i64,
    pub file_id: Option<String>,
    pub start_char_index: i64,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseContentBlockLocationCitation {
    pub cited_text: String,
    pub document_index: i64,
    pub document_title: Option<String>,
    pub end_block_index: i64,
    pub file_id: Option<String>,
    pub start_block_index: i64,
}

#[derive(Serialize, Deserialize)]
pub struct ResponsePageLocationCitation {
    pub cited_text: String,
    pub document_index: i64,
    pub document_title: Option<String>,
    pub end_page_number: i64,
    pub file_id: Option<String>,
    pub start_page_number: i64,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseSearchResultLocationCitation {
    pub cited_text: String,
    pub end_block_index: i64,
    pub search_result_index: i64,
    pub source: String,
    pub start_block_index: i64,
    pub title: Option<String>,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseWebSearchResultLocationCitation {
    pub cited_text: String,
    pub encrypted_index: String,
    pub title: Option<String>,
    pub url: String,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseTextEditorCodeExecutionToolResultError {
    pub error_code: TextEditorCodeExecutionToolResultErrorCode,
    pub error_message: Option<String>,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseTextEditorCodeExecutionViewResultBlock {
    pub content: String,
    pub file_type: ResponseTextEditorCodeExecutionViewResultBlockFileType,
    pub num_lines: Option<i64>,
    pub start_line: Option<i64>,
    pub total_lines: Option<i64>,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
pub enum ResponseTextEditorCodeExecutionViewResultBlockFileType {
    #[serde(rename = "text")]
    Text,
    #[serde(rename = "image")]
    Image,
    #[serde(rename = "pdf")]
    Pdf,
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseTextEditorCodeExecutionCreateResultBlock {
    pub is_file_update: bool,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseTextEditorCodeExecutionStrReplaceResultBlock {
    pub lines: Option<Vec<String>>,
    pub new_lines: Option<i64>,
    pub new_start: Option<i64>,
    pub old_lines: Option<i64>,
    pub old_start: Option<i64>,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseToolSearchToolResultError {
    pub error_code: ToolSearchToolResultErrorCode,
    pub error_message: Option<String>,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseToolSearchToolSearchResultBlock {
    pub tool_references: Vec<ResponseToolReferenceBlock>,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseWebFetchToolResultError {
    pub error_code: WebFetchToolResultErrorCode,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseWebFetchResultBlock {
    pub content: ResponseDocumentBlock,
    /// ISO 8601 timestamp when the content was retrieved
    pub retrieved_at: Option<String>,
    /// Fetched content URL
    pub url: String,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseWebSearchToolResultError {
    pub error_code: WebSearchToolResultErrorCode,
    pub r#type: String,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseWebSearchResultBlock {
    pub encrypted_content: String,
    pub page_age: Option<String>,
    pub title: String,
    pub r#type: String,
    pub url: String,
}

#[derive(Serialize, Deserialize)]
pub struct RequestBashCodeExecutionToolResultError {
    pub error_code: BashCodeExecutionToolResultErrorCode,
}

#[derive(Serialize, Deserialize)]
pub struct RequestBashCodeExecutionResultBlock {
    pub content: Vec<RequestBashCodeExecutionOutputBlock>,
    pub return_code: i64,
    pub stderr: String,
    pub stdout: String,
}

#[derive(Serialize, Deserialize)]
pub struct RequestCodeExecutionToolResultError {
    pub error_code: CodeExecutionToolResultErrorCode,
}

#[derive(Serialize, Deserialize)]
pub struct RequestCodeExecutionResultBlock {
    pub content: Vec<RequestCodeExecutionOutputBlock>,
    pub return_code: i64,
    pub stderr: String,
    pub stdout: String,
}

/// Code execution result with encrypted stdout for PFC + web_search results.
#[derive(Serialize, Deserialize)]
pub struct RequestEncryptedCodeExecutionResultBlock {
    pub content: Vec<RequestCodeExecutionOutputBlock>,
    pub encrypted_stdout: String,
    pub return_code: i64,
    pub stderr: String,
}

#[derive(Serialize, Deserialize)]
pub struct Base64PDFSource {
    pub data: String,
    pub media_type: String,
}

#[derive(Serialize, Deserialize)]
pub struct ContentBlockSource {
    pub content: ContentBlockSourceContent,
}

pub type ContentBlockSourceContentString = Option<String>;

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum ContentBlockSourceContentArrayItem {
    #[serde(rename = "text")]
    Text(RequestTextBlock),
    #[serde(rename = "image")]
    Image(RequestImageBlock),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

pub type ContentBlockSourceContentArray = Option<Vec<ContentBlockSourceContentArrayItem>>;

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(untagged)]
pub enum ContentBlockSourceContent {
    ContentBlockSourceContentString(ContentBlockSourceContentString),
    ContentBlockSourceContentArray(ContentBlockSourceContentArray),
    #[allow(dead_code)]
    #[serde(skip_serializing)]
    Unknown(Value),
}

#[derive(Serialize, Deserialize)]
pub struct PlainTextSource {
    pub data: String,
    pub media_type: String,
}

#[derive(Serialize, Deserialize)]
pub struct URLPDFSource {
    pub url: String,
}

#[derive(Serialize, Deserialize)]
pub struct Base64ImageSource {
    pub data: String,
    pub media_type: Base64ImageSourceMediaType,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
pub enum Base64ImageSourceMediaType {
    #[serde(rename = "image/jpeg")]
    ImageJpeg,
    #[serde(rename = "image/png")]
    ImagePng,
    #[serde(rename = "image/gif")]
    ImageGif,
    #[serde(rename = "image/webp")]
    ImageWebp,
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct URLImageSource {
    pub url: String,
}

#[derive(Serialize, Deserialize)]
pub struct RequestTextEditorCodeExecutionToolResultError {
    pub error_code: TextEditorCodeExecutionToolResultErrorCode,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error_message: Option<String>,
}

#[derive(Serialize, Deserialize)]
pub struct RequestTextEditorCodeExecutionViewResultBlock {
    pub content: String,
    pub file_type: RequestTextEditorCodeExecutionViewResultBlockFileType,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub num_lines: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub start_line: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub total_lines: Option<i64>,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
pub enum RequestTextEditorCodeExecutionViewResultBlockFileType {
    #[serde(rename = "text")]
    Text,
    #[serde(rename = "image")]
    Image,
    #[serde(rename = "pdf")]
    Pdf,
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct RequestTextEditorCodeExecutionCreateResultBlock {
    pub is_file_update: bool,
}

#[derive(Serialize, Deserialize)]
pub struct RequestTextEditorCodeExecutionStrReplaceResultBlock {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub lines: Option<Vec<String>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub new_lines: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub new_start: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub old_lines: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub old_start: Option<i64>,
}

/// Tool reference block that can be included in tool_result content.
#[derive(Serialize, Deserialize)]
pub struct RequestToolReferenceBlock {
    /// Create a cache control breakpoint at this content block.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_control: Option<RequestToolReferenceBlockCacheControl>,
    pub tool_name: String,
    pub r#type: String,
}

/// Create a cache control breakpoint at this content block.
#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum RequestToolReferenceBlockCacheControl {
    #[serde(rename = "ephemeral")]
    Ephemeral(CacheControlEphemeral),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct RequestToolSearchToolResultError {
    pub error_code: ToolSearchToolResultErrorCode,
}

#[derive(Serialize, Deserialize)]
pub struct RequestToolSearchToolSearchResultBlock {
    pub tool_references: Vec<RequestToolReferenceBlock>,
}

#[derive(Serialize, Deserialize)]
pub struct RequestWebFetchToolResultError {
    pub error_code: WebFetchToolResultErrorCode,
}

#[derive(Serialize, Deserialize)]
pub struct RequestWebFetchResultBlock {
    pub content: RequestDocumentBlock,
    /// ISO 8601 timestamp when the content was retrieved
    #[serde(skip_serializing_if = "Option::is_none")]
    pub retrieved_at: Option<String>,
    /// Fetched content URL
    pub url: String,
}

#[derive(Serialize, Deserialize)]
pub struct RequestWebSearchResultBlock {
    pub encrypted_content: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub page_age: Option<String>,
    pub title: String,
    pub r#type: String,
    pub url: String,
}

#[derive(Serialize, Deserialize)]
pub struct RequestWebSearchToolResultError {
    pub error_code: WebSearchToolResultErrorCode,
    pub r#type: String,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
pub enum BashCodeExecutionToolResultErrorCode {
    #[serde(rename = "invalid_tool_input")]
    InvalidToolInput,
    #[serde(rename = "unavailable")]
    Unavailable,
    #[serde(rename = "too_many_requests")]
    TooManyRequests,
    #[serde(rename = "execution_time_exceeded")]
    ExecutionTimeExceeded,
    #[serde(rename = "output_file_too_large")]
    OutputFileTooLarge,
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseBashCodeExecutionOutputBlock {
    pub file_id: String,
    pub r#type: String,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
pub enum CodeExecutionToolResultErrorCode {
    #[serde(rename = "invalid_tool_input")]
    InvalidToolInput,
    #[serde(rename = "unavailable")]
    Unavailable,
    #[serde(rename = "too_many_requests")]
    TooManyRequests,
    #[serde(rename = "execution_time_exceeded")]
    ExecutionTimeExceeded,
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseCodeExecutionOutputBlock {
    pub file_id: String,
    pub r#type: String,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
pub enum TextEditorCodeExecutionToolResultErrorCode {
    #[serde(rename = "invalid_tool_input")]
    InvalidToolInput,
    #[serde(rename = "unavailable")]
    Unavailable,
    #[serde(rename = "too_many_requests")]
    TooManyRequests,
    #[serde(rename = "execution_time_exceeded")]
    ExecutionTimeExceeded,
    #[serde(rename = "file_not_found")]
    FileNotFound,
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
pub enum ToolSearchToolResultErrorCode {
    #[serde(rename = "invalid_tool_input")]
    InvalidToolInput,
    #[serde(rename = "unavailable")]
    Unavailable,
    #[serde(rename = "too_many_requests")]
    TooManyRequests,
    #[serde(rename = "execution_time_exceeded")]
    ExecutionTimeExceeded,
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseToolReferenceBlock {
    pub tool_name: String,
    pub r#type: String,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
pub enum WebFetchToolResultErrorCode {
    #[serde(rename = "invalid_tool_input")]
    InvalidToolInput,
    #[serde(rename = "url_too_long")]
    UrlTooLong,
    #[serde(rename = "url_not_allowed")]
    UrlNotAllowed,
    #[serde(rename = "url_not_accessible")]
    UrlNotAccessible,
    #[serde(rename = "unsupported_content_type")]
    UnsupportedContentType,
    #[serde(rename = "too_many_requests")]
    TooManyRequests,
    #[serde(rename = "max_uses_exceeded")]
    MaxUsesExceeded,
    #[serde(rename = "unavailable")]
    Unavailable,
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseDocumentBlock {
    /// Citation configuration for the document
    pub citations: Option<ResponseCitationsConfig>,
    pub source: ResponseDocumentBlockSource,
    /// The title of the document
    pub title: Option<String>,
    pub r#type: String,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
#[serde(tag = "type")]
pub enum ResponseDocumentBlockSource {
    #[serde(rename = "base64")]
    Base64(Base64PDFSource),
    #[serde(rename = "text")]
    Text(PlainTextSource),
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
#[non_exhaustive]
pub enum WebSearchToolResultErrorCode {
    #[serde(rename = "invalid_tool_input")]
    InvalidToolInput,
    #[serde(rename = "unavailable")]
    Unavailable,
    #[serde(rename = "max_uses_exceeded")]
    MaxUsesExceeded,
    #[serde(rename = "too_many_requests")]
    TooManyRequests,
    #[serde(rename = "query_too_long")]
    QueryTooLong,
    #[serde(rename = "request_too_large")]
    RequestTooLarge,
    #[serde(other)]
    #[serde(skip_serializing)]
    Unknown,
}

#[derive(Serialize, Deserialize)]
pub struct RequestBashCodeExecutionOutputBlock {
    pub file_id: String,
    pub r#type: String,
}

#[derive(Serialize, Deserialize)]
pub struct RequestCodeExecutionOutputBlock {
    pub file_id: String,
    pub r#type: String,
}

#[derive(Serialize, Deserialize)]
pub struct ResponseCitationsConfig {
    pub enabled: bool,
}
