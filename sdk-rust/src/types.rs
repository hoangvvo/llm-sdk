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
#[derive(Debug, Clone, Serialize, Deserialize)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
#[serde(tag = "type", rename_all = "kebab-case")]
pub enum Part {
    Text(TextPart),
    Image(ImagePart),
    Audio(AudioPart),
    Source(SourcePart),
    ToolCall(ToolCallPart),
    ToolResult(ToolResultPart),
    Reasoning(ReasoningPart),
}

/// Delta parts used in partial updates.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
#[serde(tag = "type", rename_all = "kebab-case")]
pub enum PartDelta {
    Text(TextPartDelta),
    ToolCall(ToolCallPartDelta),
    Image(ImagePartDelta),
    Audio(AudioPartDelta),
    Reasoning(ReasoningPartDelta),
}

/// A message in an LLM conversation history.
#[derive(Debug, Clone, Serialize, Deserialize)]
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
#[derive(Debug, Clone, Serialize, Deserialize)]
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
#[derive(Debug, Clone, Serialize, Deserialize)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
#[serde(tag = "type", rename_all = "lowercase")]
pub enum ResponseFormatOption {
    /// Specifies that the model response should be in plain text format.
    Text,
    Json(ResponseFormatJson),
}

/// A metadata property that describes the capability of the model.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
#[serde(rename_all = "kebab-case")]
pub enum LanguageModelCapability {
    TextInput,
    TextOutput,
    ImageInput,
    ImageOutput,
    AudioInput,
    AudioOutput,
    FunctionCalling,
    StructuredOutput,
}

/// A part of the message that contains text.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct TextPart {
    pub text: String,
}

/// A part of the message that contains an image.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ImagePart {
    /// The MIME type of the image. E.g. "image/jpeg", "image/png".
    pub mime_type: String,
    /// The base64-encoded image data.
    pub image_data: String,
    /// The width of the image in pixels.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub width: Option<u32>,
    /// The height of the image in pixels.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub height: Option<u32>,
}

/// A part of the message that contains an audio.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct AudioPart {
    /// The base64-encoded audio data.
    pub audio_data: String,
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
    /// The Audio ID, if applicable.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audio_id: Option<String>,
}

/// A part of the message that contains a source with structured content.
/// It will be used for citation for supported models.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct SourcePart {
    /// The title of the document.
    pub title: String,
    /// The content of the document.
    pub content: Vec<Part>,
}

/// A part of the message that represents a call to a tool the model wants to
/// use.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ToolCallPart {
    /// The ID of the tool call, used to match the tool result with the tool
    /// call.
    pub tool_call_id: String,
    /// The name of the tool to call.
    pub tool_name: String,
    /// The arguments to pass to the tool.
    pub args: Value,
}

/// A part of the message that represents the result of a tool call.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ToolResultPart {
    /// The ID of the tool call from previous assistant message.
    pub tool_call_id: String,
    /// The name of the tool that was called.
    pub tool_name: String,
    /// The content of the tool result.
    pub content: Vec<Part>,
    /// Marks the tool result as an error.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub is_error: Option<bool>,
}

// A part of the message that represents the model reasoning.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ReasoningPart {
    /// The reasoning text content.
    pub text: String,
    /// The reasoning internal signature
    #[serde(skip_serializing_if = "Option::is_none")]
    pub signature: Option<String>,
}

/// Represents a message sent by the user.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct UserMessage {
    pub content: Vec<Part>,
}

/// Represents a message generated by the model.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct AssistantMessage {
    pub content: Vec<Part>,
}

/// A delta update for a text part, used in streaming or incremental updates of
/// a message.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct TextPartDelta {
    pub text: String,
}

/// A delta update for a tool call part, used in streaming of a tool invocation.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ToolCallPartDelta {
    /// The ID of the tool call, used to match the tool result with the tool
    /// call.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_call_id: Option<String>,
    /// The name of the tool to call.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_name: Option<String>,
    /// The partial JSON string of the arguments to pass to the tool.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub args: Option<String>,
}

/// A delta update for an image part, used in streaming of an image message.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ImagePartDelta {
    /// The MIME type of the image. E.g. "image/jpeg", "image/png".
    #[serde(skip_serializing_if = "Option::is_none")]
    pub mime_type: Option<String>,
    /// The base64-encoded image data.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub image_data: Option<String>,
    /// The width of the image in pixels.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub width: Option<u32>,
    /// The height of the image in pixels.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub height: Option<u32>,
}

/// A delta update for an audio part, used in streaming of an audio message.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct AudioPartDelta {
    /// The base64-encoded audio data.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audio_data: Option<String>,
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
    /// The audio ID, if applicable.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audio_id: Option<String>,
}

// A delta update for a reasoning part, used in streaming of reasoning messages.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ReasoningPartDelta {
    /// The reasoning text content.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub text: Option<String>,
    /// The reasoning internal signature
    #[serde(skip_serializing_if = "Option::is_none")]
    pub signature: Option<String>,
}

/// Represents a delta update in a message's content, enabling partial streaming
/// updates in LLM responses.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ContentDelta {
    pub index: usize,
    pub part: PartDelta,
}

/// Represents a JSON schema.
pub type JSONSchema = Value;

/// Represents a tool that can be used by the model.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct Tool {
    /// The name of the tool.
    pub name: String,
    /// A description of the tool.
    pub description: String,
    /// The JSON schema of the parameters that the tool accepts. The type must
    /// be "object".
    pub parameters: JSONSchema,
}

/// Represents tool result in the message history.
/// The only parts of `ToolMessage` should be Part(ToolResultPart).
#[derive(Debug, Clone, Serialize, Deserialize)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ToolMessage {
    pub content: Vec<Part>,
}

/// Represents the token usage of the model.
#[derive(Debug, Clone, Serialize, Deserialize)]
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
}

/// Represents the token usage of the model.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ModelUsage {
    pub input_tokens: u32,
    pub output_tokens: u32,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_tokens_details: Option<ModelTokensDetails>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output_tokens_details: Option<ModelTokensDetails>,
}

/// Represents the response generated by the model.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
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
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct PartialModelResponse {
    pub delta: Option<ContentDelta>,
    pub usage: Option<ModelUsage>,
}

/// The model will use the specified tool.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ToolChoiceTool {
    pub tool_name: String,
}

/// Specifies that the model response should be in JSON format adhering to a
/// specified schema.
#[derive(Debug, Clone, Serialize, Deserialize)]
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
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
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
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct ReasoningOptions {
    /// Whether to enable reasoning output.
    pub enabled: bool,
    /// Specify the budget tokens for reasoning generation.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub budget_tokens: Option<u32>,
}

/// Defines the input parameters for the language model completion.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
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
    pub reasoning: Option<ReasoningOptions>,
    /// Extra options that the model may support.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub extra: Option<LanguageModelInputExtra>,
}

pub type LanguageModelInputExtra = Value;

/// A metadata property that describes the pricing of the model.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[cfg_attr(feature = "utoipa", derive(utoipa::ToSchema))]
pub struct LanguageModelPricing {
    /// The cost in USD per single text token for input.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_cost_per_text_token: Option<f64>,
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
}
