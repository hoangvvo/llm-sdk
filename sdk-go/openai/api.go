package openai

import (
	"encoding/json"
	"fmt"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/ptr"
)

// https://platform.openai.com/docs/api-reference/chat

// ChatCompletionCreateParams represents the parameters for creating a chat completion
type ChatCompletionCreateParams struct {
	// A list of messages comprising the conversation so far. Depending on the
	// model you use, different message types (modalities) are supported, like
	// text, images, and audio.
	Messages []ChatCompletionMessageParam `json:"messages"`

	// Model ID used to generate the response, like `gpt-4o` or `o3`. OpenAI
	// offers a wide range of models with different capabilities,
	// performance characteristics, and price points. Refer to the
	// model guide to browse and compare available models.
	Model string `json:"model"`

	// Parameters for audio output. Required when audio output is requested
	// with `modalities: ["audio"]`.
	// Learn more: https://platform.openai.com/docs/guides/audio
	Audio *ChatCompletionAudioParam `json:"audio,omitempty"`

	// Number between -2.0 and 2.0. Positive values penalize new tokens based
	// on their existing frequency in the text so far, decreasing the
	// model's likelihood to repeat the same line verbatim.
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`

	// An upper bound for the number of tokens that can be generated for a
	// completion, including visible output tokens and reasoning tokens.
	MaxCompletionTokens *uint32 `json:"max_completion_tokens,omitempty"`

	// Output types that you would like the model to generate. Most models are
	// capable of generating text, which is the default:
	//
	// ["text"]
	//
	// The `gpt-4o-audio-preview` model can also be used to
	// generate audio. To request that
	// this model generate both text and audio responses, you can use:
	//
	// ["text", "audio"]
	Modalities []OpenAIModality `json:"modalities,omitempty"`

	// Number between -2.0 and 2.0. Positive values penalize new tokens based
	// on whether they appear in the text so far, increasing the model's
	// likelihood to talk about new topics.
	PresencePenalty *float64 `json:"presence_penalty,omitempty"`

	// An object specifying the format that the model must output.
	//
	// Setting to `{ "type": "json_schema", "json_schema": {...} }` enables
	// Structured Outputs which ensures the model will match your supplied
	// JSON schema. Learn more in the
	// Structured Outputs guide.
	//
	// Setting to `{ "type": "json_object" }` enables the older JSON mode,
	// which ensures the message the model generates is valid JSON. Using
	// `json_schema` is preferred for models that support it.
	ResponseFormat *OpenAIResponseFormat `json:"response_format,omitempty"`

	// This feature is in Beta. If specified, our system will make a best
	// effort to sample deterministically, such that repeated requests with
	// the same `seed` and parameters should return the same result.
	// Determinism is not guaranteed, and you should refer to the
	// `system_fingerprint` response parameter to monitor changes
	// in the backend.
	Seed *int64 `json:"seed,omitempty"`

	// If set to true, the model response data will be streamed to the client
	// as it is generated using
	// server-sent events.
	// See the
	// Streaming section below
	// for more information, along with the
	// streaming responses
	// guide for more information on how to handle the streaming events.
	Stream *bool `json:"stream,omitempty"`

	// Options for streaming response. Only set this when you set `stream:
	// true`.
	StreamOptions *ChatCompletionStreamOptions `json:"stream_options,omitempty"`

	// What sampling temperature to use, between 0 and 2. Higher values like
	// 0.8 will make the output more random, while lower values like 0.2
	// will make it more focused and deterministic. We generally recommend
	// altering this or `top_p` but not both.
	Temperature *float64 `json:"temperature,omitempty"`

	// Controls which (if any) tool is called by the model. `none` means the
	// model will not call any tool and instead generates a message. `auto`
	// means the model can pick between generating a message or calling one
	// or more tools. `required` means the model must call one or more
	// tools. Specifying a particular tool via `{"type": "function",
	// "function": {"name": "my_function"}}` forces the model to
	// call that tool.
	//
	// `none` is the default when no tools are present. `auto` is the default
	// if tools are present.
	ToolChoice *ChatCompletionToolChoiceOption `json:"tool_choice,omitempty"`

	// A list of tools the model may call. You can provide either
	// custom tools
	// or function tools.
	Tools []ChatCompletionTool `json:"tools,omitempty"`

	// An alternative to sampling with temperature, called nucleus sampling,
	// where the model considers the results of the tokens with `top_p`
	// probability mass. So 0.1 means only the tokens comprising the top
	// 10% probability mass are considered.
	//
	// We generally recommend altering this or `temperature` but not both.
	TopP *float64 `json:"top_p,omitempty"`

	Extra map[string]any `json:"-"`
}

// OpenAIModality represents output types that the model can generate
type OpenAIModality string

const (
	OpenAIModalityText  OpenAIModality = "text"
	OpenAIModalityAudio OpenAIModality = "audio"
)

