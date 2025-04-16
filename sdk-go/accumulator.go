package llmsdk

import (
	"encoding/json"
	"fmt"
	"sort"
)

// AccumulatedTextData represents accumulated text data
type AccumulatedTextData struct {
	Text string
}

// AccumulatedToolCallData represents accumulated tool call data
type AccumulatedToolCallData struct {
	ToolName   string
	ToolCallID *string
	Args       string
}

// AccumulatedImageData represents accumulated image data
type AccumulatedImageData struct {
	MimeType  *string
	ImageData string
	Width     *int
	Height    *int
}

// AccumulatedAudioData represents accumulated audio data
type AccumulatedAudioData struct {
	AudioDataChunks []string
	Format          *AudioFormat
	SampleRate      *int
	Channels        *int
	Transcript      string
	AudioID         *string
}

type AccumulatedReasoningData struct {
	Text      string
	Signature *string
}

// AccumulatedData represents accumulated data for different part types
type AccumulatedData interface {
	Type() PartType
}

func (a *AccumulatedTextData) Type() PartType {
	return PartTypeText
}

func (a *AccumulatedToolCallData) Type() PartType {
	return PartTypeToolCall
}

func (a *AccumulatedImageData) Type() PartType {
	return PartTypeImage
}

func (a *AccumulatedAudioData) Type() PartType {
	return PartTypeAudio
}

func (a *AccumulatedReasoningData) Type() PartType {
	return PartTypeReasoning
}

// newAccumulatedData creates accumulated data from a delta
func newAccumulatedData(delta ContentDelta) AccumulatedData {
	switch {
	case delta.Part.TextPartDelta != nil:
		return &AccumulatedTextData{
			Text: delta.Part.TextPartDelta.Text,
		}
	case delta.Part.ToolCallPartDelta != nil:
		toolName := ""
		if delta.Part.ToolCallPartDelta.ToolName != nil {
			toolName = *delta.Part.ToolCallPartDelta.ToolName
		}
		args := ""
		if delta.Part.ToolCallPartDelta.Args != nil {
			args = *delta.Part.ToolCallPartDelta.Args
		}
		return &AccumulatedToolCallData{
			ToolName:   toolName,
			ToolCallID: delta.Part.ToolCallPartDelta.ToolCallID,
			Args:       args,
		}
	case delta.Part.ImagePartDelta != nil:
		imageData := ""
		if delta.Part.ImagePartDelta.ImageData != nil {
			imageData = *delta.Part.ImagePartDelta.ImageData
		}
		return &AccumulatedImageData{
			ImageData: imageData,
			Width:     delta.Part.ImagePartDelta.Width,
			Height:    delta.Part.ImagePartDelta.Height,
			MimeType:  delta.Part.ImagePartDelta.MimeType,
		}
	case delta.Part.AudioPartDelta != nil:
		var audioDataChunks []string
		if delta.Part.AudioPartDelta.AudioData != nil {
			audioDataChunks = []string{*delta.Part.AudioPartDelta.AudioData}
		}
		transcript := ""
		if delta.Part.AudioPartDelta.Transcript != nil {
			transcript = *delta.Part.AudioPartDelta.Transcript
		}
		return &AccumulatedAudioData{
			AudioDataChunks: audioDataChunks,
			Format:          delta.Part.AudioPartDelta.Format,
			SampleRate:      delta.Part.AudioPartDelta.SampleRate,
			Channels:        delta.Part.AudioPartDelta.Channels,
			Transcript:      transcript,
			AudioID:         delta.Part.AudioPartDelta.AudioID,
		}
	case delta.Part.ReasoningPartDelta != nil:
		return &AccumulatedReasoningData{
			Text:      delta.Part.ReasoningPartDelta.Text,
			Signature: delta.Part.ReasoningPartDelta.Signature,
		}
	default:
		return nil
	}
}

