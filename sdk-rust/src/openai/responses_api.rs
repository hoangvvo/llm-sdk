#![allow(clippy::pedantic)]
use crate::LanguageModelInputExtra;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::HashMap;

// https://platform.openai.com/docs/api-reference/responses/create

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ResponseCreateParams {
    /// Specify additional output data to include in the model response.
    /// Currently supported values are:
    ///
    /// - `web_search_call.action.sources`: Include the sources of the web
    ///   search tool call.
    /// - `code_interpreter_call.outputs`: Includes the outputs of python code
    ///   execution in code interpreter tool call items.
    /// - `computer_call_output.output.image_url`: Include image urls from the
    ///   computer call output.
    /// - `file_search_call.results`: Include the search results of the file
    ///   search tool call.
    /// - `message.input_image.image_url`: Include image urls from the input
    ///   message.
    /// - `computer_call_output.output.image_url`: Include image urls from the
    ///   computer call output.
    /// - `reasoning.encrypted_content`: Includes an encrypted version of
    ///   reasoning tokens in reasoning item outputs. This enables reasoning
    ///   items to be used in multi-turn conversations when using the Responses
    ///   API statelessly (like when the `store` parameter is set to `false`, or
    ///   when an organization is enrolled in the zero data retention program).
    /// - `code_interpreter_call.outputs`: Includes the outputs of python code
    ///   execution in code interpreter tool call items.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub include: Option<Vec<ResponseIncludable>>,

    /// Text, image, or file inputs to the model, used to generate a response.
    ///
    /// Learn more:
    ///
    /// - [Text inputs and outputs](https://platform.openai.com/docs/guides/text)
    /// - [Image inputs](https://platform.openai.com/docs/guides/images)
    /// - [File inputs](https://platform.openai.com/docs/guides/pdf-files)
    /// - [Conversation state](https://platform.openai.com/docs/guides/conversation-state)
    /// - [Function calling](https://platform.openai.com/docs/guides/function-calling)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input: Option<Vec<ResponseInputItem>>,

    /// A system (or developer) message inserted into the model's context.
    ///
    /// When using along with `previous_response_id`, the instructions from a
    /// previous response will not be carried over to the next response.
    /// This makes it simple to swap out system (or developer) messages in
    /// new responses.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub instructions: Option<String>,

    /// An upper bound for the number of tokens that can be generated for a
    /// response, including visible output tokens and
    /// [reasoning tokens](https://platform.openai.com/docs/guides/reasoning).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_output_tokens: Option<u32>,

    /// Model ID used to generate the response, like `gpt-4o` or `o3`. `OpenAI`
    /// offers a wide range of models with different capabilities,
    /// performance characteristics, and price points. Refer to the
    /// [model guide](https://platform.openai.com/docs/models) to browse and compare
    /// available models.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,

    /// Whether to allow the model to run tool calls in parallel.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub parallel_tool_calls: Option<bool>,

    /// **gpt-5 and o-series models only**
    ///
    /// Configuration options for
    /// [reasoning models](https://platform.openai.com/docs/guides/reasoning).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reasoning: Option<Reasoning>,

    /// Whether to store the generated model response for later retrieval via
    /// API.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub store: Option<bool>,

    /// If set to true, the model response data will be streamed to the client
    /// as it is generated using
    /// [server-sent events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#Event_stream_format).
    /// See the
    /// [Streaming section below](https://platform.openai.com/docs/api-reference/responses-streaming)
    /// for more information.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub stream: Option<bool>,

    /// Options for streaming responses. Only set this when you set `stream:
    /// true`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub stream_options: Option<ResponseCreateParamsStreamOptions>,

    /// What sampling temperature to use, between 0 and 2. Higher values like
    /// 0.8 will make the output more random, while lower values like 0.2
    /// will make it more focused and deterministic. We generally recommend
    /// altering this or `top_p` but not both.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub temperature: Option<f64>,

    /// Configuration options for a text response from the model. Can be plain
    /// text or structured JSON data. Learn more:
    ///
    /// - [Text inputs and outputs](https://platform.openai.com/docs/guides/text)
    /// - [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub text: Option<ResponseTextConfig>,

    /// How the model should select which tool (or tools) to use when generating
    /// a response. See the `tools` parameter to see how to specify which
    /// tools the model can call.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_choice: Option<serde_json::Value>,

    /// An array of tools the model may call while generating a response. You
    /// can specify which tool to use by setting the `tool_choice`
    /// parameter.
    ///
    /// We support the following categories of tools:
    ///
    /// - **Built-in tools**: Tools that are provided by `OpenAI` that extend the model's
    ///   capabilities, like
    ///   [web search](https://platform.openai.com/docs/guides/tools-web-search) or
    ///   [file search](https://platform.openai.com/docs/guides/tools-file-search).
    ///   Learn more about
    ///   [built-in tools](https://platform.openai.com/docs/guides/tools).
    /// - **MCP Tools**: Integrations with third-party systems via custom MCP servers or
    ///   predefined connectors such as Google Drive and `SharePoint`. Learn more about
    ///   [MCP Tools](https://platform.openai.com/docs/guides/tools-connectors-mcp).
    /// - **Function calls (custom tools)**: Functions that are defined by you, enabling
    ///   the model to call your own code with strongly typed arguments and outputs.
    ///   Learn more about
    ///   [function calling](https://platform.openai.com/docs/guides/function-calling).
    ///   You can also use custom tools to call your own code.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tools: Option<Vec<Tool>>,

    /// An alternative to sampling with temperature, called nucleus sampling,
    /// where the model considers the results of the tokens with `top_p`
    /// probability mass. So 0.1 means only the tokens comprising the top
    /// 10% probability mass are considered.
    ///
    /// We generally recommend altering this or `temperature` but not both.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub top_p: Option<f64>,

    /// The truncation strategy to use for the model response.
    ///
    /// - `auto`: If the context of this response and previous ones exceeds the
    ///   model's context window size, the model will truncate the response to
    ///   fit the context window by dropping input items in the middle of the
    ///   conversation.
    /// - `disabled` (default): If a model response will exceed the context
    ///   window size for a model, the request will fail with a 400 error.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncation: Option<String>,

    #[serde(skip_serializing_if = "Option::is_none", flatten)]
    pub extra: Option<LanguageModelInputExtra>,
}

