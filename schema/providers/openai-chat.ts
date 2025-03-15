// https://platform.openai.com/docs/api-reference/chat

export interface ChatCompletionCreateParams {
  /**
   * A list of messages comprising the conversation so far. Depending on the
   * [model](https://platform.openai.com/docs/models) you use, different message
   * types (modalities) are supported, like
   * [text](https://platform.openai.com/docs/guides/text-generation),
   * [images](https://platform.openai.com/docs/guides/vision), and
   * [audio](https://platform.openai.com/docs/guides/audio).
   */
  messages: ChatCompletionMessageParam[];

  /**
   * Model ID used to generate the response, like `gpt-4o` or `o3`. OpenAI offers a
   * wide range of models with different capabilities, performance characteristics,
   * and price points. Refer to the
   * [model guide](https://platform.openai.com/docs/models) to browse and compare
   * available models.
   */
  model: string;

  /**
   * Parameters for audio output. Required when audio output is requested with
   * `modalities: ["audio"]`.
   * [Learn more](https://platform.openai.com/docs/guides/audio).
   */
  audio?: ChatCompletionAudioParam | null;

  /**
   * Number between -2.0 and 2.0. Positive values penalize new tokens based on their
   * existing frequency in the text so far, decreasing the model's likelihood to
   * repeat the same line verbatim.
   */
  frequency_penalty?: number | null;

  /**
   * An upper bound for the number of tokens that can be generated for a completion,
   * including visible output tokens and
   * [reasoning tokens](https://platform.openai.com/docs/guides/reasoning).
   */
  max_completion_tokens?: number | null;

  /**
   * Output types that you would like the model to generate. Most models are capable
   * of generating text, which is the default:
   *
   * `["text"]`
   *
   * The `gpt-4o-audio-preview` model can also be used to
   * [generate audio](https://platform.openai.com/docs/guides/audio). To request that
   * this model generate both text and audio responses, you can use:
   *
   * `["text", "audio"]`
   */
  modalities?: ("text" | "audio")[] | null;

  /**
   * Number between -2.0 and 2.0. Positive values penalize new tokens based on
   * whether they appear in the text so far, increasing the model's likelihood to
   * talk about new topics.
   */
  presence_penalty?: number | null;

  /**
   * An object specifying the format that the model must output.
   *
   * Setting to `{ "type": "json_schema", "json_schema": {...} }` enables Structured
   * Outputs which ensures the model will match your supplied JSON schema. Learn more
   * in the
   * [Structured Outputs guide](https://platform.openai.com/docs/guides/structured-outputs).
   *
   * Setting to `{ "type": "json_object" }` enables the older JSON mode, which
   * ensures the message the model generates is valid JSON. Using `json_schema` is
   * preferred for models that support it.
   */
  response_format?:
    | ResponseFormatText
    | ResponseFormatJSONSchema
    | ResponseFormatJSONObject;

  /**
   * This feature is in Beta. If specified, our system will make a best effort to
   * sample deterministically, such that repeated requests with the same `seed` and
   * parameters should return the same result. Determinism is not guaranteed, and you
   * should refer to the `system_fingerprint` response parameter to monitor changes
   * in the backend.
   */
  seed?: number | null;

  /**
   * If set to true, the model response data will be streamed to the client as it is
   * generated using
   * [server-sent events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#Event_stream_format).
   * See the
   * [Streaming section below](https://platform.openai.com/docs/api-reference/chat/streaming)
   * for more information, along with the
   * [streaming responses](https://platform.openai.com/docs/guides/streaming-responses)
   * guide for more information on how to handle the streaming events.
   */
  stream?: boolean | null;

  /**
   * Options for streaming response. Only set this when you set `stream: true`.
   */
  stream_options?: ChatCompletionStreamOptions | null;

  /**
   * What sampling temperature to use, between 0 and 2. Higher values like 0.8 will
   * make the output more random, while lower values like 0.2 will make it more
   * focused and deterministic. We generally recommend altering this or `top_p` but
   * not both.
   */
  temperature?: number | null;

