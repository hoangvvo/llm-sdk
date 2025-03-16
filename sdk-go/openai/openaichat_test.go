package openai_test

import (
	"testing"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/testcommon"
)

func TestChatGenerateText(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseGenerateText)
}

func TestChatStreamText(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseStreamText)
}

func TestChatGenerateWithSystemPrompt(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseGenerateWithSystemPrompt)
}

func TestChatGenerateToolCall(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseGenerateToolCall)
}

func TestChatStreamToolCall(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseStreamToolCall)
}

func TestChatGenerateTextWithToolResult(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseGenerateTextWithToolResult)
}

func TestChatStreamTextWithToolResult(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseStreamTextWithToolResult)
}

func TestChatGenerateParallelToolCalls(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseGenerateParallelToolCalls)
}

func TestChatStreamParallelToolCalls(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseStreamParallelToolCalls)
}

func TestChatStreamParallelToolCallsOfSameName(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseStreamParallelToolCallsOfSameName)
}

func TestChatStructuredResponseFormat(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseStructuredResponseFormat)
}

func TestChatSourcePartInput(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, testcommon.TestCaseSourcePartInput)
}

func TestChatGenerateAudio(t *testing.T) {
	tc := testcommon.TestCaseGenerateAudio
	tc.AdditionalInput = func(input *llmsdk.LanguageModelInput) {
		input.Extra = map[string]any{
			"audio": map[string]any{
				"voice":  "alloy",
				"format": "mp3",
			},
		}
	}
	testcommon.RunTestCase(t, audioChatModel, tc)
}

func TestChatStreamAudio(t *testing.T) {
	tc := testcommon.TestCaseStreamAudio
	tc.AdditionalInput = func(input *llmsdk.LanguageModelInput) {
		input.Extra = map[string]any{
			"audio": map[string]any{
				"voice":  "alloy",
				"format": "mp3",
			},
		}
	}
	testcommon.RunTestCase(t, audioChatModel, tc)
}

func TestChatGenerateReasoning(t *testing.T) {
	t.Skip("reasoning not supported in chat completion api")
	testcommon.RunTestCase(t, reasoningModel, testcommon.TestCaseGenerateReasoning)
}

func TestChatStreamReasoning(t *testing.T) {
	t.Skip("reasoning not supported in chat completion api")
	testcommon.RunTestCase(t, reasoningModel, testcommon.TestCaseStreamReasoning)
}

func TestChatInputReasoning(t *testing.T) {
	t.Skip("reasoning not supported in chat completion api")
	testcommon.RunTestCase(t, reasoningModel, testcommon.TestCaseInputReasoning)
}
