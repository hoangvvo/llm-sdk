// https://platform.openai.com/docs/api-reference/responses/create

export interface ResponseCreateParams {
  /**
   * Specify additional output data to include in the model response. Currently
   * supported values are:
   *
   * - `web_search_call.action.sources`: Include the sources of the web search tool
   *   call.
   * - `code_interpreter_call.outputs`: Includes the outputs of python code execution
   *   in code interpreter tool call items.
   * - `computer_call_output.output.image_url`: Include image urls from the computer
   *   call output.
   * - `file_search_call.results`: Include the search results of the file search tool
   *   call.
   * - `message.input_image.image_url`: Include image urls from the input message.
   * - `computer_call_output.output.image_url`: Include image urls from the computer
   *   call output.
   * - `reasoning.encrypted_content`: Includes an encrypted version of reasoning
   *   tokens in reasoning item outputs. This enables reasoning items to be used in
   *   multi-turn conversations when using the Responses API statelessly (like when
   *   the `store` parameter is set to `false`, or when an organization is enrolled
   *   in the zero data retention program).
   * - `code_interpreter_call.outputs`: Includes the outputs of python code execution
   *   in code interpreter tool call items.
   */
  include?: ResponseIncludable[] | null;

  /**
   * Text, image, or file inputs to the model, used to generate a response.
   *
   * Learn more:
   *
   * - [Text inputs and outputs](https://platform.openai.com/docs/guides/text)
   * - [Image inputs](https://platform.openai.com/docs/guides/images)
   * - [File inputs](https://platform.openai.com/docs/guides/pdf-files)
   * - [Conversation state](https://platform.openai.com/docs/guides/conversation-state)
   * - [Function calling](https://platform.openai.com/docs/guides/function-calling)
   */
  input?: ResponseInputItem[];

  /**
   * A system (or developer) message inserted into the model's context.
   *
   * When using along with `previous_response_id`, the instructions from a previous
   * response will not be carried over to the next response. This makes it simple to
   * swap out system (or developer) messages in new responses.
   */
  instructions?: string | null;

  /**
   * An upper bound for the number of tokens that can be generated for a response,
   * including visible output tokens and
   * [reasoning tokens](https://platform.openai.com/docs/guides/reasoning).
   */
  max_output_tokens?: number | null;

  /**
   * Model ID used to generate the response, like `gpt-4o` or `o3`. OpenAI offers a
   * wide range of models with different capabilities, performance characteristics,
   * and price points. Refer to the
   * [model guide](https://platform.openai.com/docs/models) to browse and compare
   * available models.
   */
  model?: string;

  /**
   * Whether to allow the model to run tool calls in parallel.
   */
  parallel_tool_calls?: boolean | null;

  /**
   * **gpt-5 and o-series models only**
   *
   * Configuration options for
   * [reasoning models](https://platform.openai.com/docs/guides/reasoning).
   */
  reasoning?: Reasoning | null;

  /**
   * Whether to store the generated model response for later retrieval via API.
   */
  store?: boolean | null;

  /**
   * If set to true, the model response data will be streamed to the client as it is
   * generated using
   * [server-sent events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#Event_stream_format).
   * See the
   * [Streaming section below](https://platform.openai.com/docs/api-reference/responses-streaming)
   * for more information.
   */
  stream?: boolean | null;

  /**
   * Options for streaming responses. Only set this when you set `stream: true`.
   */
  stream_options?: ResponseCreateParamsStreamOptions | null;

  /**
   * What sampling temperature to use, between 0 and 2. Higher values like 0.8 will
   * make the output more random, while lower values like 0.2 will make it more
   * focused and deterministic. We generally recommend altering this or `top_p` but
   * not both.
   */
  temperature?: number | null;

  /**
   * Configuration options for a text response from the model. Can be plain text or
   * structured JSON data. Learn more:
   *
   * - [Text inputs and outputs](https://platform.openai.com/docs/guides/text)
   * - [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs)
   */
  text?: ResponseTextConfig;

  /**
   * How the model should select which tool (or tools) to use when generating a
   * response. See the `tools` parameter to see how to specify which tools the model
   * can call.
   */
  tool_choice?:
    | ToolChoiceOptions
    | ToolChoiceAllowed
    | ToolChoiceTypes
    | ToolChoiceFunction
    | ToolChoiceCustom;

  /**
   * An array of tools the model may call while generating a response. You can
   * specify which tool to use by setting the `tool_choice` parameter.
   *
   * We support the following categories of tools:
   *
   * - **Built-in tools**: Tools that are provided by OpenAI that extend the model's
   *   capabilities, like
   *   [web search](https://platform.openai.com/docs/guides/tools-web-search) or
   *   [file search](https://platform.openai.com/docs/guides/tools-file-search).
   *   Learn more about
   *   [built-in tools](https://platform.openai.com/docs/guides/tools).
   * - **MCP Tools**: Integrations with third-party systems via custom MCP servers or
   *   predefined connectors such as Google Drive and SharePoint. Learn more about
   *   [MCP Tools](https://platform.openai.com/docs/guides/tools-connectors-mcp).
   * - **Function calls (custom tools)**: Functions that are defined by you, enabling
   *   the model to call your own code with strongly typed arguments and outputs.
   *   Learn more about
   *   [function calling](https://platform.openai.com/docs/guides/function-calling).
   *   You can also use custom tools to call your own code.
   */
  tools?: Tool[];

  /**
   * An alternative to sampling with temperature, called nucleus sampling, where the
   * model considers the results of the tokens with top_p probability mass. So 0.1
   * means only the tokens comprising the top 10% probability mass are considered.
   *
   * We generally recommend altering this or `temperature` but not both.
   */
  top_p?: number | null;

  /**
   * The truncation strategy to use for the model response.
   *
   * - `auto`: If the context of this response and previous ones exceeds the model's
   *   context window size, the model will truncate the response to fit the context
   *   window by dropping input items in the middle of the conversation.
   * - `disabled` (default): If a model response will exceed the context window size
   *   for a model, the request will fail with a 400 error.
   */
  truncation?: "auto" | "disabled" | null;
}

