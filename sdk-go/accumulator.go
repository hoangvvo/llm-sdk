package llmsdk

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/hoangvvo/llm-sdk/sdk-go/utils/audioutil"
)

// accumulatedTextData represents accumulated text data
type accumulatedTextData struct {
	Text      string
	Citations map[int]CitationDelta
}

// accumulatedImageData represents accumulated image data
type accumulatedImageData struct {
	MimeType *string
	Data     string
	Width    *int
	Height   *int
	ID       *string
}

// accumulatedAudioData represents accumulated audio data
type accumulatedAudioData struct {
	DataChunks []string
	Format     *AudioFormat
	SampleRate *int
	Channels   *int
	Transcript string
	ID         *string
}

// accumulatedData represents accumulated data for different part types
type accumulatedData struct {
	Text      *accumulatedTextData
	ToolCall  *ToolCallPartDelta
	Image     *accumulatedImageData
	Audio     *accumulatedAudioData
	Reasoning *ReasoningPartDelta
}

// newDelta creates accumulated data from a delta
func newDelta(delta ContentDelta) *accumulatedData {
	switch {
	case delta.Part.TextPartDelta != nil:
		textDelta := delta.Part.TextPartDelta
		textData := &accumulatedTextData{
			Text: textDelta.Text,
		}
		if textDelta.Citation != nil {
			textData.Citations = make(map[int]CitationDelta)
			textData.Citations[0] = *textDelta.Citation
		}
		return &accumulatedData{
			Text: textData,
		}
	case delta.Part.ToolCallPartDelta != nil:
		return &accumulatedData{
			ToolCall: delta.Part.ToolCallPartDelta,
		}
	case delta.Part.ImagePartDelta != nil:
		imageData := ""
		if delta.Part.ImagePartDelta.Data != nil {
			imageData = *delta.Part.ImagePartDelta.Data
		}
		return &accumulatedData{
			Image: &accumulatedImageData{
				Data:     imageData,
				Width:    delta.Part.ImagePartDelta.Width,
				Height:   delta.Part.ImagePartDelta.Height,
				MimeType: delta.Part.ImagePartDelta.MimeType,
				ID:       delta.Part.ImagePartDelta.ID,
			},
		}
	case delta.Part.AudioPartDelta != nil:
		var dataChunks []string
		if delta.Part.AudioPartDelta.Data != nil {
			dataChunks = []string{*delta.Part.AudioPartDelta.Data}
		}
		transcript := ""
		if delta.Part.AudioPartDelta.Transcript != nil {
			transcript = *delta.Part.AudioPartDelta.Transcript
		}
		return &accumulatedData{
			Audio: &accumulatedAudioData{
				DataChunks: dataChunks,
				Format:     delta.Part.AudioPartDelta.Format,
				SampleRate: delta.Part.AudioPartDelta.SampleRate,
				Channels:   delta.Part.AudioPartDelta.Channels,
				Transcript: transcript,
				ID:         delta.Part.AudioPartDelta.ID,
			},
		}
	case delta.Part.ReasoningPartDelta != nil:
		return &accumulatedData{
			Reasoning: delta.Part.ReasoningPartDelta,
		}
	default:
		return nil
	}
}

