package google

import (
	"testing"

	"github.com/hoangvvo/llm-sdk/sdk-go/google/googleapi"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"
)

func TestCitationsUseProviderPartIndexBeforeFiltering(t *testing.T) {
	content, err := mapGoogleContent(
		[]googleapi.Part{
			{},
			{Text: ptr.To("first")},
			{Text: ptr.To("second")},
		},
		&googleapi.GroundingMetadata{
			GroundingChunks: []googleapi.GroundingChunk{{
				Web: &googleapi.Web{
					Uri:   ptr.To("https://example.com"),
					Title: ptr.To("Example"),
				},
			}},
			GroundingSupports: []googleapi.GoogleAiGenerativelanguageV1BetaGroundingSupport{{
				Segment: &googleapi.GoogleAiGenerativelanguageV1BetaSegment{
					PartIndex: ptr.To(2),
					Text:      ptr.To("second"),
				},
				// Repeated chunk references are preserved; the provider may
				// intentionally attribute the same source more than once.
				GroundingChunkIndices: []int{0, 0},
			}},
		},
	)
	if err != nil {
		t.Fatalf("mapGoogleContent returned an error: %v", err)
	}

	if len(content) != 2 {
		t.Fatalf("expected 2 mapped parts, got %d", len(content))
	}
	if len(content[0].TextPart.Citations) != 0 {
		t.Fatal("expected no citations on the first text part")
	}
	if len(content[1].TextPart.Citations) != 2 {
		t.Fatalf("expected 2 citations on the second text part, got %d", len(content[1].TextPart.Citations))
	}
}

func TestStreamingPreservesProviderToSDKPartMapping(t *testing.T) {
	mappings := map[int]int{}
	deltas, err := mapGoogleContentToDelta(
		googleapi.Content{Parts: []googleapi.Part{
			{},
			{Text: ptr.To("first")},
			{Text: ptr.To("second")},
		}},
		nil,
		mappings,
	)
	if err != nil {
		t.Fatalf("mapGoogleContentToDelta returned an error: %v", err)
	}

	if len(deltas) != 2 || deltas[0].Index != 0 || deltas[1].Index != 1 {
		t.Fatalf("expected mapped delta indices [0 1], got %+v", deltas)
	}
	if mappings[1] != 0 || mappings[2] != 1 {
		t.Fatalf("unexpected provider part mappings: %+v", mappings)
	}
}
