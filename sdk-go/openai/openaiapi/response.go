package openaiapi

import (
	"encoding/json"
	"fmt"
)

// https://platform.openai.com/docs/api-reference/responses/create

type ResponseCreateParams struct {
	// Specify additional output data to include in the model response. Currently
	// supported values are:
	//
	// - `web_search_call.action.sources`: Include the sources of the web search tool
	//   call.
	// - `code_interpreter_call.outputs`: Includes the outputs of python code execution
	//   in code interpreter tool call items.
	// - `computer_call_output.output.image_url`: Include image urls from the computer
	//   call output.
	// - `file_search_call.results`: Include the search results of the file search tool
	//   call.
	// - `message.input_image.image_url`: Include image urls from the input message.
	// - `computer_call_output.output.image_url`: Include image urls from the computer
	//   call output.
	// - `reasoning.encrypted_content`: Includes an encrypted version of reasoning
	//   tokens in reasoning item outputs. This enables reasoning items to be used in
	//   multi-turn conversations when using the Responses API statelessly (like when
	//   the `store` parameter is set to `false`, or when an organization is enrolled
	//   in the zero data retention program).
	// - `code_interpreter_call.outputs`: Includes the outputs of python code execution
	//   in code interpreter tool call items.
	Include []ResponseIncludable `json:"include,omitempty"`

	// Text, image, or file inputs to the model, used to generate a response.
	//
	// Learn more:
	//
	// - [Text inputs and outputs](https://platform.openai.com/docs/guides/text)
	// - [Image inputs](https://platform.openai.com/docs/guides/images)
	// - [File inputs](https://platform.openai.com/docs/guides/pdf-files)
	// - [Conversation state](https://platform.openai.com/docs/guides/conversation-state)
	// - [Function calling](https://platform.openai.com/docs/guides/function-calling)
	Input []ResponseInputItem `json:"input,omitempty"`

	// A system (or developer) message inserted into the model's context.
	//
	// When using along with `previous_response_id`, the instructions from a previous
	// response will not be carried over to the next response. This makes it simple to
	// swap out system (or developer) messages in new responses.
	Instructions *string `json:"instructions,omitempty"`

	// An upper bound for the number of tokens that can be generated for a response,
	// including visible output tokens and
	// [reasoning tokens](https://platform.openai.com/docs/guides/reasoning).
	MaxOutputTokens *uint32 `json:"max_output_tokens,omitempty"`

	// Model ID used to generate the response, like `gpt-4o` or `o3`. OpenAI offers a
	// wide range of models with different capabilities, performance characteristics,
	// and price points. Refer to the
	// [model guide](https://platform.openai.com/docs/models) to browse and compare
	// available models.
	Model *string `json:"model,omitempty"`

	// Whether to allow the model to run tool calls in parallel.
	ParallelToolCalls *bool `json:"parallel_tool_calls,omitempty"`

	// **gpt-5 and o-series models only**
	//
	// Configuration options for
	// [reasoning models](https://platform.openai.com/docs/guides/reasoning).
	Reasoning *Reasoning `json:"reasoning,omitempty"`

	// Whether to store the generated model response for later retrieval via API.
	Store *bool `json:"store,omitempty"`

	// If set to true, the model response data will be streamed to the client as it is
	// generated using
	// [server-sent events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#Event_stream_format).
	// See the
	// [Streaming section below](https://platform.openai.com/docs/api-reference/responses-streaming)
	// for more information.
	Stream *bool `json:"stream,omitempty"`

	// Options for streaming responses. Only set this when you set `stream: true`.
	StreamOptions *ResponseCreateParamsStreamOptions `json:"stream_options,omitempty"`

	// What sampling temperature to use, between 0 and 2. Higher values like 0.8 will
	// make the output more random, while lower values like 0.2 will make it more
	// focused and deterministic. We generally recommend altering this or `top_p` but
	// not both.
	Temperature *float64 `json:"temperature,omitempty"`

	// Configuration options for a text response from the model. Can be plain text or
	// structured JSON data. Learn more:
	//
	// - [Text inputs and outputs](https://platform.openai.com/docs/guides/text)
	// - [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs)
	Text *ResponseTextConfig `json:"text,omitempty"`

	// How the model should select which tool (or tools) to use when generating a
	// response. See the `tools` parameter to see how to specify which tools the model
	// can call.
	ToolChoice *ToolChoice `json:"tool_choice,omitempty"`

	// An array of tools the model may call while generating a response. You can
	// specify which tool to use by setting the `tool_choice` parameter.
	//
	// We support the following categories of tools:
	//
	// - **Built-in tools**: Tools that are provided by OpenAI that extend the model's
	//   capabilities, like
	//   [web search](https://platform.openai.com/docs/guides/tools-web-search) or
	//   [file search](https://platform.openai.com/docs/guides/tools-file-search).
	//   Learn more about
	//   [built-in tools](https://platform.openai.com/docs/guides/tools).
	// - **MCP Tools**: Integrations with third-party systems via custom MCP servers or
	//   predefined connectors such as Google Drive and SharePoint. Learn more about
	//   [MCP Tools](https://platform.openai.com/docs/guides/tools-connectors-mcp).
	// - **Function calls (custom tools)**: Functions that are defined by you, enabling
	//   the model to call your own code with strongly typed arguments and outputs.
	//   Learn more about
	//   [function calling](https://platform.openai.com/docs/guides/function-calling).
	//   You can also use custom tools to call your own code.
	Tools []Tool `json:"tools,omitempty"`

	// An alternative to sampling with temperature, called nucleus sampling, where the
	// model considers the results of the tokens with top_p probability mass. So 0.1
	// means only the tokens comprising the top 10% probability mass are considered.
	//
	// We generally recommend altering this or `temperature` but not both.
	TopP *float64 `json:"top_p,omitempty"`

	// The truncation strategy to use for the model response.
	//
	// - `auto`: If the context of this response and previous ones exceeds the model's
	//   context window size, the model will truncate the response to fit the context
	//   window by dropping input items in the middle of the conversation.
	// - `disabled` (default): If a model response will exceed the context window size
	//   for a model, the request will fail with a 400 error.
	Truncation *string `json:"truncation,omitempty"`
}

// Specify additional output data to include in the model response. Currently
// supported values are:
//
//   - `web_search_call.action.sources`: Include the sources of the web search tool
//     call.
//   - `code_interpreter_call.outputs`: Includes the outputs of python code execution
//     in code interpreter tool call items.
//   - `computer_call_output.output.image_url`: Include image urls from the computer
//     call output.
//   - `file_search_call.results`: Include the search results of the file search tool
//     call.
//   - `message.input_image.image_url`: Include image urls from the input message.
//   - `computer_call_output.output.image_url`: Include image urls from the computer
//     call output.
//   - `reasoning.encrypted_content`: Includes an encrypted version of reasoning
//     tokens in reasoning item outputs. This enables reasoning items to be used in
//     multi-turn conversations when using the Responses API statelessly (like when
//     the `store` parameter is set to `false`, or when an organization is enrolled
//     in the zero data retention program).
//   - `code_interpreter_call.outputs`: Includes the outputs of python code execution
//     in code interpreter tool call items.
type ResponseIncludable string

const (
	ResponseIncludableFileSearchCallResults      ResponseIncludable = "file_search_call.results"
	ResponseIncludableMessageInputImageURL       ResponseIncludable = "message.input_image.image_url"
	ResponseIncludableComputerCallOutputImageURL ResponseIncludable = "computer_call_output.output.image_url"
	ResponseIncludableReasoningEncryptedContent  ResponseIncludable = "reasoning.encrypted_content"
	ResponseIncludableCodeInterpreterCallOutputs ResponseIncludable = "code_interpreter_call.outputs"
)

// A message input to the model with a role indicating instruction following
// hierarchy. Instructions given with the `developer` or `system` role take
// precedence over instructions given with the `user` role. Messages with the
// `assistant` role are presumed to have been generated by the model in previous
// interactions.
type ResponseInputItem struct {
	*ResponseInputItemMessage              `json:"-"`
	*ResponseOutputMessage                 `json:"-"`
	*ResponseFunctionToolCall              `json:"-"`
	*ResponseInputItemFunctionCallOutput   `json:"-"`
	*ResponseReasoningItem                 `json:"-"`
	*ResponseOutputItemImageGenerationCall `json:"-"`
}

