use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::HashMap;

/// Loosely describe audio format. Some values (e.g., 'wav') denote containers;
/// others (e.g., 'linear16') specify encoding only; cannot describe containers
/// that can contain different audio encodings.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
#[serde(rename_all = "lowercase")]
pub enum AudioFormat {
    Wav,
    Mp3,
    Linear16,
    Flac,
    Mulaw,
    Alaw,
    Aac,
    Opus,
}

/// A part of the message.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
#[serde(tag = "type", rename_all = "kebab-case")]
pub enum Part {
    Text(TextPart),
    Image(ImagePart),
    Audio(AudioPart),
    File(FilePart),
    Source(SourcePart),
    ToolCall(ToolCallPart),
    ToolResult(ToolResultPart),
    Reasoning(ReasoningPart),
}

/// Delta parts used in partial updates.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
#[serde(tag = "type", rename_all = "kebab-case")]
pub enum PartDelta {
    Text(TextPartDelta),
    ToolCall(ToolCallPartDelta),
    ToolResult(ToolResultPartDelta),
    Image(ImagePartDelta),
    Audio(AudioPartDelta),
    Reasoning(ReasoningPartDelta),
}

/// A message in an LLM conversation history.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
#[serde(tag = "role", rename_all = "lowercase")]
pub enum Message {
    User(UserMessage),
    Assistant(AssistantMessage),
    Tool(ToolMessage),
}

/// Defines the modality of content (e.g., text or audio) in LLM responses.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
#[serde(rename_all = "lowercase")]
pub enum Modality {
    Text,
    Image,
    Audio,
}

/// Determines how the model should choose which tool to use.
/// - "auto" The model will automatically choose the tool to use or not use any
///   tools.
/// - "none" The model will not use any tools.
/// - "required" The model will be forced to use a tool.
/// - { type: "tool", toolName: "toolName" } The model will use the specified
///   tool.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
#[serde(tag = "type", rename_all = "lowercase")]
pub enum ToolChoiceOption {
    /// The model will automatically choose the tool to use or not use any
    /// tools.
    Auto,
    /// The model will not use any tools.
    None,
    /// The model will be forced to use a tool.
    Required,
    /// The model will use the specified tool.
    Tool(ToolChoiceTool),
}

/// The format that the model must output.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
#[serde(tag = "type", rename_all = "lowercase")]
pub enum ResponseFormatOption {
    /// Specifies that the model response should be in plain text format.
    Text,
    Json(ResponseFormatJson),
}

/// The capabilities supported by the model.
#[allow(clippy::struct_excessive_bools)]
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Default)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct LanguageModelCapabilities {
    pub text_input: bool,
    pub text_output: bool,
    pub image_input: bool,
    pub image_output: bool,
    pub audio_input: bool,
    pub audio_output: bool,
    pub function_calling: bool,
    pub structured_output: bool,
    pub citation: bool,
    pub reasoning: bool,
}

/// A part of the message that contains text.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct TextPart {
    pub text: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub citations: Option<Vec<Citation>>,
    /// An opaque provider signature used to preserve text-part continuity when
    /// returning the part to the same provider.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub signature: Option<String>,
}

/// A part of the message that contains an image.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ImagePart {
    /// The MIME type of the image. E.g. "image/jpeg", "image/png".
    pub mime_type: String,
    /// Base64 content; either `data` or `url` must be provided.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub data: Option<String>,
    /// Fetched by the provider; URL support depends on the model.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub url: Option<String>,
    /// The width of the image in pixels.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub width: Option<u32>,
    /// The height of the image in pixels.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub height: Option<u32>,
    /// The ID of the image part, if applicable
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
}

/// A part of the message that contains an audio.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct AudioPart {
    /// Base64 content; either `data` or `url` must be provided.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub data: Option<String>,
    /// Fetched by the provider; URL support depends on the model.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub url: Option<String>,
    /// The format of the audio.
    pub format: AudioFormat,
    /// The sample rate of the audio. E.g. 44100, 48000.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub sample_rate: Option<u32>,
    /// The number of channels of the audio. E.g. 1, 2.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub channels: Option<u32>,
    /// The transcript of the audio.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub transcript: Option<String>,
    /// The ID of the audio part, if applicable
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
}