/// Specify additional output data to include in the model response. Currently
/// supported values are:
///
/// - `web_search_call.action.sources`: Include the sources of the web search
///   tool call.
/// - `code_interpreter_call.outputs`: Includes the outputs of python code
///   execution in code interpreter tool call items.
/// - `computer_call_output.output.image_url`: Include image urls from the
///   computer call output.
/// - `file_search_call.results`: Include the search results of the file search
///   tool call.
/// - `message.input_image.image_url`: Include image urls from the input
///   message.
/// - `computer_call_output.output.image_url`: Include image urls from the
///   computer call output.
/// - `reasoning.encrypted_content`: Includes an encrypted version of reasoning
///   tokens in reasoning item outputs. This enables reasoning items to be used
///   in multi-turn conversations when using the Responses API statelessly (like
///   when the `store` parameter is set to `false`, or when an organization is
///   enrolled in the zero data retention program).
/// - `code_interpreter_call.outputs`: Includes the outputs of python code
///   execution in code interpreter tool call items.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum ResponseIncludable {
    #[serde(rename = "file_search_call.results")]
    FileSearchCallResults,
    #[serde(rename = "message.input_image.image_url")]
    MessageInputImageUrl,
    #[serde(rename = "computer_call_output.output.image_url")]
    ComputerCallOutputImageUrl,
    #[serde(rename = "reasoning.encrypted_content")]
    ReasoningEncryptedContent,
    #[serde(rename = "code_interpreter_call.outputs")]
    CodeInterpreterCallOutputs,
}

/// A message input to the model with a role indicating instruction following
/// hierarchy. Instructions given with the `developer` or `system` role take
/// precedence over instructions given with the `user` role. Messages with the
/// `assistant` role are presumed to have been generated by the model in
/// previous interactions.
#[derive(Debug, Clone)]
pub enum ResponseInputItem {
    Message(ResponseInputItemMessage),
    OutputMessage(ResponseOutputMessage),
    FunctionCall(ResponseFunctionToolCall),
    FunctionCallOutput(ResponseInputItemFunctionCallOutput),
    Reasoning(ResponseReasoningItem),
    ImageGenerationCall(ResponseOutputItemImageGenerationCall),
}

// Unfortunately, custom serialization is required here because both
// Message and OutputMessage share type="message" but are differentiated by role
impl Serialize for ResponseInputItem {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeMap;

        match self {
            Self::Message(msg) => {
                let mut map = serializer.serialize_map(None)?;
                map.serialize_entry("type", "message")?;
                map.serialize_entry("content", &msg.content)?;
                map.serialize_entry("role", &msg.role)?;
                if let Some(status) = &msg.status {
                    map.serialize_entry("status", status)?;
                }
                map.end()
            }
            Self::OutputMessage(msg) => {
                let mut map = serializer.serialize_map(None)?;
                map.serialize_entry("type", "message")?;
                map.serialize_entry("id", &msg.id)?;
                map.serialize_entry("content", &msg.content)?;
                map.serialize_entry("role", &msg.role)?;
                map.serialize_entry("status", &msg.status)?;
                map.end()
            }
            Self::FunctionCall(fc) => {
                #[derive(Serialize)]
                struct Helper<'a> {
                    #[serde(rename = "type")]
                    type_: &'static str,
                    #[serde(flatten)]
                    inner: &'a ResponseFunctionToolCall,
                }
                Helper {
                    type_: "function_call",
                    inner: fc,
                }
                .serialize(serializer)
            }
            Self::FunctionCallOutput(fco) => {
                #[derive(Serialize)]
                struct Helper<'a> {
                    #[serde(rename = "type")]
                    type_: &'static str,
                    #[serde(flatten)]
                    inner: &'a ResponseInputItemFunctionCallOutput,
                }
                Helper {
                    type_: "function_call_output",
                    inner: fco,
                }
                .serialize(serializer)
            }
            Self::Reasoning(r) => {
                #[derive(Serialize)]
                struct Helper<'a> {
                    #[serde(rename = "type")]
                    type_: &'static str,
                    #[serde(flatten)]
                    inner: &'a ResponseReasoningItem,
                }
                Helper {
                    type_: "reasoning",
                    inner: r,
                }
                .serialize(serializer)
            }
            Self::ImageGenerationCall(igc) => {
                #[derive(Serialize)]
                struct Helper<'a> {
                    #[serde(rename = "type")]
                    type_: &'static str,
                    #[serde(flatten)]
                    inner: &'a ResponseOutputItemImageGenerationCall,
                }
                Helper {
                    type_: "image_generation_call",
                    inner: igc,
                }
                .serialize(serializer)
            }
        }
    }
}

impl<'de> Deserialize<'de> for ResponseInputItem {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        use serde_json::Value;

        let value = Value::deserialize(deserializer)?;
        let type_str = value
            .get("type")
            .and_then(|v| v.as_str())
            .ok_or_else(|| serde::de::Error::missing_field("type"))?;

