package llmagent_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/llmsdktest"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"
)

// MockAgentTool implements llmagent.AgentTool for testing
type MockAgentTool[C any] struct {
	name        string
	description string
	parameters  llmsdk.JSONSchema
	executeFunc func(ctx context.Context, params json.RawMessage, contextVal C, runState *llmagent.RunState) (llmagent.AgentToolResult, error)
	LastArgs    json.RawMessage
	LastContext C
	AllCalls    []json.RawMessage // Track all calls for multiple call scenarios
}

func NewMockTool[C any](name string, result llmagent.AgentToolResult, executeFunc func(ctx context.Context, params json.RawMessage, contextVal C, runState *llmagent.RunState) (llmagent.AgentToolResult, error)) *MockAgentTool[C] {
	if executeFunc == nil {
		executeFunc = func(ctx context.Context, params json.RawMessage, contextVal C, runState *llmagent.RunState) (llmagent.AgentToolResult, error) {
			return result, nil
		}
	}
	return &MockAgentTool[C]{
		name:        name,
		description: "Mock tool " + name,
		parameters: llmsdk.JSONSchema{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		executeFunc: executeFunc,
	}
}

type mockToolkit[C any] struct {
	createFn func(ctx context.Context, contextVal C) (llmagent.ToolkitSession[C], error)
}

func (m *mockToolkit[C]) CreateSession(ctx context.Context, contextVal C) (llmagent.ToolkitSession[C], error) {
	return m.createFn(ctx, contextVal)
}

type mockToolkitSession[C any] struct {
	systemPrompt      *string
	tools             []llmagent.AgentTool[C]
	systemPromptCalls int
	toolsCalls        int
	closeCalls        int
	closeErr          error
}

func (m *mockToolkitSession[C]) SystemPrompt() *string {
	m.systemPromptCalls++
	return m.systemPrompt
}

func (m *mockToolkitSession[C]) Tools() []llmagent.AgentTool[C] {
	m.toolsCalls++
	return m.tools
}

func (m *mockToolkitSession[C]) Close(context.Context) error {
	m.closeCalls++
	return m.closeErr
}

func (t *MockAgentTool[C]) Name() string {
	return t.name
}

func (t *MockAgentTool[C]) Description() string {
	return t.description
}

func (t *MockAgentTool[C]) Parameters() llmsdk.JSONSchema {
	return t.parameters
}

func (t *MockAgentTool[C]) Execute(ctx context.Context, params json.RawMessage, contextVal C, runState *llmagent.RunState) (llmagent.AgentToolResult, error) {
	t.LastArgs = params
	t.LastContext = contextVal
	t.AllCalls = append(t.AllCalls, params)
	return t.executeFunc(ctx, params, contextVal, runState)
}

func mustNewRunSession[C any](t *testing.T, params *llmagent.AgentParams[C], contextVal C) *llmagent.RunSession[C] {
	t.Helper()
	session, err := llmagent.NewRunSession(t.Context(), params, contextVal)
	if err != nil {
		t.Fatalf("failed to create run session: %v", err)
	}
	return session
}

// -------- Root-level tests (Run) --------

func TestRun_ReturnsResponse_NoToolCall(t *testing.T) {
	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart("Hi!"),
			},
		}),
	)

	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[map[string]interface{}]{
			Name:           "test_agent",
			Model:          model,
			Instructions:   []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:          []llmagent.AgentTool[map[string]interface{}]{},
			ResponseFormat: llmsdk.NewResponseFormatText(),
			MaxTurns:       10,
		},
		map[string]interface{}{},
	)

	response, err := session.Run(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Hello!"))),
		},
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedResponse := &llmagent.AgentResponse{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart("Hi!"),
		},
		Output: []llmagent.AgentItem{
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					llmsdk.NewTextPart("Hi!"),
				},
			}),
		},
	}

	if diff := cmp.Diff(expectedResponse, response); diff != "" {
		t.Errorf("response mismatch (-want +got):\n%s", diff)
	}
}

func TestRun_ExecutesSingleToolCallAndReturnsResponse(t *testing.T) {
	toolResult := llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart("Tool result"),
		},
		IsError: false,
	}

	tool := NewMockTool[map[string]interface{}]("test_tool", toolResult, nil)

	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				llmsdk.NewToolCallPart("call_1", "test_tool", json.RawMessage(`{"param": "value"}`)),
			},
			Usage: &llmsdk.ModelUsage{
				InputTokens:  1000,
				OutputTokens: 50,
			},
			Cost: ptr.To(0.0015),
		}),
	)
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart("Final response"),
			},
		}),
	)

	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[map[string]interface{}]{
			Name:           "test_agent",
			Model:          model,
			Instructions:   []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:          []llmagent.AgentTool[map[string]interface{}]{tool},
			ResponseFormat: llmsdk.NewResponseFormatText(),
			MaxTurns:       10,
		},
		map[string]interface{}{"testContext": true},
	)

	response, err := session.Run(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Use the tool"))),
		},
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify tool was called with correct arguments
	var args map[string]interface{}
	if err := json.Unmarshal(tool.LastArgs, &args); err != nil {
		t.Fatalf("failed to unmarshal tool args: %v", err)
	}

	if args["param"] != "value" {
		t.Errorf("expected param=value, got param=%v", args["param"])
	}

	// Verify context was passed correctly
	if testCtx, ok := tool.LastContext["testContext"].(bool); !ok || !testCtx {
		t.Errorf("expected testContext=true, got %v", tool.LastContext)
	}

	expectedResponse := &llmagent.AgentResponse{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart("Final response"),
		},
		Output: []llmagent.AgentItem{
			// Assistant tool call model item
			llmagent.NewAgentItemModelResponse(
				llmsdk.ModelResponse{
					Content: []llmsdk.Part{
						llmsdk.NewToolCallPart("call_1", "test_tool", json.RawMessage(`{"param": "value"}`)),
					},
					Usage: &llmsdk.ModelUsage{InputTokens: 1000, OutputTokens: 50},
					Cost:  ptr.To(0.0015),
				},
			),
			// Tool result item
			llmagent.NewAgentItemTool(
				"call_1",
				"test_tool",
				json.RawMessage(`{"param": "value"}`),
				[]llmsdk.Part{llmsdk.NewTextPart("Tool result")},
				false,
			),
			// Final model item
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					llmsdk.NewTextPart("Final response"),
				},
			}),
		},
	}

	if diff := cmp.Diff(expectedResponse, response); diff != "" {
		t.Errorf("response mismatch (-want +got):\n%s", diff)
	}
}