/// Document or video input; accepted MIME types depend on the model.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct FilePart {
    /// The MIME type of the file. E.g. "application/pdf", "text/plain",
    /// "video/mp4".
    pub mime_type: String,
    /// The file contents in the format accepted by the model.
    /// Either `data` or `url` must be provided.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub data: Option<String>,
    /// Fetched by the provider; URL support depends on the model.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub url: Option<String>,
    /// Document name; some models require it for inline files.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub filename: Option<String>,
}

/// A part of the message that contains a source with structured content.
/// It will be used for citation for supported models.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct SourcePart {
    /// The URL or identifier of the document.
    pub source: String,
    /// The title of the document.
    pub title: String,
    /// The content of the document.
    pub content: Vec<Part>,
}

/// A part of the message that represents a call to a tool the model wants to
/// use.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ToolCallPart {
    /// The ID of the tool call, used to match the tool result with the tool
    /// call.
    pub tool_call_id: String,
    pub call: ToolCall,
    /// The provider-specific signature used to preserve reasoning/tool
    /// continuity.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub signature: Option<String>,
    /// The ID of the tool call, if applicable
    /// This is different from `tool_call_id`, which is the ID used to match the
    /// tool result with the tool call.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
}

/// A part of the message that represents the result of a tool call.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ToolResultPart {
    /// The ID of the tool call from previous assistant message.
    pub tool_call_id: String,
    pub result: ToolResult,
    /// The terminal status of the tool call.
    pub status: ToolResultStatus,
}

