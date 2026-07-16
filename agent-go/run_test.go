package llmagent_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/llmsdktest"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"
	sdkstream "github.com/hoangvvo/llm-sdk/sdk-go/utils/stream"
)

func mixedSnapshotPartials() []llmsdk.PartialModelResponse {
	return []llmsdk.PartialModelResponse{
		{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta("partial text")}},
		{Delta: &llmsdk.ContentDelta{Index: 1, Part: llmsdk.NewToolCallPartDelta(
			llmsdk.WithToolCallPartDeltaToolCallID("call_1"),
			llmsdk.WithToolCallPartDeltaToolName("weather"),
			llmsdk.WithToolCallPartDeltaArgs(`{"city":"Paris"}`),
		)}},
		{Delta: &llmsdk.ContentDelta{Index: 2, Part: llmsdk.NewToolCallPartDelta(
			llmsdk.WithToolCallPartDeltaArgs("{incomplete"),
		)}},
	}
}

func mixedSnapshotModelResponse() llmsdk.ModelResponse {
	return llmsdk.ModelResponse{Content: []llmsdk.Part{
		llmsdk.NewTextPart("partial text"),
		llmsdk.NewToolCallPart("call_1", "weather", map[string]any{"city": "Paris"}),
	}}
}

type cancellationLanguageModel struct {
	partials []llmsdk.PartialModelResponse
}

func (m *cancellationLanguageModel) Provider() string { return "cancellation-test" }
func (m *cancellationLanguageModel) ModelID() string  { return "cancellation-test" }
func (m *cancellationLanguageModel) Metadata() *llmsdk.LanguageModelMetadata {
	return nil
}
func (m *cancellationLanguageModel) Generate(context.Context, *llmsdk.LanguageModelInput) (*llmsdk.ModelResponse, error) {
	return nil, errors.New("generate is not supported")
}
func (m *cancellationLanguageModel) Stream(ctx context.Context, _ *llmsdk.LanguageModelInput) (*llmsdk.LanguageModelStream, error) {
	eventChan := make(chan *llmsdk.PartialModelResponse)
	errChan := make(chan error)
	go func() {
		defer close(eventChan)
		defer close(errChan)
		for _, partial := range m.partials {
			value := partial
			select {
			case eventChan <- &value:
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			}
		}
		<-ctx.Done()
		errChan <- ctx.Err()
	}()
	return sdkstream.New(eventChan, errChan), nil
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
	closeFn           func(context.Context) error
}

func (m *mockToolkitSession[C]) SystemPrompt() *string {
	m.systemPromptCalls++
	return m.systemPrompt
}

func (m *mockToolkitSession[C]) Tools() []llmagent.AgentTool[C] {
	m.toolsCalls++
	return m.tools
}

func (m *mockToolkitSession[C]) Close(ctx context.Context) error {
	m.closeCalls++
	if m.closeFn != nil {
		return m.closeFn(ctx)
	}
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

func TestRun_RejectsEmptyInputWithoutCallingModel(t *testing.T) {
	model := llmsdktest.NewMockLanguageModel()
	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[struct{}]{Name: "test_agent", Model: model, MaxTurns: 10},
		struct{}{},
	)

	_, err := session.Run(t.Context(), llmagent.RunSessionRequest{})
	var agentErr *llmagent.AgentError
	if !errors.As(err, &agentErr) || agentErr.Kind != llmagent.InvariantErrorKind {
		t.Fatalf("expected invariant error, got %v", err)
	}
	if len(model.TrackedGenerateInputs()) != 0 {
		t.Fatal("model should not be called for empty input")
	}
}

func TestRun_ReturnsCancelledWithoutCallingModelWhenContextAlreadyCancelled(t *testing.T) {
	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueGenerateResult(llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
		Content: []llmsdk.Part{llmsdk.NewTextPart("ignored")},
	}))
	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[struct{}]{Name: "test_agent", Model: model, MaxTurns: 10},
		struct{}{},
	)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	response, err := session.Run(ctx, llmagent.RunSessionRequest{Input: []llmagent.AgentItem{
		llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Hello"))),
	}})
	if err != nil {
		t.Fatalf("expected cancelled response, got %v", err)
	}
	if response.Status != llmagent.AgentResponseStatusCancelled || len(response.Content) != 0 || len(response.Output) != 0 {
		t.Fatalf("unexpected cancelled response: %#v", response)
	}
	if len(model.TrackedGenerateInputs()) != 0 {
		t.Fatal("model should not be called after cancellation")
	}
}

