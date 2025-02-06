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

// AccumulatedAudioData represents accumulated audio data
type AccumulatedAudioData struct {
	AudioDataChunks []string
	Format          *AudioFormat
	SampleRate      *int
	Channels        *int
	Transcript      string
	AudioID         *string
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

func (a *AccumulatedAudioData) Type() PartType {
	return PartTypeAudio
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
	default:
		return nil
	}
}

// mergeTextDelta merges text delta with existing text data
func mergeTextDelta(existing *AccumulatedTextData, delta *TextPartDelta) {
	existing.Text += delta.Text
}

// mergeToolCallDelta merges tool call delta with existing tool call data
func mergeToolCallDelta(existing *AccumulatedToolCallData, delta *ToolCallPartDelta) {
	if delta.ToolName != nil {
		existing.ToolName += *delta.ToolName
	}
	if delta.ToolCallID != nil {
		existing.ToolCallID = delta.ToolCallID
	}
	if delta.Args != nil {
		existing.Args += *delta.Args
	}
}

// mergeAudioDelta merges audio delta with existing audio data
func mergeAudioDelta(existing *AccumulatedAudioData, delta *AudioPartDelta) {
	if delta.AudioData != nil {
		existing.AudioDataChunks = append(existing.AudioDataChunks, *delta.AudioData)
	}
	if delta.Format != nil {
		existing.Format = delta.Format
	}
	if delta.SampleRate != nil {
		existing.SampleRate = delta.SampleRate
	}
	if delta.Channels != nil {
		existing.Channels = delta.Channels
	}
	if delta.Transcript != nil {
		existing.Transcript += *delta.Transcript
	}
	if delta.AudioID != nil {
		existing.AudioID = delta.AudioID
	}
}

// mergeDelta merges an incoming delta with existing accumulated data
func mergeDelta(existing AccumulatedData, delta ContentDelta) error {
	switch existingData := existing.(type) {
	case *AccumulatedTextData:
		if delta.Part.TextPartDelta == nil {
			return fmt.Errorf("type mismatch at index %d: existing type is text, incoming type is not text", delta.Index)
		}
		mergeTextDelta(existingData, delta.Part.TextPartDelta)
	case *AccumulatedToolCallData:
		if delta.Part.ToolCallPartDelta == nil {
			return fmt.Errorf("type mismatch at index %d: existing type is tool-call, incoming type is not tool-call", delta.Index)
		}
		mergeToolCallDelta(existingData, delta.Part.ToolCallPartDelta)
	case *AccumulatedAudioData:
		if delta.Part.AudioPartDelta == nil {
			return fmt.Errorf("type mismatch at index %d: existing type is audio, incoming type is not audio", delta.Index)
		}
		mergeAudioDelta(existingData, delta.Part.AudioPartDelta)
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

	return NewAudioPart(concatenatedAudio, *data.Format, data.SampleRate, data.Channels, transcript, data.AudioID), nil
}

// createPart creates a final Part from accumulated data
func createPart(data AccumulatedData, index int) (Part, error) {
	switch d := data.(type) {
	case *AccumulatedTextData:
		return createTextPart(d), nil
	case *AccumulatedToolCallData:
		return createToolCallPart(d, index)
	case *AccumulatedAudioData:
		return createAudioPart(d)
	default:
		return Part{}, fmt.Errorf("unknown accumulated data type at index %d", index)
	}
}

// StreamAccumulator manages the accumulation and merging of content deltas for streaming responses
type StreamAccumulator struct {
	accumulatedParts map[int]AccumulatedData
	accumulatedUsage *ModelUsage
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
	if partial.Usage != nil {
		s.processUsage(*partial.Usage)
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

	return ModelResponse{
		Content: content,
		Usage:   s.accumulatedUsage,
		Cost:    nil,
	}, nil
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
func (s *StreamAccumulator) processUsage(usage ModelUsage) {
	if s.accumulatedUsage == nil {
		// Create a copy of the usage
		s.accumulatedUsage = &ModelUsage{
			InputTokens:         usage.InputTokens,
			OutputTokens:        usage.OutputTokens,
			InputTokensDetails:  usage.InputTokensDetails,
			OutputTokensDetails: usage.OutputTokensDetails,
		}
	} else {
		s.accumulatedUsage.InputTokens += usage.InputTokens
		s.accumulatedUsage.OutputTokens += usage.OutputTokens
	}
}
