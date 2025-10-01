package llmsdktest_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/llmsdktest"
)

func TestMockLanguageModelGenerate(t *testing.T) {
	model := llmsdktest.NewMockLanguageModel()

	response1 := llmsdk.ModelResponse{
		Content: []llmsdk.Part{llmsdk.NewTextPart("Hello, world!")},
	}
	response3 := llmsdk.ModelResponse{
		Content: []llmsdk.Part{llmsdk.NewTextPart("Goodbye, world!")},
	}

	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(response1),
		llmsdktest.NewMockGenerateResultError(errors.New("generate error")),
		llmsdktest.NewMockGenerateResultResponse(response3),
	)

	ctx := t.Context()

	input1 := &llmsdk.LanguageModelInput{
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(llmsdk.NewTextPart("Hi")),
		},
	}
	res1, err := model.Generate(ctx, input1)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if diff := cmp.Diff(res1, &response1); diff != "" {
		t.Errorf("unexpected first response (-want +got):\n%s", diff)
	}
	trackedGenerateInputs := model.TrackedGenerateInputs()
	if len(trackedGenerateInputs) != 1 {
		t.Fatalf("expected 1 tracked generate input, got %d", len(trackedGenerateInputs))
	}
	if diff := cmp.Diff(trackedGenerateInputs[0], *input1); diff != "" {
		t.Errorf("tracked generate input mismatch (-want +got):\n%s", diff)
	}

	input2 := &llmsdk.LanguageModelInput{
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(llmsdk.NewTextPart("Error")),
		},
	}
	if _, err := model.Generate(ctx, input2); err == nil || err.Error() != "generate error" {
		t.Errorf("expected generate error, got %v", err)
	}
	trackedGenerateInputs = model.TrackedGenerateInputs()
	if len(trackedGenerateInputs) != 2 {
		t.Fatalf("expected 2 tracked generate inputs, got %d", len(trackedGenerateInputs))
	}
	if diff := cmp.Diff(trackedGenerateInputs[1], *input2); diff != "" {
		t.Errorf("tracked generate input mismatch (-want +got):\n%s", diff)
	}

	input3 := &llmsdk.LanguageModelInput{
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(llmsdk.NewTextPart("Goodbye")),
		},
	}
	res3, err := model.Generate(ctx, input3)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if diff := cmp.Diff(res3, &response3); diff != "" {
		t.Errorf("unexpected third response (-want +got):\n%s", diff)
	}
	trackedGenerateInputs = model.TrackedGenerateInputs()
	if len(trackedGenerateInputs) != 3 {
		t.Fatalf("expected 3 tracked generate inputs, got %d", len(trackedGenerateInputs))
	}
	if diff := cmp.Diff(trackedGenerateInputs[2], *input3); diff != "" {
		t.Errorf("tracked generate input mismatch (-want +got):\n%s", diff)
	}

	model.Reset()
	trackedGenerateInputs = model.TrackedGenerateInputs()
	if len(trackedGenerateInputs) != 0 {
		t.Errorf("expected tracked inputs to be reset, got %d", len(trackedGenerateInputs))
	}

	model.EnqueueGenerateResult(llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
		Content: []llmsdk.Part{llmsdk.NewTextPart("After reset")},
	}))

	model.Restore()
	if len(model.TrackedGenerateInputs()) != 0 {
		t.Errorf("expected tracked inputs to be empty after restore, got %d", len(model.TrackedGenerateInputs()))
	}

	if _, err := model.Generate(ctx, input1); err == nil || err.Error() != "no mocked generate results available" {
		t.Errorf("expected no mocked generate results error after restore, got %v", err)
	}
}

func TestMockLanguageModelStream(t *testing.T) {
	model := llmsdktest.NewMockLanguageModel()

	partials1 := []llmsdk.PartialModelResponse{
		{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta("Hello")}},
		{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta(", ")}},
		{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta("world!")}},
	}
	partials3 := []llmsdk.PartialModelResponse{
		{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta("Goodbye")}},
		{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta(", ")}},
		{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta("world!")}},
	}

	model.EnqueueStreamResult(
		llmsdktest.NewMockStreamResultPartials(partials1),
		llmsdktest.NewMockStreamResultError(errors.New("stream error")),
		llmsdktest.NewMockStreamResultPartials(partials3),
	)

	ctx := t.Context()

	streamInput1 := &llmsdk.LanguageModelInput{
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(llmsdk.NewTextPart("Hi")),
		},
	}
	stream1, err := model.Stream(ctx, streamInput1)
	if err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}
	gotPartials1 := collectStreamPartials(t, stream1)
	if diff := cmp.Diff(gotPartials1, partials1); diff != "" {
		t.Errorf("unexpected partials from first stream (-want +got):\n%s", diff)
	}
	trackedStreamInputs := model.TrackedStreamInputs()
	if len(trackedStreamInputs) != 1 {
		t.Fatalf("expected 1 tracked stream input, got %d", len(trackedStreamInputs))
	}
	if diff := cmp.Diff(trackedStreamInputs[0], *streamInput1); diff != "" {
		t.Errorf("tracked stream input mismatch (-want +got):\n%s", diff)
	}

	streamInput2 := &llmsdk.LanguageModelInput{
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(llmsdk.NewTextPart("Error")),
		},
	}
	if _, err := model.Stream(ctx, streamInput2); err == nil || err.Error() != "stream error" {
		t.Errorf("expected stream error, got %v", err)
	}
	trackedStreamInputs = model.TrackedStreamInputs()
	if len(trackedStreamInputs) != 2 {
		t.Fatalf("expected 2 tracked stream inputs, got %d", len(trackedStreamInputs))
	}
	if diff := cmp.Diff(trackedStreamInputs[1], *streamInput2); diff != "" {
		t.Errorf("tracked stream input mismatch (-want +got):\n%s", diff)
	}

	streamInput3 := &llmsdk.LanguageModelInput{
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(llmsdk.NewTextPart("Goodbye")),
		},
	}
	stream3, err := model.Stream(ctx, streamInput3)
	if err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}
	gotPartials3 := collectStreamPartials(t, stream3)
	if diff := cmp.Diff(gotPartials3, partials3); diff != "" {
		t.Errorf("unexpected partials from third stream (-want +got):\n%s", diff)
	}
	trackedStreamInputs = model.TrackedStreamInputs()
	if len(trackedStreamInputs) != 3 {
		t.Fatalf("expected 3 tracked stream inputs, got %d", len(trackedStreamInputs))
	}
	if diff := cmp.Diff(trackedStreamInputs[2], *streamInput3); diff != "" {
		t.Errorf("tracked stream input mismatch (-want +got):\n%s", diff)
	}

	model.Reset()
	if len(model.TrackedStreamInputs()) != 0 {
		t.Errorf("expected tracked stream inputs to be reset, got %d", len(model.TrackedStreamInputs()))
	}

	model.EnqueueStreamResult(llmsdktest.NewMockStreamResultPartials([]llmsdk.PartialModelResponse{
		{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta("After reset")}},
	}))

	model.Restore()
	if len(model.TrackedStreamInputs()) != 0 {
		t.Errorf("expected tracked stream inputs to be empty after restore, got %d", len(model.TrackedStreamInputs()))
	}

	if _, err := model.Stream(ctx, streamInput1); err == nil || err.Error() != "no mocked stream results available" {
		t.Errorf("expected no mocked stream results error after restore, got %v", err)
	}
}

func collectStreamPartials(t *testing.T, stream *llmsdk.LanguageModelStream) []llmsdk.PartialModelResponse {
	t.Helper()
	var partials []llmsdk.PartialModelResponse
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