func TestRun_ExecutesMultipleToolCallsInParallel(t *testing.T) {
	tool1Result := llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart("Tool 1 result"),
		},
		IsError: false,
	}

	tool2Result := llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart("Tool 2 result"),
		},
		IsError: false,
	}

	tool1 := NewMockTool[map[string]interface{}]("tool_1", tool1Result, nil)
	tool2 := NewMockTool[map[string]interface{}]("tool_2", tool2Result, nil)

	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				llmsdk.NewToolCallPart("call_1", "tool_1", map[string]any{"param": "value1"}),
				llmsdk.NewToolCallPart("call_2", "tool_2", map[string]any{"param": "value2"}),
			},
			Usage: &llmsdk.ModelUsage{
				InputTokens:  2000,
				OutputTokens: 100,
			},
		}),
	)
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart("Processed both tools"),
			},
			Usage: &llmsdk.ModelUsage{
				InputTokens:  50,
				OutputTokens: 10,
			},
			Cost: ptr.To(0.0003),
		}),
	)

	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[map[string]interface{}]{
			Name:           "test_agent",
			Model:          model,
			Instructions:   []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:          []llmagent.AgentTool[map[string]interface{}]{tool1, tool2},
			ResponseFormat: llmsdk.NewResponseFormatText(),
			MaxTurns:       10,
		},
		map[string]interface{}{},
	)

	response, err := session.Run(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Use both tools"))),
		},
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify tool1 was called with correct arguments
	var tool1Args map[string]interface{}
	if err := json.Unmarshal(tool1.LastArgs, &tool1Args); err != nil {
		t.Fatalf("failed to unmarshal tool1 args: %v", err)
	}
	if tool1Args["param"] != "value1" {
		t.Errorf("expected tool1 param=value1, got param=%v", tool1Args["param"])
	}

	// Verify tool2 was called with correct arguments
	var tool2Args map[string]interface{}
	if err := json.Unmarshal(tool2.LastArgs, &tool2Args); err != nil {
		t.Fatalf("failed to unmarshal tool2 args: %v", err)
	}
	if tool2Args["param"] != "value2" {
		t.Errorf("expected tool2 param=value2, got param=%v", tool2Args["param"])
	}

	expectedResponse := &llmagent.AgentResponse{
		Content: []llmsdk.Part{llmsdk.NewTextPart("Processed both tools")},
		Output: []llmagent.AgentItem{
			// model with tool calls
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					llmsdk.NewToolCallPart("call_1", "tool_1", map[string]any{"param": "value1"}),
					llmsdk.NewToolCallPart("call_2", "tool_2", map[string]any{"param": "value2"}),
				},
				Usage: &llmsdk.ModelUsage{InputTokens: 2000, OutputTokens: 100},
			}),
			llmagent.NewAgentItemTool(
				"call_1",
				"tool_1",
				json.RawMessage(`{"param":"value1"}`),
				[]llmsdk.Part{llmsdk.NewTextPart("Tool 1 result")},
				false,
			),
			llmagent.NewAgentItemTool(
				"call_2",
				"tool_2",
				json.RawMessage(`{"param":"value2"}`),
				[]llmsdk.Part{llmsdk.NewTextPart("Tool 2 result")},
				false,
			),
			llmagent.NewAgentItemModelResponse(
				llmsdk.ModelResponse{
					Content: []llmsdk.Part{llmsdk.NewTextPart("Processed both tools")},
					Usage:   &llmsdk.ModelUsage{InputTokens: 50, OutputTokens: 10},
					Cost:    ptr.To(0.0003),
				},
			),
		},
	}

	if diff := cmp.Diff(expectedResponse, response); diff != "" {
		t.Errorf("response mismatch (-want +got): %v", diff)
	}
}

func TestRun_HandlesMultipleTurnsWithToolCalls(t *testing.T) {
	toolResult := llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart("Calculation result"),
		},
		IsError: false,
	}

	tool := NewMockTool[map[string]interface{}]("calculator", toolResult, nil)

	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				llmsdk.NewToolCallPart(
					"call_1",
					"calculator",
					json.RawMessage([]byte(`{"operation": "add", "a": 1, "b": 2}`)),
				),
			},
		}),
	)
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				llmsdk.NewToolCallPart(
					"call_2",
					"calculator",
					json.RawMessage([]byte(`{"operation": "multiply", "a": 3, "b": 4}`)),
				),
			},
		}),
	)
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart("All calculations done"),
			},
		}),
	)

	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[map[string]interface{}]{
			Name:           "test_agent",
			Model:          model,
			Instructions:   []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:          []llmagent.AgentTool[map[string]interface{}]{tool},
			ResponseFormat: llmsdk.NewResponseFormatText(),
			MaxTurns:       10,
		},
		map[string]interface{}{},
	)

	response, err := session.Run(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Calculate some numbers"))),
		},
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(tool.AllCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(tool.AllCalls))
	}

	var firstCallArgs map[string]interface{}
	if err := json.Unmarshal(tool.AllCalls[0], &firstCallArgs); err != nil {
		t.Fatalf("failed to unmarshal first call args: %v", err)
	}

	expectedFirstCall := map[string]interface{}{
		"operation": "add",
		"a":         float64(1),
		"b":         float64(2),
	}

	if firstCallArgs["operation"] != expectedFirstCall["operation"] ||
		firstCallArgs["a"] != expectedFirstCall["a"] ||
		firstCallArgs["b"] != expectedFirstCall["b"] {
		t.Errorf("expected first call %v, got %v", expectedFirstCall, firstCallArgs)
	}

	var secondCallArgs map[string]interface{}
	if err := json.Unmarshal(tool.AllCalls[1], &secondCallArgs); err != nil {
		t.Fatalf("failed to unmarshal second call args: %v", err)
	}

	expectedSecondCall := map[string]interface{}{
		"operation": "multiply",
		"a":         float64(3),
		"b":         float64(4),
	}

	if secondCallArgs["operation"] != expectedSecondCall["operation"] ||
		secondCallArgs["a"] != expectedSecondCall["a"] ||
		secondCallArgs["b"] != expectedSecondCall["b"] {
		t.Errorf("expected second call %v, got %v", expectedSecondCall, secondCallArgs)
	}

	expectedResponse := &llmagent.AgentResponse{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart("All calculations done"),
		},
		Output: []llmagent.AgentItem{
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					llmsdk.NewToolCallPart(
						"call_1",
						"calculator",
						json.RawMessage(`{"operation": "add", "a": 1, "b": 2}`),
					),
				},
			}),
			llmagent.NewAgentItemTool(
				"call_1",
				"calculator",
				json.RawMessage(`{"operation": "add", "a": 1, "b": 2}`),
				[]llmsdk.Part{llmsdk.NewTextPart("Calculation result")},
				false,
			),
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					llmsdk.NewToolCallPart(
						"call_2",
						"calculator",
						json.RawMessage(`{"operation": "multiply", "a": 3, "b": 4}`),
					),
				},
			}),
			llmagent.NewAgentItemTool(
				"call_2",
				"calculator",
				json.RawMessage(`{"operation": "multiply", "a": 3, "b": 4}`),
				[]llmsdk.Part{llmsdk.NewTextPart("Calculation result")},
				false,
			),
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					llmsdk.NewTextPart("All calculations done"),
				},
			}),
		},
	}

	if diff := cmp.Diff(expectedResponse, response); diff != "" {
		t.Errorf("response mismatch (-want +got):\n%s", diff)
	}
}

