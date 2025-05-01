package llmsdk

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestMockLanguageModelGenerate(t *testing.T) {
	model := NewMockLanguageModel()

	response1 := ModelResponse{
		Content: []Part{{TextPart: &TextPart{Text: "Hello, world!"}}},
	}
	response3 := ModelResponse{
		Content: []Part{{TextPart: &TextPart{Text: "Goodbye, world!"}}},
	}

	model.EnqueueGenerateResult(
		NewMockGenerateResultResponse(response1),
		NewMockGenerateResultError(errors.New("generate error")),
		NewMockGenerateResultResponse(response3),
	)

	ctx := context.Background()

	input1 := &LanguageModelInput{
		Messages: []Message{
			NewUserMessage(Part{TextPart: &TextPart{Text: "Hi"}}),
		},
	}
	res1, err := model.Generate(ctx, input1)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if !reflect.DeepEqual(res1, &response1) {
		t.Fatalf("unexpected first response: %+v", res1)
	}
	if len(model.TrackedGenerateInputs) != 1 || model.TrackedGenerateInputs[0] != input1 {
		t.Fatalf("generate inputs not tracked correctly: %+v", model.TrackedGenerateInputs)
	}

	input2 := &LanguageModelInput{
		Messages: []Message{
			NewUserMessage(Part{TextPart: &TextPart{Text: "Error"}}),
		},
	}
	if _, err := model.Generate(ctx, input2); err == nil || err.Error() != "generate error" {
		t.Fatalf("expected generate error, got %v", err)
	}
	if len(model.TrackedGenerateInputs) != 2 || model.TrackedGenerateInputs[1] != input2 {
		t.Fatalf("generate inputs not tracked after error: %+v", model.TrackedGenerateInputs)
	}

	input3 := &LanguageModelInput{
		Messages: []Message{
			NewUserMessage(Part{TextPart: &TextPart{Text: "Goodbye"}}),
		},
	}
	res3, err := model.Generate(ctx, input3)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if !reflect.DeepEqual(res3, &response3) {
		t.Fatalf("unexpected third response: %+v", res3)
	}
	if len(model.TrackedGenerateInputs) != 3 || model.TrackedGenerateInputs[2] != input3 {
		t.Fatalf("generate inputs not tracked after third call: %+v", model.TrackedGenerateInputs)
	}

	model.Reset()
	if len(model.TrackedGenerateInputs) != 0 {
		t.Fatalf("expected tracked inputs to be reset, got %d", len(model.TrackedGenerateInputs))
	}

	model.EnqueueGenerateResult(NewMockGenerateResultResponse(ModelResponse{
		Content: []Part{{TextPart: &TextPart{Text: "After reset"}}},
	}))

	model.Restore()
	if len(model.TrackedGenerateInputs) != 0 {
		t.Fatalf("expected tracked inputs to be empty after restore, got %d", len(model.TrackedGenerateInputs))
	}

	if _, err := model.Generate(ctx, input1); err == nil || err.Error() != "no mocked generate results available" {
		t.Fatalf("expected no mocked generate results error after restore, got %v", err)
	}
}

func TestMockLanguageModelStream(t *testing.T) {
	model := NewMockLanguageModel()

	partials1 := []PartialModelResponse{
		{Delta: &ContentDelta{Index: 0, Part: PartDelta{TextPartDelta: &TextPartDelta{Text: "Hello"}}}},
		{Delta: &ContentDelta{Index: 0, Part: PartDelta{TextPartDelta: &TextPartDelta{Text: ", "}}}},
		{Delta: &ContentDelta{Index: 0, Part: PartDelta{TextPartDelta: &TextPartDelta{Text: "world!"}}}},
	}
	partials3 := []PartialModelResponse{
		{Delta: &ContentDelta{Index: 0, Part: PartDelta{TextPartDelta: &TextPartDelta{Text: "Goodbye"}}}},
		{Delta: &ContentDelta{Index: 0, Part: PartDelta{TextPartDelta: &TextPartDelta{Text: ", "}}}},
		{Delta: &ContentDelta{Index: 0, Part: PartDelta{TextPartDelta: &TextPartDelta{Text: "world!"}}}},
	}

	model.EnqueueStreamResult(
		NewMockStreamResultPartials(partials1),
		NewMockStreamResultError(errors.New("stream error")),
		NewMockStreamResultPartials(partials3),
	)

	ctx := context.Background()

	streamInput1 := &LanguageModelInput{
		Messages: []Message{
			NewUserMessage(Part{TextPart: &TextPart{Text: "Hi"}}),
		},
	}
	stream1, err := model.Stream(ctx, streamInput1)
	if err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}
	gotPartials1 := collectStreamPartials(t, stream1)
	if !reflect.DeepEqual(gotPartials1, partials1) {
		t.Fatalf("unexpected partials from first stream: %+v", gotPartials1)
	}
	if len(model.TrackedStreamInputs) != 1 || model.TrackedStreamInputs[0] != streamInput1 {
		t.Fatalf("stream inputs not tracked correctly: %+v", model.TrackedStreamInputs)
	}

	streamInput2 := &LanguageModelInput{
		Messages: []Message{
			NewUserMessage(Part{TextPart: &TextPart{Text: "Error"}}),
		},
	}
	if _, err := model.Stream(ctx, streamInput2); err == nil || err.Error() != "stream error" {
		t.Fatalf("expected stream error, got %v", err)
	}
	if len(model.TrackedStreamInputs) != 2 || model.TrackedStreamInputs[1] != streamInput2 {
		t.Fatalf("stream inputs not tracked after error: %+v", model.TrackedStreamInputs)
	}

	streamInput3 := &LanguageModelInput{
		Messages: []Message{
			NewUserMessage(Part{TextPart: &TextPart{Text: "Goodbye"}}),
		},
	}
	stream3, err := model.Stream(ctx, streamInput3)
	if err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}
	gotPartials3 := collectStreamPartials(t, stream3)
	if !reflect.DeepEqual(gotPartials3, partials3) {
		t.Fatalf("unexpected partials from third stream: %+v", gotPartials3)
	}
	if len(model.TrackedStreamInputs) != 3 || model.TrackedStreamInputs[2] != streamInput3 {
		t.Fatalf("stream inputs not tracked after third call: %+v", model.TrackedStreamInputs)
	}

	model.Reset()
	if len(model.TrackedStreamInputs) != 0 {
		t.Fatalf("expected tracked stream inputs to be reset, got %d", len(model.TrackedStreamInputs))
	}

	model.EnqueueStreamResult(NewMockStreamResultPartials([]PartialModelResponse{
		{Delta: &ContentDelta{Index: 0, Part: PartDelta{TextPartDelta: &TextPartDelta{Text: "After reset"}}}},
	}))

	model.Restore()
	if len(model.TrackedStreamInputs) != 0 {
		t.Fatalf("expected tracked stream inputs to be empty after restore, got %d", len(model.TrackedStreamInputs))
	}

	if _, err := model.Stream(ctx, streamInput1); err == nil || err.Error() != "no mocked stream results available" {
		t.Fatalf("expected no mocked stream results error after restore, got %v", err)
	}
}

func collectStreamPartials(t *testing.T, stream *LanguageModelStream) []PartialModelResponse {
	t.Helper()
	var partials []PartialModelResponse
	for stream.Next() {
		current := stream.Current()
		if current != nil {
			partials = append(partials, *current)
		}
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream error: %v", err)
	}
	return partials
}