/**
 * Specify additional output data to include in the model response. Currently
 * supported values are:
 *
 * - `web_search_call.action.sources`: Include the sources of the web search tool
 *   call.
 * - `code_interpreter_call.outputs`: Includes the outputs of python code execution
 *   in code interpreter tool call items.
 * - `computer_call_output.output.image_url`: Include image urls from the computer
 *   call output.
 * - `file_search_call.results`: Include the search results of the file search tool
 *   call.
 * - `message.input_image.image_url`: Include image urls from the input message.
 * - `computer_call_output.output.image_url`: Include image urls from the computer
 *   call output.
 * - `reasoning.encrypted_content`: Includes an encrypted version of reasoning
 *   tokens in reasoning item outputs. This enables reasoning items to be used in
 *   multi-turn conversations when using the Responses API statelessly (like when
 *   the `store` parameter is set to `false`, or when an organization is enrolled
 *   in the zero data retention program).
 * - `code_interpreter_call.outputs`: Includes the outputs of python code execution
 *   in code interpreter tool call items.
 */
export type ResponseIncludable =
  | "file_search_call.results"
  | "message.input_image.image_url"
  | "computer_call_output.output.image_url"
  | "reasoning.encrypted_content"
  | "code_interpreter_call.outputs";

/**
 * A message input to the model with a role indicating instruction following
 * hierarchy. Instructions given with the `developer` or `system` role take
 * precedence over instructions given with the `user` role. Messages with the
 * `assistant` role are presumed to have been generated by the model in previous
 * interactions.
 */
export type ResponseInputItem =
  | ResponseInputItemMessage
  | ResponseOutputMessage
  | ResponseFunctionToolCall
  | ResponseInputItemFunctionCallOutput
  | ResponseReasoningItem
  | ResponseOutputItemImageGenerationCall;

/**
 * A message input to the model with a role indicating instruction following
 * hierarchy. Instructions given with the `developer` or `system` role take
 * precedence over instructions given with the `user` role.
 */
export interface ResponseInputItemMessage {
  /**
   * A list of one or many input items to the model, containing different content
   * types.
   */
  content: ResponseInputMessageContentList;

  /**
   * The role of the message input. One of `user`, `system`, or `developer`.
   */
  role: "user" | "system" | "developer";

  /**
   * The status of item. One of `in_progress`, `completed`, or `incomplete`.
   * Populated when items are returned via API.
   */
  status?: "in_progress" | "completed" | "incomplete";

  /**
   * The type of the message input. Always set to `message`.
   */
  type?: "message";
}

/**
 * A list of one or many input items to the model, containing different content
 * types.
 */
export type ResponseInputMessageContentList = ResponseInputContent[];

/**
 * A text input to the model.
 */
export type ResponseInputContent =
  | ResponseInputText
  | ResponseInputImage
  | ResponseInputFile
  | ResponseInputAudio;

/**
 * A text input to the model.
 */
export interface ResponseInputText {
  /**
   * The text input to the model.
   */
  text: string;

  /**
   * The type of the input item. Always `input_text`.
   */
  type: "input_text";
}

/**
 * An image input to the model. Learn about
 * [image inputs](https://platform.openai.com/docs/guides/vision).
 */
export interface ResponseInputImage {
  /**
   * The detail level of the image to be sent to the model. One of `high`, `low`, or
   * `auto`. Defaults to `auto`.
   */
  detail: "low" | "high" | "auto";

  /**
   * The type of the input item. Always `input_image`.
   */
  type: "input_image";

  /**
   * The ID of the file to be sent to the model.
   */
  file_id?: string | null;

  /**
   * The URL of the image to be sent to the model. A fully qualified URL or base64
   * encoded image in a data URL.
   */
  image_url?: string | null;
}

/**
 * A file input to the model.
 */
export interface ResponseInputFile {
  /**
   * The type of the input item. Always `input_file`.
   */
  type: "input_file";

  /**
   * The content of the file to be sent to the model.
   */
  file_data?: string;

  /**
   * The ID of the file to be sent to the model.
   */
  file_id?: string | null;

  /**
   * The URL of the file to be sent to the model.
   */
  file_url?: string;

  /**
   * The name of the file to be sent to the model.
   */
  filename?: string;
}

/**
 * An audio input to the model.
 */
export interface ResponseInputAudio {
  input_audio: ResponseInputAudioInputAudio;

  /**
   * The type of the input item. Always `input_audio`.
   */
  type: "input_audio";
}

export interface ResponseInputAudioInputAudio {
  /**
   * Base64-encoded audio data.
   */
  data: string;

  /**
   * The format of the audio data. Currently supported formats are `mp3` and `wav`.
   */
  format: "mp3" | "wav";
}

export interface ResponseOutputMessage {
  /**
   * The unique ID of the output message.
   */
  id: string;

  /**
   * The content of the output message.
   */
  content: (ResponseOutputText | ResponseOutputRefusal)[];

  /**
   * The role of the output message. Always `assistant`.
   */
  role: "assistant";

  /**
   * The status of the message input. One of `in_progress`, `completed`, or
   * `incomplete`. Populated when input items are returned via API.
   */
  status: "in_progress" | "completed" | "incomplete";

  /**
   * The type of the output message. Always `message`.
   */
  type: "message";
}

/**
 * A text output from the model.
 */
export interface ResponseOutputText {
  /**
   * The annotations of the text output.
   */
  annotations: (
    | ResponseOutputTextFileCitation
    | ResponseOutputTextURLCitation
    | ResponseOutputTextFilePath
  )[];

  /**
   * The text output from the model.
   */
  text: string;

  /**
   * The type of the output text. Always `output_text`.
   */
  type: "output_text";
}

/**
 * A citation to a file.
 */
export interface ResponseOutputTextFileCitation {
  /**
   * The ID of the file.
   */
  file_id: string;

  /**
   * The filename of the file cited.
   */
  filename: string;

  /**
   * The index of the file in the list of files.
   */
  index: number;