  /**
   * Controls which (if any) tool is called by the model. `none` means the model will
   * not call any tool and instead generates a message. `auto` means the model can
   * pick between generating a message or calling one or more tools. `required` means
   * the model must call one or more tools. Specifying a particular tool via
   * `{"type": "function", "function": {"name": "my_function"}}` forces the model to
   * call that tool.
   *
   * `none` is the default when no tools are present. `auto` is the default if tools
   * are present.
   */
  tool_choice?: ChatCompletionToolChoiceOption;

  /**
   * A list of tools the model may call. You can provide either
   * [custom tools](https://platform.openai.com/docs/guides/function-calling#custom-tools)
   * or [function tools](https://platform.openai.com/docs/guides/function-calling).
   */
  tools?: ChatCompletionTool[];

  /**
   * An alternative to sampling with temperature, called nucleus sampling, where the
   * model considers the results of the tokens with top_p probability mass. So 0.1
   * means only the tokens comprising the top 10% probability mass are considered.
   *
   * We generally recommend altering this or `temperature` but not both.
   */
  top_p?: number | null;
}

/**
 * Developer-provided instructions that the model should follow, regardless of
 * messages sent by the user. With o1 models and newer, `developer` messages
 * replace the previous `system` messages.
 */
export type ChatCompletionMessageParam =
  | ChatCompletionDeveloperMessageParam
  | ChatCompletionSystemMessageParam
  | ChatCompletionUserMessageParam
  | ChatCompletionAssistantMessageParam
  | ChatCompletionToolMessageParam;

/**
 * Developer-provided instructions that the model should follow, regardless of
 * messages sent by the user. With o1 models and newer, `developer` messages
 * replace the previous `system` messages.
 */
export interface ChatCompletionDeveloperMessageParam {
  /**
   * The contents of the developer message.
   */
  content: DeveloperContentPart[];

  /**
   * The role of the messages author, in this case `developer`.
   */
  role: "developer";

  /**
   * An optional name for the participant. Provides the model information to
   * differentiate between participants of the same role.
   */
  name?: string;
}

/**
 * A union of possible developer content part
 */
export type DeveloperContentPart = ChatCompletionContentPartText;

/**
 * Developer-provided instructions that the model should follow, regardless of
 * messages sent by the user. With o1 models and newer, use `developer` messages
 * for this purpose instead.
 */
export interface ChatCompletionSystemMessageParam {
  /**
   * The contents of the system message.
   */
  content: SystemContentPart[];

  /**
   * The role of the messages author, in this case `system`.
   */
  role: "system";

  /**
   * An optional name for the participant. Provides the model information to
   * differentiate between participants of the same role.
   */
  name?: string;
}

/**
 * A union of possible system content part
 */
export type SystemContentPart = ChatCompletionContentPartText;

/**
 * Messages sent by an end user, containing prompts or additional context
 * information.
 */
export interface ChatCompletionUserMessageParam {
  /**
   * The contents of the user message.
   */
  content: ChatCompletionContentPart[];

  /**
   * The role of the messages author, in this case `user`.
   */
  role: "user";

  /**
   * An optional name for the participant. Provides the model information to
   * differentiate between participants of the same role.
   */
  name?: string;
}

/**
 * Messages sent by the model in response to user messages.
 */
export interface ChatCompletionAssistantMessageParam {
  /**
   * The role of the messages author, in this case `assistant`.
   */
  role: "assistant";

  /**
   * Data about a previous audio response from the model.
   * [Learn more](https://platform.openai.com/docs/guides/audio).
   */
  audio?: ChatCompletionAssistantMessageParamAudio | null;

  /**
   * The contents of the assistant message. Required unless `tool_calls` or
   * `function_call` is specified.
   */
  content?: AssistantContentPart[] | null;

