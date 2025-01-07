package llmsdk

import (
	"encoding/json"
	"fmt"
)

// AudioFormat loosely describes audio format. Some values (e.g., 'wav') denote containers;
// others (e.g., 'linear16') specify encoding only; cannot describe containers
// that can contain different audio encodings.
type AudioFormat string

const (
	AudioFormatWav      AudioFormat = "wav"
	AudioFormatMP3      AudioFormat = "mp3"
	AudioFormatLinear16 AudioFormat = "linear16"
	AudioFormatFLAC     AudioFormat = "flac"
	AudioFormatMulaw    AudioFormat = "mulaw"
	AudioFormatAlaw     AudioFormat = "alaw"
	AudioFormatAAC      AudioFormat = "aac"
	AudioFormatOpus     AudioFormat = "opus"
)

// Part represents a part of the message.
type Part struct {
	TextPart       *TextPart       `json:"-"`
	ImagePart      *ImagePart      `json:"-"`
	AudioPart      *AudioPart      `json:"-"`
	ToolCallPart   *ToolCallPart   `json:"-"`
	ToolResultPart *ToolResultPart `json:"-"`
}

type PartType string

const (
	PartTypeText       PartType = "text"
	PartTypeImage      PartType = "image"
	PartTypeAudio      PartType = "audio"
	PartTypeToolCall   PartType = "tool-call"
	PartTypeToolResult PartType = "tool-result"
)

func (p Part) Type() PartType {
	switch {
	case p.TextPart != nil:
		return PartTypeText
	case p.ImagePart != nil:
		return PartTypeImage
	case p.AudioPart != nil:
		return PartTypeAudio
	case p.ToolCallPart != nil:
		return PartTypeToolCall
	case p.ToolResultPart != nil:
		return PartTypeToolResult
	default:
		return ""
	}
}

// TextPart represents a part of the message that contains text.
type TextPart struct {
	Text string  `json:"text"`
	ID   *string `json:"id,omitempty"`
}

// ImagePart represents a part of the message that contains an image.
type ImagePart struct {
	MimeType  string  `json:"mime_type"`
	ImageData string  `json:"image_data"`
	Width     *int    `json:"width,omitempty"`
	Height    *int    `json:"height,omitempty"`
	ID        *string `json:"id,omitempty"`
}

// AudioPart represents a part of the message that contains an audio.
type AudioPart struct {
	AudioData  string      `json:"audio_data"`
	Format     AudioFormat `json:"format"`
	SampleRate *int        `json:"sample_rate,omitempty"`
	Channels   *int        `json:"channels,omitempty"`
	Transcript *string     `json:"transcript,omitempty"`
	ID         *string     `json:"id,omitempty"`
}

// ToolCallPart represents a part of the message that represents a call to a tool the model wants to use.
type ToolCallPart struct {
	ToolCallID string         `json:"tool_call_id"`
	ToolName   string         `json:"tool_name"`
	Args       map[string]any `json:"args"`
	ID         *string        `json:"id,omitempty"`
}

// ToolResultPart represents a part of the message that represents the result of a tool call.
type ToolResultPart struct {
	ToolCallID string `json:"tool_call_id"`
	ToolName   string `json:"tool_name"`
	Content    []Part `json:"content"`
	IsError    *bool  `json:"is_error,omitempty"`
}

// MarshalJSON implements custom JSON marshaling for Part
func (p Part) MarshalJSON() ([]byte, error) {
	if p.TextPart != nil {
		return json.Marshal(struct {
			Type PartType `json:"type"`
			*TextPart
		}{
			Type:     PartTypeText,
			TextPart: p.TextPart,
		})
	}
	if p.ImagePart != nil {
		return json.Marshal(struct {
			Type PartType `json:"type"`
			*ImagePart
		}{
			Type:      PartTypeImage,
			ImagePart: p.ImagePart,
		})
	}
	if p.AudioPart != nil {
		return json.Marshal(struct {
			Type PartType `json:"type"`
			*AudioPart
		}{
			Type:      PartTypeAudio,
			AudioPart: p.AudioPart,
		})
	}
	if p.ToolCallPart != nil {
		return json.Marshal(struct {
			Type PartType `json:"type"`
			*ToolCallPart
		}{
			Type:         PartTypeToolCall,
			ToolCallPart: p.ToolCallPart,
		})
	}
	if p.ToolResultPart != nil {
		return json.Marshal(struct {
			Type PartType `json:"type"`
			*ToolResultPart
		}{
			Type:           PartTypeToolResult,
			ToolResultPart: p.ToolResultPart,
		})
	}
	return nil, fmt.Errorf("part has no content")
}

