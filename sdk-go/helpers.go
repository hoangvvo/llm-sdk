package llmsdk

import (
	"encoding/json"
)

// NewTextPart creates a new text part
func NewTextPart(text string) Part {
	return Part{
		TextPart: &TextPart{
			Text: text,
		},
	}
}

// NewImagePart creates a new image part
func NewImagePart(imageData, mimeType string, opts ...ImagePartOption) Part {
	imagePart := &ImagePart{
		ImageData: imageData,
		MimeType:  mimeType,
	}

	for _, opt := range opts {
		opt(imagePart)
	}

	return Part{
		ImagePart: imagePart,
	}
}

type ImagePartOption func(*ImagePart)

func WithImageWidth(width int) ImagePartOption {
	return func(p *ImagePart) {
		p.Width = &width
	}
}

func WithImageHeight(height int) ImagePartOption {
	return func(p *ImagePart) {
		p.Height = &height
	}
}

// NewAudioPart creates a new audio part
func NewAudioPart(audioData string, format AudioFormat, opts ...AudioPartOption) Part {
	audioPart := &AudioPart{
		AudioData: audioData,
		Format:    format,
	}

	for _, opt := range opts {
		opt(audioPart)
	}

	return Part{
		AudioPart: audioPart,
	}
}

type AudioPartOption func(*AudioPart)

func WithAudioSampleRate(sampleRate int) AudioPartOption {
	return func(p *AudioPart) {
		p.SampleRate = &sampleRate
	}
}

func WithAudioChannels(channels int) AudioPartOption {
	return func(p *AudioPart) {
		p.Channels = &channels
	}
}

func WithAudioTranscript(transcript string) AudioPartOption {
	return func(p *AudioPart) {
		p.Transcript = &transcript
	}
}

func WithAudioID(audioID string) AudioPartOption {
	return func(p *AudioPart) {
		p.ID = &audioID
	}
}

func NewReasoningPart(text string, opts ...ReasoingPartOption) Part {
	reasoningPart := &ReasoningPart{
		Text: text,
	}

	for _, opt := range opts {
		opt(reasoningPart)
	}

	return Part{
		ReasoningPart: reasoningPart,
	}
}

type ReasoingPartOption func(*ReasoningPart)

func WithReasoningSignature(signature string) ReasoingPartOption {
	return func(p *ReasoningPart) {
		p.Signature = &signature
	}
}

func WithReasoningID(id string) ReasoingPartOption {
	return func(p *ReasoningPart) {
		p.ID = &id
	}
}

// NewSourcePart creates a new source part
func NewSourcePart(title string, content []Part) Part {
	return Part{
		SourcePart: &SourcePart{
			Title:   title,
			Content: content,
		},
	}
}

// NewToolCallPart creates a new tool call part
func NewToolCallPart(toolCallID, toolName string, args any, opts ...ToolCallPartOption) Part {
	// TODO: handle error
	argsJSON, _ := json.Marshal(args)

	toolCallPart := Part{
		ToolCallPart: &ToolCallPart{
			ToolCallID: toolCallID,
			ToolName:   toolName,
			Args:       argsJSON,
		},
	}

	for _, opt := range opts {
		opt(toolCallPart.ToolCallPart)
	}

	return toolCallPart
}

type ToolCallPartOption func(*ToolCallPart)

func WithToolCallPartID(id string) ToolCallPartOption {
	return func(p *ToolCallPart) {
		p.ID = &id
	}
}

// NewToolResultPart creates a new tool result part
func NewToolResultPart(toolCallID, toolName string, content []Part, isError bool) Part {
	return Part{
		ToolResultPart: &ToolResultPart{
			ToolCallID: toolCallID,
			ToolName:   toolName,
			Content:    content,
			IsError:    isError,
		},
	}
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

// NewToolChoiceAuto creates an auto tool choice
func NewToolChoiceAuto() *ToolChoiceOption {
	return &ToolChoiceOption{Auto: &ToolChoiceAuto{}}
}

// NewToolChoiceNone creates a none tool choice
func NewToolChoiceNone() *ToolChoiceOption {
	return &ToolChoiceOption{None: &ToolChoiceNone{}}
}

// NewToolChoiceRequired creates a required tool choice
func NewToolChoiceRequired() *ToolChoiceOption {
	return &ToolChoiceOption{Required: &ToolChoiceRequired{}}
}

// NewToolChoiceTool creates a specific tool choice
func NewToolChoiceTool(toolName string) *ToolChoiceOption {
	return &ToolChoiceOption{Tool: &ToolChoiceTool{ToolName: toolName}}
}

// NewResponseFormatText creates a text response format
func NewResponseFormatText() *ResponseFormatOption {
	return &ResponseFormatOption{Text: &ResponseFormatText{}}
}

// NewResponseFormatJSON creates a JSON response format
func NewResponseFormatJSON(name string, description *string, schema *JSONSchema) *ResponseFormatOption {
	return &ResponseFormatOption{
		JSON: &ResponseFormatJSON{
			Name:        name,
			Description: description,
			Schema:      schema,
		},
	}
}
