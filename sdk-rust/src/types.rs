use serde::{Deserialize, Serialize};
use serde_json::Value;

#[derive(Serialize, Deserialize, Debug)]
/// A part of the message that contains text.
pub struct TextPart {
    #[serde(rename = "text")]
    pub text: String,
    #[serde(rename = "id")]
    pub id: Option<String>,
}

#[derive(Serialize, Deserialize, Debug)]
/// A part of the message that contains an image.
pub struct ImagePart {
    #[serde(rename = "mime_type")]
    pub mime_type: String,
    #[serde(rename = "image_data")]
    pub image_data: String,
    #[serde(rename = "width")]
    pub width: Option<i64>,
    #[serde(rename = "height")]
    pub height: Option<i64>,
    #[serde(rename = "id")]
    pub id: Option<String>,
}

#[derive(Serialize, Deserialize, Debug)]
/// Loosely describe audio format. Some values (e.g., 'wav') denote containers;
/// others (e.g., 'linear16') specify encoding only; cannot describe containers
/// that can contain different audio encodings.
pub enum AudioFormat {
    #[serde(rename = "wav")]
    Wav,
    #[serde(rename = "mp3")]
    Mp3,
    #[serde(rename = "linear16")]
    Linear16,
    #[serde(rename = "flac")]
    Flac,
    #[serde(rename = "mulaw")]
    Mulaw,
    #[serde(rename = "alaw")]
    Alaw,
    #[serde(rename = "aac")]
    Aac,
    #[serde(rename = "opus")]
    Opus,
}

#[derive(Serialize, Deserialize, Debug)]
/// A part of the message that contains an audio.
pub struct AudioPart {
    #[serde(rename = "audio_data")]
    pub audio_data: String,
    #[serde(rename = "format")]
    pub format: Option<AudioFormat>,
    #[serde(rename = "sample_rate")]
    pub sample_rate: Option<i64>,
    #[serde(rename = "channels")]
    pub channels: Option<i64>,
    #[serde(rename = "transcript")]
    pub transcript: Option<String>,
    #[serde(rename = "id")]
    pub id: Option<String>,
}

#[derive(Serialize, Deserialize, Debug)]
/// A part of the message that represents a call to a tool the model wants to
/// use.
pub struct ToolCallPart {
    #[serde(rename = "tool_call_id")]
    pub tool_call_id: String,
    #[serde(rename = "tool_name")]
    pub tool_name: String,
    #[serde(rename = "args")]
    pub args: Option<Value>,
    #[serde(rename = "id")]
    pub id: Option<String>,
}

#[derive(Serialize, Deserialize, Debug)]
/// A part of the message that represents the result of a tool call.
pub struct ToolResultPart {
    #[serde(rename = "tool_call_id")]
    pub tool_call_id: String,
    #[serde(rename = "tool_name")]
    pub tool_name: String,
    #[serde(rename = "result")]
    pub result: Value,
    #[serde(rename = "is_error")]
    pub is_error: Option<bool>,
}

#[derive(Serialize, Deserialize, Debug)]
#[serde(tag = "type")]
/// A part of the message.
pub enum Part {
    #[serde(rename = "text")]
    Text(TextPart),
    #[serde(rename = "image")]
    Image(ImagePart),
    #[serde(rename = "audio")]
    Audio(AudioPart),
    #[serde(rename = "tool-call")]
    ToolCall(ToolCallPart),
    #[serde(rename = "tool-result")]
    ToolResult(ToolResultPart),
}

#[derive(Serialize, Deserialize, Debug)]
/// Represents a message sent by the user.
pub struct UserMessage {
    #[serde(rename = "content")]
    pub content: Vec<Part>,
}

#[derive(Serialize, Deserialize, Debug)]
/// Represents a message generated by the model.
pub struct AssistantMessage {
    #[serde(rename = "content")]
    pub content: Vec<Part>,
}

#[derive(Serialize, Deserialize, Debug)]
/// A delta update for a text part, used in streaming or incremental updates of
/// a message.
pub struct TextPartDelta {
    #[serde(rename = "text")]
    pub text: String,
    #[serde(rename = "id")]
    pub id: Option<String>,
}

#[derive(Serialize, Deserialize, Debug)]
/// A delta update for a tool call part, used in streaming of a tool invocation.
pub struct ToolCallPartDelta {
    #[serde(rename = "tool_call_id")]
    pub tool_call_id: Option<String>,
    #[serde(rename = "tool_name")]
    pub tool_name: Option<String>,
    #[serde(rename = "args")]
    pub args: Option<String>,
    #[serde(rename = "id")]
    pub id: Option<String>,
}