/// The terminal status of a tool call.
#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
#[serde(rename_all = "snake_case")]
pub enum ToolResultStatus {
    Completed,
    Failed,
    Cancelled,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum ToolCall {
    Function(FunctionToolCall),
    WebSearch(WebSearchToolCall),
    ToolSearch(ToolSearchToolCall),
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct FunctionToolCall {
    pub name: String,
    pub args: Value,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
#[serde(rename_all = "snake_case")]
pub enum WebSearchToolCallStatus {
    InProgress,
    Searching,
    Completed,
    Failed,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum WebSearchAction {
    Search { queries: Vec<String> },
    OpenPage { url: String },
    FindInPage { url: String, pattern: String },
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct WebSearchToolCall {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub action: Option<WebSearchAction>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub status: Option<WebSearchToolCallStatus>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
#[serde(rename_all = "snake_case")]
pub enum ToolSearchToolCallStatus {
    InProgress,
    Completed,
    Failed,
}

/// A provider-hosted search over the deferred tools of the request.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ToolSearchToolCall {
    /// Opaque search arguments; preserve for conversation replay.
    pub args: Value,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub status: Option<ToolSearchToolCallStatus>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum ToolResult {
    Function(FunctionToolResult),
    WebSearch(WebSearchToolResult),
    ToolSearch(ToolSearchToolResult),
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct FunctionToolResult {
    pub name: String,
    pub content: Vec<Part>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct WebSearchSource {
    pub url: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub title: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub page_age: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub signature: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct WebSearchToolResult {
    pub sources: Vec<WebSearchSource>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error_code: Option<String>,
}

/// Discovered tools made available to the model.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ToolSearchToolResult {
    /// Discovered tool names; each must be declared in the request's `tools`.
    pub tool_names: Vec<String>,
    /// Provider error code required to replay a failed hosted tool search.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error_code: Option<String>,
}

// A part of the message that represents the model reasoning.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ReasoningPart {
    /// The reasoning text content.
    pub text: String,
    /// The reasoning internal signature
    #[serde(skip_serializing_if = "Option::is_none")]
    pub signature: Option<String>,
    /// The ID of the reasoning part, if applicable
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct Citation {
    /**
     * The URL or identifier of the document being cited.
     */
    pub source: String,
    /**
     * The title of the document being cited.
     */
    #[serde(skip_serializing_if = "Option::is_none")]
    pub title: Option<String>,
    /**
     * The text snippet from the document being cited.
     */
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cited_text: Option<String>,
    /**
     * The start index of the document content part being cited.
     */
    #[serde(skip_serializing_if = "Option::is_none")]
    pub start_index: Option<usize>,
    /**
     * The end index of the document content part being cited.
     */
    #[serde(skip_serializing_if = "Option::is_none")]
    pub end_index: Option<usize>,
    /**
     * An opaque provider signature used to preserve citation continuity
     * when returning it to the same provider.
     */
    #[serde(skip_serializing_if = "Option::is_none")]
    pub signature: Option<String>,
}

/// Represents a message sent by the user.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct UserMessage {
    pub content: Vec<Part>,
}

/// Represents a message generated by the model.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct AssistantMessage {
    pub content: Vec<Part>,
}

/// A delta update for a text part, used in streaming or incremental updates of
/// a message.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct TextPartDelta {
    pub text: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub citation: Option<CitationDelta>,
    /// An opaque provider signature used to preserve text-part continuity when
    /// returning the part to the same provider.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub signature: Option<String>,
}

/// A delta update for a citation part, used in streaming of citation messages.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Default)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct CitationDelta {
    /// The type of the citation delta.
    #[serde(rename = "type")]
    pub r#type: String,
    /// The URL or identifier of the document being cited.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub source: Option<String>,
    /// The title of the document being cited.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub title: Option<String>,
    /// The text snippet from the document being cited.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cited_text: Option<String>,
    /// The start index of the document content part being cited.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub start_index: Option<usize>,
    /// The end index of the document content part being cited.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub end_index: Option<usize>,
    /// An opaque provider signature used to preserve citation continuity when
    /// returning it to the same provider.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub signature: Option<String>,
}

/// A delta update for a tool call part, used in streaming of a tool invocation.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ToolCallPartDelta {
    /// The ID of the tool call, used to match the tool result with the tool
    /// call.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_call_id: Option<String>,
    pub call: ToolCallDelta,
    /// The provider-specific signature used to preserve reasoning/tool
    /// continuity.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub signature: Option<String>,
    /// The ID of the tool call, if applicable
    /// This is different from `tool_call_id`, which is the ID used to match the
    /// tool result with the tool call.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum ToolCallDelta {
    Function(FunctionToolCallDelta),
    WebSearch(WebSearchToolCallDelta),
    ToolSearch(ToolSearchToolCallDelta),
}

impl Default for ToolCallDelta {
    fn default() -> Self {
        Self::Function(FunctionToolCallDelta::default())
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct FunctionToolCallDelta {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub args: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct WebSearchToolCallDelta {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub action: Option<WebSearchAction>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub status: Option<WebSearchToolCallStatus>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ToolSearchToolCallDelta {
    /// The partial JSON string of the search arguments.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub args: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub status: Option<ToolSearchToolCallStatus>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ToolResultPartDelta {
    pub tool_call_id: String,
    pub result: ToolResult,
    pub status: ToolResultStatus,
}

/// A delta update for an image part, used in streaming of an image message.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ImagePartDelta {
    /// The MIME type of the image. E.g. "image/jpeg", "image/png".
    #[serde(skip_serializing_if = "Option::is_none")]
    pub mime_type: Option<String>,
    /// The base64-encoded image data.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub data: Option<String>,
    /// The width of the image in pixels.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub width: Option<u32>,
    /// The height of the image in pixels.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub height: Option<u32>,
    /// The ID of the image part, if applicable
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
}

/// A delta update for an audio part, used in streaming of an audio message.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct AudioPartDelta {
    /// The base64-encoded audio data.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub data: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub format: Option<AudioFormat>,
    /// The sample rate of the audio. E.g. 44100, 48000.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub sample_rate: Option<u32>,
    /// The number of channels of the audio. E.g. 1, 2.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub channels: Option<u32>,
    /// The transcript of the audio.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub transcript: Option<String>,
    /// The ID of the audio part, if applicable
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
}

// A delta update for a reasoning part, used in streaming of reasoning messages.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ReasoningPartDelta {
    /// The reasoning text content.
    pub text: String,
    /// The reasoning internal signature
    #[serde(skip_serializing_if = "Option::is_none")]
    pub signature: Option<String>,
    /// The ID of the reasoning part, if applicable
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
}

/// Represents a delta update in a message's content, enabling partial streaming
/// updates in LLM responses.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ContentDelta {
    pub index: usize,
    pub part: PartDelta,
}

/// Represents a JSON schema.
pub type JSONSchema = Value;

/// Represents a tool that can be used by the model.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
#[serde(tag = "type")]
pub enum Tool {
    #[serde(rename = "function")]
    Function(FunctionTool),
    #[serde(rename = "web_search")]
    WebSearch(WebSearchTool),
    #[serde(rename = "tool_search")]
    ToolSearch(ToolSearchTool),
}

/// Represents a client-executed function tool that can be used by the model.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct FunctionTool {
    /// The name of the tool.
    pub name: String,
    /// A description of the tool.
    pub description: String,
    /// The JSON schema of the parameters that the tool accepts. The type must
    /// be "object".
    pub parameters: JSONSchema,
    /// Hide the tool from the model until a `tool_search` tool discovers it.
    /// Providers without tool search ignore this flag and load the tool
    /// eagerly.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub defer_loading: Option<bool>,
}

/// Loads deferred function tools on demand through hosted search.
#[derive(Debug, Clone, Default, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ToolSearchTool {
    /// The search algorithm, when the provider offers a choice. Defaults to
    /// "bm25".
    #[serde(skip_serializing_if = "Option::is_none")]
    pub strategy: Option<ToolSearchStrategy>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
#[serde(rename_all = "lowercase")]
pub enum ToolSearchStrategy {
    Regex,
    Bm25,
}

/// Represents a provider-hosted web search tool.
#[derive(Debug, Clone, Default, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct WebSearchTool {
    /// Restricts search results to these domains when supported by the
    /// provider.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_domains: Option<Vec<String>>,
    /// Limits the number of searches the provider may perform when supported.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_uses: Option<u32>,
    /// An approximate user location used to localize web search results.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub user_location: Option<WebSearchUserLocation>,
}

/// An approximate user location used to localize web search results.
#[derive(Debug, Clone, Default, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct WebSearchUserLocation {
    /// The city of the user.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub city: Option<String>,
    /// The region or state of the user.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub region: Option<String>,
    /// The two-letter ISO 3166-1 country code of the user.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub country: Option<String>,
    /// The IANA timezone of the user.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub timezone: Option<String>,
}