// mergeDelta merges an incoming delta with existing accumulated data
func mergeDelta(existing AccumulatedData, delta ContentDelta) error {
	switch existingData := existing.(type) {
	case *AccumulatedTextData:
		textPartDelta := delta.Part.TextPartDelta
		if textPartDelta == nil {
			return fmt.Errorf("type mismatch at index %d: existing type is text, incoming type is not text", delta.Index)
		}
		existingData.Text += textPartDelta.Text
	case *AccumulatedToolCallData:
		toolCallPartDelta := delta.Part.ToolCallPartDelta
		if toolCallPartDelta == nil {
			return fmt.Errorf("type mismatch at index %d: existing type is tool-call, incoming type is not tool-call", delta.Index)
		}
		if toolCallPartDelta.ToolName != nil {
			existingData.ToolName += *toolCallPartDelta.ToolName
		}
		if toolCallPartDelta.ToolCallID != nil {
			existingData.ToolCallID = toolCallPartDelta.ToolCallID
		}
		if toolCallPartDelta.Args != nil {
			existingData.Args += *toolCallPartDelta.Args
		}
	case *AccumulatedImageData:
		imagePartDelta := delta.Part.ImagePartDelta
		if imagePartDelta == nil {
			return fmt.Errorf("type mismatch at index %d: existing type is image, incoming type is not image", delta.Index)
		}
		if imagePartDelta.ImageData != nil {
			existingData.ImageData += *imagePartDelta.ImageData
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

	case *AccumulatedAudioData:
		audioPartDelta := delta.Part.AudioPartDelta
		if audioPartDelta == nil {
			return fmt.Errorf("type mismatch at index %d: existing type is audio, incoming type is not audio", delta.Index)
		}
		if audioPartDelta.AudioData != nil {
			existingData.AudioDataChunks = append(existingData.AudioDataChunks, *audioPartDelta.AudioData)
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
		if audioPartDelta.AudioID != nil {
			existingData.AudioID = audioPartDelta.AudioID
		}

	case *AccumulatedReasoningData:
		reasoningPartDelta := delta.Part.ReasoningPartDelta
		if reasoningPartDelta == nil {
			return fmt.Errorf("type mismatch at index %d: existing type is reasoning, incoming type is not reasoning", delta.Index)
		}
		existingData.Text += reasoningPartDelta.Text
		if reasoningPartDelta.Signature != nil {
			existingData.Signature = reasoningPartDelta.Signature
		}
	default:
		return fmt.Errorf("unknown accumulated data type at index %d", delta.Index)
	}
	return nil
}

// createTextPart creates a text part from accumulated text data
func createTextPart(data *AccumulatedTextData) Part {
	return NewTextPart(data.Text)
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
func createToolCallPart(data *AccumulatedToolCallData, index int) (Part, error) {
	if data.ToolCallID == nil {
		return Part{}, NewInvariantError("", fmt.Sprintf("Missing required field tool_call_id at index %d", index))
	}
	if data.ToolName == "" {
		return Part{}, NewInvariantError("", fmt.Sprintf("Missing required field tool_name at index %d", index))
	}

	args, err := parseToolCallArgs(data.Args)
	if err != nil {
		return Part{}, err
	}

	return NewToolCallPart(*data.ToolCallID, data.ToolName, args), nil
}

// createImagePart creates an image part from accumulated image data
func createImagePart(data *AccumulatedImageData, index int) (Part, error) {
	if data.MimeType == nil || data.ImageData == "" {
		return Part{}, NewInvariantError("", fmt.Sprintf("Missing required fields at index %d: ImageData=%v, MimeType=%v", index, data.ImageData, data.MimeType))
	}

	return Part{
		ImagePart: &ImagePart{
			ImageData: data.ImageData,
			Width:     data.Width,
			Height:    data.Height,
			MimeType:  *data.MimeType,
		},
	}, nil
}

// createAudioPart creates an audio part from accumulated audio data
func createAudioPart(data *AccumulatedAudioData) (Part, error) {
	if data.Format == nil {
		return Part{}, NewInvariantError("", "Missing required field format for audio part")
	}

	if *data.Format != AudioFormatLinear16 {
		return Part{}, NewNotImplementedError("", fmt.Sprintf("Only linear16 format is supported for audio concatenation. Received: %s", *data.Format))
	}

	concatenatedAudio, err := concatenateB64AudioChunks(data.AudioDataChunks)
	if err != nil {
		return Part{}, err
	}

	var transcript *string
	if data.Transcript != "" {
		transcript = &data.Transcript
	}

	return Part{
		AudioPart: &AudioPart{
			AudioData:  concatenatedAudio,
			Format:     *data.Format,
			SampleRate: data.SampleRate,
			Channels:   data.Channels,
			Transcript: transcript,
			AudioID:    data.AudioID,
		},
	}, nil
}

// createReasoningPart creates a reasoning part from accumulated reasoning data
func createReasoningPart(data *AccumulatedReasoningData) Part {
	return Part{
		ReasoningPart: &ReasoningPart{
			Text:      data.Text,
			Signature: data.Signature,
		},
	}
}

// createPart creates a final Part from accumulated data
func createPart(data AccumulatedData, index int) (Part, error) {
	switch d := data.(type) {
	case *AccumulatedTextData:
		return createTextPart(d), nil
	case *AccumulatedToolCallData:
		return createToolCallPart(d, index)
	case *AccumulatedImageData:
		return createImagePart(d, index)
	case *AccumulatedAudioData:
		return createAudioPart(d)
	case *AccumulatedReasoningData:
		return createReasoningPart(d), nil
	default:
		return Part{}, fmt.Errorf("unknown accumulated data type at index %d", index)
	}
}

// StreamAccumulator manages the accumulation and merging of content deltas for streaming responses
type StreamAccumulator struct {
	accumulatedParts map[int]AccumulatedData
	accumulatedUsage *ModelUsage
	cost             float64
}

// NewStreamAccumulator creates a new StreamAccumulator
func NewStreamAccumulator() *StreamAccumulator {
	return &StreamAccumulator{
		accumulatedParts: make(map[int]AccumulatedData),
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
	s.accumulatedParts = make(map[int]AccumulatedData)
	s.accumulatedUsage = nil
}

// processDelta processes a single delta, either merging with existing or creating new
func (s *StreamAccumulator) processDelta(delta ContentDelta) error {
	existing, exists := s.accumulatedParts[delta.Index]
	if exists {
		return mergeDelta(existing, delta)
	} else {
		accumulated := newAccumulatedData(delta)
		if accumulated == nil {
			return fmt.Errorf("unable to initialize accumulated data for delta at index %d", delta.Index)
		}
		s.accumulatedParts[delta.Index] = accumulated
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
