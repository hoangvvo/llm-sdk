package llmagent_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
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

// -------- Root-level tests (Run) --------

func TestRun_ReturnsResponse_NoToolCall(t *testing.T) {
	model := llmsdk.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdk.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{TextPart: &llmsdk.TextPart{Text: "Hi!"}},
			},
		}),
	)

	session := llmagent.NewRunSession(
		&llmagent.AgentParams[map[string]interface{}]{
			Name:           "test_agent",
			Model:          model,
			Instructions:   []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:          []llmagent.AgentTool[map[string]interface{}]{},
			ResponseFormat: llmsdk.NewResponseFormatText(),
			MaxTurns:       10,
		},
	)

	response, err := session.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.Message{
				UserMessage: &llmsdk.UserMessage{
					Content: []llmsdk.Part{
						{TextPart: &llmsdk.TextPart{Text: "Hello!"}},
					},
				},
			}),
		},
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedResponse := &llmagent.AgentResponse{
		Content: []llmsdk.Part{
			{TextPart: &llmsdk.TextPart{Text: "Hi!"}},
		},
		Output: []llmagent.AgentItem{
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					{TextPart: &llmsdk.TextPart{Text: "Hi!"}},
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
			{TextPart: &llmsdk.TextPart{Text: "Tool result"}},
		},
		IsError: false,
	}

	tool := NewMockTool[map[string]interface{}]("test_tool", toolResult, nil)

	model := llmsdk.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdk.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{ToolCallPart: &llmsdk.ToolCallPart{
					ToolName:   "test_tool",
					ToolCallID: "call_1",
					Args:       json.RawMessage(`{"param": "value"}`),
				}},
			},
			Usage: &llmsdk.ModelUsage{
				InputTokens:  1000,
				OutputTokens: 50,
			},
			Cost: ptr.To(0.0015),
		}),
	)
	model.EnqueueGenerateResult(
		llmsdk.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{TextPart: &llmsdk.TextPart{Text: "Final response"}},
			},
		}),
	)

	session := llmagent.NewRunSession(
		&llmagent.AgentParams[map[string]interface{}]{
			Name:           "test_agent",
			Model:          model,
			Instructions:   []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:          []llmagent.AgentTool[map[string]interface{}]{tool},
			ResponseFormat: llmsdk.NewResponseFormatText(),
			MaxTurns:       10,
		},
	)

	response, err := session.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{"testContext": true},
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.Message{
				UserMessage: &llmsdk.UserMessage{
					Content: []llmsdk.Part{
						{TextPart: &llmsdk.TextPart{Text: "Use the tool"}},
					},
				},
			}),
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
			{TextPart: &llmsdk.TextPart{Text: "Final response"}},
		},
		Output: []llmagent.AgentItem{
			// Assistant tool call model item
			llmagent.NewAgentItemModelResponse(
				llmsdk.ModelResponse{
					Content: []llmsdk.Part{
						{ToolCallPart: &llmsdk.ToolCallPart{
							ToolName:   "test_tool",
							ToolCallID: "call_1",
							Args:       json.RawMessage(`{"param": "value"}`),
						}},
					},
					Usage: &llmsdk.ModelUsage{InputTokens: 1000, OutputTokens: 50},
					Cost:  ptr.To(0.0015),
				},
			),
			// Tool result item
			{
				Tool: &llmagent.AgentItemTool{
					ToolCallID: "call_1",
					ToolName:   "test_tool",
					Input:      json.RawMessage(`{"param": "value"}`),
					Output: []llmsdk.Part{
						{TextPart: &llmsdk.TextPart{Text: "Tool result"}},
					},
					IsError: false,
				},
			},
			// Final model item
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					{TextPart: &llmsdk.TextPart{Text: "Final response"}},
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
			{TextPart: &llmsdk.TextPart{Text: "Tool 1 result"}},
		},
		IsError: false,
	}

	tool2Result := llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			{TextPart: &llmsdk.TextPart{Text: "Tool 2 result"}},
		},
		IsError: false,
	}

	tool1 := NewMockTool[map[string]interface{}]("tool_1", tool1Result, nil)
	tool2 := NewMockTool[map[string]interface{}]("tool_2", tool2Result, nil)

	model := llmsdk.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdk.NewMockGenerateResultResponse(llmsdk.ModelResponse{
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
		llmsdk.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{TextPart: &llmsdk.TextPart{Text: "Processed both tools"}},
			},
			Usage: &llmsdk.ModelUsage{
				InputTokens:  50,
				OutputTokens: 10,
			},
			Cost: ptr.To(0.0003),
		}),
	)

	session := llmagent.NewRunSession(
		&llmagent.AgentParams[map[string]interface{}]{
			Name:           "test_agent",
			Model:          model,
			Instructions:   []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:          []llmagent.AgentTool[map[string]interface{}]{tool1, tool2},
			ResponseFormat: llmsdk.NewResponseFormatText(),
			MaxTurns:       10,
		},
	)

	response, err := session.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.Message{
				UserMessage: &llmsdk.UserMessage{
					Content: []llmsdk.Part{
						{TextPart: &llmsdk.TextPart{Text: "Use both tools"}},
					},
				},
			}),
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
			{ // tool result 1
				Tool: &llmagent.AgentItemTool{
					ToolCallID: "call_1",
					ToolName:   "tool_1",
					Input:      json.RawMessage(`{"param":"value1"}`),
					Output:     []llmsdk.Part{llmsdk.NewTextPart("Tool 1 result")},
					IsError:    false,
				},
			},
			{ // tool result 2
				Tool: &llmagent.AgentItemTool{
					ToolCallID: "call_2",
					ToolName:   "tool_2",
					Input:      json.RawMessage(`{"param":"value2"}`),
					Output:     []llmsdk.Part{llmsdk.NewTextPart("Tool 2 result")},
					IsError:    false,
				},
			},
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
			{TextPart: &llmsdk.TextPart{Text: "Calculation result"}},
		},
		IsError: false,
	}

	tool := NewMockTool[map[string]interface{}]("calculator", toolResult, nil)

	model := llmsdk.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdk.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{ToolCallPart: &llmsdk.ToolCallPart{
					ToolName:   "calculator",
					ToolCallID: "call_1",
					Args:       json.RawMessage([]byte(`{"operation": "add", "a": 1, "b": 2}`)),
				}},
			},
		}),
	)
	model.EnqueueGenerateResult(
		llmsdk.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{ToolCallPart: &llmsdk.ToolCallPart{
					ToolName:   "calculator",
					ToolCallID: "call_2",
					Args:       json.RawMessage([]byte(`{"operation": "multiply", "a": 3, "b": 4}`)),
				}},
			},
		}),
	)
	model.EnqueueGenerateResult(
		llmsdk.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{TextPart: &llmsdk.TextPart{Text: "All calculations done"}},
			},
		}),
	)

	session := llmagent.NewRunSession(
		&llmagent.AgentParams[map[string]interface{}]{
			Name:           "test_agent",
			Model:          model,
			Instructions:   []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:          []llmagent.AgentTool[map[string]interface{}]{tool},
			ResponseFormat: llmsdk.NewResponseFormatText(),
			MaxTurns:       10,
		},
	)

	response, err := session.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.Message{
				UserMessage: &llmsdk.UserMessage{
					Content: []llmsdk.Part{
						{TextPart: &llmsdk.TextPart{Text: "Calculate some numbers"}},
					},
				},
			}),
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
			{TextPart: &llmsdk.TextPart{Text: "All calculations done"}},
		},
		Output: []llmagent.AgentItem{
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					{ToolCallPart: &llmsdk.ToolCallPart{
						ToolName:   "calculator",
						ToolCallID: "call_1",
						Args:       json.RawMessage(`{"operation": "add", "a": 1, "b": 2}`),
					}},
				},
			}),
			{
				Tool: &llmagent.AgentItemTool{
					ToolCallID: "call_1",
					ToolName:   "calculator",
					Input:      json.RawMessage(`{"operation": "add", "a": 1, "b": 2}`),
					Output: []llmsdk.Part{
						{TextPart: &llmsdk.TextPart{Text: "Calculation result"}},
					},
					IsError: false,
				},
			},
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					{ToolCallPart: &llmsdk.ToolCallPart{
						ToolName:   "calculator",
						ToolCallID: "call_2",
						Args:       json.RawMessage(`{"operation": "multiply", "a": 3, "b": 4}`),
					}},
				},
			}),
			{
				Tool: &llmagent.AgentItemTool{
					ToolCallID: "call_2",
					ToolName:   "calculator",
					Input:      json.RawMessage(`{"operation": "multiply", "a": 3, "b": 4}`),
					Output: []llmsdk.Part{
						{TextPart: &llmsdk.TextPart{Text: "Calculation result"}},
					},
					IsError: false,
				},
			},
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					{TextPart: &llmsdk.TextPart{Text: "All calculations done"}},
				},
			}),
		},
	}

	if diff := cmp.Diff(expectedResponse, response); diff != "" {
		t.Errorf("response mismatch (-want +got):\n%s", diff)
	}
}

