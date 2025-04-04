/** Config for models.generate_content parameters. */
export declare interface GenerateContentParameters {
  /** ID of the model to use. For a list of models, see `Google models
     <https://cloud.google.com/vertex-ai/generative-ai/docs/learn/models>`_. */
  model: string;
  /** Content of the request.
   */
  contents: Content[];
  /** Configuration that contains optional model parameters.
   */
  /** Code that enables the system to interact with external systems to
     perform an action outside of the knowledge and scope of the model.
     */
  tools?: Tool[];
  /** Associates model output to a specific function call.
   */
  toolConfig?: ToolConfig;
  /** Instructions for the model to steer it toward better performance.
     For example, "Answer as concisely as possible" or "Don't use technical
     terms in your response".
     */
  systemInstruction?: Content;
  generationConfig?: GenerateContentConfig;
}

/** Contains the multi-part content of a message. */
export declare interface Content {
  /** List of parts that constitute a single message. Each part may have
     a different IANA MIME type. */
  parts?: Part[];
  /** Optional. The producer of the content. Must be either 'user' or
     'model'. Useful to set for multi-turn conversations, otherwise can be
     empty. If role is not specified, SDK will determine the role. */
  role?: string;
}

/** A datatype containing media content.

 Exactly one field within a Part should be set, representing the specific type
 of content being conveyed. Using multiple fields within the same `Part`
 instance is considered invalid.
 */
export declare interface Part {
  /** Indicates if the part is thought from the model. */
  thought?: boolean;
  /** Optional. Inlined bytes data. */
  inlineData?: Blob_2;
  /** Optional. URI based data. */
  fileData?: FileData;
  /** An opaque signature for the thought so it can be reused in subsequent requests.
   * @remarks Encoded as base64 string. */
  thoughtSignature?: string;
  /** Optional. A predicted [FunctionCall] returned from the model that contains a string representing the [FunctionDeclaration.name] with the parameters and their values. */
  functionCall?: FunctionCall;
  /** Optional. The result output of a [FunctionCall] that contains a string representing the [FunctionDeclaration.name] and a structured JSON object containing any output from the function call. It is used as context to the model. */
  functionResponse?: FunctionResponse;
  /** Optional. Text part (can be code). */
  text?: string;
}

export interface Blob_2 {
  /** Optional. Display name of the blob. Used to provide a label or filename to distinguish blobs. This field is not currently used in the Gemini GenerateContent calls. */
  displayName?: string;
  /** Required. Raw bytes.
   * @remarks Encoded as base64 string. */
  data?: string;
  /** Required. The IANA standard MIME type of the source data. */
  mimeType?: string;
}

export interface FileData {
  /** Optional. Display name of the file data. Used to provide a label or filename to distinguish file datas. It is not currently used in the Gemini GenerateContent calls. */
  displayName?: string;
  /** Required. URI. */
  fileUri?: string;
  /** Required. The IANA standard MIME type of the source data. */
  mimeType?: string;
}

/** A function call. */
export declare interface FunctionCall {
  /** The unique id of the function call. If populated, the client to execute the
     `function_call` and return the response with the matching `id`. */
  id?: string;
  /** Optional. The function parameters and values in JSON object format. See [FunctionDeclaration.parameters] for parameter details. */
  args?: Record<string, unknown>;
  /** Required. The name of the function to call. Matches [FunctionDeclaration.name]. */
  name?: string;
}

/** A function response. */
export declare class FunctionResponse {
  /** Optional. The id of the function call this response is for. Populated by the client to match the corresponding function call `id`. */
  id?: string;
  /** Required. The name of the function to call. Matches [FunctionDeclaration.name] and [FunctionCall.name]. */
  name?: string;
  /** Required. The function response in JSON object format. Use "output" key to specify function output and "error" key to specify error details (if any). If "output" and "error" keys are not specified, then whole "response" is treated as function output. */
  response?: Record<string, unknown>;
}