func TestRun_ReturnsExistingAssistantResponseWithoutNewModelOutput(t *testing.T) {
	model := llmsdktest.NewMockLanguageModel()

	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[map[string]interface{}]{
			Name:           "cached",
			Model:          model,
			Instructions:   nil,
			Tools:          nil,
			ResponseFormat: llmsdk.NewResponseFormatText(),
			MaxTurns:       10,
		},
		map[string]interface{}{},
	)

	response, err := session.Run(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("What did I say?"))),
			llmagent.NewAgentItemMessage(llmsdk.NewAssistantMessage(llmsdk.NewTextPart("Cached answer"))),
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := &llmagent.AgentResponse{
		Content: []llmsdk.Part{llmsdk.NewTextPart("Cached answer")},
		Output:  []llmagent.AgentItem{},
	}
	if diff := cmp.Diff(expected, response); diff != "" {
		t.Errorf("response mismatch (-want +got):\n%s", diff)
	}

	if len(model.TrackedGenerateInputs()) != 0 {
		t.Fatalf("expected no model invocations, got %d", len(model.TrackedGenerateInputs()))
	}

	if err := session.Close(t.Context()); err != nil {
		t.Fatalf("expected no close error, got %v", err)
	}
}

func TestRun_ResumesToolProcessingFromToolMessageWithPartialResults(t *testing.T) {
	tool := NewMockTool[map[string]interface{}](
		"resume_tool",
		llmagent.AgentToolResult{
			Content: []llmsdk.Part{llmsdk.NewTextPart("call_2 result")},
			IsError: false,
		},
		nil,
	)

	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{llmsdk.NewTextPart("Final reply")},
		}),
	)

	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[map[string]interface{}]{
			Name:           "resumable",
			Model:          model,
			Tools:          []llmagent.AgentTool[map[string]interface{}]{tool},
			ResponseFormat: llmsdk.NewResponseFormatText(),
			MaxTurns:       10,
		},
		map[string]interface{}{},
	)

	call1Args := json.RawMessage(`{"step": 1}`)
	call2Args := json.RawMessage(`{"step": 2}`)
	isError := false
	toolResult := llmsdk.NewToolResultPart("call_1", "resume_tool", []llmsdk.Part{llmsdk.NewTextPart("already done")}, isError)

	response, err := session.Run(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Continue"))),
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					llmsdk.NewToolCallPart("call_1", "resume_tool", call1Args),
					llmsdk.NewToolCallPart("call_2", "resume_tool", call2Args),
				},
			}),
			llmagent.NewAgentItemMessage(llmsdk.NewToolMessage(toolResult)),
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(tool.AllCalls) != 1 || string(tool.AllCalls[0]) != string(call2Args) {
		t.Fatalf("unexpected tool calls: %v", tool.AllCalls)
	}

	expected := &llmagent.AgentResponse{
		Content: []llmsdk.Part{llmsdk.NewTextPart("Final reply")},
		Output: []llmagent.AgentItem{
			llmagent.NewAgentItemTool("call_2", "resume_tool", call2Args, []llmsdk.Part{llmsdk.NewTextPart("call_2 result")}, false),
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{Content: []llmsdk.Part{llmsdk.NewTextPart("Final reply")}}),
		},
	}
	if diff := cmp.Diff(expected, response); diff != "" {
		t.Errorf("response mismatch (-want +got):\n%s", diff)
	}

	if err := session.Close(t.Context()); err != nil {
		t.Fatalf("expected no close error, got %v", err)
	}
}

