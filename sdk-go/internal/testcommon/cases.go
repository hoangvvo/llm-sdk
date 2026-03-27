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
	"github.com/tidwall/gjson"
)

// TestCase represents a complete test case.
type TestCase struct {
	Name                string
	Stages              []TestStage
	AdditionalInput     func(*llmsdk.LanguageModelInput)
	CustomOutputContent func([]PartAssertion) []PartAssertion
}

type TestStage struct {
	InputTemplate json.RawMessage
	Method        TestMethod
	Output        OutputAssertion
}

// TestDataJSON represents the structure of the JSON test data.
type TestDataJSON struct {
	Tools     []llmsdk.Tool  `json:"tools"`
	TestCases []TestCaseJSON `json:"test_cases"`
}

// TestCaseJSON represents a test case in JSON format.
type TestCaseJSON struct {
	Name   string          `json:"name"`
	Stages []TestStageJSON `json:"stages"`
}

type TestStageJSON struct {
	Type       string                 `json:"type"`
	Input      json.RawMessage        `json:"input"`
	InputTools []string               `json:"input_tools,omitempty"`
	Expect     map[string]interface{} `json:"expect"`
}

var (
	testData  *TestDataJSON
	toolsMap  map[string]llmsdk.Tool
	testCases map[string]TestCase
	initOnce  sync.Once
)

func ensureInitialized() {
	initOnce.Do(func() {
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

		toolsMap = make(map[string]llmsdk.Tool)
		for _, tool := range testData.Tools {
			toolsMap[tool.Name] = tool
		}

		testCases = make(map[string]TestCase)
		for _, tc := range testData.TestCases {
			testCase := convertJSONToTestCase(tc)
			testCases[tc.Name] = testCase
		}
	})
}

func convertJSONToTestCase(tc TestCaseJSON) TestCase {
	stages := make([]TestStage, 0, len(tc.Stages))
	for _, stage := range tc.Stages {
		stages = append(stages, TestStage{
			InputTemplate: buildStageInputTemplate(stage.Input, stage.InputTools),
			Method:        parseTestMethod(stage.Type),
			Output:        convertOutput(stage.Expect),
		})
	}

	return TestCase{
		Name:   tc.Name,
		Stages: stages,
	}
}

func buildStageInputTemplate(input json.RawMessage, inputTools []string) json.RawMessage {
	if len(inputTools) == 0 {
		return append(json.RawMessage(nil), input...)
	}

	var inputValue map[string]interface{}
	if err := json.Unmarshal(input, &inputValue); err != nil {
		panic(fmt.Sprintf("Failed to parse stage input: %v", err))
	}

	inputValue["tools"] = resolveTools(inputTools)
	data, err := json.Marshal(inputValue)
	if err != nil {
		panic(fmt.Sprintf("Failed to encode stage input: %v", err))
	}
	return data
}

func parseTestMethod(value string) TestMethod {
	if value == "stream" {
		return Stream
	}
	return Generate
}

func convertOutput(output map[string]interface{}) OutputAssertion {
	converted := OutputAssertion{}
	if content, ok := output["content"].([]interface{}); ok {
		converted.Content = convertOutputAssertions(content)
	}
	return converted
}

func convertOutputAssertions(content []interface{}) []PartAssertion {
	assertions := []PartAssertion{}

	for _, part := range content {
		partMap := part.(map[string]interface{})
		partType := partMap["type"].(string)

		switch partType {
		case "text":
			text := partMap["text"].(string)
			assertions = append(assertions, NewTextAssertion(text))
		case "tool_call":
			args := ""
			if value, ok := partMap["args"].(string); ok {
				args = value
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
			audioID := false
			if value, ok := partMap["id"].(bool); ok {
				audioID = value
			}
			assertions = append(assertions, NewAudioAssertion(audioID, transcript))
		case "image":
			imageID := false
			if value, ok := partMap["id"].(bool); ok {
				imageID = value
			}
			assertions = append(assertions, NewImageAssertion(imageID))
		case "reasoning":
			assertions = append(assertions, NewReasoningAssertion(partMap["text"].(string)))
		}
	}

	return assertions
}

func resolveTools(toolNames []string) []llmsdk.Tool {
	resolved := make([]llmsdk.Tool, 0, len(toolNames))
	for _, toolName := range toolNames {
		tool, exists := toolsMap[toolName]
		if !exists {
			panic(fmt.Sprintf("Tool %q not found in test data", toolName))
		}
		resolved = append(resolved, tool)
	}
	return resolved
}

func resolveStageInput(
	inputTemplate json.RawMessage,
	context map[string]interface{},
) (llmsdk.LanguageModelInput, error) {
	var inputValue interface{}
	if err := json.Unmarshal(inputTemplate, &inputValue); err != nil {
		return llmsdk.LanguageModelInput{}, err
	}

	resolved, err := resolveStageRefs(inputValue, context)
	if err != nil {
		return llmsdk.LanguageModelInput{}, err
	}

	data, err := json.Marshal(resolved)
	if err != nil {
		return llmsdk.LanguageModelInput{}, err
	}

	var input llmsdk.LanguageModelInput
	if err := json.Unmarshal(data, &input); err != nil {
		return llmsdk.LanguageModelInput{}, err
	}
	return input, nil
}

func resolveStageRefs(value interface{}, root interface{}) (interface{}, error) {
	switch typed := value.(type) {
	case []interface{}:
		resolved := make([]interface{}, len(typed))
		for i, child := range typed {
			next, err := resolveStageRefs(child, root)
			if err != nil {
				return nil, err
			}
			resolved[i] = next
		}
		return resolved, nil
	case map[string]interface{}:
		if refPath, ok := getStageRefPath(typed); ok {
			return resolveRefPath(refPath, root)
		}

		resolved := make(map[string]interface{}, len(typed))
		for key, child := range typed {
			next, err := resolveStageRefs(child, root)
			if err != nil {
				return nil, err
			}
			resolved[key] = next
		}
		return resolved, nil
	default:
		return value, nil
	}
}

func getStageRefPath(value map[string]interface{}) (string, bool) {
	if len(value) != 1 {
		return "", false
	}
	ref, ok := value["$ref"].(string)
	return ref, ok
}

func resolveRefPath(path string, root interface{}) (interface{}, error) {
	data, err := json.Marshal(root)
	if err != nil {
		return nil, err
	}

	result := gjson.GetBytes(data, path)
	if !result.Exists() {
		return nil, fmt.Errorf("invalid stage ref path %q", path)
	}

	if result.Raw != "" {
		var resolved interface{}
		if err := json.Unmarshal([]byte(result.Raw), &resolved); err != nil {
			return nil, err
		}
		return resolved, nil
	}

	return cloneJSONValue(result.Value())
}

func cloneJSONValue(value interface{}) (interface{}, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var cloned interface{}
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, err
	}
	return cloned, nil
}

