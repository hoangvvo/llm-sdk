package googleapi

import "encoding/json"

// Config for models.generate_content parameters.
type GenerateContentParameters struct {
	// ID of the model to use. For a list of models, see `Google models
	// <https://cloud.google.com/vertex-ai/generative-ai/docs/learn/models>`_.
	Model string `json:"model"`
	// Content of the request.
	Contents []Content `json:"contents"`
	// Configuration that contains optional model parameters.
	// Code that enables the system to interact with external systems to
	// perform an action outside of the knowledge and scope of the model.
	Tools []Tool `json:"tools,omitempty"`
	// Associates model output to a specific function call.
	ToolConfig *ToolConfig `json:"toolConfig,omitempty"`
	// Instructions for the model to steer it toward better performance.
	// For example, "Answer as concisely as possible" or "Don't use technical
	// terms in your response".
	SystemInstruction *Content               `json:"systemInstruction,omitempty"`
	GenerationConfig  *GenerateContentConfig `json:"generationConfig,omitempty"`
}

// Contains the multi-part content of a message.
type Content struct {
	// List of parts that constitute a single message. Each part may have
	// a different IANA MIME type.
	Parts []Part `json:"parts,omitempty"`
	// Optional. The producer of the content. Must be either 'user' or
	// 'model'. Useful to set for multi-turn conversations, otherwise can be
	// empty. If role is not specified, SDK will determine the role.
	Role string `json:"role,omitempty"`
}

// A datatype containing media content.
//
// Exactly one field within a Part should be set, representing the specific type
// of content being conveyed. Using multiple fields within the same `Part`
// instance is considered invalid.
type Part struct {
	// Indicates if the part is thought from the model.
	Thought *bool `json:"thought,omitempty"`
	// Optional. Inlined bytes data.
	InlineData *Blob2 `json:"inlineData,omitempty"`
	// Optional. URI based data.
	FileData *FileData `json:"fileData,omitempty"`
	// An opaque signature for the thought so it can be reused in subsequent requests.
	// @remarks Encoded as base64 string.
	ThoughtSignature *string `json:"thoughtSignature,omitempty"`
	// Optional. A predicted [FunctionCall] returned from the model that contains a string representing the [FunctionDeclaration.name] with the parameters and their values.
	FunctionCall *FunctionCall `json:"functionCall,omitempty"`
	// Optional. The result output of a [FunctionCall] that contains a string representing the [FunctionDeclaration.name] and a structured JSON object containing any output from the function call. It is used as context to the model.
	FunctionResponse *FunctionResponse `json:"functionResponse,omitempty"`
	// Optional. Text part (can be code).
	Text *string `json:"text,omitempty"`
}

type Blob2 struct {
	// Optional. Display name of the blob. Used to provide a label or filename to distinguish blobs. This field is not currently used in the Gemini GenerateContent calls.
	DisplayName *string `json:"displayName,omitempty"`
	// Required. Raw bytes.
	// @remarks Encoded as base64 string.
	Data *string `json:"data,omitempty"`
	// Required. The IANA standard MIME type of the source data.
	MimeType *string `json:"mimeType,omitempty"`
}

type FileData struct {
	// Optional. Display name of the file data. Used to provide a label or filename to distinguish file datas. It is not currently used in the Gemini GenerateContent calls.
	DisplayName *string `json:"displayName,omitempty"`
	// Required. URI.
	FileUri *string `json:"fileUri,omitempty"`
	// Required. The IANA standard MIME type of the source data.
	MimeType *string `json:"mimeType,omitempty"`
}

// A function call.
type FunctionCall struct {
	// The unique id of the function call. If populated, the client to execute the
	// `function_call` and return the response with the matching `id`.
	Id *string `json:"id,omitempty"`
	// Optional. The function parameters and values in JSON object format. See [FunctionDeclaration.parameters] for parameter details.
	Args json.RawMessage `json:"args,omitempty"`
	// Required. The name of the function to call. Matches [FunctionDeclaration.name].
	Name *string `json:"name,omitempty"`
}

// A function response.
type FunctionResponse struct {
	// Optional. The id of the function call this response is for. Populated by the client to match the corresponding function call `id`.
	Id *string `json:"id,omitempty"`
	// Required. The name of the function to call. Matches [FunctionDeclaration.name] and [FunctionCall.name].
	Name *string `json:"name,omitempty"`
	// Required. The function response in JSON object format. Use "output" key to specify function output and "error" key to specify error details (if any). If "output" and "error" keys are not specified, then whole "response" is treated as function output.
	Response map[string]interface{} `json:"response,omitempty"`
}

