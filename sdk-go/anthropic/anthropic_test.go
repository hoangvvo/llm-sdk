package anthropic_test

import (
	"os"
	"testing"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/anthropic"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/testcommon"
	"github.com/joho/godotenv"
)

func anthropicModel(t *testing.T) *anthropic.AnthropicModel {
	t.Helper()
	_ = godotenv.Load("../../.env")
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Fatal("ANTHROPIC_API_KEY must be set")
	}
	return anthropic.NewAnthropicModel("claude-sonnet-5", anthropic.AnthropicModelOptions{
		APIKey: apiKey,
	})
}

var reasoningOptions = []testcommon.TestCaseOption{
	testcommon.WithProfile("anthropic_adaptive_reasoning"),
}

func TestTextGeneration(t *testing.T) {
	testcommon.RunTestGroup(t, anthropicModel(t), "text_generation")
}

func TestConversation(t *testing.T) {
	testcommon.RunTestGroup(t, anthropicModel(t), "conversation")
}

func TestToolUse(t *testing.T) {
	testcommon.RunTestGroup(t, anthropicModel(t), "tool_use")
}

func TestStructuredOutput(t *testing.T) {
	testcommon.RunTestGroup(t, anthropicModel(t), "structured_output")
}

func TestGenerationOptions(t *testing.T) {
	testcommon.RunTestGroup(t, anthropicModel(t), "generation_options")
}

func TestSourceInput(t *testing.T) {
	testcommon.RunTestGroup(t, anthropicModel(t), "source_input")
}

func TestMultimodalToolResult(t *testing.T) {
	testcommon.RunTestGroup(t, anthropicModel(t), "multimodal_tool_result")
}

func TestWebSearch(t *testing.T) {
	testcommon.RunTestGroup(t, anthropicModel(t), "web_search", testcommon.WithProfile("anthropic_web_search"))
}

func TestImageInput(t *testing.T) {
	testcommon.RunTestGroup(t, anthropicModel(t), "image_input")
}

func TestReasoning(t *testing.T) {
	testcommon.RunTestGroup(t, anthropicModel(t), "reasoning", reasoningOptions...)
}

func TestReasoningToolUse(t *testing.T) {
	testcommon.RunTestGroup(t, anthropicModel(t), "reasoning_tool_use")
}

func TestAnthropicRefusal(t *testing.T) {
	testcommon.RunTestGroup(t, anthropicModel(t), "anthropic_refusal")
}

func TestAnthropicWebSearchFailure(t *testing.T) {
	testcommon.RunTestGroup(t, anthropicModel(t), "anthropic_web_search_failure")
}

func TestTransport(t *testing.T) {
	testcommon.RunTransportTestGroup(t, "anthropic_transport", func(baseURL string) llmsdk.LanguageModel {
		return anthropic.NewAnthropicModel("test-model", anthropic.AnthropicModelOptions{
			APIKey:  "test-token",
			BaseURL: baseURL,
		})
	})
}