func TestRun_ThrowsAgentMaxTurnsExceededError(t *testing.T) {
	toolResult := llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			{TextPart: &llmsdk.TextPart{Text: "Tool result"}},
		},
		IsError: false,
	}

	tool := NewMockTool[map[string]interface{}]("test_tool", toolResult, nil)

	model := llmsdk.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdk.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{ToolCallPart: &llmsdk.ToolCallPart{
					ToolName:   "test_tool",
					ToolCallID: "call_1",
					Args:       json.RawMessage(`{}`),
				}},
			},
		}),
	)
	model.EnqueueGenerateResult(
		llmsdk.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{ToolCallPart: &llmsdk.ToolCallPart{
					ToolName:   "test_tool",
					ToolCallID: "call_2",
					Args:       json.RawMessage(`{}`),
				}},
			},
		}),
	)
	model.EnqueueGenerateResult(
		llmsdk.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{ToolCallPart: &llmsdk.ToolCallPart{
					ToolName:   "test_tool",
					ToolCallID: "call_3",
					Args:       json.RawMessage(`{}`),
				}},
			},
		}),
	)

	session := llmagent.NewRunSession(
		&llmagent.AgentParams[map[string]interface{}]{
			Name:           "test_agent",
			Model:          model,
			Instructions:   []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:          []llmagent.AgentTool[map[string]interface{}]{tool},
			ResponseFormat: &llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
			MaxTurns:       2,
		},
	)

	_, err := session.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.Message{
				UserMessage: &llmsdk.UserMessage{
					Content: []llmsdk.Part{
						{TextPart: &llmsdk.TextPart{Text: "Keep using tools"}},
					},
				},
			}),
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
	model := llmsdk.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdk.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{ToolCallPart: &llmsdk.ToolCallPart{
					ToolName:   "non_existent_tool",
					ToolCallID: "call_1",
					Args:       json.RawMessage(`{}`),
				}},
			},
		}),
	)

	session := llmagent.NewRunSession(
		&llmagent.AgentParams[map[string]interface{}]{
			Name:           "test_agent",
			Model:          model,
			Instructions:   []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:          []llmagent.AgentTool[map[string]interface{}]{},
			ResponseFormat: &llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
			MaxTurns:       10,
		},
	)

	_, err := session.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.Message{
				UserMessage: &llmsdk.UserMessage{
					Content: []llmsdk.Part{
						{TextPart: &llmsdk.TextPart{Text: "Use a tool"}},
					},
				},
			}),
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

	model := llmsdk.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdk.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{ToolCallPart: &llmsdk.ToolCallPart{
					ToolName:   "failing_tool",
					ToolCallID: "call_1",
					Args:       json.RawMessage(`{}`),
				}},
			},
		}),
	)

	session := llmagent.NewRunSession(
		&llmagent.AgentParams[map[string]interface{}]{
			Name:           "test_agent",
			Model:          model,
			Instructions:   []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:          []llmagent.AgentTool[map[string]interface{}]{tool},
			ResponseFormat: &llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
			MaxTurns:       10,
		},
	)

	_, err := session.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.Message{
				UserMessage: &llmsdk.UserMessage{
					Content: []llmsdk.Part{
						{TextPart: &llmsdk.TextPart{Text: "Use the tool"}},
					},
				},
			}),
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
			{TextPart: &llmsdk.TextPart{Text: "Error: Invalid parameters"}},
		},
		IsError: true,
	}

	tool := NewMockTool[map[string]interface{}]("test_tool", toolResult, nil)

	model := llmsdk.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdk.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{ToolCallPart: &llmsdk.ToolCallPart{
					ToolName:   "test_tool",
					ToolCallID: "call_1",
					Args:       json.RawMessage(`{"invalid": true}`),
				}},
			},
		}),
	)
	model.EnqueueGenerateResult(
		llmsdk.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{TextPart: &llmsdk.TextPart{Text: "Handled the error"}},
			},
		}),
	)

	session := llmagent.NewRunSession(
		&llmagent.AgentParams[map[string]interface{}]{
			Name:           "test_agent",
			Model:          model,
			Instructions:   []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:          []llmagent.AgentTool[map[string]interface{}]{tool},
			ResponseFormat: &llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
			MaxTurns:       10,
		},
	)

	response, err := session.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.Message{
				UserMessage: &llmsdk.UserMessage{
					Content: []llmsdk.Part{
						{TextPart: &llmsdk.TextPart{Text: "Use the tool"}},
					},
				},
			}),
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
			{TextPart: &llmsdk.TextPart{Text: "Handled the error"}},
		},
		Output: []llmagent.AgentItem{
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					{ToolCallPart: &llmsdk.ToolCallPart{
						ToolName:   "test_tool",
						ToolCallID: "call_1",
						Args:       json.RawMessage(`{"invalid": true}`),
					}},
				},
			}),
			{
				Tool: &llmagent.AgentItemTool{
					ToolCallID: "call_1",
					ToolName:   "test_tool",
					Input:      json.RawMessage(`{"invalid": true}`),
					Output: []llmsdk.Part{
						{TextPart: &llmsdk.TextPart{Text: "Error: Invalid parameters"}},
					},
					IsError: true,
				},
			},
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					{TextPart: &llmsdk.TextPart{Text: "Handled the error"}},
				},
			}),
		},
	}

	if diff := cmp.Diff(expectedResponse, response); diff != "" {
		t.Errorf("response mismatch (-want +got):\n%s", diff)
	}
}