  /**
   * The type of the file citation. Always `file_citation`.
   */
  type: "file_citation";
}

/**
 * A citation for a web resource used to generate a model response.
 */
export interface ResponseOutputTextURLCitation {
  /**
   * The index of the last character of the URL citation in the message.
   */
  end_index: number;

  /**
   * The index of the first character of the URL citation in the message.
   */
  start_index: number;

  /**
   * The title of the web resource.
   */
  title: string;

  /**
   * The type of the URL citation. Always `url_citation`.
   */
  type: "url_citation";

  /**
   * The URL of the web resource.
   */
  url: string;
}

/**
 * A path to a file.
 */
export interface ResponseOutputTextFilePath {
  /**
   * The ID of the file.
   */
  file_id: string;

  /**
   * The index of the file in the list of files.
   */
  index: number;

  /**
   * The type of the file path. Always `file_path`.
   */
  type: "file_path";
}

/**
 * A refusal from the model.
 */
export interface ResponseOutputRefusal {
  /**
   * The refusal explanation from the model.
   */
  refusal: string;

  /**
   * The type of the refusal. Always `refusal`.
   */
  type: "refusal";
}

/**
 * A tool call to run a function. See the
 * [function calling guide](https://platform.openai.com/docs/guides/function-calling)
 * for more information.
 */
export interface ResponseFunctionToolCall {
  /**
   * A JSON string of the arguments to pass to the function.
   */
  arguments: string;

  /**
   * The unique ID of the function tool call generated by the model.
   */
  call_id: string;

  /**
   * The name of the function to run.
   */
  name: string;

  /**
   * The type of the function tool call. Always `function_call`.
   */
  type: "function_call";

  /**
   * The unique ID of the function tool call.
   */
  id?: string;

  /**
   * The status of the item. One of `in_progress`, `completed`, or `incomplete`.
   * Populated when items are returned via API.
   */
  status?: "in_progress" | "completed" | "incomplete";
}

/**
 * The output of a function tool call.
 */
export interface ResponseInputItemFunctionCallOutput {
  /**
   * The unique ID of the function tool call generated by the model.
   */
  call_id: string;

  /**
   * A JSON string of the output of the function tool call.
   */
  output: string;

  /**
   * The type of the function tool call output. Always `function_call_output`.
   */
  type: "function_call_output";

  /**
   * The unique ID of the function tool call output. Populated when this item is
   * returned via API.
   */
  id?: string | null;

  /**
   * The status of the item. One of `in_progress`, `completed`, or `incomplete`.
   * Populated when items are returned via API.
   */
  status?: "in_progress" | "completed" | "incomplete" | null;
}

/**
 * A description of the chain of thought used by a reasoning model while generating
 * a response. Be sure to include these items in your `input` to the Responses API
 * for subsequent turns of a conversation if you are manually
 * [managing context](https://platform.openai.com/docs/guides/conversation-state).
 */
export interface ResponseReasoningItem {
  /**
   * The unique identifier of the reasoning content.
   */
  id: string;

  /**
   * Reasoning summary content.
   */
  summary: ResponseReasoningItemSummaryUnion[];

  /**
   * The type of the object. Always `reasoning`.
   */
  type: "reasoning";

  /**
   * Reasoning text content.
   */
  content?: ResponseReasoningItemContent[];

  /**
   * The encrypted content of the reasoning item - populated when a response is
   * generated with `reasoning.encrypted_content` in the `include` parameter.
   */
  encrypted_content?: string | null;

  /**
   * The status of the item. One of `in_progress`, `completed`, or `incomplete`.
   * Populated when items are returned via API.
   */
  status?: "in_progress" | "completed" | "incomplete";
}

export type ResponseReasoningItemSummaryUnion = ResponseReasoningItemSummary;

export interface ResponseReasoningItemSummary {
  /**
   * Summary text content.
   */
  text: string;

  /**
   * The type of the object. Always `summary_text`.
   */
  type: "summary_text";
}

export type ResponseReasoningItemContentUnion = ResponseReasoningItemContent;

export interface ResponseReasoningItemContent {
  /**
   * Reasoning text output from the model.
   */
  text: string;

  /**
   * The type of the object. Always `reasoning_text`.
   */
  type: "reasoning_text";
}

export interface ResponseOutputItemImageGenerationCall {
  /**
   * The unique ID of the image generation call.
   */
  id: string;

  /**
   * The generated image encoded in base64.
   */
  result: string | null;

  /**
   * The status of the image generation call.
   */
  status: "in_progress" | "completed" | "generating" | "failed";

  /**
   * The type of the image generation call. Always `image_generation_call`.
   */
  type: "image_generation_call";
}

/**
 * **o-series models only**
 *
 * Configuration options for
 * [reasoning models](https://platform.openai.com/docs/guides/reasoning).
 */
export interface Reasoning {
  /**
   * Constrains effort on reasoning for
   * [reasoning models](https://platform.openai.com/docs/guides/reasoning). Currently
   * supported values are `minimal`, `low`, `medium`, and `high`. Reducing reasoning
   * effort can result in faster responses and fewer tokens used on reasoning in a
   * response.
   */
  effort?: ReasoningEffort | null;

  /**
   * A summary of the reasoning performed by the model. This can be useful for
   * debugging and understanding the model's reasoning process. One of `auto`,
   * `concise`, or `detailed`.
   */
  summary?: "auto" | "concise" | "detailed" | null;
}

/**
 * Constrains effort on reasoning for
 * [reasoning models](https://platform.openai.com/docs/guides/reasoning). Currently
 * supported values are `minimal`, `low`, `medium`, and `high`. Reducing reasoning
 * effort can result in faster responses and fewer tokens used on reasoning in a
 * response.
 */
export type ReasoningEffort = "minimal" | "low" | "medium" | "high" | null;

/**
 * Options for streaming responses. Only set this when you set `stream: true`.
 */
