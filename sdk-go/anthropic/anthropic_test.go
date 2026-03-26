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

	model = anthropic.NewAnthropicModel("claude-sonnet-4-5", anthropic.AnthropicModelOptions{
		APIKey: apiKey,
	})

	m.Run()
}

var reasoningOptions = testcommon.WithAdditionalInput(func(input *llmsdk.LanguageModelInput) {
	patchAnthropicStrictToolSchemas(input)
	input.Reasoning = &llmsdk.ReasoningOptions{
		Enabled:      true,
		BudgetTokens: ptr.To[uint32](3000),
	}
})

var anthropicCompatOptions = testcommon.WithAdditionalInput(func(input *llmsdk.LanguageModelInput) {
	patchAnthropicStrictToolSchemas(input)
})

func patchAnthropicStrictToolSchemas(input *llmsdk.LanguageModelInput) {
	for i := range input.Tools {
		input.Tools[i].Parameters = patchAnthropicToolSchema(input.Tools[i].Name, input.Tools[i].Parameters)
	}
}

func patchAnthropicToolSchema(name string, value llmsdk.JSONSchema) llmsdk.JSONSchema {
	if name != "get_weather" {
		return value
	}

	properties, ok := value["properties"].(map[string]any)
	if !ok {
		return value
	}

	preferredUnit, ok := properties["preferred_unit"].(map[string]any)
	if !ok {
		return value
	}

	// Temporary Anthropic test workaround: strict tools currently reject the
	// shared nullable-enum shape on get_weather.preferred_unit in practice.
	patchedParameters := make(llmsdk.JSONSchema, len(value))
	for key, child := range value {
		patchedParameters[key] = child
	}

	patchedProperties := make(map[string]any, len(properties))
	for key, child := range properties {
		patchedProperties[key] = child
	}

	patchedPreferredUnit := make(map[string]any, len(preferredUnit))
	for key, child := range preferredUnit {
		patchedPreferredUnit[key] = child
	}
	patchedPreferredUnit["type"] = "string"
	patchedProperties["preferred_unit"] = patchedPreferredUnit
	patchedParameters["properties"] = patchedProperties
	return patchedParameters
}

func TestGenerateText(t *testing.T) {
	testcommon.RunTestCase(t, model, "generate_text", anthropicCompatOptions)
}

func TestStreamText(t *testing.T) {
	testcommon.RunTestCase(t, model, "stream_text", anthropicCompatOptions)
}

func TestGenerateWithSystemPrompt(t *testing.T) {
	testcommon.RunTestCase(t, model, "generate_with_system_prompt", anthropicCompatOptions)
}

func TestGenerateToolCall(t *testing.T) {
	testcommon.RunTestCase(t, model, "generate_tool_call", anthropicCompatOptions)
}

func TestStreamToolCall(t *testing.T) {
	testcommon.RunTestCase(t, model, "stream_tool_call", anthropicCompatOptions)
}

func TestGenerateTextWithToolResult(t *testing.T) {
	testcommon.RunTestCase(t, model, "generate_text_from_tool_result", anthropicCompatOptions)
}

func TestStreamTextWithToolResult(t *testing.T) {
	testcommon.RunTestCase(t, model, "stream_text_from_tool_result", anthropicCompatOptions)
}

func TestGenerateTextWithImageToolResult(t *testing.T) {
	testcommon.RunTestCase(t, model, "generate_text_from_image_tool_result", anthropicCompatOptions)
}

func TestGenerateParallelToolCalls(t *testing.T) {
	testcommon.RunTestCase(t, model, "generate_parallel_tool_calls", anthropicCompatOptions)
}

func TestStreamParallelToolCalls(t *testing.T) {
	testcommon.RunTestCase(t, model, "stream_parallel_tool_calls", anthropicCompatOptions)
}

func TestStreamParallelToolCallsOfSameName(t *testing.T) {
	testcommon.RunTestCase(t, model, "stream_parallel_tool_calls_of_same_name", anthropicCompatOptions)
}

func TestStructuredResponseFormat(t *testing.T) {
	testcommon.RunTestCase(t, model, "structured_response_format", anthropicCompatOptions)
}

func TestSourcePartInput(t *testing.T) {
	testcommon.RunTestCase(t, model, "source_part_input", anthropicCompatOptions)
}

func TestGenerateImage(t *testing.T) {
	t.Skip("model does not support image generation")
	testcommon.RunTestCase(t, model, "generate_image")
}

func TestStreamImage(t *testing.T) {
	t.Skip("model does not support image generation")
	testcommon.RunTestCase(t, model, "stream_image")
}

func TestGenerateImageInput(t *testing.T) {
	testcommon.RunTestCase(t, model, "generate_image_input", anthropicCompatOptions)
}

func TestStreamImageInput(t *testing.T) {
	testcommon.RunTestCase(t, model, "stream_image_input", anthropicCompatOptions)
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