        match type_str {
            "message" => {
                // Check role to differentiate
                if let Some(role) = value.get("role").and_then(|v| v.as_str()) {
                    if role == "assistant" && value.get("id").is_some() {
                        serde_json::from_value(value)
                            .map(ResponseInputItem::OutputMessage)
                            .map_err(serde::de::Error::custom)
                    } else {
                        serde_json::from_value(value)
                            .map(ResponseInputItem::Message)
                            .map_err(serde::de::Error::custom)
                    }
                } else {
                    Err(serde::de::Error::missing_field("role"))
                }
            }
            "function_call" => serde_json::from_value(value)
                .map(ResponseInputItem::FunctionCall)
                .map_err(serde::de::Error::custom),
            "function_call_output" => serde_json::from_value(value)
                .map(ResponseInputItem::FunctionCallOutput)
                .map_err(serde::de::Error::custom),
            "reasoning" => serde_json::from_value(value)
                .map(ResponseInputItem::Reasoning)
                .map_err(serde::de::Error::custom),
            "image_generation_call" => serde_json::from_value(value)
                .map(ResponseInputItem::ImageGenerationCall)
                .map_err(serde::de::Error::custom),
            _ => Err(serde::de::Error::unknown_variant(
                type_str,
                &[
                    "message",
                    "function_call",
                    "function_call_output",
                    "reasoning",
                    "image_generation_call",
                ],
            )),
        }
    }
}

/// A message input to the model with a role indicating instruction following
/// hierarchy. Instructions given with the `developer` or `system` role take
/// precedence over instructions given with the `user` role.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseInputItemMessage {
    /// A list of one or many input items to the model, containing different
    /// content types.
    pub content: Vec<ResponseInputContent>,

    /// The role of the message input. One of `user`, `system`, or `developer`.
    pub role: String,

    /// The status of item. One of `in_progress`, `completed`, or `incomplete`.
    /// Populated when items are returned via API.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub status: Option<String>,
}

/// A text input to the model.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
#[allow(clippy::enum_variant_names)]
pub enum ResponseInputContent {
    InputText(ResponseInputText),
    InputImage(ResponseInputImage),
    InputFile(ResponseInputFile),
    InputAudio(ResponseInputAudio),
}

/// A text input to the model.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseInputText {
    /// The text input to the model.
    pub text: String,
}

/// An image input to the model. Learn about
/// [image inputs](https://platform.openai.com/docs/guides/vision).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseInputImage {
    /// The detail level of the image to be sent to the model. One of `high`,
    /// `low`, or `auto`. Defaults to `auto`.
    pub detail: String,

    /// The ID of the file to be sent to the model.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub file_id: Option<String>,

    /// The URL of the image to be sent to the model. A fully qualified URL or
    /// base64 encoded image in a data URL.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub image_url: Option<String>,
}

/// A file input to the model.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseInputFile {
    /// The content of the file to be sent to the model.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub file_data: Option<String>,

    /// The ID of the file to be sent to the model.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub file_id: Option<String>,

    /// The URL of the file to be sent to the model.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub file_url: Option<String>,

    /// The name of the file to be sent to the model.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub filename: Option<String>,
}

/// An audio input to the model.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseInputAudio {
    pub input_audio: ResponseInputAudioInputAudio,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseInputAudioInputAudio {
    /// Base64-encoded audio data.
    pub data: String,

    /// The format of the audio data. Currently supported formats are `mp3` and
    /// `wav`.
    pub format: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseOutputMessage {
    /// The unique ID of the output message.
    pub id: String,

    /// The content of the output message.
    pub content: Vec<ResponseOutputContent>,

    /// The role of the output message. Always `assistant`.
    pub role: String,

    /// The status of the message input. One of `in_progress`, `completed`, or
    /// `incomplete`. Populated when input items are returned via API.
    pub status: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum ResponseOutputContent {
    OutputText(ResponseOutputText),
    Refusal(ResponseOutputRefusal),
}

/// A text output from the model.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseOutputText {
    /// The annotations of the text output.
    pub annotations: Vec<ResponseOutputTextAnnotation>,

    /// The text output from the model.
    pub text: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum ResponseOutputTextAnnotation {
    FileCitation(ResponseOutputTextFileCitation),
    UrlCitation(ResponseOutputTextURLCitation),
    FilePath(ResponseOutputTextFilePath),
}

/// A citation to a file.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseOutputTextFileCitation {
    /// The ID of the file.
    pub file_id: String,

    /// The filename of the file cited.
    pub filename: String,

    /// The index of the file in the list of files.
    pub index: u32,
}

/// A citation for a web resource used to generate a model response.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseOutputTextURLCitation {
    /// The index of the last character of the URL citation in the message.
    pub end_index: u32,

    /// The index of the first character of the URL citation in the message.
    pub start_index: u32,

    /// The title of the web resource.
    pub title: String,

    /// The URL of the web resource.
    pub url: String,
}

/// A path to a file.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseOutputTextFilePath {
    /// The ID of the file.
    pub file_id: String,

    /// The index of the file in the list of files.
    pub index: u32,
}

/// A refusal from the model.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseOutputRefusal {
    /// The refusal explanation from the model.
    pub refusal: String,
}

/// A tool call to run a function. See the
/// [function calling guide](https://platform.openai.com/docs/guides/function-calling)
/// for more information.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseFunctionToolCall {
    /// A JSON string of the arguments to pass to the function.
    pub arguments: String,

    /// The unique ID of the function tool call generated by the model.
    pub call_id: String,

    /// The name of the function to run.
    pub name: String,

    /// The unique ID of the function tool call.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,

    /// The status of the item. One of `in_progress`, `completed`, or
    /// `incomplete`. Populated when items are returned via API.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub status: Option<String>,
}

/// The output of a function tool call.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseInputItemFunctionCallOutput {
    /// The unique ID of the function tool call generated by the model.
    pub call_id: String,

    /// A JSON string of the output of the function tool call.
    pub output: String,

    /// The unique ID of the function tool call output. Populated when this item
    /// is returned via API.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,

    /// The status of the item. One of `in_progress`, `completed`, or
    /// `incomplete`. Populated when items are returned via API.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub status: Option<String>,
}

