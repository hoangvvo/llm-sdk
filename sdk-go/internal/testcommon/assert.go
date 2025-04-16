package testcommon

import (
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

// TextPartAssertion represents an assertion for text parts
type TextPartAssertion struct {
	Text *regexp.Regexp
}

func (a TextPartAssertion) Assert(t *testing.T, content []llmsdk.Part) {
	t.Helper()

	for _, part := range content {
		if part.TextPart != nil && a.Text.MatchString(part.TextPart.Text) {
			return
		}
	}

	contentJSON, _ := json.MarshalIndent(content, "", "  ")
	t.Errorf("Expected matching text part:\nExpected: %s\nReceived:\n%s", a.Text.String(), string(contentJSON))
}

// ToolCallPartAssertionArgProp represents tool call argument assertions
type ToolCallPartAssertionArgProp map[string]*regexp.Regexp

// ToolCallPartAssertion represents an assertion for tool call parts
type ToolCallPartAssertion struct {
	ToolName string
	Args     ToolCallPartAssertionArgProp
}

// matchToolCallArgs matches tool call arguments against assertions
func matchToolCallArgs(raw json.RawMessage, expected ToolCallPartAssertionArgProp) bool {
	var actual map[string]any
	if err := json.Unmarshal(raw, &actual); err != nil {
		return false
	}

	for key, expectedRegex := range expected {
		actualValue, exists := actual[key]
		if !exists {
			return false
		}

		actualStr := fmt.Sprintf("%v", actualValue)
		if !expectedRegex.MatchString(actualStr) {
			return false
		}
	}
	return true
}

func (a ToolCallPartAssertion) Assert(t *testing.T, content []llmsdk.Part) {
	t.Helper()

	for _, part := range content {
		if part.ToolCallPart != nil &&
			part.ToolCallPart.ToolName == a.ToolName &&
			matchToolCallArgs(part.ToolCallPart.Args, a.Args) {
			return
		}
	}

	contentJSON, _ := json.MarshalIndent(content, "", "  ")
	t.Errorf("Expected matching tool call part:\nExpected tool %s with args %+v\nReceived:\n%s", a.ToolName, a.Args, string(contentJSON))
}

type AudioPartAssertion struct {
	AudioID    bool // whether to check for audio ID presence
	Transcript *regexp.Regexp
}

func (a AudioPartAssertion) Assert(t *testing.T, content []llmsdk.Part) {
	t.Helper()

	for _, part := range content {
		if part.AudioPart != nil {
			if len(part.AudioPart.AudioData) == 0 {
				t.Errorf("Audio data must be present")
				return
			}
			if a.AudioID && (part.AudioPart.ID == nil || *part.AudioPart.ID == "") {
				t.Errorf("Expected audio ID to be present")
				return
			}
			if a.Transcript != nil {
				if part.AudioPart.Transcript == nil || !a.Transcript.MatchString(*part.AudioPart.Transcript) {
					t.Errorf("Expected matching transcript:\nExpected: %s\nReceived: %v", a.Transcript.String(),
						part.AudioPart.Transcript)
					return
				}
			}
			// pass
			return
		}
	}

	contentJSON, _ := json.MarshalIndent(content, "", "  ")
	t.Errorf("Expected matching audio part:\nReceived:\n%s", string(contentJSON))
}

type ReasoningPartAssertion struct {
	Text *regexp.Regexp
}

func (a ReasoningPartAssertion) Assert(t *testing.T, content []llmsdk.Part) {
	t.Helper()

	for _, part := range content {
		if part.ReasoningPart != nil {
			if !a.Text.MatchString(part.ReasoningPart.Text) {
				t.Errorf("Expected matching reasoning text:\nExpected: %s\nReceived: %s", a.Text.String(), part.ReasoningPart.Text)
				return
			}

			// pass
			return
		}
	}

	contentJSON, _ := json.MarshalIndent(content, "", "  ")
	t.Errorf("Expected matching reasoning part:\nExpected: %s\nReceived:\n%s", a.Text.String(), string(contentJSON))
}

// PartAssertion represents assertions for different part types
type PartAssertion struct {
	TextPart      *TextPartAssertion
	ToolCallPart  *ToolCallPartAssertion
	ReasoningPart *ReasoningPartAssertion
	AudioPart     *AudioPartAssertion
}

// TestMethod represents the test method type
type TestMethod int

const (
	Generate TestMethod = iota
	Stream
)

// OutputAssertion represents the expected output
type OutputAssertion struct {
	Content []PartAssertion
}

// RunTests executes multiple test cases
func RunTests(t *testing.T, testCases []TestCase, model llmsdk.LanguageModel) {
	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("%s / %s", model.Provider(), testCase.Name), func(t *testing.T) {
			RunTestCase(t, model, testCase.Name)
		})
	}
}

// assertContentPart asserts that content matches assertions
func assertContentPart(t *testing.T, content []llmsdk.Part, assertions []PartAssertion) {
	t.Helper()

	for _, assertion := range assertions {
		if assertion.TextPart != nil {
			assertion.TextPart.Assert(t, content)
		}
		if assertion.ToolCallPart != nil {
			assertion.ToolCallPart.Assert(t, content)
		}
		if assertion.AudioPart != nil {
			assertion.AudioPart.Assert(t, content)
		}
		if assertion.ReasoningPart != nil {
			assertion.ReasoningPart.Assert(t, content)
		}
	}
}

// NewTextAssertion creates a new text part assertion
func NewTextAssertion(pattern string) PartAssertion {
	return PartAssertion{
		TextPart: &TextPartAssertion{
			Text: regexp.MustCompile(pattern),
		},
	}
}

// NewToolCallAssertion creates a new tool call part assertion
func NewToolCallAssertion(toolName string, args map[string]string) PartAssertion {
	argAssertions := make(ToolCallPartAssertionArgProp)
	for key, pattern := range args {
		argAssertions[key] = regexp.MustCompile(pattern)
	}

	return PartAssertion{
		ToolCallPart: &ToolCallPartAssertion{
			ToolName: toolName,
			Args:     argAssertions,
		},
	}
}

func NewReasoningAssertion(textPattern string) PartAssertion {
	return PartAssertion{
		ReasoningPart: &ReasoningPartAssertion{
			Text: regexp.MustCompile(textPattern),
		},
	}
}

func NewAudioAssertion(audioID bool, transcriptPattern string) PartAssertion {
	var transcriptRegex *regexp.Regexp
	if transcriptPattern != "" {
		transcriptRegex = regexp.MustCompile(transcriptPattern)
	}
	return PartAssertion{
		AudioPart: &AudioPartAssertion{
			AudioID:    audioID,
			Transcript: transcriptRegex,
		},
	}
}