export interface ResponseCreateParamsStreamOptions {
  /**
   * When true, stream obfuscation will be enabled. Stream obfuscation adds random
   * characters to an `obfuscation` field on streaming delta events to normalize
   * payload sizes as a mitigation to certain side-channel attacks. These obfuscation
   * fields are included by default, but add a small amount of overhead to the data
   * stream. You can set `include_obfuscation` to false to optimize for bandwidth if
   * you trust the network links between your application and the OpenAI API.
   */
  include_obfuscation?: boolean;
}

/**
 * Configuration options for a text response from the model. Can be plain text or
 * structured JSON data. Learn more:
 *
 * - [Text inputs and outputs](https://platform.openai.com/docs/guides/text)
 * - [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs)
 */
export interface ResponseTextConfig {
  /**
   * An object specifying the format that the model must output.
   *
   * Configuring `{ "type": "json_schema" }` enables Structured Outputs, which
   * ensures the model will match your supplied JSON schema. Learn more in the
   * [Structured Outputs guide](https://platform.openai.com/docs/guides/structured-outputs).
   *
   * The default format is `{ "type": "text" }` with no additional options.
   *
   * **Not recommended for gpt-4o and newer models:**
   *
   * Setting to `{ "type": "json_object" }` enables the older JSON mode, which
   * ensures the message the model generates is valid JSON. Using `json_schema` is
   * preferred for models that support it.
   */
  format?: ResponseFormatTextConfig;

  /**
   * Constrains the verbosity of the model's response. Lower values will result in
   * more concise responses, while higher values will result in more verbose
   * responses. Currently supported values are `low`, `medium`, and `high`.
   */
  verbosity?: "low" | "medium" | "high" | null;
}

export type ResponseFormatTextConfig =
  | ResponseFormatText
  | ResponseFormatTextJSONSchemaConfig
  | ResponseFormatJSONObject;

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
export interface ResponseFormatTextJSONSchemaConfig {
  /**
   * The name of the response format. Must be a-z, A-Z, 0-9, or contain underscores
   * and dashes, with a maximum length of 64.
   */
  name: string;

  /**
   * The schema for the response format, described as a JSON Schema object. Learn how
   * to build JSON schemas [here](https://json-schema.org/).
   */
  schema: Record<string, unknown>;

  /**
   * The type of response format being defined. Always `json_schema`.
   */
  type: "json_schema";

  /**
   * A description of what the response format is for, used by the model to determine
   * how to respond in the format.
   */
  description?: string;

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

export type ToolChoiceOptions = "none" | "auto" | "required";

/**
 * Constrains the tools available to the model to a pre-defined set.
 */
export interface ToolChoiceAllowed {
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
   * For the Responses API, the list of tool definitions might look like:
   *
   * ```json
   * [
   *   { "type": "function", "name": "get_weather" },
   *   { "type": "mcp", "server_label": "deepwiki" },
   *   { "type": "image_generation" }
   * ]
   * ```
   */
  tools: Record<string, unknown>[];

  /**
   * Allowed tool configuration type. Always `allowed_tools`.
   */
  type: "allowed_tools";
}

/**
 * Indicates that the model should use a built-in tool to generate a response.
 * [Learn more about built-in tools](https://platform.openai.com/docs/guides/tools).
 */
export interface ToolChoiceTypes {
  /**
   * The type of hosted tool the model should to use. Learn more about
   * [built-in tools](https://platform.openai.com/docs/guides/tools).
   *
   * Allowed values are:
   *
   * - `file_search`
   * - `web_search_preview`
   * - `computer_use_preview`
   * - `code_interpreter`
   * - `mcp`
   * - `image_generation`
   */
  type: "file_search" | "web_search_preview" | "image_generation";
}

/**
 * Use this option to force the model to call a specific function.
 */
export interface ToolChoiceFunction {
  /**
   * The name of the function to call.
   */
  name: string;

  /**
   * For function calling, the type is always `function`.
   */
  type: "function";
}

/**
 * Use this option to force the model to call a specific custom tool.
 */
export interface ToolChoiceCustom {
  /**
   * The name of the custom tool to call.
   */
  name: string;

  /**
   * For custom tool calling, the type is always `custom`.
   */
  type: "custom";
}

/**
 * A tool that can be used to generate a response.
 */
export type Tool = FunctionTool | WebSearchTool | ToolImageGeneration;

/**
 * Defines a function in your own code the model can choose to call. Learn more
 * about
 * [function calling](https://platform.openai.com/docs/guides/function-calling).
 */
export interface FunctionTool {
  /**
   * The name of the function to call.
   */
  name: string;

  /**
   * A JSON schema object describing the parameters of the function.
   */
  parameters: Record<string, unknown> | null;

  /**
   * Whether to enforce strict parameter validation. Default `true`.
   */
  strict: boolean | null;

  /**
   * The type of the function tool. Always `function`.
   */
  type: "function";

  /**
   * A description of the function. Used by the model to determine whether or not to
   * call the function.
   */
  description?: string | null;
}

/**
 * Search the Internet for sources related to the prompt. Learn more about the
 * [web search tool](https://platform.openai.com/docs/guides/tools-web-search).
 */
export interface WebSearchTool {
  /**
   * The type of the web search tool. One of `web_search` or `web_search_2025_08_26`.
   */
  type: "web_search" | "web_search_2025_08_26";

  /**
   * Filters for the search.
   */
  filters?: WebSearchToolFilters | null;

  /**
   * High level guidance for the amount of context window space to use for the
   * search. One of `low`, `medium`, or `high`. `medium` is the default.
   */
  search_context_size?: "low" | "medium" | "high";

  /**
   * The approximate location of the user.
   */
  user_location?: WebSearchToolUserLocation | null;
}

/**
 * Filters for the search.
 */
export interface WebSearchToolFilters {
  /**
   * Allowed domains for the search. If not provided, all domains are allowed.
   * Subdomains of the provided domains are allowed as well.
   *
   * Example: `["pubmed.ncbi.nlm.nih.gov"]`
   */
  allowed_domains?: string[] | null;
}

/**
 * The approximate location of the user.
 */
export interface WebSearchToolUserLocation {
  /**
   * Free text input for the city of the user, e.g. `San Francisco`.
   */
  city?: string | null;