/// A description of the chain of thought used by a reasoning model while
/// generating a response. Be sure to include these items in your `input` to the
/// Responses API for subsequent turns of a conversation if you are manually
/// [managing context](https://platform.openai.com/docs/guides/conversation-state).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseReasoningItem {
    /// The unique identifier of the reasoning content.
    pub id: String,

    /// Reasoning summary content.
    pub summary: Vec<ResponseReasoningItemSummaryUnion>,

    /// Reasoning text content.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub content: Option<Vec<ResponseReasoningItemContentUnion>>,

    /// The encrypted content of the reasoning item - populated when a response
    /// is generated with `reasoning.encrypted_content` in the `include`
    /// parameter.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub encrypted_content: Option<String>,

    /// The status of the item. One of `in_progress`, `completed`, or
    /// `incomplete`. Populated when items are returned via API.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub status: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum ResponseReasoningItemSummaryUnion {
    SummaryText(ResponseReasoningItemSummary),
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseReasoningItemSummary {
    /// Summary text content.
    pub text: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum ResponseReasoningItemContentUnion {
    ReasoningText(ResponseReasoningItemContent),
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseReasoningItemContent {
    /// Reasoning text output from the model.
    pub text: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseOutputItemImageGenerationCall {
    /// The unique ID of the image generation call.
    pub id: String,

    /// The generated image encoded in base64.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub result: Option<String>,

    /// The status of the image generation call.
    pub status: String,

    pub output_format: String, // png, jpeg, etc.

    pub size: Option<String>, // {number}x{number}
}

/// **o-series models only**
///
/// Configuration options for
/// [reasoning models](https://platform.openai.com/docs/guides/reasoning).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Reasoning {
    /// Constrains effort on reasoning for
    /// [reasoning models](https://platform.openai.com/docs/guides/reasoning). Currently
    /// supported values are `minimal`, `low`, `medium`, and `high`. Reducing
    /// reasoning effort can result in faster responses and fewer tokens
    /// used on reasoning in a response.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub effort: Option<ReasoningEffort>,

    /// A summary of the reasoning performed by the model. This can be useful
    /// for debugging and understanding the model's reasoning process. One
    /// of `auto`, `concise`, or `detailed`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub summary: Option<String>,
}

/// Constrains effort on reasoning for
/// [reasoning models](https://platform.openai.com/docs/guides/reasoning). Currently
/// supported values are `minimal`, `low`, `medium`, and `high`. Reducing
/// reasoning effort can result in faster responses and fewer tokens used on
/// reasoning in a response.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ReasoningEffort {
    Minimal,
    Low,
    Medium,
    High,
}

/// Options for streaming responses. Only set this when you set `stream: true`.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseCreateParamsStreamOptions {
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
}

/// Configuration options for a text response from the model. Can be plain text
/// or structured JSON data. Learn more:
///
/// - [Text inputs and outputs](https://platform.openai.com/docs/guides/text)
/// - [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseTextConfig {
    /// An object specifying the format that the model must output.
    ///
    /// Configuring `{ "type": "json_schema" }` enables Structured Outputs,
    /// which ensures the model will match your supplied JSON schema. Learn
    /// more in the [Structured Outputs guide](https://platform.openai.com/docs/guides/structured-outputs).
    ///
    /// The default format is `{ "type": "text" }` with no additional options.
    ///
    /// **Not recommended for gpt-4o and newer models:**
    ///
    /// Setting to `{ "type": "json_object" }` enables the older JSON mode,
    /// which ensures the message the model generates is valid JSON. Using
    /// `json_schema` is preferred for models that support it.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub format: Option<ResponseFormatTextConfig>,

    /// Constrains the verbosity of the model's response. Lower values will
    /// result in more concise responses, while higher values will result in
    /// more verbose responses. Currently supported values are `low`,
    /// `medium`, and `high`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub verbosity: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum ResponseFormatTextConfig {
    Text(ResponseFormatText),
    JsonSchema(ResponseFormatTextJSONSchemaConfig),
    JsonObject(ResponseFormatJSONObject),
}

/// Default response format. Used to generate text responses.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseFormatText {}

/// JSON Schema response format. Used to generate structured JSON responses.
/// Learn more about
/// [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseFormatTextJSONSchemaConfig {
    /// The name of the response format. Must be a-z, A-Z, 0-9, or contain
    /// underscores and dashes, with a maximum length of 64.
    pub name: String,

    /// The schema for the response format, described as a JSON Schema object.
    /// Learn how to build JSON schemas [here](https://json-schema.org/).
    pub schema: Value,

    /// A description of what the response format is for, used by the model to
    /// determine how to respond in the format.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,

    /// Whether to enable strict schema adherence when generating the output. If
    /// set to true, the model will always follow the exact schema defined
    /// in the `schema` field. Only a subset of JSON Schema is supported
    /// when `strict` is `true`. To learn more, read the
    /// [Structured Outputs guide](https://platform.openai.com/docs/guides/structured-outputs).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub strict: Option<bool>,
}

/// JSON object response format. An older method of generating JSON responses.
/// Using `json_schema` is recommended for models that support it. Note that the
/// model will not generate JSON without a system or user message instructing it
/// to do so.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseFormatJSONObject {}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ToolChoiceOptions {
    None,
    Auto,
    Required,
}

/// Constrains the tools available to the model to a pre-defined set.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ToolChoiceAllowed {
    /// Constrains the tools available to the model to a pre-defined set.
    ///
    /// `auto` allows the model to pick from among the allowed tools and
    /// generate a message.
    ///
    /// `required` requires the model to call one or more of the allowed tools.
    pub mode: String,

    /// A list of tool definitions that the model should be allowed to call.
    ///
    /// For the Responses API, the list of tool definitions might look like:
    ///
    /// ```json
    /// [
    ///   { "type": "function", "name": "get_weather" },
    ///   { "type": "mcp", "server_label": "deepwiki" },
    ///   { "type": "image_generation" }
    /// ]
    /// ```
    pub tools: Vec<HashMap<String, serde_json::Value>>,
}