func TestRun_RejectsDuplicateToolCallIDsBeforeExecution(t *testing.T) {
	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueGenerateResult(llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
		Content: []llmsdk.Part{
			llmsdk.NewToolCallPart("duplicate", "first", map[string]any{}),
			llmsdk.NewToolCallPart("duplicate", "second", map[string]any{}),
		},
	}))
	first := NewMockTool[struct{}]("first", llmagent.AgentToolResult{}, nil)
	second := NewMockTool[struct{}]("second", llmagent.AgentToolResult{}, nil)
	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[struct{}]{
			Name:     "test_agent",
			Model:    model,
			Tools:    llmagent.FunctionTools[struct{}](first, second),
			MaxTurns: 10,
		},
		struct{}{},
	)

	_, err := session.Run(t.Context(), llmagent.RunSessionRequest{Input: []llmagent.AgentItem{
		llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Use tools"))),
	}})
	if err == nil || !strings.Contains(err.Error(), "duplicate tool call ID: duplicate") {
		t.Fatalf("expected duplicate-ID error, got %v", err)
	}
	if len(first.AllCalls) != 0 || len(second.AllCalls) != 0 {
		t.Fatal("tools must not execute after duplicate IDs are detected")
	}
}

func TestRun_PassesCurrentTurnAndAccumulatedItemsToTool(t *testing.T) {
	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueGenerateResult(
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{Content: []llmsdk.Part{
			llmsdk.NewToolCallPart("call_1", "inspect_state", map[string]any{}),
		}}),
		llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{Content: []llmsdk.Part{
			llmsdk.NewTextPart("done"),
		}}),
	)
	type observation struct {
		turn      uint
		itemTypes []llmagent.AgentItemType
	}
	observed := make(chan observation, 1)
	tool := NewMockTool[struct{}]("inspect_state", llmagent.AgentToolResult{
		Content: []llmsdk.Part{llmsdk.NewTextPart("inspected")},
	}, func(_ context.Context, _ json.RawMessage, _ struct{}, state *llmagent.RunState) (llmagent.AgentToolResult, error) {
		items := state.Items()
		types := make([]llmagent.AgentItemType, 0, len(items))
		for _, item := range items {
			types = append(types, item.Type())
		}
		observed <- observation{turn: state.CurrentTurn, itemTypes: types}
		return llmagent.AgentToolResult{Content: []llmsdk.Part{llmsdk.NewTextPart("inspected")}}, nil
	})
	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[struct{}]{
			Name: "test_agent", Model: model, Tools: llmagent.FunctionTools[struct{}](tool), MaxTurns: 10,
		},
		struct{}{},
	)

	_, err := session.Run(t.Context(), llmagent.RunSessionRequest{Input: []llmagent.AgentItem{
		llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Inspect"))),
	}})
	if err != nil {
		t.Fatalf("run session: %v", err)
	}
	got := <-observed
	if got.turn != 1 || len(got.itemTypes) != 2 || got.itemTypes[0] != llmagent.AgentItemTypeMessage || got.itemTypes[1] != llmagent.AgentItemTypeModel {
		t.Fatalf("unexpected run-state observation: %#v", got)
	}
}

func TestRunStream_InvalidDeltaSequenceReturnsInvariantError(t *testing.T) {
	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueStreamResult(llmsdktest.NewMockStreamResultPartials([]llmsdk.PartialModelResponse{
		{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta("hello")}},
		{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewReasoningPartDelta("wrong type")}},
	}))
	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[struct{}]{Name: "test_agent", Model: model, MaxTurns: 10},
		struct{}{},
	)
	stream, err := session.RunStream(t.Context(), llmagent.RunSessionRequest{Input: []llmagent.AgentItem{
		llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Stream"))),
	}})
	if err != nil {
		t.Fatalf("create stream: %v", err)
	}
	for stream.Next() {
	}
	var agentErr *llmagent.AgentError
	if !errors.As(stream.Err(), &agentErr) || agentErr.Kind != llmagent.InvariantErrorKind {
		t.Fatalf("expected invariant stream error, got %v", stream.Err())
	}
}