  /**
   * An optional name for the participant. Provides the model information to
   * differentiate between participants of the same role.
   */
  name?: string;

  /**
   * The refusal message by the assistant.
   */
  refusal?: string | null;

  /**
   * The tool calls generated by the model, such as function calls.
   */
  tool_calls?: ChatCompletionMessageToolCall[];
}

/**
 * A union of possible assistant content part
 */
export type AssistantContentPart =
  | ChatCompletionContentPartText
  | ChatCompletionContentPartRefusal;

export interface ChatCompletionToolMessageParam {
  /**
   * The contents of the tool message.
   */
  content: ChatCompletionContentPartText[];

  /**
   * The role of the messages author, in this case `tool`.
   */
  role: "tool";

  /**
   * Tool call that this message is responding to.
   */
  tool_call_id: string;
}

/**
 * Learn about
 * [text inputs](https://platform.openai.com/docs/guides/text-generation).
 */
export type ChatCompletionContentPart =
  | ChatCompletionContentPartText
  | ChatCompletionContentPartImage
  | ChatCompletionContentPartInputAudio
  | ChatCompletionContentPartFile;

/**
 * Learn about
 * [text inputs](https://platform.openai.com/docs/guides/text-generation).
 */
export interface ChatCompletionContentPartText {
  /**
   * The text content.
   */
  text: string;

  /**
   * The type of the content part.
   */
  type: "text";
}

export interface ChatCompletionContentPartImageImageURL {
  /**
   * Either a URL of the image or the base64 encoded image data.
   */
  url: string;

  /**
   * Specifies the detail level of the image. Learn more in the
   * [Vision guide](https://platform.openai.com/docs/guides/vision#low-or-high-fidelity-image-understanding).
   */
  detail?: "auto" | "low" | "high";
}

/**
 * Learn about [image inputs](https://platform.openai.com/docs/guides/vision).
 */
export interface ChatCompletionContentPartImage {
  image_url: ChatCompletionContentPartImageImageURL;

  /**
   * The type of the content part.
   */
  type: "image_url";
}

export interface ChatCompletionContentPartInputAudioInputAudio {
  /**
   * Base64 encoded audio data.
   */
  data: string;

  /**
   * The format of the encoded audio data. Currently supports "wav" and "mp3".
   */
  format: "wav" | "mp3";
}

/**
 * Learn about [audio inputs](https://platform.openai.com/docs/guides/audio).
 */
export interface ChatCompletionContentPartInputAudio {
  input_audio: ChatCompletionContentPartInputAudioInputAudio;

  /**
   * The type of the content part. Always `input_audio`.
   */
  type: "input_audio";
}

export interface ChatCompletionContentPartFileFile {
  /**
   * The base64 encoded file data, used when passing the file to the model as a
   * string.
   */
  file_data?: string;

  /**
   * The ID of an uploaded file to use as input.
   */
  file_id?: string;

  /**
   * The name of the file, used when passing the file to the model as a string.
   */
  filename?: string;
}

/**
 * Learn about [file inputs](https://platform.openai.com/docs/guides/text) for text
 * generation.
 */
export interface ChatCompletionContentPartFile {
  file: ChatCompletionContentPartFileFile;

  /**
   * The type of the content part. Always `file`.
   */
  type: "file";
}

export interface ChatCompletionContentPartRefusal {
  /**
   * The refusal message generated by the model.
   */
  refusal: string;

  /**
   * The type of the content part.
   */
  type: "refusal";
}

/**
 * Parameters for audio output. Required when audio output is requested with
 * `modalities: ["audio"]`.
 * [Learn more](https://platform.openai.com/docs/guides/audio).
 */
export interface ChatCompletionAudioParam {
  /**
   * Specifies the output audio format. Must be one of `wav`, `mp3`, `flac`, `opus`,
   * or `pcm16`.
   */
  format: "wav" | "aac" | "mp3" | "flac" | "opus" | "pcm16";

