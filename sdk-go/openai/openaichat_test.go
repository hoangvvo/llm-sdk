package openai_test

import (
	"testing"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/testcommon"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"
)

func TestChatGenerateText(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, "generate_text")
}

func TestChatStreamText(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, "stream_text")
}

func TestChatGenerateWithSystemPrompt(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, "generate_with_system_prompt")
}

func TestChatGenerateToolCall(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, "generate_tool_call")
}

func TestChatStreamToolCall(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, "stream_tool_call")
}

func TestChatGenerateTextWithToolResult(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, "generate_text_from_tool_result")
}

func TestChatStreamTextWithToolResult(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, "stream_text_from_tool_result")
}

func TestChatGenerateParallelToolCalls(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, "generate_parallel_tool_calls")
}

func TestChatStreamParallelToolCalls(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, "stream_parallel_tool_calls")
}

func TestChatStreamParallelToolCallsOfSameName(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, "stream_parallel_tool_calls_of_same_name")
}

func TestChatStructuredResponseFormat(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, "structured_response_format")
}

func TestChatSourcePartInput(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, "source_part_input")
}

func TestChatGenerateImage(t *testing.T) {
	t.Skip("chat completion api does not support image generation")
	testcommon.RunTestCase(t, chatModel, "generate_image")
}

func TestChatStreamImage(t *testing.T) {
	t.Skip("chat completion api does not support image generation")
	testcommon.RunTestCase(t, chatModel, "stream_image")
}

func TestChatGenerateImageInput(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, "generate_image_input")
}

func TestChatStreamImageInput(t *testing.T) {
	testcommon.RunTestCase(t, chatModel, "stream_image_input")
}

func TestChatGenerateAudio(t *testing.T) {
	testcommon.RunTestCase(t, audioChatModel, "generate_audio", testcommon.WithAdditionalInput(
		func(input *llmsdk.LanguageModelInput) {
			input.Audio = &llmsdk.AudioOptions{
				Format: ptr.To(llmsdk.AudioFormatMP3),
				Voice:  ptr.To("alloy"),
			}
		}))
}

func TestChatStreamAudio(t *testing.T) {
	testcommon.RunTestCase(t, audioChatModel, "stream_audio", testcommon.WithAdditionalInput(
		func(input *llmsdk.LanguageModelInput) {
			input.Audio = &llmsdk.AudioOptions{
				Format: ptr.To(llmsdk.AudioFormatLinear16),
				Voice:  ptr.To("alloy"),
			}
		}))
}

func TestChatGenerateReasoning(t *testing.T) {
	t.Skip("reasoning not supported in chat completion api")
	testcommon.RunTestCase(t, model, "generate_reasoning")
}

func TestChatStreamReasoning(t *testing.T) {
	t.Skip("reasoning not supported in chat completion api")
	testcommon.RunTestCase(t, model, "stream_reasoning")
}
