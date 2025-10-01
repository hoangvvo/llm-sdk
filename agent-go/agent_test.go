package llmagent_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/llmsdktest"
)

func TestAgent_Run(t *testing.T) {
	t.Run("creates session, runs, and closes", func(t *testing.T) {
		model := llmsdktest.NewMockLanguageModel()
		model.EnqueueGenerateResult(
			llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					llmsdk.NewTextPart("Mock response"),
				},
			}),
		)
		agent := llmagent.NewAgent[map[string]interface{}]("test-agent", model)

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
	})
}

func TestAgent_RunStream(t *testing.T) {
	t.Run("creates session, streams, and closes", func(t *testing.T) {
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
		agent := llmagent.NewAgent[map[string]interface{}]("test-agent", model)

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
	})
}