/** Optional model configuration parameters.

 For more information, see `Content generation parameters
 <https://cloud.google.com/vertex-ai/generative-ai/docs/multimodal/content-generation-parameters>`_.
 */
export declare interface GenerateContentConfig {
  /** Value that controls the degree of randomness in token selection.
     Lower temperatures are good for prompts that require a less open-ended or
     creative response, while higher temperatures can lead to more diverse or
     creative results.
     */
  temperature?: number;
  /** Tokens are selected from the most to least probable until the sum
     of their probabilities equals this value. Use a lower value for less
     random responses and a higher value for more random responses.
     */
  topP?: number;
  /** For each token selection step, the ``top_k`` tokens with the
     highest probabilities are sampled. Then tokens are further filtered based
     on ``top_p`` with the final token selected using temperature sampling. Use
     a lower number for less random responses and a higher number for more
     random responses. Must be a non-negative integer.
     */
  topK?: number;
  /** Number of response variations to return.
   */
  candidateCount?: number;
  /** Maximum number of tokens that can be generated in the response.
   */
  maxOutputTokens?: number;
  /** List of strings that tells the model to stop generating text if one
     of the strings is encountered in the response.
     */
  stopSequences?: string[];
  /** Positive values penalize tokens that already appear in the
     generated text, increasing the probability of generating more diverse
     content.
     */
  presencePenalty?: number;
  /** Positive values penalize tokens that repeatedly appear in the
     generated text, increasing the probability of generating more diverse
     content.
     */
  frequencyPenalty?: number;
  /** When ``seed`` is fixed to a specific number, the model makes a best
     effort to provide the same response for repeated requests. By default, a
     random number is used.
     */
  seed?: number;
  /** Output response mimetype of the generated candidate text.
     Supported mimetype:
     - `text/plain`: (default) Text output.
     - `application/json`: JSON response in the candidates.
     The model needs to be prompted to output the appropriate response type,
     otherwise the behavior is undefined.
     This is a preview feature.
     */
  responseMimeType?: string;
  /** Optional. Output schema of the generated response. This is an alternative to `response_schema` that accepts [JSON Schema](https://json-schema.org/). If set, `response_schema` must be omitted, but `response_mime_type` is required. While the full JSON Schema may be sent, not all features are supported. Specifically, only the following properties are supported: - `$id` - `$defs` - `$ref` - `$anchor` - `type` - `format` - `title` - `description` - `enum` (for strings and numbers) - `items` - `prefixItems` - `minItems` - `maxItems` - `minimum` - `maximum` - `anyOf` - `oneOf` (interpreted the same as `anyOf`) - `properties` - `additionalProperties` - `required` The non-standard `propertyOrdering` property may also be set. Cyclic references are unrolled to a limited degree and, as such, may only be used within non-required properties. (Nullable properties are not sufficient.) If `$ref` is set on a sub-schema, no other properties, except for than those starting as a `$`, may be set. */
  responseJsonSchema?: unknown;
  /** The requested modalities of the response. Represents the set of
     modalities that the model can return.
     */
  responseModalities?: string[];
  /** The speech generation configuration.
   */
  speechConfig?: SpeechConfig;
  /** If enabled, audio timestamp will be included in the request to the
     model.
     */
  audioTimestamp?: boolean;
  /** The thinking features configuration.
   */
  thinkingConfig?: ThinkingConfig;
}

/** Tool details of a tool that the model may use to generate a response. */
export interface Tool {
  /** List of function declarations that the tool supports. */
  functionDeclarations?: FunctionDeclaration[];
}

/** Defines a function that the model can generate JSON inputs for.

 The inputs are based on `OpenAPI 3.0 specifications
 <https://spec.openapis.org/oas/v3.0.3>`_.
 */