// Optional model configuration parameters.
//
// For more information, see `Content generation parameters
// <https://cloud.google.com/vertex-ai/generative-ai/docs/multimodal/content-generation-parameters>`_.
type GenerateContentConfig struct {
	// Value that controls the degree of randomness in token selection.
	// Lower temperatures are good for prompts that require a less open-ended or
	// creative response, while higher temperatures can lead to more diverse or
	// creative results.
	Temperature *float64 `json:"temperature,omitempty"`
	// Tokens are selected from the most to least probable until the sum
	// of their probabilities equals this value. Use a lower value for less
	// random responses and a higher value for more random responses.
	TopP *float64 `json:"topP,omitempty"`
	// For each token selection step, the ``top_k`` tokens with the
	// highest probabilities are sampled. Then tokens are further filtered based
	// on ``top_p`` with the final token selected using temperature sampling. Use
	// a lower number for less random responses and a higher number for more
	// random responses.
	TopK *float64 `json:"topK,omitempty"`
	// Number of response variations to return.
	CandidateCount *int `json:"candidateCount,omitempty"`
	// Maximum number of tokens that can be generated in the response.
	MaxOutputTokens *uint32 `json:"maxOutputTokens,omitempty"`
	// List of strings that tells the model to stop generating text if one
	// of the strings is encountered in the response.
	StopSequences []string `json:"stopSequences,omitempty"`
	// Positive values penalize tokens that already appear in the
	// generated text, increasing the probability of generating more diverse
	// content.
	PresencePenalty *float64 `json:"presencePenalty,omitempty"`
	// Positive values penalize tokens that repeatedly appear in the
	// generated text, increasing the probability of generating more diverse
	// content.
	FrequencyPenalty *float64 `json:"frequencyPenalty,omitempty"`
	// When ``seed`` is fixed to a specific number, the model makes a best
	// effort to provide the same response for repeated requests. By default, a
	// random number is used.
	Seed *int64 `json:"seed,omitempty"`
	// Output response mimetype of the generated candidate text.
	// Supported mimetype:
	// - `text/plain`: (default) Text output.
	// - `application/json`: JSON response in the candidates.
	// The model needs to be prompted to output the appropriate response type,
	// otherwise the behavior is undefined.
	// This is a preview feature.
	ResponseMimeType *string `json:"responseMimeType,omitempty"`
	// Optional. Output schema of the generated response. This is an alternative to `response_schema` that accepts [JSON Schema](https://json-schema.org/). If set, `response_schema` must be omitted, but `response_mime_type` is required. While the full JSON Schema may be sent, not all features are supported. Specifically, only the following properties are supported: - `$id` - `$defs` - `$ref` - `$anchor` - `type` - `format` - `title` - `description` - `enum` (for strings and numbers) - `items` - `prefixItems` - `minItems` - `maxItems` - `minimum` - `maximum` - `anyOf` - `oneOf` (interpreted the same as `anyOf`) - `properties` - `additionalProperties` - `required` The non-standard `propertyOrdering` property may also be set. Cyclic references are unrolled to a limited degree and, as such, may only be used within non-required properties. (Nullable properties are not sufficient.) If `$ref` is set on a sub-schema, no other properties, except for than those starting as a `$`, may be set.
	ResponseJsonSchema any `json:"responseJsonSchema,omitempty"`
	// The requested modalities of the response. Represents the set of
	// modalities that the model can return.
	ResponseModalities []string `json:"responseModalities,omitempty"`
	// The speech generation configuration.
	SpeechConfig *SpeechConfig `json:"speechConfig,omitempty"`
	// If enabled, audio timestamp will be included in the request to the
	// model.
	AudioTimestamp *bool `json:"audioTimestamp,omitempty"`
	// The thinking features configuration.
	ThinkingConfig *ThinkingConfig `json:"thinkingConfig,omitempty"`
}

// Tool details of a tool that the model may use to generate a response.
type Tool struct {
	// List of function declarations that the tool supports.
	FunctionDeclarations []FunctionDeclaration `json:"functionDeclarations,omitempty"`
}

