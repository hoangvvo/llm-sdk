package google_test

import (
	"os"
	"testing"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/google"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/testcommon"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"
	"github.com/joho/godotenv"
)

var model *google.GoogleModel
var audioModel *google.GoogleModel
var imageModel *google.GoogleModel
var reasoningModel *google.GoogleModel

func TestMain(m *testing.M) {
	godotenv.Load("../../.env")
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		panic("GOOGLE_API_KEY must be set")
	}

	model = google.NewGoogleModel("gemini-2.5-flash", google.GoogleModelOptions{
		APIKey: apiKey,
	})
	audioModel = google.NewGoogleModel("gemini-2.5-flash-preview-tts", google.GoogleModelOptions{
		APIKey: apiKey,
	})
	imageModel = google.NewGoogleModel("gemini-2.5-flash-image-preview", google.GoogleModelOptions{
		APIKey: apiKey,
	})
	reasoningModel = google.NewGoogleModel("gemini-2.0-flash-thinking-exp-01-21", google.GoogleModelOptions{
		APIKey: apiKey,
	})

	m.Run()
}

func TestGenerateText(t *testing.T) {
	testcommon.RunTestCase(t, model, "generate_text")
}

func TestStreamText(t *testing.T) {
	testcommon.RunTestCase(t, model, "stream_text")
}

func TestGenerateWithSystemPrompt(t *testing.T) {
	testcommon.RunTestCase(t, model, "generate_with_system_prompt")
}

func TestGenerateToolCall(t *testing.T) {
	testcommon.RunTestCase(t, model, "generate_tool_call")
}

func TestStreamToolCall(t *testing.T) {
	testcommon.RunTestCase(t, model, "stream_tool_call")
}

func TestGenerateTextWithToolResult(t *testing.T) {
	testcommon.RunTestCase(t, model, "generate_text_from_tool_result")
}

func TestStreamTextWithToolResult(t *testing.T) {
	testcommon.RunTestCase(t, model, "stream_text_from_tool_result")
}

func TestGenerateParallelToolCalls(t *testing.T) {
	testcommon.RunTestCase(t, model, "generate_parallel_tool_calls")
}

func TestStreamParallelToolCalls(t *testing.T) {
	testcommon.RunTestCase(t, model, "stream_parallel_tool_calls")
}

func TestStreamParallelToolCallsOfSameName(t *testing.T) {
	testcommon.RunTestCase(t, model, "stream_parallel_tool_calls_of_same_name")
}

func TestStructuredResponseFormat(t *testing.T) {
	testcommon.RunTestCase(t, model, "structured_response_format")
}

func TestSourcePartInput(t *testing.T) {
	testcommon.RunTestCase(t, model, "source_part_input")
}

func TestGenerateImage(t *testing.T) {
	testcommon.RunTestCase(t, imageModel, "generate_image")
}

func TestStreamImage(t *testing.T) {
	testcommon.RunTestCase(t, imageModel, "stream_image")
}

func TestGenerateAudio(t *testing.T) {
	testcommon.RunTestCase(t, audioModel, "generate_audio",
		testcommon.WithAdditionalInput(func(lmi *llmsdk.LanguageModelInput) {
			lmi.Modalities = []llmsdk.Modality{llmsdk.ModalityAudio}
			lmi.Audio = &llmsdk.AudioOptions{Voice: ptr.To("Zephyr")}
		}),
		testcommon.WithCustomOutputContent(func(content []testcommon.PartAssertion) []testcommon.PartAssertion {
			newContent := []testcommon.PartAssertion{}
			for _, part := range content {
				if part.AudioPart != nil {
					part.AudioPart.AudioID = false
					part.AudioPart.Transcript = nil
				}
				newContent = append(newContent, part)
			}
			return newContent
		}),
	)
}

func TestStreamAudio(t *testing.T) {
	testcommon.RunTestCase(
		t, audioModel, "stream_audio",
		testcommon.WithAdditionalInput(func(lmi *llmsdk.LanguageModelInput) {
			lmi.Modalities = []llmsdk.Modality{llmsdk.ModalityAudio}
			lmi.Audio = &llmsdk.AudioOptions{Voice: ptr.To("Zephyr")}
		}),
		testcommon.WithCustomOutputContent(func(content []testcommon.PartAssertion) []testcommon.PartAssertion {
			newContent := []testcommon.PartAssertion{}
			for _, part := range content {
				if part.AudioPart != nil {
					part.AudioPart.AudioID = false
					part.AudioPart.Transcript = nil
				}
				newContent = append(newContent, part)
			}
			return newContent
		}),
	)
}

func TestGenerateReasoning(t *testing.T) {
	testcommon.RunTestCase(t, reasoningModel, "generate_reasoning")
}

func TestStreamReasoning(t *testing.T) {
	testcommon.RunTestCase(t, reasoningModel, "stream_reasoning")
}
