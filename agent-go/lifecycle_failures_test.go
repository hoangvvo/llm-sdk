package llmagent_test

import (
	"context"
	"errors"
	"testing"

	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/llmsdktest"
)

func TestRunSession_ClosesPartialInitializationOnToolkitFailure(t *testing.T) {
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

func TestAgentRun_PreservesModelErrorWhenCleanupFails(t *testing.T) {
	modelFailure := llmsdk.NewInvalidInputError("generation failed")
	cleanupFailure := errors.New("cleanup failed")
	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueGenerateResult(llmsdktest.NewMockGenerateResultError(modelFailure))
	toolkitSession := &mockToolkitSession[struct{}]{closeErr: cleanupFailure}
	agent := llmagent.NewAgent(
		"test_agent",
		model,
		llmagent.WithToolkits[struct{}](&mockToolkit[struct{}]{createFn: func(context.Context, struct{}) (llmagent.ToolkitSession[struct{}], error) {
			return toolkitSession, nil
		}}),
	)

	_, err := agent.Run(t.Context(), llmagent.AgentRequest[struct{}]{
		Context: struct{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Hello"))),
		},
	})
	if !errors.Is(err, modelFailure) || errors.Is(err, cleanupFailure) {
		t.Fatalf("expected primary model error, got %v", err)
	}
}
