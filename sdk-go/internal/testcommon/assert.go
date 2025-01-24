package testcommon

import (
	"context"
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

// PartAssertion represents assertions for different part types
type PartAssertion struct {
	TextPart     *TextPartAssertion
	ToolCallPart *ToolCallPartAssertion
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

// TestCase represents a complete test case
type TestCase struct {
	Name                 string
	Input                llmsdk.LanguageModelInput
	Method               TestMethod
	RequiredCapabilities []llmsdk.LanguageModelCapability
	Output               OutputAssertion
	CompatibleSchema     bool
}

// RunTestCase executes a single test case
func RunTestCase(t *testing.T, model llmsdk.LanguageModel, testCase TestCase) {
	t.Helper()

	// Skip if required capabilities are not met (simplified for now)
	if len(testCase.RequiredCapabilities) > 0 {
		t.Skip("Required capabilities checking not yet implemented")
		return
	}

	ctx := context.Background()

	switch testCase.Method {
	case Generate:
		result, err := model.Generate(ctx, &testCase.Input)
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}
		assertContentPart(t, result.Content, testCase.Output.Content)
	case Stream:
		stream, err := model.Stream(ctx, &testCase.Input)
		if err != nil {
			t.Fatalf("Stream failed: %v", err)
		}

		accumulator := llmsdk.NewStreamAccumulator()
		for stream.Next() {
			partial := stream.Current()
			if err := accumulator.AddPartial(*partial); err != nil {
				t.Fatalf("Failed to add partial: %v", err)
			}
		}

		if err := stream.Err(); err != nil {
			t.Fatalf("Stream error: %v", err)
		}

		result, err := accumulator.ComputeResponse()
		if err != nil {
			t.Fatalf("Failed to compute response: %v", err)
		}
		assertContentPart(t, result.Content, testCase.Output.Content)
	}
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