/// Represents tool result in the message history.
/// The only parts of `ToolMessage` should be Part(ToolResultPart).
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ToolMessage {
    pub content: Vec<Part>,
}

/// A breakdown of `input_tokens` or `output_tokens`, using the provider's own
/// counting.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ModelTokensDetails {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub text_tokens: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cached_text_tokens: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audio_tokens: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cached_audio_tokens: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub image_tokens: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cached_image_tokens: Option<u32>,
    /// Cache reads, billed at the cached-input rate.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cached_tokens: Option<u32>,
    /// Cache writes, billed separately from cache reads.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_write_tokens: Option<u32>,
    /// The subset of `cache_write_tokens` stored with extended retention (see
    /// `cache_retention`), which some providers bill at a higher rate.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub extended_cache_write_tokens: Option<u32>,
    /// The tokens spent on reasoning.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reasoning_tokens: Option<u32>,
}

/// Hosted tool usage billed per request, separately from tokens.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ModelServerToolUsage {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub web_search_requests: Option<u32>,
}

/// Represents the token usage of the model.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ModelUsage {
    /// The input tokens as reported by the provider. Whether cached and
    /// cache-write tokens are included depends on the provider; see
    /// `ModelUsageCostOptions`.
    pub input_tokens: u32,
    /// The output tokens as reported by the provider. Whether reasoning tokens
    /// are included depends on the provider; see `ModelUsageCostOptions`.
    pub output_tokens: u32,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_tokens_details: Option<ModelTokensDetails>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output_tokens_details: Option<ModelTokensDetails>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub server_tool_use: Option<ModelServerToolUsage>,
}

/// Represents the response generated by the model.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ModelResponse {
    pub content: Vec<Part>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub usage: Option<ModelUsage>,
    /// The cost of the response.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cost: Option<f64>,
}

/// Represents a partial response from the language model, useful for streaming
/// output via async generator.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct PartialModelResponse {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub delta: Option<ContentDelta>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub usage: Option<ModelUsage>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cost: Option<f64>,
}

/// The model will use the specified tool.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ToolChoiceTool {
    pub tool_name: String,
}

/// Specifies that the model response should be in JSON format adhering to a
/// specified schema.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ResponseFormatJson {
    /// The name of the schema.
    pub name: String,
    /// The description of the schema.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub schema: Option<JSONSchema>,
}

/// Options for audio generation.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct AudioOptions {
    /// The format of the audio.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub format: Option<AudioFormat>,
    /// The provider-specifc voice ID to use for audio generation.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub voice: Option<String>,
    /// The language code for the audio generation.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub language: Option<String>,
}

/// Options for reasoning generation.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ReasoningOptions {
    /// Whether to enable reasoning output.
    pub enabled: bool,
    /// Specify the budget tokens for reasoning generation.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub budget_tokens: Option<u32>,
}

