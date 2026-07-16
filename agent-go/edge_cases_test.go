package llmagent_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/llmsdktest"
)

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

func TestAgentRun_CancellationReachesToolAndStillClosesSession(t *testing.T) {
	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueGenerateResult(llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
		Content: []llmsdk.Part{llmsdk.NewToolCallPart("call_1", "wait", map[string]any{})},
	}))
	started := make(chan struct{})
	tool := NewMockTool[struct{}]("wait", llmagent.AgentToolResult{}, func(ctx context.Context, _ json.RawMessage, _ struct{}, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
		close(started)
		<-ctx.Done()
		return llmagent.AgentToolResult{}, ctx.Err()
	})
	var cleanupContextErr error
	toolkitSession := &mockToolkitSession[struct{}]{
		closeFn: func(ctx context.Context) error {
			cleanupContextErr = ctx.Err()
			return nil
		},
	}
	agent := llmagent.NewAgent(
		"test_agent",
		model,
		llmagent.WithTools[struct{}](llmagent.NewAgentFunctionTool[struct{}](tool)),
		llmagent.WithToolkits[struct{}](&mockToolkit[struct{}]{createFn: func(context.Context, struct{}) (llmagent.ToolkitSession[struct{}], error) {
			return toolkitSession, nil
		}}),
	)
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() {
		_, err := agent.Run(ctx, llmagent.AgentRequest[struct{}]{
			Context: struct{}{},
			Input: []llmagent.AgentItem{
				llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Wait"))),
			},
		})
		result <- err
	}()
	<-started
	cancel()
	err := <-result
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if toolkitSession.closeCalls != 1 {
		t.Fatalf("expected toolkit cleanup after cancellation, got %d", toolkitSession.closeCalls)
	}
	if cleanupContextErr != nil {
		t.Fatalf("cleanup inherited canceled context: %v", cleanupContextErr)
	}
}