// Developer-provided instructions that the model should follow, regardless of
// messages sent by the user. With o1 models and newer, `developer` messages
// replace the previous `system` messages.
type ChatCompletionMessageParam struct {
	Developer *ChatCompletionDeveloperMessageParam `json:"-"`
	System    *ChatCompletionSystemMessageParam    `json:"-"`
	User      *ChatCompletionUserMessageParam      `json:"-"`
	Assistant *ChatCompletionAssistantMessageParam `json:"-"`
	Tool      *ChatCompletionToolMessageParam      `json:"-"`
}

func (m ChatCompletionMessageParam) MarshalJSON() ([]byte, error) {
	if m.Developer != nil {
		return json.Marshal(struct {
			Role string `json:"role"`
			*ChatCompletionDeveloperMessageParam
		}{
			Role:                                "developer",
			ChatCompletionDeveloperMessageParam: m.Developer,
		})
	}
	if m.System != nil {
		return json.Marshal(struct {
			Role string `json:"role"`
			*ChatCompletionSystemMessageParam
		}{
			Role:                             "system",
			ChatCompletionSystemMessageParam: m.System,
		})
	}
	if m.User != nil {
		return json.Marshal(struct {
			Role string `json:"role"`
			*ChatCompletionUserMessageParam
		}{
			Role:                           "user",
			ChatCompletionUserMessageParam: m.User,
		})
	}
	if m.Assistant != nil {
		return json.Marshal(struct {
			Role string `json:"role"`
			*ChatCompletionAssistantMessageParam
		}{
			Role:                                "assistant",
			ChatCompletionAssistantMessageParam: m.Assistant,
		})
	}
	if m.Tool != nil {
		return json.Marshal(struct {
			Role string `json:"role"`
			*ChatCompletionToolMessageParam
		}{
			Role:                           "tool",
			ChatCompletionToolMessageParam: m.Tool,
		})
	}
	return nil, fmt.Errorf("message has no content")
}