func TestRun_ResumesToolProcessingWhenTrailingToolEntries(t *testing.T) {
	tool := NewMockTool[map[string]interface{}](
		"resume_tool",
		llmagent.AgentToolResult{
			Content: []llmsdk.Part{llmsdk.NewTextPart("call_2 via item")},
			IsError: false,
		},
		nil,
	)

	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{llmsdk.NewTextPart("Final reply from items")},
		}),
	)

	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[map[string]interface{}]{
			Name:           "resumable_tool_items",
			Model:          model,
			Tools:          []llmagent.AgentTool[map[string]interface{}]{tool},
			ResponseFormat: llmsdk.NewResponseFormatText(),
			MaxTurns:       10,
		},
		map[string]interface{}{},
	)

	call1Args := json.RawMessage(`{"stage": 1}`)
	call2Args := json.RawMessage(`{"stage": 2}`)
	response, err := session.Run(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Continue"))),
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					llmsdk.NewToolCallPart("call_1", "resume_tool", call1Args),
					llmsdk.NewToolCallPart("call_2", "resume_tool", call2Args),
				},
			}),
			llmagent.NewAgentItemTool("call_1", "resume_tool", call1Args, []llmsdk.Part{llmsdk.NewTextPart("already done")}, false),
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(tool.AllCalls) != 1 || string(tool.AllCalls[0]) != string(call2Args) {
		t.Fatalf("unexpected tool calls: %v", tool.AllCalls)
	}

	expected := &llmagent.AgentResponse{
		Content: []llmsdk.Part{llmsdk.NewTextPart("Final reply from items")},
		Output: []llmagent.AgentItem{
			llmagent.NewAgentItemTool("call_2", "resume_tool", call2Args, []llmsdk.Part{llmsdk.NewTextPart("call_2 via item")}, false),
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{Content: []llmsdk.Part{llmsdk.NewTextPart("Final reply from items")}}),
		},
	}
	if diff := cmp.Diff(expected, response); diff != "" {
		t.Errorf("response mismatch (-want +got):\n%s", diff)
	}

	if err := session.Close(t.Context()); err != nil {
		t.Fatalf("expected no close error, got %v", err)
	}
}

func TestRun_ReturnsErrorWhenToolResultsLackPrecedingAssistantContent(t *testing.T) {
	tool := NewMockTool[map[string]interface{}](
		"resume_tool",
		llmagent.AgentToolResult{
			Content: []llmsdk.Part{llmsdk.NewTextPart("unused")},
			IsError: false,
		},
		nil,
	)

	model := llmsdktest.NewMockLanguageModel()

	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[map[string]interface{}]{
			Name:           "resumable_error",
			Model:          model,
			Tools:          []llmagent.AgentTool[map[string]interface{}]{tool},
			ResponseFormat: llmsdk.NewResponseFormatText(),
			MaxTurns:       10,
		},
		map[string]interface{}{},
	)

	isError := false
	_, err := session.Run(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Resume"))),
			llmagent.NewAgentItemMessage(llmsdk.NewToolMessage(llmsdk.NewToolResultPart(
				"call_1",
				"resume_tool",
				[]llmsdk.Part{llmsdk.NewTextPart("orphan")},
				isError,
			))),
		},
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	var agentErr *llmagent.AgentError
	if !errors.As(err, &agentErr) || agentErr.Kind != llmagent.InvariantErrorKind {
		t.Fatalf("expected invariant error, got %v", err)
	}

	if err := session.Close(t.Context()); err != nil {
		t.Fatalf("expected no close error, got %v", err)
	}
}

func TestRun_ThrowsAgentMaxTurnsExceededError(t *testing.T) {
	toolResult := llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart("Tool result"),
		},
		IsError: false,
	}

	tool := NewMockTool[map[string]interface{}]("test_tool", toolResult, nil)

	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				llmsdk.NewToolCallPart("call_1", "test_tool", json.RawMessage(`{}`)),
			},
		}),
	)
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				llmsdk.NewToolCallPart("call_2", "test_tool", json.RawMessage(`{}`)),
			},
		}),
	)
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				llmsdk.NewToolCallPart("call_3", "test_tool", json.RawMessage(`{}`)),
			},
		}),
	)

	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[map[string]interface{}]{
			Name:         "test_agent",
			Model:        model,
			Instructions: []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:        []llmagent.AgentTool[map[string]interface{}]{tool},
			MaxTurns:     2,
		},
		map[string]interface{}{},
	)

	_, err := session.Run(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Keep using tools"))),
		},
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var agentErr *llmagent.AgentError
	if !errors.As(err, &agentErr) {
		t.Fatalf("expected AgentError, got %T", err)
	}

	if agentErr.Kind != llmagent.AgentErrorKindMaxTurnsExceeded {
		t.Errorf("expected max turns exceeded error, got %s", agentErr.Kind)
	}
}

func TestRun_ThrowsAgentInvariantError_WhenToolNotFound(t *testing.T) {
	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				llmsdk.NewToolCallPart("call_1", "non_existent_tool", json.RawMessage(`{}`)),
			},
		}),
	)

	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[map[string]interface{}]{
			Name:         "test_agent",
			Model:        model,
			Instructions: []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:        []llmagent.AgentTool[map[string]interface{}]{},
			MaxTurns:     10,
		},
		map[string]interface{}{},
	)

	_, err := session.Run(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Use a tool"))),
		},
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var agentErr *llmagent.AgentError
	if !errors.As(err, &agentErr) {
		t.Fatalf("expected AgentError, got %T", err)
	}

	if agentErr.Kind != llmagent.InvariantErrorKind {
		t.Errorf("expected invariant error, got %s", agentErr.Kind)
	}
}

func TestRun_ThrowsAgentToolExecutionError_WhenToolExecutionFails(t *testing.T) {
	tool := NewMockTool("failing_tool", llmagent.AgentToolResult{}, func(ctx context.Context, params json.RawMessage, contextVal map[string]interface{}, runState *llmagent.RunState) (llmagent.AgentToolResult, error) {
		return llmagent.AgentToolResult{}, errors.New("tool execution failed")
	})

	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				llmsdk.NewToolCallPart("call_1", "failing_tool", json.RawMessage(`{}`)),
			},
		}),
	)

	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[map[string]interface{}]{
			Name:         "test_agent",
			Model:        model,
			Instructions: []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:        []llmagent.AgentTool[map[string]interface{}]{tool},
			MaxTurns:     10,
		},
		map[string]interface{}{},
	)

	_, err := session.Run(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Use the tool"))),
		},
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var agentErr *llmagent.AgentError
	if !errors.As(err, &agentErr) {
		t.Fatalf("expected AgentError, got %T", err)
	}

	if agentErr.Kind != llmagent.ToolExecutionErrorKind {
		t.Errorf("expected tool execution error, got %s", agentErr.Kind)
	}
}

