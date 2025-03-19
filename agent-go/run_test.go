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

// MockLanguageModel implements llmsdk.LanguageModel for testing
type MockLanguageModel struct {
	responses        []*llmsdk.ModelResponse
	partialResponses [][]llmsdk.PartialModelResponse
	errors           []error
	streamErrors     []error
	generateCalls    []*llmsdk.LanguageModelInput
	streamCalls      []*llmsdk.LanguageModelInput
}

func NewMockLanguageModel() *MockLanguageModel {
	return &MockLanguageModel{
		responses:        []*llmsdk.ModelResponse{},
		partialResponses: [][]llmsdk.PartialModelResponse{},
		errors:           []error{},
		streamErrors:     []error{},
		generateCalls:    []*llmsdk.LanguageModelInput{},
		streamCalls:      []*llmsdk.LanguageModelInput{},
	}
}

func (m *MockLanguageModel) AddResponses(responses ...*llmsdk.ModelResponse) *MockLanguageModel {
	m.responses = append(m.responses, responses...)
	return m
}

func (m *MockLanguageModel) AddPartialResponses(partialResponses ...[]llmsdk.PartialModelResponse) *MockLanguageModel {
	m.partialResponses = append(m.partialResponses, partialResponses...)
	return m
}

func (m *MockLanguageModel) AddError(err error) *MockLanguageModel {
	m.errors = append(m.errors, err)
	return m
}

func (m *MockLanguageModel) AddStreamError(err error) *MockLanguageModel {
	m.streamErrors = append(m.streamErrors, err)
	return m
}

func (m *MockLanguageModel) Generate(ctx context.Context, input *llmsdk.LanguageModelInput) (*llmsdk.ModelResponse, error) {
	m.generateCalls = append(m.generateCalls, input)

	if len(m.errors) > 0 {
		err := m.errors[0]
		m.errors = m.errors[1:]
		return nil, err
	}

	if len(m.responses) == 0 {
		return nil, errors.New("no mock response")
	}

	response := m.responses[0]
	m.responses = m.responses[1:]
	return response, nil
}

func (m *MockLanguageModel) Stream(ctx context.Context, input *llmsdk.LanguageModelInput) (*llmsdk.LanguageModelStream, error) {
	m.streamCalls = append(m.streamCalls, input)

	if len(m.streamErrors) > 0 {
		err := m.streamErrors[0]
		m.streamErrors = m.streamErrors[1:]
		return nil, err
	}

	if len(m.partialResponses) == 0 {
		return nil, errors.New("no mock partial response")
	}

	partials := m.partialResponses[0]
	m.partialResponses = m.partialResponses[1:]
	return NewMockLanguageModelStream(partials), nil
}

func (m *MockLanguageModel) ModelID() string {
	return "mock-model"
}

func (m *MockLanguageModel) Provider() llmsdk.ProviderName {
	return "mock"
}

func (m *MockLanguageModel) Metadata() *llmsdk.LanguageModelMetadata {
	return &llmsdk.LanguageModelMetadata{}
}