func (r ResponseInputItem) MarshalJSON() ([]byte, error) {
	if r.ResponseInputItemMessage != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseInputItemMessage
		}{
			Type:                     "message",
			ResponseInputItemMessage: r.ResponseInputItemMessage,
		})
	}
	if r.ResponseOutputMessage != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseOutputMessage
		}{
			Type:                  "message",
			ResponseOutputMessage: r.ResponseOutputMessage,
		})
	}
	if r.ResponseFunctionToolCall != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseFunctionToolCall
		}{
			Type:                     "function_call",
			ResponseFunctionToolCall: r.ResponseFunctionToolCall,
		})
	}
	if r.ResponseInputItemFunctionCallOutput != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseInputItemFunctionCallOutput
		}{
			Type:                                "function_call_output",
			ResponseInputItemFunctionCallOutput: r.ResponseInputItemFunctionCallOutput,
		})
	}
	if r.ResponseReasoningItem != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseReasoningItem
		}{
			Type:                  "reasoning",
			ResponseReasoningItem: r.ResponseReasoningItem,
		})
	}
	if r.ResponseOutputItemImageGenerationCall != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseOutputItemImageGenerationCall
		}{
			Type:                                  "image_generation_call",
			ResponseOutputItemImageGenerationCall: r.ResponseOutputItemImageGenerationCall,
		})
	}
	return nil, fmt.Errorf("ResponseInputItem has no content")
}

