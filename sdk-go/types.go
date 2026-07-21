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
	SourcePart     *SourcePart     `json:"-"`
	ToolCallPart   *ToolCallPart   `json:"-"`
	ToolResultPart *ToolResultPart `json:"-"`
	ReasoningPart  *ReasoningPart  `json:"-"`
}

type PartType string

const (
	PartTypeText       PartType = "text"
	PartTypeImage      PartType = "image"
	PartTypeAudio      PartType = "audio"
	PartTypeSource     PartType = "source"
	PartTypeToolCall   PartType = "tool-call"
	PartTypeToolResult PartType = "tool-result"
	PartTypeReasoning  PartType = "reasoning"
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
	case p.ReasoningPart != nil:
		return PartTypeReasoning
	default:
		return ""
	}
}

// TextPart represents a part of the message that contains text.
type TextPart struct {
	Text      string     `json:"text"`
	Citations []Citation `json:"citations,omitempty"`
	// An opaque provider signature used to preserve text-part continuity when returning the part to the same provider.
	Signature *string `json:"signature,omitempty"`
}

// ImagePart represents a part of the message that contains an image.
type ImagePart struct {
	// The MIME type of the image. E.g. "image/jpeg", "image/png".
	MimeType string `json:"mime_type"`
	// The base64-encoded image data.
	Data string `json:"data"`
	// The width of the image in pixels.
	Width *int `json:"width,omitempty"`
	// The height of the image in pixels.
	Height *int `json:"height,omitempty"`
	// ID of the image part, if applicable
	ID *string `json:"id,omitempty"`
}

// AudioPart represents a part of the message that contains an audio.
type AudioPart struct {
	// The base64-encoded audio data.
	Data   string      `json:"data"`
	Format AudioFormat `json:"format"`
	// The sample rate of the audio. E.g. 44100, 48000.
	SampleRate *int `json:"sample_rate,omitempty"`
	// The number of channels of the audio. E.g. 1, 2.
	Channels *int `json:"channels,omitempty"`
	// The transcript of the audio.
	Transcript *string `json:"transcript,omitempty"`
	// The ID of the part, if applicable.
	ID *string `json:"id,omitempty"`
}

// SourcePart represents a part of the message that contains a source with structured content.
// It will be used for citation for supported models.
type SourcePart struct {
	// The URL or identifier of the document.
	Source string `json:"source"`
	// The title of the document.
	Title string `json:"title"`
	// The content of the document.
	Content []Part `json:"content"`
}

// ToolCallPart represents a part of the message that represents a call to a tool the model wants to use.
type ToolCallPart struct {
	// The ID of the tool call, used to match the tool result with the tool call.
	ToolCallID string   `json:"tool_call_id"`
	Call       ToolCall `json:"call"`
	// The provider-specific signature used to preserve reasoning/tool continuity.
	Signature *string `json:"signature,omitempty"`
	// The ID of the part, if applicable.
	// This is different from ToolCallID which is used to match tool results.
	ID *string `json:"id,omitempty"`
}

// ToolResultPart represents a part of the message that represents the result of a tool call.
type ToolResultPart struct {
	// The ID of the tool call from previous assistant message.
	ToolCallID string     `json:"tool_call_id"`
	Result     ToolResult `json:"result"`
	// Status is the terminal status of the tool call.
	Status ToolResultStatus `json:"status"`
}

// ToolResultStatus is the terminal status of a tool call.
type ToolResultStatus string

const (
	ToolResultStatusCompleted ToolResultStatus = "completed"
	ToolResultStatusFailed    ToolResultStatus = "failed"
	ToolResultStatusCancelled ToolResultStatus = "cancelled"
)

type ToolCall struct {
	Function  *FunctionToolCall  `json:"-"`
	WebSearch *WebSearchToolCall `json:"-"`
}

type FunctionToolCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type WebSearchToolCallStatus string

const (
	WebSearchToolCallStatusInProgress WebSearchToolCallStatus = "in_progress"
	WebSearchToolCallStatusSearching  WebSearchToolCallStatus = "searching"
	WebSearchToolCallStatusCompleted  WebSearchToolCallStatus = "completed"
	WebSearchToolCallStatusFailed     WebSearchToolCallStatus = "failed"
)

