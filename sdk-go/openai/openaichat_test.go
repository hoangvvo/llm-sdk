package openai_test

import (
	"context"
	"testing"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/testcommon"
)

func TestChatGenerateText(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseGenerateText)
}

func TestChatStreamText(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseStreamText)
}

func TestChatGenerateWithSystemPrompt(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseGenerateWithSystemPrompt)
}

func TestChatGenerateToolCall(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseGenerateToolCall)
}

func TestChatStreamToolCall(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseStreamToolCall)
}

func TestChatGenerateTextWithToolResult(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseGenerateTextWithToolResult)
}

func TestChatStreamTextWithToolResult(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseStreamTextWithToolResult)
}

func TestChatGenerateParallelToolCalls(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseGenerateParallelToolCalls)
}

func TestChatStreamParallelToolCalls(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseStreamParallelToolCalls)
}

func TestChatStreamParallelToolCallsOfSameName(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseStreamParallelToolCallsOfSameName)
}

func TestChatStructuredResponseFormat(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseStructuredResponseFormat)
}

func TestChatSourcePartInput(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseSourcePartInput)
}

func TestChatGenerateAudio(t *testing.T) {
	ctx := context.Background()
	response, err := audioChatModel.Generate(ctx, &llmsdk.LanguageModelInput{
		Modalities: []llmsdk.Modality{llmsdk.ModalityText, llmsdk.ModalityAudio},
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(
				llmsdk.NewTextPart("Hello"),
			),
		},
		Extra: map[string]any{
			"audio": map[string]any{
				"voice":  "alloy",
				"format": "mp3",
			},
		},
	})

	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Find audio part
	var audioPart *llmsdk.AudioPart
	for _, part := range response.Content {
		if part.AudioPart != nil {
			audioPart = part.AudioPart
			break
		}
	}

	if audioPart == nil {
		t.Fatal("Audio part must be present")
	}

	if audioPart.AudioData == "" {
		t.Fatal("Audio data must be present")
	}

	if audioPart.Transcript == nil || *audioPart.Transcript == "" {
		t.Fatal("Transcript must be present")
	}
}

func TestChatStreamAudio(t *testing.T) {
	ctx := context.Background()
	stream, err := audioChatModel.Stream(ctx, &llmsdk.LanguageModelInput{
		Modalities: []llmsdk.Modality{llmsdk.ModalityText, llmsdk.ModalityAudio},
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(
				llmsdk.NewTextPart("Hello"),
			),
		},
		Extra: map[string]any{
			"audio": map[string]any{
				"voice":  "alloy",
				"format": "pcm16",
			},
		},
	})

	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	accumulator := llmsdk.NewStreamAccumulator()
	for stream.Next() {
		partial := stream.Current()
		if err := accumulator.AddPartial(*partial); err != nil {
			t.Fatalf("Failed to add partial: %v", err)
		}
	}

	if err := stream.Err(); err != nil {
		t.Fatalf("Stream error: %v", err)
	}

	response, err := accumulator.ComputeResponse()
	if err != nil {
		t.Fatalf("Failed to compute response: %v", err)
	}

	// Find audio part
	var audioPart *llmsdk.AudioPart
	for _, part := range response.Content {
		if part.AudioPart != nil {
			audioPart = part.AudioPart
			break
		}
	}

	if audioPart == nil {
		t.Fatal("Audio part must be present")
	}

	if audioPart.AudioData == "" {
		t.Fatal("Audio data must be present")
	}

	if audioPart.Transcript == nil || *audioPart.Transcript == "" {
		t.Fatal("Transcript must be present")
	}

	if audioPart.AudioID == nil || *audioPart.AudioID == "" {
		t.Fatal("Audio part ID must be present")
	}
}
