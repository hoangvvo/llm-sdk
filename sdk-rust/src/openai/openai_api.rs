use serde::{Deserialize, Serialize};
use serde_json::Value;

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionCreateParamsBase {
    #[serde(rename = "model")]
    pub model: String,
    #[serde(rename = "messages")]
    pub messages: Vec<ChatCompletionMessageParam>,
    #[serde(rename = "max_tokens")]
    pub max_tokens: Option<i64>,
    #[serde(rename = "temperature")]
    pub temperature: Option<f64>,
    #[serde(rename = "top_p")]
    pub top_p: Option<f64>,
    #[serde(rename = "presence_penalty")]
    pub presence_penalty: Option<f64>,
    #[serde(rename = "frequency_penalty")]
    pub frequency_penalty: Option<f64>,
    #[serde(rename = "seed")]
    pub seed: Option<i64>,
    #[serde(rename = "tools")]
    pub tools: Option<Vec<ChatCompletionTool>>,
    #[serde(rename = "tool_choice")]
    pub tool_choice: Option<ChatCompletionToolChoiceOption>,
    #[serde(rename = "response_format")]
    pub response_format: Option<ResponseFormat>,
    #[serde(rename = "modalities")]
    pub modalities: Option<Vec<ChatCompletionModality>>,
    #[serde(rename = "audio")]
    pub audio: Option<ChatCompletionAudioParam>,
    #[serde(flatten)]
    pub extra: Option<Value>,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(tag = "stream")]
