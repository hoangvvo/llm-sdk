package anthropic_test

import (
	"os"
	"testing"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/anthropic"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/testcommon"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"
	"github.com/joho/godotenv"
)

var model *anthropic.AnthropicModel

func TestMain(m *testing.M) {
	godotenv.Load("../../.env")
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		panic("ANTHROPIC_API_KEY must be set")
	}

	model = anthropic.NewAnthropicModel("claude-sonnet-4-20250514", anthropic.AnthropicModelOptions{
		APIKey: apiKey,
	})

	m.Run()
}

var reasoningOptions = testcommon.WithAdditionalInput(func(input *llmsdk.LanguageModelInput) {
	input.Reasoning = &llmsdk.ReasoningOptions{
		Enabled:      true,
		BudgetTokens: ptr.To[uint32](3000),
	}
})

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
	t.Skip("model does not support image generation")
	testcommon.RunTestCase(t, model, "generate_image")
}

func TestStreamImage(t *testing.T) {
	t.Skip("model does not support image generation")
	testcommon.RunTestCase(t, model, "stream_image")
}

func TestGenerateAudio(t *testing.T) {
	t.Skip("model does not support audio")
	testcommon.RunTestCase(t, model, "generate_audio")
}

func TestStreamAudio(t *testing.T) {
	t.Skip("model does not support audio")
	testcommon.RunTestCase(t, model, "stream_audio")
}

func TestGenerateReasoning(t *testing.T) {
	testcommon.RunTestCase(t, model, "generate_reasoning", reasoningOptions)
}

func TestStreamReasoning(t *testing.T) {
	testcommon.RunTestCase(t, model, "stream_reasoning", reasoningOptions)
}