/// Indicates that the model should use a built-in tool to generate a response.
/// [Learn more about built-in tools](https://platform.openai.com/docs/guides/tools).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ToolChoiceTypes {
    /// The type of hosted tool the model should to use. Learn more about
    /// [built-in tools](https://platform.openai.com/docs/guides/tools).
    ///
    /// Allowed values are:
    ///
    /// - `file_search`
    /// - `web_search_preview`
    /// - `computer_use_preview`
    /// - `code_interpreter`
    /// - `mcp`
    /// - `image_generation`
    #[serde(rename = "type")]
    pub tool_type: String,
}

/// Use this option to force the model to call a specific function.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ToolChoiceFunction {
    /// The name of the function to call.
    pub name: String,

    /// For function calling, the type is always `function`.
    #[serde(rename = "type")]
    pub choice_type: String,
}

/// Use this option to force the model to call a specific custom tool.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ToolChoiceCustom {
    /// The name of the custom tool to call.
    pub name: String,

    /// For custom tool calling, the type is always `custom`.
    #[serde(rename = "type")]
    pub choice_type: String,
}

/// A tool that can be used to generate a response.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum Tool {
    Function(FunctionTool),
    WebSearch(WebSearchTool),
    #[serde(rename = "web_search_2025_08_26")]
    WebSearch202508(WebSearchTool),
    ImageGeneration(ToolImageGeneration),
}

/// Defines a function in your own code the model can choose to call. Learn more
/// about
/// [function calling](https://platform.openai.com/docs/guides/function-calling).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FunctionTool {
    /// The name of the function to call.
    pub name: String,

    /// A JSON schema object describing the parameters of the function.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub parameters: Option<serde_json::Value>,

    /// Whether to enforce strict parameter validation. Default `true`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub strict: Option<bool>,

    /// A description of the function. Used by the model to determine whether or
    /// not to call the function.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
}

/// Search the Internet for sources related to the prompt. Learn more about the
/// [web search tool](https://platform.openai.com/docs/guides/tools-web-search).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WebSearchTool {
    /// Filters for the search.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub filters: Option<WebSearchToolFilters>,

    /// High level guidance for the amount of context window space to use for
    /// the search. One of `low`, `medium`, or `high`. `medium` is the
    /// default.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub search_context_size: Option<String>,

    /// The approximate location of the user.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub user_location: Option<WebSearchToolUserLocation>,
}

/// Filters for the search.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WebSearchToolFilters {
    /// Allowed domains for the search. If not provided, all domains are
    /// allowed. Subdomains of the provided domains are allowed as well.
    ///
    /// Example: `["pubmed.ncbi.nlm.nih.gov"]`
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_domains: Option<Vec<String>>,
}

/// The approximate location of the user.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WebSearchToolUserLocation {
    /// Free text input for the city of the user, e.g. `San Francisco`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub city: Option<String>,

    /// The two-letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1) of
    /// the user, e.g. `US`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub country: Option<String>,

    /// Free text input for the region of the user, e.g. `California`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub region: Option<String>,

    /// The [IANA timezone](https://timeapi.io/documentation/iana-timezones) of the
    /// user, e.g. `America/Los_Angeles`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub timezone: Option<String>,

    /// The type of location approximation. Always `approximate`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub location_type: Option<String>,
}

/// A tool that generates images using a model like `gpt-image-1`.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ToolImageGeneration {
    /// Background type for the generated image. One of `transparent`, `opaque`,
    /// or `auto`. Default: `auto`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub background: Option<String>,

    /// Control how much effort the model will exert to match the style and
    /// features, especially facial features, of input images. This
    /// parameter is only supported for `gpt-image-1`. Supports `high` and
    /// `low`. Defaults to `low`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_fidelity: Option<String>,

    /// Optional mask for inpainting. Contains `image_url` (string, optional)
    /// and `file_id` (string, optional).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_image_mask: Option<ImageGenerationInputImageMask>,

    /// The image generation model to use. Default: `gpt-image-1`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,

    /// Moderation level for the generated image. Default: `auto`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub moderation: Option<String>,

    /// Compression level for the output image. Default: 100.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output_compression: Option<u32>,

    /// The output format of the generated image. One of `png`, `webp`, or
    /// `jpeg`. Default: `png`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output_format: Option<String>,

    /// Number of partial images to generate in streaming mode, from 0 (default
    /// value) to 3.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub partial_images: Option<u32>,

    /// The quality of the generated image. One of `low`, `medium`, `high`, or
    /// `auto`. Default: `auto`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub quality: Option<String>,

    /// The size of the generated image. One of `1024x1024`, `1024x1536`,
    /// `1536x1024`, or `auto`. Default: `auto`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub size: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ImageGenerationInputImageMask {
    /// File ID for the mask image.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub file_id: Option<String>,

    /// Base64-encoded mask image.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub image_url: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Response {
    /// Unique identifier for this Response.
    pub id: String,

    /// Unix timestamp (in seconds) of when this Response was created.
    pub created_at: i64,

    /// Model ID used to generate the response, like `gpt-4o` or `o3`. `OpenAI`
    /// offers a wide range of models with different capabilities,
    /// performance characteristics, and price points. Refer to the
    /// [model guide](https://platform.openai.com/docs/models) to browse and compare
    /// available models.
    pub model: String,

    /// The object type of this resource - always set to `response`.
    pub object: String,

    /// An array of content items generated by the model.
    ///
    /// - The length and order of items in the `output` array is dependent on
    ///   the model's response.
    /// - Rather than accessing the first item in the `output` array and
    ///   assuming it's an `assistant` message with the content generated by the
    ///   model, you might consider using the `output_text` property where
    ///   supported in SDKs.
    pub output: Vec<ResponseOutputItem>,

    /// The status of the response generation. One of `completed`, `failed`,
    /// `in_progress`, `cancelled`, `queued`, or `incomplete`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub status: Option<ResponseStatus>,

    /// Represents token usage details including input tokens, output tokens, a
    /// breakdown of output tokens, and the total tokens used.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub usage: Option<ResponseUsage>,
}

