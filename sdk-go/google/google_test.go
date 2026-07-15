package google_test

import (
	"os"
	"testing"

	"github.com/hoangvvo/llm-sdk/sdk-go/google"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/testcommon"
	"github.com/joho/godotenv"
)

var model *google.GoogleModel
var audioModel *google.GoogleModel
var imageModel *google.GoogleModel
var multimodalToolModel *google.GoogleModel
var reasoningModel *google.GoogleModel

func TestMain(m *testing.M) {
	godotenv.Load("../../.env")
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		panic("GOOGLE_API_KEY must be set")
	}

	model = google.NewGoogleModel("gemini-3.1-flash-lite", google.GoogleModelOptions{
		APIKey: apiKey,
	})
	audioModel = google.NewGoogleModel("gemini-3.1-flash-tts-preview", google.GoogleModelOptions{
		APIKey: apiKey,
	})
	imageModel = google.NewGoogleModel("gemini-3.1-flash-image", google.GoogleModelOptions{
		APIKey: apiKey,
	})
	multimodalToolModel = google.NewGoogleModel("gemini-3.1-pro-preview", google.GoogleModelOptions{
		APIKey: apiKey,
	})
	reasoningModel = google.NewGoogleModel("gemini-3.1-pro-preview", google.GoogleModelOptions{
		APIKey: apiKey,
	})

	m.Run()
}

func TestTextGeneration(t *testing.T) {
	testcommon.RunTestGroup(t, model, "text_generation")
}

func TestConversation(t *testing.T) {
	testcommon.RunTestGroup(t, model, "conversation")
}

func TestToolUse(t *testing.T) {
	testcommon.RunTestGroup(t, model, "tool_use")
}

func TestStructuredOutput(t *testing.T) {
	testcommon.RunTestGroup(t, model, "structured_output")
}

func TestGenerationOptions(t *testing.T) {
	testcommon.RunTestGroup(t, model, "generation_options")
}

func TestSourceInput(t *testing.T) {
	testcommon.RunTestGroup(t, model, "source_input")
}

func TestMultimodalToolResult(t *testing.T) {
	testcommon.RunTestGroup(t, multimodalToolModel, "multimodal_tool_result")
}

func TestWebSearch(t *testing.T) {
	testcommon.RunTestGroup(t, model, "web_search", testcommon.WithProfile("google_web_search"))
}

func TestImageGeneration(t *testing.T) {
	testcommon.RunTestGroup(t, imageModel, "image_generation")
}

func TestImageInput(t *testing.T) {
	testcommon.RunTestGroup(t, imageModel, "image_input")
}

func TestAudioGeneration(t *testing.T) {
	testcommon.RunTestGroup(t, audioModel, "audio_generation", testcommon.WithProfile("google_audio"))
}

func TestReasoning(t *testing.T) {
	testcommon.RunTestGroup(t, reasoningModel, "reasoning")
}

func TestReasoningToolUse(t *testing.T) {
	testcommon.RunTestGroup(t, reasoningModel, "reasoning_tool_use")
}
