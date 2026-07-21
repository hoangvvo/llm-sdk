package google_test

import (
	"os"
	"testing"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/google"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/testcommon"
	"github.com/joho/godotenv"
)

func googleModel(t *testing.T, modelID string) *google.GoogleModel {
	t.Helper()
	_ = godotenv.Load("../../.env")
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		t.Fatal("GOOGLE_API_KEY must be set")
	}
	return google.NewGoogleModel(modelID, google.GoogleModelOptions{
		APIKey: apiKey,
	})
}

func TestTextGeneration(t *testing.T) {
	testcommon.RunTestGroup(t, googleModel(t, "gemini-3.1-flash-lite"), "text_generation")
}

func TestConversation(t *testing.T) {
	testcommon.RunTestGroup(t, googleModel(t, "gemini-3.1-flash-lite"), "conversation")
}

func TestToolUse(t *testing.T) {
	testcommon.RunTestGroup(t, googleModel(t, "gemini-3.1-flash-lite"), "tool_use")
}

func TestStructuredOutput(t *testing.T) {
	testcommon.RunTestGroup(t, googleModel(t, "gemini-3.1-flash-lite"), "structured_output")
}

func TestGenerationOptions(t *testing.T) {
	testcommon.RunTestGroup(t, googleModel(t, "gemini-3.1-flash-lite"), "generation_options")
}

func TestSourceInput(t *testing.T) {
	testcommon.RunTestGroup(t, googleModel(t, "gemini-3.1-flash-lite"), "source_input")
}

func TestMultimodalToolResult(t *testing.T) {
	testcommon.RunTestGroup(t, googleModel(t, "gemini-3.1-pro-preview"), "multimodal_tool_result")
}

func TestWebSearch(t *testing.T) {
	testcommon.RunTestGroup(t, googleModel(t, "gemini-3.1-flash-lite"), "web_search", testcommon.WithProfile("google_web_search"))
}

func TestImageGeneration(t *testing.T) {
	testcommon.RunTestGroup(t, googleModel(t, "gemini-3.1-flash-image"), "image_generation")
}

func TestImageInput(t *testing.T) {
	testcommon.RunTestGroup(t, googleModel(t, "gemini-3.1-flash-image"), "image_input")
}

func TestAudioGeneration(t *testing.T) {
	testcommon.RunTestGroup(t, googleModel(t, "gemini-3.1-flash-tts-preview"), "audio_generation", testcommon.WithProfile("google_audio"))
}

func TestReasoning(t *testing.T) {
	testcommon.RunTestGroup(t, googleModel(t, "gemini-3.1-pro-preview"), "reasoning")
}

func TestReasoningToolUse(t *testing.T) {
	testcommon.RunTestGroup(t, googleModel(t, "gemini-3.1-pro-preview"), "reasoning_tool_use")
}

func TestTransport(t *testing.T) {
	testcommon.RunTransportTestGroup(t, "google_transport", func(baseURL string) llmsdk.LanguageModel {
		return google.NewGoogleModel("test-model", google.GoogleModelOptions{
			APIKey:  "test-token",
			BaseURL: baseURL,
		})
	})
}