/// An output message from the model.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum ResponseOutputItem {
    Message(ResponseOutputMessage),
    FunctionCall(ResponseFunctionToolCall),
    WebSearchCall(ResponseFunctionWebSearch),
    Reasoning(ResponseReasoningItem),
    ImageGenerationCall(ResponseOutputItemImageGenerationCall),
}

/// The results of a web search tool call. See the
/// [web search guide](https://platform.openai.com/docs/guides/tools-web-search) for
/// more information.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseFunctionWebSearch {
    /// The unique ID of the web search tool call.
    pub id: String,

    /// The status of the web search tool call.
    pub status: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ResponseStatus {
    Completed,
    Failed,
    InProgress,
    Cancelled,
    Queued,
    Incomplete,
}

/// Represents token usage details including input tokens, output tokens, a
/// breakdown of output tokens, and the total tokens used.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseUsage {
    /// The number of input tokens.
    pub input_tokens: u32,

    /// A detailed breakdown of the input tokens.
    pub input_tokens_details: ResponseUsageInputTokensDetails,

    /// The number of output tokens.
    pub output_tokens: u32,

    /// A detailed breakdown of the output tokens.
    pub output_tokens_details: ResponseUsageOutputTokensDetails,

    /// The total number of tokens used.
    pub total_tokens: u32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseUsageInputTokensDetails {
    /// The number of tokens that were retrieved from the cache.
    /// [More on prompt caching](https://platform.openai.com/docs/guides/prompt-caching).
    pub cached_tokens: u32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseUsageOutputTokensDetails {
    /// The number of reasoning tokens.
    pub reasoning_tokens: u32,
}

/// Emitted when there is a partial audio response.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type")]
pub enum ResponseStreamEvent {
    #[serde(rename = "response.audio.delta")]
    AudioDelta(ResponseAudioDeltaEvent),
    #[serde(rename = "response.audio.done")]
    AudioDone(ResponseAudioDoneEvent),
    #[serde(rename = "response.audio.transcript.delta")]
    AudioTranscriptDelta(ResponseAudioTranscriptDeltaEvent),
    #[serde(rename = "response.audio.transcript.done")]
    AudioTranscriptDone(ResponseAudioTranscriptDoneEvent),
    #[serde(rename = "response.completed")]
    Completed(ResponseCompletedEvent),
    #[serde(rename = "response.content_part.added")]
    ContentPartAdded(ResponseContentPartAddedEvent),
    #[serde(rename = "response.content_part.done")]
    ContentPartDone(ResponseContentPartDoneEvent),
    #[serde(rename = "response.created")]
    Created(ResponseCreatedEvent),
    #[serde(rename = "error")]
    Error(ResponseErrorEvent),
    #[serde(rename = "response.function_call_arguments.delta")]
    FunctionCallArgumentsDelta(ResponseFunctionCallArgumentsDeltaEvent),
    #[serde(rename = "response.function_call_arguments.done")]
    FunctionCallArgumentsDone(ResponseFunctionCallArgumentsDoneEvent),
    #[serde(rename = "response.in_progress")]
    InProgress(ResponseInProgressEvent),
    #[serde(rename = "response.failed")]
    Failed(ResponseFailedEvent),
    #[serde(rename = "response.incomplete")]
    Incomplete(ResponseIncompleteEvent),
    #[serde(rename = "response.output_item.added")]
    OutputItemAdded(ResponseOutputItemAddedEvent),
    #[serde(rename = "response.output_item.done")]
    OutputItemDone(ResponseOutputItemDoneEvent),
    #[serde(rename = "response.reasoning_summary_part.added")]
    ReasoningSummaryPartAdded(ResponseReasoningSummaryPartAddedEvent),
    #[serde(rename = "response.reasoning_summary_part.done")]
    ReasoningSummaryPartDone(ResponseReasoningSummaryPartDoneEvent),
    #[serde(rename = "response.reasoning_summary_text.delta")]
    ReasoningSummaryTextDelta(ResponseReasoningSummaryTextDeltaEvent),
    #[serde(rename = "response.reasoning_summary_text.done")]
    ReasoningSummaryTextDone(ResponseReasoningSummaryTextDoneEvent),
    #[serde(rename = "response.reasoning_text.delta")]
    ReasoningTextDelta(ResponseReasoningTextDeltaEvent),
    #[serde(rename = "response.reasoning_text.done")]
    ReasoningTextDone(ResponseReasoningTextDoneEvent),
    #[serde(rename = "response.refusal.delta")]
    RefusalDelta(ResponseRefusalDeltaEvent),
    #[serde(rename = "response.refusal.done")]
    RefusalDone(ResponseRefusalDoneEvent),
    #[serde(rename = "response.output_text.delta")]
    TextDelta(ResponseTextDeltaEvent),
    #[serde(rename = "response.output_text.done")]
    TextDone(ResponseTextDoneEvent),
    #[serde(rename = "response.image_generation_call.completed")]
    ImageGenCallCompleted(ResponseImageGenCallCompletedEvent),
    #[serde(rename = "response.image_generation_call.generating")]
    ImageGenCallGenerating(ResponseImageGenCallGeneratingEvent),
    #[serde(rename = "response.image_generation_call.in_progress")]
    ImageGenCallInProgress(ResponseImageGenCallInProgressEvent),
    #[serde(rename = "response.image_generation_call.partial_image")]
    ImageGenCallPartialImage(ResponseImageGenCallPartialImageEvent),
    #[serde(rename = "response.output_text.annotation.added")]
    OutputTextAnnotationAdded(ResponseOutputTextAnnotationAddedEvent),

    #[serde(untagged)]
    Other {},
}

/// Emitted when there is a partial audio response.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseAudioDeltaEvent {
    /// A chunk of Base64 encoded response audio bytes.
    pub delta: String,

    /// A sequence number for this chunk of the stream response.
    pub sequence_number: u32,
}