  /**
   * The voice the model uses to respond. Supported voices are `alloy`, `ash`,
   * `ballad`, `coral`, `echo`, `fable`, `nova`, `onyx`, `sage`, and `shimmer`.
   */
  voice:
    | (string & {})
    | "alloy"
    | "ash"
    | "ballad"
    | "coral"
    | "echo"
    | "sage"
    | "shimmer"
    | "verse";
}

/**
 * Controls which (if any) tool is called by the model. `none` means the model will
 * not call any tool and instead generates a message. `auto` means the model can
 * pick between generating a message or calling one or more tools. `required` means
 * the model must call one or more tools. Specifying a particular tool via
 * `{"type": "function", "function": {"name": "my_function"}}` forces the model to
 * call that tool.
 *
 * `none` is the default when no tools are present. `auto` is the default if tools
 * are present.
 */
export type ChatCompletionToolChoiceOption =
  | "none"
  | "auto"
  | "required"
  | ChatCompletionAllowedToolChoice
  | ChatCompletionNamedToolChoice;

/**
 * Constrains the tools available to the model to a pre-defined set.
 */
export interface ChatCompletionAllowedToolChoice {
  /**
   * Constrains the tools available to the model to a pre-defined set.
   */
  allowed_tools: ChatCompletionAllowedTools;

  /**
   * Allowed tool configuration type. Always `allowed_tools`.
   */
  type: "allowed_tools";
}

/**
 * Constrains the tools available to the model to a pre-defined set.
 */
export interface ChatCompletionAllowedTools {
  /**
   * Constrains the tools available to the model to a pre-defined set.
   *
   * `auto` allows the model to pick from among the allowed tools and generate a
   * message.
   *
   * `required` requires the model to call one or more of the allowed tools.
   */
  mode: "auto" | "required";

  /**
   * A list of tool definitions that the model should be allowed to call.
   *
   * For the Chat Completions API, the list of tool definitions might look like:
   *
   * ```json
   * [
   *   { "type": "function", "function": { "name": "get_weather" } },
   *   { "type": "function", "function": { "name": "get_time" } }
   * ]
   * ```
   */
  tools: Record<string, unknown>[];
}

/**
 * Specifies a tool the model should use. Use to force the model to call a specific
 * function.
 */
export interface ChatCompletionNamedToolChoice {
  function: ChatCompletionNamedToolChoiceFunction;

  /**
   * For function calling, the type is always `function`.
   */
  type: "function";
}

export interface ChatCompletionNamedToolChoiceFunction {
  /**
   * The name of the function to call.
   */
  name: string;
}

/**
 * A call to a function tool created by the model.
 */
export type ChatCompletionMessageToolCall =
  ChatCompletionMessageFunctionToolCall;

/**
 * A call to a function tool created by the model.
 */
export interface ChatCompletionMessageFunctionToolCall {
  /**
   * The ID of the tool call.
   */
  id: string;

  /**
   * The function that the model called.
   */
  function: ChatCompletionMessageFunctionToolCallFunction;

  /**
   * The type of the tool. Currently, only `function` is supported.
   */
  type: "function";
}

export interface ChatCompletionMessageFunctionToolCallFunction {
  /**
   * The arguments to call the function with, as generated by the model in JSON
   * format. Note that the model does not always generate valid JSON, and may
   * hallucinate parameters not defined by your function schema. Validate the
   * arguments in your code before calling your function.
   */
  arguments: string;

  /**
   * The name of the function to call.
   */
  name: string;
}

/**
 * A function tool that can be used to generate a response.
 */
export type ChatCompletionTool = ChatCompletionFunctionTool;

/**
 * A function tool that can be used to generate a response.
 */
export interface ChatCompletionFunctionTool {
  function: FunctionDefinition;

  /**
   * The type of the tool. Currently, only `function` is supported.
   */
  type: "function";
}