// UnmarshalJSON implements custom JSON unmarshaling for Part
func (p *Part) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type PartType `json:"type"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "text":
		var t TextPart
		if err := json.Unmarshal(data, &t); err != nil {
			return err
		}
		p.TextPart = &t
	case "image":
		var i ImagePart
		if err := json.Unmarshal(data, &i); err != nil {
			return err
		}
		p.ImagePart = &i
	case "audio":
		var a AudioPart
		if err := json.Unmarshal(data, &a); err != nil {
			return err
		}
		p.AudioPart = &a
	case "tool-call":
		var tc ToolCallPart
		if err := json.Unmarshal(data, &tc); err != nil {
			return err
		}
		p.ToolCallPart = &tc
	case "tool-result":
		var tr struct {
			ToolCallID string            `json:"tool_call_id"`
			ToolName   string            `json:"tool_name"`
			Content    []json.RawMessage `json:"content"`
			IsError    *bool             `json:"is_error,omitempty"`
		}
		if err := json.Unmarshal(data, &tr); err != nil {
			return err
		}

		var content []Part
		for _, raw := range tr.Content {
			var part Part
			if err := json.Unmarshal(raw, &part); err != nil {
				return err
			}
			content = append(content, part)
		}

		p.ToolResultPart = &ToolResultPart{
			ToolCallID: tr.ToolCallID,
			ToolName:   tr.ToolName,
			Content:    content,
			IsError:    tr.IsError,
		}
	default:
		return fmt.Errorf("unknown part type: %s", temp.Type)
	}

	return nil
}

// NewTextPart creates a new text part
func NewTextPart(text string, id *string) Part {
	return Part{
		TextPart: &TextPart{
			Text: text,
			ID:   id,
		},
	}
}

// NewImagePart creates a new image part
func NewImagePart(mimeType, imageData string, width, height *int, id *string) Part {
	return Part{
		ImagePart: &ImagePart{
			MimeType:  mimeType,
			ImageData: imageData,
			Width:     width,
			Height:    height,
			ID:        id,
		},
	}
}

// NewAudioPart creates a new audio part
func NewAudioPart(audioData string, format AudioFormat, sampleRate, channels *int, transcript, id *string) Part {
	return Part{
		AudioPart: &AudioPart{
			AudioData:  audioData,
			Format:     format,
			SampleRate: sampleRate,
			Channels:   channels,
			Transcript: transcript,
			ID:         id,
		},
	}
}

// NewToolCallPart creates a new tool call part
func NewToolCallPart(toolCallID, toolName string, args map[string]any, id *string) Part {
	return Part{
		ToolCallPart: &ToolCallPart{
			ToolCallID: toolCallID,
			ToolName:   toolName,
			Args:       args,
			ID:         id,
		},
	}
}

// NewToolResultPart creates a new tool result part
func NewToolResultPart(toolCallID, toolName string, content []Part, isError *bool) Part {
	return Part{
		ToolResultPart: &ToolResultPart{
			ToolCallID: toolCallID,
			ToolName:   toolName,
			Content:    content,
			IsError:    isError,
		},
	}
}

// PartDelta represents delta parts used in partial updates.
type PartDelta struct {
	TextPartDelta     *TextPartDelta     `json:"-"`
	ToolCallPartDelta *ToolCallPartDelta `json:"-"`
	AudioPartDelta    *AudioPartDelta    `json:"-"`
}

// TextPartDelta represents a delta update for a text part, used in streaming or incremental updates of a message.
type TextPartDelta struct {
	Text string  `json:"text"`
	ID   *string `json:"id,omitempty"`
}

// ToolCallPartDelta represents a delta update for a tool call part, used in streaming of a tool invocation.
type ToolCallPartDelta struct {
	ToolCallID *string `json:"tool_call_id,omitempty"`
	ToolName   *string `json:"tool_name,omitempty"`
	Args       *string `json:"args,omitempty"`
	ID         *string `json:"id,omitempty"`
}

// AudioPartDelta represents a delta update for an audio part, used in streaming of an audio message.
type AudioPartDelta struct {
	AudioData  *string      `json:"audio_data,omitempty"`
	Format     *AudioFormat `json:"format,omitempty"`
	SampleRate *int         `json:"sample_rate,omitempty"`
	Channels   *int         `json:"channels,omitempty"`
	Transcript *string      `json:"transcript,omitempty"`
	ID         *string      `json:"id,omitempty"`
}

// MarshalJSON implements custom JSON marshaling for PartDelta
func (p PartDelta) MarshalJSON() ([]byte, error) {
	if p.TextPartDelta != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*TextPartDelta
		}{
			Type:          "text",
			TextPartDelta: p.TextPartDelta,
		})
	}
	if p.ToolCallPartDelta != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ToolCallPartDelta
		}{
			Type:              "tool-call",
			ToolCallPartDelta: p.ToolCallPartDelta,
		})
	}
	if p.AudioPartDelta != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*AudioPartDelta
		}{
			Type:           "audio",
			AudioPartDelta: p.AudioPartDelta,
		})
	}
	return nil, fmt.Errorf("part delta has no content")
}

// UnmarshalJSON implements custom JSON unmarshaling for PartDelta
func (p *PartDelta) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "text":
		var t TextPartDelta
		if err := json.Unmarshal(data, &t); err != nil {
			return err
		}
		p.TextPartDelta = &t
	case "tool-call":
		var tc ToolCallPartDelta
		if err := json.Unmarshal(data, &tc); err != nil {
			return err
		}
		p.ToolCallPartDelta = &tc
	case "audio":
		var a AudioPartDelta
		if err := json.Unmarshal(data, &a); err != nil {
			return err
		}
		p.AudioPartDelta = &a
	default:
		return fmt.Errorf("unknown part delta type: %s", temp.Type)
	}

	return nil
}

// Message represents a message in an LLM conversation history.
type Message struct {
	UserMessage      *UserMessage      `json:"-"`
	AssistantMessage *AssistantMessage `json:"-"`
	ToolMessage      *ToolMessage      `json:"-"`
}

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

func (m Message) Role() Role {
	switch {
	case m.UserMessage != nil:
		return RoleUser
	case m.AssistantMessage != nil:
		return RoleAssistant
	case m.ToolMessage != nil:
		return RoleTool
	}
	return ""
}

// UserMessage represents a message sent by the user.
type UserMessage struct {
	Content []Part `json:"content"`
}

// AssistantMessage represents a message generated by the model.
type AssistantMessage struct {
	Content []Part `json:"content"`
}

// ToolMessage represents tool result in the message history.
// Only ToolResultPart should be included in the content.
type ToolMessage struct {
	Content []Part `json:"content"`
}

// MarshalJSON implements custom JSON marshaling for Message
func (m Message) MarshalJSON() ([]byte, error) {
	if m.UserMessage != nil {
		return json.Marshal(struct {
			Role Role `json:"role"`
			*UserMessage
		}{
			Role:        RoleUser,
			UserMessage: m.UserMessage,
		})
	}
	if m.AssistantMessage != nil {
		return json.Marshal(struct {
			Role Role `json:"role"`
			*AssistantMessage
		}{
			Role:             RoleAssistant,
			AssistantMessage: m.AssistantMessage,
		})
	}
	if m.ToolMessage != nil {
		return json.Marshal(struct {
			Role Role `json:"role"`
			*ToolMessage
		}{
			Role:        RoleTool,
			ToolMessage: m.ToolMessage,
		})
	}
	return nil, fmt.Errorf("message has no content")
}

// UnmarshalJSON implements custom JSON unmarshaling for Message
func (m *Message) UnmarshalJSON(data []byte) error {
	var temp struct {
		Role    string            `json:"role"`
		Content []json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Parse content parts
	var content []Part
	for _, raw := range temp.Content {
		var part Part
		if err := json.Unmarshal(raw, &part); err != nil {
			return err
		}
		content = append(content, part)
	}

	switch temp.Role {
	case "user":
		m.UserMessage = &UserMessage{Content: content}
	case "assistant":
		m.AssistantMessage = &AssistantMessage{Content: content}
	case "tool":
		m.ToolMessage = &ToolMessage{Content: content}
	default:
		return fmt.Errorf("unknown message role: %s", temp.Role)
	}

	return nil
}

// NewUserMessage creates a new user message
func NewUserMessage(parts ...Part) Message {
	return Message{
		UserMessage: &UserMessage{Content: parts},
	}
}

// NewAssistantMessage creates a new assistant message
func NewAssistantMessage(parts ...Part) Message {
	return Message{
		AssistantMessage: &AssistantMessage{Content: parts},
	}
}

// NewToolMessage creates a new tool message
func NewToolMessage(parts ...Part) Message {
	return Message{
		ToolMessage: &ToolMessage{Content: parts},
	}
}

// Modality defines the modality of content (e.g., text or audio) in LLM responses.
type Modality string

const (
	ModalityText  Modality = "text"
	ModalityAudio Modality = "audio"
)

// ToolChoiceOption determines how the model should choose which tool to use.
type ToolChoiceOption struct {
	Auto     *ToolChoiceAuto     `json:"-"`
	None     *ToolChoiceNone     `json:"-"`
	Required *ToolChoiceRequired `json:"-"`
	Tool     *ToolChoiceTool     `json:"-"`
}

// ToolChoiceAuto means the model will automatically choose the tool to use or not use any tools.
type ToolChoiceAuto struct{}

// ToolChoiceNone means the model will not use any tools.
type ToolChoiceNone struct{}

// ToolChoiceRequired means the model will be forced to use a tool.
type ToolChoiceRequired struct{}

// ToolChoiceTool means the model will use the specified tool.
type ToolChoiceTool struct {
	ToolName string `json:"tool_name"`
}

// AsAuto returns the option as ToolChoiceAuto if it is one
func (t ToolChoiceOption) AsAuto() *ToolChoiceAuto {
	return t.Auto
}

// AsNone returns the option as ToolChoiceNone if it is one
func (t ToolChoiceOption) AsNone() *ToolChoiceNone {
	return t.None
}

// AsRequired returns the option as ToolChoiceRequired if it is one
func (t ToolChoiceOption) AsRequired() *ToolChoiceRequired {
	return t.Required
}

// AsTool returns the option as ToolChoiceTool if it is one
func (t ToolChoiceOption) AsTool() *ToolChoiceTool {
	return t.Tool
}

// MarshalJSON implements custom JSON marshaling for ToolChoiceOption
func (t ToolChoiceOption) MarshalJSON() ([]byte, error) {
	if t.Auto != nil {
		return json.Marshal(map[string]string{"type": "auto"})
	}
	if t.None != nil {
		return json.Marshal(map[string]string{"type": "none"})
	}
	if t.Required != nil {
		return json.Marshal(map[string]string{"type": "required"})
	}
	if t.Tool != nil {
		return json.Marshal(struct {
			Type     string `json:"type"`
			ToolName string `json:"tool_name"`
		}{
			Type:     "tool",
			ToolName: t.Tool.ToolName,
		})
	}
	return nil, fmt.Errorf("tool choice has no content")
}

// UnmarshalJSON implements custom JSON unmarshaling for ToolChoiceOption
func (t *ToolChoiceOption) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type     string `json:"type"`
		ToolName string `json:"tool_name,omitempty"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "auto":
		t.Auto = &ToolChoiceAuto{}
	case "none":
		t.None = &ToolChoiceNone{}
	case "required":
		t.Required = &ToolChoiceRequired{}
	case "tool":
		t.Tool = &ToolChoiceTool{ToolName: temp.ToolName}
	default:
		return fmt.Errorf("unknown tool choice type: %s", temp.Type)
	}

	return nil
}