func (m *ChatCompletionMessageParam) UnmarshalJSON(data []byte) error {
	var temp struct {
		Role string `json:"role"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Role {
	case "developer":
		var d ChatCompletionDeveloperMessageParam
		if err := json.Unmarshal(data, &d); err != nil {
			return err
		}
		m.Developer = &d
	case "system":
		var s ChatCompletionSystemMessageParam
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		m.System = &s
	case "user":
		var u ChatCompletionUserMessageParam
		if err := json.Unmarshal(data, &u); err != nil {
			return err
		}
		m.User = &u
	case "assistant":
		var a ChatCompletionAssistantMessageParam
		if err := json.Unmarshal(data, &a); err != nil {
			return err
		}
		m.Assistant = &a
	case "tool":
		var t ChatCompletionToolMessageParam
		if err := json.Unmarshal(data, &t); err != nil {
			return err
		}
		m.Tool = &t
	default:
		return fmt.Errorf("unknown message role: %s", temp.Role)
	}

	return nil
}

// Developer-provided instructions that the model should follow, regardless of
// messages sent by the user. With o1 models and newer, `developer` messages
// replace the previous `system` messages.
type ChatCompletionDeveloperMessageParam struct {
	// The contents of the developer message.
	Content []DeveloperContentPart `json:"content"`

	// An optional name for the participant. Provides the model information to
	// differentiate between participants of the same role.
	Name *string `json:"name,omitempty"`
}

// DeveloperContentPart represents content parts for developer messages
type DeveloperContentPart struct {
	Text *ChatCompletionContentPartText `json:"-"`
}

func (d DeveloperContentPart) MarshalJSON() ([]byte, error) {
	if d.Text != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ChatCompletionContentPartText
		}{
			Type:                          "text",
			ChatCompletionContentPartText: d.Text,
		})
	}
	return nil, fmt.Errorf("developer content part has no content")
}

func (d *DeveloperContentPart) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "text":
		var t ChatCompletionContentPartText
		if err := json.Unmarshal(data, &t); err != nil {
			return err
		}
		d.Text = &t
	default:
		return fmt.Errorf("unknown developer content part type: %s", temp.Type)
	}

	return nil
}

// Developer-provided instructions that the model should follow, regardless of
// messages sent by the user. With o1 models and newer, use `developer`
// messages for this purpose instead.
type ChatCompletionSystemMessageParam struct {
	// The contents of the system message.
	Content []SystemContentPart `json:"content"`
}

// SystemContentPart represents content parts for system messages
type SystemContentPart struct {
	Text *ChatCompletionContentPartText `json:"-"`
}

func (s SystemContentPart) MarshalJSON() ([]byte, error) {
	if s.Text != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ChatCompletionContentPartText
		}{
			Type:                          "text",
			ChatCompletionContentPartText: s.Text,
		})
	}
	return nil, fmt.Errorf("system content part has no content")
}

func (s *SystemContentPart) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "text":
		var t ChatCompletionContentPartText
		if err := json.Unmarshal(data, &t); err != nil {
			return err
		}
		s.Text = &t
	default:
		return fmt.Errorf("unknown system content part type: %s", temp.Type)
	}

	return nil
}

// Messages sent by an end user, containing prompts or additional context
// information.
type ChatCompletionUserMessageParam struct {
	// The contents of the user message.
	Content []ChatCompletionContentPart `json:"content"`

	// An optional name for the participant. Provides the model information to
	// differentiate between participants of the same role.
	Name *string `json:"name,omitempty"`
}

// Messages sent by the model in response to user messages.
type ChatCompletionAssistantMessageParam struct {
	// Data about a previous audio response from the model.
	// Learn more: https://platform.openai.com/docs/guides/audio
	Audio *ChatCompletionAssistantMessageParamAudio `json:"audio,omitempty"`

	// The contents of the assistant message. Required unless `tool_calls` or
	// `function_call` is specified.
	Content []AssistantContentPart `json:"content,omitempty"`

	// An optional name for the participant. Provides the model information to
	// differentiate between participants of the same role.
	Name *string `json:"name,omitempty"`

	// The refusal message by the assistant.
	Refusal *string `json:"refusal,omitempty"`

	// The tool calls generated by the model, such as function calls.
	ToolCalls []ChatCompletionMessageToolCall `json:"tool_calls,omitempty"`
}

type ChatCompletionToolMessageParam struct {
	// The contents of the tool message.
	Content []ToolContentPart `json:"content"`

	// Tool call that this message is responding to.
	ToolCallID string `json:"tool_call_id"`
}

// ToolContentPart represents content parts for tool messages
type ToolContentPart struct {
	Text *ChatCompletionContentPartText `json:"-"`
}

func (t ToolContentPart) MarshalJSON() ([]byte, error) {
	if t.Text != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ChatCompletionContentPartText
		}{
			Type:                          "text",
			ChatCompletionContentPartText: t.Text,
		})
	}
	return nil, fmt.Errorf("tool content part has no content")
}

func (t *ToolContentPart) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "text":
		var txt ChatCompletionContentPartText
		if err := json.Unmarshal(data, &txt); err != nil {
			return err
		}
		t.Text = &txt
	default:
		return fmt.Errorf("unknown tool content part type: %s", temp.Type)
	}

	return nil
}

// Learn about text inputs: https://platform.openai.com/docs/guides/text-generation
type ChatCompletionContentPart struct {
	Text       *ChatCompletionContentPartText       `json:"-"`
	Image      *ChatCompletionContentPartImage      `json:"-"`
	InputAudio *ChatCompletionContentPartInputAudio `json:"-"`
	File       *ChatCompletionContentPartFile       `json:"-"`
}

func (c ChatCompletionContentPart) MarshalJSON() ([]byte, error) {
	if c.Text != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ChatCompletionContentPartText
		}{
			Type:                          "text",
			ChatCompletionContentPartText: c.Text,
		})
	}
	if c.Image != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ChatCompletionContentPartImage
		}{
			Type:                           "image_url",
			ChatCompletionContentPartImage: c.Image,
		})
	}
	if c.InputAudio != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ChatCompletionContentPartInputAudio
		}{
			Type:                                "input_audio",
			ChatCompletionContentPartInputAudio: c.InputAudio,
		})
	}
	if c.File != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ChatCompletionContentPartFile
		}{
			Type:                          "file",
			ChatCompletionContentPartFile: c.File,
		})
	}
	return nil, fmt.Errorf("content part has no content")
}

func (c *ChatCompletionContentPart) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "text":
		var t ChatCompletionContentPartText
		if err := json.Unmarshal(data, &t); err != nil {
			return err
		}
		c.Text = &t
	case "image_url":
		var i ChatCompletionContentPartImage
		if err := json.Unmarshal(data, &i); err != nil {
			return err
		}
		c.Image = &i
	case "input_audio":
		var a ChatCompletionContentPartInputAudio
		if err := json.Unmarshal(data, &a); err != nil {
			return err
		}
		c.InputAudio = &a
	case "file":
		var f ChatCompletionContentPartFile
		if err := json.Unmarshal(data, &f); err != nil {
			return err
		}
		c.File = &f
	default:
		return fmt.Errorf("unknown content part type: %s", temp.Type)
	}

	return nil
}

// Learn about text inputs: https://platform.openai.com/docs/guides/text-generation
type ChatCompletionContentPartText struct {
	// The text content.
	Text string `json:"text"`
}

// Learn about image inputs: https://platform.openai.com/docs/guides/vision
type ChatCompletionContentPartImage struct {
	ImageURL ChatCompletionContentPartImageImageURL `json:"image_url"`
}

type ChatCompletionContentPartImageImageURL struct {
	// Either a URL of the image or the base64 encoded image data.
	URL string `json:"url"`

	// Specifies the detail level of the image. Learn more in the
	// Vision guide: https://platform.openai.com/docs/guides/vision#low-or-high-fidelity-image-understanding
	Detail *ImageDetail `json:"detail,omitempty"`
}

// ImageDetail represents the detail level of an image
type ImageDetail string

const (
	ImageDetailAuto ImageDetail = "auto"
	ImageDetailLow  ImageDetail = "low"
	ImageDetailHigh ImageDetail = "high"
)

// Learn about audio inputs: https://platform.openai.com/docs/guides/audio
type ChatCompletionContentPartInputAudio struct {
	InputAudio ChatCompletionContentPartInputAudioInputAudio `json:"input_audio"`
}

type ChatCompletionContentPartInputAudioInputAudio struct {
	// Base64 encoded audio data.
	Data string `json:"data"`

	// The format of the encoded audio data. Currently supports "wav" and
	// "mp3".
	Format AudioInputFormat `json:"format"`
}

// AudioInputFormat represents audio input formats
type AudioInputFormat string

const (
	AudioInputFormatWav AudioInputFormat = "wav"
	AudioInputFormatMp3 AudioInputFormat = "mp3"
)

// Learn about file inputs for text generation: https://platform.openai.com/docs/guides/text
type ChatCompletionContentPartFile struct {
	File ChatCompletionContentPartFileFile `json:"file"`
}

type ChatCompletionContentPartFileFile struct {
	// The base64 encoded file data, used when passing the file to the model as
	// a string.
	FileData *string `json:"file_data,omitempty"`

	// The ID of an uploaded file to use as input.
	FileID *string `json:"file_id,omitempty"`

	// The name of the file, used when passing the file to the model as a
	// string.
	Filename *string `json:"filename,omitempty"`
}

// AssistantContentPart represents content parts for assistant messages
type AssistantContentPart struct {
	Text    *ChatCompletionContentPartText    `json:"-"`
	Refusal *ChatCompletionContentPartRefusal `json:"-"`
}

func (a AssistantContentPart) MarshalJSON() ([]byte, error) {
	if a.Text != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ChatCompletionContentPartText
		}{
			Type:                          "text",
			ChatCompletionContentPartText: a.Text,
		})
	}
	if a.Refusal != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ChatCompletionContentPartRefusal
		}{
			Type:                             "refusal",
			ChatCompletionContentPartRefusal: a.Refusal,
		})
	}
	return nil, fmt.Errorf("assistant content part has no content")
}

func (a *AssistantContentPart) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "text":
		var t ChatCompletionContentPartText
		if err := json.Unmarshal(data, &t); err != nil {
			return err
		}
		a.Text = &t
	case "refusal":
		var r ChatCompletionContentPartRefusal
		if err := json.Unmarshal(data, &r); err != nil {
			return err
		}
		a.Refusal = &r
	default:
		return fmt.Errorf("unknown assistant content part type: %s", temp.Type)
	}

	return nil
}

type ChatCompletionContentPartRefusal struct {
	// The refusal message generated by the model.
	Refusal string `json:"refusal"`
}

type ChatCompletionAssistantMessageParamAudio struct {
	// Unique identifier for a previous audio response from the model.
	ID string `json:"id"`
}

// Parameters for audio output. Required when audio output is requested with
// `modalities: ["audio"]`.
// Learn more: https://platform.openai.com/docs/guides/audio
type ChatCompletionAudioParam struct {
	// Specifies the output audio format. Must be one of `wav`, `mp3`, `flac`,
	// `opus`, or `pcm16`.
	Format AudioOutputFormat `json:"format"`

	// The voice the model uses to respond. Supported voices are `alloy`,
	// `ash`, `ballad`, `coral`, `echo`, `fable`, `nova`, `onyx`, `sage`,
	// and `shimmer`.
	Voice Voice `json:"voice"`
}

// AudioOutputFormat represents audio output formats
type AudioOutputFormat string

const (
	AudioOutputFormatWav   AudioOutputFormat = "wav"
	AudioOutputFormatAac   AudioOutputFormat = "aac"
	AudioOutputFormatMp3   AudioOutputFormat = "mp3"
	AudioOutputFormatFlac  AudioOutputFormat = "flac"
	AudioOutputFormatOpus  AudioOutputFormat = "opus"
	AudioOutputFormatPcm16 AudioOutputFormat = "pcm16"
)

// Voice represents available voices
type Voice string

const (
	VoiceAlloy   Voice = "alloy"
	VoiceAsh     Voice = "ash"
	VoiceBallad  Voice = "ballad"
	VoiceCoral   Voice = "coral"
	VoiceEcho    Voice = "echo"
	VoiceSage    Voice = "sage"
	VoiceShimmer Voice = "shimmer"
	VoiceVerse   Voice = "verse"
)

// Controls which (if any) tool is called by the model. `none` means the model
// will not call any tool and instead generates a message. `auto` means the
// model can pick between generating a message or calling one or more tools.
// `required` means the model must call one or more tools. Specifying a
// particular tool via `{"type": "function", "function": {"name":
// "my_function"}}` forces the model to call that tool.
//
// `none` is the default when no tools are present. `auto` is the default if
// tools are present.
type ChatCompletionToolChoiceOption struct {
	None     *bool                            `json:"-"`
	Auto     *bool                            `json:"-"`
	Required *bool                            `json:"-"`
	Allowed  *ChatCompletionAllowedToolChoice `json:"-"`
	Named    *ChatCompletionNamedToolChoice   `json:"-"`
}

func (t ChatCompletionToolChoiceOption) MarshalJSON() ([]byte, error) {
	if t.None != nil {
		return json.Marshal("none")
	}
	if t.Auto != nil {
		return json.Marshal("auto")
	}
	if t.Required != nil {
		return json.Marshal("required")
	}
	if t.Allowed != nil {
		return json.Marshal(t.Allowed)
	}
	if t.Named != nil {
		return json.Marshal(t.Named)
	}
	return nil, fmt.Errorf("tool choice has no content")
}

func (t *ChatCompletionToolChoiceOption) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		switch str {
		case "none":
			t.None = ptr.To(true)
		case "auto":
			t.Auto = ptr.To(true)
		case "required":
			t.Required = ptr.To(true)
		default:
			return fmt.Errorf("unknown tool choice string: %s", str)
		}
		return nil
	}

	// Try to unmarshal as object
	var temp struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "allowed_tools":
		var a ChatCompletionAllowedToolChoice
		if err := json.Unmarshal(data, &a); err != nil {
			return err
		}
		t.Allowed = &a
	case "function":
		var n ChatCompletionNamedToolChoice
		if err := json.Unmarshal(data, &n); err != nil {
			return err
		}
		t.Named = &n
	default:
		return fmt.Errorf("unknown tool choice type: %s", temp.Type)
	}

	return nil
}

// Constrains the tools available to the model to a pre-defined set.
type ChatCompletionAllowedToolChoice struct {
	// Constrains the tools available to the model to a pre-defined set.
	AllowedTools ChatCompletionAllowedTools `json:"allowed_tools"`

	// Allowed tool configuration type. Always `allowed_tools`.
	Type string `json:"type"`
}

// Constrains the tools available to the model to a pre-defined set.
type ChatCompletionAllowedTools struct {
	// Constrains the tools available to the model to a pre-defined set.
	//
	// `auto` allows the model to pick from among the allowed tools and
	// generate a message.
	//
	// `required` requires the model to call one or more of the allowed tools.
	Mode AllowedToolsMode `json:"mode"`

	// A list of tool definitions that the model should be allowed to call.
	//
	// For the Chat Completions API, the list of tool definitions might look
	// like:
	//
	// ```json
	// [
	//   { "type": "function", "function": { "name": "get_weather" } },
	//   { "type": "function", "function": { "name": "get_time" } }
	// ]
	// ```
	Tools []map[string]any `json:"tools"`
}

// AllowedToolsMode represents the mode for allowed tools
type AllowedToolsMode string

const (
	AllowedToolsModeAuto     AllowedToolsMode = "auto"
	AllowedToolsModeRequired AllowedToolsMode = "required"
)

// Specifies a tool the model should use. Use to force the model to call a
// specific function.
type ChatCompletionNamedToolChoice struct {
	Function ChatCompletionNamedToolChoiceFunction `json:"function"`

	// For function calling, the type is always `function`.
	Type string `json:"type"`
}

type ChatCompletionNamedToolChoiceFunction struct {
	// The name of the function to call.
	Name string `json:"name"`
}

// A call to a function tool created by the model.
type ChatCompletionMessageToolCall struct {
	Function *ChatCompletionMessageFunctionToolCall `json:"-"`
}

func (t ChatCompletionMessageToolCall) MarshalJSON() ([]byte, error) {
	if t.Function != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ChatCompletionMessageFunctionToolCall
		}{
			Type:                                  "function",
			ChatCompletionMessageFunctionToolCall: t.Function,
		})
	}
	return nil, fmt.Errorf("tool call has no content")
}