  /**
   * The two-letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1) of
   * the user, e.g. `US`.
   */
  country?: string | null;

  /**
   * Free text input for the region of the user, e.g. `California`.
   */
  region?: string | null;

  /**
   * The [IANA timezone](https://timeapi.io/documentation/iana-timezones) of the
   * user, e.g. `America/Los_Angeles`.
   */
  timezone?: string | null;

  /**
   * The type of location approximation. Always `approximate`.
   */
  type?: "approximate";
}

/**
 * A tool that generates images using a model like `gpt-image-1`.
 */
export interface ToolImageGeneration {
  /**
   * The type of the image generation tool. Always `image_generation`.
   */
  type: "image_generation";

  /**
   * Background type for the generated image. One of `transparent`, `opaque`, or
   * `auto`. Default: `auto`.
   */
  background?: "transparent" | "opaque" | "auto";

  /**
   * Control how much effort the model will exert to match the style and features,
   * especially facial features, of input images. This parameter is only supported
   * for `gpt-image-1`. Supports `high` and `low`. Defaults to `low`.
   */
  input_fidelity?: "high" | "low" | null;

  /**
   * Optional mask for inpainting. Contains `image_url` (string, optional) and
   * `file_id` (string, optional).
   */
  input_image_mask?: ImageGenerationInputImageMask;

  /**
   * The image generation model to use. Default: `gpt-image-1`.
   */
  model?: "gpt-image-1";

  /**
   * Moderation level for the generated image. Default: `auto`.
   */
  moderation?: "auto" | "low";

  /**
   * Compression level for the output image. Default: 100.
   */
  output_compression?: number;

  /**
   * The output format of the generated image. One of `png`, `webp`, or `jpeg`.
   * Default: `png`.
   */
  output_format?: "png" | "webp" | "jpeg";

  /**
   * Number of partial images to generate in streaming mode, from 0 (default value)
   * to 3.
   */
  partial_images?: number;

  /**
   * The quality of the generated image. One of `low`, `medium`, `high`, or `auto`.
   * Default: `auto`.
   */
  quality?: "low" | "medium" | "high" | "auto";

  /**
   * The size of the generated image. One of `1024x1024`, `1024x1536`, `1536x1024`,
   * or `auto`. Default: `auto`.
   */
  size?: "1024x1024" | "1024x1536" | "1536x1024" | "auto";
}

export interface ImageGenerationInputImageMask {
  /**
   * File ID for the mask image.
   */
  file_id?: string;

  /**
   * Base64-encoded mask image.
   */
  image_url?: string;
}

export interface Response {
  /**
   * Unique identifier for this Response.
   */
  id: string;

  /**
   * Unix timestamp (in seconds) of when this Response was created.
   */
  created_at: number;

  /**
   * Model ID used to generate the response, like `gpt-4o` or `o3`. OpenAI offers a
   * wide range of models with different capabilities, performance characteristics,
   * and price points. Refer to the
   * [model guide](https://platform.openai.com/docs/models) to browse and compare
   * available models.
   */
  model: string;

  /**
   * The object type of this resource - always set to `response`.
   */
  object: "response";

  /**
   * An array of content items generated by the model.
   *
   * - The length and order of items in the `output` array is dependent on the
   *   model's response.
   * - Rather than accessing the first item in the `output` array and assuming it's
   *   an `assistant` message with the content generated by the model, you might
   *   consider using the `output_text` property where supported in SDKs.
   */
  output: ResponseOutputItem[];

  /**
   * The status of the response generation. One of `completed`, `failed`,
   * `in_progress`, `cancelled`, `queued`, or `incomplete`.
   */
  status?: ResponseStatus;

  /**
   * Represents token usage details including input tokens, output tokens, a
   * breakdown of output tokens, and the total tokens used.
   */
  usage?: ResponseUsage;
}

/**
 * An output message from the model.
 */
export type ResponseOutputItem =
  | ResponseOutputMessage
  | ResponseFunctionToolCall
  | ResponseFunctionWebSearch
  | ResponseReasoningItem
  | ResponseOutputItemImageGenerationCall;

/**
 * The results of a web search tool call. See the
 * [web search guide](https://platform.openai.com/docs/guides/tools-web-search) for
 * more information.
 */
export interface ResponseFunctionWebSearch {
  /**
   * The unique ID of the web search tool call.
   */
  id: string;

  /**
   * The status of the web search tool call.
   */
  status: "in_progress" | "searching" | "completed" | "failed";

  /**
   * The type of the web search tool call. Always `web_search_call`.
   */
  type: "web_search_call";
}

export type ResponseStatus =
  | "completed"
  | "failed"
  | "in_progress"
  | "cancelled"
  | "queued"
  | "incomplete";

/**
 * Represents token usage details including input tokens, output tokens, a
 * breakdown of output tokens, and the total tokens used.
 */
export interface ResponseUsage {
  /**
   * The number of input tokens.
   */
  input_tokens: number;

  /**
   * A detailed breakdown of the input tokens.
   */
  input_tokens_details: ResponseUsageInputTokensDetails;

  /**
   * The number of output tokens.
   */
  output_tokens: number;

  /**
   * A detailed breakdown of the output tokens.
   */
  output_tokens_details: ResponseUsageOutputTokensDetails;