export interface FunctionDeclaration {
  /** Optional. Description and purpose of the function. Model uses it to decide how and whether to call the function. */
  description?: string;
  /** Required. The name of the function to call. Must start with a letter or an underscore. Must be a-z, A-Z, 0-9, or contain underscores, dots and dashes, with a maximum length of 64. */
  name?: string;
  /** Optional. Describes the parameters to this function in JSON Schema Object format. Reflects the Open API 3.03 Parameter Object. string Key: the name of the parameter. Parameter names are case sensitive. Schema Value: the Schema defining the type used for the parameter. For function with no parameters, this can be left unset. Parameter names must start with a letter or an underscore and must only contain chars a-z, A-Z, 0-9, or underscores with a maximum length of 64. Example with 1 required and 1 optional parameter: type: OBJECT properties: param1: type: STRING param2: type: INTEGER required: - param1 */
  parameters?: Record<string, unknown>;
  /** Optional. Describes the parameters to the function in JSON Schema format. The schema must describe an object where the properties are the parameters to the function. For example: ``` { "type": "object", "properties": { "name": { "type": "string" }, "age": { "type": "integer" } }, "additionalProperties": false, "required": ["name", "age"], "propertyOrdering": ["name", "age"] } ``` This field is mutually exclusive with `parameters`. */
  parametersJsonSchema?: unknown;
  /** Optional. Describes the output from this function in JSON Schema format. Reflects the Open API 3.03 Response Object. The Schema defines the type used for the response value of the function. */
  response?: Record<string, unknown>;
  /** Optional. Describes the output from this function in JSON Schema format. The value specified by the schema is the response value of the function. This field is mutually exclusive with `response`. */
  responseJsonSchema?: unknown;
}

/** Tool config.

 This config is shared for all tools provided in the request.
 */
export interface ToolConfig {
  /** Optional. Function calling config. */
  functionCallingConfig?: FunctionCallingConfig;
}

/** Function calling config. */
export interface FunctionCallingConfig {
  /** Optional. Function calling mode. */
  mode?: FunctionCallingConfigMode;
  /** Optional. Function names to call. Only set when the Mode is ANY. Function names should match [FunctionDeclaration.name]. With mode set to ANY, model will predict a function call from the set of function names provided. */
  allowedFunctionNames?: string[];
}

/** Config for the function calling config mode. */
export enum FunctionCallingConfigMode {
  /**
   * The function calling config mode is unspecified. Should not be used.
   */
  MODE_UNSPECIFIED = "MODE_UNSPECIFIED",
  /**
   * Default model behavior, model decides to predict either function calls or natural language response.
   */
  AUTO = "AUTO",
  /**
   * Model is constrained to always predicting function calls only. If "allowed_function_names" are set, the predicted function calls will be limited to any one of "allowed_function_names", else the predicted function calls will be any one of the provided "function_declarations".
   */
  ANY = "ANY",
  /**
   * Model will not predict any function calls. Model behavior is same as when not passing any function declarations.
   */
  NONE = "NONE",
  /**
   * Model decides to predict either a function call or a natural language response, but will validate function calls with constrained decoding. If "allowed_function_names" are set, the predicted function call will be limited to any one of "allowed_function_names", else the predicted function call will be any one of the provided "function_declarations".
   */
  VALIDATED = "VALIDATED",
}

/** The speech generation configuration. */
export declare interface SpeechConfig {
  /** The configuration for the speaker to use.
   */
  voiceConfig?: VoiceConfig;
  /** The configuration for the multi-speaker setup.
     It is mutually exclusive with the voice_config field.
     */
  multiSpeakerVoiceConfig?: MultiSpeakerVoiceConfig;
  /** Language code (ISO 639. e.g. en-US) for the speech synthesization.
     Only available for Live API.
     */
  languageCode?: string;
}

/** The configuration for the voice to use. */
export declare interface VoiceConfig {
  /** The configuration for the speaker to use.
   */
  prebuiltVoiceConfig?: PrebuiltVoiceConfig;
}