func TestRun_HandlesToolReturningErrorResult(t *testing.T) {
	toolResult := llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart("Error: Invalid parameters"),
		},
		IsError: true,
	}

	tool := NewMockTool[map[string]interface{}]("test_tool", toolResult, nil)

	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				llmsdk.NewToolCallPart("call_1", "test_tool", json.RawMessage(`{"invalid": true}`)),
			},
		}),
	)
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart("Handled the error"),
			},
		}),
	)

	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[map[string]interface{}]{
			Name:         "test_agent",
			Model:        model,
			Instructions: []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:        []llmagent.AgentTool[map[string]interface{}]{tool},
			MaxTurns:     10,
		},
		map[string]interface{}{},
	)

	response, err := session.Run(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Use the tool"))),
		},
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(tool.AllCalls) != 1 {
		t.Fatalf("expected tool to be called once, got %d", len(tool.AllCalls))
	}

	var toolArgs map[string]interface{}
	if err := json.Unmarshal(tool.AllCalls[0], &toolArgs); err != nil {
		t.Fatalf("failed to unmarshal tool args: %v", err)
	}
	if toolArgs["invalid"] != true {
		t.Errorf("expected invalid=true, got %v", toolArgs["invalid"])
	}

	expectedResponse := &llmagent.AgentResponse{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart("Handled the error"),
		},
		Output: []llmagent.AgentItem{
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					llmsdk.NewToolCallPart("call_1", "test_tool", json.RawMessage(`{"invalid": true}`)),
				},
			}),
			llmagent.NewAgentItemTool(
				"call_1",
				"test_tool",
				json.RawMessage(`{"invalid": true}`),
				[]llmsdk.Part{llmsdk.NewTextPart("Error: Invalid parameters")},
				true,
			),
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					llmsdk.NewTextPart("Handled the error"),
				},
			}),
		},
	}

	if diff := cmp.Diff(expectedResponse, response); diff != "" {
		t.Errorf("response mismatch (-want +got):\n%s", diff)
	}
}

func TestRun_PassesSamplingParametersToModel(t *testing.T) {
	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart("Response"),
			},
		}),
	)

	temp := 0.7
	topP := 0.9
	topK := int32(40)
	presencePenalty := 0.1
	frequencyPenalty := 0.2

	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[map[string]interface{}]{
			Name:             "test_agent",
			Model:            model,
			Instructions:     []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:            []llmagent.AgentTool[map[string]interface{}]{},
			MaxTurns:         10,
			Temperature:      &temp,
			TopP:             &topP,
			TopK:             &topK,
			PresencePenalty:  &presencePenalty,
			FrequencyPenalty: &frequencyPenalty,
		},
		map[string]interface{}{},
	)

	_, err := session.Run(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Hello"))),
		},
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	trackedGenerateInputs := model.TrackedGenerateInputs()
	if len(trackedGenerateInputs) != 1 {
		t.Fatalf("expected 1 generate call, got %d", len(trackedGenerateInputs))
	}

	call := trackedGenerateInputs[0]
	if call.Temperature == nil || *call.Temperature != temp {
		t.Errorf("expected temperature %f, got %v", temp, call.Temperature)
	}
	if call.TopP == nil || *call.TopP != topP {
		t.Errorf("expected topP %f, got %v", topP, call.TopP)
	}
	if call.TopK == nil || *call.TopK != topK {
		t.Errorf("expected topK %d, got %v", topK, call.TopK)
	}
	if call.PresencePenalty == nil || *call.PresencePenalty != presencePenalty {
		t.Errorf("expected presencePenalty %f, got %v", presencePenalty, call.PresencePenalty)
	}
	if call.FrequencyPenalty == nil || *call.FrequencyPenalty != frequencyPenalty {
		t.Errorf("expected frequencyPenalty %f, got %v", frequencyPenalty, call.FrequencyPenalty)
	}
}

func TestRun_IncludesStringAndDynamicFunctionInstructionsInSystemPrompt(t *testing.T) {
	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart("Response"),
			},
		}),
	)

	instructions := []llmagent.InstructionParam[map[string]interface{}]{
		{String: ptr.To("You are a helpful assistant.")},
		{Func: func(ctx context.Context, ctxVal map[string]interface{}) (string, error) {
			if userRole, ok := ctxVal["userRole"].(string); ok {
				return "The user is a " + userRole + ".", nil
			}
			return "", nil
		}},
		{String: ptr.To("Always be polite.")},
	}

	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[map[string]interface{}]{
			Name:         "test_agent",
			Model:        model,
			Instructions: instructions,
			Tools:        []llmagent.AgentTool[map[string]interface{}]{},
			MaxTurns:     10,
		},
		map[string]interface{}{"userRole": "developer"},
	)

	_, err := session.Run(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Hello"))),
		},
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	trackedGenerateInputs := model.TrackedGenerateInputs()
	if len(trackedGenerateInputs) != 1 {
		t.Fatalf("expected 1 generate call, got %d", len(trackedGenerateInputs))
	}

	call := trackedGenerateInputs[0]
	expectedSystemPrompt := "You are a helpful assistant.\nThe user is a developer.\nAlways be polite."

	if call.SystemPrompt == nil || *call.SystemPrompt != expectedSystemPrompt {
		t.Errorf("expected system prompt %q, got %q", expectedSystemPrompt, *call.SystemPrompt)
	}
}

