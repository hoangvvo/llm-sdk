package openai_test

import (
	"testing"

	"github.com/hoangvvo/llm-sdk/sdk-go/internal/testcommon"
)

var noReasoning = testcommon.WithProfile("reasoning_disabled")

func TestChatTextGeneration(t *testing.T) {
	testcommon.RunTestGroup(t, chatModel, "text_generation", noReasoning)
}

func TestChatConversation(t *testing.T) {
	testcommon.RunTestGroup(t, chatModel, "conversation", noReasoning)
}

func TestChatToolUse(t *testing.T) {
	testcommon.RunTestGroup(t, chatModel, "tool_use", noReasoning)
}

func TestChatStructuredOutput(t *testing.T) {
	testcommon.RunTestGroup(t, chatModel, "structured_output", noReasoning)
}

func TestChatGenerationOptions(t *testing.T) {
	testcommon.RunTestGroup(t, chatModel, "generation_options", noReasoning)
}

func TestChatSourceInput(t *testing.T) {
	testcommon.RunTestGroup(t, chatModel, "source_input", noReasoning)
}

func TestChatImageInput(t *testing.T) {
	testcommon.RunTestGroup(t, chatModel, "image_input")
}

func TestChatGenerateAudio(t *testing.T) {
	testcommon.RunTestCase(t, audioChatModel, "generate_audio",
		testcommon.WithProfile("openai_audio_mp3"))
}

func TestChatStreamAudio(t *testing.T) {
	testcommon.RunTestCase(t, audioChatModel, "stream_audio",
		testcommon.WithProfile("openai_audio_linear16"))
}