type WebSearchAction struct {
	Type    string   `json:"type"`
	Queries []string `json:"queries,omitempty"`
	URL     string   `json:"url,omitempty"`
	Pattern string   `json:"pattern,omitempty"`
}

type WebSearchToolCall struct {
	Action *WebSearchAction         `json:"action,omitempty"`
	Status *WebSearchToolCallStatus `json:"status,omitempty"`
}

type ToolResult struct {
	Function  *FunctionToolResult  `json:"-"`
	WebSearch *WebSearchToolResult `json:"-"`
}

type FunctionToolResult struct {
	Name    string `json:"name"`
	Content []Part `json:"content"`
}

type WebSearchSource struct {
	URL       string  `json:"url"`
	Title     *string `json:"title,omitempty"`
	PageAge   *string `json:"page_age,omitempty"`
	Signature *string `json:"signature,omitempty"`
}

type WebSearchToolResult struct {
	Sources   []WebSearchSource `json:"sources"`
	ErrorCode *string           `json:"error_code,omitempty"`
}

func (c ToolCall) MarshalJSON() ([]byte, error) {
	if c.Function != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*FunctionToolCall
		}{Type: "function", FunctionToolCall: c.Function})
	}
	if c.WebSearch != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*WebSearchToolCall
		}{Type: "web_search", WebSearchToolCall: c.WebSearch})
	}
	return nil, fmt.Errorf("tool call has no content")
}

func (c *ToolCall) UnmarshalJSON(data []byte) error {
	var tag struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &tag); err != nil {
		return err
	}
	switch tag.Type {
	case "function":
		var value FunctionToolCall
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		c.Function = &value
	case "web_search":
		var value WebSearchToolCall
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		c.WebSearch = &value
	default:
		return fmt.Errorf("unknown tool call type: %s", tag.Type)
	}
	return nil
}

func (r ToolResult) MarshalJSON() ([]byte, error) {
	if r.Function != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*FunctionToolResult
		}{Type: "function", FunctionToolResult: r.Function})
	}
	if r.WebSearch != nil {
		result := *r.WebSearch
		if result.Sources == nil {
			result.Sources = []WebSearchSource{}
		}
		return json.Marshal(struct {
			Type string `json:"type"`
			*WebSearchToolResult
		}{Type: "web_search", WebSearchToolResult: &result})
	}
	return nil, fmt.Errorf("tool result has no content")
}

func (r *ToolResult) UnmarshalJSON(data []byte) error {
	var tag struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &tag); err != nil {
		return err
	}
	switch tag.Type {
	case "function":
		var value FunctionToolResult
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		r.Function = &value
	case "web_search":
		var value WebSearchToolResult
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		r.WebSearch = &value
	default:
		return fmt.Errorf("unknown tool result type: %s", tag.Type)
	}
	return nil
}

// ReasoningPart represents part of the message that represents the model reasoning.
type ReasoningPart struct {
	// The reasoning text content
	Text string `json:"text"`
	//  The reasoning internal signature
	Signature *string `json:"signature,omitempty"`
	// The ID of the reasoning part, if applicable.
	ID *string `json:"id,omitempty"`
}