#[derive(Serialize, Deserialize, Debug)]
/// A delta update for an audio part, used in streaming of an audio message.
pub struct AudioPartDelta {
    #[serde(rename = "audio_data")]
    pub audio_data: Option<String>,
    #[serde(rename = "format")]
    pub format: Option<AudioFormat>,
    #[serde(rename = "sample_rate")]
    pub sample_rate: Option<i64>,
    #[serde(rename = "channels")]
    pub channels: Option<i64>,
    #[serde(rename = "transcript")]
    pub transcript: Option<String>,
    #[serde(rename = "id")]
    pub id: Option<String>,
}

#[derive(Serialize, Deserialize, Debug)]
#[serde(tag = "type")]
/// Delta parts used in partial updates.
pub enum ContentDeltaPart {
    #[serde(rename = "text")]
    Text(TextPartDelta),
    #[serde(rename = "tool-call")]
    ToolCall(ToolCallPartDelta),
    #[serde(rename = "audio")]
    Audio(AudioPartDelta),
}

#[derive(Serialize, Deserialize, Debug)]
/// Represents a delta update in a message's content, enabling partial streaming
/// updates in LLM responses.
pub struct ContentDelta {
    #[serde(rename = "index")]
    pub index: i64,
    #[serde(rename = "part")]
    pub part: ContentDeltaPart,
}

pub type JSONSchema = Value;

#[derive(Serialize, Deserialize, Debug)]
/// Represents a tool that can be used by the model.
pub struct Tool {
    #[serde(rename = "name")]
    pub name: String,
    #[serde(rename = "description")]
    pub description: String,
    #[serde(rename = "parameters")]
    pub parameters: Option<JSONSchema>,
}

#[derive(Serialize, Deserialize, Debug)]
/// Represents tool result in the message history.
pub struct ToolMessage {
    #[serde(rename = "content")]
    pub content: Vec<ToolResultPart>,
}

#[derive(Serialize, Deserialize, Debug)]
#[serde(tag = "role")]
/// A message in an LLM conversation history.
pub enum Message {
    #[serde(rename = "user")]
    User(UserMessage),
    #[serde(rename = "assistant")]
    Assistant(AssistantMessage),
    #[serde(rename = "tool")]
    Tool(ToolMessage),
}

#[derive(Serialize, Deserialize, Debug)]
/// Represents the token usage of the model.
pub struct ModelTokensDetails {
    #[serde(rename = "text_tokens")]
    pub text_tokens: Option<i64>,
    #[serde(rename = "cached_text_tokens")]
    pub cached_text_tokens: Option<i64>,
    #[serde(rename = "audio_tokens")]
    pub audio_tokens: Option<i64>,
    #[serde(rename = "cached_audio_tokens")]
    pub cached_audio_tokens: Option<i64>,
    #[serde(rename = "image_tokens")]
    pub image_tokens: Option<i64>,
    #[serde(rename = "cached_image_tokens")]
    pub cached_image_tokens: Option<i64>,
}

#[derive(Serialize, Deserialize, Debug)]
/// Represents the token usage of the model.
pub struct ModelUsage {
    #[serde(rename = "input_tokens")]
    pub input_tokens: i64,
    #[serde(rename = "output_tokens")]
    pub output_tokens: i64,
    #[serde(rename = "input_tokens_details")]
    pub input_tokens_details: Option<ModelTokensDetails>,
    #[serde(rename = "output_tokens_details")]
    pub output_tokens_details: Option<ModelTokensDetails>,
}

#[derive(Serialize, Deserialize, Debug)]
/// Represents the response generated by the model.
pub struct ModelResponse {
    #[serde(rename = "content")]
    pub content: Vec<Part>,
    #[serde(rename = "usage")]
    pub usage: Option<ModelUsage>,
    #[serde(rename = "cost")]
    pub cost: Option<f64>,
}

#[derive(Serialize, Deserialize, Debug)]
/// Represents a partial response from the language model, useful for streaming
/// output via async generator.
pub struct PartialModelResponse {
    #[serde(rename = "delta")]
    pub delta: ContentDelta,
}

#[derive(Serialize, Deserialize, Debug)]
/// The model will use the specified tool.
pub struct ToolChoiceTool {
    #[serde(rename = "tool_name")]
    pub tool_name: String,
}