func (r *ResponseInputItem) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type string `json:"type"`
		Role string `json:"role"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "message":
		if temp.Role == "assistant" {
			var om ResponseOutputMessage
			if err := json.Unmarshal(data, &om); err == nil {
				r.ResponseOutputMessage = &om
				return nil
			}
		}
		var m ResponseInputItemMessage
		if err := json.Unmarshal(data, &m); err != nil {
			return err
		}
		r.ResponseInputItemMessage = &m
	case "function_call":
		var f ResponseFunctionToolCall
		if err := json.Unmarshal(data, &f); err != nil {
			return err
		}
		r.ResponseFunctionToolCall = &f
	case "function_call_output":
		var o ResponseInputItemFunctionCallOutput
		if err := json.Unmarshal(data, &o); err != nil {
			return err
		}
		r.ResponseInputItemFunctionCallOutput = &o
	case "reasoning":
		var re ResponseReasoningItem
		if err := json.Unmarshal(data, &re); err != nil {
			return err
		}
		r.ResponseReasoningItem = &re
	case "image_generation_call":
		var i ResponseOutputItemImageGenerationCall
		if err := json.Unmarshal(data, &i); err != nil {
			return err
		}
		r.ResponseOutputItemImageGenerationCall = &i
	default:
		return fmt.Errorf("unknown ResponseInputItem type: %s", temp.Type)
	}

	return nil
}

// A message input to the model with a role indicating instruction following
// hierarchy. Instructions given with the `developer` or `system` role take
// precedence over instructions given with the `user` role.
type ResponseInputItemMessage struct {
	// A list of one or many input items to the model, containing different content
	// types.
	Content ResponseInputMessageContentList `json:"content"`

	// The role of the message input. One of `user`, `system`, or `developer`.
	Role string `json:"role"`

	// The status of item. One of `in_progress`, `completed`, or `incomplete`.
	// Populated when items are returned via API.
	Status *string `json:"status,omitempty"`
}

// A list of one or many input items to the model, containing different content
// types.
type ResponseInputMessageContentList []ResponseInputContent

// A text input to the model.
type ResponseInputContent struct {
	*ResponseInputText  `json:"-"`
	*ResponseInputImage `json:"-"`
	*ResponseInputFile  `json:"-"`
	*ResponseInputAudio `json:"-"`
}

func (r ResponseInputContent) MarshalJSON() ([]byte, error) {
	if r.ResponseInputText != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseInputText
		}{
			Type:              "input_text",
			ResponseInputText: r.ResponseInputText,
		})
	}
	if r.ResponseInputImage != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseInputImage
		}{
			Type:               "input_image",
			ResponseInputImage: r.ResponseInputImage,
		})
	}
	if r.ResponseInputFile != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseInputFile
		}{
			Type:              "input_file",
			ResponseInputFile: r.ResponseInputFile,
		})
	}
	if r.ResponseInputAudio != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseInputAudio
		}{
			Type:               "input_audio",
			ResponseInputAudio: r.ResponseInputAudio,
		})
	}
	return nil, fmt.Errorf("ResponseInputContent has no content")
}

func (r *ResponseInputContent) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "input_text":
		var t ResponseInputText
		if err := json.Unmarshal(data, &t); err != nil {
			return err
		}
		r.ResponseInputText = &t
	case "input_image":
		var i ResponseInputImage
		if err := json.Unmarshal(data, &i); err != nil {
			return err
		}
		r.ResponseInputImage = &i
	case "input_file":
		var f ResponseInputFile
		if err := json.Unmarshal(data, &f); err != nil {
			return err
		}
		r.ResponseInputFile = &f
	case "input_audio":
		var a ResponseInputAudio
		if err := json.Unmarshal(data, &a); err != nil {
			return err
		}
		r.ResponseInputAudio = &a
	default:
		return fmt.Errorf("unknown ResponseInputContent type: %s", temp.Type)
	}

	return nil
}

// A text input to the model.
type ResponseInputText struct {
	// The text input to the model.
	Text string `json:"text"`
}

// An image input to the model. Learn about
// [image inputs](https://platform.openai.com/docs/guides/vision).
type ResponseInputImage struct {
	// The detail level of the image to be sent to the model. One of `high`, `low`, or
	// `auto`. Defaults to `auto`.
	Detail string `json:"detail"`

	// The ID of the file to be sent to the model.
	FileID *string `json:"file_id,omitempty"`

	// The URL of the image to be sent to the model. A fully qualified URL or base64
	// encoded image in a data URL.
	ImageURL *string `json:"image_url,omitempty"`
}

// A file input to the model.
type ResponseInputFile struct {
	// The content of the file to be sent to the model.
	FileData *string `json:"file_data,omitempty"`

	// The ID of the file to be sent to the model.
	FileID *string `json:"file_id,omitempty"`

	// The URL of the file to be sent to the model.
	FileURL *string `json:"file_url,omitempty"`

	// The name of the file to be sent to the model.
	Filename *string `json:"filename,omitempty"`
}

// An audio input to the model.
type ResponseInputAudio struct {
	InputAudio ResponseInputAudioInputAudio `json:"input_audio"`
}

type ResponseInputAudioInputAudio struct {
	// Base64-encoded audio data.
	Data string `json:"data"`

	// The format of the audio data. Currently supported formats are `mp3` and `wav`.
	Format string `json:"format"`
}

type ResponseOutputMessage struct {
	// The unique ID of the output message.
	ID string `json:"id"`

	// The content of the output message.
	Content []ResponseOutputContent `json:"content"`

	// The role of the output message. Always `assistant`.
	Role string `json:"role"`

	// The status of the message input. One of `in_progress`, `completed`, or
	// `incomplete`. Populated when input items are returned via API.
	Status string `json:"status"`
}

type ResponseOutputContent struct {
	*ResponseOutputText    `json:"-"`
	*ResponseOutputRefusal `json:"-"`
}

func (r ResponseOutputContent) MarshalJSON() ([]byte, error) {
	if r.ResponseOutputText != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseOutputText
		}{
			Type:               "output_text",
			ResponseOutputText: r.ResponseOutputText,
		})
	}
	if r.ResponseOutputRefusal != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseOutputRefusal
		}{
			Type:                  "refusal",
			ResponseOutputRefusal: r.ResponseOutputRefusal,
		})
	}
	return nil, fmt.Errorf("ResponseOutputContent has no content")
}

func (r *ResponseOutputContent) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "output_text":
		var t ResponseOutputText
		if err := json.Unmarshal(data, &t); err != nil {
			return err
		}
		r.ResponseOutputText = &t
	case "refusal":
		var ref ResponseOutputRefusal
		if err := json.Unmarshal(data, &ref); err != nil {
			return err
		}
		r.ResponseOutputRefusal = &ref
	default:
		return fmt.Errorf("unknown ResponseOutputContent type: %s", temp.Type)
	}

	return nil
}

// A text output from the model.
type ResponseOutputText struct {
	// The annotations of the text output.
	Annotations []ResponseOutputTextAnnotation `json:"annotations"`

	// The text output from the model.
	Text string `json:"text"`
}

type ResponseOutputTextAnnotation struct {
	*ResponseOutputTextFileCitation `json:"-"`
	*ResponseOutputTextURLCitation  `json:"-"`
	*ResponseOutputTextFilePath     `json:"-"`
}

func (r ResponseOutputTextAnnotation) MarshalJSON() ([]byte, error) {
	if r.ResponseOutputTextFileCitation != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseOutputTextFileCitation
		}{
			Type:                           "file_citation",
			ResponseOutputTextFileCitation: r.ResponseOutputTextFileCitation,
		})
	}
	if r.ResponseOutputTextURLCitation != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseOutputTextURLCitation
		}{
			Type:                          "url_citation",
			ResponseOutputTextURLCitation: r.ResponseOutputTextURLCitation,
		})
	}
	if r.ResponseOutputTextFilePath != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseOutputTextFilePath
		}{
			Type:                       "file_path",
			ResponseOutputTextFilePath: r.ResponseOutputTextFilePath,
		})
	}
	return nil, fmt.Errorf("ResponseOutputTextAnnotation has no content")
}

func (r *ResponseOutputTextAnnotation) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "file_citation":
		var fc ResponseOutputTextFileCitation
		if err := json.Unmarshal(data, &fc); err != nil {
			return err
		}
		r.ResponseOutputTextFileCitation = &fc
	case "url_citation":
		var uc ResponseOutputTextURLCitation
		if err := json.Unmarshal(data, &uc); err != nil {
			return err
		}
		r.ResponseOutputTextURLCitation = &uc
	case "file_path":
		var fp ResponseOutputTextFilePath
		if err := json.Unmarshal(data, &fp); err != nil {
			return err
		}
		r.ResponseOutputTextFilePath = &fp
	default:
		return fmt.Errorf("unknown ResponseOutputTextAnnotation type: %s", temp.Type)
	}

	return nil
}

// A citation to a file.
type ResponseOutputTextFileCitation struct {
	// The ID of the file.
	FileID string `json:"file_id"`

	// The filename of the file cited.
	Filename string `json:"filename"`

	// The index of the file in the list of files.
	Index int `json:"index"`
}

// A citation for a web resource used to generate a model response.
type ResponseOutputTextURLCitation struct {
	// The index of the last character of the URL citation in the message.
	EndIndex int `json:"end_index"`

	// The index of the first character of the URL citation in the message.
	StartIndex int `json:"start_index"`

	// The title of the web resource.
	Title string `json:"title"`

	// The URL of the web resource.
	URL string `json:"url"`
}

// A path to a file.
type ResponseOutputTextFilePath struct {
	// The ID of the file.
	FileID string `json:"file_id"`

	// The index of the file in the list of files.
	Index int `json:"index"`
}

// A refusal from the model.
type ResponseOutputRefusal struct {
	// The refusal explanation from the model.
	Refusal string `json:"refusal"`
}

// A tool call to run a function. See the
// [function calling guide](https://platform.openai.com/docs/guides/function-calling)
// for more information.
type ResponseFunctionToolCall struct {
	// A JSON string of the arguments to pass to the function.
	Arguments string `json:"arguments"`

	// The unique ID of the function tool call generated by the model.
	CallID string `json:"call_id"`

	// The name of the function to run.
	Name string `json:"name"`

	// The unique ID of the function tool call.
	ID *string `json:"id,omitempty"`

	// The status of the item. One of `in_progress`, `completed`, or `incomplete`.
	// Populated when items are returned via API.
	Status *string `json:"status,omitempty"`
}

// The output of a function tool call.
type ResponseInputItemFunctionCallOutput struct {
	// The unique ID of the function tool call generated by the model.
	CallID string `json:"call_id"`

	// A JSON string of the output of the function tool call.
	Output string `json:"output"`

	// The unique ID of the function tool call output. Populated when this item is
	// returned via API.
	ID *string `json:"id,omitempty"`

	// The status of the item. One of `in_progress`, `completed`, or `incomplete`.
	// Populated when items are returned via API.
	Status *string `json:"status,omitempty"`
}

// A description of the chain of thought used by a reasoning model while generating
// a response. Be sure to include these items in your `input` to the Responses API
// for subsequent turns of a conversation if you are manually
// [managing context](https://platform.openai.com/docs/guides/conversation-state).
type ResponseReasoningItem struct {
	// The unique identifier of the reasoning content.
	ID string `json:"id"`

	// Reasoning summary content.
	Summary []ResponseReasoningItemSummaryUnion `json:"summary"`

	// Reasoning text content.
	Content []ResponseReasoningItemContentUnion `json:"content,omitempty"`

	// The encrypted content of the reasoning item - populated when a response is
	// generated with `reasoning.encrypted_content` in the `include` parameter.
	EncryptedContent *string `json:"encrypted_content,omitempty"`

	// The status of the item. One of `in_progress`, `completed`, or `incomplete`.
	// Populated when items are returned via API.
	Status *string `json:"status,omitempty"`
}

type ResponseReasoningItemSummaryUnion struct {
	ResponseReasoningItemSummary *ResponseReasoningItemSummary `json:"-"`
}

func (r ResponseReasoningItemSummaryUnion) MarshalJSON() ([]byte, error) {
	if r.ResponseReasoningItemSummary != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseReasoningItemSummary
		}{
			Type:                         "summary_text",
			ResponseReasoningItemSummary: r.ResponseReasoningItemSummary,
		})
	}
	return nil, fmt.Errorf("ResponseReasoningItemSummaryUnion has no content")
}

func (r *ResponseReasoningItemSummaryUnion) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "summary_text":
		var s ResponseReasoningItemSummary
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		r.ResponseReasoningItemSummary = &s
	default:
		return fmt.Errorf("unknown ResponseReasoningItemSummaryUnion type: %s", temp.Type)
	}

	return nil
}

type ResponseReasoningItemContentUnion struct {
	ReasoningText *ResponseReasoningItemContent `json:"-"`
}

func (r ResponseReasoningItemContentUnion) MarshalJSON() ([]byte, error) {
	switch {
	case r.ReasoningText != nil:
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseReasoningItemContent
		}{
			Type:                         "reasoning_text",
			ResponseReasoningItemContent: r.ReasoningText,
		})
	}
	return nil, fmt.Errorf("ResponseReasoningItemContentUnion has no content")
}

type ResponseReasoningItemSummary struct {
	// Summary text content.
	Text string `json:"text"`
}

type ResponseReasoningItemContent struct {
	// Reasoning text output from the model.
	Text string `json:"text"`
}

type ResponseOutputItemImageGenerationCall struct {
	// The unique ID of the image generation call.
	ID string `json:"id"`

	// The generated image encoded in base64.
	Result *string `json:"result,omitempty"`

	// The status of the image generation call.
	Status string `json:"status"`

	// png, jpeg, etc.
	OutputFormat string `json:"output_format,omitempty"`

	// {number}x{number}
	Size string `json:"size,omitempty"`
}

// **o-series models only**
//
// Configuration options for
// [reasoning models](https://platform.openai.com/docs/guides/reasoning).
type Reasoning struct {
	// Constrains effort on reasoning for
	// [reasoning models](https://platform.openai.com/docs/guides/reasoning). Currently
	// supported values are `minimal`, `low`, `medium`, and `high`. Reducing reasoning
	// effort can result in faster responses and fewer tokens used on reasoning in a
	// response.
	Effort *ReasoningEffort `json:"effort,omitempty"`

	// A summary of the reasoning performed by the model. This can be useful for
	// debugging and understanding the model's reasoning process. One of `auto`,
	// `concise`, or `detailed`.
	Summary *string `json:"summary,omitempty"`
}

// Constrains effort on reasoning for
// [reasoning models](https://platform.openai.com/docs/guides/reasoning). Currently
// supported values are `minimal`, `low`, `medium`, and `high`. Reducing reasoning
// effort can result in faster responses and fewer tokens used on reasoning in a
// response.
type ReasoningEffort string

const (
	ReasoningEffortMinimal ReasoningEffort = "minimal"
	ReasoningEffortLow     ReasoningEffort = "low"
	ReasoningEffortMedium  ReasoningEffort = "medium"
	ReasoningEffortHigh    ReasoningEffort = "high"
)

// Options for streaming responses. Only set this when you set `stream: true`.
type ResponseCreateParamsStreamOptions struct {
	// When true, stream obfuscation will be enabled. Stream obfuscation adds random
	// characters to an `obfuscation` field on streaming delta events to normalize
	// payload sizes as a mitigation to certain side-channel attacks. These obfuscation
	// fields are included by default, but add a small amount of overhead to the data
	// stream. You can set `include_obfuscation` to false to optimize for bandwidth if
	// you trust the network links between your application and the OpenAI API.
	IncludeObfuscation *bool `json:"include_obfuscation,omitempty"`
}

// Configuration options for a text response from the model. Can be plain text or
// structured JSON data. Learn more:
//
// - [Text inputs and outputs](https://platform.openai.com/docs/guides/text)
// - [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs)
type ResponseTextConfig struct {
	// An object specifying the format that the model must output.
	//
	// Configuring `{ "type": "json_schema" }` enables Structured Outputs, which
	// ensures the model will match your supplied JSON schema. Learn more in the
	// [Structured Outputs guide](https://platform.openai.com/docs/guides/structured-outputs).
	//
	// The default format is `{ "type": "text" }` with no additional options.
	//
	// **Not recommended for gpt-4o and newer models:**
	//
	// Setting to `{ "type": "json_object" }` enables the older JSON mode, which
	// ensures the message the model generates is valid JSON. Using `json_schema` is
	// preferred for models that support it.
	Format *ResponseFormatTextConfig `json:"format,omitempty"`

	// Constrains the verbosity of the model's response. Lower values will result in
	// more concise responses, while higher values will result in more verbose
	// responses. Currently supported values are `low`, `medium`, and `high`.
	Verbosity *string `json:"verbosity,omitempty"`
}

type ResponseFormatTextConfig struct {
	*ResponseFormatText                 `json:"-"`
	*ResponseFormatTextJSONSchemaConfig `json:"-"`
	*ResponseFormatJSONObject           `json:"-"`
}

func (r ResponseFormatTextConfig) MarshalJSON() ([]byte, error) {
	if r.ResponseFormatText != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseFormatText
		}{
			Type:               "text",
			ResponseFormatText: r.ResponseFormatText,
		})
	}
	if r.ResponseFormatTextJSONSchemaConfig != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseFormatTextJSONSchemaConfig
		}{
			Type:                               "json_schema",
			ResponseFormatTextJSONSchemaConfig: r.ResponseFormatTextJSONSchemaConfig,
		})
	}
	if r.ResponseFormatJSONObject != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseFormatJSONObject
		}{
			Type:                     "json_object",
			ResponseFormatJSONObject: r.ResponseFormatJSONObject,
		})
	}
	return nil, fmt.Errorf("ResponseFormatTextConfig has no content")
}

func (r *ResponseFormatTextConfig) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "text":
		var t ResponseFormatText
		if err := json.Unmarshal(data, &t); err != nil {
			return err
		}
		r.ResponseFormatText = &t
	case "json_schema":
		var js ResponseFormatTextJSONSchemaConfig
		if err := json.Unmarshal(data, &js); err != nil {
			return err
		}
		r.ResponseFormatTextJSONSchemaConfig = &js
	case "json_object":
		var jo ResponseFormatJSONObject
		if err := json.Unmarshal(data, &jo); err != nil {
			return err
		}
		r.ResponseFormatJSONObject = &jo
	default:
		return fmt.Errorf("unknown ResponseFormatTextConfig type: %s", temp.Type)
	}

	return nil
}

// Default response format. Used to generate text responses.
type ResponseFormatText struct {
}

// JSON Schema response format. Used to generate structured JSON responses. Learn
// more about
// [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs).
type ResponseFormatTextJSONSchemaConfig struct {
	// The name of the response format. Must be a-z, A-Z, 0-9, or contain underscores
	// and dashes, with a maximum length of 64.
	Name string `json:"name"`

	// The schema for the response format, described as a JSON Schema object. Learn how
	// to build JSON schemas [here](https://json-schema.org/).
	Schema map[string]any `json:"schema"`

	// A description of what the response format is for, used by the model to determine
	// how to respond in the format.
	Description *string `json:"description,omitempty"`

	// Whether to enable strict schema adherence when generating the output. If set to
	// true, the model will always follow the exact schema defined in the `schema`
	// field. Only a subset of JSON Schema is supported when `strict` is `true`. To
	// learn more, read the
	// [Structured Outputs guide](https://platform.openai.com/docs/guides/structured-outputs).
	Strict *bool `json:"strict,omitempty"`
}

// JSON object response format. An older method of generating JSON responses. Using
// `json_schema` is recommended for models that support it. Note that the model
// will not generate JSON without a system or user message instructing it to do so.
type ResponseFormatJSONObject struct {
}

type ToolChoiceOptions string

const (
	ToolChoiceOptionsNone     ToolChoiceOptions = "none"
	ToolChoiceOptionsAuto     ToolChoiceOptions = "auto"
	ToolChoiceOptionsRequired ToolChoiceOptions = "required"
)

// Constrains the tools available to the model to a pre-defined set.
type ToolChoiceAllowed struct {
	Type string `json:"type"`
	// Constrains the tools available to the model to a pre-defined set.
	//
	// `auto` allows the model to pick from among the allowed tools and generate a
	// message.
	//
	// `required` requires the model to call one or more of the allowed tools.
	Mode ToolChoiceOptions `json:"mode"`

	// A list of tool definitions that the model should be allowed to call.
	//
	// For the Responses API, the list of tool definitions might look like:
	//
	// ```json
	// [
	//   { "type": "function", "name": "get_weather" },
	//   { "type": "mcp", "server_label": "deepwiki" },
	//   { "type": "image_generation" }
	// ]
	// ```
	Tools []map[string]any `json:"tools"`
}

