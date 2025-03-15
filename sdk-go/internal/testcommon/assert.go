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

// ToolCallPartAssertionArgProp represents tool call argument assertions
type ToolCallPartAssertionArgProp map[string]*regexp.Regexp

// ToolCallPartAssertion represents an assertion for tool call parts
type ToolCallPartAssertion struct {
	ToolName string
	Args     ToolCallPartAssertionArgProp
}

type ReasoningPartAssertion struct {
	Text    *regexp.Regexp
	Summary *regexp.Regexp
}

// PartAssertion represents assertions for different part types
type PartAssertion struct {
	TextPart      *TextPartAssertion
	ToolCallPart  *ToolCallPartAssertion
	ReasoningPart *ReasoningPartAssertion
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
			RunTestCase(t, model, testCase)
		})
	}
}

// assertContentPart asserts that content matches assertions
func assertContentPart(t *testing.T, content []llmsdk.Part, assertions []PartAssertion) {
	t.Helper()

	for _, assertion := range assertions {
		if assertion.TextPart != nil {
			assertTextPart(t, content, *assertion.TextPart)
		}
		if assertion.ToolCallPart != nil {
			assertToolCallPart(t, content, *assertion.ToolCallPart)
		}
		if assertion.ReasoningPart != nil {
			assertReasoningPart(t, content, *assertion.ReasoningPart)
		}
	}
}

// assertTextPart asserts text part content
func assertTextPart(t *testing.T, content []llmsdk.Part, assertion TextPartAssertion) {
	t.Helper()

	for _, part := range content {
		if part.TextPart != nil && assertion.Text.MatchString(part.TextPart.Text) {
			return
		}
	}

	contentJSON, _ := json.MarshalIndent(content, "", "  ")
	t.Errorf("Expected matching text part:\nExpected: %s\nReceived:\n%s", assertion.Text.String(), string(contentJSON))
}

// assertToolCallPart asserts tool call part content
func assertToolCallPart(t *testing.T, content []llmsdk.Part, assertion ToolCallPartAssertion) {
	t.Helper()

	for _, part := range content {
		if part.ToolCallPart != nil &&
			part.ToolCallPart.ToolName == assertion.ToolName &&
			matchToolCallArgs(part.ToolCallPart.Args, assertion.Args) {
			return
		}
	}

	contentJSON, _ := json.MarshalIndent(content, "", "  ")
	t.Errorf("Expected matching tool call part:\nExpected tool %s with args %+v\nReceived:\n%s", assertion.ToolName, assertion.Args, string(contentJSON))
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

func assertReasoningPart(t *testing.T, content []llmsdk.Part, assertion ReasoningPartAssertion) {
	t.Helper()

	for _, part := range content {
		if part.ReasoningPart != nil {
			if assertion.Text != nil && !assertion.Text.MatchString(part.ReasoningPart.Text) {
				t.Errorf("Expected matching reasoning text:\nExpected: %s\nReceived: %s", assertion.Text.String(), part.ReasoningPart.Text)
				return
			}
			if assertion.Summary != nil && (part.ReasoningPart.Summary == nil || !assertion.Summary.MatchString(*part.ReasoningPart.Summary)) {
				t.Errorf("Expected matching reasoning summary:\nExpected: %s\nReceived: %v", assertion.Summary.String(), part.ReasoningPart.Summary)
				return
			}

			// pass
			return
		}
	}

	contentJSON, _ := json.MarshalIndent(content, "", "  ")
	t.Errorf("Expected matching reasoning part:\nExpected: %s\nReceived:\n%s", assertion.Text.String(), string(contentJSON))
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

func NewReasoningAssertion(textPattern, summaryPattern string) PartAssertion {
	var text, summary *regexp.Regexp
	if textPattern != "" {
		text = regexp.MustCompile(textPattern)
	}
	if summaryPattern != "" {
		summary = regexp.MustCompile(summaryPattern)
	}
	return PartAssertion{
		ReasoningPart: &ReasoningPartAssertion{
			Text:    text,
			Summary: summary,
		},
	}
}