// Defines a function that the model can generate JSON inputs for.
//
// The inputs are based on `OpenAPI 3.0 specifications
// <https://spec.openapis.org/oas/v3.0.3>`_.
type FunctionDeclaration struct {
	// Optional. Description and purpose of the function. Model uses it to decide how and whether to call the function.
	Description *string `json:"description,omitempty"`
	// Required. The name of the function to call. Must start with a letter or an underscore. Must be a-z, A-Z, 0-9, or contain underscores, dots and dashes, with a maximum length of 64.
	Name *string `json:"name,omitempty"`
	// Optional. Describes the parameters to this function in JSON Schema Object format. Reflects the Open API 3.03 Parameter Object. string Key: the name of the parameter. Parameter names are case sensitive. Schema Value: the Schema defining the type used for the parameter. For function with no parameters, this can be left unset. Parameter names must start with a letter or an underscore and must only contain chars a-z, A-Z, 0-9, or underscores with a maximum length of 64. Example with 1 required and 1 optional parameter: type: OBJECT properties: param1: type: STRING param2: type: INTEGER required: - param1
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	// Optional. Describes the parameters to the function in JSON Schema format. The schema must describe an object where the properties are the parameters to the function. For example:
	// ```
	// { "type": "object", "properties": { "name": { "type": "string" }, "age": { "type": "integer" } }, "additionalProperties": false, "required": ["name", "age"], "propertyOrdering": ["name", "age"] }
	// ```
	// This field is mutually exclusive with `parameters`.
	ParametersJsonSchema any `json:"parametersJsonSchema,omitempty"`
	// Optional. Describes the output from this function in JSON Schema format. Reflects the Open API 3.03 Response Object. The Schema defines the type used for the response value of the function.
	Response map[string]interface{} `json:"response,omitempty"`
	// Optional. Describes the output from this function in JSON Schema format. The value specified by the schema is the response value of the function. This field is mutually exclusive with `response`.
	ResponseJsonSchema any `json:"responseJsonSchema,omitempty"`
}

// Tool config.
//
// This config is shared for all tools provided in the request.
type ToolConfig struct {
	// Optional. Function calling config.
	FunctionCallingConfig *FunctionCallingConfig `json:"functionCallingConfig,omitempty"`
}

// Function calling config.
type FunctionCallingConfig struct {
	// Optional. Function calling mode.
	Mode *FunctionCallingConfigMode `json:"mode,omitempty"`
	// Optional. Function names to call. Only set when the Mode is ANY. Function names should match [FunctionDeclaration.name]. With mode set to ANY, model will predict a function call from the set of function names provided.
	AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
}

// Config for the function calling config mode.
type FunctionCallingConfigMode string

const (
	// The function calling config mode is unspecified. Should not be used.
	FunctionCallingConfigModeUnspecified FunctionCallingConfigMode = "MODE_UNSPECIFIED"
	// Default model behavior, model decides to predict either function calls or natural language response.
	FunctionCallingConfigModeAuto FunctionCallingConfigMode = "AUTO"
	// Model is constrained to always predicting function calls only. If "allowed_function_names" are set, the predicted function calls will be limited to any one of "allowed_function_names", else the predicted function calls will be any one of the provided "function_declarations".
	FunctionCallingConfigModeAny FunctionCallingConfigMode = "ANY"
	// Model will not predict any function calls. Model behavior is same as when not passing any function declarations.
	FunctionCallingConfigModeNone FunctionCallingConfigMode = "NONE"
	// Model decides to predict either a function call or a natural language response, but will validate function calls with constrained decoding. If "allowed_function_names" are set, the predicted function call will be limited to any one of "allowed_function_names", else the predicted function call will be any one of the provided "function_declarations".
	FunctionCallingConfigModeValidated FunctionCallingConfigMode = "VALIDATED"
)

// The speech generation configuration.
type SpeechConfig struct {
	// The configuration for the speaker to use.
	VoiceConfig *VoiceConfig `json:"voiceConfig,omitempty"`
	// The configuration for the multi-speaker setup.
	// It is mutually exclusive with the voice_config field.
	MultiSpeakerVoiceConfig *MultiSpeakerVoiceConfig `json:"multiSpeakerVoiceConfig,omitempty"`
	// Language code (ISO 639. e.g. en-US) for the speech synthesization.
	// Only available for Live API.
	LanguageCode *string `json:"languageCode,omitempty"`
}

// The configuration for the voice to use.
type VoiceConfig struct {
	// The configuration for the speaker to use.
	PrebuiltVoiceConfig *PrebuiltVoiceConfig `json:"prebuiltVoiceConfig,omitempty"`
}

// The configuration for the prebuilt speaker to use.
type PrebuiltVoiceConfig struct {
	// The name of the prebuilt voice to use.
	VoiceName *string `json:"voiceName,omitempty"`
}

