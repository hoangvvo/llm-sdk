package openai_test

import (
	"os"
	"testing"

	"github.com/hoangvvo/llm-sdk/sdk-go/internal/testcommon"
	"github.com/hoangvvo/llm-sdk/sdk-go/openai"
	"github.com/joho/godotenv"
)

var chatModel *openai.OpenAIChatModel
var audioChatModel *openai.OpenAIChatModel

var model *openai.OpenAIModel
var reasoningModel *openai.OpenAIModel

func TestMain(m *testing.M) {
	godotenv.Load("../../.env")
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		panic("OPENAI_API_KEY must be set")
	}

	model = openai.NewOpenAIModel("gpt-5.6-sol", openai.OpenAIModelOptions{
		APIKey: apiKey,
	})
	reasoningModel = openai.NewOpenAIModel("o1", openai.OpenAIModelOptions{
		APIKey: apiKey,
	})

	chatModel = openai.NewOpenAIChatModel("gpt-5.6-terra", openai.OpenAIChatModelOptions{
		APIKey: apiKey,
	})
	audioChatModel = openai.NewOpenAIChatModel("gpt-audio-1.5", openai.OpenAIChatModelOptions{
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
	testcommon.RunTestGroup(t, model, "multimodal_tool_result")
}

func TestWebSearch(t *testing.T) {
	testcommon.RunTestGroup(t, model, "web_search")
}

func TestImageGeneration(t *testing.T) {
	testcommon.RunTestGroup(t, model, "image_generation")
}

func TestImageInput(t *testing.T) {
	testcommon.RunTestGroup(t, model, "image_input")
}

func TestReasoning(t *testing.T) {
	testcommon.RunTestGroup(t, reasoningModel, "reasoning", testcommon.WithProfile("openai_opaque_reasoning"))
}

func TestReasoningToolUse(t *testing.T) {
	testcommon.RunTestGroup(t, model, "reasoning_tool_use")
}