  /**
   * The total number of tokens used.
   */
  total_tokens: number;
}

export interface ResponseUsageInputTokensDetails {
  /**
   * The number of tokens that were retrieved from the cache.
   * [More on prompt caching](https://platform.openai.com/docs/guides/prompt-caching).
   */
  cached_tokens: number;
}

export interface ResponseUsageOutputTokensDetails {
  /**
   * The number of reasoning tokens.
   */
  reasoning_tokens: number;
}

/**
 * Emitted when there is a partial audio response.
 */
export type ResponseStreamEvent =
  | ResponseAudioDeltaEvent
  | ResponseAudioDoneEvent
  | ResponseAudioTranscriptDeltaEvent
  | ResponseAudioTranscriptDoneEvent
  | ResponseCompletedEvent
  | ResponseContentPartAddedEvent
  | ResponseContentPartDoneEvent
  | ResponseCreatedEvent
  | ResponseErrorEvent
  | ResponseFunctionCallArgumentsDeltaEvent
  | ResponseFunctionCallArgumentsDoneEvent
  | ResponseInProgressEvent
  | ResponseFailedEvent
  | ResponseIncompleteEvent
  | ResponseOutputItemAddedEvent
  | ResponseOutputItemDoneEvent
  | ResponseReasoningSummaryPartAddedEvent
  | ResponseReasoningSummaryPartDoneEvent
  | ResponseReasoningSummaryTextDeltaEvent
  | ResponseReasoningSummaryTextDoneEvent
  | ResponseReasoningTextDeltaEvent
  | ResponseReasoningTextDoneEvent
  | ResponseRefusalDeltaEvent
  | ResponseRefusalDoneEvent
  | ResponseTextDeltaEvent
  | ResponseTextDoneEvent
  | ResponseImageGenCallCompletedEvent
  | ResponseImageGenCallGeneratingEvent
  | ResponseImageGenCallInProgressEvent
  | ResponseImageGenCallPartialImageEvent
  | ResponseOutputTextAnnotationAddedEvent;

/**
 * Emitted when there is a partial audio response.
 */
export interface ResponseAudioDeltaEvent {
  /**
   * A chunk of Base64 encoded response audio bytes.
   */
  delta: string;

  /**
   * A sequence number for this chunk of the stream response.
   */
  sequence_number: number;

  /**
   * The type of the event. Always `response.audio.delta`.
   */
  type: "response.audio.delta";
}

/**
 * Emitted when the audio response is complete.
 */
export interface ResponseAudioDoneEvent {
  /**
   * The sequence number of the delta.
   */
  sequence_number: number;

  /**
   * The type of the event. Always `response.audio.done`.
   */
  type: "response.audio.done";
}

/**
 * Emitted when there is a partial transcript of audio.
 */
export interface ResponseAudioTranscriptDeltaEvent {
  /**
   * The partial transcript of the audio response.
   */
  delta: string;

  /**
   * The sequence number of this event.
   */
  sequence_number: number;

  /**
   * The type of the event. Always `response.audio.transcript.delta`.
   */
  type: "response.audio.transcript.delta";
}

/**
 * Emitted when the full audio transcript is completed.
 */
export interface ResponseAudioTranscriptDoneEvent {
  /**
   * The sequence number of this event.
   */
  sequence_number: number;

  /**
   * The type of the event. Always `response.audio.transcript.done`.
   */
  type: "response.audio.transcript.done";
}

/**
 * Emitted when the model response is complete.
 */
export interface ResponseCompletedEvent {
  /**
   * Properties of the completed response.
   */
  response: Response;

  /**
   * The sequence number for this event.
   */
  sequence_number: number;

  /**
   * The type of the event. Always `response.completed`.
   */
  type: "response.completed";
}

/**
 * Emitted when a new content part is added.
 */
export interface ResponseContentPartAddedEvent {
  /**
   * The index of the content part that was added.
   */
  content_index: number;

  /**
   * The ID of the output item that the content part was added to.
   */
  item_id: string;

  /**
   * The index of the output item that the content part was added to.
   */
  output_index: number;

  /**
   * The content part that was added.
   */
  part: ResponseContentPartEventPart;

  /**
   * The sequence number of this event.
   */
  sequence_number: number;

  /**
   * The type of the event. Always `response.content_part.added`.
   */
  type: "response.content_part.added";
}

export type ResponseContentPartEventPart =
  | ResponseOutputText
  | ResponseOutputRefusal;

/**
 * Emitted when a content part is done.
 */
export interface ResponseContentPartDoneEvent {
  /**
   * The index of the content part that is done.
   */
  content_index: number;

  /**
   * The ID of the output item that the content part was added to.
   */
  item_id: string;

  /**
   * The index of the output item that the content part was added to.
   */
  output_index: number;

  /**
   * The content part that is done.
   */
  part: ResponseContentPartEventPart;

  /**
   * The sequence number of this event.
   */
  sequence_number: number;

  /**
   * The type of the event. Always `response.content_part.done`.
   */
  type: "response.content_part.done";
}

/**
 * An event that is emitted when a response is created.
 */
export interface ResponseCreatedEvent {
  /**
   * The response that was created.
   */
  response: Response;

  /**
   * The sequence number for this event.
   */
  sequence_number: number;

  /**
   * The type of the event. Always `response.created`.
   */
  type: "response.created";
}

/**
 * Emitted when an error occurs.
 */
export interface ResponseErrorEvent {
  /**
   * The error code.
   */
  code: string | null;

  /**
   * The error message.
   */
  message: string;

  /**
   * The error parameter.
   */
  param: string | null;

  /**
   * The sequence number of this event.
   */
  sequence_number: number;

  /**
   * The type of the event. Always `error`.
   */
  type: "error";
}

/**
 * Emitted when there is a partial function-call arguments delta.
 */
export interface ResponseFunctionCallArgumentsDeltaEvent {
  /**
   * The function-call arguments delta that is added.
   */
  delta: string;

  /**
   * The ID of the output item that the function-call arguments delta is added to.
   */
  item_id: string;

  /**
   * The index of the output item that the function-call arguments delta is added to.
   */
  output_index: number;

  /**
   * The sequence number of this event.
   */
  sequence_number: number;

  /**
   * The type of the event. Always `response.function_call_arguments.delta`.
   */
  type: "response.function_call_arguments.delta";
}

/**
 * Emitted when function-call arguments are finalized.
 */
export interface ResponseFunctionCallArgumentsDoneEvent {
  /**
   * The function-call arguments.
   */
  arguments: string;

  /**
   * The ID of the item.
   */
  item_id: string;

  /**
   * The index of the output item.
   */
  output_index: number;

