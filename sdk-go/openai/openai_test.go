package openai_test

import (
	"os"
	"testing"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/testcommon"
	"github.com/hoangvvo/llm-sdk/sdk-go/openai"
	"github.com/joho/godotenv"
)

func openAIAPIKey(t *testing.T) string {
	t.Helper()
	_ = godotenv.Load("../../.env")
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Fatal("OPENAI_API_KEY must be set")
	}
	return apiKey
}

func openAIModel(t *testing.T, modelID string) *openai.OpenAIModel {
	t.Helper()
	return openai.NewOpenAIModel(modelID, openai.OpenAIModelOptions{
		APIKey: openAIAPIKey(t),
	})
}

func openAIChatModel(t *testing.T, modelID string) *openai.OpenAIChatModel {
	t.Helper()
	return openai.NewOpenAIChatModel(modelID, openai.OpenAIChatModelOptions{
		APIKey: openAIAPIKey(t),
	})
}

func TestTextGeneration(t *testing.T) {
	testcommon.RunTestGroup(t, openAIModel(t, "gpt-5.6-sol"), "text_generation")
}

func TestConversation(t *testing.T) {
	testcommon.RunTestGroup(t, openAIModel(t, "gpt-5.6-sol"), "conversation")
}

func TestToolUse(t *testing.T) {
	testcommon.RunTestGroup(t, openAIModel(t, "gpt-5.6-sol"), "tool_use")
}

func TestStructuredOutput(t *testing.T) {
	testcommon.RunTestGroup(t, openAIModel(t, "gpt-5.6-sol"), "structured_output")
}

func TestGenerationOptions(t *testing.T) {
	testcommon.RunTestGroup(t, openAIModel(t, "gpt-5.6-sol"), "generation_options")
}

func TestSourceInput(t *testing.T) {
	testcommon.RunTestGroup(t, openAIModel(t, "gpt-5.6-sol"), "source_input")
}

func TestMultimodalToolResult(t *testing.T) {
	testcommon.RunTestGroup(t, openAIModel(t, "gpt-5.6-sol"), "multimodal_tool_result")
}

func TestWebSearch(t *testing.T) {
	testcommon.RunTestGroup(t, openAIModel(t, "gpt-5.6-sol"), "web_search")
}

func TestImageGeneration(t *testing.T) {
	testcommon.RunTestGroup(t, openAIModel(t, "gpt-5.6-sol"), "image_generation")
}

func TestImageInput(t *testing.T) {
	testcommon.RunTestGroup(t, openAIModel(t, "gpt-5.6-sol"), "image_input")
}

func TestReasoning(t *testing.T) {
	testcommon.RunTestGroup(t, openAIModel(t, "o1"), "reasoning", testcommon.WithProfile("openai_opaque_reasoning"))
}

func TestReasoningToolUse(t *testing.T) {
	testcommon.RunTestGroup(t, openAIModel(t, "gpt-5.6-sol"), "reasoning_tool_use")
}

func TestTransport(t *testing.T) {
	testcommon.RunTransportTestGroup(t, "openai_transport", func(baseURL string) llmsdk.LanguageModel {
		return openai.NewOpenAIModel("test-model", openai.OpenAIModelOptions{
			APIKey:  "test-token",
			BaseURL: baseURL + "/v1",
		})
	})
}