func NewMockLanguageModelStream(partials []llmsdk.PartialModelResponse) *llmsdk.LanguageModelStream {
	eventChan := make(chan *llmsdk.PartialModelResponse)
	errChan := make(chan error)

	go func() {
		defer close(eventChan)
		defer close(errChan)

		for _, partial := range partials {
			eventChan <- &partial
		}
	}()

	return llmsdk.NewLanguageModelStream(eventChan, errChan)
}

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
	model := NewMockLanguageModel().AddResponses(&llmsdk.ModelResponse{
		Content: []llmsdk.Part{
			{TextPart: &llmsdk.TextPart{Text: "Hi!"}},
		},
	})

	session := llmagent.NewRunSession(
		"test_agent",
		model,
		[]llmagent.InstructionParam[map[string]interface{}]{},
		[]llmagent.AgentTool[map[string]interface{}]{},
		&llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
		10,
		nil, nil, nil, nil, nil,
	)

	response, err := session.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewMessageAgentItem(llmsdk.Message{
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
			llmagent.NewMessageAgentItem(llmsdk.Message{
				AssistantMessage: &llmsdk.AssistantMessage{
					Content: []llmsdk.Part{
						{TextPart: &llmsdk.TextPart{Text: "Hi!"}},
					},
				},
			}),
		},
		ModelCalls: []llmagent.ModelCallInfo{
			{
				Cost:     nil,
				Usage:    nil,
				ModelID:  model.ModelID(),
				Provider: model.Provider(),
			},
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

	model := NewMockLanguageModel().
		AddResponses(&llmsdk.ModelResponse{
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
		}).
		AddResponses(&llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{TextPart: &llmsdk.TextPart{Text: "Final response"}},
			},
		})

	session := llmagent.NewRunSession(
		"test_agent",
		model,
		[]llmagent.InstructionParam[map[string]interface{}]{},
		[]llmagent.AgentTool[map[string]interface{}]{tool},
		&llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
		10,
		nil, nil, nil, nil, nil,
	)

	response, err := session.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{"testContext": true},
		Input: []llmagent.AgentItem{
			llmagent.NewMessageAgentItem(llmsdk.Message{
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
			// Assistant tool call message
			llmagent.NewMessageAgentItem(llmsdk.Message{
				AssistantMessage: &llmsdk.AssistantMessage{
					Content: []llmsdk.Part{
						{ToolCallPart: &llmsdk.ToolCallPart{
							ToolName:   "test_tool",
							ToolCallID: "call_1",
							Args:       json.RawMessage(`{"param": "value"}`),
						}},
					},
				},
			}),
			// Tool result message
			llmagent.NewMessageAgentItem(llmsdk.Message{
				ToolMessage: &llmsdk.ToolMessage{
					Content: []llmsdk.Part{
						{ToolResultPart: &llmsdk.ToolResultPart{
							ToolCallID: "call_1",
							ToolName:   "test_tool",
							Content: []llmsdk.Part{
								{TextPart: &llmsdk.TextPart{Text: "Tool result"}},
							},
							IsError: false,
						}},
					},
				},
			}),
			// Final assistant response message
			llmagent.NewMessageAgentItem(llmsdk.Message{
				AssistantMessage: &llmsdk.AssistantMessage{
					Content: []llmsdk.Part{
						{TextPart: &llmsdk.TextPart{Text: "Final response"}},
					},
				},
			}),
		},
		ModelCalls: []llmagent.ModelCallInfo{
			{
				Usage: &llmsdk.ModelUsage{
					InputTokens:  1000,
					OutputTokens: 50,
				},
				Cost:     ptr.To(0.0015),
				ModelID:  model.ModelID(),
				Provider: model.Provider(),
			},
			{
				ModelID:  model.ModelID(),
				Provider: model.Provider(),
			},
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

	model := NewMockLanguageModel().
		AddResponses(&llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				llmsdk.NewToolCallPart("call_1", "tool_1", map[string]any{"param": "value1"}),
				llmsdk.NewToolCallPart("call_2", "tool_2", map[string]any{"param": "value2"}),
			},
			Usage: &llmsdk.ModelUsage{
				InputTokens:  2000,
				OutputTokens: 100,
			},
		}).
		AddResponses(&llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{TextPart: &llmsdk.TextPart{Text: "Processed both tools"}},
			},
			Usage: &llmsdk.ModelUsage{
				InputTokens:  50,
				OutputTokens: 10,
			},
			Cost: ptr.To(0.0003),
		})

	session := llmagent.NewRunSession(
		"test_agent",
		model,
		[]llmagent.InstructionParam[map[string]interface{}]{},
		[]llmagent.AgentTool[map[string]interface{}]{tool1, tool2},
		&llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
		10,
		nil, nil, nil, nil, nil,
	)

	response, err := session.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewMessageAgentItem(llmsdk.Message{
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
			llmagent.NewMessageAgentItem(
				llmsdk.NewAssistantMessage(
					llmsdk.NewToolCallPart("call_1", "tool_1", map[string]any{"param": "value1"}),
					llmsdk.NewToolCallPart("call_2", "tool_2", map[string]any{"param": "value2"}),
				),
			),
			llmagent.NewMessageAgentItem(
				llmsdk.NewToolMessage(
					llmsdk.NewToolResultPart("call_1", "tool_1", []llmsdk.Part{
						llmsdk.NewTextPart("Tool 1 result"),
					}, false),
					llmsdk.NewToolResultPart("call_2", "tool_2", []llmsdk.Part{
						llmsdk.NewTextPart("Tool 2 result"),
					}, false),
				),
			),
			llmagent.NewMessageAgentItem(
				llmsdk.NewAssistantMessage(
					llmsdk.NewTextPart("Processed both tools"),
				),
			),
		},
		ModelCalls: []llmagent.ModelCallInfo{
			{
				Usage: &llmsdk.ModelUsage{
					InputTokens:  2000,
					OutputTokens: 100,
				},
				ModelID:  model.ModelID(),
				Provider: model.Provider(),
			},
			{
				Usage: &llmsdk.ModelUsage{
					InputTokens:  50,
					OutputTokens: 10,
				},
				Cost:     ptr.To(0.0003),
				ModelID:  model.ModelID(),
				Provider: model.Provider(),
			},
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

	model := NewMockLanguageModel().
		AddResponses(&llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{ToolCallPart: &llmsdk.ToolCallPart{
					ToolName:   "calculator",
					ToolCallID: "call_1",
					Args:       json.RawMessage([]byte(`{"operation": "add", "a": 1, "b": 2}`)),
				}},
			},
		}).
		AddResponses(&llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{ToolCallPart: &llmsdk.ToolCallPart{
					ToolName:   "calculator",
					ToolCallID: "call_2",
					Args:       json.RawMessage([]byte(`{"operation": "multiply", "a": 3, "b": 4}`)),
				}},
			},
		}).
		AddResponses(&llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{TextPart: &llmsdk.TextPart{Text: "All calculations done"}},
			},
		})

	session := llmagent.NewRunSession(
		"test_agent",
		model,
		[]llmagent.InstructionParam[map[string]interface{}]{},
		[]llmagent.AgentTool[map[string]interface{}]{tool},
		&llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
		10,
		nil, nil, nil, nil, nil,
	)

	response, err := session.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewMessageAgentItem(llmsdk.Message{
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

	if response.Content[0].TextPart.Text != "All calculations done" {
		t.Errorf("expected all calculations done, got %q", response.Content[0].TextPart.Text)
	}

	if len(response.Output) != 5 {
		t.Errorf("expected 5 output messages, got %d", len(response.Output))
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

	model := NewMockLanguageModel().
		AddResponses(&llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{ToolCallPart: &llmsdk.ToolCallPart{
					ToolName:   "test_tool",
					ToolCallID: "call_1",
					Args:       json.RawMessage(`{}`),
				}},
			},
		}).
		AddResponses(&llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{ToolCallPart: &llmsdk.ToolCallPart{
					ToolName:   "test_tool",
					ToolCallID: "call_2",
					Args:       json.RawMessage(`{}`),
				}},
			},
		}).
		AddResponses(&llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{ToolCallPart: &llmsdk.ToolCallPart{
					ToolName:   "test_tool",
					ToolCallID: "call_3",
					Args:       json.RawMessage(`{}`),
				}},
			},
		})

	session := llmagent.NewRunSession(
		"test_agent",
		model,
		[]llmagent.InstructionParam[map[string]interface{}]{},
		[]llmagent.AgentTool[map[string]interface{}]{tool},
		&llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
		2, // max turns is 2
		nil, nil, nil, nil, nil,
	)

	_, err := session.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewMessageAgentItem(llmsdk.Message{
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
	model := NewMockLanguageModel().AddResponses(&llmsdk.ModelResponse{
		Content: []llmsdk.Part{
			{ToolCallPart: &llmsdk.ToolCallPart{
				ToolName:   "non_existent_tool",
				ToolCallID: "call_1",
				Args:       json.RawMessage(`{}`),
			}},
		},
	})

	session := llmagent.NewRunSession(
		"test_agent",
		model,
		[]llmagent.InstructionParam[map[string]interface{}]{},
		[]llmagent.AgentTool[map[string]interface{}]{},
		&llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
		10,
		nil, nil, nil, nil, nil,
	)

	_, err := session.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewMessageAgentItem(llmsdk.Message{
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

	model := NewMockLanguageModel().AddResponses(&llmsdk.ModelResponse{
		Content: []llmsdk.Part{
			{ToolCallPart: &llmsdk.ToolCallPart{
				ToolName:   "failing_tool",
				ToolCallID: "call_1",
				Args:       json.RawMessage(`{}`),
			}},
		},
	})

	session := llmagent.NewRunSession(
		"test_agent",
		model,
		[]llmagent.InstructionParam[map[string]interface{}]{},
		[]llmagent.AgentTool[map[string]interface{}]{tool},
		&llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
		10,
		nil, nil, nil, nil, nil,
	)

	_, err := session.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewMessageAgentItem(llmsdk.Message{
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

	model := NewMockLanguageModel().
		AddResponses(&llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{ToolCallPart: &llmsdk.ToolCallPart{
					ToolName:   "test_tool",
					ToolCallID: "call_1",
					Args:       json.RawMessage(`{"invalid": true}`),
				}},
			},
		}).
		AddResponses(&llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				{TextPart: &llmsdk.TextPart{Text: "Handled the error"}},
			},
		})

	session := llmagent.NewRunSession(
		"test_agent",
		model,
		[]llmagent.InstructionParam[map[string]interface{}]{},
		[]llmagent.AgentTool[map[string]interface{}]{tool},
		&llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
		10,
		nil, nil, nil, nil, nil,
	)

	response, err := session.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewMessageAgentItem(llmsdk.Message{
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

	// Check that the tool result was marked as an error
	if len(response.Output) < 2 {
		t.Fatalf("expected at least 2 output messages, got %d", len(response.Output))
	}

	toolMessage := response.Output[1]
	if toolMessage.Message == nil || toolMessage.Message.ToolMessage == nil {
		t.Fatal("expected tool message")
	}

	if len(toolMessage.Message.ToolMessage.Content) == 0 {
		t.Fatal("expected tool message content")
	}

	toolResultPart := toolMessage.Message.ToolMessage.Content[0].ToolResultPart
	if toolResultPart == nil {
		t.Fatal("expected tool result part")
	}

	if !toolResultPart.IsError {
		t.Error("expected tool result to be marked as error")
	}

	if response.Content[0].TextPart.Text != "Handled the error" {
		t.Errorf("expected handled the error, got %q", response.Content[0].TextPart.Text)
	}
}

func TestRun_PassesSamplingParametersToModel(t *testing.T) {
	model := NewMockLanguageModel().AddResponses(&llmsdk.ModelResponse{
		Content: []llmsdk.Part{
			{TextPart: &llmsdk.TextPart{Text: "Response"}},
		},
	})

	temp := 0.7
	topP := 0.9
	topK := 40.0
	presencePenalty := 0.1
	frequencyPenalty := 0.2

	session := llmagent.NewRunSession(
		"test_agent",
		model,
		[]llmagent.InstructionParam[map[string]interface{}]{},
		[]llmagent.AgentTool[map[string]interface{}]{},
		&llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
		10,
		&temp, &topP, &topK, &presencePenalty, &frequencyPenalty,
	)

	_, err := session.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewMessageAgentItem(llmsdk.Message{
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

	if len(model.generateCalls) != 1 {
		t.Fatalf("expected 1 generate call, got %d", len(model.generateCalls))
	}

	call := *model.generateCalls[0]
	if call.Temperature == nil || *call.Temperature != temp {
		t.Errorf("expected temperature %f, got %v", temp, call.Temperature)
	}
	if call.TopP == nil || *call.TopP != topP {
		t.Errorf("expected topP %f, got %v", topP, call.TopP)
	}
	if call.TopK == nil || *call.TopK != topK {
		t.Errorf("expected topK %f, got %v", topK, call.TopK)
	}
	if call.PresencePenalty == nil || *call.PresencePenalty != presencePenalty {
		t.Errorf("expected presencePenalty %f, got %v", presencePenalty, call.PresencePenalty)
	}
	if call.FrequencyPenalty == nil || *call.FrequencyPenalty != frequencyPenalty {
		t.Errorf("expected frequencyPenalty %f, got %v", frequencyPenalty, call.FrequencyPenalty)
	}
}

func TestRun_IncludesStringAndDynamicFunctionInstructionsInSystemPrompt(t *testing.T) {
	model := NewMockLanguageModel().AddResponses(&llmsdk.ModelResponse{
		Content: []llmsdk.Part{
			{TextPart: &llmsdk.TextPart{Text: "Response"}},
		},
	})

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
		"test_agent",
		model,
		instructions,
		[]llmagent.AgentTool[map[string]interface{}]{},
		&llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
		10,
		nil, nil, nil, nil, nil,
	)

	_, err := session.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{"userRole": "developer"},
		Input: []llmagent.AgentItem{
			llmagent.NewMessageAgentItem(llmsdk.Message{
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

	if len(model.generateCalls) != 1 {
		t.Fatalf("expected 1 generate call, got %d", len(model.generateCalls))
	}

	call := *model.generateCalls[0]
	expectedSystemPrompt := "You are a helpful assistant.\nThe user is a developer.\nAlways be polite."

	if call.SystemPrompt == nil || *call.SystemPrompt != expectedSystemPrompt {
		t.Errorf("expected system prompt %q, got %q", expectedSystemPrompt, *call.SystemPrompt)
	}
}

// -------- Root-level tests (RunStream) --------

func TestRunStream_StreamsResponse_NoToolCall(t *testing.T) {
	model := NewMockLanguageModel().AddPartialResponses([]llmsdk.PartialModelResponse{
		{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.PartDelta{TextPartDelta: &llmsdk.TextPartDelta{Text: "Hel"}}}},
		{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.PartDelta{TextPartDelta: &llmsdk.TextPartDelta{Text: "lo"}}}},
		{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.PartDelta{TextPartDelta: &llmsdk.TextPartDelta{Text: "!"}}}},
	})

	session := llmagent.NewRunSession(
		"test_agent",
		model,
		[]llmagent.InstructionParam[map[string]interface{}]{},
		[]llmagent.AgentTool[map[string]interface{}]{},
		&llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
		10,
		nil, nil, nil, nil, nil,
	)

	stream, err := session.RunStream(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewMessageAgentItem(llmsdk.Message{
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

	// Should have partial events, a message event, and a response event
	if len(events) < 4 {
		t.Errorf("expected at least 4 events, got %d", len(events))
	}

	// Check that we got partial events
	partialCount := 0
	for _, event := range events {
		if event.Partial != nil {
			partialCount++
		}
	}

	if partialCount != 3 {
		t.Errorf("expected 3 partial events, got %d", partialCount)
	}

	// Check final response
	finalResponse := events[len(events)-1]
	if finalResponse.Response == nil {
		t.Error("expected final event to be a response")
	} else if len(finalResponse.Response.Content) == 0 || finalResponse.Response.Content[0].TextPart.Text != "Hello!" {
		t.Errorf("expected final response to be 'Hello!', got %v", finalResponse.Response.Content)
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

	model := NewMockLanguageModel().
		AddPartialResponses([]llmsdk.PartialModelResponse{
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.PartDelta{ToolCallPartDelta: &llmsdk.ToolCallPartDelta{
				ToolName:   &toolName,
				ToolCallID: &callId,
				Args:       &args,
			}}}},
		}).
		AddPartialResponses([]llmsdk.PartialModelResponse{
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.PartDelta{TextPartDelta: &llmsdk.TextPartDelta{Text: "Final response"}}}},
		})

	session := llmagent.NewRunSession(
		"test_agent",
		model,
		[]llmagent.InstructionParam[map[string]interface{}]{},
		[]llmagent.AgentTool[map[string]interface{}]{tool},
		&llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
		10,
		nil, nil, nil, nil, nil,
	)

	stream, err := session.RunStream(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewMessageAgentItem(llmsdk.Message{
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

	// Count event types
	partialCount := 0
	messageCount := 0
	responseCount := 0

	for _, event := range events {
		if event.Partial != nil {
			partialCount++
		} else if event.Message != nil {
			messageCount++
		} else if event.Response != nil {
			responseCount++
		}
	}

	if partialCount < 2 {
		t.Errorf("expected at least 2 partial events, got %d", partialCount)
	}
	if messageCount != 3 {
		t.Errorf("expected 3 message events, got %d", messageCount)
	}
	if responseCount != 1 {
		t.Errorf("expected 1 response event, got %d", responseCount)
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

	model := NewMockLanguageModel().
		AddPartialResponses([]llmsdk.PartialModelResponse{
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.PartDelta{ToolCallPartDelta: &llmsdk.ToolCallPartDelta{
				ToolName:   &toolName,
				ToolCallID: &callId1,
				Args:       &args,
			}}}},
		}).
		AddPartialResponses([]llmsdk.PartialModelResponse{
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.PartDelta{ToolCallPartDelta: &llmsdk.ToolCallPartDelta{
				ToolName:   &toolName,
				ToolCallID: &callId2,
				Args:       &args,
			}}}},
		}).
		AddPartialResponses([]llmsdk.PartialModelResponse{
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.PartDelta{ToolCallPartDelta: &llmsdk.ToolCallPartDelta{
				ToolName:   &toolName,
				ToolCallID: &callId3,
				Args:       &args,
			}}}},
		})

	session := llmagent.NewRunSession(
		"test_agent",
		model,
		[]llmagent.InstructionParam[map[string]interface{}]{},
		[]llmagent.AgentTool[map[string]interface{}]{tool},
		&llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
		2, // max turns is 2
		nil, nil, nil, nil, nil,
	)

	stream, err := session.RunStream(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewMessageAgentItem(llmsdk.Message{
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
	model := NewMockLanguageModel().AddResponses(&llmsdk.ModelResponse{
		Content: []llmsdk.Part{
			{TextPart: &llmsdk.TextPart{Text: "Response"}},
		},
	})

	session := llmagent.NewRunSession(
		"test_agent",
		model,
		[]llmagent.InstructionParam[map[string]interface{}]{},
		[]llmagent.AgentTool[map[string]interface{}]{},
		&llmsdk.ResponseFormatOption{Text: &llmsdk.ResponseFormatText{}},
		10,
		nil, nil, nil, nil, nil,
	)

	_, err := session.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
		Context: map[string]interface{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewMessageAgentItem(llmsdk.Message{
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