func TestRunSession_ClosesInitializedToolkitsWhenCreationFails(t *testing.T) {
	initialized := &mockToolkitSession[struct{}]{}
	initFailure := errors.New("second toolkit failed")
	model := llmsdktest.NewMockLanguageModel()
	_, err := llmagent.NewRunSession(
		t.Context(),
		&llmagent.AgentParams[struct{}]{
			Name:  "test_agent",
			Model: model,
			Toolkits: []llmagent.Toolkit[struct{}]{
				&mockToolkit[struct{}]{createFn: func(context.Context, struct{}) (llmagent.ToolkitSession[struct{}], error) {
					return initialized, nil
				}},
				&mockToolkit[struct{}]{createFn: func(context.Context, struct{}) (llmagent.ToolkitSession[struct{}], error) {
					return nil, initFailure
				}},
			},
			MaxTurns: 10,
		},
		struct{}{},
	)
	if !errors.Is(err, initFailure) {
		t.Fatalf("expected initialization failure, got %v", err)
	}
	if initialized.closeCalls != 1 {
		t.Fatalf("expected initialized toolkit cleanup, got %d", initialized.closeCalls)
	}
}

func TestRunSession_CloseAttemptsEveryToolkitAndReportsFailure(t *testing.T) {
	cleanupFailure := errors.New("cleanup failed")
	failing := &mockToolkitSession[struct{}]{closeErr: cleanupFailure}
	successful := &mockToolkitSession[struct{}]{}
	model := llmsdktest.NewMockLanguageModel()
	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[struct{}]{
			Name:  "test_agent",
			Model: model,
			Toolkits: []llmagent.Toolkit[struct{}]{
				&mockToolkit[struct{}]{createFn: func(context.Context, struct{}) (llmagent.ToolkitSession[struct{}], error) {
					return failing, nil
				}},
				&mockToolkit[struct{}]{createFn: func(context.Context, struct{}) (llmagent.ToolkitSession[struct{}], error) {
					return successful, nil
				}},
			},
			MaxTurns: 10,
		},
		struct{}{},
	)

	err := session.Close(t.Context())
	if !errors.Is(err, cleanupFailure) {
		t.Fatalf("expected cleanup failure, got %v", err)
	}
	if failing.closeCalls != 1 || successful.closeCalls != 1 {
		t.Fatalf("expected every toolkit to close: failing=%d successful=%d", failing.closeCalls, successful.closeCalls)
	}
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
		Status: llmagent.AgentResponseStatusCompleted,
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
			Tools:          llmagent.FunctionTools(tool),
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
		Status: llmagent.AgentResponseStatusCompleted,
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
				llmsdk.ToolResultStatusCompleted,
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
			Tools:          llmagent.FunctionTools(tool1, tool2),
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
		Status:  llmagent.AgentResponseStatusCompleted,
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
				llmsdk.ToolResultStatusCompleted,
			),
			llmagent.NewAgentItemTool(
				"call_2",
				"tool_2",
				json.RawMessage(`{"param":"value2"}`),
				[]llmsdk.Part{llmsdk.NewTextPart("Tool 2 result")},
				llmsdk.ToolResultStatusCompleted,
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
			Tools:          llmagent.FunctionTools(tool),
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
		Status: llmagent.AgentResponseStatusCompleted,
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
				llmsdk.ToolResultStatusCompleted,
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
				llmsdk.ToolResultStatusCompleted,
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
		Status:  llmagent.AgentResponseStatusCompleted,
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
			Tools:          llmagent.FunctionTools(tool),
			ResponseFormat: llmsdk.NewResponseFormatText(),
			MaxTurns:       10,
		},
		map[string]interface{}{},
	)

	call1Args := json.RawMessage(`{"step": 1}`)
	call2Args := json.RawMessage(`{"step": 2}`)
	toolResult := llmsdk.NewToolResultPart("call_1", "resume_tool", []llmsdk.Part{llmsdk.NewTextPart("already done")})

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
		Status:  llmagent.AgentResponseStatusCompleted,
		Content: []llmsdk.Part{llmsdk.NewTextPart("Final reply")},
		Output: []llmagent.AgentItem{
			llmagent.NewAgentItemTool("call_2", "resume_tool", call2Args, []llmsdk.Part{llmsdk.NewTextPart("call_2 result")}, llmsdk.ToolResultStatusCompleted),
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
			Tools:          llmagent.FunctionTools(tool),
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
			llmagent.NewAgentItemTool("call_1", "resume_tool", call1Args, []llmsdk.Part{llmsdk.NewTextPart("already done")}, llmsdk.ToolResultStatusCompleted),
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(tool.AllCalls) != 1 || string(tool.AllCalls[0]) != string(call2Args) {
		t.Fatalf("unexpected tool calls: %v", tool.AllCalls)
	}

	expected := &llmagent.AgentResponse{
		Status:  llmagent.AgentResponseStatusCompleted,
		Content: []llmsdk.Part{llmsdk.NewTextPart("Final reply from items")},
		Output: []llmagent.AgentItem{
			llmagent.NewAgentItemTool("call_2", "resume_tool", call2Args, []llmsdk.Part{llmsdk.NewTextPart("call_2 via item")}, llmsdk.ToolResultStatusCompleted),
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
			Tools:          llmagent.FunctionTools(tool),
			ResponseFormat: llmsdk.NewResponseFormatText(),
			MaxTurns:       10,
		},
		map[string]interface{}{},
	)

	_, err := session.Run(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Resume"))),
			llmagent.NewAgentItemMessage(llmsdk.NewToolMessage(llmsdk.NewToolResultPart(
				"call_1",
				"resume_tool",
				[]llmsdk.Part{llmsdk.NewTextPart("orphan")},
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
			Tools:        llmagent.FunctionTools(tool),
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
	if agentErr.Snapshot == nil {
		t.Fatal("expected a run snapshot")
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
			Tools:        llmagent.FunctionTools(tool),
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
			Tools:        llmagent.FunctionTools(tool),
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
		Status: llmagent.AgentResponseStatusCompleted,
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
				llmsdk.ToolResultStatusFailed,
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

func TestRun_PassesProviderHostedToolsToModel(t *testing.T) {
	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueGenerateResult(llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
		Content: []llmsdk.Part{llmsdk.NewTextPart("Search complete")},
	}))
	webSearchTool := llmsdk.WebSearchTool{AllowedDomains: []string{"example.com"}}
	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[struct{}]{
			Name:     "test_agent",
			Model:    model,
			Tools:    []llmagent.AgentTool[struct{}]{llmagent.NewAgentWebSearchTool[struct{}](webSearchTool)},
			MaxTurns: 10,
		},
		struct{}{},
	)

	_, err := session.Run(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Find an example"))),
		},
	})
	if err != nil {
		t.Fatalf("run session: %v", err)
	}

	inputs := model.TrackedGenerateInputs()
	if len(inputs) != 1 {
		t.Fatalf("expected one model call, got %d", len(inputs))
	}
	expected := []llmsdk.Tool{{WebSearchTool: &webSearchTool}}
	if diff := cmp.Diff(expected, inputs[0].Tools); diff != "" {
		t.Fatalf("hosted tools mismatch (-want +got):\n%s", diff)
	}
}

func TestRun_ReturnsLanguageModelErrorWhenGenerationFails(t *testing.T) {
	model := llmsdktest.NewMockLanguageModel()
	modelErr := llmsdk.NewInvalidInputError("API quota exceeded")
	model.EnqueueGenerateResult(llmsdktest.NewMockGenerateResultError(modelErr))
	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[struct{}]{Name: "test_agent", Model: model, MaxTurns: 10},
		struct{}{},
	)

	_, err := session.Run(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Hello"))),
		},
	})
	if !errors.Is(err, modelErr) {
		t.Fatalf("expected wrapped model error, got %v", err)
	}
	var agentErr *llmagent.AgentError
	if !errors.As(err, &agentErr) || agentErr.Kind != llmagent.LanguageModelErrorKind {
		t.Fatalf("expected language model agent error, got %v", err)
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

	dynamicTool := NewMockTool(
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
		tools:        llmagent.FunctionTools(dynamicTool),
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
		if input.Tools[0].FunctionTool == nil || input.Tools[0].FunctionTool.Name != "lookup-order" {
			t.Fatalf("unexpected tools: %v", input.Tools)
		}
	}

	expectedResponse := &llmagent.AgentResponse{
		Status:  llmagent.AgentResponseStatusCompleted,
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
				llmsdk.ToolResultStatusCompleted,
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

func TestRunStream_ReturnsCancelledWithoutCallingModelWhenContextAlreadyCancelled(t *testing.T) {
	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueStreamResult(llmsdktest.NewMockStreamResultPartials([]llmsdk.PartialModelResponse{
		{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta("ignored")}},
	}))
	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[struct{}]{Name: "test_agent", Model: model, MaxTurns: 10},
		struct{}{},
	)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	stream, err := session.RunStream(ctx, llmagent.RunSessionRequest{Input: []llmagent.AgentItem{
		llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Hello"))),
	}})
	if err != nil {
		t.Fatalf("create stream: %v", err)
	}
	var events []*llmagent.AgentStreamEvent
	for stream.Next() {
		events = append(events, stream.Current())
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("expected cancelled response, got %v", err)
	}
	if len(events) != 1 || events[0].Response == nil || events[0].Response.Status != llmagent.AgentResponseStatusCancelled {
		t.Fatalf("unexpected cancellation events: %#v", events)
	}
	if len(model.TrackedStreamInputs()) != 0 {
		t.Fatal("model should not be called after cancellation")
	}
}

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
				Status: llmagent.AgentResponseStatusCompleted,
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
			Tools:        llmagent.FunctionTools(tool),
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
				if itemEvent.Item.Tool.Status != llmsdk.ToolResultStatusCompleted {
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
		Status: llmagent.AgentResponseStatusCompleted,
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
				llmsdk.ToolResultStatusCompleted,
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

func TestRunStream_HandlesMultipleTurns(t *testing.T) {
	tool := NewMockTool[struct{}]("calculator", llmagent.AgentToolResult{
		Content: []llmsdk.Part{llmsdk.NewTextPart("Calculation done")},
	}, nil)
	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueStreamResult(
		llmsdktest.NewMockStreamResultPartials([]llmsdk.PartialModelResponse{{
			Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewToolCallPartDelta(
				llmsdk.WithToolCallPartDeltaToolName("calculator"),
				llmsdk.WithToolCallPartDeltaToolCallID("call_1"),
				llmsdk.WithToolCallPartDeltaArgs(`{"a":1,"b":2}`),
			)},
		}}),
		llmsdktest.NewMockStreamResultPartials([]llmsdk.PartialModelResponse{{
			Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewToolCallPartDelta(
				llmsdk.WithToolCallPartDeltaToolName("calculator"),
				llmsdk.WithToolCallPartDeltaToolCallID("call_2"),
				llmsdk.WithToolCallPartDeltaArgs(`{"a":3,"b":4}`),
			)},
		}}),
		llmsdktest.NewMockStreamResultPartials([]llmsdk.PartialModelResponse{{
			Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta("All done")},
		}}),
	)
	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[struct{}]{
			Name:     "test_agent",
			Model:    model,
			Tools:    llmagent.FunctionTools[struct{}](tool),
			MaxTurns: 10,
		},
		struct{}{},
	)

	stream, err := session.RunStream(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Calculate"))),
		},
	})
	if err != nil {
		t.Fatalf("create stream: %v", err)
	}

	var events []*llmagent.AgentStreamEvent
	for stream.Next() {
		events = append(events, stream.Current())
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("consume stream: %v", err)
	}

	eventTypes := make([]llmagent.AgentStreamEventType, 0, len(events))
	for _, event := range events {
		switch {
		case event.Partial != nil:
			eventTypes = append(eventTypes, llmagent.AgentStreamEventTypePartial)
		case event.Item != nil:
			eventTypes = append(eventTypes, llmagent.AgentStreamEventTypeItem)
		case event.Response != nil:
			eventTypes = append(eventTypes, llmagent.AgentStreamEventTypeResponse)
		}
	}
	expectedTypes := []llmagent.AgentStreamEventType{
		llmagent.AgentStreamEventTypePartial,
		llmagent.AgentStreamEventTypeItem,
		llmagent.AgentStreamEventTypeItem,
		llmagent.AgentStreamEventTypePartial,
		llmagent.AgentStreamEventTypeItem,
		llmagent.AgentStreamEventTypeItem,
		llmagent.AgentStreamEventTypePartial,
		llmagent.AgentStreamEventTypeItem,
		llmagent.AgentStreamEventTypeResponse,
	}
	if diff := cmp.Diff(expectedTypes, eventTypes); diff != "" {
		t.Fatalf("event sequence mismatch (-want +got):\n%s", diff)
	}
	if len(tool.AllCalls) != 2 {
		t.Fatalf("expected two tool executions, got %d", len(tool.AllCalls))
	}
	finalResponse := events[len(events)-1].Response
	if finalResponse == nil || finalResponse.Text() != "All done" || len(finalResponse.Output) != 5 {
		t.Fatalf("unexpected final response: %#v", finalResponse)
	}
}