// Indicates that the model should use a built-in tool to generate a response.
// [Learn more about built-in tools](https://platform.openai.com/docs/guides/tools).
type ToolChoiceTypes struct {
	// The type of hosted tool the model should to use. Learn more about
	// [built-in tools](https://platform.openai.com/docs/guides/tools).
	//
	// Allowed values are:
	//
	// - `file_search`
	// - `web_search_preview`
	// - `computer_use_preview`
	// - `code_interpreter`
	// - `mcp`
	// - `image_generation`
	Type string `json:"type"`
}

// Use this option to force the model to call a specific function.
type ToolChoiceFunction struct {
	// The name of the function to call.
	Name string `json:"name"`

	Type string `json:"type"`
}

// Use this option to force the model to call a specific custom tool.
type ToolChoiceCustom struct {
	// The name of the custom tool to call.
	Name string `json:"name"`

	Type string `json:"type"`
}

// Use this option to force the model to call a specific tool on a remote MCP
// server.
type ToolChoiceMCP struct {
	Type string `json:"type"`

	ServerLabel string `json:"server_label"`

	Name *string `json:"name,omitempty"`
}

// ToolChoice captures the union of supported tool choice options for the
// Responses API.
type ToolChoice struct {
	Options  *ToolChoiceOptions
	Allowed  *ToolChoiceAllowed
	Types    *ToolChoiceTypes
	Function *ToolChoiceFunction
	MCP      *ToolChoiceMCP
	Custom   *ToolChoiceCustom
}

func (t ToolChoice) MarshalJSON() ([]byte, error) {
	switch {
	case t.Options != nil:
		return json.Marshal(t.Options)
	case t.Allowed != nil:
		return json.Marshal(t.Allowed)
	case t.Types != nil:
		return json.Marshal(t.Types)
	case t.Function != nil:
		return json.Marshal(t.Function)
	case t.MCP != nil:
		return json.Marshal(t.MCP)
	case t.Custom != nil:
		return json.Marshal(t.Custom)
	default:
		return []byte("null"), nil
	}
}

func (t *ToolChoice) UnmarshalJSON(data []byte) error {
	var option ToolChoiceOptions
	if err := json.Unmarshal(data, &option); err == nil {
		switch option {
		case ToolChoiceOptionsAuto, ToolChoiceOptionsNone, ToolChoiceOptionsRequired:
			*t = ToolChoice{Options: &option}
			return nil
		}
	}

	var discriminator struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &discriminator); err != nil {
		return err
	}

	switch discriminator.Type {
	case "allowed_tools":
		var value ToolChoiceAllowed
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*t = ToolChoice{Allowed: &value}
	case "function":
		var value ToolChoiceFunction
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*t = ToolChoice{Function: &value}
	case "mcp":
		var value ToolChoiceMCP
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*t = ToolChoice{MCP: &value}
	case "custom":
		var value ToolChoiceCustom
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*t = ToolChoice{Custom: &value}
	default:
		var value ToolChoiceTypes
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*t = ToolChoice{Types: &value}
	}

	return nil
}