/// Emitted when the audio response is complete.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseAudioDoneEvent {
    /// The sequence number of the delta.
    pub sequence_number: u32,
}

/// Emitted when there is a partial transcript of audio.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseAudioTranscriptDeltaEvent {
    /// The partial transcript of the audio response.
    pub delta: String,

    /// The sequence number of this event.
    pub sequence_number: u32,
}

/// Emitted when the full audio transcript is completed.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseAudioTranscriptDoneEvent {
    /// The sequence number of this event.
    pub sequence_number: u32,
}

/// Emitted when the model response is complete.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseCompletedEvent {
    /// Properties of the completed response.
    pub response: Response,

    /// The sequence number for this event.
    pub sequence_number: u32,
}

/// Emitted when a new content part is added.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseContentPartAddedEvent {
    /// The index of the content part that was added.
    pub content_index: u32,

    /// The ID of the output item that the content part was added to.
    pub item_id: String,

    /// The index of the output item that the content part was added to.
    pub output_index: usize,

    /// The content part that was added.
    pub part: ResponseContentPartEventPart,

    /// The sequence number of this event.
    pub sequence_number: u32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum ResponseContentPartEventPart {
    OutputText(ResponseOutputText),
    Refusal(ResponseOutputRefusal),
}

/// Emitted when a content part is done.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseContentPartDoneEvent {
    /// The index of the content part that is done.
    pub content_index: u32,

    /// The ID of the output item that the content part was added to.
    pub item_id: String,

    /// The index of the output item that the content part was added to.
    pub output_index: usize,

    /// The content part that is done.
    pub part: ResponseContentPartEventPart,

    /// The sequence number of this event.
    pub sequence_number: u32,
}

/// An event that is emitted when a response is created.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseCreatedEvent {
    /// The response that was created.
    pub response: Response,

    /// The sequence number for this event.
    pub sequence_number: u32,
}

/// Emitted when an error occurs.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseErrorEvent {
    /// The error code.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub code: Option<String>,

    /// The error message.
    pub message: String,

    /// The error parameter.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub param: Option<String>,

    /// The sequence number of this event.
    pub sequence_number: u32,
}

/// Emitted when there is a partial function-call arguments delta.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseFunctionCallArgumentsDeltaEvent {
    /// The function-call arguments delta that is added.
    pub delta: String,

    /// The ID of the output item that the function-call arguments delta is
    /// added to.
    pub item_id: String,

    /// The index of the output item that the function-call arguments delta is
    /// added to.
    pub output_index: usize,

    /// The sequence number of this event.
    pub sequence_number: u32,
}

/// Emitted when function-call arguments are finalized.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseFunctionCallArgumentsDoneEvent {
    /// The function-call arguments.
    pub arguments: String,

    /// The ID of the item.
    pub item_id: String,

    /// The index of the output item.
    pub output_index: usize,

    /// The sequence number of this event.
    pub sequence_number: u32,
}

/// Emitted when the response is in progress.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseInProgressEvent {
    /// The response that is in progress.
    pub response: Response,

    /// The sequence number of this event.
    pub sequence_number: u32,
}

/// An event that is emitted when a response fails.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseFailedEvent {
    /// The response that failed.
    pub response: Response,

    /// The sequence number of this event.
    pub sequence_number: u32,
}

/// An event that is emitted when a response finishes as incomplete.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseIncompleteEvent {
    /// The response that was incomplete.
    pub response: Response,

    /// The sequence number of this event.
    pub sequence_number: u32,
}

/// Emitted when a new output item is added.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseOutputItemAddedEvent {
    /// The output item that was added.
    pub item: ResponseOutputItem,

    /// The index of the output item that was added.
    pub output_index: usize,

    /// The sequence number of this event.
    pub sequence_number: u32,
}

/// Emitted when an output item is marked done.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseOutputItemDoneEvent {
    /// The output item that was marked done.
    pub item: ResponseOutputItem,

    /// The index of the output item that was marked done.
    pub output_index: usize,

    /// The sequence number of this event.
    pub sequence_number: u32,
}

/// Emitted when a new reasoning summary part is added.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseReasoningSummaryPartAddedEvent {
    /// The ID of the item this summary part is associated with.
    pub item_id: String,

    /// The index of the output item this summary part is associated with.
    pub output_index: usize,

    /// The summary part that was added.
    pub part: ResponseReasoningSummaryPartAddedEventPart,

    /// The sequence number of this event.
    pub sequence_number: u32,

    /// The index of the summary part within the reasoning summary.
    pub summary_index: u32,
}

/// The summary part that was added.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseReasoningSummaryPartAddedEventPart {
    /// The text of the summary part.
    pub text: String,
}

/// Emitted when a reasoning summary part is completed.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseReasoningSummaryPartDoneEvent {
    /// The ID of the item this summary part is associated with.
    pub item_id: String,

    /// The index of the output item this summary part is associated with.
    pub output_index: usize,

    /// The completed summary part.
    pub part: ResponseReasoningSummaryPartDoneEventPart,

    /// The sequence number of this event.
    pub sequence_number: u32,

    /// The index of the summary part within the reasoning summary.
    pub summary_index: u32,
}

/// The completed summary part.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseReasoningSummaryPartDoneEventPart {
    /// The text of the summary part.
    pub text: String,
}