// NewToolChoiceAuto creates an auto tool choice
func NewToolChoiceAuto() ToolChoiceOption {
	return ToolChoiceOption{Auto: &ToolChoiceAuto{}}
}

// NewToolChoiceNone creates a none tool choice
func NewToolChoiceNone() ToolChoiceOption {
	return ToolChoiceOption{None: &ToolChoiceNone{}}
}

// NewToolChoiceRequired creates a required tool choice
func NewToolChoiceRequired() ToolChoiceOption {
	return ToolChoiceOption{Required: &ToolChoiceRequired{}}
}

// NewToolChoiceTool creates a specific tool choice
func NewToolChoiceTool(toolName string) ToolChoiceOption {
	return ToolChoiceOption{Tool: &ToolChoiceTool{ToolName: toolName}}
}

// ResponseFormatOption represents the format that the model must output.
type ResponseFormatOption struct {
	Text *ResponseFormatText `json:"-"`
	JSON *ResponseFormatJSON `json:"-"`
}

// ResponseFormatText specifies that the model response should be in plain text format.
type ResponseFormatText struct{}

// ResponseFormatJSON specifies that the model response should be in JSON format adhering to a specified schema.
type ResponseFormatJSON struct {
	// The name of the schema.
	Name string `json:"name"`
	// The description of the schema.
	Description *string     `json:"description,omitempty"`
	Schema      *JSONSchema `json:"schema,omitempty"`
}