  /**
   * The sequence number of this event.
   */
  sequence_number: number;

  type: "response.function_call_arguments.done";
}

/**
 * Emitted when the response is in progress.
 */
export interface ResponseInProgressEvent {
  /**
   * The response that is in progress.
   */
  response: Response;

  /**
   * The sequence number of this event.
   */
  sequence_number: number;

  /**
   * The type of the event. Always `response.in_progress`.
   */
  type: "response.in_progress";
}

/**
 * An event that is emitted when a response fails.
 */
export interface ResponseFailedEvent {
  /**
   * The response that failed.
   */
  response: Response;

  /**
   * The sequence number of this event.
   */
  sequence_number: number;

  /**
   * The type of the event. Always `response.failed`.
   */
  type: "response.failed";
}

/**
 * An event that is emitted when a response finishes as incomplete.
 */
export interface ResponseIncompleteEvent {
  /**
   * The response that was incomplete.
   */
  response: Response;

  /**
   * The sequence number of this event.
   */
  sequence_number: number;

  /**
   * The type of the event. Always `response.incomplete`.
   */
  type: "response.incomplete";
}

/**
 * Emitted when a new output item is added.
 */
export interface ResponseOutputItemAddedEvent {
  /**
   * The output item that was added.
   */
  item: ResponseOutputItem;

  /**
   * The index of the output item that was added.
   */
  output_index: number;

  /**
   * The sequence number of this event.
   */
  sequence_number: number;

  /**
   * The type of the event. Always `response.output_item.added`.
   */
  type: "response.output_item.added";
}

/**
 * Emitted when an output item is marked done.
 */
export interface ResponseOutputItemDoneEvent {
  /**
   * The output item that was marked done.
   */
  item: ResponseOutputItem;

  /**
   * The index of the output item that was marked done.
   */
  output_index: number;

  /**
   * The sequence number of this event.
   */
  sequence_number: number;

  /**
   * The type of the event. Always `response.output_item.done`.
   */
  type: "response.output_item.done";
}

/**
 * Emitted when a new reasoning summary part is added.
 */
export interface ResponseReasoningSummaryPartAddedEvent {
  /**
   * The ID of the item this summary part is associated with.
   */
  item_id: string;

  /**
   * The index of the output item this summary part is associated with.
   */
  output_index: number;

  /**
   * The summary part that was added.
   */
  part: ResponseReasoningSummaryPartAddedEventPart;

  /**
   * The sequence number of this event.
   */
  sequence_number: number;

  /**
   * The index of the summary part within the reasoning summary.
   */
  summary_index: number;

  /**
   * The type of the event. Always `response.reasoning_summary_part.added`.
   */
  type: "response.reasoning_summary_part.added";
}

/**
 * The summary part that was added.
 */
export interface ResponseReasoningSummaryPartAddedEventPart {
  /**
   * The text of the summary part.
   */
  text: string;

  /**
   * The type of the summary part. Always `summary_text`.
   */
  type: "summary_text";
}

/**
 * Emitted when a reasoning summary part is completed.
 */
export interface ResponseReasoningSummaryPartDoneEvent {
  /**
   * The ID of the item this summary part is associated with.
   */
  item_id: string;

  /**
   * The index of the output item this summary part is associated with.
   */
  output_index: number;

  /**
   * The completed summary part.
   */
  part: ResponseReasoningSummaryPartDoneEventPart;

  /**
   * The sequence number of this event.
   */
  sequence_number: number;

  /**
   * The index of the summary part within the reasoning summary.
   */
  summary_index: number;

  /**
   * The type of the event. Always `response.reasoning_summary_part.done`.
   */
  type: "response.reasoning_summary_part.done";
}

/**
 * The completed summary part.
 */
export interface ResponseReasoningSummaryPartDoneEventPart {
  /**
   * The text of the summary part.
   */
  text: string;

  /**
   * The type of the summary part. Always `summary_text`.
   */
  type: "summary_text";
}

/**
 * Emitted when a delta is added to a reasoning summary text.
 */
export interface ResponseReasoningSummaryTextDeltaEvent {
  /**
   * The text delta that was added to the summary.
   */
  delta: string;

  /**
   * The ID of the item this summary text delta is associated with.
   */
  item_id: string;

  /**
   * The index of the output item this summary text delta is associated with.
   */
  output_index: number;

  /**
   * The sequence number of this event.
   */
  sequence_number: number;

  /**
   * The index of the summary part within the reasoning summary.
   */
  summary_index: number;

  /**
   * The type of the event. Always `response.reasoning_summary_text.delta`.
   */
  type: "response.reasoning_summary_text.delta";
}

/**
 * Emitted when a reasoning summary text is completed.
 */
export interface ResponseReasoningSummaryTextDoneEvent {
  /**
   * The ID of the item this summary text is associated with.
   */
  item_id: string;

  /**
   * The index of the output item this summary text is associated with.
   */
  output_index: number;

  /**
   * The sequence number of this event.
   */
  sequence_number: number;

  /**
   * The index of the summary part within the reasoning summary.
   */
  summary_index: number;

  /**
   * The full text of the completed reasoning summary.
   */
  text: string;

  /**
   * The type of the event. Always `response.reasoning_summary_text.done`.
   */
  type: "response.reasoning_summary_text.done";
}

/**
 * Emitted when a delta is added to a reasoning text.
 */
export interface ResponseReasoningTextDeltaEvent {
  /**
   * The index of the reasoning content part this delta is associated with.
   */
  content_index: number;

  /**
   * The text delta that was added to the reasoning content.
   */
  delta: string;

  /**
   * The ID of the item this reasoning text delta is associated with.
   */
  item_id: string;

  /**
   * The index of the output item this reasoning text delta is associated with.
   */
  output_index: number;

  /**
   * The sequence number of this event.
   */
  sequence_number: number;

  /**
   * The type of the event. Always `response.reasoning_text.delta`.
   */
  type: "response.reasoning_text.delta";
}

/**
 * Emitted when a reasoning text is completed.
 */
export interface ResponseReasoningTextDoneEvent {
  /**
   * The index of the reasoning content part.
   */
  content_index: number;