// The configuration for the multi-speaker setup.
type MultiSpeakerVoiceConfig struct {
	// The configuration for the speaker to use.
	SpeakerVoiceConfigs []SpeakerVoiceConfig `json:"speakerVoiceConfigs,omitempty"`
}

// The configuration for the speaker to use.
type SpeakerVoiceConfig struct {
	// The name of the speaker to use. Should be the same as in the
	// prompt.
	Speaker *string `json:"speaker,omitempty"`
	// The configuration for the voice to use.
	VoiceConfig *VoiceConfig `json:"voiceConfig,omitempty"`
}

// The thinking features configuration.
type ThinkingConfig struct {
	// Indicates whether to include thoughts in the response. If true, thoughts are returned only if the model supports thought and thoughts are available.
	IncludeThoughts *bool `json:"includeThoughts,omitempty"`
	// Indicates the thinking budget in tokens. 0 is DISABLED. -1 is AUTOMATIC. The default values and allowed ranges are model dependent.
	ThinkingBudget *int `json:"thinkingBudget,omitempty"`
}

// Response message for PredictionService.GenerateContent.
type GenerateContentResponse struct {
	// Response variations returned by the model.
	Candidates []Candidate `json:"candidates,omitempty"`
	// Timestamp when the request is made to the server.
	CreateTime *string `json:"createTime,omitempty"`
	// Output only. The model version used to generate the response.
	ModelVersion *string `json:"modelVersion,omitempty"`
	// Output only. response_id is used to identify each response. It is the encoding of the event_id.
	ResponseId *string `json:"responseId,omitempty"`
	// Usage metadata about the response(s).
	UsageMetadata *GenerateContentResponseUsageMetadata `json:"usageMetadata,omitempty"`
}

// A response candidate generated from the model.
type Candidate struct {
	// Contains the multi-part content of the response.
	Content *Content `json:"content,omitempty"`
	// Source attribution of the generated content.
	CitationMetadata *CitationMetadata `json:"citationMetadata,omitempty"`
	// Describes the reason the model stopped generating tokens.
	FinishMessage *string `json:"finishMessage,omitempty"`
	// Number of tokens for this candidate.
	TokenCount *int `json:"tokenCount,omitempty"`
	// The reason why the model stopped generating tokens.
	// If empty, the model has not stopped generating the tokens.
	FinishReason *FinishReason `json:"finishReason,omitempty"`
	// Output only. Average log probability score of the candidate.
	AvgLogprobs *float64 `json:"avgLogprobs,omitempty"`
	// Output only. Index of the candidate.
	Index *int `json:"index,omitempty"`
}

// Citation information when the model quotes another source.
type CitationMetadata struct {
	// Contains citation information when the model directly quotes, at
	// length, from another source. Can include traditional websites and code
	// repositories.
	Citations []Citation `json:"citations,omitempty"`
}

// Source attributions for content.
type Citation struct {
	// Output only. End index into the content.
	EndIndex *int `json:"endIndex,omitempty"`
	// Output only. License of the attribution.
	License *string `json:"license,omitempty"`
	// Output only. Publication date of the attribution.
	PublicationDate *GoogleTypeDate `json:"publicationDate,omitempty"`
	// Output only. Start index into the content.
	StartIndex *int `json:"startIndex,omitempty"`
	// Output only. Title of the attribution.
	Title *string `json:"title,omitempty"`
	// Output only. Url reference of the attribution.
	Uri *string `json:"uri,omitempty"`
}

// Represents a whole or partial calendar date, such as a birthday. The time of day and time zone are either specified elsewhere or are insignificant. The date is relative to the Gregorian Calendar. This can represent one of the following: * A full date, with non-zero year, month, and day values. * A month and day, with a zero year (for example, an anniversary). * A year on its own, with a zero month and a zero day. * A year and month, with a zero day (for example, a credit card expiration date). Related types: * google.type.TimeOfDay * google.type.DateTime * google.protobuf.Timestamp
type GoogleTypeDate struct {
	// Day of a month. Must be from 1 to 31 and valid for the year and month, or 0 to specify a year by itself or a year and month where the day isn't significant.
	Day *int `json:"day,omitempty"`
	// Month of a year. Must be from 1 to 12, or 0 to specify a year without a month and day.
	Month *int `json:"month,omitempty"`
	// Year of the date. Must be from 1 to 9999, or 0 to specify a date without a year.
	Year *int `json:"year,omitempty"`
}