func TestRun_PassesSamplingParametersToModel(t *testing.T) {
	model := llmsdk.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdk.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{TextPart: &llmsdk.TextPart{Text: "Response"}},
			},
		}),
	)

	temp := 0.7
	topP := 0.9
	topK := int32(40)
	presencePenalty := 0.1
	frequencyPenalty := 0.2

	session := llmagent.NewRunSession(
		&llmagent.AgentParams[map[string]interface{}]{
			Name:             "test_agent",
			Model:            model,
			Instructions:     []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:            []llmagent.AgentTool[map[string]interface{}]{},
			ResponseFormat:   &llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
			MaxTurns:         10,
			Temperature:      &temp,
			TopP:             &topP,
			TopK:             &topK,
			PresencePenalty:  &presencePenalty,
			FrequencyPenalty: &frequencyPenalty,
		},
	)

	_, err := session.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.Message{
				UserMessage: &llmsdk.UserMessage{
					Content: []llmsdk.Part{
						{TextPart: &llmsdk.TextPart{Text: "Hello"}},
					},
				},
			}),
		},
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(model.TrackedGenerateInputs) != 1 {
		t.Fatalf("expected 1 generate call, got %d", len(model.TrackedGenerateInputs))
	}

	call := *model.TrackedGenerateInputs[0]
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
	model := llmsdk.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdk.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{TextPart: &llmsdk.TextPart{Text: "Response"}},
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

	session := llmagent.NewRunSession(
		&llmagent.AgentParams[map[string]interface{}]{
			Name:           "test_agent",
			Model:          model,
			Instructions:   instructions,
			Tools:          []llmagent.AgentTool[map[string]interface{}]{},
			ResponseFormat: &llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
			MaxTurns:       10,
		},
	)

	_, err := session.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{"userRole": "developer"},
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.Message{
				UserMessage: &llmsdk.UserMessage{
					Content: []llmsdk.Part{
						{TextPart: &llmsdk.TextPart{Text: "Hello"}},
					},
				},
			}),
		},
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(model.TrackedGenerateInputs) != 1 {
		t.Fatalf("expected 1 generate call, got %d", len(model.TrackedGenerateInputs))
	}

	call := *model.TrackedGenerateInputs[0]
	expectedSystemPrompt := "You are a helpful assistant.\nThe user is a developer.\nAlways be polite."

	if call.SystemPrompt == nil || *call.SystemPrompt != expectedSystemPrompt {
		t.Errorf("expected system prompt %q, got %q", expectedSystemPrompt, *call.SystemPrompt)
	}
}