func (t *ChatCompletionMessageToolCall) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "function":
		var f ChatCompletionMessageFunctionToolCall
		if err := json.Unmarshal(data, &f); err != nil {
			return err
		}
		t.Function = &f
	default:
		return fmt.Errorf("unknown tool call type: %s", temp.Type)
	}

	return nil
}

// A call to a function tool created by the model.
type ChatCompletionMessageFunctionToolCall struct {
	// The ID of the tool call.
	ID string `json:"id"`

	// The function that the model called.
	Function ChatCompletionMessageFunctionToolCallFunction `json:"function"`
}

type ChatCompletionMessageFunctionToolCallFunction struct {
	// The arguments to call the function with, as generated by the model in
	// JSON format. Note that the model does not always generate valid
	// JSON, and may hallucinate parameters not defined by your function
	// schema. Validate the arguments in your code before calling your
	// function.
	Arguments string `json:"arguments"`

	// The name of the function to call.
	Name string `json:"name"`
}

// A function tool that can be used to generate a response.
type ChatCompletionTool struct {
	Function *ChatCompletionFunctionTool `json:"-"`
}

func (t ChatCompletionTool) MarshalJSON() ([]byte, error) {
	if t.Function != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ChatCompletionFunctionTool
		}{
			Type:                       "function",
			ChatCompletionFunctionTool: t.Function,
		})
	}
	return nil, fmt.Errorf("tool has no content")
}