/** The configuration for the prebuilt speaker to use. */
export declare interface PrebuiltVoiceConfig {
  /** The name of the prebuilt voice to use. */
  voiceName?: string;
}

/** The configuration for the multi-speaker setup. */
export declare interface MultiSpeakerVoiceConfig {
  /** The configuration for the speaker to use. */
  speakerVoiceConfigs?: SpeakerVoiceConfig[];
}

/** The configuration for the speaker to use. */
export declare interface SpeakerVoiceConfig {
  /** The name of the speaker to use. Should be the same as in the
     prompt. */
  speaker?: string;
  /** The configuration for the voice to use. */
  voiceConfig?: VoiceConfig;
}

/** The thinking features configuration. */
export declare interface ThinkingConfig {
  /** Indicates whether to include thoughts in the response. If true, thoughts are returned only if the model supports thought and thoughts are available.
   */
  includeThoughts?: boolean;
  /** Indicates the thinking budget in tokens. 0 is DISABLED. -1 is AUTOMATIC. The default values and allowed ranges are model dependent.
   */
  thinkingBudget?: number;
}

/** Response message for PredictionService.GenerateContent. */
export declare class GenerateContentResponse {
  /** Response variations returned by the model.
   */
  candidates?: Candidate[];
  /** Timestamp when the request is made to the server.
   */
  createTime?: string;
  /** Output only. The model version used to generate the response. */
  modelVersion?: string;
  /** Output only. response_id is used to identify each response. It is the encoding of the event_id. */
  responseId?: string;
  /** Usage metadata about the response(s). */
  usageMetadata?: GenerateContentResponseUsageMetadata;
}

/** A response candidate generated from the model. */
export declare interface Candidate {
  /** Contains the multi-part content of the response.
   */
  content?: Content;
  /** Source attribution of the generated content.
   */
  citationMetadata?: CitationMetadata;
  /** Describes the reason the model stopped generating tokens.
   */
  finishMessage?: string;
  /** Number of tokens for this candidate.
   */
  tokenCount?: number;
  /** The reason why the model stopped generating tokens.
     If empty, the model has not stopped generating the tokens.
     */
  finishReason?: FinishReason;
  /** Output only. Average log probability score of the candidate. */
  avgLogprobs?: number;
  /** Output only. Index of the candidate. */
  index?: number;
}

/** Citation information when the model quotes another source. */
export declare interface CitationMetadata {
  /** Contains citation information when the model directly quotes, at
     length, from another source. Can include traditional websites and code
     repositories.
     */
  citations?: Citation[];
}

/** Source attributions for content. */
export declare interface Citation {
  /** Output only. End index into the content. */
  endIndex?: number;
  /** Output only. License of the attribution. */
  license?: string;
  /** Output only. Publication date of the attribution. */
  publicationDate?: GoogleTypeDate;
  /** Output only. Start index into the content. */
  startIndex?: number;
  /** Output only. Title of the attribution. */
  title?: string;
  /** Output only. Url reference of the attribution. */
  uri?: string;
}

/** Represents a whole or partial calendar date, such as a birthday. The time of day and time zone are either specified elsewhere or are insignificant. The date is relative to the Gregorian Calendar. This can represent one of the following: * A full date, with non-zero year, month, and day values. * A month and day, with a zero year (for example, an anniversary). * A year on its own, with a zero month and a zero day. * A year and month, with a zero day (for example, a credit card expiration date). Related types: * google.type.TimeOfDay * google.type.DateTime * google.protobuf.Timestamp */
export declare interface GoogleTypeDate {
  /** Day of a month. Must be from 1 to 31 and valid for the year and month, or 0 to specify a year by itself or a year and month where the day isn't significant. */
  day?: number;
  /** Month of a year. Must be from 1 to 12, or 0 to specify a year without a month and day. */
  month?: number;
  /** Year of the date. Must be from 1 to 9999, or 0 to specify a date without a year. */
  year?: number;
}

/** Output only. The reason why the model stopped generating tokens.

 If empty, the model has not stopped generating the tokens.
 */
