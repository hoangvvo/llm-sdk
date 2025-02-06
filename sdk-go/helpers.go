package llmsdk

import (
	"encoding/json"
)

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

// NewSourcePart creates a new source part
func NewSourcePart(title string, content []Part, id *string) Part {
	return Part{
		SourcePart: &SourcePart{
			Title:   title,
			Content: content,
			ID:      id,
		},
	}
}

// NewToolCallPart creates a new tool call part
func NewToolCallPart(toolCallID, toolName string, args any, id *string) Part {
	// TODO: handle error
	argsJSON, _ := json.Marshal(args)

	return Part{
		ToolCallPart: &ToolCallPart{
			ToolCallID: toolCallID,
			ToolName:   toolName,
			Args:       argsJSON,
			ID:         id,
		},
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