func TestRunStream_ReturnsLanguageModelError(t *testing.T) {
	model := llmsdktest.NewMockLanguageModel()
	modelErr := llmsdk.NewInvalidInputError("stream failed")
	model.EnqueueStreamResult(llmsdktest.NewMockStreamResultError(modelErr))
	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[struct{}]{Name: "test_agent", Model: model, MaxTurns: 10},
		struct{}{},
	)

	stream, err := session.RunStream(t.Context(), llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Hello"))),
		},
	})
	if err != nil {
		t.Fatalf("create stream: %v", err)
	}
	for stream.Next() {
	}
	if !errors.Is(stream.Err(), modelErr) {
		t.Fatalf("expected wrapped model error, got %v", stream.Err())
	}
	var agentErr *llmagent.AgentError
	if !errors.As(stream.Err(), &agentErr) || agentErr.Kind != llmagent.LanguageModelErrorKind {
		t.Fatalf("expected language model agent error, got %v", stream.Err())
	}
}

func TestRunStream_CommitsMaterializablePartialContentBeforeError(t *testing.T) {
	model := llmsdktest.NewMockLanguageModel()
	modelErr := llmsdk.NewInvalidInputError("stream failed")
	model.EnqueueStreamResult(llmsdktest.NewMockStreamResultPartialsThenError(
		mixedSnapshotPartials(),
		modelErr,
	))
	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[struct{}]{Name: "test_agent", Model: model, MaxTurns: 10},
		struct{}{},
	)
	stream, err := session.RunStream(t.Context(), llmagent.RunSessionRequest{Input: []llmagent.AgentItem{
		llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Hello"))),
	}})
	if err != nil {
		t.Fatalf("create stream: %v", err)
	}
	var events []llmagent.AgentStreamEvent
	for stream.Next() {
		events = append(events, *stream.Current())
	}
	var agentErr *llmagent.AgentError
	if !errors.As(stream.Err(), &agentErr) || agentErr.Kind != llmagent.LanguageModelErrorKind {
		t.Fatalf("expected language model agent error, got %v", stream.Err())
	}
	if agentErr.Snapshot == nil {
		t.Fatal("expected a run snapshot")
	}
	expectedItem := llmagent.NewAgentItemModelResponse(mixedSnapshotModelResponse())
	if diff := cmp.Diff([]llmagent.AgentItem{expectedItem}, agentErr.Snapshot.Output); diff != "" {
		t.Fatalf("snapshot output mismatch (-want +got):\n%s", diff)
	}
	if len(events) != 4 {
		t.Fatalf("expected three partial events and one item event, got %#v", events)
	}
	for _, event := range events[:3] {
		if event.Partial == nil || event.Response != nil {
			t.Fatalf("expected partial events first, got %#v", events)
		}
	}
	if events[3].Item == nil || cmp.Diff(expectedItem, events[3].Item.Item) != "" {
		t.Fatalf("expected committed model item event, got %#v", events[3])
	}
}

