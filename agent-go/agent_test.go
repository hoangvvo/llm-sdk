package llmagent_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/llmsdktest"
)

func TestAgent_Run(t *testing.T) {
	t.Run("creates session, runs, and closes", func(t *testing.T) {
		toolkitSession := &mockToolkitSession[map[string]interface{}]{}
		model := llmsdktest.NewMockLanguageModel()
		model.EnqueueGenerateResult(
			llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					llmsdk.NewTextPart("Mock response"),
				},
			}),
		)
		agent := llmagent.NewAgent(
			"test-agent",
			model,
			llmagent.WithToolkits(&mockToolkit[map[string]interface{}]{
				createFn: func(context.Context, map[string]interface{}) (llmagent.ToolkitSession[map[string]interface{}], error) {
					return toolkitSession, nil
				},
			}),
		)

		response, err := agent.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
			Context: map[string]interface{}{},
			Input: []llmagent.AgentItem{
				llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Hello"))),
			},
		})

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		expectedResponse := &llmagent.AgentResponse{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart("Mock response"),
			},
			Output: []llmagent.AgentItem{
				llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
					Content: []llmsdk.Part{
						llmsdk.NewTextPart("Mock response"),
					},
				}),
			},
		}

		if diff := cmp.Diff(expectedResponse, response); diff != "" {
			t.Errorf("response mismatch (-want +got): %s", diff)
		}
		if toolkitSession.closeCalls != 1 {
			t.Fatalf("expected toolkit session to close once, got %d", toolkitSession.closeCalls)
		}
	})

	t.Run("closes session when generation fails", func(t *testing.T) {
		toolkitSession := &mockToolkitSession[map[string]interface{}]{}
		model := llmsdktest.NewMockLanguageModel()
		modelErr := llmsdk.NewInvalidInputError("generation failed")
		model.EnqueueGenerateResult(llmsdktest.NewMockGenerateResultError(modelErr))
		agent := llmagent.NewAgent(
			"test-agent",
			model,
			llmagent.WithToolkits(&mockToolkit[map[string]interface{}]{
				createFn: func(context.Context, map[string]interface{}) (llmagent.ToolkitSession[map[string]interface{}], error) {
					return toolkitSession, nil
				},
			}),
		)

		_, err := agent.Run(t.Context(), llmagent.AgentRequest[map[string]interface{}]{
			Context: map[string]interface{}{},
			Input: []llmagent.AgentItem{
				llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Hello"))),
			},
		})
		if !errors.Is(err, modelErr) {
			t.Fatalf("expected wrapped model error, got %v", err)
		}
		if toolkitSession.closeCalls != 1 {
			t.Fatalf("expected toolkit session to close once, got %d", toolkitSession.closeCalls)
		}
	})
}

func TestAgent_RunStream(t *testing.T) {
	t.Run("creates session, streams, and closes", func(t *testing.T) {
		toolkitSession := &mockToolkitSession[map[string]interface{}]{}
		model := llmsdktest.NewMockLanguageModel()
		model.EnqueueStreamResult(
			llmsdktest.NewMockStreamResultPartials([]llmsdk.PartialModelResponse{
				{
					Delta: &llmsdk.ContentDelta{
						Index: 0,
						Part:  llmsdk.NewTextPartDelta("Mock"),
					},
				},
			}),
		)
		agent := llmagent.NewAgent(
			"test-agent",
			model,
			llmagent.WithToolkits(&mockToolkit[map[string]interface{}]{
				createFn: func(context.Context, map[string]interface{}) (llmagent.ToolkitSession[map[string]interface{}], error) {
					return toolkitSession, nil
				},
			}),
		)

		stream, err := agent.RunStream(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
			Context: map[string]interface{}{},
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
					Delta: &llmsdk.ContentDelta{
						Index: 0,
						Part:  llmsdk.NewTextPartDelta("Mock"),
					},
				},
			},
			llmagent.NewAgentStreamItemEvent(
				0,
				llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
					Content: []llmsdk.Part{
						llmsdk.NewTextPart("Mock"),
					},
				}),
			),
			{
				Response: &llmagent.AgentResponse{
					Content: []llmsdk.Part{
						llmsdk.NewTextPart("Mock"),
					},
					Output: []llmagent.AgentItem{
						llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
							Content: []llmsdk.Part{
								llmsdk.NewTextPart("Mock"),
							},
						}),
					},
				},
			},
		}

		if diff := cmp.Diff(expectedEvents, events); diff != "" {
			t.Errorf("stream events mismatch (-want +got):\n%s", diff)
		}
		if toolkitSession.closeCalls != 1 {
			t.Fatalf("expected toolkit session to close once, got %d", toolkitSession.closeCalls)
		}
	})

	t.Run("closes session when streaming fails", func(t *testing.T) {
		toolkitSession := &mockToolkitSession[map[string]interface{}]{}
		model := llmsdktest.NewMockLanguageModel()
		modelErr := llmsdk.NewInvalidInputError("stream failed")
		model.EnqueueStreamResult(llmsdktest.NewMockStreamResultError(modelErr))
		agent := llmagent.NewAgent(
			"test-agent",
			model,
			llmagent.WithToolkits(&mockToolkit[map[string]interface{}]{
				createFn: func(context.Context, map[string]interface{}) (llmagent.ToolkitSession[map[string]interface{}], error) {
					return toolkitSession, nil
				},
			}),
		)

		stream, err := agent.RunStream(t.Context(), llmagent.AgentRequest[map[string]interface{}]{
			Context: map[string]interface{}{},
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
		if toolkitSession.closeCalls != 1 {
			t.Fatalf("expected toolkit session to close once, got %d", toolkitSession.closeCalls)
		}
	})
}