// AsText returns the format as ResponseFormatText if it is one
func (r ResponseFormatOption) AsText() *ResponseFormatText {
	return r.Text
}

// AsJSON returns the format as ResponseFormatJSON if it is one
func (r ResponseFormatOption) AsJSON() *ResponseFormatJSON {
	return r.JSON
}

// MarshalJSON implements custom JSON marshaling for ResponseFormatOption
func (r ResponseFormatOption) MarshalJSON() ([]byte, error) {
	if r.Text != nil {
		return json.Marshal(map[string]string{"type": "text"})
	}
	if r.JSON != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseFormatJSON
		}{
			Type:               "json",
			ResponseFormatJSON: r.JSON,
		})
	}
	return nil, fmt.Errorf("response format has no content")
}

// UnmarshalJSON implements custom JSON unmarshaling for ResponseFormatOption
func (r *ResponseFormatOption) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type        string      `json:"type"`
		Name        string      `json:"name,omitempty"`
		Description *string     `json:"description,omitempty"`
		Schema      *JSONSchema `json:"schema,omitempty"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case "text":
		r.Text = &ResponseFormatText{}
	case "json":
		r.JSON = &ResponseFormatJSON{
			Name:        temp.Name,
			Description: temp.Description,
			Schema:      temp.Schema,
		}
	default:
		return fmt.Errorf("unknown response format type: %s", temp.Type)
	}

	return nil
}

// NewResponseFormatText creates a text response format
func NewResponseFormatText() ResponseFormatOption {
	return ResponseFormatOption{Text: &ResponseFormatText{}}
}

// NewResponseFormatJSON creates a JSON response format
func NewResponseFormatJSON(name string, description *string, schema *JSONSchema) ResponseFormatOption {
	return ResponseFormatOption{
		JSON: &ResponseFormatJSON{
			Name:        name,
			Description: description,
			Schema:      schema,
		},
	}
}

// LanguageModelCapability represents a metadata property that describes the capability of the model.
type LanguageModelCapability string

const (
	CapabilityStructuredOutput       LanguageModelCapability = "structured-output"
	CapabilityFunctionCalling        LanguageModelCapability = "function-calling"
	CapabilityStructuredOutputStrict LanguageModelCapability = "structured-output-strict"
	CapabilityAudioInput             LanguageModelCapability = "audio-input"
	CapabilityAudioOutput            LanguageModelCapability = "audio-output"
	CapabilityImageInput             LanguageModelCapability = "image-input"
	CapabilityImageOutput            LanguageModelCapability = "image-output"
)

// ContentDelta represents a delta update in a message's content, enabling partial streaming updates in LLM responses.
type ContentDelta struct {
	Index int       `json:"index"`
	Part  PartDelta `json:"part"`
}

// JSONSchema represents a JSON schema.
type JSONSchema map[string]any

// Tool represents a tool that can be used by the model.
type Tool struct {
	// The name of the tool.
	Name string `json:"name"`
	// A description of the tool.
	Description string `json:"description"`
	// The JSON schema of the parameters that the tool accepts. The type must be "object".
	Parameters JSONSchema `json:"parameters"`
}

// ModelTokensDetails represents the token usage details of the model.
type ModelTokensDetails struct {
	TextTokens        *int `json:"text_tokens,omitempty"`
	CachedTextTokens  *int `json:"cached_text_tokens,omitempty"`
	AudioTokens       *int `json:"audio_tokens,omitempty"`
	CachedAudioTokens *int `json:"cached_audio_tokens,omitempty"`
	ImageTokens       *int `json:"image_tokens,omitempty"`
	CachedImageTokens *int `json:"cached_image_tokens,omitempty"`
}

// ModelUsage represents the token usage of the model.
type ModelUsage struct {
	InputTokens         int                 `json:"input_tokens"`
	OutputTokens        int                 `json:"output_tokens"`
	InputTokensDetails  *ModelTokensDetails `json:"input_tokens_details,omitempty"`
	OutputTokensDetails *ModelTokensDetails `json:"output_tokens_details,omitempty"`
}

// ModelResponse represents the response generated by the model.
type ModelResponse struct {
	Content []Part      `json:"content"`
	Usage   *ModelUsage `json:"usage,omitempty"`
	// The cost of the response.
	Cost *float64 `json:"cost,omitempty"`
}

// PartialModelResponse represents a partial response from the language model, useful for streaming output via async generator.
type PartialModelResponse struct {
	Delta *ContentDelta `json:"delta,omitempty"`
	Usage *ModelUsage   `json:"usage,omitempty"`
}

// LanguageModelInput defines the input parameters for the language model completion.
type LanguageModelInput struct {
	// A system prompt is a way of providing context and instructions to the model
	SystemPrompt *string `json:"system_prompt,omitempty"`
	// A list of messages comprising the conversation so far.
	Messages []Message `json:"messages"`
	// Definitions of tools that the model may use.
	Tools          []Tool                `json:"tools,omitempty"`
	ToolChoice     *ToolChoiceOption     `json:"tool_choice,omitempty"`
	ResponseFormat *ResponseFormatOption `json:"response_format,omitempty"`
	// The maximum number of tokens that can be generated in the chat completion.
	MaxTokens *int64 `json:"max_tokens,omitempty"`
	// Amount of randomness injected into the response. Ranges from 0.0 to 1.0
	Temperature *float64 `json:"temperature,omitempty"`
	// An alternative to sampling with temperature, called nucleus sampling, where the model considers the results of the tokens with top_p probability mass. Ranges from 0.0 to 1.0
	TopP *float64 `json:"top_p,omitempty"`
	// Only sample from the top K options for each subsequent token. Used to remove 'long tail' low probability responses. Ranges from 0.0 to 1.0
	TopK *float64 `json:"top_k,omitempty"`
	// Positive values penalize new tokens based on whether they appear in the text so far, increasing the model's likelihood to talk about new topics.
	PresencePenalty *float64 `json:"presence_penalty,omitempty"`
	// Positive values penalize new tokens based on their existing frequency in the text so far, decreasing the model's likelihood to repeat the same line verbatim.
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`
	// The seed (integer), if set and supported by the model, to enable deterministic results.
	Seed *int64 `json:"seed,omitempty"`
	// The modalities that the model should support.
	Modalities []Modality `json:"modalities,omitempty"`
	// A set of key/value pairs that store additional information about the request. This is forwarded to the model provider if supported.
	Metadata map[string]string `json:"metadata,omitempty"`
	// Extra options that the model may support.
	Extra map[string]any `json:"extra,omitempty"`
}