// mergeDelta merges an incoming delta with existing accumulated data
func mergeDelta(existing accumulatedData, delta ContentDelta) error {
	switch {
	case existing.Text != nil:
		textPartDelta := delta.Part.TextPartDelta
		if textPartDelta == nil {
			return fmt.Errorf("type mismatch at index %d: existing type is text, incoming type is not text", delta.Index)
		}
		existingData := existing.Text
		existingData.Text += textPartDelta.Text
		if textPartDelta.Citation != nil {
			if existingData.Citations == nil {
				existingData.Citations = make(map[int]CitationDelta)
			}
			citationIndex := len(existingData.Citations)
			existingData.Citations[citationIndex] = *textPartDelta.Citation
		}
	case existing.ToolCall != nil:
		toolCallPartDelta := delta.Part.ToolCallPartDelta
		if toolCallPartDelta == nil {
			return fmt.Errorf("type mismatch at index %d: existing type is tool-call, incoming type is not tool-call", delta.Index)
		}
		existingData := existing.ToolCall
		if toolCallPartDelta.ToolName != nil {
			if existingData.ToolName == nil {
				existingData.ToolName = new(string)
			}
			*existingData.ToolName += *toolCallPartDelta.ToolName
		}
		if toolCallPartDelta.ToolCallID != nil {
			existingData.ToolCallID = toolCallPartDelta.ToolCallID
		}
		if toolCallPartDelta.Args != nil {
			if existingData.Args == nil {
				existingData.Args = new(string)
			}
			*existingData.Args += *toolCallPartDelta.Args
		}
		if toolCallPartDelta.ID != nil {
			existingData.ID = toolCallPartDelta.ID
		}
		if toolCallPartDelta.ThoughtSignature != nil {
			existingData.ThoughtSignature = toolCallPartDelta.ThoughtSignature
		}
	case existing.Image != nil:
		imagePartDelta := delta.Part.ImagePartDelta
		if imagePartDelta == nil {
			return fmt.Errorf("type mismatch at index %d: existing type is image, incoming type is not image", delta.Index)
		}
		existingData := existing.Image
		if imagePartDelta.Data != nil {
			existingData.Data += *imagePartDelta.Data
		}
		if imagePartDelta.Width != nil {
			existingData.Width = imagePartDelta.Width
		}
		if imagePartDelta.Height != nil {
			existingData.Height = imagePartDelta.Height
		}
		if imagePartDelta.MimeType != nil {
			existingData.MimeType = imagePartDelta.MimeType
		}
		if imagePartDelta.ID != nil {
			existingData.ID = imagePartDelta.ID
		}

	case existing.Audio != nil:
		audioPartDelta := delta.Part.AudioPartDelta
		if audioPartDelta == nil {
			return fmt.Errorf("type mismatch at index %d: existing type is audio, incoming type is not audio", delta.Index)
		}
		existingData := existing.Audio
		if audioPartDelta.Data != nil {
			existingData.DataChunks = append(existingData.DataChunks, *audioPartDelta.Data)
		}
		if audioPartDelta.Format != nil {
			existingData.Format = audioPartDelta.Format
		}
		if audioPartDelta.SampleRate != nil {
			existingData.SampleRate = audioPartDelta.SampleRate
		}
		if audioPartDelta.Channels != nil {
			existingData.Channels = audioPartDelta.Channels
		}
		if audioPartDelta.Transcript != nil {
			existingData.Transcript += *audioPartDelta.Transcript
		}
		if audioPartDelta.ID != nil {
			existingData.ID = audioPartDelta.ID
		}

	case existing.Reasoning != nil:
		reasoningPartDelta := delta.Part.ReasoningPartDelta
		if reasoningPartDelta == nil {
			return fmt.Errorf("type mismatch at index %d: existing type is reasoning, incoming type is not reasoning", delta.Index)
		}
		existingData := existing.Reasoning
		existingData.Text += reasoningPartDelta.Text
		if reasoningPartDelta.Signature != nil {
			existingData.Signature = reasoningPartDelta.Signature
		}
		if reasoningPartDelta.ID != nil {
			existingData.ID = reasoningPartDelta.ID
		}
	default:
		return fmt.Errorf("unknown accumulated data type at index %d", delta.Index)
	}
	return nil
}