export interface FunctionDefinition {
  /**
   * The name of the function to be called. Must be a-z, A-Z, 0-9, or contain
   * underscores and dashes, with a maximum length of 64.
   */
  name: string;

  /**
   * A description of what the function does, used by the model to choose when and
   * how to call the function.
   */
  description?: string;

  /**
   * The parameters the functions accepts, described as a JSON Schema object. See the
   * [guide](https://platform.openai.com/docs/guides/function-calling) for examples,
   * and the
   * [JSON Schema reference](https://json-schema.org/understanding-json-schema/) for
   * documentation about the format.
   *
   * Omitting `parameters` defines a function with an empty parameter list.
   */
  parameters?: FunctionParameters;

  /**
   * Whether to enable strict schema adherence when generating the function call. If
   * set to true, the model will follow the exact schema defined in the `parameters`
   * field. Only a subset of JSON Schema is supported when `strict` is `true`. Learn
   * more about Structured Outputs in the
   * [function calling guide](https://platform.openai.com/docs/guides/function-calling).
   */
  strict?: boolean | null;
}

/**
 * The parameters the functions accepts, described as a JSON Schema object. See the
 * [guide](https://platform.openai.com/docs/guides/function-calling) for examples,
 * and the
 * [JSON Schema reference](https://json-schema.org/understanding-json-schema/) for
 * documentation about the format.
 *
 * Omitting `parameters` defines a function with an empty parameter list.
 */
export type FunctionParameters = Record<string, unknown>;

export interface ChatCompletionAssistantMessageParamAudio {
  /**
   * Unique identifier for a previous audio response from the model.
   */
  id: string;
}

/**
 * Default response format. Used to generate text responses.
 */
export interface ResponseFormatText {
  /**
   * The type of response format being defined. Always `text`.
   */
  type: "text";
}

/**
 * JSON Schema response format. Used to generate structured JSON responses. Learn
 * more about
 * [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs).
 */
export interface ResponseFormatJSONSchema {
  /**
   * Structured Outputs configuration options, including a JSON Schema.
   */
  json_schema: ResponseFormatJSONSchemaJSONSchema;

  /**
   * The type of response format being defined. Always `json_schema`.
   */
  type: "json_schema";
}

/**
 * Structured Outputs configuration options, including a JSON Schema.
 */
export interface ResponseFormatJSONSchemaJSONSchema {
  /**
   * The name of the response format. Must be a-z, A-Z, 0-9, or contain underscores
   * and dashes, with a maximum length of 64.
   */
  name: string;

  /**
   * A description of what the response format is for, used by the model to determine
   * how to respond in the format.
   */
  description?: string;

  /**
   * The schema for the response format, described as a JSON Schema object. Learn how
   * to build JSON schemas [here](https://json-schema.org/).
   */
  schema?: Record<string, unknown>;

  /**
   * Whether to enable strict schema adherence when generating the output. If set to
   * true, the model will always follow the exact schema defined in the `schema`
   * field. Only a subset of JSON Schema is supported when `strict` is `true`. To
   * learn more, read the
   * [Structured Outputs guide](https://platform.openai.com/docs/guides/structured-outputs).
   */
  strict?: boolean | null;
}

/**
 * JSON object response format. An older method of generating JSON responses. Using
 * `json_schema` is recommended for models that support it. Note that the model
 * will not generate JSON without a system or user message instructing it to do so.
 */
export interface ResponseFormatJSONObject {
  /**
   * The type of response format being defined. Always `json_object`.
   */
  type: "json_object";
}

/**
 * Options for streaming response. Only set this when you set `stream: true`.
 */
export interface ChatCompletionStreamOptions {
  /**
   * If set, an additional chunk will be streamed before the `data: [DONE]` message.
   * The `usage` field on this chunk shows the token usage statistics for the entire
   * request, and the `choices` field will always be an empty array.
   *
   * All other chunks will also include a `usage` field, but with a null value.
   * **NOTE:** If the stream is interrupted, you may not receive the final usage
   * chunk which contains the total token usage for the request.
   */
  include_usage?: boolean;
}