// A tool that can be used to generate a response.
type Tool struct {
	*FunctionTool        `json:"-"`
	*WebSearchTool       `json:"-"`
	*ToolImageGeneration `json:"-"`
}

func (t Tool) MarshalJSON() ([]byte, error) {
	if t.FunctionTool != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*FunctionTool
		}{
			Type:         "function",
			FunctionTool: t.FunctionTool,
		})
	}
	if t.WebSearchTool != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*WebSearchTool
		}{
			Type:          "web_search",
			WebSearchTool: t.WebSearchTool,
		})
	}
	if t.ToolImageGeneration != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ToolImageGeneration
		}{
			Type:                "image_generation",
			ToolImageGeneration: t.ToolImageGeneration,
		})
	}
	return nil, fmt.Errorf("Tool has no content")
}

func (t *Tool) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "function":
		var f FunctionTool
		if err := json.Unmarshal(data, &f); err != nil {
			return err
		}
		t.FunctionTool = &f
	case "web_search", "web_search_2025_08_26":
		var w WebSearchTool
		if err := json.Unmarshal(data, &w); err != nil {
			return err
		}
		t.WebSearchTool = &w
	case "image_generation":
		var i ToolImageGeneration
		if err := json.Unmarshal(data, &i); err != nil {
			return err
		}
		t.ToolImageGeneration = &i
	default:
		return fmt.Errorf("unknown Tool type: %s", temp.Type)
	}

	return nil
}

// Defines a function in your own code the model can choose to call. Learn more
// about
// [function calling](https://platform.openai.com/docs/guides/function-calling).
type FunctionTool struct {
	// The name of the function to call.
	Name string `json:"name"`

	// A JSON schema object describing the parameters of the function.
	Parameters any `json:"parameters,omitempty"`

	// Whether to enforce strict parameter validation. Default `true`.
	Strict *bool `json:"strict,omitempty"`

	// A description of the function. Used by the model to determine whether or not to
	// call the function.
	Description *string `json:"description,omitempty"`
}

// Search the Internet for sources related to the prompt. Learn more about the
// [web search tool](https://platform.openai.com/docs/guides/tools-web-search).
type WebSearchTool struct {
	// Filters for the search.
	Filters *WebSearchToolFilters `json:"filters,omitempty"`

	// High level guidance for the amount of context window space to use for the
	// search. One of `low`, `medium`, or `high`. `medium` is the default.
	SearchContextSize *string `json:"search_context_size,omitempty"`

	// The approximate location of the user.
	UserLocation *WebSearchToolUserLocation `json:"user_location,omitempty"`
}

// Filters for the search.
type WebSearchToolFilters struct {
	// Allowed domains for the search. If not provided, all domains are allowed.
	// Subdomains of the provided domains are allowed as well.
	//
	// Example: `["pubmed.ncbi.nlm.nih.gov"]`
	AllowedDomains []string `json:"allowed_domains,omitempty"`
}

// The approximate location of the user.
type WebSearchToolUserLocation struct {
	// Free text input for the city of the user, e.g. `San Francisco`.
	City *string `json:"city,omitempty"`

	// The two-letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1) of
	// the user, e.g. `US`.
	Country *string `json:"country,omitempty"`

	// Free text input for the region of the user, e.g. `California`.
	Region *string `json:"region,omitempty"`

	// The [IANA timezone](https://timeapi.io/documentation/iana-timezones) of the
	// user, e.g. `America/Los_Angeles`.
	Timezone *string `json:"timezone,omitempty"`
}

// A tool that generates images using a model like `gpt-image-1`.
type ToolImageGeneration struct {
	// Background type for the generated image. One of `transparent`, `opaque`, or
	// `auto`. Default: `auto`.
	Background *string `json:"background,omitempty"`

	// Control how much effort the model will exert to match the style and features,
	// especially facial features, of input images. This parameter is only supported
	// for `gpt-image-1`. Supports `high` and `low`. Defaults to `low`.
	InputFidelity *string `json:"input_fidelity,omitempty"`

	// Optional mask for inpainting. Contains `image_url` (string, optional) and
	// `file_id` (string, optional).
	InputImageMask *ImageGenerationInputImageMask `json:"input_image_mask,omitempty"`

	// The image generation model to use. Default: `gpt-image-1`.
	Model *string `json:"model,omitempty"`

	// Moderation level for the generated image. Default: `auto`.
	Moderation *string `json:"moderation,omitempty"`

	// Compression level for the output image. Default: 100.
	OutputCompression *int `json:"output_compression,omitempty"`

	// The output format of the generated image. One of `png`, `webp`, or `jpeg`.
	// Default: `png`.
	OutputFormat *string `json:"output_format,omitempty"`

	// Number of partial images to generate in streaming mode, from 0 (default value)
	// to 3.
	PartialImages *int `json:"partial_images,omitempty"`

	// The quality of the generated image. One of `low`, `medium`, `high`, or `auto`.
	// Default: `auto`.
	Quality *string `json:"quality,omitempty"`

	// The size of the generated image. One of `1024x1024`, `1024x1536`, `1536x1024`,
	// or `auto`. Default: `auto`.
	Size *string `json:"size,omitempty"`
}

type ImageGenerationInputImageMask struct {
	// File ID for the mask image.
	FileID *string `json:"file_id,omitempty"`

	// Base64-encoded mask image.
	ImageURL *string `json:"image_url,omitempty"`
}

type Response struct {
	// Unique identifier for this Response.
	ID string `json:"id"`

	// Unix timestamp (in seconds) of when this Response was created.
	CreatedAt int64 `json:"created_at"`

	// Model ID used to generate the response, like `gpt-4o` or `o3`. OpenAI offers a
	// wide range of models with different capabilities, performance characteristics,
	// and price points. Refer to the
	// [model guide](https://platform.openai.com/docs/models) to browse and compare
	// available models.
	Model string `json:"model"`

	// The object type of this resource - always set to `response`.
	Object string `json:"object"`

	// An array of content items generated by the model.
	//
	// - The length and order of items in the `output` array is dependent on the
	//   model's response.
	// - Rather than accessing the first item in the `output` array and assuming it's
	//   an `assistant` message with the content generated by the model, you might
	//   consider using the `output_text` property where supported in SDKs.
	Output []ResponseOutputItem `json:"output"`

	// The status of the response generation. One of `completed`, `failed`,
	// `in_progress`, `cancelled`, `queued`, or `incomplete`.
	Status *ResponseStatus `json:"status,omitempty"`

	// Represents token usage details including input tokens, output tokens, a
	// breakdown of output tokens, and the total tokens used.
	Usage *ResponseUsage `json:"usage,omitempty"`
}

// An output message from the model.
type ResponseOutputItem struct {
	*ResponseOutputMessage                 `json:"-"`
	*ResponseFunctionToolCall              `json:"-"`
	*ResponseFunctionWebSearch             `json:"-"`
	*ResponseReasoningItem                 `json:"-"`
	*ResponseOutputItemImageGenerationCall `json:"-"`
}

func (r ResponseOutputItem) MarshalJSON() ([]byte, error) {
	if r.ResponseOutputMessage != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseOutputMessage
		}{
			Type:                  "message",
			ResponseOutputMessage: r.ResponseOutputMessage,
		})
	}
	if r.ResponseFunctionToolCall != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseFunctionToolCall
		}{
			Type:                     "function_call",
			ResponseFunctionToolCall: r.ResponseFunctionToolCall,
		})
	}
	if r.ResponseFunctionWebSearch != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseFunctionWebSearch
		}{
			Type:                      "web_search_call",
			ResponseFunctionWebSearch: r.ResponseFunctionWebSearch,
		})
	}
	if r.ResponseReasoningItem != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseReasoningItem
		}{
			Type:                  "reasoning",
			ResponseReasoningItem: r.ResponseReasoningItem,
		})
	}
	if r.ResponseOutputItemImageGenerationCall != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseOutputItemImageGenerationCall
		}{
			Type:                                  "image_generation_call",
			ResponseOutputItemImageGenerationCall: r.ResponseOutputItemImageGenerationCall,
		})
	}
	return nil, fmt.Errorf("ResponseOutputItem has no content")
}

