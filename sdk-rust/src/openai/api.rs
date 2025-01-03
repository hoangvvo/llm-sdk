use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::HashMap;

use crate::JSONSchema;

// https://platform.openai.com/docs/api-reference/chat

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ChatCompletionCreateParams {
    /// A list of messages comprising the conversation so far. Depending on the
    /// [model](https://platform.openai.com/docs/models) you use, different message
    /// types (modalities) are supported, like
    /// [text](https://platform.openai.com/docs/guides/text-generation),
    /// [images](https://platform.openai.com/docs/guides/vision), and
    /// [audio](https://platform.openai.com/docs/guides/audio).
    pub messages: Vec<ChatCompletionMessageParam>,

    /// Model ID used to generate the response, like `gpt-4o` or `o3`. `OpenAI`
    /// offers a wide range of models with different capabilities,
    /// performance characteristics, and price points. Refer to the
    /// [model guide](https://platform.openai.com/docs/models) to browse and compare
    /// available models.
    pub model: String,

    /// Parameters for audio output. Required when audio output is requested
    /// with `modalities: ["audio"]`.
    /// [Learn more](https://platform.openai.com/docs/guides/audio).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audio: Option<ChatCompletionAudioParam>,

    /// Number between -2.0 and 2.0. Positive values penalize new tokens based
    /// on their existing frequency in the text so far, decreasing the
    /// model's likelihood to repeat the same line verbatim.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub frequency_penalty: Option<f64>,

    /// An upper bound for the number of tokens that can be generated for a
    /// completion, including visible output tokens and
    /// [reasoning tokens](https://platform.openai.com/docs/guides/reasoning).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_completion_tokens: Option<u32>,

    /// Output types that you would like the model to generate. Most models are
    /// capable of generating text, which is the default:
    ///
    /// `["text"]`
    ///
    /// The `gpt-4o-audio-preview` model can also be used to
    /// [generate audio](https://platform.openai.com/docs/guides/audio). To request that
    /// this model generate both text and audio responses, you can use:
    ///
    /// `["text", "audio"]`
    #[serde(skip_serializing_if = "Option::is_none")]
    pub modalities: Option<Vec<Modality>>,

    /// Number between -2.0 and 2.0. Positive values penalize new tokens based
    /// on whether they appear in the text so far, increasing the model's
    /// likelihood to talk about new topics.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub presence_penalty: Option<f64>,

    /// An object specifying the format that the model must output.
    ///
    /// Setting to `{ "type": "json_schema", "json_schema": {...} }` enables
    /// Structured Outputs which ensures the model will match your supplied
    /// JSON schema. Learn more in the
    /// [Structured Outputs guide](https://platform.openai.com/docs/guides/structured-outputs).
    ///
    /// Setting to `{ "type": "json_object" }` enables the older JSON mode,
    /// which ensures the message the model generates is valid JSON. Using
    /// `json_schema` is preferred for models that support it.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub response_format: Option<ResponseFormat>,

    /// This feature is in Beta. If specified, our system will make a best
    /// effort to sample deterministically, such that repeated requests with
    /// the same `seed` and parameters should return the same result.
    /// Determinism is not guaranteed, and you should refer to the
    /// `system_fingerprint` response parameter to monitor changes
    /// in the backend.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub seed: Option<i64>,

    /// If set to true, the model response data will be streamed to the client
    /// as it is generated using
    /// [server-sent events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#Event_stream_format).
    /// See the
    /// [Streaming section below](https://platform.openai.com/docs/api-reference/chat/streaming)
    /// for more information, along with the
    /// [streaming responses](https://platform.openai.com/docs/guides/streaming-responses)
    /// guide for more information on how to handle the streaming events.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub stream: Option<bool>,

    /// Options for streaming response. Only set this when you set `stream:
    /// true`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub stream_options: Option<ChatCompletionStreamOptions>,

    /// What sampling temperature to use, between 0 and 2. Higher values like
    /// 0.8 will make the output more random, while lower values like 0.2
    /// will make it more focused and deterministic. We generally recommend
    /// altering this or `top_p` but not both.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub temperature: Option<f64>,

    /// Controls which (if any) tool is called by the model. `none` means the
    /// model will not call any tool and instead generates a message. `auto`
    /// means the model can pick between generating a message or calling one
    /// or more tools. `required` means the model must call one or more
    /// tools. Specifying a particular tool via `{"type": "function",
    /// "function": {"name": "my_function"}}` forces the model to
    /// call that tool.
    ///
    /// `none` is the default when no tools are present. `auto` is the default
    /// if tools are present.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_choice: Option<ChatCompletionToolChoiceOption>,

    /// A list of tools the model may call. You can provide either
    /// [custom tools](https://platform.openai.com/docs/guides/function-calling#custom-tools)
    /// or [function tools](https://platform.openai.com/docs/guides/function-calling).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tools: Option<Vec<ChatCompletionTool>>,

    /// An alternative to sampling with temperature, called nucleus sampling,
    /// where the model considers the results of the tokens with `top_p`
    /// probability mass. So 0.1 means only the tokens comprising the top
    /// 10% probability mass are considered.
    ///
    /// We generally recommend altering this or `temperature` but not both.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub top_p: Option<f64>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Modality {
    Text,
    Audio,
}

