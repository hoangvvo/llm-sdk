package testcommon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

// TextPartAssertion represents an assertion for text parts
type TextPartAssertion struct {
	Text     *regexp.Regexp
	Citation *CitationAssertion
}

type CitationAssertion struct {
	Source    *regexp.Regexp
	Title     *regexp.Regexp
	CitedText *regexp.Regexp
}

func (a CitationAssertion) matches(citation llmsdk.Citation) bool {
	if a.Source != nil && !a.Source.MatchString(citation.Source) {
		return false
	}
	if a.Title != nil && (citation.Title == nil || !a.Title.MatchString(*citation.Title)) {
		return false
	}
	if a.CitedText != nil && (citation.CitedText == nil || !a.CitedText.MatchString(*citation.CitedText)) {
		return false
	}
	return true
}

func (a TextPartAssertion) matchesCitation(citations []llmsdk.Citation) bool {
	if a.Citation == nil {
		return true
	}
	for _, citation := range citations {
		if a.Citation.matches(citation) {
			return true
		}
	}
	return false
}

func (a TextPartAssertion) Assert(t *testing.T, content []llmsdk.Part) {
	t.Helper()

	for _, part := range content {
		if part.TextPart != nil && a.Text.MatchString(part.TextPart.Text) && a.matchesCitation(part.TextPart.Citations) {
			return
		}
	}

	contentJSON, _ := json.MarshalIndent(content, "", "  ")
	t.Errorf("Expected matching text part:\nExpected: %s\nReceived:\n%s", a.Text.String(), string(contentJSON))
}

// ToolCallPartAssertion represents an assertion for tool call parts
type ToolCallPartAssertion struct {
	ToolName string
	Args     *regexp.Regexp
}

// matchToolCallArgs matches tool call arguments against assertions
func matchToolCallArgs(raw json.RawMessage, expected *regexp.Regexp) bool {
	if expected == nil {
		return true
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return false
	}

	return expected.Match(compact.Bytes())
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
			if len(part.AudioPart.Data) == 0 {
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

type ImagePartAssertion struct {
	ImageID bool // whether to check for image ID presence
}

func (a ImagePartAssertion) Assert(t *testing.T, content []llmsdk.Part) {
	t.Helper()

	for _, part := range content {
		if part.ImagePart != nil {
			if len(part.ImagePart.Data) == 0 {
				t.Errorf("Image data must be present")
				return
			}
			if a.ImageID && (part.ImagePart.ID == nil || *part.ImagePart.ID == "") {
				t.Errorf("Expected image ID to be present")
				return
			}

			// pass
			return
		}
	}

	contentJSON, _ := json.MarshalIndent(content, "", "  ")
	t.Errorf("Expected matching image part:\nReceived:\n%s", string(contentJSON))
}

type ReasoningPartAssertion struct {
	Text      *regexp.Regexp
	Signature bool
}

func (a ReasoningPartAssertion) matches(part *llmsdk.ReasoningPart) bool {
	textMatches := a.Text.MatchString(part.Text)
	signatureMatches := !a.Signature || (part.Signature != nil && *part.Signature != "")
	return textMatches && signatureMatches
}

func (a ReasoningPartAssertion) Assert(t *testing.T, content []llmsdk.Part) {
	t.Helper()

	for _, part := range content {
		if part.ReasoningPart != nil && a.matches(part.ReasoningPart) {
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
	ImagePart     *ImagePartAssertion
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
		if assertion.ImagePart != nil {
			assertion.ImagePart.Assert(t, content)
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
			Text: regexp.MustCompile("(?s)" + pattern),
		},
	}
}

func NewTextAssertionWithCitation(pattern string, citation CitationAssertion) PartAssertion {
	assertion := NewTextAssertion(pattern)
	assertion.TextPart.Citation = &citation
	return assertion
}

// NewToolCallAssertion creates a new tool call part assertion
func NewToolCallAssertion(toolName string, argsPattern string) PartAssertion {
	var args *regexp.Regexp
	if argsPattern != "" {
		args = regexp.MustCompile("(?s)" + argsPattern)
	}
	return PartAssertion{
		ToolCallPart: &ToolCallPartAssertion{
			ToolName: toolName,
			Args:     args,
		},
	}
}

func NewReasoningAssertion(textPattern string, signature ...bool) PartAssertion {
	hasSignature := len(signature) > 0 && signature[0]
	return PartAssertion{
		ReasoningPart: &ReasoningPartAssertion{
			Text:      regexp.MustCompile("(?s)" + textPattern),
			Signature: hasSignature,
		},
	}
}

func NewAudioAssertion(audioID bool, transcriptPattern string) PartAssertion {
	var transcriptRegex *regexp.Regexp
	if transcriptPattern != "" {
		transcriptRegex = regexp.MustCompile("(?s)" + transcriptPattern)
	}
	return PartAssertion{
		AudioPart: &AudioPartAssertion{
			AudioID:    audioID,
			Transcript: transcriptRegex,
		},
	}
}

func NewImageAssertion(imageID bool) PartAssertion {
	return PartAssertion{
		ImagePart: &ImagePartAssertion{
			ImageID: imageID,
		},
	}
}
