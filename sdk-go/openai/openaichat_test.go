package openai_test

import (
	"testing"

	"github.com/hoangvvo/llm-sdk/sdk-go/internal/testcommon"
)

var noReasoning = testcommon.WithProfile("reasoning_disabled")

func TestChatTextGeneration(t *testing.T) {
	testcommon.RunTestGroup(t, openAIChatModel(t, "gpt-5.6-terra"), "text_generation", noReasoning)
}

func TestChatConversation(t *testing.T) {
	testcommon.RunTestGroup(t, openAIChatModel(t, "gpt-5.6-terra"), "conversation", noReasoning)
}

func TestChatToolUse(t *testing.T) {
	testcommon.RunTestGroup(t, openAIChatModel(t, "gpt-5.6-terra"), "tool_use", noReasoning)
}

func TestChatStructuredOutput(t *testing.T) {
	testcommon.RunTestGroup(t, openAIChatModel(t, "gpt-5.6-terra"), "structured_output", noReasoning)
}

func TestChatGenerationOptions(t *testing.T) {
	testcommon.RunTestGroup(t, openAIChatModel(t, "gpt-5.6-terra"), "generation_options", noReasoning)
}

func TestChatSourceInput(t *testing.T) {
	testcommon.RunTestGroup(t, openAIChatModel(t, "gpt-5.6-terra"), "source_input", noReasoning)
}

func TestChatImageInput(t *testing.T) {
	testcommon.RunTestGroup(t, openAIChatModel(t, "gpt-5.6-terra"), "image_input")
}

func TestChatGenerateAudio(t *testing.T) {
	testcommon.RunTestCase(t, openAIChatModel(t, "gpt-audio-1.5"), "generate_audio",
		testcommon.WithProfile("openai_audio_mp3"))
}

func TestChatStreamAudio(t *testing.T) {
	testcommon.RunTestCase(t, openAIChatModel(t, "gpt-audio-1.5"), "stream_audio",
		testcommon.WithProfile("openai_audio_linear16"))
}