/// Developer-provided instructions that the model should follow, regardless of
/// messages sent by the user. With o1 models and newer, `developer` messages
/// replace the previous `system` messages.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "role", rename_all = "snake_case")]
pub enum ChatCompletionMessageParam {
    Developer(ChatCompletionDeveloperMessageParam),
    System(ChatCompletionSystemMessageParam),
    User(ChatCompletionUserMessageParam),
    Assistant(ChatCompletionAssistantMessageParam),
    Tool(ChatCompletionToolMessageParam),
}

/// Developer-provided instructions that the model should follow, regardless of
/// messages sent by the user. With o1 models and newer, `developer` messages
/// replace the previous `system` messages.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionDeveloperMessageParam {
    /// The contents of the developer message.
    pub content: Vec<ChatCompletionContentPartText>,

    /// An optional name for the participant. Provides the model information to
    /// differentiate between participants of the same role.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
}

/// Developer-provided instructions that the model should follow, regardless of
/// messages sent by the user. With o1 models and newer, use `developer`
/// messages for this purpose instead.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionSystemMessageParam {
    /// The contents of the system message.
    pub content: Vec<ChatCompletionContentPartText>,
}

/// Messages sent by an end user, containing prompts or additional context
/// information.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ChatCompletionUserMessageParam {
    /// The contents of the user message.
    pub content: Vec<ChatCompletionContentPart>,

    /// An optional name for the participant. Provides the model information to
    /// differentiate between participants of the same role.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
}

/// Messages sent by the model in response to user messages.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ChatCompletionAssistantMessageParam {
    /// Data about a previous audio response from the model.
    /// [Learn more](https://platform.openai.com/docs/guides/audio).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audio: Option<ChatCompletionAssistantMessageParamAudio>,

    /// The contents of the assistant message. Required unless `tool_calls` or
    /// `function_call` is specified.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub content: Option<Vec<AssistantContentPart>>,

    /// An optional name for the participant. Provides the model information to
    /// differentiate between participants of the same role.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,

    /// The refusal message by the assistant.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub refusal: Option<String>,

    /// The tool calls generated by the model, such as function calls.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_calls: Option<Vec<ChatCompletionMessageToolCall>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum AssistantContentPart {
    Text(ChatCompletionContentPartText),
    Refusal(ChatCompletionContentPartRefusal),
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionToolMessageParam {
    /// The contents of the tool message.
    pub content: Vec<ChatCompletionContentPartText>,

    /// Tool call that this message is responding to.
    pub tool_call_id: String,
}

/// Learn about
/// [text inputs](https://platform.openai.com/docs/guides/text-generation).
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum ChatCompletionContentPart {
    Text(ChatCompletionContentPartText),
    #[serde(rename = "image_url")]
    Image(ChatCompletionContentPartImage),
    InputAudio(ChatCompletionContentPartInputAudio),
    File(ChatCompletionContentPartFile),
}