// Citation represents a citation to a source document.
type Citation struct {
	// The URL or identifier of the document being cited.
	Source string `json:"source"`
	// The title of the document being cited.
	Title *string `json:"title,omitempty"`
	// The text snippet from the document being cited.
	CitedText *string `json:"cited_text,omitempty"`
	// The start index of the document content part being cited.
	StartIndex *int `json:"start_index,omitempty"`
	// The end index of the document content part being cited.
	EndIndex *int `json:"end_index,omitempty"`
	// An opaque provider signature used to preserve citation continuity when returning it to the same provider.
	Signature *string `json:"signature,omitempty"`
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
	if p.SourcePart != nil {
		return json.Marshal(struct {
			Type PartType `json:"type"`
			*SourcePart
		}{
			Type:       PartTypeSource,
			SourcePart: p.SourcePart,
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
	if p.ReasoningPart != nil {
		return json.Marshal(struct {
			Type PartType `json:"type"`
			*ReasoningPart
		}{
			Type:          PartTypeReasoning,
			ReasoningPart: p.ReasoningPart,
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
	case "source":
		var s SourcePart
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		p.SourcePart = &s
	case "tool-call":
		var tc ToolCallPart
		if err := json.Unmarshal(data, &tc); err != nil {
			return err
		}
		p.ToolCallPart = &tc
	case "tool-result":
		var tr ToolResultPart
		if err := json.Unmarshal(data, &tr); err != nil {
			return err
		}
		p.ToolResultPart = &tr
	case "reasoning":
		var r ReasoningPart
		if err := json.Unmarshal(data, &r); err != nil {
			return err
		}
		p.ReasoningPart = &r
	default:
		return fmt.Errorf("unknown part type: %s", temp.Type)
	}

	return nil
}

// PartDelta represents delta parts used in partial updates.
type PartDelta struct {
	TextPartDelta       *TextPartDelta       `json:"-"`
	ToolCallPartDelta   *ToolCallPartDelta   `json:"-"`
	ToolResultPartDelta *ToolResultPartDelta `json:"-"`
	ImagePartDelta      *ImagePartDelta      `json:"-"`
	AudioPartDelta      *AudioPartDelta      `json:"-"`
	ReasoningPartDelta  *ReasoningPartDelta  `json:"-"`
}

// TextPartDelta represents a delta update for a text part, used in streaming or incremental updates of a message.
type TextPartDelta struct {
	Text      string         `json:"text"`
	Citation  *CitationDelta `json:"citation,omitempty"`
	Signature *string        `json:"signature,omitempty"`
}

// CitationDelta represents a delta update for a citation part, used in streaming of citation messages.
type CitationDelta struct {
	Source     *string `json:"source,omitempty"`
	Title      *string `json:"title,omitempty"`
	CitedText  *string `json:"cited_text,omitempty"`
	StartIndex *int    `json:"start_index,omitempty"`
	EndIndex   *int    `json:"end_index,omitempty"`
	Signature  *string `json:"signature,omitempty"`
}

func (c CitationDelta) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string `json:"type"`
		*CitationDelta
	}{
		Type:          "citation",
		CitationDelta: &c,
	})
}

// ToolCallPartDelta represents a delta update for a tool call part, used in streaming of a tool invocation.
type ToolCallPartDelta struct {
	ToolCallID *string       `json:"tool_call_id,omitempty"`
	Call       ToolCallDelta `json:"call"`
	Signature  *string       `json:"signature,omitempty"`
	ID         *string       `json:"id,omitempty"`
}

type ToolCallDelta struct {
	Function  *FunctionToolCallDelta  `json:"-"`
	WebSearch *WebSearchToolCallDelta `json:"-"`
}

type FunctionToolCallDelta struct {
	Name *string `json:"name,omitempty"`
	Args *string `json:"args,omitempty"`
}

type WebSearchToolCallDelta struct {
	Action *WebSearchAction         `json:"action,omitempty"`
	Status *WebSearchToolCallStatus `json:"status,omitempty"`
}

func (c ToolCallDelta) MarshalJSON() ([]byte, error) {
	if c.Function != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*FunctionToolCallDelta
		}{Type: "function", FunctionToolCallDelta: c.Function})
	}
	if c.WebSearch != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*WebSearchToolCallDelta
		}{Type: "web_search", WebSearchToolCallDelta: c.WebSearch})
	}
	return nil, fmt.Errorf("tool call delta has no content")
}

func (c *ToolCallDelta) UnmarshalJSON(data []byte) error {
	var tag struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &tag); err != nil {
		return err
	}
	switch tag.Type {
	case "function":
		var value FunctionToolCallDelta
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		c.Function = &value
	case "web_search":
		var value WebSearchToolCallDelta
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		c.WebSearch = &value
	default:
		return fmt.Errorf("unknown tool call delta type: %s", tag.Type)
	}
	return nil
}

type ToolResultPartDelta struct {
	ToolCallID string           `json:"tool_call_id"`
	Result     ToolResult       `json:"result"`
	Status     ToolResultStatus `json:"status"`
}