func (r *ResponseOutputItem) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "message":
		var m ResponseOutputMessage
		if err := json.Unmarshal(data, &m); err != nil {
			return err
		}
		r.ResponseOutputMessage = &m
	case "function_call":
		var f ResponseFunctionToolCall
		if err := json.Unmarshal(data, &f); err != nil {
			return err
		}
		r.ResponseFunctionToolCall = &f
	case "web_search_call":
		var w ResponseFunctionWebSearch
		if err := json.Unmarshal(data, &w); err != nil {
			return err
		}
		r.ResponseFunctionWebSearch = &w
	case "reasoning":
		var re ResponseReasoningItem
		if err := json.Unmarshal(data, &re); err != nil {
			return err
		}
		r.ResponseReasoningItem = &re
	case "image_generation_call":
		var i ResponseOutputItemImageGenerationCall
		if err := json.Unmarshal(data, &i); err != nil {
			return err
		}
		r.ResponseOutputItemImageGenerationCall = &i
	default:
		return fmt.Errorf("unknown ResponseOutputItem type: %s", temp.Type)
	}

	return nil
}

// The results of a web search tool call. See the
// [web search guide](https://platform.openai.com/docs/guides/tools-web-search) for
// more information.
type ResponseFunctionWebSearch struct {
	// The unique ID of the web search tool call.
	ID string `json:"id"`

	// The status of the web search tool call.
	Status string `json:"status"`
}

type ResponseStatus string

const (
	ResponseStatusCompleted  ResponseStatus = "completed"
	ResponseStatusFailed     ResponseStatus = "failed"
	ResponseStatusInProgress ResponseStatus = "in_progress"
	ResponseStatusCancelled  ResponseStatus = "cancelled"
	ResponseStatusQueued     ResponseStatus = "queued"
	ResponseStatusIncomplete ResponseStatus = "incomplete"
)

// Represents token usage details including input tokens, output tokens, a
// breakdown of output tokens, and the total tokens used.
type ResponseUsage struct {
	// The number of input tokens.
	InputTokens int `json:"input_tokens"`

	// A detailed breakdown of the input tokens.
	InputTokensDetails ResponseUsageInputTokensDetails `json:"input_tokens_details"`

	// The number of output tokens.
	OutputTokens int `json:"output_tokens"`

	// A detailed breakdown of the output tokens.
	OutputTokensDetails ResponseUsageOutputTokensDetails `json:"output_tokens_details"`

	// The total number of tokens used.
	TotalTokens int `json:"total_tokens"`
}

type ResponseUsageInputTokensDetails struct {
	// The number of tokens that were retrieved from the cache.
	// [More on prompt caching](https://platform.openai.com/docs/guides/prompt-caching).
	CachedTokens int `json:"cached_tokens"`
}

type ResponseUsageOutputTokensDetails struct {
	// The number of reasoning tokens.
	ReasoningTokens int `json:"reasoning_tokens"`
}

// Emitted when there is a partial audio response.
type ResponseStreamEvent struct {
	ResponseAudioDeltaEvent                 *ResponseAudioDeltaEvent                 `json:"-"`
	ResponseAudioDoneEvent                  *ResponseAudioDoneEvent                  `json:"-"`
	ResponseAudioTranscriptDeltaEvent       *ResponseAudioTranscriptDeltaEvent       `json:"-"`
	ResponseAudioTranscriptDoneEvent        *ResponseAudioTranscriptDoneEvent        `json:"-"`
	ResponseCompletedEvent                  *ResponseCompletedEvent                  `json:"-"`
	ResponseContentPartAddedEvent           *ResponseContentPartAddedEvent           `json:"-"`
	ResponseContentPartDoneEvent            *ResponseContentPartDoneEvent            `json:"-"`
	ResponseCreatedEvent                    *ResponseCreatedEvent                    `json:"-"`
	ResponseErrorEvent                      *ResponseErrorEvent                      `json:"-"`
	ResponseFunctionCallArgumentsDeltaEvent *ResponseFunctionCallArgumentsDeltaEvent `json:"-"`
	ResponseFunctionCallArgumentsDoneEvent  *ResponseFunctionCallArgumentsDoneEvent  `json:"-"`
	ResponseInProgressEvent                 *ResponseInProgressEvent                 `json:"-"`
	ResponseFailedEvent                     *ResponseFailedEvent                     `json:"-"`
	ResponseIncompleteEvent                 *ResponseIncompleteEvent                 `json:"-"`
	ResponseOutputItemAddedEvent            *ResponseOutputItemAddedEvent            `json:"-"`
	ResponseOutputItemDoneEvent             *ResponseOutputItemDoneEvent             `json:"-"`
	ResponseReasoningSummaryPartAddedEvent  *ResponseReasoningSummaryPartAddedEvent  `json:"-"`
	ResponseReasoningSummaryPartDoneEvent   *ResponseReasoningSummaryPartDoneEvent   `json:"-"`
	ResponseReasoningSummaryTextDeltaEvent  *ResponseReasoningSummaryTextDeltaEvent  `json:"-"`
	ResponseReasoningSummaryTextDoneEvent   *ResponseReasoningSummaryTextDoneEvent   `json:"-"`
	ResponseReasoningTextDeltaEvent         *ResponseReasoningTextDeltaEvent         `json:"-"`
	ResponseReasoningTextDoneEvent          *ResponseReasoningTextDoneEvent          `json:"-"`
	ResponseRefusalDeltaEvent               *ResponseRefusalDeltaEvent               `json:"-"`
	ResponseRefusalDoneEvent                *ResponseRefusalDoneEvent                `json:"-"`
	ResponseTextDeltaEvent                  *ResponseTextDeltaEvent                  `json:"-"`
	ResponseTextDoneEvent                   *ResponseTextDoneEvent                   `json:"-"`
	ResponseImageGenCallCompletedEvent      *ResponseImageGenCallCompletedEvent      `json:"-"`
	ResponseImageGenCallGeneratingEvent     *ResponseImageGenCallGeneratingEvent     `json:"-"`
	ResponseImageGenCallInProgressEvent     *ResponseImageGenCallInProgressEvent     `json:"-"`
	ResponseImageGenCallPartialImageEvent   *ResponseImageGenCallPartialImageEvent   `json:"-"`
	ResponseOutputTextAnnotationAddedEvent  *ResponseOutputTextAnnotationAddedEvent  `json:"-"`
}

func (r *ResponseStreamEvent) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "response.audio.delta":
		var e ResponseAudioDeltaEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseAudioDeltaEvent = &e
	case "response.audio.done":
		var e ResponseAudioDoneEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseAudioDoneEvent = &e
	case "response.audio.transcript.delta":
		var e ResponseAudioTranscriptDeltaEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseAudioTranscriptDeltaEvent = &e
	case "response.audio.transcript.done":
		var e ResponseAudioTranscriptDoneEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseAudioTranscriptDoneEvent = &e
	case "response.completed":
		var e ResponseCompletedEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseCompletedEvent = &e
	case "response.content_part.added":
		var e ResponseContentPartAddedEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseContentPartAddedEvent = &e
	case "response.content_part.done":
		var e ResponseContentPartDoneEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseContentPartDoneEvent = &e
	case "response.created":
		var e ResponseCreatedEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseCreatedEvent = &e
	case "error":
		var e ResponseErrorEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseErrorEvent = &e
	case "response.function_call_arguments.delta":
		var e ResponseFunctionCallArgumentsDeltaEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseFunctionCallArgumentsDeltaEvent = &e
	case "response.function_call_arguments.done":
		var e ResponseFunctionCallArgumentsDoneEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseFunctionCallArgumentsDoneEvent = &e
	case "response.in_progress":
		var e ResponseInProgressEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseInProgressEvent = &e
	case "response.failed":
		var e ResponseFailedEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseFailedEvent = &e
	case "response.incomplete":
		var e ResponseIncompleteEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseIncompleteEvent = &e
	case "response.output_item.added":
		var e ResponseOutputItemAddedEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseOutputItemAddedEvent = &e
	case "response.output_item.done":
		var e ResponseOutputItemDoneEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseOutputItemDoneEvent = &e
	case "response.reasoning_summary_part.added":
		var e ResponseReasoningSummaryPartAddedEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseReasoningSummaryPartAddedEvent = &e
	case "response.reasoning_summary_part.done":
		var e ResponseReasoningSummaryPartDoneEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseReasoningSummaryPartDoneEvent = &e
	case "response.reasoning_summary_text.delta":
		var e ResponseReasoningSummaryTextDeltaEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseReasoningSummaryTextDeltaEvent = &e
	case "response.reasoning_summary_text.done":
		var e ResponseReasoningSummaryTextDoneEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseReasoningSummaryTextDoneEvent = &e
	case "response.reasoning_text.delta":
		var e ResponseReasoningTextDeltaEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseReasoningTextDeltaEvent = &e
	case "response.reasoning_text.done":
		var e ResponseReasoningTextDoneEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseReasoningTextDoneEvent = &e
	case "response.refusal.delta":
		var e ResponseRefusalDeltaEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseRefusalDeltaEvent = &e
	case "response.refusal.done":
		var e ResponseRefusalDoneEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseRefusalDoneEvent = &e
	case "response.output_text.delta":
		var e ResponseTextDeltaEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseTextDeltaEvent = &e
	case "response.output_text.done":
		var e ResponseTextDoneEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseTextDoneEvent = &e
	case "response.image_generation_call.completed":
		var e ResponseImageGenCallCompletedEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseImageGenCallCompletedEvent = &e
	case "response.image_generation_call.generating":
		var e ResponseImageGenCallGeneratingEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseImageGenCallGeneratingEvent = &e
	case "response.image_generation_call.in_progress":
		var e ResponseImageGenCallInProgressEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseImageGenCallInProgressEvent = &e
	case "response.image_generation_call.partial_image":
		var e ResponseImageGenCallPartialImageEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseImageGenCallPartialImageEvent = &e
	case "response.output_text.annotation.added":
		var e ResponseOutputTextAnnotationAddedEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		r.ResponseOutputTextAnnotationAddedEvent = &e
	default:
		// unrecognize event
		return nil
	}

	return nil
}