func TestRunStream_RecordsCancelledResultsForMaterializedToolCalls(t *testing.T) {
	model := &cancellationLanguageModel{partials: mixedSnapshotPartials()}
	session := mustNewRunSession(
		t,
		&llmagent.AgentParams[struct{}]{Name: "test_agent", Model: model, MaxTurns: 10},
		struct{}{},
	)
	ctx, cancel := context.WithCancel(t.Context())
	stream, err := session.RunStream(ctx, llmagent.RunSessionRequest{Input: []llmagent.AgentItem{
		llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Hello"))),
	}})
	if err != nil {
		t.Fatalf("create stream: %v", err)
	}
	for index := 0; index < 3; index++ {
		if !stream.Next() || stream.Current().Partial == nil {
			t.Fatalf("expected partial event %d, got error %v", index, stream.Err())
		}
	}
	cancel()
	if !stream.Next() || stream.Current().Item == nil {
		t.Fatalf("expected committed model item, got error %v", stream.Err())
	}
	expectedItem := llmagent.NewAgentItemModelResponse(mixedSnapshotModelResponse())
	if diff := cmp.Diff(expectedItem, stream.Current().Item.Item); diff != "" {
		t.Fatalf("model item mismatch (-want +got):\n%s", diff)
	}
	if !stream.Next() || stream.Current().Item == nil {
		t.Fatalf("expected cancelled tool item, got error %v", stream.Err())
	}
	toolCall := mixedSnapshotModelResponse().Content[1].ToolCallPart
	expectedToolItem := llmagent.NewAgentItemTool(
		toolCall.ToolCallID,
		toolCall.ToolName,
		toolCall.Args,
		[]llmsdk.Part{},
		llmsdk.ToolResultStatusCancelled,
	)
	if diff := cmp.Diff(expectedToolItem, stream.Current().Item.Item); diff != "" {
		t.Fatalf("cancelled tool item mismatch (-want +got):\n%s", diff)
	}
	if !stream.Next() || stream.Current().Response == nil {
		t.Fatalf("expected cancelled response, got error %v", stream.Err())
	}
	response := stream.Current().Response
	if response.Status != llmagent.AgentResponseStatusCancelled {
		t.Fatalf("expected cancelled status, got %q", response.Status)
	}
	if diff := cmp.Diff([]llmsdk.Part{}, response.Content); diff != "" {
		t.Fatalf("cancelled content mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]llmagent.AgentItem{expectedItem, expectedToolItem}, response.Output); diff != "" {
		t.Fatalf("cancelled output mismatch (-want +got):\n%s", diff)
	}
	if stream.Next() || stream.Err() != nil {
		t.Fatalf("expected clean end after cancelled response, got %v", stream.Err())
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
			Tools:        llmagent.FunctionTools(tool),
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
		tools:        llmagent.FunctionTools(dynamicTool),
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
				Status:  llmagent.AgentResponseStatusCompleted,
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
	if len(inputs[0].Tools) != 1 || inputs[0].Tools[0].FunctionTool == nil || inputs[0].Tools[0].FunctionTool.Name != "noop" {
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

func TestRunSession_ReturnsInitErrorWhenInstructionResolutionFails(t *testing.T) {
	model := llmsdktest.NewMockLanguageModel()
	cause := errors.New("could not load tenant instructions")
	_, err := llmagent.NewRunSession(
		t.Context(),
		&llmagent.AgentParams[struct{}]{
			Name:  "test_agent",
			Model: model,
			Instructions: []llmagent.InstructionParam[struct{}]{{
				Func: func(context.Context, struct{}) (string, error) {
					return "", cause
				},
			}},
			MaxTurns: 10,
		},
		struct{}{},
	)
	if !errors.Is(err, cause) {
		t.Fatalf("expected wrapped instruction error, got %v", err)
	}
	var agentErr *llmagent.AgentError
	if !errors.As(err, &agentErr) || agentErr.Kind != llmagent.InitErrorKind {
		t.Fatalf("expected agent initialization error, got %v", err)
	}
}

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