// ImagePartDelta represents a delta update for an image part, used in streaming of an image message.
type ImagePartDelta struct {
	// MimeType is the MIME type of the image. E.g. "image/jpeg", "image/png".
	MimeType *string `json:"mime_type,omitempty"`
	// Data is the base64-encoded image data.
	Data *string `json:"data,omitempty"`
	// Width is the width of the image in pixels.
	Width *int `json:"width,omitempty"`
	// Height is the height of the image in pixels.
	Height *int `json:"height,omitempty"`
	// ID of the image part, if applicable
	ID *string `json:"id,omitempty"`
}

// AudioPartDelta represents a delta update for an audio part, used in streaming of an audio message.
type AudioPartDelta struct {
	// Data is the base64-encoded audio data.
	Data       *string      `json:"data,omitempty"`
	Format     *AudioFormat `json:"format,omitempty"`
	SampleRate *int         `json:"sample_rate,omitempty"`
	Channels   *int         `json:"channels,omitempty"`
	Transcript *string      `json:"transcript,omitempty"`
	ID         *string      `json:"id,omitempty"`
}

// ReasoningPartDelta represents a delta update for a reasoning part, used in streaming of reasoning messages.
type ReasoningPartDelta struct {
	Text      string  `json:"text,omitempty"`
	Signature *string `json:"signature,omitempty"`
	ID        *string `json:"id,omitempty"`
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
	if p.ToolResultPartDelta != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ToolResultPartDelta
		}{Type: "tool-result", ToolResultPartDelta: p.ToolResultPartDelta})
	}
	if p.ImagePartDelta != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ImagePartDelta
		}{
			Type:           "image",
			ImagePartDelta: p.ImagePartDelta,
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
	if p.ReasoningPartDelta != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ReasoningPartDelta
		}{
			Type:               "reasoning",
			ReasoningPartDelta: p.ReasoningPartDelta,
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
	case "tool-result":
		var tr ToolResultPartDelta
		if err := json.Unmarshal(data, &tr); err != nil {
			return err
		}
		p.ToolResultPartDelta = &tr
	case "audio":
		var a AudioPartDelta
		if err := json.Unmarshal(data, &a); err != nil {
			return err
		}
		p.AudioPartDelta = &a
	case "image":
		var i ImagePartDelta
		if err := json.Unmarshal(data, &i); err != nil {
			return err
		}
		p.ImagePartDelta = &i
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
		Role    string `json:"role"`
		Content []Part `json:"content"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Role {
	case "user":
		m.UserMessage = &UserMessage{Content: temp.Content}
	case "assistant":
		m.AssistantMessage = &AssistantMessage{Content: temp.Content}
	case "tool":
		m.ToolMessage = &ToolMessage{Content: temp.Content}
	default:
		return fmt.Errorf("unknown message role: %s", temp.Role)
	}

	return nil
}

// Modality defines the modality of content (e.g., text or audio) in LLM responses.
type Modality string

const (
	ModalityText  Modality = "text"
	ModalityImage Modality = "image"
	ModalityAudio Modality = "audio"
)

// ToolChoiceOption determines how the model should choose which tool to use.
// - "auto" The model will automatically choose the tool to use or not use any tools.
// - "none" The model will not use any tools.
// - "required" The model will be forced to use a tool.
// - { type: "tool", toolName: "toolName" } The model will use the specified tool.
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

// AudioOptions represents options for audio generation.
type AudioOptions struct {
	// The desired audio format.
	Format *AudioFormat `json:"format,omitempty"`
	// The provider-specific voice ID to use for audio generation.
	Voice *string `json:"voice,omitempty"`
	// The language code for the audio generation
	LanguageCode *string `json:"language_code,omitempty"`
}

// Options for reasoning generation.
type ReasoningOptions struct {
	// Whether to enable reasoning output.
	Enabled bool `json:"enabled"`
	// Specify the budget tokens for reasoning generation.
	BudgetTokens *uint32 `json:"budget_tokens,omitempty"`
}

// LanguageModelCapabilities describes the capabilities supported by the model.
type LanguageModelCapabilities struct {
	TextInput        bool `json:"text_input"`
	TextOutput       bool `json:"text_output"`
	ImageInput       bool `json:"image_input"`
	ImageOutput      bool `json:"image_output"`
	AudioInput       bool `json:"audio_input"`
	AudioOutput      bool `json:"audio_output"`
	FunctionCalling  bool `json:"function_calling"`
	StructuredOutput bool `json:"structured_output"`
	Citation         bool `json:"citation"`
	Reasoning        bool `json:"reasoning"`
}

// ContentDelta represents a delta update in a message's content, enabling partial streaming updates in LLM responses.
type ContentDelta struct {
	Index int       `json:"index"`
	Part  PartDelta `json:"part"`
}

// JSONSchema represents a JSON schema.
type JSONSchema map[string]any

// Tool represents a tool that can be used by the model.
type Tool struct {
	FunctionTool  *FunctionTool  `json:"-"`
	WebSearchTool *WebSearchTool `json:"-"`
}

type ToolType string

const (
	ToolTypeFunction  ToolType = "function"
	ToolTypeWebSearch ToolType = "web_search"
)

func (t Tool) Type() ToolType {
	switch {
	case t.FunctionTool != nil:
		return ToolTypeFunction
	case t.WebSearchTool != nil:
		return ToolTypeWebSearch
	default:
		return ""
	}
}

// FunctionTool represents a client-executed function tool that can be used by the model.
type FunctionTool struct {
	// The name of the tool.
	Name string `json:"name"`
	// A description of the tool.
	Description string `json:"description"`
	// The JSON schema of the parameters that the tool accepts. The type must be "object".
	Parameters JSONSchema `json:"parameters"`
}

// WebSearchTool represents a provider-hosted web search tool.
type WebSearchTool struct {
	// Restricts search results to these domains when supported by the provider.
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	// An approximate user location used to localize web search results.
	UserLocation *WebSearchUserLocation `json:"user_location,omitempty"`
}

// WebSearchUserLocation is an approximate user location used to localize web search results.
type WebSearchUserLocation struct {
	// The city of the user.
	City *string `json:"city,omitempty"`
	// The region or state of the user.
	Region *string `json:"region,omitempty"`
	// The two-letter ISO 3166-1 country code of the user.
	Country *string `json:"country,omitempty"`
	// The IANA timezone of the user.
	Timezone *string `json:"timezone,omitempty"`
}

// MarshalJSON implements custom JSON marshaling for Tool.
func (t Tool) MarshalJSON() ([]byte, error) {
	if t.FunctionTool != nil {
		return json.Marshal(struct {
			Type ToolType `json:"type"`
			*FunctionTool
		}{
			Type:         ToolTypeFunction,
			FunctionTool: t.FunctionTool,
		})
	}
	if t.WebSearchTool != nil {
		return json.Marshal(struct {
			Type ToolType `json:"type"`
			*WebSearchTool
		}{
			Type:          ToolTypeWebSearch,
			WebSearchTool: t.WebSearchTool,
		})
	}
	return nil, fmt.Errorf("tool has no content")
}

// UnmarshalJSON implements custom JSON unmarshaling for Tool.
func (t *Tool) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type ToolType `json:"type"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	switch temp.Type {
	case ToolTypeFunction:
		var tool FunctionTool
		if err := json.Unmarshal(data, &tool); err != nil {
			return err
		}
		t.FunctionTool = &tool
	case ToolTypeWebSearch:
		var tool WebSearchTool
		if err := json.Unmarshal(data, &tool); err != nil {
			return err
		}
		t.WebSearchTool = &tool
	default:
		return fmt.Errorf("unknown tool type: %s", temp.Type)
	}

	return nil
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
	Cost  *float64      `json:"cost,omitempty"`
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
	MaxTokens *uint32 `json:"max_tokens,omitempty"`
	// Amount of randomness injected into the response. Ranges from 0.0 to 1.0
	Temperature *float64 `json:"temperature,omitempty"`
	// An alternative to sampling with temperature, called nucleus sampling, where the model considers the results of the tokens with top_p probability mass. Ranges from 0.0 to 1.0
	TopP *float64 `json:"top_p,omitempty"`
	// Only sample from the top K options for each subsequent token. Used to remove 'long tail' low probability responses. Must be a non-negative integer.
	TopK *int32 `json:"top_k,omitempty"`
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
	// Options for audio generation.
	Audio *AudioOptions `json:"audio,omitempty"`
	// Options for reasoning generation.
	Reasoning *ReasoningOptions `json:"reasoning,omitempty"`
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