// createTextPart creates a text part from accumulated text data
func createTextPart(data *accumulatedTextData, index int) (Part, error) {
	var opts []TextPartOption

	if len(data.Citations) > 0 {
		indices := make([]int, 0, len(data.Citations))
		for citationIndex := range data.Citations {
			indices = append(indices, citationIndex)
		}
		sort.Ints(indices)

		citations := make([]Citation, 0, len(indices))
		for _, citationIndex := range indices {
			citationDelta := data.Citations[citationIndex]
			if citationDelta.Source == nil || citationDelta.StartIndex == nil || citationDelta.EndIndex == nil {
				return Part{}, NewInvariantError(
					"",
					fmt.Sprintf(
						"Incomplete citation data for text part at index %d: source=%v, start_index=%v, end_index=%v",
						index,
						citationDelta.Source,
						citationDelta.StartIndex,
						citationDelta.EndIndex,
					),
				)
			}

			citation := Citation{
				Source:     *citationDelta.Source,
				StartIndex: *citationDelta.StartIndex,
				EndIndex:   *citationDelta.EndIndex,
			}
			if citationDelta.Title != nil {
				citation.Title = citationDelta.Title
			}
			if citationDelta.CitedText != nil {
				citation.CitedText = citationDelta.CitedText
			}
			citations = append(citations, citation)
		}

		if len(citations) > 0 {
			opts = append(opts, WithTextCitations(citations))
		}
	}

	return NewTextPart(data.Text, opts...), nil
}

// parseToolCallArgs parses tool call arguments from JSON string
func parseToolCallArgs(args string) (map[string]any, error) {
	if args == "" {
		return make(map[string]any), nil
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(args), &result); err != nil {
		return nil, NewInvariantError("", fmt.Sprintf("Invalid tool call arguments: %s: %s", args, err.Error()))
	}
	return result, nil
}

// createToolCallPart creates a tool call part from accumulated tool call data
func createToolCallPart(data *ToolCallPartDelta, index int) (Part, error) {
	if data.ToolCallID == nil {
		return Part{}, NewInvariantError("", fmt.Sprintf("Missing required field tool_call_id at index %d", index))
	}
	if data.ToolName == nil {
		return Part{}, NewInvariantError("", fmt.Sprintf("Missing required field tool_name at index %d", index))
	}

	strArgs := ""
	if data.Args != nil {
		strArgs = *data.Args
	}
	args, err := parseToolCallArgs(strArgs)
	if err != nil {
		return Part{}, err
	}

	var opts []ToolCallPartOption
	if data.ID != nil {
		opts = append(opts, WithToolCallPartID(*data.ID))
	}
	if data.ThoughtSignature != nil {
		opts = append(opts, WithToolCallThoughtSignature(*data.ThoughtSignature))
	}

	toolCallPart := NewToolCallPart(*data.ToolCallID, *data.ToolName, args, opts...)
	return toolCallPart, nil
}

// createImagePart creates an image part from accumulated image data
func createImagePart(data *accumulatedImageData, index int) (Part, error) {
	if data.MimeType == nil || data.Data == "" {
		return Part{}, NewInvariantError("", fmt.Sprintf("Missing required fields at index %d: Data=%v, MimeType=%v", index, data.Data, data.MimeType))
	}

	var opts []ImagePartOption
	if data.Width != nil {
		opts = append(opts, WithImageWidth(*data.Width))
	}
	if data.Height != nil {
		opts = append(opts, WithImageHeight(*data.Height))
	}
	if data.ID != nil {
		opts = append(opts, WithImageID(*data.ID))
	}

	return NewImagePart(data.Data, *data.MimeType, opts...), nil
}

// createAudioPart creates an audio part from accumulated audio data
func createAudioPart(data *accumulatedAudioData) (Part, error) {
	if data.Format == nil {
		return Part{}, NewInvariantError("", "Missing required field format for audio part")
	}

	if *data.Format != AudioFormatLinear16 {
		return Part{}, NewNotImplementedError("", fmt.Sprintf("Only linear16 format is supported for audio concatenation. Received: %s", *data.Format))
	}

	concatenatedAudio, err := audioutil.ConcatenateB64AudioChunks(data.DataChunks)
	if err != nil {
		return Part{}, err
	}

	var opts []AudioPartOption
	if data.SampleRate != nil {
		opts = append(opts, WithAudioSampleRate(*data.SampleRate))
	}
	if data.Channels != nil {
		opts = append(opts, WithAudioChannels(*data.Channels))
	}
	if data.Transcript != "" {
		opts = append(opts, WithAudioTranscript(data.Transcript))
	}
	if data.ID != nil {
		opts = append(opts, WithAudioID(*data.ID))
	}

	return NewAudioPart(concatenatedAudio, *data.Format, opts...), nil
}