/// Learn about
/// [text inputs](https://platform.openai.com/docs/guides/text-generation).
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ChatCompletionContentPartText {
    /// The text content.
    pub text: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionContentPartImageImageURL {
    /// Either a URL of the image or the base64 encoded image data.
    pub url: String,

    /// Specifies the detail level of the image. Learn more in the
    /// [Vision guide](https://platform.openai.com/docs/guides/vision#low-or-high-fidelity-image-understanding).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub detail: Option<ImageDetail>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ImageDetail {
    Auto,
    Low,
    High,
}

/// Learn about [image inputs](https://platform.openai.com/docs/guides/vision).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionContentPartImage {
    pub image_url: ChatCompletionContentPartImageImageURL,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionContentPartInputAudioInputAudio {
    /// Base64 encoded audio data.
    pub data: String,

    /// The format of the encoded audio data. Currently supports "wav" and
    /// "mp3".
    pub format: AudioInputFormat,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum AudioInputFormat {
    Wav,
    Mp3,
}

/// Learn about [audio inputs](https://platform.openai.com/docs/guides/audio).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionContentPartInputAudio {
    pub input_audio: ChatCompletionContentPartInputAudioInputAudio,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionContentPartFileFile {
    /// The base64 encoded file data, used when passing the file to the model as
    /// a string.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub file_data: Option<String>,

    /// The ID of an uploaded file to use as input.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub file_id: Option<String>,

    /// The name of the file, used when passing the file to the model as a
    /// string.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub filename: Option<String>,
}

/// Learn about [file inputs](https://platform.openai.com/docs/guides/text) for text
/// generation.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionContentPartFile {
    pub file: ChatCompletionContentPartFileFile,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionContentPartRefusal {
    /// The refusal message generated by the model.
    pub refusal: String,
}

/// Parameters for audio output. Required when audio output is requested with
/// `modalities: ["audio"]`.
/// [Learn more](https://platform.openai.com/docs/guides/audio).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionAudioParam {
    /// Specifies the output audio format. Must be one of `wav`, `mp3`, `flac`,
    /// `opus`, or `pcm16`.
    pub format: AudioOutputFormat,

    /// The voice the model uses to respond. Supported voices are `alloy`,
    /// `ash`, `ballad`, `coral`, `echo`, `fable`, `nova`, `onyx`, `sage`,
    /// and `shimmer`.
    pub voice: Voice,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum AudioOutputFormat {
    Wav,
    Aac,
    Mp3,
    Flac,
    Opus,
    Pcm16,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
pub enum Voice {
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
    /// Custom voice
    Custom(String),
}

/// Controls which (if any) tool is called by the model. `none` means the model
/// will not call any tool and instead generates a message. `auto` means the
/// model can pick between generating a message or calling one or more tools.
/// `required` means the model must call one or more tools. Specifying a
/// particular tool via `{"type": "function", "function": {"name":
/// "my_function"}}` forces the model to call that tool.
///
/// `none` is the default when no tools are present. `auto` is the default if
/// tools are present.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
pub enum ChatCompletionToolChoiceOption {
    None,
    Auto,
    Required,
    Allowed(ChatCompletionAllowedToolChoice),
    Named(ChatCompletionNamedToolChoice),
}

/// Constrains the tools available to the model to a pre-defined set.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionAllowedToolChoice {
    /// Constrains the tools available to the model to a pre-defined set.
    pub allowed_tools: ChatCompletionAllowedTools,

    /// Allowed tool configuration type. Always `allowed_tools`.
    #[serde(rename = "type")]
    pub type_: String, // Always "allowed_tools"
}

/// Constrains the tools available to the model to a pre-defined set.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionAllowedTools {
    /// Constrains the tools available to the model to a pre-defined set.
    ///
    /// `auto` allows the model to pick from among the allowed tools and
    /// generate a message.
    ///
    /// `required` requires the model to call one or more of the allowed tools.
    pub mode: AllowedToolsMode,

    /// A list of tool definitions that the model should be allowed to call.
    ///
    /// For the Chat Completions API, the list of tool definitions might look
    /// like:
    ///
    /// ```json
    /// [
    ///   { "type": "function", "function": { "name": "get_weather" } },
    ///   { "type": "function", "function": { "name": "get_time" } }
    /// ]
    /// ```
    pub tools: Vec<HashMap<String, Value>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum AllowedToolsMode {
    Auto,
    Required,
}

/// Specifies a tool the model should use. Use to force the model to call a
/// specific function.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionNamedToolChoice {
    pub function: ChatCompletionNamedToolChoiceFunction,

    /// For function calling, the type is always `function`.
    #[serde(rename = "type")]
    pub type_: String, // Always "function"
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionNamedToolChoiceFunction {
    /// The name of the function to call.
    pub name: String,
}

/// A call to a function tool created by the model.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum ChatCompletionMessageToolCall {
    Function(ChatCompletionMessageFunctionToolCall),
}

/// A call to a function tool created by the model.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionMessageFunctionToolCall {
    /// The ID of the tool call.
    pub id: String,

    /// The function that the model called.
    pub function: ChatCompletionMessageFunctionToolCallFunction,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionMessageFunctionToolCallFunction {
    /// The arguments to call the function with, as generated by the model in
    /// JSON format. Note that the model does not always generate valid
    /// JSON, and may hallucinate parameters not defined by your function
    /// schema. Validate the arguments in your code before calling your
    /// function.
    pub arguments: String,

    /// The name of the function to call.
    pub name: String,
}

/// A function tool that can be used to generate a response.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum ChatCompletionTool {
    Function(ChatCompletionFunctionTool),
}

