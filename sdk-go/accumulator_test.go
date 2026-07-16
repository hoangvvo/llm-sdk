package llmsdk_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"
)

func TestStreamAccumulator_ReconstructsInterleavedMultipartStream(t *testing.T) {
	accumulator := llmsdk.NewStreamAccumulator()
	partials := []llmsdk.PartialModelResponse{
		{
			Delta: &llmsdk.ContentDelta{Index: 2, Part: llmsdk.NewReasoningPartDelta("think ")},
			Usage: &llmsdk.ModelUsage{InputTokens: 2, OutputTokens: 1},
			Cost:  ptr.To(0.1),
		},
		{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta("Hel")}},
		{Delta: &llmsdk.ContentDelta{Index: 1, Part: llmsdk.NewToolCallPartDelta(
			llmsdk.WithToolCallPartDeltaToolCallID("call_1"),
			llmsdk.WithToolCallPartDeltaToolName("weather"),
			llmsdk.WithToolCallPartDeltaArgs(`{"city":`),
		)}},
		{
			Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta("lo")},
			Usage: &llmsdk.ModelUsage{InputTokens: 3, OutputTokens: 4},
			Cost:  ptr.To(0.2),
		},
		{Delta: &llmsdk.ContentDelta{Index: 1, Part: llmsdk.NewToolCallPartDelta(
			llmsdk.WithToolCallPartDeltaArgs(`"Paris"}`),
		)}},
		{Delta: &llmsdk.ContentDelta{Index: 2, Part: llmsdk.NewReasoningPartDelta(
			"done",
			llmsdk.WithReasoningPartDeltaSignature("sig"),
		)}},
	}

	for _, partial := range partials {
		if err := accumulator.AddPartial(partial); err != nil {
			t.Fatalf("add partial: %v", err)
		}
	}
	if accumulator.Size() != 3 || accumulator.IsEmpty() {
		t.Fatalf("unexpected accumulator state: size=%d empty=%v", accumulator.Size(), accumulator.IsEmpty())
	}

	response, err := accumulator.ComputeResponse()
	if err != nil {
		t.Fatalf("compute response: %v", err)
	}
	expected := llmsdk.ModelResponse{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart("Hello"),
			llmsdk.NewToolCallPart("call_1", "weather", map[string]any{"city": "Paris"}),
			llmsdk.NewReasoningPart("think done", llmsdk.WithReasoningSignature("sig")),
		},
		Usage: &llmsdk.ModelUsage{InputTokens: 5, OutputTokens: 5},
		Cost:  ptr.To(0.30000000000000004),
	}
	if diff := cmp.Diff(expected, response); diff != "" {
		t.Fatalf("response mismatch (-want +got):\n%s", diff)
	}
}

func TestStreamAccumulator_RejectsMismatchedAndMalformedDeltas(t *testing.T) {
	t.Run("part type changes", func(t *testing.T) {
		accumulator := llmsdk.NewStreamAccumulator()
		if err := accumulator.AddPartial(llmsdk.PartialModelResponse{
			Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta("hello")},
		}); err != nil {
			t.Fatalf("add initial partial: %v", err)
		}
		if err := accumulator.AddPartial(llmsdk.PartialModelResponse{
			Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewReasoningPartDelta("wrong")},
		}); err == nil {
			t.Fatal("expected a part-type mismatch error")
		}
	})

	t.Run("malformed tool arguments", func(t *testing.T) {
		accumulator := llmsdk.NewStreamAccumulator()
		if err := accumulator.AddPartial(llmsdk.PartialModelResponse{
			Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewToolCallPartDelta(
				llmsdk.WithToolCallPartDeltaToolCallID("call_1"),
				llmsdk.WithToolCallPartDeltaToolName("weather"),
				llmsdk.WithToolCallPartDeltaArgs("{bad json"),
			)},
		}); err != nil {
			t.Fatalf("add partial: %v", err)
		}
		if _, err := accumulator.ComputeResponse(); err == nil {
			t.Fatal("expected malformed tool arguments to fail")
		}
	})
}

func TestStreamAccumulator_ClearRemovesContentAndMetadata(t *testing.T) {
	accumulator := llmsdk.NewStreamAccumulator()
	if err := accumulator.AddPartial(llmsdk.PartialModelResponse{
		Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta("old")},
		Usage: &llmsdk.ModelUsage{InputTokens: 2, OutputTokens: 1},
		Cost:  ptr.To(0.4),
	}); err != nil {
		t.Fatalf("add partial: %v", err)
	}

	accumulator.Clear()
	response, err := accumulator.ComputeResponse()
	if err != nil {
		t.Fatalf("compute cleared response: %v", err)
	}
	if !accumulator.IsEmpty() || response.Usage != nil || response.Cost != nil || len(response.Content) != 0 {
		t.Fatalf("clear retained state: %#v", response)
	}
}

func TestStreamAccumulator_SnapshotsIndependentlyMaterializableParts(t *testing.T) {
	accumulator := llmsdk.NewStreamAccumulator()
	partials := []llmsdk.PartialModelResponse{
		{
			Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta("partial")},
			Usage: &llmsdk.ModelUsage{InputTokens: 2, OutputTokens: 3},
			Cost:  ptr.To(0.25),
		},
		{Delta: &llmsdk.ContentDelta{Index: 1, Part: llmsdk.NewToolCallPartDelta(
			llmsdk.WithToolCallPartDeltaToolCallID("call_1"),
			llmsdk.WithToolCallPartDeltaToolName("weather"),
			llmsdk.WithToolCallPartDeltaArgs(`{"city":"Paris"}`),
		)}},
		{Delta: &llmsdk.ContentDelta{Index: 2, Part: llmsdk.NewToolCallPartDelta(
			llmsdk.WithToolCallPartDeltaArgs("{incomplete"),
		)}},
		{Delta: &llmsdk.ContentDelta{Index: 3, Part: llmsdk.NewImagePartDelta(
			llmsdk.WithImagePartDeltaData("aGVsbG8="),
			llmsdk.WithImagePartDeltaMimeType("image/png"),
		)}},
		{Delta: &llmsdk.ContentDelta{Index: 4, Part: llmsdk.NewAudioPartDelta(
			llmsdk.WithAudioPartDeltaData("AAABAA=="),
			llmsdk.WithAudioPartDeltaFormat(llmsdk.AudioFormatLinear16),
		)}},
	}
	for _, partial := range partials {
		if err := accumulator.AddPartial(partial); err != nil {
			t.Fatalf("add partial: %v", err)
		}
	}

	expected := llmsdk.ModelResponse{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart("partial"),
			llmsdk.NewToolCallPart("call_1", "weather", map[string]any{"city": "Paris"}),
			llmsdk.NewImagePart("aGVsbG8=", "image/png"),
			llmsdk.NewAudioPart("AAABAA==", llmsdk.AudioFormatLinear16),
		},
		Usage: &llmsdk.ModelUsage{InputTokens: 2, OutputTokens: 3},
		Cost:  ptr.To(0.25),
	}
	if diff := cmp.Diff(expected, accumulator.Snapshot()); diff != "" {
		t.Fatalf("snapshot mismatch (-want +got):\n%s", diff)
	}
	if _, err := accumulator.ComputeResponse(); err == nil {
		t.Fatal("strict response unexpectedly accepted an incomplete part")
	}
}