// createReasoningPart creates a reasoning part from accumulated reasoning data
func createReasoningPart(data *ReasoningPartDelta) Part {
	var opts []ReasoningPartOption
	if data.Signature != nil {
		opts = append(opts, WithReasoningSignature(*data.Signature))
	}
	if data.ID != nil {
		opts = append(opts, WithReasoningID(*data.ID))
	}
	return NewReasoningPart(data.Text, opts...)
}

// createPart creates a final Part from accumulated data
func createPart(data accumulatedData, index int) (Part, error) {
	switch {
	case data.Text != nil:
		return createTextPart(data.Text, index)
	case data.ToolCall != nil:
		return createToolCallPart(data.ToolCall, index)
	case data.Image != nil:
		return createImagePart(data.Image, index)
	case data.Audio != nil:
		return createAudioPart(data.Audio)
	case data.Reasoning != nil:
		return createReasoningPart(data.Reasoning), nil
	default:
		return Part{}, fmt.Errorf("unknown accumulated data type at index %d", index)
	}
}

// StreamAccumulator manages the accumulation and merging of content deltas for streaming responses
type StreamAccumulator struct {
	accumulatedParts map[int]accumulatedData
	accumulatedUsage *ModelUsage
	cost             float64
}

// NewStreamAccumulator creates a new StreamAccumulator
func NewStreamAccumulator() *StreamAccumulator {
	return &StreamAccumulator{
		accumulatedParts: make(map[int]accumulatedData),
		accumulatedUsage: nil,
	}
}

// AddPartial adds a chunk of content deltas to the accumulator
func (s *StreamAccumulator) AddPartial(partial PartialModelResponse) error {
	if partial.Delta != nil {
		if err := s.processDelta(*partial.Delta); err != nil {
			return err
		}
	}
	if partial.Usage != nil || partial.Cost != nil {
		s.processUsage(partial.Usage, partial.Cost)
	}
	return nil
}

// ComputeResponse computes the final response from accumulated deltas
func (s *StreamAccumulator) ComputeResponse() (ModelResponse, error) {
	// Sort indices for consistent ordering
	var indices []int
	for index := range s.accumulatedParts {
		indices = append(indices, index)
	}
	sort.Ints(indices)

	var content []Part
	for _, index := range indices {
		data := s.accumulatedParts[index]
		part, err := createPart(data, index)
		if err != nil {
			return ModelResponse{}, err
		}
		content = append(content, part)
	}

	r := ModelResponse{
		Content: content,
		Usage:   s.accumulatedUsage,
		Cost:    nil,
	}
	if s.cost > 0 {
		r.Cost = &s.cost
	}
	return r, nil
}

// Size gets the number of accumulated parts
func (s *StreamAccumulator) Size() int {
	return len(s.accumulatedParts)
}

// IsEmpty checks if the accumulator has any data
func (s *StreamAccumulator) IsEmpty() bool {
	return len(s.accumulatedParts) == 0
}

// Clear clears all accumulated data
func (s *StreamAccumulator) Clear() {
	s.accumulatedParts = make(map[int]accumulatedData)
	s.accumulatedUsage = nil
}

// processDelta processes a single delta, either merging with existing or creating new
func (s *StreamAccumulator) processDelta(delta ContentDelta) error {
	existing, exists := s.accumulatedParts[delta.Index]
	if exists {
		return mergeDelta(existing, delta)
	} else {
		accumulated := newDelta(delta)
		if accumulated == nil {
			return fmt.Errorf("unable to initialize accumulated data for delta at index %d", delta.Index)
		}
		s.accumulatedParts[delta.Index] = *accumulated
		return nil
	}
}

// processUsage processes usage statistics
func (s *StreamAccumulator) processUsage(usage *ModelUsage, cost *float64) {
	if usage != nil {
		if s.accumulatedUsage == nil {
			// Create a copy of the usage
			s.accumulatedUsage = &ModelUsage{}
		}
		s.accumulatedUsage.Add(usage)
	}
	if cost != nil {
		s.cost += *cost
	}
}