/// A function tool that can be used to generate a response.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionFunctionTool {
    pub function: FunctionDefinition,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FunctionDefinition {
    /// The name of the function to be called. Must be a-z, A-Z, 0-9, or contain
    /// underscores and dashes, with a maximum length of 64.
    pub name: String,

    /// A description of what the function does, used by the model to choose
    /// when and how to call the function.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,

    /// The parameters the functions accepts, described as a JSON Schema object.
    /// See the [guide](https://platform.openai.com/docs/guides/function-calling) for examples,
    /// and the
    /// [JSON Schema reference](https://json-schema.org/understanding-json-schema/) for
    /// documentation about the format.
    ///
    /// Omitting `parameters` defines a function with an empty parameter list.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub parameters: Option<FunctionParameters>,

    /// Whether to enable strict schema adherence when generating the function
    /// call. If set to true, the model will follow the exact schema defined
    /// in the `parameters` field. Only a subset of JSON Schema is supported
    /// when `strict` is `true`. Learn more about Structured Outputs in the
    /// [function calling guide](https://platform.openai.com/docs/guides/function-calling).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub strict: Option<bool>,
}

/// The parameters the functions accepts, described as a JSON Schema object. See
/// the [guide](https://platform.openai.com/docs/guides/function-calling) for examples,
/// and the
/// [JSON Schema reference](https://json-schema.org/understanding-json-schema/) for
/// documentation about the format.
///
/// Omitting `parameters` defines a function with an empty parameter list.
pub type FunctionParameters = JSONSchema;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionAssistantMessageParamAudio {
    /// Unique identifier for a previous audio response from the model.
    pub id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum ResponseFormat {
    /// Default response format. Used to generate text responses.
    Text,
    /// JSON Schema response format. Used to generate structured JSON responses.
    /// Learn more about
    /// [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs).
    JsonSchema(ResponseFormatJSONSchema),
    /// JSON object response format. An older method of generating JSON
    /// responses. Using `json_schema` is recommended for models that
    /// support it. Note that the model will not generate JSON without a
    /// system or user message instructing it to do so.
    JsonObject,
}

/// JSON Schema response format. Used to generate structured JSON responses.
/// Learn more about
/// [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseFormatJSONSchema {
    /// Structured Outputs configuration options, including a JSON Schema.
    pub json_schema: ResponseFormatJSONSchemaJSONSchema,
}

/// Structured Outputs configuration options, including a JSON Schema.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseFormatJSONSchemaJSONSchema {
    /// The name of the response format. Must be a-z, A-Z, 0-9, or contain
    /// underscores and dashes, with a maximum length of 64.
    pub name: String,

    /// A description of what the response format is for, used by the model to
    /// determine how to respond in the format.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,

    /// The schema for the response format, described as a JSON Schema object.
    /// Learn how to build JSON schemas [here](https://json-schema.org/).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub schema: Option<JSONSchema>,

    /// Whether to enable strict schema adherence when generating the output. If
    /// set to true, the model will always follow the exact schema defined
    /// in the `schema` field. Only a subset of JSON Schema is supported
    /// when `strict` is `true`. To learn more, read the
    /// [Structured Outputs guide](https://platform.openai.com/docs/guides/structured-outputs).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub strict: Option<bool>,
}