export declare enum FinishReason {
  /**
   * The finish reason is unspecified.
   */
  FINISH_REASON_UNSPECIFIED = "FINISH_REASON_UNSPECIFIED",
  /**
   * Token generation reached a natural stopping point or a configured stop sequence.
   */
  STOP = "STOP",
  /**
   * Token generation reached the configured maximum output tokens.
   */
  MAX_TOKENS = "MAX_TOKENS",
  /**
   * Token generation stopped because the content potentially contains safety violations. NOTE: When streaming, [content][] is empty if content filters blocks the output.
   */
  SAFETY = "SAFETY",
  /**
   * The token generation stopped because of potential recitation.
   */
  RECITATION = "RECITATION",
  /**
   * The token generation stopped because of using an unsupported language.
   */
  LANGUAGE = "LANGUAGE",
  /**
   * All other reasons that stopped the token generation.
   */
  OTHER = "OTHER",
  /**
   * Token generation stopped because the content contains forbidden terms.
   */
  BLOCKLIST = "BLOCKLIST",
  /**
   * Token generation stopped for potentially containing prohibited content.
   */
  PROHIBITED_CONTENT = "PROHIBITED_CONTENT",
  /**
   * Token generation stopped because the content potentially contains Sensitive Personally Identifiable Information (SPII).
   */
  SPII = "SPII",
  /**
   * The function call generated by the model is invalid.
   */
  MALFORMED_FUNCTION_CALL = "MALFORMED_FUNCTION_CALL",
  /**
   * Token generation stopped because generated images have safety violations.
   */
  IMAGE_SAFETY = "IMAGE_SAFETY",
  /**
   * The tool call generated by the model is invalid.
   */
  UNEXPECTED_TOOL_CALL = "UNEXPECTED_TOOL_CALL",
}

/** Usage metadata about response(s). */
export declare class GenerateContentResponseUsageMetadata {
  /** Output only. List of modalities of the cached content in the request input. */
  cacheTokensDetails?: ModalityTokenCount[];
  /** Output only. Number of tokens in the cached part in the input (the cached content). */
  cachedContentTokenCount?: number;
  /** Number of tokens in the response(s). */
  candidatesTokenCount?: number;
  /** Output only. List of modalities that were returned in the response. */
  candidatesTokensDetails?: ModalityTokenCount[];
  /** Number of tokens in the request. When `cached_content` is set, this is still the total effective prompt size meaning this includes the number of tokens in the cached content. */
  promptTokenCount?: number;
  /** Output only. List of modalities that were processed in the request input. */
  promptTokensDetails?: ModalityTokenCount[];
  /** Output only. Number of tokens present in thoughts output. */
  thoughtsTokenCount?: number;
  /** Output only. Number of tokens present in tool-use prompt(s). */
  toolUsePromptTokenCount?: number;
  /** Output only. List of modalities that were processed for tool-use request inputs. */
  toolUsePromptTokensDetails?: ModalityTokenCount[];
  /** Total token count for prompt, response candidates, and tool-use prompts (if present). */
  totalTokenCount?: number;
}

/** Represents token counting info for a single modality. */
export declare interface ModalityTokenCount {
  /** The modality associated with this token count. */
  modality?: MediaModality;
  /** Number of tokens. */
  tokenCount?: number;
}

/** Server content modalities. */
export declare enum MediaModality {
  /**
   * The modality is unspecified.
   */
  MODALITY_UNSPECIFIED = "MODALITY_UNSPECIFIED",
  /**
   * Plain text.
   */
  TEXT = "TEXT",
  /**
   * Images.
   */
  IMAGE = "IMAGE",
  /**
   * Video.
   */
  VIDEO = "VIDEO",
  /**
   * Audio.
   */
  AUDIO = "AUDIO",
  /**
   * Document, e.g. PDF.
   */
  DOCUMENT = "DOCUMENT",
}
