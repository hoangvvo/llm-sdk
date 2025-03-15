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

	model = openai.NewOpenAIModel(openai.OpenAIModelOptions{
		APIKey:  apiKey,
		ModelID: "gpt-4o",
	})
	reasoningModel = openai.NewOpenAIModel(openai.OpenAIModelOptions{
		APIKey:  apiKey,
		ModelID: "o1",
	})

	chatModel = openai.NewOpenAIChatModel(openai.OpenAIModelOptions{
		APIKey:  apiKey,
		ModelID: "gpt-4o",
	})
	audioChatModel = openai.NewOpenAIChatModel(openai.OpenAIModelOptions{
		APIKey:  apiKey,
		ModelID: "gpt-4o-audio-preview",
	})

	m.Run()
}

func TestGenerateText(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseGenerateText)
}

func TestStreamText(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseStreamText)
}

func TestGenerateWithSystemPrompt(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseGenerateWithSystemPrompt)
}

func TestGenerateToolCall(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseGenerateToolCall)
}

func TestStreamToolCall(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseStreamToolCall)
}

func TestGenerateTextWithToolResult(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseGenerateTextWithToolResult)
}

func TestStreamTextWithToolResult(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseStreamTextWithToolResult)
}

func TestGenerateParallelToolCalls(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseGenerateParallelToolCalls)
}

func TestStreamParallelToolCalls(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseStreamParallelToolCalls)
}

func TestStreamParallelToolCallsOfSameName(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseStreamParallelToolCallsOfSameName)
}

func TestStructuredResponseFormat(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseStructuredResponseFormat)
}

func TestSourcePartInput(t *testing.T) {
	testcommon.RunTestCase(t, model, testcommon.TestCaseSourcePartInput)
}

func TestGenerateReasoning(t *testing.T) {
	testcommon.RunTestCase(t, reasoningModel, testcommon.TestCaseGenerateReasoning)
}

func TestStreamReasoning(t *testing.T) {
	testcommon.RunTestCase(t, reasoningModel, testcommon.TestCaseStreamReasoning)
}

func TestInputReasoning(t *testing.T) {
	testcommon.RunTestCase(t, reasoningModel, testcommon.TestCaseInputReasoning)
}
