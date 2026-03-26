package partutil

import (
	"slices"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

// GuessDeltaIndex tries to guess the appropriate index for a delta based on existing deltas.
//
// Because of the difference in mapping, especially in OpenAI cases,
// where text and audio part does not have indexes
// or in Google cases, where no parts have indexes,
// we need to guess an index for the incoming delta
// which is required in our unified interface.
//
// toolCallIndex does not always correspond to the index of the tool call in
// the deltas because some providers keep tool call separate from other parts
// (e.g openai). We can match this against the existing tool call deltas.
func GuessDeltaIndex(part llmsdk.PartDelta, allContentDeltas []llmsdk.ContentDelta, toolCallIndex *int) int {
	// contentDeltas may have the structure of
	// [part0 partial, part0 partial, part1 partial].
	// For the purpose of this matching, we want only
	// [part0, part1]
	uniqueContentDeltas := slices.CompactFunc(slices.Clone(allContentDeltas), func(a, b llmsdk.ContentDelta) bool {
		return a.Index == b.Index
	})

	if toolCallIndex != nil && part.ToolCallPartDelta != nil {
		// Providers like OpenAI track tool calls in a separate field, so we
		// need to reconcile that. To understand how this matching works:
		// [Provider]
		// toolCalls: [index 0] [index 1]
		// [LLM-SDK state]
		// parts: [index 0 text] [index 1 tool] [index 2 text] [index 3 tool]
		// In this case, we need to map the tool index 0 -> 1 and 1 -> 3
		var toolPartDeltas []llmsdk.ContentDelta
		for _, contentDelta := range uniqueContentDeltas {
			if contentDelta.Part.ToolCallPartDelta != nil {
				toolPartDeltas = append(toolPartDeltas, contentDelta)
			}
		}

		if *toolCallIndex < len(toolPartDeltas) {
			return toolPartDeltas[*toolCallIndex].Index
		}
		// If no matching tool call delta found, return the length of
		// uniqueContentDeltas. This is because we want to append a new tool
		// call delta
		return len(uniqueContentDeltas)
	}

	// Attempt to find the LAST matching delta in uniqueContentDeltas
	var matchingDelta *llmsdk.ContentDelta
	for i := len(uniqueContentDeltas) - 1; i >= 0; i-- {
		contentDelta := &uniqueContentDeltas[i]
		// Inline matching logic: For text and audio parts, they are the matching delta
		// if their types are the same. This is because providers that do not
		// provide indexes like only have 1 part for each type (e.g openai has only 1
		// message.content or 1 message.audio)
		isMatch := false
		if contentDelta.Part.TextPartDelta != nil && part.TextPartDelta != nil {
			isMatch = true
		} else if contentDelta.Part.AudioPartDelta != nil && part.AudioPartDelta != nil {
			isMatch = true
		}
		// For tool calls, we can't reliably match them
		// because there can be multiple tool calls with the same tool name
		// Different types don't match

		if isMatch {
			matchingDelta = contentDelta
			break
		}
	}

	if matchingDelta != nil {
		return matchingDelta.Index
	}

	// If no matching delta found, return max index + 1
	maxIndex := 0
	for _, contentDelta := range uniqueContentDeltas {
		if contentDelta.Index > maxIndex {
			maxIndex = contentDelta.Index
		}
	}

	// Since we're using int, we start from 0 instead of -1
	if maxIndex == 0 && len(uniqueContentDeltas) == 0 {
		return 0
	} else {
		return maxIndex + 1
	}
}

func LooselyConvertPartToPartDelta(part llmsdk.Part) llmsdk.PartDelta {
	switch {
	case part.TextPart != nil:
		return llmsdk.PartDelta{
			TextPartDelta: &llmsdk.TextPartDelta{
				Text: part.TextPart.Text,
			},
		}
	case part.ToolCallPart != nil:
		argsStr := string(part.ToolCallPart.Args)
		return llmsdk.PartDelta{
			ToolCallPartDelta: &llmsdk.ToolCallPartDelta{
				ToolCallID:       &part.ToolCallPart.ToolCallID,
				ToolName:         &part.ToolCallPart.ToolName,
				Args:             &argsStr,
				ID:               part.ToolCallPart.ID,
				ThoughtSignature: part.ToolCallPart.ThoughtSignature,
			},
		}
	case part.ReasoningPart != nil:
		return llmsdk.PartDelta{
			ReasoningPartDelta: &llmsdk.ReasoningPartDelta{
				Text:      part.ReasoningPart.Text,
				Signature: part.ReasoningPart.Signature,
				ID:        part.ReasoningPart.ID,
			},
		}
	case part.ImagePart != nil:
		return llmsdk.PartDelta{
			ImagePartDelta: &llmsdk.ImagePartDelta{
				MimeType: &part.ImagePart.MimeType,
				Data:     &part.ImagePart.Data,
				Width:    part.ImagePart.Width,
				Height:   part.ImagePart.Height,
				ID:       part.ImagePart.ID,
			},
		}
	case part.AudioPart != nil:
		return llmsdk.PartDelta{
			AudioPartDelta: &llmsdk.AudioPartDelta{
				Data:       &part.AudioPart.Data,
				Format:     &part.AudioPart.Format,
				SampleRate: part.AudioPart.SampleRate,
				Channels:   part.AudioPart.Channels,
				Transcript: part.AudioPart.Transcript,
				ID:         part.AudioPart.ID,
			},
		}
	default:
		return llmsdk.PartDelta{}
	}
}
