package openai_test

import (
	"context"
	"os"
	"testing"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/testcommon"
	"github.com/hoangvvo/llm-sdk/sdk-go/openai"
	"github.com/joho/godotenv"
)

var model *openai.OpenAIModel
var audioModel *openai.OpenAIModel

func TestMain(m *testing.M) {
	godotenv.Load("../../.env")
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		panic("OPENAI_API_KEY must be set")
	}
	model = openai.NewOpenAIModel(openai.OpenAIModelOptions{
		APIKey:  apiKey,
		ModelID: "gpt-4o",
	}).WithMetadata(&llmsdk.LanguageModelMetadata{
		Capabilities: []llmsdk.LanguageModelCapability{
			llmsdk.CapabilityFunctionCalling,
			llmsdk.CapabilityImageInput,
			llmsdk.CapabilityStructuredOutput,
		},
	})
	audioModel = openai.NewOpenAIModel(openai.OpenAIModelOptions{
		APIKey:  apiKey,
		ModelID: "gpt-4o-audio-preview",
	}).WithMetadata(&llmsdk.LanguageModelMetadata{
		Capabilities: []llmsdk.LanguageModelCapability{
			llmsdk.CapabilityAudioInput,
			llmsdk.CapabilityAudioOutput,
			llmsdk.CapabilityFunctionCalling,
			llmsdk.CapabilityImageInput,
			llmsdk.CapabilityStructuredOutput,
		},
	})
	m.Run()
}

func TestGenerateText(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseGenerateText)
}

func TestStreamText(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseStreamText)
}

func TestGenerateWithSystemPrompt(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseGenerateWithSystemPrompt)
}

func TestGenerateToolCall(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseGenerateToolCall)
}

func TestStreamToolCall(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseStreamToolCall)
}

func TestGenerateTextWithToolResult(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseGenerateTextWithToolResult)
}

func TestStreamTextWithToolResult(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseStreamTextWithToolResult)
}

func TestGenerateParallelToolCalls(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseGenerateParallelToolCalls)
}

func TestStreamParallelToolCalls(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseStreamParallelToolCalls)
}

func TestStreamParallelToolCallsOfSameName(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseStreamParallelToolCallsOfSameName)
}

func TestStructuredResponseFormat(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseStructuredResponseFormat)
}

func TestSourcePartInput(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseSourcePartInput)
}

func TestGenerateAudio(t *testing.T) {
	ctx := context.Background()
	response, err := audioModel.Generate(ctx, &llmsdk.LanguageModelInput{
		Modalities: []llmsdk.Modality{llmsdk.ModalityText, llmsdk.ModalityAudio},
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(
				llmsdk.NewTextPart("Hello", nil),
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

func TestStreamAudio(t *testing.T) {
	ctx := context.Background()
	stream, err := audioModel.Stream(ctx, &llmsdk.LanguageModelInput{
		Modalities: []llmsdk.Modality{llmsdk.ModalityText, llmsdk.ModalityAudio},
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(
				llmsdk.NewTextPart("Hello", nil),
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

	if audioPart.ID == nil || *audioPart.ID == "" {
		t.Fatal("Audio part ID must be present")
	}
}