pub enum ChatCompletionCreateParams {
    #[serde(rename = "false")]
    NonStreaming {
        #[serde(flatten)]
        base: ChatCompletionCreateParamsBase,
    },
    #[serde(rename = "true")]
    Streaming {
        #[serde(flatten)]
        base: ChatCompletionCreateParamsBase,
        #[serde(rename = "stream_options")]
        stream_options: Option<ChatCompletionStreamOptions>,
    },
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(tag = "role")]
pub enum ChatCompletionMessageParam {
    #[serde(rename = "system")]
    System(ChatCompletionSystemMessageParam),
    #[serde(rename = "user")]
    User(ChatCompletionUserMessageParam),
    #[serde(rename = "assistant")]
    Assistant(ChatCompletionAssistantMessageParam),
    #[serde(rename = "tool")]
    Tool(ChatCompletionToolMessageParam),
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionSystemMessageParam {
    #[serde(rename = "content")]
    pub content: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionUserMessageParam {
    #[serde(rename = "content")]
    pub content: Vec<ChatCompletionContentPart>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionAssistantMessageParam {
    #[serde(rename = "content")]
    pub content: Option<Vec<ChatCompletionContentPartText>>,
    #[serde(rename = "audio")]
    pub audio: Option<ChatCompletionAssistantMessageParamAudio>,
    #[serde(rename = "tool_calls")]
    pub tool_calls: Option<Vec<ChatCompletionMessageToolCall>>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionToolMessageParam {
    #[serde(rename = "content")]
    pub content: String,
    #[serde(rename = "tool_call_id")]
    pub tool_call_id: String,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(tag = "type")]
pub enum ChatCompletionContentPart {
    #[serde(rename = "text")]
    Text(ChatCompletionContentPartText),
    #[serde(rename = "image_url")]
    Image(ChatCompletionContentPartImage),
    #[serde(rename = "input_audio")]
    InputAudio(ChatCompletionContentPartInputAudio),
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionContentPartText {
    #[serde(rename = "text")]
    pub text: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionContentPartImage {
    #[serde(rename = "image_url")]
    pub image_url: ChatCompletionContentPartImageUrl,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionContentPartImageUrl {
    #[serde(rename = "url")]
    pub url: String,
    #[serde(rename = "detail")]
    pub detail: Option<ChatCompletionContentPartImageDetail>,
}

#[derive(Debug, Serialize, Deserialize)]
pub enum ChatCompletionContentPartImageDetail {
    #[serde(rename = "auto")]
    Auto,
    #[serde(rename = "low")]
    Low,
    #[serde(rename = "high")]
    High,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionContentPartInputAudio {
    #[serde(rename = "input_audio")]
    pub input_audio: ChatCompletionContentPartInputAudioData,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionContentPartInputAudioData {
    #[serde(rename = "data")]
    pub data: String,
    #[serde(rename = "format")]
    pub format: ChatCompletionContentPartInputAudioFormat,
}

#[derive(Debug, Serialize, Deserialize)]
pub enum ChatCompletionContentPartInputAudioFormat {
    #[serde(rename = "wav")]
    Wav,
    #[serde(rename = "mp3")]
    Mp3,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionAssistantMessageParamAudio {
    #[serde(rename = "id")]
    pub id: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionMessageToolCall {
    #[serde(rename = "id")]
    pub id: String,
    #[serde(rename = "function")]
    pub function: ChatCompletionMessageToolCallFunction,
    #[serde(rename = "type")]
    pub type_: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionMessageToolCallFunction {
    #[serde(rename = "name")]
    pub name: String,
    #[serde(rename = "arguments")]
    pub arguments: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionTool {
    #[serde(rename = "function")]
    pub function: ChatCompletionToolFunction,
    #[serde(rename = "type")]
    pub type_: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionToolFunction {
    #[serde(rename = "name")]
    pub name: String,
    #[serde(rename = "description")]
    pub description: Option<String>,
    #[serde(rename = "parameters")]
    pub parameters: Option<Value>,
    #[serde(rename = "strict")]
    pub strict: Option<bool>,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(untagged)]
pub enum ChatCompletionToolChoiceOption {
    #[serde(rename = "none")]
    None,
    #[serde(rename = "auto")]
    Auto,
    #[serde(rename = "required")]
    Required,
    Named(ChatCompletionNamedToolChoice),
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionNamedToolChoice {
    #[serde(rename = "function")]
    pub function: ChatCompletionNamedToolChoiceFunction,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionNamedToolChoiceFunction {
    #[serde(rename = "name")]
    pub name: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub enum ChatCompletionModality {
    #[serde(rename = "text")]
    Text,
    #[serde(rename = "audio")]
    Audio,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionAudioParam {
    #[serde(rename = "format")]
    pub format: ChatCompletionAudioParamFormat,
    #[serde(rename = "voice")]
    pub voice: ChatCompletionAudioParamVoice,
}

#[derive(Debug, Serialize, Deserialize)]
pub enum ChatCompletionAudioParamFormat {
    #[serde(rename = "wav")]
    Wav,
    #[serde(rename = "mp3")]
    Mp3,
    #[serde(rename = "flac")]
    Flac,
    #[serde(rename = "opus")]
    Opus,
    #[serde(rename = "pcm16")]
    Pcm16,
}

#[derive(Debug, Serialize, Deserialize)]
pub enum ChatCompletionAudioParamVoice {
    #[serde(rename = "alloy")]
    Alloy,
    #[serde(rename = "ash")]
    Ash,
    #[serde(rename = "ballad")]
    Ballad,
    #[serde(rename = "coral")]
    Coral,
    #[serde(rename = "echo")]
    Echo,
    #[serde(rename = "sage")]
    Sage,
    #[serde(rename = "shimmer")]
    Shimmer,
    #[serde(rename = "verse")]
    Verse,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionStreamOptions {
    #[serde(rename = "include_usage")]
    pub include_usage: Option<bool>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionChunkChoiceDelta {
    #[serde(rename = "content")]
    pub content: Option<String>,
    #[serde(rename = "refusal")]
    pub refusal: Option<String>,
    #[serde(rename = "role")]
    pub role: Option<ChatCompletionChunkChoiceDeltaRole>,
    #[serde(rename = "tool_calls")]
    pub tool_calls: Option<Vec<ChatCompletionChunkChoiceDeltaToolCall>>,
    #[serde(rename = "audio")]
    pub audio: Option<ChatCompletionChunkChoiceDeltaAudio>,
}

#[derive(Debug, Serialize, Deserialize)]
pub enum ChatCompletionChunkChoiceDeltaRole {
    #[serde(rename = "developer")]
    Developer,
    #[serde(rename = "system")]
    System,
    #[serde(rename = "user")]
    User,
    #[serde(rename = "assistant")]
    Assistant,
    #[serde(rename = "tool")]
    Tool,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionChunkChoiceDeltaAudio {
    #[serde(rename = "id")]
    pub id: String,
    #[serde(rename = "data")]
    pub data: String,
    #[serde(rename = "transcript")]
    pub transcript: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionChunkChoiceDeltaToolCall {
    #[serde(rename = "index")]
    pub index: f64,
    #[serde(rename = "id")]
    pub id: Option<String>,
    #[serde(rename = "function")]
    pub function: ChatCompletionChunkChoiceDeltaToolCallFunction,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionChunkChoiceDeltaToolCallFunction {
    #[serde(rename = "name")]
    pub name: String,
    #[serde(rename = "arguments")]
    pub arguments: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CompletionUsage {
    #[serde(rename = "completion_tokens")]
    pub completion_tokens: f64,
    #[serde(rename = "prompt_tokens")]
    pub prompt_tokens: f64,
    #[serde(rename = "total_tokens")]
    pub total_tokens: f64,
    #[serde(rename = "completion_tokens_details")]
    pub completion_tokens_details: Option<CompletionTokensDetails>,
    #[serde(rename = "prompt_tokens_details")]
    pub prompt_tokens_details: Option<PromptTokensDetails>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CompletionTokensDetails {
    #[serde(rename = "accepted_prediction_tokens")]
    pub accepted_prediction_tokens: Option<f64>,
    #[serde(rename = "audio_tokens")]
    pub audio_tokens: Option<f64>,
    #[serde(rename = "reasoning_tokens")]
    pub reasoning_tokens: Option<f64>,
    #[serde(rename = "rejected_prediction_tokens")]
    pub rejected_prediction_tokens: Option<f64>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct PromptTokensDetails {
    #[serde(rename = "audio_tokens")]
    pub audio_tokens: Option<f64>,
    #[serde(rename = "cached_tokens")]
    pub cached_tokens: Option<f64>,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(tag = "type")]
pub enum ResponseFormat {
    #[serde(rename = "json_object")]
    JSONObject,
    #[serde(rename = "json_schema")]
    JSONSchema(ResponseFormatJSONSchema),
    #[serde(rename = "text")]
    Text,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ResponseFormatJSONSchema {
    #[serde(rename = "json_schema")]
    pub json_schema: ResponseFormatJSONSchemaContent,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ResponseFormatJSONSchemaContent {
    #[serde(rename = "name")]
    pub name: String,
    #[serde(rename = "schema")]
    pub schema: Value,
    #[serde(rename = "strict")]
    pub strict: bool,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletion {
    #[serde(rename = "id")]
    pub id: String,
    #[serde(rename = "choices")]
    pub choices: Vec<ChatCompletionChoice>,
    #[serde(rename = "created")]
    pub created: i64,
    #[serde(rename = "model")]
    pub model: String,
    // This field is expected to always be "chat.completion"
    #[serde(rename = "object")]
    pub object: String,
    #[serde(rename = "usage")]
    pub usage: Option<CompletionUsage>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionChoice {
    #[serde(rename = "index")]
    pub index: i64,
    #[serde(rename = "message")]
    pub message: ChatCompletionMessage,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionMessage {
    #[serde(rename = "content")]
    pub content: Option<String>,
    #[serde(rename = "refusal")]
    pub refusal: Option<String>,
    // This field should always be "assistant"
    #[serde(rename = "role")]
    pub role: String,
    #[serde(rename = "audio")]
    pub audio: Option<ChatCompletionAudio>,
    #[serde(rename = "tool_calls")]
    pub tool_calls: Option<Vec<ChatCompletionMessageToolCall>>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionAudio {
    #[serde(rename = "id")]
    pub id: String,
    #[serde(rename = "data")]
    pub data: String,
    #[serde(rename = "expires_at")]
    pub expires_at: i64,
    #[serde(rename = "transcript")]
    pub transcript: String,
}