// LanguageModelPricing represents a metadata property that describes the pricing of the model.
type LanguageModelPricing struct {
	// The cost in USD per single text token for input.
	InputCostPerTextToken *float64 `json:"input_cost_per_text_token,omitempty"`
	// The cost in USD per single cached text token for input.
	InputCostPerCachedTextToken *float64 `json:"input_cost_per_cached_text_token,omitempty"`
	// The cost in USD per single text token for output.
	OutputCostPerTextToken *float64 `json:"output_cost_per_text_token,omitempty"`
	// The cost in USD per single audio token for input.
	InputCostPerAudioToken *float64 `json:"input_cost_per_audio_token,omitempty"`
	// The cost in USD per single cached audio token for input.
	InputCostPerCachedAudioToken *float64 `json:"input_cost_per_cached_audio_token,omitempty"`
	// The cost in USD per single audio token for output.
	OutputCostPerAudioToken *float64 `json:"output_cost_per_audio_token,omitempty"`
	// The cost in USD per single image token for input.
	InputCostPerImageToken *float64 `json:"input_cost_per_image_token,omitempty"`
	// The cost in USD per single cached image token for input.
	InputCostPerCachedImageToken *float64 `json:"input_cost_per_cached_image_token,omitempty"`
	// The cost in USD per single image token for output.
	OutputCostPerImageToken *float64 `json:"output_cost_per_image_token,omitempty"`
}