func TestRun_MergesToolkitPromptsAndTools(t *testing.T) {
	type customerContext struct {
		Customer string
	}

	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				llmsdk.NewToolCallPart("call-1", "lookup-order", json.RawMessage(`{"orderId":"123"}`)),
			},
		}),
	)
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{llmsdk.NewTextPart("Order ready")},
		}),
	)

	executed := []struct {
		Context customerContext
		Args    map[string]string
		Turn    uint
	}{}

	dynamicTool := NewMockTool[customerContext](
		"lookup-order",
		llmagent.AgentToolResult{},
		func(ctx context.Context, params json.RawMessage, contextVal customerContext, runState *llmagent.RunState) (llmagent.AgentToolResult, error) {
			var args map[string]string
			if err := json.Unmarshal(params, &args); err != nil {
				return llmagent.AgentToolResult{}, err
			}
			executed = append(executed, struct {
				Context customerContext
				Args    map[string]string
				Turn    uint
			}{
				Context: contextVal,
				Args:    args,
				Turn:    runState.CurrentTurn,
			})

			orderID := args["orderId"]
			text := "Order " + orderID + " ready for " + contextVal.Customer
			return llmagent.AgentToolResult{
				Content: []llmsdk.Part{llmsdk.NewTextPart(text)},
				IsError: false,
			}, nil
		},
	)

	toolkitPrompt := "Toolkit prompt"
	toolkitSession := &mockToolkitSession[customerContext]{
		systemPrompt: &toolkitPrompt,
		tools:        []llmagent.AgentTool[customerContext]{dynamicTool},
	}

	var createdContexts []customerContext
	toolkit := &mockToolkit[customerContext]{
		createFn: func(ctx context.Context, contextVal customerContext) (llmagent.ToolkitSession[customerContext], error) {
			createdContexts = append(createdContexts, contextVal)
			return toolkitSession, nil
		},
	}

	ctxVal := customerContext{Customer: "Ada"}
	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[customerContext]{
			Name:           "toolkit-agent",
			Model:          model,
			Instructions:   []llmagent.InstructionParam[customerContext]{},
			Tools:          []llmagent.AgentTool[customerContext]{},
			Toolkits:       []llmagent.Toolkit[customerContext]{toolkit},
			ResponseFormat: llmsdk.NewResponseFormatText(),
			MaxTurns:       10,
		},
		ctxVal,
	)

	response, err := session.Run(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Status?"))),
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if diff := cmp.Diff([]customerContext{ctxVal}, createdContexts); diff != "" {
		t.Fatalf("unexpected contexts (-want +got):\n%s", diff)
	}

	if len(executed) != 1 {
		t.Fatalf("expected 1 tool execution, got %d", len(executed))
	}
	if executed[0].Context != ctxVal {
		t.Fatalf("unexpected tool context: %v", executed[0].Context)
	}
	if executed[0].Args["orderId"] != "123" {
		t.Fatalf("expected orderId 123, got %s", executed[0].Args["orderId"])
	}
	if executed[0].Turn != 1 {
		t.Fatalf("expected execution on turn 1, got %d", executed[0].Turn)
	}

	inputs := model.TrackedGenerateInputs()
	if len(inputs) != 2 {
		t.Fatalf("expected 2 tracked generate inputs, got %d", len(inputs))
	}
	for _, input := range inputs {
		if input.SystemPrompt == nil || *input.SystemPrompt != toolkitPrompt {
			t.Fatalf("unexpected system prompt: %v", input.SystemPrompt)
		}
		if len(input.Tools) != 1 {
			t.Fatalf("expected 1 tool, got %d", len(input.Tools))
		}
		if input.Tools[0].Name != "lookup-order" {
			t.Fatalf("unexpected tool name: %s", input.Tools[0].Name)
		}
	}

	expectedResponse := &llmagent.AgentResponse{
		Content: []llmsdk.Part{llmsdk.NewTextPart("Order ready")},
		Output: []llmagent.AgentItem{
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					llmsdk.NewToolCallPart("call-1", "lookup-order", json.RawMessage(`{"orderId":"123"}`)),
				},
			}),
			llmagent.NewAgentItemTool(
				"call-1",
				"lookup-order",
				json.RawMessage(`{"orderId":"123"}`),
				[]llmsdk.Part{llmsdk.NewTextPart("Order 123 ready for Ada")},
				false,
			),
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{llmsdk.NewTextPart("Order ready")},
			}),
		},
	}

	if diff := cmp.Diff(expectedResponse, response); diff != "" {
		t.Fatalf("response mismatch (-want +got):\n%s", diff)
	}

	if err := session.Close(t.Context()); err != nil {
		t.Fatalf("expected no close error, got %v", err)
	}
	if toolkitSession.closeCalls != 1 {
		t.Fatalf("expected toolkit close once, got %d", toolkitSession.closeCalls)
	}
}

// -------- Root-level tests (RunStream) --------

func TestRunStream_StreamsResponse_NoToolCall(t *testing.T) {
	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueStreamResult(
		llmsdktest.NewMockStreamResultPartials([]llmsdk.PartialModelResponse{
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta("Hel")}},
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta("lo")}},
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta("!")}},
		}),
	)

	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[map[string]interface{}]{
			Name:         "test_agent",
			Model:        model,
			Instructions: []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:        []llmagent.AgentTool[map[string]interface{}]{},
			MaxTurns:     10,
		},
		map[string]interface{}{},
	)

	stream, err := session.RunStream(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Hi"))),
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	events := []*llmagent.AgentStreamEvent{}
	for stream.Next() {
		events = append(events, stream.Current())
	}

	if err := stream.Err(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedEvents := []*llmagent.AgentStreamEvent{
		{
			Partial: &llmsdk.PartialModelResponse{
				Delta: &llmsdk.ContentDelta{
					Index: 0,
					Part:  llmsdk.NewTextPartDelta("Hel"),
				},
			},
		},
		{
			Partial: &llmsdk.PartialModelResponse{
				Delta: &llmsdk.ContentDelta{
					Index: 0,
					Part:  llmsdk.NewTextPartDelta("lo"),
				},
			},
		},
		{
			Partial: &llmsdk.PartialModelResponse{
				Delta: &llmsdk.ContentDelta{
					Index: 0,
					Part:  llmsdk.NewTextPartDelta("!"),
				},
			},
		},
		{
			Item: &llmagent.AgentStreamItemEvent{
				Item: llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
					Content: []llmsdk.Part{
						llmsdk.NewTextPart("Hello!"),
					},
				}),
			},
		},
		{
			Response: &llmagent.AgentResponse{
				Content: []llmsdk.Part{
					llmsdk.NewTextPart("Hello!"),
				},
				Output: []llmagent.AgentItem{
					llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
						Content: []llmsdk.Part{
							llmsdk.NewTextPart("Hello!"),
						},
					}),
				},
			},
		},
	}

	if diff := cmp.Diff(expectedEvents, events); diff != "" {
		t.Errorf("stream events mismatch (-want +got):\n%s", diff)
	}
}