func clonePartAssertions(content []PartAssertion) []PartAssertion {
	cloned := make([]PartAssertion, 0, len(content))
	for _, assertion := range content {
		next := PartAssertion{}
		if assertion.TextPart != nil {
			text := *assertion.TextPart
			next.TextPart = &text
		}
		if assertion.ToolCallPart != nil {
			toolCall := *assertion.ToolCallPart
			next.ToolCallPart = &toolCall
		}
		if assertion.AudioPart != nil {
			audio := *assertion.AudioPart
			next.AudioPart = &audio
		}
		if assertion.ImagePart != nil {
			image := *assertion.ImagePart
			next.ImagePart = &image
		}
		if assertion.ReasoningPart != nil {
			reasoning := *assertion.ReasoningPart
			next.ReasoningPart = &reasoning
		}
		cloned = append(cloned, next)
	}
	return cloned
}

func appendMessages(history []llmsdk.Message, next ...llmsdk.Message) []llmsdk.Message {
	combined := make([]llmsdk.Message, 0, len(history)+len(next))
	combined = append(combined, history...)
	combined = append(combined, next...)
	return combined
}

func assistantContentToValue(content []llmsdk.Part) (interface{}, error) {
	data, err := json.Marshal(content)
	if err != nil {
		return nil, err
	}

	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func toolCallPartsToValue(content []llmsdk.Part) (interface{}, error) {
	toolCalls := make([]llmsdk.Part, 0)
	for _, part := range content {
		if part.ToolCallPart != nil {
			toolCalls = append(toolCalls, part)
		}
	}
	return assistantContentToValue(toolCalls)
}

// RunTestCase executes a single test case by name.
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
	context := map[string]interface{}{
		"stages": []interface{}{},
	}
	history := []llmsdk.Message{}

	for _, stage := range testCase.Stages {
		input, err := resolveStageInput(stage.InputTemplate, context)
		if err != nil {
			t.Fatalf("Failed to resolve stage input: %v", err)
		}

		stageMessages := append([]llmsdk.Message(nil), input.Messages...)
		stagedInput := input
		stagedInput.Messages = appendMessages(history, stageMessages...)

		requestInput := stagedInput
		if testCase.AdditionalInput != nil {
			testCase.AdditionalInput(&requestInput)
		}

		outputContent := clonePartAssertions(stage.Output.Content)
		if testCase.CustomOutputContent != nil {
			outputContent = testCase.CustomOutputContent(outputContent)
		}

		var assistantMessage llmsdk.Message
		var assistantContent []llmsdk.Part
		switch stage.Method {
		case Generate:
			result, err := model.Generate(ctx, &requestInput)
			if err != nil {
				t.Fatalf("Generate failed: %v", err)
			}
			assertContentPart(t, result.Content, outputContent)
			assistantContent = result.Content
			assistantMessage = llmsdk.NewAssistantMessage(result.Content...)
		case Stream:
			stream, err := model.Stream(ctx, &requestInput)
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
			assertContentPart(t, result.Content, outputContent)
			assistantContent = result.Content
			assistantMessage = llmsdk.NewAssistantMessage(result.Content...)
		}

		history = appendMessages(stagedInput.Messages, assistantMessage)

		assistantParts, err := assistantContentToValue(assistantContent)
		if err != nil {
			t.Fatalf("Failed to encode stage assistant output: %v", err)
		}
		toolCallParts, err := toolCallPartsToValue(assistantContent)
		if err != nil {
			t.Fatalf("Failed to encode stage tool calls: %v", err)
		}
		context["stages"] = append(context["stages"].([]interface{}), map[string]interface{}{
			"assistant":  assistantParts,
			"tool_calls": toolCallParts,
		})
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
		tc.CustomOutputContent = f
	}
}

// GetWeatherTool returns the standard weather tool for testing.
func GetWeatherTool() llmsdk.Tool {
	ensureInitialized()
	return toolsMap["get_weather"]
}

// GetStockPriceTool returns the standard stock price tool for testing.
func GetStockPriceTool() llmsdk.Tool {
	ensureInitialized()
	return toolsMap["get_stock_price"]
}