// Emitted when there is a partial audio response.
type ResponseAudioDeltaEvent struct {
	// A chunk of Base64 encoded response audio bytes.
	Delta string `json:"delta"`

	// A sequence number for this chunk of the stream response.
	SequenceNumber int `json:"sequence_number"`
}

// Emitted when the audio response is complete.
type ResponseAudioDoneEvent struct {
	// The sequence number of the delta.
	SequenceNumber int `json:"sequence_number"`
}

// Emitted when there is a partial transcript of audio.
type ResponseAudioTranscriptDeltaEvent struct {
	// The partial transcript of the audio response.
	Delta string `json:"delta"`

	// The sequence number of this event.
	SequenceNumber int `json:"sequence_number"`
}

// Emitted when the full audio transcript is completed.
type ResponseAudioTranscriptDoneEvent struct {
	// The sequence number of this event.
	SequenceNumber int `json:"sequence_number"`
}

// Emitted when the model response is complete.
type ResponseCompletedEvent struct {
	// Properties of the completed response.
	Response Response `json:"response"`

	// The sequence number for this event.
	SequenceNumber int `json:"sequence_number"`
}

// Emitted when a new content part is added.
type ResponseContentPartAddedEvent struct {
	// The index of the content part that was added.
	ContentIndex int `json:"content_index"`

	// The ID of the output item that the content part was added to.
	ItemID string `json:"item_id"`

	// The index of the output item that the content part was added to.
	OutputIndex int `json:"output_index"`

	// The content part that was added.
	Part ResponseContentPartEventPart `json:"part"`

	// The sequence number of this event.
	SequenceNumber int `json:"sequence_number"`
}

type ResponseContentPartEventPart struct {
	Text    *ResponseOutputText    `json:"-"`
	Refusal *ResponseOutputRefusal `json:"-"`
}

func (r ResponseContentPartEventPart) MarshalJSON() ([]byte, error) {
	if r.Text != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseOutputText
		}{
			Type:               "output_text",
			ResponseOutputText: r.Text,
		})
	}
	if r.Refusal != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseOutputRefusal
		}{
			Type:                  "refusal",
			ResponseOutputRefusal: r.Refusal,
		})
	}
	return nil, fmt.Errorf("ResponseContentPartEventPart has no content")
}

func (r *ResponseContentPartEventPart) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "output_text":
		var t ResponseOutputText
		if err := json.Unmarshal(data, &t); err != nil {
			return err
		}
		r.Text = &t
	case "refusal":
		var ref ResponseOutputRefusal
		if err := json.Unmarshal(data, &ref); err != nil {
			return err
		}
		r.Refusal = &ref
	default:
		return fmt.Errorf("unknown ResponseContentPartEventPart type: %s", temp.Type)
	}

	return nil
}

// Emitted when a content part is done.
type ResponseContentPartDoneEvent struct {
	// The index of the content part that is done.
	ContentIndex int `json:"content_index"`

	// The ID of the output item that the content part was added to.
	ItemID string `json:"item_id"`

	// The index of the output item that the content part was added to.
	OutputIndex int `json:"output_index"`

	// The content part that is done.
	Part ResponseContentPartEventPart `json:"part"`

	// The sequence number of this event.
	SequenceNumber int `json:"sequence_number"`
}

// An event that is emitted when a response is created.
type ResponseCreatedEvent struct {
	// The response that was created.
	Response Response `json:"response"`

	// The sequence number for this event.
	SequenceNumber int `json:"sequence_number"`
}

// Emitted when an error occurs.
type ResponseErrorEvent struct {
	// The error code.
	Code *string `json:"code,omitempty"`

	// The error message.
	Message string `json:"message"`

	// The error parameter.
	Param *string `json:"param,omitempty"`

	// The sequence number of this event.
	SequenceNumber int `json:"sequence_number"`
}

// Emitted when there is a partial function-call arguments delta.
type ResponseFunctionCallArgumentsDeltaEvent struct {
	// The function-call arguments delta that is added.
	Delta string `json:"delta"`

	// The ID of the output item that the function-call arguments delta is added to.
	ItemID string `json:"item_id"`

	// The index of the output item that the function-call arguments delta is added to.
	OutputIndex int `json:"output_index"`

	// The sequence number of this event.
	SequenceNumber int `json:"sequence_number"`
}

// Emitted when function-call arguments are finalized.
type ResponseFunctionCallArgumentsDoneEvent struct {
	// The function-call arguments.
	Arguments string `json:"arguments"`

	// The ID of the item.
	ItemID string `json:"item_id"`

	// The index of the output item.
	OutputIndex int `json:"output_index"`

	// The sequence number of this event.
	SequenceNumber int `json:"sequence_number"`
}

// Emitted when the response is in progress.
type ResponseInProgressEvent struct {
	// The response that is in progress.
	Response Response `json:"response"`

	// The sequence number of this event.
	SequenceNumber int `json:"sequence_number"`
}

// An event that is emitted when a response fails.
type ResponseFailedEvent struct {
	// The response that failed.
	Response Response `json:"response"`

	// The sequence number of this event.
	SequenceNumber int `json:"sequence_number"`
}

// An event that is emitted when a response finishes as incomplete.
type ResponseIncompleteEvent struct {
	// The response that was incomplete.
	Response Response `json:"response"`

	// The sequence number of this event.
	SequenceNumber int `json:"sequence_number"`
}

// Emitted when a new output item is added.
type ResponseOutputItemAddedEvent struct {
	// The output item that was added.
	Item ResponseOutputItem `json:"item"`

	// The index of the output item that was added.
	OutputIndex int `json:"output_index"`

	// The sequence number of this event.
	SequenceNumber int `json:"sequence_number"`
}

// Emitted when an output item is marked done.
type ResponseOutputItemDoneEvent struct {
	// The output item that was marked done.
	Item ResponseOutputItem `json:"item"`

	// The index of the output item that was marked done.
	OutputIndex int `json:"output_index"`

	// The sequence number of this event.
	SequenceNumber int `json:"sequence_number"`
}

// Emitted when a new reasoning summary part is added.
type ResponseReasoningSummaryPartAddedEvent struct {
	// The ID of the item this summary part is associated with.
	ItemID string `json:"item_id"`

	// The index of the output item this summary part is associated with.
	OutputIndex int `json:"output_index"`

	// The summary part that was added.
	Part ResponseReasoningSummaryPartAddedEventPart `json:"part"`

	// The sequence number of this event.
	SequenceNumber int `json:"sequence_number"`

	// The index of the summary part within the reasoning summary.
	SummaryIndex int `json:"summary_index"`

	// The type of the event. Always `response.reasoning_summary_part.added`.
	Type string `json:"type"`
}

// The summary part that was added.
type ResponseReasoningSummaryPartAddedEventPart struct {
	// The text of the summary part.
	Text string `json:"text"`
}

// Emitted when a reasoning summary part is completed.
type ResponseReasoningSummaryPartDoneEvent struct {
	// The ID of the item this summary part is associated with.
	ItemID string `json:"item_id"`

	// The index of the output item this summary part is associated with.
	OutputIndex int `json:"output_index"`

	// The completed summary part.
	Part ResponseReasoningSummaryPartDoneEventPart `json:"part"`

	// The sequence number of this event.
	SequenceNumber int `json:"sequence_number"`

	// The index of the summary part within the reasoning summary.
	SummaryIndex int `json:"summary_index"`

	// The type of the event. Always `response.reasoning_summary_part.done`.
	Type string `json:"type"`
}

