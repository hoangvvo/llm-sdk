package mcp_test

import (
	"context"
	"errors"
	"testing"

	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmmcp "github.com/hoangvvo/llm-sdk/agent-go/mcp"
	"github.com/hoangvvo/llm-sdk/sdk-go/llmsdktest"
)

func TestMCPInitialization_ResolvesParamsFromContext(t *testing.T) {
	type sessionContext struct{ Endpoint string }
	var contexts []sessionContext
	model := llmsdktest.NewMockLanguageModel()
	toolkit := llmmcp.NewMCPToolkit(func(_ context.Context, value sessionContext) (llmmcp.MCPParams, error) {
		contexts = append(contexts, value)
		return llmmcp.NewMCPStreamableHTTPParams(value.Endpoint, ""), nil
	})
	agent := llmagent.NewAgent(
		"test_agent",
		model,
		llmagent.WithToolkits[sessionContext](toolkit),
	)

	_, err := agent.CreateSession(t.Context(), sessionContext{Endpoint: "://invalid"})
	if err == nil {
		t.Fatal("expected invalid MCP endpoint to fail initialization")
	}
	var agentErr *llmagent.AgentError
	if !errors.As(err, &agentErr) || agentErr.Kind != llmagent.InitErrorKind {
		t.Fatalf("expected agent initialization error, got %v", err)
	}
	if len(contexts) != 1 || contexts[0].Endpoint != "://invalid" {
		t.Fatalf("resolver received unexpected contexts: %#v", contexts)
	}
}

func TestMCPInitialization_WrapsResolverError(t *testing.T) {
	cause := errors.New("credential lookup failed")
	model := llmsdktest.NewMockLanguageModel()
	toolkit := llmmcp.NewMCPToolkit(func(context.Context, struct{}) (llmmcp.MCPParams, error) {
		return llmmcp.MCPParams{}, cause
	})
	agent := llmagent.NewAgent(
		"test_agent",
		model,
		llmagent.WithToolkits[struct{}](toolkit),
	)

	_, err := agent.CreateSession(t.Context(), struct{}{})
	if !errors.Is(err, cause) {
		t.Fatalf("expected wrapped resolver error, got %v", err)
	}
	var agentErr *llmagent.AgentError
	if !errors.As(err, &agentErr) || agentErr.Kind != llmagent.InitErrorKind {
		t.Fatalf("expected agent initialization error, got %v", err)
	}
}