func (t *ChatCompletionTool) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "function":
		var f ChatCompletionFunctionTool
		if err := json.Unmarshal(data, &f); err != nil {
			return err
		}
		t.Function = &f
	default:
		return fmt.Errorf("unknown tool type: %s", temp.Type)
	}

	return nil
}

// A function tool that can be used to generate a response.
type ChatCompletionFunctionTool struct {
	Function FunctionDefinition `json:"function"`
}

type FunctionDefinition struct {
	// The name of the function to be called. Must be a-z, A-Z, 0-9, or contain
	// underscores and dashes, with a maximum length of 64.
	Name string `json:"name"`

	// A description of what the function does, used by the model to choose
	// when and how to call the function.
	Description *string `json:"description,omitempty"`

	// The parameters the functions accepts, described as a JSON Schema object.
	// See the guide for examples,
	// and the
	// JSON Schema reference for
	// documentation about the format.
	//
	// Omitting `parameters` defines a function with an empty parameter list.
	Parameters *FunctionParameters `json:"parameters,omitempty"`

	// Whether to enable strict schema adherence when generating the function
	// call. If set to true, the model will follow the exact schema defined
	// in the `parameters` field. Only a subset of JSON Schema is supported
	// when `strict` is `true`. Learn more about Structured Outputs in the
	// function calling guide.
	Strict *bool `json:"strict,omitempty"`
}