// The completed summary part.
type ResponseReasoningSummaryPartDoneEventPart struct {
	// The text of the summary part.
	Text string `json:"text"`
}

// Emitted when a delta is added to a reasoning summary text.
type ResponseReasoningSummaryTextDeltaEvent struct {
	// The text delta that was added to the summary.
	Delta string `json:"delta"`

	// The ID of the item this summary text delta is associated with.
	ItemID string `json:"item_id"`

	// The index of the output item this summary text delta is associated with.
	OutputIndex int `json:"output_index"`

	// The sequence number of this event.
	SequenceNumber int `json:"sequence_number"`

	// The index of the summary part within the reasoning summary.
	SummaryIndex int `json:"summary_index"`

	// The type of the event. Always `response.reasoning_summary_text.delta`.
	Type string `json:"type"`
}

// Emitted when a reasoning summary text is completed.
type ResponseReasoningSummaryTextDoneEvent struct {
	// The ID of the item this summary text is associated with.
	ItemID string `json:"item_id"`

	// The index of the output item this summary text is associated with.
	OutputIndex int `json:"output_index"`

	// The sequence number of this event.
	SequenceNumber int `json:"sequence_number"`

	// The index of the summary part within the reasoning summary.
	SummaryIndex int `json:"summary_index"`

	// The full text of the completed reasoning summary.
	Text string `json:"text"`

	// The type of the event. Always `response.reasoning_summary_text.done`.
	Type string `json:"type"`
}

// Emitted when a delta is added to a reasoning text.
type ResponseReasoningTextDeltaEvent struct {
	// The index of the reasoning content part this delta is associated with.
	ContentIndex int `json:"content_index"`

	// The text delta that was added to the reasoning content.
	Delta string `json:"delta"`

	// The ID of the item this reasoning text delta is associated with.
	ItemID string `json:"item_id"`

	// The index of the output item this reasoning text delta is associated with.
	OutputIndex int `json:"output_index"`

	// The sequence number of this event.
	SequenceNumber int `json:"sequence_number"`

	// The type of the event. Always `response.reasoning_text.delta`.
	Type string `json:"type"`
}

// Emitted when a reasoning text is completed.
type ResponseReasoningTextDoneEvent struct {
	// The index of the reasoning content part.
	ContentIndex int `json:"content_index"`

	// The ID of the item this reasoning text is associated with.
	ItemID string `json:"item_id"`

	// The index of the output item this reasoning text is associated with.
	OutputIndex int `json:"output_index"`

	// The sequence number of this event.
	SequenceNumber int `json:"sequence_number"`

	// The full text of the completed reasoning content.
	Text string `json:"text"`

	// The type of the event. Always `response.reasoning_text.done`.
	Type string `json:"type"`
}

// Emitted when there is a partial refusal text.
type ResponseRefusalDeltaEvent struct {
	// The index of the content part that the refusal text is added to.
	ContentIndex int `json:"content_index"`

	// The refusal text that is added.
	Delta string `json:"delta"`

	// The ID of the output item that the refusal text is added to.
	ItemID string `json:"item_id"`

	// The index of the output item that the refusal text is added to.
	OutputIndex int `json:"output_index"`

	// The sequence number of this event.
	SequenceNumber int `json:"sequence_number"`

	// The type of the event. Always `response.refusal.delta`.
	Type string `json:"type"`
}

// Emitted when refusal text is finalized.
type ResponseRefusalDoneEvent struct {
	// The index of the content part that the refusal text is finalized.
	ContentIndex int `json:"content_index"`

	// The ID of the output item that the refusal text is finalized.
	ItemID string `json:"item_id"`

	// The index of the output item that the refusal text is finalized.
	OutputIndex int `json:"output_index"`

	// The refusal text that is finalized.
	Refusal string `json:"refusal"`

	// The sequence number of this event.
	SequenceNumber int `json:"sequence_number"`

	// The type of the event. Always `response.refusal.done`.
	Type string `json:"type"`
}

// Emitted when there is an additional text delta.
type ResponseTextDeltaEvent struct {
	// The index of the content part that the text delta was added to.
	ContentIndex int `json:"content_index"`

	// The text delta that was added.
	Delta string `json:"delta"`

	// The ID of the output item that the text delta was added to.
	ItemID string `json:"item_id"`

	// The index of the output item that the text delta was added to.
	OutputIndex int `json:"output_index"`

	// The sequence number for this event.
	SequenceNumber int `json:"sequence_number"`

	// The type of the event. Always `response.output_text.delta`.
	Type string `json:"type"`
}

// Emitted when text content is finalized.
type ResponseTextDoneEvent struct {
	// The index of the content part that the text content is finalized.
	ContentIndex int `json:"content_index"`

	// The ID of the output item that the text content is finalized.
	ItemID string `json:"item_id"`

	// The index of the output item that the text content is finalized.
	OutputIndex int `json:"output_index"`

	// The sequence number for this event.
	SequenceNumber int `json:"sequence_number"`

	// The text content that is finalized.
	Text string `json:"text"`

	// The type of the event. Always `response.output_text.done`.
	Type string `json:"type"`
}

// Emitted when an image generation tool call has completed and the final image is
// available.
type ResponseImageGenCallCompletedEvent struct {
	// The unique identifier of the image generation item being processed.
	ItemID string `json:"item_id"`

	// The index of the output item in the response's output array.
	OutputIndex int `json:"output_index"`

	// The sequence number of this event.
	SequenceNumber int `json:"sequence_number"`

	// The type of the event. Always 'response.image_generation_call.completed'.
	Type string `json:"type"`
}

// Emitted when an image generation tool call is actively generating an image
// (intermediate state).
type ResponseImageGenCallGeneratingEvent struct {
	// The unique identifier of the image generation item being processed.
	ItemID string `json:"item_id"`

	// The index of the output item in the response's output array.
	OutputIndex int `json:"output_index"`

	// The sequence number of the image generation item being processed.
	SequenceNumber int `json:"sequence_number"`

	// The type of the event. Always 'response.image_generation_call.generating'.
	Type string `json:"type"`
}

// Emitted when an image generation tool call is in progress.
type ResponseImageGenCallInProgressEvent struct {
	// The unique identifier of the image generation item being processed.
	ItemID string `json:"item_id"`

	// The index of the output item in the response's output array.
	OutputIndex int `json:"output_index"`

	// The sequence number of the image generation item being processed.
	SequenceNumber int `json:"sequence_number"`

	// The type of the event. Always 'response.image_generation_call.in_progress'.
	Type string `json:"type"`
}

// Emitted when a partial image is available during image generation streaming.
type ResponseImageGenCallPartialImageEvent struct {
	// The unique identifier of the image generation item being processed.
	ItemID string `json:"item_id"`

	// The index of the output item in the response's output array.
	OutputIndex int `json:"output_index"`

	// Base64-encoded partial image data, suitable for rendering as an image.
	PartialImageB64 string `json:"partial_image_b64"`

	// 0-based index for the partial image (backend is 1-based, but this is 0-based for
	// the user).
	PartialImageIndex int `json:"partial_image_index"`

	// The sequence number of the image generation item being processed.
	SequenceNumber int `json:"sequence_number"`

	// The type of the event. Always 'response.image_generation_call.partial_image'.
	Type string `json:"type"`

	Size         string `json:"size"`          // Size of the partial image (e.g. "1024x768")
	OutputFormat string `json:"output_format"` // png, jpeg, etc.
}

// Emitted when an annotation is added to output text content.
type ResponseOutputTextAnnotationAddedEvent struct {
	// The annotation object being added. (See annotation schema for details.)
	Annotation interface{} `json:"annotation"`

	// The index of the annotation within the content part.
	AnnotationIndex int `json:"annotation_index"`

	// The index of the content part within the output item.
	ContentIndex int `json:"content_index"`

	// The unique identifier of the item to which the annotation is being added.
	ItemID string `json:"item_id"`

	// The index of the output item in the response's output array.
	OutputIndex int `json:"output_index"`

	// The sequence number of this event.
	SequenceNumber int `json:"sequence_number"`

	// The type of the event. Always 'response.output_text.annotation.added'.
	Type string `json:"type"`
}