func TestRunStream_StreamsToolCallExecutionAndResponse(t *testing.T) {
	toolResult := llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart("Tool result"),
		},
		IsError: false,
	}

	tool := NewMockTool[map[string]interface{}]("test_tool", toolResult, nil)

	toolName := "test_tool"
	callId := "call_1"
	args := `{"operation": "add", "a": 1, "b": 2}`

	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueStreamResult(
		llmsdktest.NewMockStreamResultPartials([]llmsdk.PartialModelResponse{
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewToolCallPartDelta(
				llmsdk.WithToolCallPartDeltaToolName(toolName),
				llmsdk.WithToolCallPartDeltaToolCallID(callId),
				llmsdk.WithToolCallPartDeltaArgs(args),
			)}},
		}),
	)
	model.EnqueueStreamResult(
		llmsdktest.NewMockStreamResultPartials([]llmsdk.PartialModelResponse{
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta("Final response")}},
		}),
	)

	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[map[string]interface{}]{
			Name:         "test_agent",
			Model:        model,
			Instructions: []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:        []llmagent.AgentTool[map[string]interface{}]{tool},
			MaxTurns:     10,
		},
		map[string]interface{}{},
	)

	stream, err := session.RunStream(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Use tool"))),
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	events := []*llmagent.AgentStreamEvent{}
	for stream.Next() {
		events = append(events, stream.Current())
	}

	if err := stream.Err(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	summaries := make([]string, 0, len(events))
	for _, event := range events {
		switch {
		case event.Partial != nil:
			if delta := event.Partial.Delta; delta != nil {
				if delta.Part.ToolCallPartDelta != nil && delta.Part.ToolCallPartDelta.ToolName != nil {
					summaries = append(summaries, "partial:tool-call:"+*delta.Part.ToolCallPartDelta.ToolName)
				} else if delta.Part.TextPartDelta != nil {
					summaries = append(summaries, "partial:text:"+delta.Part.TextPartDelta.Text)
				} else {
					summaries = append(summaries, "partial:other")
				}
			} else {
				summaries = append(summaries, "partial:other")
			}
		case event.Item != nil:
			itemEvent := event.Item
			if itemEvent.Item.Model != nil {
				if len(itemEvent.Item.Model.Content) > 0 {
					part := itemEvent.Item.Model.Content[0]
					if part.ToolCallPart != nil {
						summaries = append(summaries, "item:model:tool-call:"+part.ToolCallPart.ToolName)
					} else if part.TextPart != nil {
						summaries = append(summaries, "item:model:text:"+part.TextPart.Text)
					} else {
						summaries = append(summaries, "item:model:other")
					}
				} else {
					summaries = append(summaries, "item:model:empty")
				}
			} else if itemEvent.Item.Tool != nil {
				suffix := ":false"
				if itemEvent.Item.Tool.IsError {
					suffix = ":true"
				}
				summaries = append(summaries, "item:tool:"+itemEvent.Item.Tool.ToolName+suffix)
			} else {
				summaries = append(summaries, "item:other")
			}
		case event.Response != nil:
			if len(event.Response.Content) > 0 && event.Response.Content[0].TextPart != nil {
				summaries = append(summaries, "response:text:"+event.Response.Content[0].TextPart.Text)
			} else {
				summaries = append(summaries, "response:other")
			}
		default:
			summaries = append(summaries, "unknown")
		}
	}

	expectedSummaries := []string{
		"partial:tool-call:test_tool",
		"item:model:tool-call:test_tool",
		"item:tool:test_tool:false",
		"partial:text:Final response",
		"item:model:text:Final response",
		"response:text:Final response",
	}

	if diff := cmp.Diff(expectedSummaries, summaries); diff != "" {
		t.Errorf("event summary mismatch (-want +got):\n%s", diff)
	}

	finalEvent := events[len(events)-1]
	if finalEvent.Response == nil {
		t.Fatal("expected final event to include a response")
	}

	expectedResponse := &llmagent.AgentResponse{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart("Final response"),
		},
		Output: []llmagent.AgentItem{
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					llmsdk.NewToolCallPart("call_1", "test_tool", map[string]any{"operation": "add", "a": 1, "b": 2}),
				},
			}),
			llmagent.NewAgentItemTool(
				"call_1",
				"test_tool",
				json.RawMessage(`{"a":1,"b":2,"operation":"add"}`),
				[]llmsdk.Part{llmsdk.NewTextPart("Tool result")},
				false,
			),
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					llmsdk.NewTextPart("Final response"),
				},
			}),
		},
	}

	if diff := cmp.Diff(expectedResponse, finalEvent.Response); diff != "" {
		t.Errorf("final response mismatch (-want +got):\n%s", diff)
	}

	// Verify tool call arguments
	var toolArgs map[string]interface{}
	if err := json.Unmarshal(tool.LastArgs, &toolArgs); err != nil {
		t.Fatalf("failed to unmarshal tool args: %v", err)
	}

	expectedArgs := map[string]interface{}{
		"operation": "add",
		"a":         float64(1),
		"b":         float64(2),
	}

	if toolArgs["operation"] != expectedArgs["operation"] ||
		toolArgs["a"] != expectedArgs["a"] ||
		toolArgs["b"] != expectedArgs["b"] {
		t.Errorf("expected tool args %v, got %v", expectedArgs, toolArgs)
	}
}

