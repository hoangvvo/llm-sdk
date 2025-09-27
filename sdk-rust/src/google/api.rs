#![allow(clippy::pedantic, clippy::style)]
use crate::LanguageModelInputExtra;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::HashMap;

/// Config for `models.generate_content` parameters.
#[derive(Serialize, Deserialize, Debug, Clone, Default)]
#[serde(rename_all = "camelCase")]
pub struct GenerateContentParameters {
    /// ID of the model to use. For a list of models, see `Google models
    /// <https://cloud.google.com/vertex-ai/generative-ai/docs/learn/models>`_.
    pub model: String,
    /// Content of the request.
    pub contents: Vec<Content>,
    /// Configuration that contains optional model parameters.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tools: Option<Vec<Tool>>,
    /// Associates model output to a specific function call.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_config: Option<ToolConfig>,
    /// Instructions for the model to steer it toward better performance.
    /// For example, "Answer as concisely as possible" or "Don't use technical
    /// terms in your response".
    #[serde(skip_serializing_if = "Option::is_none")]
    pub system_instruction: Option<Content>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub generation_config: Option<GenerateContentConfig>,
    #[serde(skip_serializing_if = "Option::is_none", flatten)]
    pub extra: Option<LanguageModelInputExtra>,
}

/// Contains the multi-part content of a message.
#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "camelCase")]
pub struct Content {
    /// List of parts that constitute a single message. Each part may have
    /// a different IANA MIME type.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub parts: Option<Vec<Part>>,
    /// Optional. The producer of the content. Must be either 'user' or
    /// 'model'. Useful to set for multi-turn conversations, otherwise can be
    /// empty. If role is not specified, SDK will determine the role.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub role: Option<String>,
}