/**
 * Represents a chat completion response returned by model, based on the provided
 * input.
 */
export interface ChatCompletion {
  /**
   * A unique identifier for the chat completion.
   */
  id: string;

  /**
   * A list of chat completion choices. Can be more than one if `n` is greater
   * than 1.
   */
  choices: ChatCompletionChoice[];

  /**
   * The Unix timestamp (in seconds) of when the chat completion was created.
   */
  created: number;

  /**
   * The model used for the chat completion.
   */
  model: string;

  /**
   * The object type, which is always `chat.completion`.
   */
  object: "chat.completion";

  /**
   * Usage statistics for the completion request.
   */
  usage?: CompletionsAPICompletionUsage;
}

export interface ChatCompletionChoice {
  /**
   * The reason the model stopped generating tokens. This will be `stop` if the model
   * hit a natural stop point or a provided stop sequence, `length` if the maximum
   * number of tokens specified in the request was reached, `content_filter` if
   * content was omitted due to a flag from our content filters, `tool_calls` if the
   * model called a tool, or `function_call` (deprecated) if the model called a
   * function.
   */
  finish_reason:
    | "stop"
    | "length"
    | "tool_calls"
    | "content_filter"
    | "function_call";

  /**
   * The index of the choice in the list of choices.
   */
  index: number;

  /**
   * A chat completion message generated by the model.
   */
  message: CompletionsCompletionsAPIChatCompletionMessage;
}

/**
 * Usage statistics for the completion request.
 */
export interface CompletionsAPICompletionUsage {
  /**
   * Number of tokens in the generated completion.
   */
  completion_tokens: number;

  /**
   * Number of tokens in the prompt.
   */
  prompt_tokens: number;

  /**
   * Total number of tokens used in the request (prompt + completion).
   */
  total_tokens: number;

  /**
   * Breakdown of tokens used in a completion.
   */
  completion_tokens_details?: CompletionUsageCompletionTokensDetails;

  /**
   * Breakdown of tokens used in the prompt.
   */
  prompt_tokens_details?: CompletionUsagePromptTokensDetails;
}
/**
 * Breakdown of tokens used in a completion.
 */
export interface CompletionUsageCompletionTokensDetails {
  /**
   * When using Predicted Outputs, the number of tokens in the prediction that
   * appeared in the completion.
   */
  accepted_prediction_tokens?: number;

  /**
   * Audio input tokens generated by the model.
   */
  audio_tokens?: number;

  /**
   * Tokens generated by the model for reasoning.
   */
  reasoning_tokens?: number;

  /**
   * When using Predicted Outputs, the number of tokens in the prediction that did
   * not appear in the completion. However, like reasoning tokens, these tokens are
   * still counted in the total completion tokens for purposes of billing, output,
   * and context window limits.
   */
  rejected_prediction_tokens?: number;
}

/**
 * Breakdown of tokens used in the prompt.
 */
export interface CompletionUsagePromptTokensDetails {
  /**
   * Audio input tokens present in the prompt.
   */
  audio_tokens?: number;

  /**
   * Cached tokens present in the prompt.
   */
  cached_tokens?: number;
}

/**
 * A chat completion message generated by the model.
 */
export interface CompletionsCompletionsAPIChatCompletionMessage {
  /**
   * The contents of the message.
   */
  content: string | null;

  /**
   * The refusal message generated by the model.
   */
  refusal: string | null;

  /**
   * The role of the author of this message.
   */
  role: "assistant";

  /**
   * If the audio output modality is requested, this object contains data about the
   * audio response from the model.
   * [Learn more](https://platform.openai.com/docs/guides/audio).
   */
  audio?: ChatCompletionAudio | null;

  /**
   * The tool calls generated by the model, such as function calls.
   */
  tool_calls?: ChatCompletionMessageToolCall[];
}