/// Options for streaming response. Only set this when you set `stream: true`.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionStreamOptions {
    /// When true, stream obfuscation will be enabled. Stream obfuscation adds
    /// random characters to an `obfuscation` field on streaming delta
    /// events to normalize payload sizes as a mitigation to certain
    /// side-channel attacks. These obfuscation fields are included by
    /// default, but add a small amount of overhead to the data stream. You
    /// can set `include_obfuscation` to false to optimize for bandwidth if
    /// you trust the network links between your application and the `OpenAI`
    /// API.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub include_obfuscation: Option<bool>,

    /// If set, an additional chunk will be streamed before the `data: [DONE]`
    /// message. The `usage` field on this chunk shows the token usage
    /// statistics for the entire request, and the `choices` field will
    /// always be an empty array.
    ///
    /// All other chunks will also include a `usage` field, but with a null
    /// value. **NOTE:** If the stream is interrupted, you may not receive
    /// the final usage chunk which contains the total token usage for the
    /// request.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub include_usage: Option<bool>,
}

/// Represents a chat completion response returned by model, based on the
/// provided input.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletion {
    /// A unique identifier for the chat completion.
    pub id: String,

    /// A list of chat completion choices. Can be more than one if `n` is
    /// greater than 1.
    pub choices: Vec<ChatCompletionChoice>,

    /// The Unix timestamp (in seconds) of when the chat completion was created.
    pub created: i64,

    /// The model used for the chat completion.
    pub model: String,

    /// The object type, which is always `chat.completion`.
    pub object: String, // Always "chat.completion"

    /// Usage statistics for the completion request.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub usage: Option<CompletionsAPICompletionUsage>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionChoice {
    /// The reason the model stopped generating tokens. This will be `stop` if
    /// the model hit a natural stop point or a provided stop sequence,
    /// `length` if the maximum number of tokens specified in the request
    /// was reached, `content_filter` if content was omitted due to a flag
    /// from our content filters, `tool_calls` if the model called a tool,
    /// or `function_call` (deprecated) if the model called a function.
    pub finish_reason: FinishReason,

    /// The index of the choice in the list of choices.
    pub index: i32,

    /// A chat completion message generated by the model.
    pub message: CompletionsCompletionsAPIChatCompletionMessage,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum FinishReason {
    Stop,
    Length,
    ToolCalls,
    ContentFilter,
    FunctionCall,
}

/// Usage statistics for the completion request.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CompletionsAPICompletionUsage {
    /// Number of tokens in the generated completion.
    pub completion_tokens: u32,

    /// Number of tokens in the prompt.
    pub prompt_tokens: u32,

    /// Total number of tokens used in the request (prompt + completion).
    pub total_tokens: u32,

    /// Breakdown of tokens used in a completion.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub completion_tokens_details: Option<CompletionUsageCompletionTokensDetails>,

    /// Breakdown of tokens used in the prompt.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub prompt_tokens_details: Option<CompletionUsagePromptTokensDetails>,
}

/// Breakdown of tokens used in a completion.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CompletionUsageCompletionTokensDetails {
    /// When using Predicted Outputs, the number of tokens in the prediction
    /// that appeared in the completion.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub accepted_prediction_tokens: Option<i32>,

    /// Audio input tokens generated by the model.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audio_tokens: Option<i32>,

    /// Tokens generated by the model for reasoning.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reasoning_tokens: Option<i32>,

    /// When using Predicted Outputs, the number of tokens in the prediction
    /// that did not appear in the completion. However, like reasoning
    /// tokens, these tokens are still counted in the total completion
    /// tokens for purposes of billing, output, and context window limits.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub rejected_prediction_tokens: Option<i32>,
}

/// Breakdown of tokens used in the prompt.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CompletionUsagePromptTokensDetails {
    /// Audio input tokens present in the prompt.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audio_tokens: Option<i32>,

    /// Cached tokens present in the prompt.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cached_tokens: Option<i32>,
}