func TestRunStream_ThrowsErrorWhenMaxTurnsExceeded(t *testing.T) {
	toolResult := llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart("Tool result"),
		},
		IsError: false,
	}

	tool := NewMockTool[map[string]interface{}]("test_tool", toolResult, nil)

	toolName := "test_tool"
	callId1 := "call_1"
	callId2 := "call_2"
	callId3 := "call_3"
	args := "{}"

	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueStreamResult(
		llmsdktest.NewMockStreamResultPartials([]llmsdk.PartialModelResponse{
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewToolCallPartDelta(
				llmsdk.WithToolCallPartDeltaToolName(toolName),
				llmsdk.WithToolCallPartDeltaToolCallID(callId1),
				llmsdk.WithToolCallPartDeltaArgs(args),
			)}},
		}),
	)
	model.EnqueueStreamResult(
		llmsdktest.NewMockStreamResultPartials([]llmsdk.PartialModelResponse{
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewToolCallPartDelta(
				llmsdk.WithToolCallPartDeltaToolName(toolName),
				llmsdk.WithToolCallPartDeltaToolCallID(callId2),
				llmsdk.WithToolCallPartDeltaArgs(args),
			)}},
		}),
	)
	model.EnqueueStreamResult(
		llmsdktest.NewMockStreamResultPartials([]llmsdk.PartialModelResponse{
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewToolCallPartDelta(
				llmsdk.WithToolCallPartDeltaToolName(toolName),
				llmsdk.WithToolCallPartDeltaToolCallID(callId3),
				llmsdk.WithToolCallPartDeltaArgs(args),
			)}},
		}),
	)

	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[map[string]interface{}]{
			Name:         "test_agent",
			Model:        model,
			Instructions: []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:        []llmagent.AgentTool[map[string]interface{}]{tool},
			MaxTurns:     2,
		},
		map[string]interface{}{},
	)

	stream, err := session.RunStream(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Keep using tools"))),
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Consume events until error
	for stream.Next() {
		// consume events
	}

	err = stream.Err()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var agentErr *llmagent.AgentError
	if !errors.As(err, &agentErr) {
		t.Fatalf("expected AgentError, got %T", err)
	}

	if agentErr.Kind != llmagent.AgentErrorKindMaxTurnsExceeded {
		t.Errorf("expected max turns exceeded error, got %s", agentErr.Kind)
	}
}

func TestRunStream_MergesToolkitPromptsAndTools(t *testing.T) {
	type customerContext struct {
		Customer string
	}

	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueStreamResult(
		llmsdktest.NewMockStreamResultPartials([]llmsdk.PartialModelResponse{
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta("Done")}},
		}),
	)

	dynamicTool := NewMockTool[customerContext](
		"noop",
		llmagent.AgentToolResult{Content: []llmsdk.Part{}, IsError: false},
		nil,
	)

	toolkitPrompt := "Streaming toolkit prompt"
	toolkitSession := &mockToolkitSession[customerContext]{
		systemPrompt: &toolkitPrompt,
		tools:        []llmagent.AgentTool[customerContext]{dynamicTool},
	}

	var createdContexts []customerContext
	toolkit := &mockToolkit[customerContext]{
		createFn: func(ctx context.Context, contextVal customerContext) (llmagent.ToolkitSession[customerContext], error) {
			createdContexts = append(createdContexts, contextVal)
			return toolkitSession, nil
		},
	}

	ctxVal := customerContext{Customer: "Ben"}
	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[customerContext]{
			Name:           "toolkit-stream-agent",
			Model:          model,
			Instructions:   []llmagent.InstructionParam[customerContext]{},
			Tools:          []llmagent.AgentTool[customerContext]{},
			Toolkits:       []llmagent.Toolkit[customerContext]{toolkit},
			ResponseFormat: llmsdk.NewResponseFormatText(),
			MaxTurns:       10,
		},
		ctxVal,
	)

	stream, err := session.RunStream(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Hello"))),
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	events := []*llmagent.AgentStreamEvent{}
	for stream.Next() {
		events = append(events, stream.Current())
	}

	if err := stream.Err(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedEvents := []*llmagent.AgentStreamEvent{
		{
			Partial: &llmsdk.PartialModelResponse{
				Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta("Done")},
			},
		},
		llmagent.NewAgentStreamItemEvent(
			0,
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{llmsdk.NewTextPart("Done")},
			}),
		),
		{
			Response: &llmagent.AgentResponse{
				Content: []llmsdk.Part{llmsdk.NewTextPart("Done")},
				Output: []llmagent.AgentItem{
					llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
						Content: []llmsdk.Part{llmsdk.NewTextPart("Done")},
					}),
				},
			},
		},
	}

	if diff := cmp.Diff(expectedEvents, events); diff != "" {
		t.Fatalf("stream events mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([]customerContext{ctxVal}, createdContexts); diff != "" {
		t.Fatalf("unexpected contexts (-want +got):\n%s", diff)
	}
	inputs := model.TrackedStreamInputs()
	if len(inputs) != 1 {
		t.Fatalf("expected 1 tracked stream input, got %d", len(inputs))
	}
	if inputs[0].SystemPrompt == nil || *inputs[0].SystemPrompt != toolkitPrompt {
		t.Fatalf("unexpected system prompt: %v", inputs[0].SystemPrompt)
	}
	if len(inputs[0].Tools) != 1 || inputs[0].Tools[0].Name != "noop" {
		t.Fatalf("unexpected tools: %v", inputs[0].Tools)
	}

	if err := session.Close(t.Context()); err != nil {
		t.Fatalf("expected no close error, got %v", err)
	}
	if toolkitSession.closeCalls != 1 {
		t.Fatalf("expected toolkit close once, got %d", toolkitSession.closeCalls)
	}
}

// -------- Root-level lifecycle test --------

func TestRun_CloseCleansUpSessionResources(t *testing.T) {
	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart("Response"),
			},
		}),
	)

	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[map[string]interface{}]{
			Name:         "test_agent",
			Model:        model,
			Instructions: []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:        []llmagent.AgentTool[map[string]interface{}]{},
			MaxTurns:     10,
		},
		map[string]interface{}{},
	)

	_, err := session.Run(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Hello"))),
		},
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := session.Close(t.Context()); err != nil {
		t.Fatalf("expected no close error, got %v", err)
	}

	_, err = session.Run(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Hello again"))),
		},
	})

	if err == nil {
		t.Fatal("expected error when running after close, got nil")
	}

	var invariantErr *llmagent.AgentError
	if !errors.As(err, &invariantErr) || invariantErr.Kind != llmagent.InvariantErrorKind {
		t.Fatalf("expected invariant error after close, got %v", err)
	}
}