// The parameters the functions accepts, described as a JSON Schema object. See
// the guide for examples,
// and the
// JSON Schema reference for
// documentation about the format.
//
// Omitting `parameters` defines a function with an empty parameter list.
type FunctionParameters = llmsdk.JSONSchema

type OpenAIResponseFormat struct {
	// Default response format. Used to generate text responses.
	Text *bool `json:"-"`

	// JSON Schema response format. Used to generate structured JSON responses.
	// Learn more about Structured Outputs: https://platform.openai.com/docs/guides/structured-outputs
	JsonSchema *ResponseFormatJSONSchema `json:"-"`

	// JSON object response format. An older method of generating JSON
	// responses. Using `json_schema` is recommended for models that
	// support it. Note that the model will not generate JSON without a
	// system or user message instructing it to do so.
	JsonObject *bool `json:"-"`
}

func (r OpenAIResponseFormat) MarshalJSON() ([]byte, error) {
	if r.Text != nil {
		return json.Marshal(map[string]string{"type": "text"})
	}
	if r.JsonSchema != nil {
		return json.Marshal(struct {
			Type       string                   `json:"type"`
			JsonSchema ResponseFormatJSONSchema `json:"json_schema"`
		}{
			Type:       "json_schema",
			JsonSchema: *r.JsonSchema,
		})
	}
	if r.JsonObject != nil {
		return json.Marshal(map[string]string{"type": "json_object"})
	}
	return nil, fmt.Errorf("response format has no content")
}

