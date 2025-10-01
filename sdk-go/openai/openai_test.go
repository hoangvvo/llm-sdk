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

func TestMain(m *testing.M) {
	godotenv.Load("../../.env")
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		panic("OPENAI_API_KEY must be set")
	}

	model = openai.NewOpenAIModel("gpt-5", openai.OpenAIModelOptions{
		APIKey: apiKey,
	})

	chatModel = openai.NewOpenAIChatModel("gpt-4o", openai.OpenAIChatModelOptions{
		APIKey: apiKey,
	})
	audioChatModel = openai.NewOpenAIChatModel("gpt-4o-audio-preview", openai.OpenAIChatModelOptions{
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
	testcommon.RunTestCase(t, model, "generate_image")
}

func TestStreamImage(t *testing.T) {
	testcommon.RunTestCase(t, model, "stream_image")
}

func TestGenerateImageInput(t *testing.T) {
	testcommon.RunTestCase(t, model, "generate_image_input")
}

func TestStreamImageInput(t *testing.T) {
	testcommon.RunTestCase(t, model, "stream_image_input")
}

func TestGenerateAudio(t *testing.T) {
	t.Skip("audio not supported in responses api")
	testcommon.RunTestCase(t, model, "generate_audio")
}

func TestStreamAudio(t *testing.T) {
	t.Skip("audio not supported in responses api")
	testcommon.RunTestCase(t, model, "stream_audio")
}

func TestGenerateReasoning(t *testing.T) {
	testcommon.RunTestCase(t, model, "generate_reasoning")
}

func TestStreamReasoning(t *testing.T) {
	testcommon.RunTestCase(t, model, "stream_reasoning")
}