/// A datatype containing media content.
///
/// Exactly one field within a Part should be set, representing the specific
/// type of content being conveyed. Using multiple fields within the same `Part`
/// instance is considered invalid.
#[derive(Serialize, Deserialize, Debug, Clone, Default)]
#[serde(rename_all = "camelCase")]
pub struct Part {
    /// Indicates if the part is thought from the model.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub thought: Option<bool>,
    /// Optional. Inlined bytes data.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub inline_data: Option<Blob2>,
    /// Optional. URI based data.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub file_data: Option<FileData>,
    /// An opaque signature for the thought so it can be reused in subsequent
    /// requests. @remarks Encoded as base64 string.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub thought_signature: Option<String>,
    /// Optional. A predicted [`FunctionCall`] returned from the model that
    /// contains a string representing the [FunctionDeclaration.name] with the
    /// parameters and their values.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub function_call: Option<FunctionCall>,
    /// Optional. The result output of a [`FunctionCall`] that contains a string
    /// representing the [FunctionDeclaration.name] and a structured JSON object
    /// containing any output from the function call. It is used as context to
    /// the model.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub function_response: Option<FunctionResponse>,
    /// Optional. Text part (can be code).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub text: Option<String>,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "camelCase")]
pub struct Blob2 {
    /// Optional. Display name of the blob. Used to provide a label or filename
    /// to distinguish blobs. This field is not currently used in the Gemini
    /// `GenerateContent` calls.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub display_name: Option<String>,
    /// Required. Raw bytes.
    /// @remarks Encoded as base64 string.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub data: Option<String>,
    /// Required. The IANA standard MIME type of the source data.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub mime_type: Option<String>,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "camelCase")]
pub struct FileData {
    /// Optional. Display name of the file data. Used to provide a label or
    /// filename to distinguish file datas. It is not currently used in the
    /// Gemini `GenerateContent` calls.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub display_name: Option<String>,
    /// Required. URI.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub file_uri: Option<String>,
    /// Required. The IANA standard MIME type of the source data.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub mime_type: Option<String>,
}

/// A function call.
#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "camelCase")]
pub struct FunctionCall {
    /// The unique id of the function call. If populated, the client to execute
    /// the `function_call` and return the response with the matching `id`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
    /// Optional. The function parameters and values in JSON object format. See
    /// [FunctionDeclaration.parameters] for parameter details.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub args: Option<Value>,
    /// Required. The name of the function to call. Matches
    /// [FunctionDeclaration.name].
    #[serde(skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
}

/// A function response.
#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "camelCase")]
pub struct FunctionResponse {
    /// Optional. The id of the function call this response is for. Populated by
    /// the client to match the corresponding function call `id`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
    /// Required. The name of the function to call. Matches
    /// [FunctionDeclaration.name] and [FunctionCall.name].
    #[serde(skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    /// Required. The function response in JSON object format. Use "output" key
    /// to specify function output and "error" key to specify error details (if
    /// any). If "output" and "error" keys are not specified, then whole
    /// "response" is treated as function output.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub response: Option<HashMap<String, Value>>,
}

/// Optional model configuration parameters.
///
/// For more information, see `Content generation parameters
/// <https://cloud.google.com/vertex-ai/generative-ai/docs/multimodal/content-generation-parameters>`_.
#[derive(Serialize, Deserialize, Debug, Clone, Default)]
#[serde(rename_all = "camelCase")]
pub struct GenerateContentConfig {
    /// Value that controls the degree of randomness in token selection.
    /// Lower temperatures are good for prompts that require a less open-ended
    /// or creative response, while higher temperatures can lead to more
    /// diverse or creative results.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub temperature: Option<f64>,
    /// Tokens are selected from the most to least probable until the sum
    /// of their probabilities equals this value. Use a lower value for less
    /// random responses and a higher value for more random responses.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub top_p: Option<f64>,
    /// For each token selection step, the ``top_k`` tokens with the
    /// highest probabilities are sampled. Then tokens are further filtered
    /// based on ``top_p`` with the final token selected using temperature
    /// sampling. Use a lower number for less random responses and a higher
    /// number for more random responses. Must be a non-negative integer.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub top_k: Option<i32>,
    /// Number of response variations to return.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub candidate_count: Option<i32>,
    /// Maximum number of tokens that can be generated in the response.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_output_tokens: Option<u32>,
    /// List of strings that tells the model to stop generating text if one
    /// of the strings is encountered in the response.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub stop_sequences: Option<Vec<String>>,
    /// Positive values penalize tokens that already appear in the
    /// generated text, increasing the probability of generating more diverse
    /// content.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub presence_penalty: Option<f64>,
    /// Positive values penalize tokens that repeatedly appear in the
    /// generated text, increasing the probability of generating more diverse
    /// content.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub frequency_penalty: Option<f64>,
    /// When ``seed`` is fixed to a specific number, the model makes a best
    /// effort to provide the same response for repeated requests. By default, a
    /// random number is used.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub seed: Option<i64>,
    /// Output response mimetype of the generated candidate text.
    /// Supported mimetype:
    /// - `text/plain`: (default) Text output.
    /// - `application/json`: JSON response in the candidates.
    /// The model needs to be prompted to output the appropriate response type,
    /// otherwise the behavior is undefined.
    /// This is a preview feature.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub response_mime_type: Option<String>,
    /// Optional. Output schema of the generated response. This is an alternative to `response_schema` that accepts [JSON Schema](https://json-schema.org/). If set, `response_schema` must be omitted, but `response_mime_type` is required. While the full JSON Schema may be sent, not all features are supported. Specifically, only the following properties are supported: - `$id` - `$defs` - `$ref` - `$anchor` - `type` - `format` - `title` - `description` - `enum` (for strings and numbers) - `items` - `prefixItems` - `minItems` - `maxItems` - `minimum` - `maximum` - `anyOf` - `oneOf` (interpreted the same as `anyOf`) - `properties` - `additionalProperties` - `required` The non-standard `propertyOrdering` property may also be set. Cyclic references are unrolled to a limited degree and, as such, may only be used within non-required properties. (Nullable properties are not sufficient.) If `$ref` is set on a sub-schema, no other properties, except for than those starting as a `$`, may be set.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub response_json_schema: Option<Value>,
    /// The requested modalities of the response. Represents the set of
    /// modalities that the model can return.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub response_modalities: Option<Vec<String>>,
    /// The speech generation configuration.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub speech_config: Option<SpeechConfig>,
    /// If enabled, audio timestamp will be included in the request to the
    /// model.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audio_timestamp: Option<bool>,
    /// The thinking features configuration.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub thinking_config: Option<ThinkingConfig>,
}

/// Tool details of a tool that the model may use to generate a response.
#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "camelCase")]
pub struct Tool {
    /// List of function declarations that the tool supports.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub function_declarations: Option<Vec<FunctionDeclaration>>,
}

/// Defines a function that the model can generate JSON inputs for.
///
/// The inputs are based on `OpenAPI 3.0 specifications
/// <https://spec.openapis.org/oas/v3.0.3>`_.
#[derive(Serialize, Deserialize, Debug, Clone, Default)]
#[serde(rename_all = "camelCase")]
pub struct FunctionDeclaration {
    /// Optional. Description and purpose of the function. Model uses it to
    /// decide how and whether to call the function.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    /// Required. The name of the function to call. Must start with a letter or
    /// an underscore. Must be a-z, A-Z, 0-9, or contain underscores, dots and
    /// dashes, with a maximum length of 64.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    /// Optional. Describes the parameters to this function in JSON Schema
    /// Object format. Reflects the Open API 3.03 Parameter Object. string Key:
    /// the name of the parameter. Parameter names are case sensitive. Schema
    /// Value: the Schema defining the type used for the parameter. For function
    /// with no parameters, this can be left unset. Parameter names must start
    /// with a letter or an underscore and must only contain chars a-z, A-Z,
    /// 0-9, or underscores with a maximum length of 64. Example with 1 required
    /// and 1 optional parameter: type: OBJECT properties: param1: type: STRING
    /// param2: type: INTEGER required: - param1
    #[serde(skip_serializing_if = "Option::is_none")]
    pub parameters: Option<HashMap<String, Value>>,
    /// Optional. Describes the parameters to the function in JSON Schema
    /// format. The schema must describe an object where the properties are the
    /// parameters to the function. For example: ``` { "type": "object",
    /// "properties": { "name": { "type": "string" }, "age": { "type": "integer"
    /// } }, "additionalProperties": false, "required": ["name", "age"],
    /// "propertyOrdering": ["name", "age"] } ``` This field is mutually
    /// exclusive with `parameters`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub parameters_json_schema: Option<Value>,
    /// Optional. Describes the output from this function in JSON Schema format.
    /// Reflects the Open API 3.03 Response Object. The Schema defines the type
    /// used for the response value of the function.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub response: Option<HashMap<String, Value>>,
    /// Optional. Describes the output from this function in JSON Schema format.
    /// The value specified by the schema is the response value of the function.
    /// This field is mutually exclusive with `response`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub response_json_schema: Option<Value>,
}

/// Tool config.
///
/// This config is shared for all tools provided in the request.
#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "camelCase")]
pub struct ToolConfig {
    /// Optional. Function calling config.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub function_calling_config: Option<FunctionCallingConfig>,
}

/// Function calling config.
#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "camelCase")]
pub struct FunctionCallingConfig {
    /// Optional. Function calling mode.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub mode: Option<FunctionCallingConfigMode>,
    /// Optional. Function names to call. Only set when the Mode is ANY.
    /// Function names should match [FunctionDeclaration.name]. With mode set to
    /// ANY, model will predict a function call from the set of function names
    /// provided.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allowed_function_names: Option<Vec<String>>,
}

/// Config for the function calling config mode.
#[derive(Serialize, Deserialize, Debug, Clone, Copy)]
pub enum FunctionCallingConfigMode {
    /// The function calling config mode is unspecified. Should not be used.
    #[serde(rename = "MODE_UNSPECIFIED")]
    ModeUnspecified,
    /// Default model behavior, model decides to predict either function calls
    /// or natural language response.
    #[serde(rename = "AUTO")]
    Auto,
    /// Model is constrained to always predicting function calls only. If
    /// "`allowed_function_names`" are set, the predicted function calls will be
    /// limited to any one of "`allowed_function_names`", else the predicted
    /// function calls will be any one of the provided
    /// "`function_declarations`".
    #[serde(rename = "ANY")]
    Any,
    /// Model will not predict any function calls. Model behavior is same as
    /// when not passing any function declarations.
    #[serde(rename = "NONE")]
    None,
    /// Model decides to predict either a function call or a natural language
    /// response, but will validate function calls with constrained decoding. If
    /// "`allowed_function_names`" are set, the predicted function call will be
    /// limited to any one of "`allowed_function_names`", else the predicted
    /// function call will be any one of the provided "`function_declarations`".
    #[serde(rename = "VALIDATED")]
    Validated,
}

/// The speech generation configuration.
#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "camelCase")]
pub struct SpeechConfig {
    /// The configuration for the speaker to use.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub voice_config: Option<VoiceConfig>,
    /// The configuration for the multi-speaker setup.
    /// It is mutually exclusive with the `voice_config` field.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub multi_speaker_voice_config: Option<MultiSpeakerVoiceConfig>,
    /// Language code (ISO 639. e.g. en-US) for the speech synthesization.
    /// Only available for Live API.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub language_code: Option<String>,
}

/// The configuration for the voice to use.
#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "camelCase")]
pub struct VoiceConfig {
    /// The configuration for the speaker to use.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub prebuilt_voice_config: Option<PrebuiltVoiceConfig>,
}

/// The configuration for the prebuilt speaker to use.
#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "camelCase")]
pub struct PrebuiltVoiceConfig {
    /// The name of the prebuilt voice to use.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub voice_name: Option<String>,
}

/// The configuration for the multi-speaker setup.
#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "camelCase")]
pub struct MultiSpeakerVoiceConfig {
    /// The configuration for the speaker to use.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub speaker_voice_configs: Option<Vec<SpeakerVoiceConfig>>,
}

/// The configuration for the speaker to use.
#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "camelCase")]
pub struct SpeakerVoiceConfig {
    /// The name of the speaker to use. Should be the same as in the
    /// prompt.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub speaker: Option<String>,
    /// The configuration for the voice to use.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub voice_config: Option<VoiceConfig>,
}

/// The thinking features configuration.
#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "camelCase")]
pub struct ThinkingConfig {
    /// Indicates whether to include thoughts in the response. If true, thoughts
    /// are returned only if the model supports thought and thoughts are
    /// available.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub include_thoughts: Option<bool>,
    /// Indicates the thinking budget in tokens. 0 is DISABLED. -1 is AUTOMATIC.
    /// The default values and allowed ranges are model dependent.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub thinking_budget: Option<i32>,
}

/// Response message for PredictionService.GenerateContent.
#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "camelCase")]
pub struct GenerateContentResponse {
    /// Response variations returned by the model.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub candidates: Option<Vec<Candidate>>,
    /// Timestamp when the request is made to the server.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub create_time: Option<String>,
    /// Output only. The model version used to generate the response.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub model_version: Option<String>,
    /// Output only. `response_id` is used to identify each response. It is the
    /// encoding of the `event_id`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub response_id: Option<String>,
    /// Usage metadata about the response(s).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub usage_metadata: Option<GenerateContentResponseUsageMetadata>,
}

/// A response candidate generated from the model.
#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "camelCase")]
pub struct Candidate {
    /// Contains the multi-part content of the response.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub content: Option<Content>,
    /// Source attribution of the generated content.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub citation_metadata: Option<CitationMetadata>,
    /// Describes the reason the model stopped generating tokens.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub finish_message: Option<String>,
    /// Number of tokens for this candidate.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub token_count: Option<i32>,
    /// The reason why the model stopped generating tokens.
    /// If empty, the model has not stopped generating the tokens.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub finish_reason: Option<FinishReason>,
    /// Output only. Average log probability score of the candidate.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub avg_logprobs: Option<f64>,
    /// Output only. Index of the candidate.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub index: Option<i32>,
}

/// Citation information when the model quotes another source.
#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "camelCase")]
pub struct CitationMetadata {
    /// Contains citation information when the model directly quotes, at
    /// length, from another source. Can include traditional websites and code
    /// repositories.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub citations: Option<Vec<Citation>>,
}

/// Source attributions for content.
#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "camelCase")]
pub struct Citation {
    /// Output only. End index into the content.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub end_index: Option<i32>,
    /// Output only. License of the attribution.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub license: Option<String>,
    /// Output only. Publication date of the attribution.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub publication_date: Option<GoogleTypeDate>,
    /// Output only. Start index into the content.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub start_index: Option<i32>,
    /// Output only. Title of the attribution.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub title: Option<String>,
    /// Output only. Url reference of the attribution.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub uri: Option<String>,
}

/// Represents a whole or partial calendar date, such as a birthday. The time of
/// day and time zone are either specified elsewhere or are insignificant. The
/// date is relative to the Gregorian Calendar. This can represent one of the
/// following: * A full date, with non-zero year, month, and day values. * A
/// month and day, with a zero year (for example, an anniversary). * A year on
/// its own, with a zero month and a zero day. * A year and month, with a zero
/// day (for example, a credit card expiration date). Related types: *
/// google.type.TimeOfDay * google.type.DateTime * google.protobuf.Timestamp
#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "camelCase")]
pub struct GoogleTypeDate {
    /// Day of a month. Must be from 1 to 31 and valid for the year and month,
    /// or 0 to specify a year by itself or a year and month where the day isn't
    /// significant.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub day: Option<i32>,
    /// Month of a year. Must be from 1 to 12, or 0 to specify a year without a
    /// month and day.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub month: Option<i32>,
    /// Year of the date. Must be from 1 to 9999, or 0 to specify a date without
    /// a year.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub year: Option<i32>,
}

/// Output only. The reason why the model stopped generating tokens.
///
/// If empty, the model has not stopped generating the tokens.
#[derive(Serialize, Deserialize, Debug, Clone, Copy)]
pub enum FinishReason {
    /// The finish reason is unspecified.
    #[serde(rename = "FINISH_REASON_UNSPECIFIED")]
    Unspecified,
    /// Token generation reached a natural stopping point or a configured stop
    /// sequence.
    #[serde(rename = "STOP")]
    Stop,
    /// Token generation reached the configured maximum output tokens.
    #[serde(rename = "MAX_TOKENS")]
    MaxTokens,
    /// Token generation stopped because the content potentially contains safety
    /// violations. NOTE: When streaming, [content][] is empty if content
    /// filters blocks the output.
    #[serde(rename = "SAFETY")]
    Safety,
    /// The token generation stopped because of potential recitation.
    #[serde(rename = "RECITATION")]
    Recitation,
    /// The token generation stopped because of using an unsupported language.
    #[serde(rename = "LANGUAGE")]
    Language,
    /// All other reasons that stopped the token generation.
    #[serde(rename = "OTHER")]
    Other,
    /// Token generation stopped because the content contains forbidden terms.
    #[serde(rename = "BLOCKLIST")]
    Blocklist,
    /// Token generation stopped for potentially containing prohibited content.
    #[serde(rename = "PROHIBITED_CONTENT")]
    ProhibitedContent,
    /// Token generation stopped because the content potentially contains
    /// Sensitive Personally Identifiable Information (SPII).
    #[serde(rename = "SPII")]
    Spii,
    /// The function call generated by the model is invalid.
    #[serde(rename = "MALFORMED_FUNCTION_CALL")]
    MalformedFunctionCall,
    /// Token generation stopped because generated images have safety
    /// violations.
    #[serde(rename = "IMAGE_SAFETY")]
    ImageSafety,
    /// The tool call generated by the model is invalid.
    #[serde(rename = "UNEXPECTED_TOOL_CALL")]
    UnexpectedToolCall,
}

/// Usage metadata about response(s).
#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "camelCase")]
pub struct GenerateContentResponseUsageMetadata {
    /// Output only. List of modalities of the cached content in the request
    /// input.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_tokens_details: Option<Vec<ModalityTokenCount>>,
    /// Output only. Number of tokens in the cached part in the input (the
    /// cached content).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cached_content_token_count: Option<u32>,
    /// Number of tokens in the response(s).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub candidates_token_count: Option<u32>,
    /// Output only. List of modalities that were returned in the response.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub candidates_tokens_details: Option<Vec<ModalityTokenCount>>,
    /// Number of tokens in the request. When `cached_content` is set, this is
    /// still the total effective prompt size meaning this includes the number
    /// of tokens in the cached content.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub prompt_token_count: Option<u32>,
    /// Output only. List of modalities that were processed in the request
    /// input.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub prompt_tokens_details: Option<Vec<ModalityTokenCount>>,
    /// Output only. Number of tokens present in thoughts output.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub thoughts_token_count: Option<u32>,
    /// Output only. Number of tokens present in tool-use prompt(s).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_use_prompt_token_count: Option<u32>,
    /// Output only. List of modalities that were processed for tool-use request
    /// inputs.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_use_prompt_tokens_details: Option<Vec<ModalityTokenCount>>,
    /// Total token count for prompt, response candidates, and tool-use prompts
    /// (if present).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub total_token_count: Option<i32>,
}

/// Represents token counting info for a single modality.
#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "camelCase")]
pub struct ModalityTokenCount {
    /// The modality associated with this token count.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub modality: Option<MediaModality>,
    /// Number of tokens.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub token_count: Option<u32>,
}

/// Server content modalities.
#[derive(Serialize, Deserialize, Debug, Clone, Copy)]
pub enum MediaModality {
    /// The modality is unspecified.
    #[serde(rename = "MODALITY_UNSPECIFIED")]
    ModalityUnspecified,
    /// Plain text.
    #[serde(rename = "TEXT")]
    Text,
    /// Images.
    #[serde(rename = "IMAGE")]
    Image,
    /// Video.
    #[serde(rename = "VIDEO")]
    Video,
    /// Audio.
    #[serde(rename = "AUDIO")]
    Audio,
    /// Document, e.g. PDF.
    #[serde(rename = "DOCUMENT")]
    Document,
}
