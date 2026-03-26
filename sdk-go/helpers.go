package llmsdk

import (
	"encoding/json"
)

// NewTextPart creates a new text part
func NewTextPart(text string, opts ...TextPartOption) Part {
	textPart := &TextPart{
		Text: text,
	}

	for _, opt := range opts {
		opt(textPart)
	}

	return Part{TextPart: textPart}
}

type TextPartOption func(*TextPart)

func WithTextCitations(citations []Citation) TextPartOption {
	return func(p *TextPart) {
		p.Citations = citations
	}
}

// NewImagePart creates a new image part
func NewImagePart(data, mimeType string, opts ...ImagePartOption) Part {
	imagePart := &ImagePart{
		Data:     data,
		MimeType: mimeType,
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

func WithImageID(imageID string) ImagePartOption {
	return func(p *ImagePart) {
		p.ID = &imageID
	}
}

// NewAudioPart creates a new audio part
func NewAudioPart(data string, format AudioFormat, opts ...AudioPartOption) Part {
	audioPart := &AudioPart{
		Data:   data,
		Format: format,
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

// NewSourcePart creates a new source part
func NewSourcePart(source string, title string, content []Part) Part {
	return Part{
		SourcePart: &SourcePart{
			Source:  source,
			Title:   title,
			Content: content,
		},
	}
}

func NewReasoningPart(text string, opts ...ReasoningPartOption) Part {
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

type ReasoningPartOption func(*ReasoningPart)

func WithReasoningSignature(signature string) ReasoningPartOption {
	return func(p *ReasoningPart) {
		p.Signature = &signature
	}
}

func WithReasoningID(id string) ReasoningPartOption {
	return func(p *ReasoningPart) {
		p.ID = &id
	}
}

// NewToolCallPart creates a new tool call part
func NewToolCallPart(toolCallID, toolName string, args any, opts ...ToolCallPartOption) Part {
	var argsJSON []byte
	switch v := args.(type) {
	case nil:
		argsJSON = nil
	case json.RawMessage:
		argsJSON = v
	case []byte:
		argsJSON = v
	default:
		// TODO: handle error
		argsJSON, _ = json.Marshal(v)
	}

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

func WithToolCallThoughtSignature(signature string) ToolCallPartOption {
	return func(p *ToolCallPart) {
		p.ThoughtSignature = &signature
	}
}

// NewToolResultPart creates a new tool result part
func NewToolResultPart(toolCallID, toolName string, content []Part, opts ...ToolResultPartOption) Part {
	toolResultPart := Part{
		ToolResultPart: &ToolResultPart{
			ToolCallID: toolCallID,
			ToolName:   toolName,
			Content:    content,
		},
	}
	for _, opt := range opts {
		opt(toolResultPart.ToolResultPart)
	}
	return toolResultPart
}

type ToolResultPartOption func(*ToolResultPart)

func WithToolResultIsError(isError bool) ToolResultPartOption {
	return func(p *ToolResultPart) {
		p.IsError = isError
	}
}

// NewTextPartDelta constructs a text part delta with optional citation updates.
func NewTextPartDelta(text string, opts ...TextPartDeltaOption) PartDelta {
	textDelta := &TextPartDelta{Text: text}

	for _, opt := range opts {
		opt(textDelta)
	}

	return PartDelta{TextPartDelta: textDelta}
}

type TextPartDeltaOption func(*TextPartDelta)

func WithTextPartDeltaCitation(citation *CitationDelta) TextPartDeltaOption {
	return func(p *TextPartDelta) {
		p.Citation = citation
	}
}

// NewCitationDelta constructs a citation delta for streaming updates.
func NewCitationDelta(opts ...CitationDeltaOption) *CitationDelta {
	citation := &CitationDelta{}

	for _, opt := range opts {
		opt(citation)
	}

	return citation
}

type CitationDeltaOption func(*CitationDelta)

func WithCitationDeltaSource(source string) CitationDeltaOption {
	return func(c *CitationDelta) {
		c.Source = &source
	}
}

func WithCitationDeltaTitle(title string) CitationDeltaOption {
	return func(c *CitationDelta) {
		c.Title = &title
	}
}

func WithCitationDeltaCitedText(citedText string) CitationDeltaOption {
	return func(c *CitationDelta) {
		c.CitedText = &citedText
	}
}

func WithCitationDeltaStartIndex(start int) CitationDeltaOption {
	return func(c *CitationDelta) {
		c.StartIndex = &start
	}
}

func WithCitationDeltaEndIndex(end int) CitationDeltaOption {
	return func(c *CitationDelta) {
		c.EndIndex = &end
	}
}

// NewToolCallPartDelta constructs a tool call part delta for streaming tool invocations.
func NewToolCallPartDelta(opts ...ToolCallPartDeltaOption) PartDelta {
	toolCallDelta := &ToolCallPartDelta{}

	for _, opt := range opts {
		opt(toolCallDelta)
	}

	return PartDelta{ToolCallPartDelta: toolCallDelta}
}

type ToolCallPartDeltaOption func(*ToolCallPartDelta)

func WithToolCallPartDeltaToolCallID(toolCallID string) ToolCallPartDeltaOption {
	return func(p *ToolCallPartDelta) {
		p.ToolCallID = &toolCallID
	}
}

func WithToolCallPartDeltaToolName(toolName string) ToolCallPartDeltaOption {
	return func(p *ToolCallPartDelta) {
		p.ToolName = &toolName
	}
}

func WithToolCallPartDeltaArgs(args string) ToolCallPartDeltaOption {
	return func(p *ToolCallPartDelta) {
		p.Args = &args
	}
}

func WithToolCallPartDeltaID(id string) ToolCallPartDeltaOption {
	return func(p *ToolCallPartDelta) {
		p.ID = &id
	}
}

func WithToolCallPartDeltaThoughtSignature(signature string) ToolCallPartDeltaOption {
	return func(p *ToolCallPartDelta) {
		p.ThoughtSignature = &signature
	}
}

// NewImagePartDelta constructs an image part delta for incremental image updates.
func NewImagePartDelta(opts ...ImagePartDeltaOption) PartDelta {
	imageDelta := &ImagePartDelta{}

	for _, opt := range opts {
		opt(imageDelta)
	}

	return PartDelta{ImagePartDelta: imageDelta}
}

type ImagePartDeltaOption func(*ImagePartDelta)

func WithImagePartDeltaMimeType(mimeType string) ImagePartDeltaOption {
	return func(p *ImagePartDelta) {
		p.MimeType = &mimeType
	}
}

func WithImagePartDeltaData(data string) ImagePartDeltaOption {
	return func(p *ImagePartDelta) {
		p.Data = &data
	}
}

func WithImagePartDeltaWidth(width int) ImagePartDeltaOption {
	return func(p *ImagePartDelta) {
		p.Width = &width
	}
}

func WithImagePartDeltaHeight(height int) ImagePartDeltaOption {
	return func(p *ImagePartDelta) {
		p.Height = &height
	}
}

func WithImagePartDeltaID(id string) ImagePartDeltaOption {
	return func(p *ImagePartDelta) {
		p.ID = &id
	}
}

// NewAudioPartDelta constructs an audio part delta for streamed audio results.
func NewAudioPartDelta(opts ...AudioPartDeltaOption) PartDelta {
	audioDelta := &AudioPartDelta{}

	for _, opt := range opts {
		opt(audioDelta)
	}

	return PartDelta{AudioPartDelta: audioDelta}
}

type AudioPartDeltaOption func(*AudioPartDelta)

func WithAudioPartDeltaData(data string) AudioPartDeltaOption {
	return func(p *AudioPartDelta) {
		p.Data = &data
	}
}

func WithAudioPartDeltaFormat(format AudioFormat) AudioPartDeltaOption {
	return func(p *AudioPartDelta) {
		p.Format = &format
	}
}

func WithAudioPartDeltaSampleRate(sampleRate int) AudioPartDeltaOption {
	return func(p *AudioPartDelta) {
		p.SampleRate = &sampleRate
	}
}

func WithAudioPartDeltaChannels(channels int) AudioPartDeltaOption {
	return func(p *AudioPartDelta) {
		p.Channels = &channels
	}
}

func WithAudioPartDeltaTranscript(transcript string) AudioPartDeltaOption {
	return func(p *AudioPartDelta) {
		p.Transcript = &transcript
	}
}

func WithAudioPartDeltaID(id string) AudioPartDeltaOption {
	return func(p *AudioPartDelta) {
		p.ID = &id
	}
}

// NewReasoningPartDelta constructs a reasoning part delta for streamed reasoning traces.
func NewReasoningPartDelta(text string, opts ...ReasoningPartDeltaOption) PartDelta {
	reasoningDelta := &ReasoningPartDelta{Text: text}

	for _, opt := range opts {
		opt(reasoningDelta)
	}

	return PartDelta{ReasoningPartDelta: reasoningDelta}
}

type ReasoningPartDeltaOption func(*ReasoningPartDelta)

func WithReasoningPartDeltaSignature(signature string) ReasoningPartDeltaOption {
	return func(p *ReasoningPartDelta) {
		p.Signature = &signature
	}
}

func WithReasoningPartDeltaID(id string) ReasoningPartDeltaOption {
	return func(p *ReasoningPartDelta) {
		p.ID = &id
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