#[derive(Serialize, Deserialize, Debug)]
#[serde(tag = "type")]
/// Determines how the model should choose which tool to use.
pub enum ToolChoiceOption {
    #[serde(rename = "auto")]
    Auto,
    #[serde(rename = "none")]
    None,
    #[serde(rename = "required")]
    Required,
    #[serde(rename = "tool")]
    Tool(ToolChoiceTool),
}

#[derive(Serialize, Deserialize, Debug)]
/// Specifies that the model response should be in JSON format adhering to a
/// specified schema.
pub struct ResponseFormatJson {
    #[serde(rename = "name")]
    pub name: String,
    #[serde(rename = "description")]
    pub description: Option<String>,
    #[serde(rename = "schema")]
    pub schema: Option<JSONSchema>,
}

#[derive(Serialize, Deserialize, Debug)]
#[serde(tag = "type")]
/// The format that the model must output.
pub enum ResponseFormatOption {
    #[serde(rename = "text")]
    Text,
    #[serde(rename = "json")]
    Json(ResponseFormatJson),
}

#[derive(Serialize, Deserialize, Debug)]
/// Defines the modality of content (e.g., text or audio) in LLM responses.
pub enum Modality {
    #[serde(rename = "text")]
    Text,
    #[serde(rename = "audio")]
    Audio,
}

#[derive(Serialize, Deserialize, Default)]
/// Defines the input parameters for the language model completion.
pub struct LanguageModelInput {
    #[serde(rename = "system_prompt")]
    pub system_prompt: Option<String>,
    #[serde(rename = "messages")]
    pub messages: Vec<Message>,
    #[serde(rename = "tools")]
    pub tools: Option<Vec<Tool>>,
    #[serde(rename = "tool_choice")]
    pub tool_choice: Option<ToolChoiceOption>,
    #[serde(rename = "response_format")]
    pub response_format: Option<ResponseFormatOption>,
    #[serde(rename = "max_tokens")]
    pub max_tokens: Option<i64>,
    #[serde(rename = "temperature")]
    pub temperature: Option<f64>,
    #[serde(rename = "top_p")]
    pub top_p: Option<f64>,
    #[serde(rename = "top_k")]
    pub top_k: Option<f64>,
    #[serde(rename = "presence_penalty")]
    pub presence_penalty: Option<f64>,
    #[serde(rename = "frequency_penalty")]
    pub frequency_penalty: Option<f64>,
    #[serde(rename = "seed")]
    pub seed: Option<i64>,
    #[serde(rename = "modalities")]
    pub modalities: Option<Vec<Modality>>,
    #[serde(rename = "extra")]
    pub extra: Option<Value>,
}

#[derive(Serialize, Deserialize, Debug)]
/// A metadata property that describes the capability of the model.
pub enum LanguageModelCapability {
    #[serde(rename = "structured-output")]
    StructuredOutput,
    #[serde(rename = "function-calling")]
    FunctionCalling,
    #[serde(rename = "structured-output-strict")]
    StructuredOutputStrict,
    #[serde(rename = "audio-input")]
    AudioInput,
    #[serde(rename = "audio-output")]
    AudioOutput,
    #[serde(rename = "image-input")]
    ImageInput,
    #[serde(rename = "image-output")]
    ImageOutput,
}

#[derive(Serialize, Deserialize, Debug)]
/// A metadata property that describes the pricing of the model.
pub struct LanguageModelPricing {
    #[serde(rename = "input_cost_per_text_token")]
    pub input_cost_per_text_token: Option<f64>,
    #[serde(rename = "input_cost_per_cached_text_token")]
    pub input_cost_per_cached_text_token: Option<f64>,
    #[serde(rename = "output_cost_per_text_token")]
    pub output_cost_per_text_token: Option<f64>,
    #[serde(rename = "input_cost_per_audio_token")]
    pub input_cost_per_audio_token: Option<f64>,
    #[serde(rename = "input_cost_per_cached_audio_token")]
    pub input_cost_per_cached_audio_token: Option<f64>,
    #[serde(rename = "output_cost_per_audio_token")]
    pub output_cost_per_audio_token: Option<f64>,
    #[serde(rename = "input_cost_per_image_token")]
    pub input_cost_per_image_token: Option<f64>,
    #[serde(rename = "input_cost_per_cached_image_token")]
    pub input_cost_per_cached_image_token: Option<f64>,
    #[serde(rename = "output_cost_per_image_token")]
    pub output_cost_per_image_token: Option<f64>,
}