// Output only. The reason why the model stopped generating tokens.
//
// If empty, the model has not stopped generating the tokens.
type FinishReason string

const (
	// The finish reason is unspecified.
	FinishReasonUnspecified FinishReason = "FINISH_REASON_UNSPECIFIED"
	// Token generation reached a natural stopping point or a configured stop sequence.
	FinishReasonStop FinishReason = "STOP"
	// Token generation reached the configured maximum output tokens.
	FinishReasonMaxTokens FinishReason = "MAX_TOKENS"
	// Token generation stopped because the content potentially contains safety violations. NOTE: When streaming, [content][] is empty if content filters blocks the output.
	FinishReasonSafety FinishReason = "SAFETY"
	// The token generation stopped because of potential recitation.
	FinishReasonRecitation FinishReason = "RECITATION"
	// The token generation stopped because of using an unsupported language.
	FinishReasonLanguage FinishReason = "LANGUAGE"
	// All other reasons that stopped the token generation.
	FinishReasonOther FinishReason = "OTHER"
	// Token generation stopped because the content contains forbidden terms.
	FinishReasonBlocklist FinishReason = "BLOCKLIST"
	// Token generation stopped for potentially containing prohibited content.
	FinishReasonProhibitedContent FinishReason = "PROHIBITED_CONTENT"
	// Token generation stopped because the content potentially contains Sensitive Personally Identifiable Information (SPII).
	FinishReasonSPII FinishReason = "SPII"
	// The function call generated by the model is invalid.
	FinishReasonMalformedFunctionCall FinishReason = "MALFORMED_FUNCTION_CALL"
	// Token generation stopped because generated images have safety violations.
	FinishReasonImageSafety FinishReason = "IMAGE_SAFETY"
	// The tool call generated by the model is invalid.
	FinishReasonUnexpectedToolCall FinishReason = "UNEXPECTED_TOOL_CALL"
)

// Usage metadata about response(s).
type GenerateContentResponseUsageMetadata struct {
	// Output only. List of modalities of the cached content in the request input.
	CacheTokensDetails []ModalityTokenCount `json:"cacheTokensDetails,omitempty"`
	// Output only. Number of tokens in the cached part in the input (the cached content).
	CachedContentTokenCount *int `json:"cachedContentTokenCount,omitempty"`
	// Number of tokens in the response(s).
	CandidatesTokenCount *int `json:"candidatesTokenCount,omitempty"`
	// Output only. List of modalities that were returned in the response.
	CandidatesTokensDetails []ModalityTokenCount `json:"candidatesTokensDetails,omitempty"`
	// Number of tokens in the request. When `cached_content` is set, this is still the total effective prompt size meaning this includes the number of tokens in the cached content.
	PromptTokenCount *int `json:"promptTokenCount,omitempty"`
	// Output only. List of modalities that were processed in the request input.
	PromptTokensDetails []ModalityTokenCount `json:"promptTokensDetails,omitempty"`
	// Output only. Number of tokens present in thoughts output.
	ThoughtsTokenCount *int `json:"thoughtsTokenCount,omitempty"`
	// Output only. Number of tokens present in tool-use prompt(s).
	ToolUsePromptTokenCount *int `json:"toolUsePromptTokenCount,omitempty"`
	// Output only. List of modalities that were processed for tool-use request inputs.
	ToolUsePromptTokensDetails []ModalityTokenCount `json:"toolUsePromptTokensDetails,omitempty"`
	// Total token count for prompt, response candidates, and tool-use prompts (if present).
	TotalTokenCount *int `json:"totalTokenCount,omitempty"`
}

// Represents token counting info for a single modality.
type ModalityTokenCount struct {
	// The modality associated with this token count.
	Modality *MediaModality `json:"modality,omitempty"`
	// Number of tokens.
	TokenCount *int `json:"tokenCount,omitempty"`
}

// Server content modalities.
type MediaModality string

const (
	// The modality is unspecified.
	MediaModalityUnspecified MediaModality = "MODALITY_UNSPECIFIED"
	// Plain text.
	MediaModalityText MediaModality = "TEXT"
	// Images.
	MediaModalityImage MediaModality = "IMAGE"
	// Video.
	MediaModalityVideo MediaModality = "VIDEO"
	// Audio.
	MediaModalityAudio MediaModality = "AUDIO"
	// Document, e.g. PDF.
	MediaModalityDocument MediaModality = "DOCUMENT"
)