func (r *OpenAIResponseFormat) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type       string                    `json:"type"`
		JsonSchema *ResponseFormatJSONSchema `json:"json_schema,omitempty"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "text":
		r.Text = ptr.To(true)
	case "json_schema":
		r.JsonSchema = temp.JsonSchema
	case "json_object":
		r.JsonObject = ptr.To(true)
	default:
		return fmt.Errorf("unknown response format type: %s", temp.Type)
	}

	return nil
}

// JSON Schema response format. Used to generate structured JSON responses.
// Learn more about Structured Outputs: https://platform.openai.com/docs/guides/structured-outputs
type ResponseFormatJSONSchema struct {
	// Structured Outputs configuration options, including a JSON Schema.
	JsonSchema ResponseFormatJSONSchemaJSONSchema `json:"json_schema"`
}

// Structured Outputs configuration options, including a JSON Schema.
type ResponseFormatJSONSchemaJSONSchema struct {
	// The name of the response format. Must be a-z, A-Z, 0-9, or contain
	// underscores and dashes, with a maximum length of 64.
	Name string `json:"name"`

	// A description of what the response format is for, used by the model to
	// determine how to respond in the format.
	Description *string `json:"description,omitempty"`

	// The schema for the response format, described as a JSON Schema object.
	// Learn how to build JSON schemas here: https://json-schema.org/
	Schema *llmsdk.JSONSchema `json:"schema,omitempty"`

	// Whether to enable strict schema adherence when generating the output. If
	// set to true, the model will always follow the exact schema defined
	// in the `schema` field. Only a subset of JSON Schema is supported
	// when `strict` is `true`. To learn more, read the
	// Structured Outputs guide: https://platform.openai.com/docs/guides/structured-outputs
	Strict *bool `json:"strict,omitempty"`
}

// Options for streaming response. Only set this when you set `stream: true`.
type ChatCompletionStreamOptions struct {
	// When true, stream obfuscation will be enabled. Stream obfuscation adds
	// random characters to an `obfuscation` field on streaming delta
	// events to normalize payload sizes as a mitigation to certain
	// side-channel attacks. These obfuscation fields are included by
	// default, but add a small amount of overhead to the data stream. You
	// can set `include_obfuscation` to false to optimize for bandwidth if
	// you trust the network links between your application and the OpenAI
	// API.
	IncludeObfuscation *bool `json:"include_obfuscation,omitempty"`

	// If set, an additional chunk will be streamed before the `data: [DONE]`
	// message. The `usage` field on this chunk shows the token usage
	// statistics for the entire request, and the `choices` field will
	// always be an empty array.
	//
	// All other chunks will also include a `usage` field, but with a null
	// value. **NOTE:** If the stream is interrupted, you may not receive
	// the final usage chunk which contains the total token usage for the
	// request.
	IncludeUsage *bool `json:"include_usage,omitempty"`
}

// Represents a chat completion response returned by model, based on the
// provided input.
type ChatCompletion struct {
	// A unique identifier for the chat completion.
	ID string `json:"id"`

	// A list of chat completion choices. Can be more than one if `n` is
	// greater than 1.
	Choices []ChatCompletionChoice `json:"choices"`

	// The Unix timestamp (in seconds) of when the chat completion was created.
	Created int64 `json:"created"`

	// The model used for the chat completion.
	Model string `json:"model"`

	// The object type, which is always `chat.completion`.
	Object string `json:"object"`

	// Usage statistics for the completion request.
	Usage *CompletionUsage `json:"usage,omitempty"`
}

type ChatCompletionChoice struct {
	// The reason the model stopped generating tokens. This will be `stop` if
	// the model hit a natural stop point or a provided stop sequence,
	// `length` if the maximum number of tokens specified in the request
	// was reached, `content_filter` if content was omitted due to a flag
	// from our content filters, `tool_calls` if the model called a tool,
	// or `function_call` (deprecated) if the model called a function.
	FinishReason FinishReason `json:"finish_reason"`

	// The index of the choice in the list of choices.
	Index int32 `json:"index"`

	// A chat completion message generated by the model.
	Message ChatCompletionMessage `json:"message"`
}

// FinishReason represents why the model stopped generating
type FinishReason string

const (
	FinishReasonStop          FinishReason = "stop"
	FinishReasonLength        FinishReason = "length"
	FinishReasonToolCalls     FinishReason = "tool_calls"
	FinishReasonContentFilter FinishReason = "content_filter"
	FinishReasonFunctionCall  FinishReason = "function_call"
)

// Usage statistics for the completion request.
type CompletionUsage struct {
	// Number of tokens in the generated completion.
	CompletionTokens uint32 `json:"completion_tokens"`

	// Number of tokens in the prompt.
	PromptTokens uint32 `json:"prompt_tokens"`

	// Total number of tokens used in the request (prompt + completion).
	TotalTokens uint32 `json:"total_tokens"`

	// Breakdown of tokens used in a completion.
	CompletionTokensDetails *CompletionTokensDetails `json:"completion_tokens_details,omitempty"`

	// Breakdown of tokens used in the prompt.
	PromptTokensDetails *PromptTokensDetails `json:"prompt_tokens_details,omitempty"`
}

// Breakdown of tokens used in a completion.
type CompletionTokensDetails struct {
	// When using Predicted Outputs, the number of tokens in the prediction
	// that appeared in the completion.
	AcceptedPredictionTokens *int32 `json:"accepted_prediction_tokens,omitempty"`

	// Audio input tokens generated by the model.
	AudioTokens *int32 `json:"audio_tokens,omitempty"`

	// Tokens generated by the model for reasoning.
	ReasoningTokens *int32 `json:"reasoning_tokens,omitempty"`

	// When using Predicted Outputs, the number of tokens in the prediction
	// that did not appear in the completion. However, like reasoning
	// tokens, these tokens are still counted in the total completion
	// tokens for purposes of billing, output, and context window limits.
	RejectedPredictionTokens *int32 `json:"rejected_prediction_tokens,omitempty"`
}

// Breakdown of tokens used in the prompt.
type PromptTokensDetails struct {
	// Audio input tokens present in the prompt.
	AudioTokens *int32 `json:"audio_tokens,omitempty"`

	// Cached tokens present in the prompt.
	CachedTokens *int32 `json:"cached_tokens,omitempty"`
}

// A chat completion message generated by the model.
type ChatCompletionMessage struct {
	// The contents of the message.
	Content *string `json:"content"`

	// The refusal message generated by the model.
	Refusal *string `json:"refusal"`

	// The role of the author of this message.
	Role string `json:"role"`

	// If the audio output modality is requested, this object contains data
	// about the audio response from the model.
	// Learn more: https://platform.openai.com/docs/guides/audio
	Audio *ChatCompletionAudio `json:"audio,omitempty"`

	// The tool calls generated by the model, such as function calls.
	ToolCalls []ChatCompletionMessageToolCall `json:"tool_calls,omitempty"`
}

// If the audio output modality is requested, this object contains data about
// the audio response from the model.
// Learn more: https://platform.openai.com/docs/guides/audio
type ChatCompletionAudio struct {
	// Unique identifier for this audio response.
	ID string `json:"id"`

	// Base64 encoded audio bytes generated by the model, in the format
	// specified in the request.
	Data string `json:"data"`

	// The Unix timestamp (in seconds) for when this audio response will no
	// longer be accessible on the server for use in multi-turn
	// conversations.
	ExpiresAt int64 `json:"expires_at"`

	// Transcript of the audio generated by the model.
	Transcript string `json:"transcript"`
}

// Represents a streamed chunk of a chat completion response returned by the
// model, based on the provided input.
// Learn more: https://platform.openai.com/docs/guides/streaming-responses
type ChatCompletionChunk struct {
	// A unique identifier for the chat completion. Each chunk has the same ID.
	ID string `json:"id"`

	// A list of chat completion choices. Can contain more than one elements if
	// `n` is greater than 1. Can also be empty for the last chunk if you
	// set `stream_options: {"include_usage": true}`.
	Choices []ChatCompletionChunkChoice `json:"choices"`

	// The Unix timestamp (in seconds) of when the chat completion was created.
	// Each chunk has the same timestamp.
	Created int64 `json:"created"`

	// The model to generate the completion.
	Model string `json:"model"`

	// The object type, which is always `chat.completion.chunk`.
	Object string `json:"object"`

	// An optional field that will only be present when you set
	// `stream_options: {"include_usage": true}` in your request. When present,
	// it contains a null value **except for the last chunk** which
	// contains the token usage statistics for the entire request.
	//
	// **NOTE:** If the stream is interrupted or cancelled, you may not receive
	// the final usage chunk which contains the total token usage for the
	// request.
	Usage *CompletionUsage `json:"usage,omitempty"`
}

type ChatCompletionChunkChoice struct {
	// A chat completion delta generated by streamed model responses.
	Delta ChatCompletionChunkChoiceDelta `json:"delta"`

	// The reason the model stopped generating tokens. This will be `stop` if
	// the model hit a natural stop point or a provided stop sequence,
	// `length` if the maximum number of tokens specified in the request
	// was reached, `content_filter` if content was omitted due to a flag
	// from our content filters, `tool_calls` if the model called a tool,
	// or `function_call` (deprecated) if the model called a function.
	FinishReason *FinishReason `json:"finish_reason"`

	// The index of the choice in the list of choices.
	Index int32 `json:"index"`
}

// A chat completion delta generated by streamed model responses.
type ChatCompletionChunkChoiceDelta struct {
	// The contents of the chunk message.
	Content *string `json:"content,omitempty"`

	// The refusal message generated by the model.
	Refusal *string `json:"refusal,omitempty"`

	// The role of the author of this message.
	Role *DeltaRole `json:"role,omitempty"`

	ToolCalls []ChatCompletionChunkChoiceDeltaToolCall `json:"tool_calls,omitempty"`

	// @undocumented
	Audio *ChatCompletionChunkChoiceDeltaAudio `json:"audio,omitempty"`
}

// DeltaRole represents the role in a delta message
type DeltaRole string

const (
	DeltaRoleDeveloper DeltaRole = "developer"
	DeltaRoleSystem    DeltaRole = "system"
	DeltaRoleUser      DeltaRole = "user"
	DeltaRoleAssistant DeltaRole = "assistant"
	DeltaRoleTool      DeltaRole = "tool"
)

type ChatCompletionChunkChoiceDeltaToolCall struct {
	Index int `json:"index"`

	// The ID of the tool call.
	ID *string `json:"id,omitempty"`

	Function *ChatCompletionChunkChoiceDeltaToolCallFunction `json:"function,omitempty"`

	// The type of the tool. Currently, only `function` is supported.
	Type *string `json:"type,omitempty"`
}

type ChatCompletionChunkChoiceDeltaToolCallFunction struct {
	// The arguments to call the function with, as generated by the model in
	// JSON format. Note that the model does not always generate valid
	// JSON, and may hallucinate parameters not defined by your function
	// schema. Validate the arguments in your code before calling your
	// function.
	Arguments *string `json:"arguments,omitempty"`

	// The name of the function to call.
	Name *string `json:"name,omitempty"`
}

// @undocumented
type ChatCompletionChunkChoiceDeltaAudio struct {
	ID         *string `json:"id,omitempty"`
	Data       *string `json:"data,omitempty"`
	Transcript *string `json:"transcript,omitempty"`
}