  /**
   * The ID of the item this reasoning text is associated with.
   */
  item_id: string;

  /**
   * The index of the output item this reasoning text is associated with.
   */
  output_index: number;

  /**
   * The sequence number of this event.
   */
  sequence_number: number;

  /**
   * The full text of the completed reasoning content.
   */
  text: string;

  /**
   * The type of the event. Always `response.reasoning_text.done`.
   */
  type: "response.reasoning_text.done";
}

/**
 * Emitted when there is a partial refusal text.
 */
export interface ResponseRefusalDeltaEvent {
  /**
   * The index of the content part that the refusal text is added to.
   */
  content_index: number;

  /**
   * The refusal text that is added.
   */
  delta: string;

  /**
   * The ID of the output item that the refusal text is added to.
   */
  item_id: string;

  /**
   * The index of the output item that the refusal text is added to.
   */
  output_index: number;

  /**
   * The sequence number of this event.
   */
  sequence_number: number;

  /**
   * The type of the event. Always `response.refusal.delta`.
   */
  type: "response.refusal.delta";
}

/**
 * Emitted when refusal text is finalized.
 */
export interface ResponseRefusalDoneEvent {
  /**
   * The index of the content part that the refusal text is finalized.
   */
  content_index: number;

  /**
   * The ID of the output item that the refusal text is finalized.
   */
  item_id: string;

  /**
   * The index of the output item that the refusal text is finalized.
   */
  output_index: number;

  /**
   * The refusal text that is finalized.
   */
  refusal: string;

  /**
   * The sequence number of this event.
   */
  sequence_number: number;

  /**
   * The type of the event. Always `response.refusal.done`.
   */
  type: "response.refusal.done";
}

/**
 * Emitted when there is an additional text delta.
 */
export interface ResponseTextDeltaEvent {
  /**
   * The index of the content part that the text delta was added to.
   */
  content_index: number;

  /**
   * The text delta that was added.
   */
  delta: string;

  /**
   * The ID of the output item that the text delta was added to.
   */
  item_id: string;

  /**
   * The index of the output item that the text delta was added to.
   */
  output_index: number;

  /**
   * The sequence number for this event.
   */
  sequence_number: number;

  /**
   * The type of the event. Always `response.output_text.delta`.
   */
  type: "response.output_text.delta";
}

/**
 * Emitted when text content is finalized.
 */
export interface ResponseTextDoneEvent {
  /**
   * The index of the content part that the text content is finalized.
   */
  content_index: number;

  /**
   * The ID of the output item that the text content is finalized.
   */
  item_id: string;

  /**
   * The index of the output item that the text content is finalized.
   */
  output_index: number;

  /**
   * The sequence number for this event.
   */
  sequence_number: number;

  /**
   * The text content that is finalized.
   */
  text: string;

  /**
   * The type of the event. Always `response.output_text.done`.
   */
  type: "response.output_text.done";
}

/**
 * Emitted when an image generation tool call has completed and the final image is
 * available.
 */
export interface ResponseImageGenCallCompletedEvent {
  /**
   * The unique identifier of the image generation item being processed.
   */
  item_id: string;

  /**
   * The index of the output item in the response's output array.
   */
  output_index: number;

  /**
   * The sequence number of this event.
   */
  sequence_number: number;

  /**
   * The type of the event. Always 'response.image_generation_call.completed'.
   */
  type: "response.image_generation_call.completed";
}

/**
 * Emitted when an image generation tool call is actively generating an image
 * (intermediate state).
 */
export interface ResponseImageGenCallGeneratingEvent {
  /**
   * The unique identifier of the image generation item being processed.
   */
  item_id: string;

  /**
   * The index of the output item in the response's output array.
   */
  output_index: number;

  /**
   * The sequence number of the image generation item being processed.
   */
  sequence_number: number;

  /**
   * The type of the event. Always 'response.image_generation_call.generating'.
   */
  type: "response.image_generation_call.generating";
}

/**
 * Emitted when an image generation tool call is in progress.
 */
export interface ResponseImageGenCallInProgressEvent {
  /**
   * The unique identifier of the image generation item being processed.
   */
  item_id: string;

  /**
   * The index of the output item in the response's output array.
   */
  output_index: number;

  /**
   * The sequence number of the image generation item being processed.
   */
  sequence_number: number;

  /**
   * The type of the event. Always 'response.image_generation_call.in_progress'.
   */
  type: "response.image_generation_call.in_progress";
}

/**
 * Emitted when a partial image is available during image generation streaming.
 */
export interface ResponseImageGenCallPartialImageEvent {
  /**
   * The unique identifier of the image generation item being processed.
   */
  item_id: string;

  /**
   * The index of the output item in the response's output array.
   */
  output_index: number;

  /**
   * Base64-encoded partial image data, suitable for rendering as an image.
   */
  partial_image_b64: string;

  /**
   * 0-based index for the partial image (backend is 1-based, but this is 0-based for
   * the user).
   */
  partial_image_index: number;

  /**
   * The sequence number of the image generation item being processed.
   */
  sequence_number: number;

  /**
   * The type of the event. Always 'response.image_generation_call.partial_image'.
   */
  type: "response.image_generation_call.partial_image";
}

/**
 * Emitted when an annotation is added to output text content.
 */
export interface ResponseOutputTextAnnotationAddedEvent {
  /**
   * The annotation object being added. (See annotation schema for details.)
   */
  annotation: unknown;

  /**
   * The index of the annotation within the content part.
   */
  annotation_index: number;

  /**
   * The index of the content part within the output item.
   */
  content_index: number;

  /**
   * The unique identifier of the item to which the annotation is being added.
   */
  item_id: string;

  /**
   * The index of the output item in the response's output array.
   */
  output_index: number;

  /**
   * The sequence number of this event.
   */
  sequence_number: number;

  /**
   * The type of the event. Always 'response.output_text.annotation.added'.
   */
  type: "response.output_text.annotation.added";
}