/**
 * If the audio output modality is requested, this object contains data about the
 * audio response from the model.
 * [Learn more](https://platform.openai.com/docs/guides/audio).
 */
export interface ChatCompletionAudio {
  /**
   * Unique identifier for this audio response.
   */
  id: string;

  /**
   * Base64 encoded audio bytes generated by the model, in the format specified in
   * the request.
   */
  data: string;

  /**
   * The Unix timestamp (in seconds) for when this audio response will no longer be
   * accessible on the server for use in multi-turn conversations.
   */
  expires_at: number;

  /**
   * Transcript of the audio generated by the model.
   */
  transcript: string;
}

/**
 * Represents a streamed chunk of a chat completion response returned by the model,
 * based on the provided input.
 * [Learn more](https://platform.openai.com/docs/guides/streaming-responses).
 */
export interface ChatCompletionChunk {
  /**
   * A unique identifier for the chat completion. Each chunk has the same ID.
   */
  id: string;

  /**
   * A list of chat completion choices. Can contain more than one elements if `n` is
   * greater than 1. Can also be empty for the last chunk if you set
   * `stream_options: {"include_usage": true}`.
   */
  choices: ChatCompletionChunkChoice[];

  /**
   * The Unix timestamp (in seconds) of when the chat completion was created. Each
   * chunk has the same timestamp.
   */
  created: number;

  /**
   * The model to generate the completion.
   */
  model: string;

  /**
   * The object type, which is always `chat.completion.chunk`.
   */
  object: "chat.completion.chunk";

  /**
   * An optional field that will only be present when you set
   * `stream_options: {"include_usage": true}` in your request. When present, it
   * contains a null value **except for the last chunk** which contains the token
   * usage statistics for the entire request.
   *
   * **NOTE:** If the stream is interrupted or cancelled, you may not receive the
   * final usage chunk which contains the total token usage for the request.
   */
  usage?: CompletionsAPICompletionUsage | null;
}

export interface ChatCompletionChunkChoice {
  /**
   * A chat completion delta generated by streamed model responses.
   */
  delta: ChatCompletionChunkChoiceDelta;

  /**
   * The reason the model stopped generating tokens. This will be `stop` if the model
   * hit a natural stop point or a provided stop sequence, `length` if the maximum
   * number of tokens specified in the request was reached, `content_filter` if
   * content was omitted due to a flag from our content filters, `tool_calls` if the
   * model called a tool, or `function_call` (deprecated) if the model called a
   * function.
   */
  finish_reason:
    | "stop"
    | "length"
    | "tool_calls"
    | "content_filter"
    | "function_call"
    | null;

  /**
   * The index of the choice in the list of choices.
   */
  index: number;
}

/**
 * A chat completion delta generated by streamed model responses.
 */
export interface ChatCompletionChunkChoiceDelta {
  /**
   * The contents of the chunk message.
   */
  content?: string | null;

  /**
   * The refusal message generated by the model.
   */
  refusal?: string | null;

  /**
   * The role of the author of this message.
   */
  role?: "developer" | "system" | "user" | "assistant" | "tool";

  tool_calls?: ChatCompletionChunkChoiceDeltaToolCall[];
}

export interface ChatCompletionChunkChoiceDeltaToolCall {
  index: number;

  /**
   * The ID of the tool call.
   */
  id?: string;

  function?: ChatCompletionChunkChoiceDeltaToolCallFunction;

  /**
   * The type of the tool. Currently, only `function` is supported.
   */
  type?: "function";
}

export interface ChatCompletionChunkChoiceDeltaToolCallFunction {
  /**
   * The arguments to call the function with, as generated by the model in JSON
   * format. Note that the model does not always generate valid JSON, and may
   * hallucinate parameters not defined by your function schema. Validate the
   * arguments in your code before calling your function.
   */
  arguments?: string;

  /**
   * The name of the function to call.
   */
  name?: string;
}
