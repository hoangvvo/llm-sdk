package testcommon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

type caseInfo struct {
	StageCount int `json:"stage_count"`
}

type preparedStage struct {
	Method string          `json:"method"`
	Input  json.RawMessage `json:"input"`
}

type stageResult struct {
	Assistant []llmsdk.Part `json:"assistant"`
	ToolCalls []llmsdk.Part `json:"tool_calls"`
}

type stageContext struct {
	Stages []stageResult `json:"stages"`
}

type protocolRequest struct {
	Command  string        `json:"command"`
	TestCase string        `json:"test_case"`
	Stage    int           `json:"stage"`
	Context  *stageContext `json:"context,omitempty"`
	Content  []llmsdk.Part `json:"content,omitempty"`
	Profile  string        `json:"profile,omitempty"`
}

func protocolPath() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to locate sdk-tests protocol")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "..", "sdk-tests", "cli.mjs")
}

func callProtocol(request protocolRequest, response any) error {
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode sdk-tests request: %w", err)
	}

	cmd := exec.Command("node", protocolPath())
	cmd.Stdin = bytes.NewReader(requestJSON)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("sdk-tests: %s", message)
	}
	if response == nil {
		return nil
	}
	if err := json.Unmarshal(output, response); err != nil {
		return fmt.Errorf("decode sdk-tests response: %w", err)
	}
	return nil
}

type testCaseConfig struct {
	Profile string
}

// TestCaseOption customizes the SDK-specific adapter around a shared test case.
type TestCaseOption func(*testCaseConfig)

// WithProfile applies a named input/expectation profile from sdk-tests/tests.json.
func WithProfile(profile string) TestCaseOption {
	return func(config *testCaseConfig) {
		config.Profile = profile
	}
}

func getToolCalls(content []llmsdk.Part) []llmsdk.Part {
	toolCalls := make([]llmsdk.Part, 0)
	for _, part := range content {
		if part.ToolCallPart != nil {
			toolCalls = append(toolCalls, part)
		}
	}
	return toolCalls
}

func appendMessages(history []llmsdk.Message, next ...llmsdk.Message) []llmsdk.Message {
	combined := make([]llmsdk.Message, 0, len(history)+len(next))
	combined = append(combined, history...)
	combined = append(combined, next...)
	return combined
}

// RunTestCase executes a shared sdk-tests case through the Go SDK adapter.
func RunTestCase(t *testing.T, model llmsdk.LanguageModel, testCaseName string, opts ...TestCaseOption) {
	t.Helper()

	config := testCaseConfig{}
	for _, opt := range opts {
		opt(&config)
	}

	var info caseInfo
	if err := callProtocol(protocolRequest{
		Command:  "case_info",
		TestCase: testCaseName,
	}, &info); err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	context := stageContext{Stages: []stageResult{}}
	history := []llmsdk.Message{}

	for stageIndex := 0; stageIndex < info.StageCount; stageIndex++ {
		var stage preparedStage
		if err := callProtocol(protocolRequest{
			Command:  "prepare_stage",
			TestCase: testCaseName,
			Stage:    stageIndex,
			Context:  &context,
			Profile:  config.Profile,
		}, &stage); err != nil {
			t.Fatal(err)
		}

		var input llmsdk.LanguageModelInput
		if err := json.Unmarshal(stage.Input, &input); err != nil {
			t.Fatalf("decode prepared stage input: %v", err)
		}
		stageMessages := append([]llmsdk.Message(nil), input.Messages...)
		input.Messages = appendMessages(history, stageMessages...)

		var assistantContent []llmsdk.Part
		switch stage.Method {
		case "generate":
			result, err := model.Generate(ctx, &input)
			if err != nil {
				t.Fatalf("generate failed: %v", err)
			}
			assistantContent = result.Content
		case "stream":
			stream, err := model.Stream(ctx, &input)
			if err != nil {
				t.Fatalf("stream failed: %v", err)
			}
			accumulator := llmsdk.NewStreamAccumulator()
			for stream.Next() {
				if err := accumulator.AddPartial(*stream.Current()); err != nil {
					t.Fatalf("add stream partial: %v", err)
				}
			}
			if err := stream.Err(); err != nil {
				t.Fatalf("stream failed: %v", err)
			}
			result, err := accumulator.ComputeResponse()
			if err != nil {
				t.Fatalf("compute stream response: %v", err)
			}
			assistantContent = result.Content
		default:
			t.Fatalf("unsupported shared test method %q", stage.Method)
		}

		if err := callProtocol(protocolRequest{
			Command:  "validate_output",
			TestCase: testCaseName,
			Stage:    stageIndex,
			Content:  assistantContent,
			Profile:  config.Profile,
		}, nil); err != nil {
			t.Fatal(err)
		}

		history = appendMessages(input.Messages, llmsdk.NewAssistantMessage(assistantContent...))
		context.Stages = append(context.Stages, stageResult{
			Assistant: assistantContent,
			ToolCalls: getToolCalls(assistantContent),
		})
	}
}
