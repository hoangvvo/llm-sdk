package testcommon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

// TestCase represents a complete test case
type TestCase struct {
	Name            string
	Input           llmsdk.LanguageModelInput
	Method          TestMethod
	Output          OutputAssertion
	AdditionalInput func(*llmsdk.LanguageModelInput)
}

// TestDataJSON represents the structure of the JSON test data
type TestDataJSON struct {
	Tools     []llmsdk.Tool  `json:"tools"`
	TestCases []TestCaseJSON `json:"test_cases"`
}

// TestCaseJSON represents a test case in JSON format
type TestCaseJSON struct {
	Name       string                    `json:"name"`
	Type       string                    `json:"type"`
	Input      llmsdk.LanguageModelInput `json:"input"`
	InputTools []string                  `json:"input_tools,omitempty"`
	Output     map[string]interface{}    `json:"output"`
}

var (
	testData  *TestDataJSON
	toolsMap  map[string]llmsdk.Tool
	testCases map[string]TestCase
	initOnce  sync.Once
)

func ensureInitialized() {
	initOnce.Do(func() {
		// Load test data from JSON
		_, filename, _, _ := runtime.Caller(0)
		dir := filepath.Dir(filename)
		jsonPath := filepath.Join(dir, "..", "..", "..", "sdk-tests", "tests.json")

		data, err := os.ReadFile(jsonPath)
		if err != nil {
			panic(fmt.Sprintf("Failed to read test data: %v", err))
		}

		testData = &TestDataJSON{}
		if err := json.Unmarshal(data, testData); err != nil {
			panic(fmt.Sprintf("Failed to parse test data: %v", err))
		}

		// Build tools map
		toolsMap = make(map[string]llmsdk.Tool)
		for _, tool := range testData.Tools {
			toolsMap[tool.Name] = tool
		}

		// Build test cases
		testCases = make(map[string]TestCase)
		for _, tc := range testData.TestCases {
			testCase := convertJSONToTestCase(tc)
			testCases[tc.Name] = testCase
		}
	})
}

func convertJSONToTestCase(tc TestCaseJSON) TestCase {
	// Input is already a LanguageModelInput from JSON unmarshaling
	input := tc.Input

	// Handle tools
	if len(tc.InputTools) > 0 {
		input.Tools = []llmsdk.Tool{}
		for _, toolName := range tc.InputTools {
			if tool, exists := toolsMap[toolName]; exists {
				input.Tools = append(input.Tools, tool)
			}
		}
	}

	// Convert output
	output := OutputAssertion{}
	if content, ok := tc.Output["content"].([]interface{}); ok {
		output.Content = convertOutputAssertions(content)
	}

	// Determine method
	method := Generate
	if tc.Type == "stream" {
		method = Stream
	}

	return TestCase{
		Name:   tc.Name,
		Input:  input,
		Method: method,
		Output: output,
	}
}

func convertOutputAssertions(content []interface{}) []PartAssertion {
	assertions := []PartAssertion{}

	for _, part := range content {
		partMap := part.(map[string]interface{})
		partType := partMap["type"].(string)

		switch partType {
		case "text":
			text := partMap["text"].(string)
			// Always treat as regex
			assertions = append(assertions, NewTextAssertion(text))
		case "tool_call":
			args := make(map[string]string)
			if argsMap, ok := partMap["args"].(map[string]interface{}); ok {
				for k, v := range argsMap {
					args[k] = fmt.Sprintf("%v", v)
				}
			}
			assertions = append(assertions, NewToolCallAssertion(
				partMap["tool_name"].(string),
				args,
			))
		case "audio":
			var transcript string
			if t, ok := partMap["transcript"].(string); ok {
				transcript = t
			}
			assertions = append(assertions, NewAudioAssertion(
				partMap["id"].(bool),
				transcript,
			))
		case "image":
			assertions = append(assertions, NewImageAssertion(partMap["id"].(bool)))
		case "reasoning":
			assertions = append(assertions, NewReasoningAssertion(partMap["text"].(string)))
		}
	}

	return assertions
}

// RunTestCase executes a single test case by name
func RunTestCase(t *testing.T, model llmsdk.LanguageModel, testCaseName string, opts ...TestCaseOption) {
	t.Helper()

	ensureInitialized()
	testCase, exists := testCases[testCaseName]
	if !exists {
		t.Fatalf("Test case %q not found", testCaseName)
	}

	for _, opt := range opts {
		opt(&testCase)
	}

	ctx := t.Context()

	input := &testCase.Input
	if testCase.AdditionalInput != nil {
		testCase.AdditionalInput(input)
	}

	switch testCase.Method {
	case Generate:
		result, err := model.Generate(ctx, input)
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}
		assertContentPart(t, result.Content, testCase.Output.Content)
	case Stream:
		stream, err := model.Stream(ctx, input)
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

type TestCaseOption func(*TestCase)

func WithAdditionalInput(f func(*llmsdk.LanguageModelInput)) TestCaseOption {
	return func(tc *TestCase) {
		tc.AdditionalInput = f
	}
}

func WithCustomOutputContent(f func([]PartAssertion) []PartAssertion) TestCaseOption {
	return func(tc *TestCase) {
		tc.Output.Content = f(tc.Output.Content)
	}
}

// GetWeatherTool returns the standard weather tool for testing
func GetWeatherTool() llmsdk.Tool {
	ensureInitialized()
	return toolsMap["get_weather"]
}

// GetStockPriceTool returns the standard stock price tool for testing
func GetStockPriceTool() llmsdk.Tool {
	ensureInitialized()
	return toolsMap["get_stock_price"]
}