/// Emitted when a delta is added to a reasoning summary text.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseReasoningSummaryTextDeltaEvent {
    /// The text delta that was added to the summary.
    pub delta: String,

    /// The ID of the item this summary text delta is associated with.
    pub item_id: String,

    /// The index of the output item this summary text delta is associated with.
    pub output_index: usize,

    /// The sequence number of this event.
    pub sequence_number: u32,

    /// The index of the summary part within the reasoning summary.
    pub summary_index: u32,
}

/// Emitted when a reasoning summary text is completed.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseReasoningSummaryTextDoneEvent {
    /// The ID of the item this summary text is associated with.
    pub item_id: String,

    /// The index of the output item this summary text is associated with.
    pub output_index: usize,

    /// The sequence number of this event.
    pub sequence_number: u32,

    /// The index of the summary part within the reasoning summary.
    pub summary_index: u32,

    /// The full text of the completed reasoning summary.
    pub text: String,
}

/// Emitted when a delta is added to a reasoning text.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseReasoningTextDeltaEvent {
    /// The index of the reasoning content part this delta is associated with.
    pub content_index: u32,

    /// The text delta that was added to the reasoning content.
    pub delta: String,

    /// The ID of the item this reasoning text delta is associated with.
    pub item_id: String,

    /// The index of the output item this reasoning text delta is associated
    /// with.
    pub output_index: usize,

    /// The sequence number of this event.
    pub sequence_number: u32,
}

/// Emitted when a reasoning text is completed.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseReasoningTextDoneEvent {
    /// The index of the reasoning content part.
    pub content_index: u32,

    /// The ID of the item this reasoning text is associated with.
    pub item_id: String,

    /// The index of the output item this reasoning text is associated with.
    pub output_index: usize,

    /// The sequence number of this event.
    pub sequence_number: u32,

    /// The full text of the completed reasoning content.
    pub text: String,
}

/// Emitted when there is a partial refusal text.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseRefusalDeltaEvent {
    /// The index of the content part that the refusal text is added to.
    pub content_index: u32,

    /// The refusal text that is added.
    pub delta: String,

    /// The ID of the output item that the refusal text is added to.
    pub item_id: String,

    /// The index of the output item that the refusal text is added to.
    pub output_index: usize,

    /// The sequence number of this event.
    pub sequence_number: u32,
}

/// Emitted when refusal text is finalized.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseRefusalDoneEvent {
    /// The index of the content part that the refusal text is finalized.
    pub content_index: u32,

    /// The ID of the output item that the refusal text is finalized.
    pub item_id: String,

    /// The index of the output item that the refusal text is finalized.
    pub output_index: usize,

    /// The refusal text that is finalized.
    pub refusal: String,

    /// The sequence number of this event.
    pub sequence_number: u32,
}

/// Emitted when there is an additional text delta.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseTextDeltaEvent {
    /// The index of the content part that the text delta was added to.
    pub content_index: u32,

    /// The text delta that was added.
    pub delta: String,

    /// The ID of the output item that the text delta was added to.
    pub item_id: String,

    /// The index of the output item that the text delta was added to.
    pub output_index: usize,

    /// The sequence number for this event.
    pub sequence_number: u32,
}

/// Emitted when text content is finalized.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseTextDoneEvent {
    /// The index of the content part that the text content is finalized.
    pub content_index: u32,

    /// The ID of the output item that the text content is finalized.
    pub item_id: String,

    /// The index of the output item that the text content is finalized.
    pub output_index: usize,

    /// The sequence number for this event.
    pub sequence_number: u32,

    /// The text content that is finalized.
    pub text: String,
}

/// Emitted when an image generation tool call has completed and the final image
/// is available.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseImageGenCallCompletedEvent {
    /// The unique identifier of the image generation item being processed.
    pub item_id: String,

    /// The index of the output item in the response's output array.
    pub output_index: usize,

    /// The sequence number of this event.
    pub sequence_number: u32,
}

/// Emitted when an image generation tool call is actively generating an image
/// (intermediate state).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseImageGenCallGeneratingEvent {
    /// The unique identifier of the image generation item being processed.
    pub item_id: String,

    /// The index of the output item in the response's output array.
    pub output_index: usize,

    /// The sequence number of the image generation item being processed.
    pub sequence_number: u32,
}

/// Emitted when an image generation tool call is in progress.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseImageGenCallInProgressEvent {
    /// The unique identifier of the image generation item being processed.
    pub item_id: String,

    /// The index of the output item in the response's output array.
    pub output_index: usize,

    /// The sequence number of the image generation item being processed.
    pub sequence_number: u32,
}

/// Emitted when a partial image is available during image generation streaming.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseImageGenCallPartialImageEvent {
    /// The unique identifier of the image generation item being processed.
    pub item_id: String,

    /// The index of the output item in the response's output array.
    pub output_index: usize,

    /// Base64-encoded partial image data, suitable for rendering as an image.
    pub partial_image_b64: String,

    /// 0-based index for the partial image (backend is 1-based, but this is
    /// 0-based for the user).
    pub partial_image_index: u32,

    /// The sequence number of the image generation item being processed.
    pub sequence_number: u32,

    pub size: Option<String>,
    pub output_format: String,
}

/// Emitted when an annotation is added to output text content.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseOutputTextAnnotationAddedEvent {
    /// The annotation object being added. (See annotation schema for details.)
    pub annotation: serde_json::Value,

    /// The index of the annotation within the content part.
    pub annotation_index: u32,

    /// The index of the content part within the output item.
    pub content_index: u32,

    /// The unique identifier of the item to which the annotation is being
    /// added.
    pub item_id: String,

    /// The index of the output item in the response's output array.
    pub output_index: usize,

    /// The sequence number of this event.
    pub sequence_number: u32,
}