/// A chat completion message generated by the model.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CompletionsCompletionsAPIChatCompletionMessage {
    /// The contents of the message.
    pub content: Option<String>,

    /// The refusal message generated by the model.
    pub refusal: Option<String>,

    /// The role of the author of this message.
    pub role: String, // Always "assistant"

    /// If the audio output modality is requested, this object contains data
    /// about the audio response from the model.
    /// [Learn more](https://platform.openai.com/docs/guides/audio).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audio: Option<ChatCompletionAudio>,

    /// The tool calls generated by the model, such as function calls.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_calls: Option<Vec<ChatCompletionMessageToolCall>>,
}

/// If the audio output modality is requested, this object contains data about
/// the audio response from the model.
/// [Learn more](https://platform.openai.com/docs/guides/audio).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionAudio {
    /// Unique identifier for this audio response.
    pub id: String,

    /// Base64 encoded audio bytes generated by the model, in the format
    /// specified in the request.
    pub data: String,

    /// The Unix timestamp (in seconds) for when this audio response will no
    /// longer be accessible on the server for use in multi-turn
    /// conversations.
    pub expires_at: i64,

    /// Transcript of the audio generated by the model.
    pub transcript: String,
}

/// Represents a streamed chunk of a chat completion response returned by the
/// model, based on the provided input.
/// [Learn more](https://platform.openai.com/docs/guides/streaming-responses).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionChunk {
    /// A unique identifier for the chat completion. Each chunk has the same ID.
    pub id: String,

    /// A list of chat completion choices. Can contain more than one elements if
    /// `n` is greater than 1. Can also be empty for the last chunk if you
    /// set `stream_options: {"include_usage": true}`.
    pub choices: Vec<ChatCompletionChunkChoice>,

    /// The Unix timestamp (in seconds) of when the chat completion was created.
    /// Each chunk has the same timestamp.
    pub created: i64,

    /// The model to generate the completion.
    pub model: String,

    /// The object type, which is always `chat.completion.chunk`.
    pub object: String, // Always "chat.completion.chunk"

    /// An optional field that will only be present when you set
    /// `stream_options: {"include_usage": true}` in your request. When present,
    /// it contains a null value **except for the last chunk** which
    /// contains the token usage statistics for the entire request.
    ///
    /// **NOTE:** If the stream is interrupted or cancelled, you may not receive
    /// the final usage chunk which contains the total token usage for the
    /// request.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub usage: Option<CompletionsAPICompletionUsage>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionChunkChoice {
    /// A chat completion delta generated by streamed model responses.
    pub delta: ChatCompletionChunkChoiceDelta,

    /// The reason the model stopped generating tokens. This will be `stop` if
    /// the model hit a natural stop point or a provided stop sequence,
    /// `length` if the maximum number of tokens specified in the request
    /// was reached, `content_filter` if content was omitted due to a flag
    /// from our content filters, `tool_calls` if the model called a tool,
    /// or `function_call` (deprecated) if the model called a function.
    pub finish_reason: Option<FinishReason>,

    /// The index of the choice in the list of choices.
    pub index: i32,
}

/// A chat completion delta generated by streamed model responses.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionChunkChoiceDelta {
    /// The contents of the chunk message.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub content: Option<String>,

    /// The refusal message generated by the model.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub refusal: Option<String>,

    /// The role of the author of this message.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub role: Option<DeltaRole>,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_calls: Option<Vec<ChatCompletionChunkChoiceDeltaToolCall>>,

    // @undocumented
    pub audio: Option<ChatCompletionChunkChoiceDeltaAudio>,
}

// @undocumented
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionChunkChoiceDeltaAudio {
    pub id: Option<String>,
    pub data: Option<String>,
    pub transcript: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum DeltaRole {
    Developer,
    System,
    User,
    Assistant,
    Tool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionChunkChoiceDeltaToolCall {
    pub index: usize,

    /// The ID of the tool call.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub function: Option<ChatCompletionChunkChoiceDeltaToolCallFunction>,

    /// The type of the tool. Currently, only `function` is supported.
    #[serde(rename = "type", skip_serializing_if = "Option::is_none")]
    pub type_: Option<String>, // Always "function" when present
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionChunkChoiceDeltaToolCallFunction {
    /// The arguments to call the function with, as generated by the model in
    /// JSON format. Note that the model does not always generate valid
    /// JSON, and may hallucinate parameters not defined by your function
    /// schema. Validate the arguments in your code before calling your
    /// function.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub arguments: Option<String>,

    /// The name of the function to call.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
}