// -------- Root-level tests (RunStream) --------

func TestRunStream_StreamsResponse_NoToolCall(t *testing.T) {
	model := llmsdk.NewMockLanguageModel()
	model.EnqueueStreamResult(
		llmsdk.NewMockStreamResultPartials([]llmsdk.PartialModelResponse{
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.PartDelta{TextPartDelta: &llmsdk.TextPartDelta{Text: "Hel"}}}},
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.PartDelta{TextPartDelta: &llmsdk.TextPartDelta{Text: "lo"}}}},
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.PartDelta{TextPartDelta: &llmsdk.TextPartDelta{Text: "!"}}}},
		}),
	)

	session := llmagent.NewRunSession(
		&llmagent.AgentParams[map[string]interface{}]{
			Name:           "test_agent",
			Model:          model,
			Instructions:   []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:          []llmagent.AgentTool[map[string]interface{}]{},
			ResponseFormat: &llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
			MaxTurns:       10,
		},
	)

	stream, err := session.RunStream(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.Message{
				UserMessage: &llmsdk.UserMessage{
					Content: []llmsdk.Part{
						{TextPart: &llmsdk.TextPart{Text: "Hi"}},
					},
				},
			}),
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
					Part:  llmsdk.PartDelta{TextPartDelta: &llmsdk.TextPartDelta{Text: "Hel"}},
				},
			},
		},
		{
			Partial: &llmsdk.PartialModelResponse{
				Delta: &llmsdk.ContentDelta{
					Index: 0,
					Part:  llmsdk.PartDelta{TextPartDelta: &llmsdk.TextPartDelta{Text: "lo"}},
				},
			},
		},
		{
			Partial: &llmsdk.PartialModelResponse{
				Delta: &llmsdk.ContentDelta{
					Index: 0,
					Part:  llmsdk.PartDelta{TextPartDelta: &llmsdk.TextPartDelta{Text: "!"}},
				},
			},
		},
		{
			Item: func() *llmagent.AgentItem {
				item := llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
					Content: []llmsdk.Part{
						{TextPart: &llmsdk.TextPart{Text: "Hello!"}},
					},
				})
				return &item
			}(),
		},
		{
			Response: &llmagent.AgentResponse{
				Content: []llmsdk.Part{
					{TextPart: &llmsdk.TextPart{Text: "Hello!"}},
				},
				Output: []llmagent.AgentItem{
					llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
						Content: []llmsdk.Part{
							{TextPart: &llmsdk.TextPart{Text: "Hello!"}},
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
			{TextPart: &llmsdk.TextPart{Text: "Tool result"}},
		},
		IsError: false,
	}

	tool := NewMockTool[map[string]interface{}]("test_tool", toolResult, nil)

	toolName := "test_tool"
	callId := "call_1"
	args := `{"operation": "add", "a": 1, "b": 2}`

	model := llmsdk.NewMockLanguageModel()
	model.EnqueueStreamResult(
		llmsdk.NewMockStreamResultPartials([]llmsdk.PartialModelResponse{
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.PartDelta{ToolCallPartDelta: &llmsdk.ToolCallPartDelta{
				ToolName:   &toolName,
				ToolCallID: &callId,
				Args:       &args,
			}}}},
		}),
	)
	model.EnqueueStreamResult(
		llmsdk.NewMockStreamResultPartials([]llmsdk.PartialModelResponse{
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.PartDelta{TextPartDelta: &llmsdk.TextPartDelta{Text: "Final response"}}}},
		}),
	)

	session := llmagent.NewRunSession(
		&llmagent.AgentParams[map[string]interface{}]{
			Name:           "test_agent",
			Model:          model,
			Instructions:   []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:          []llmagent.AgentTool[map[string]interface{}]{tool},
			ResponseFormat: &llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
			MaxTurns:       10,
		},
	)

	stream, err := session.RunStream(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.Message{
				UserMessage: &llmsdk.UserMessage{
					Content: []llmsdk.Part{
						{TextPart: &llmsdk.TextPart{Text: "Use tool"}},
					},
				},
			}),
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
			if event.Item.Model != nil {
				if len(event.Item.Model.Content) > 0 {
					part := event.Item.Model.Content[0]
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
			} else if event.Item.Tool != nil {
				suffix := ":false"
				if event.Item.Tool.IsError {
					suffix = ":true"
				}
				summaries = append(summaries, "item:tool:"+event.Item.Tool.ToolName+suffix)
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
			{TextPart: &llmsdk.TextPart{Text: "Final response"}},
		},
		Output: []llmagent.AgentItem{
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					llmsdk.NewToolCallPart("call_1", "test_tool", map[string]any{"operation": "add", "a": 1, "b": 2}),
				},
			}),
			{
				Tool: &llmagent.AgentItemTool{
					ToolCallID: "call_1",
					ToolName:   "test_tool",
					Input:      json.RawMessage(`{"a":1,"b":2,"operation":"add"}`),
					Output: []llmsdk.Part{
						{TextPart: &llmsdk.TextPart{Text: "Tool result"}},
					},
					IsError: false,
				},
			},
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					{TextPart: &llmsdk.TextPart{Text: "Final response"}},
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
			{TextPart: &llmsdk.TextPart{Text: "Tool result"}},
		},
		IsError: false,
	}

	tool := NewMockTool[map[string]interface{}]("test_tool", toolResult, nil)

	toolName := "test_tool"
	callId1 := "call_1"
	callId2 := "call_2"
	callId3 := "call_3"
	args := "{}"

	model := llmsdk.NewMockLanguageModel()
	model.EnqueueStreamResult(
		llmsdk.NewMockStreamResultPartials([]llmsdk.PartialModelResponse{
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.PartDelta{ToolCallPartDelta: &llmsdk.ToolCallPartDelta{
				ToolName:   &toolName,
				ToolCallID: &callId1,
				Args:       &args,
			}}}},
		}),
	)
	model.EnqueueStreamResult(
		llmsdk.NewMockStreamResultPartials([]llmsdk.PartialModelResponse{
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.PartDelta{ToolCallPartDelta: &llmsdk.ToolCallPartDelta{
				ToolName:   &toolName,
				ToolCallID: &callId2,
				Args:       &args,
			}}}},
		}),
	)
	model.EnqueueStreamResult(
		llmsdk.NewMockStreamResultPartials([]llmsdk.PartialModelResponse{
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.PartDelta{ToolCallPartDelta: &llmsdk.ToolCallPartDelta{
				ToolName:   &toolName,
				ToolCallID: &callId3,
				Args:       &args,
			}}}},
		}),
	)

	session := llmagent.NewRunSession(
		&llmagent.AgentParams[map[string]interface{}]{
			Name:           "test_agent",
			Model:          model,
			Instructions:   []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:          []llmagent.AgentTool[map[string]interface{}]{tool},
			ResponseFormat: &llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
			MaxTurns:       2,
		},
	)

	stream, err := session.RunStream(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.Message{
				UserMessage: &llmsdk.UserMessage{
					Content: []llmsdk.Part{
						{TextPart: &llmsdk.TextPart{Text: "Keep using tools"}},
					},
				},
			}),
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

// -------- Root-level lifecycle test --------

func TestRun_FinishCleansUpSessionResources(t *testing.T) {
	model := llmsdk.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdk.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{TextPart: &llmsdk.TextPart{Text: "Response"}},
			},
		}),
	)

	session := llmagent.NewRunSession(
		&llmagent.AgentParams[map[string]interface{}]{
			Name:           "test_agent",
			Model:          model,
			Instructions:   []llmagent.InstructionParam[map[string]interface{}]{},
			Tools:          []llmagent.AgentTool[map[string]interface{}]{},
			ResponseFormat: &llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
			MaxTurns:       10,
		},
	)

	_, err := session.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.Message{
				UserMessage: &llmsdk.UserMessage{
					Content: []llmsdk.Part{
						{TextPart: &llmsdk.TextPart{Text: "Hello"}},
					},
				},
			}),
		},
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Call finish (currently no-op but tests the interface)
	session.Finish()

	// In this implementation, we don't prevent reuse after finish,
	// but the test shows the expected lifecycle
}