/// Defines the input parameters for the language model completion.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct LanguageModelInput {
    /// A system prompt is a way of providing context and instructions to the
    /// model
    #[serde(skip_serializing_if = "Option::is_none")]
    pub system_prompt: Option<String>,
    /// A list of messages comprising the conversation so far.
    pub messages: Vec<Message>,
    /// Definitions of tools that the model may use.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tools: Option<Vec<Tool>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_choice: Option<ToolChoiceOption>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub response_format: Option<ResponseFormatOption>,
    /// The maximum number of tokens that can be generated in the chat
    /// completion.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_tokens: Option<u32>,
    /// Amount of randomness injected into the response. Ranges from 0.0 to 1.0
    #[serde(skip_serializing_if = "Option::is_none")]
    pub temperature: Option<f64>,
    /// An alternative to sampling with temperature, called nucleus sampling,
    /// where the model considers the results of the tokens with `top_p`
    /// probability mass. Ranges from 0.0 to 1.0
    #[serde(skip_serializing_if = "Option::is_none")]
    pub top_p: Option<f64>,
    /// Only sample from the top K options for each subsequent token. Used to
    /// remove 'long tail' low probability responses. Must be a non-negative
    /// integer.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub top_k: Option<i32>,
    /// Positive values penalize new tokens based on whether they appear in the
    /// text so far, increasing the model's likelihood to talk about new topics.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub presence_penalty: Option<f64>,
    /// Positive values penalize new tokens based on their existing frequency in
    /// the text so far, decreasing the model's likelihood to repeat the same
    /// line verbatim.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub frequency_penalty: Option<f64>,
    /// The seed (integer), if set and supported by the model, to enable
    /// deterministic results.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub seed: Option<i64>,
    /// The modalities that the model should support.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub modalities: Option<Vec<Modality>>,
    /// A set of key/value pairs that store additional information about the
    /// request. This is forwarded to the model provider if supported.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub metadata: Option<HashMap<String, String>>,
    /// Options for audio generation.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audio: Option<AudioOptions>,
    /// Options for reasoning generation.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reasoning: Option<ReasoningOptions>,
    /// Opts into the provider's prompt cache and chooses how long entries are
    /// kept. `Standard` uses the provider's default retention and
    /// `Extended` the longest it offers. Providers without a retention
    /// setting ignore this option.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_retention: Option<CacheRetention>,
}

/// How long prompt cache entries are kept.
#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
#[serde(rename_all = "lowercase")]
pub enum CacheRetention {
    Standard,
    Extended,
}

/// A metadata property that describes the pricing of the model.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct LanguageModelPricing {
    /// The cost in USD per single text token for input.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_cost_per_text_token: Option<f64>,
    /// The cost in USD per single cached input token.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_cost_per_cached_token: Option<f64>,
    /// The cost in USD per single cache-write input token.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_cost_per_cache_write_token: Option<f64>,
    /// The cost in USD per single cache-write input token stored with extended
    /// retention. Defaults to `input_cost_per_cache_write_token`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_cost_per_extended_cache_write_token: Option<f64>,
    /// The cost in USD per single cached text token for input.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_cost_per_cached_text_token: Option<f64>,
    /// The cost in USD per single text token for output.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output_cost_per_text_token: Option<f64>,
    /// The cost in USD per single audio token for input.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_cost_per_audio_token: Option<f64>,
    /// The cost in USD per single cached audio token for input.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_cost_per_cached_audio_token: Option<f64>,
    /// The cost in USD per single audio token for output.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output_cost_per_audio_token: Option<f64>,
    /// The cost in USD per single image token for input.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_cost_per_image_token: Option<f64>,
    /// The cost in USD per single cached image token for input.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_cost_per_cached_image_token: Option<f64>,
    /// The cost in USD per single image token for output.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output_cost_per_image_token: Option<f64>,
    /// The cost in USD per provider-hosted web search request.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cost_per_web_search_request: Option<f64>,
    /// Rate multipliers applied to requests whose input exceeds a token
    /// threshold.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub long_context: Option<LanguageModelLongContextPricing>,
}

/// Rate multipliers applied to requests whose input exceeds a token threshold.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct LanguageModelLongContextPricing {
    /// The request is priced with the multipliers when `input_tokens` exceeds
    /// this value.
    pub threshold_tokens: u32,
    /// The multiplier applied to every input rate. Defaults to 1.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_cost_multiplier: Option<f64>,
    /// The multiplier applied to every output rate. Defaults to 1.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output_cost_multiplier: Option<f64>,
}
