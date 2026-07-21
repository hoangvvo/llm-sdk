package anthropic_test

import (
	"os"
	"testing"

	"github.com/hoangvvo/llm-sdk/sdk-go/anthropic"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/testcommon"
	"github.com/joho/godotenv"
)

var model *anthropic.AnthropicModel

func TestMain(m *testing.M) {
	godotenv.Load("../../.env")
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		panic("ANTHROPIC_API_KEY must be set")
	}

	model = anthropic.NewAnthropicModel("claude-sonnet-5", anthropic.AnthropicModelOptions{
		APIKey: apiKey,
	})

	m.Run()
}

var reasoningOptions = []testcommon.TestCaseOption{
	testcommon.WithProfile("anthropic_adaptive_reasoning"),
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
	testcommon.RunTestGroup(t, model, "multimodal_tool_result")
}

func TestWebSearch(t *testing.T) {
	testcommon.RunTestGroup(t, model, "web_search", testcommon.WithProfile("anthropic_web_search"))
}

func TestImageInput(t *testing.T) {
	testcommon.RunTestGroup(t, model, "image_input")
}

func TestReasoning(t *testing.T) {
	testcommon.RunTestGroup(t, model, "reasoning", reasoningOptions...)
}

func TestReasoningToolUse(t *testing.T) {
	testcommon.RunTestGroup(t, model, "reasoning_tool_use")
}

func TestAnthropicRefusal(t *testing.T) {
	testcommon.RunTestGroup(t, model, "anthropic_refusal")
}
