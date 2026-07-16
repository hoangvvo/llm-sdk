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

func TestAgent_Run(t *testing.T) {
	t.Run("returns cleanup error when session cleanup fails", func(t *testing.T) {
		model := llmsdktest.NewMockLanguageModel()
		model.EnqueueGenerateResult(llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{llmsdk.NewTextPart("done")},
		}))
		toolkitSession := &mockToolkitSession[struct{}]{closeErr: errors.New("cleanup failed")}
		agent := llmagent.NewAgent(
			"test_agent",
			model,
			llmagent.WithToolkits[struct{}](&mockToolkit[struct{}]{createFn: func(context.Context, struct{}) (llmagent.ToolkitSession[struct{}], error) {
				return toolkitSession, nil
			}}),
		)

		response, err := agent.Run(t.Context(), llmagent.AgentRequest[struct{}]{
			Context: struct{}{},
			Input: []llmagent.AgentItem{
				llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Hello"))),
			},
		})
		if response != nil {
			t.Fatalf("cleanup failure returned both response and error: %#v", response)
		}
		var agentErr *llmagent.AgentError
		if !errors.As(err, &agentErr) || agentErr.Kind != llmagent.CleanupErrorKind {
			t.Fatalf("expected cleanup error, got %v", err)
		}
		if agentErr.Snapshot != nil {
			t.Fatalf("unexpected cleanup snapshot: %#v", agentErr.Snapshot)
		}
	})

	t.Run("preserves model error when cleanup also fails", func(t *testing.T) {
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
	})

	t.Run("returns cancelled tool result and closes session", func(t *testing.T) {
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
		type runResult struct {
			response *llmagent.AgentResponse
			err      error
		}
		result := make(chan runResult, 1)
		initial := llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Wait")))
		go func() {
			response, err := agent.Run(ctx, llmagent.AgentRequest[struct{}]{
				Context: struct{}{},
				Input:   []llmagent.AgentItem{initial},
			})
			result <- runResult{response: response, err: err}
		}()
		<-started
		cancel()
		got := <-result
		if got.err != nil {
			t.Fatalf("expected cancelled response, got %v", got.err)
		}
		if got.response.Status != llmagent.AgentResponseStatusCancelled {
			t.Fatalf("expected cancelled status, got %q", got.response.Status)
		}
		if len(got.response.Output) != 2 || got.response.Output[1].Tool == nil {
			t.Fatalf("expected model and cancelled tool output, got %#v", got.response.Output)
		}
		if got.response.Output[1].Tool.Status != llmsdk.ToolResultStatusCancelled {
			t.Fatalf("expected cancelled tool status, got %q", got.response.Output[1].Tool.Status)
		}
		if len(got.response.Output[1].Tool.Output) != 0 {
			t.Fatalf("expected content-neutral cancelled result, got %#v", got.response.Output[1].Tool.Output)
		}
		if toolkitSession.closeCalls != 1 {
			t.Fatalf("expected toolkit cleanup after cancellation, got %d", toolkitSession.closeCalls)
		}
		if cleanupContextErr != nil {
			t.Fatalf("cleanup inherited canceled context: %v", cleanupContextErr)
		}

		model.EnqueueGenerateResult(llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{llmsdk.NewTextPart("continued")},
		}))
		nextInput := append([]llmagent.AgentItem{initial}, got.response.Output...)
		nextInput = append(nextInput, llmagent.NewAgentItemMessage(
			llmsdk.NewUserMessage(llmsdk.NewTextPart("Continue")),
		))
		nextAgent := llmagent.NewAgent[struct{}]("test_agent", model)
		if _, err := nextAgent.Run(t.Context(), llmagent.AgentRequest[struct{}]{
			Context: struct{}{},
			Input:   nextInput,
		}); err != nil {
			t.Fatalf("next run failed: %v", err)
		}
		inputs := model.TrackedGenerateInputs()
		if len(inputs) != 2 || len(inputs[1].Messages) < 3 || inputs[1].Messages[2].ToolMessage == nil {
			t.Fatalf("expected cancelled tool result before next user message, got %#v", inputs)
		}
		toolParts := inputs[1].Messages[2].ToolMessage.Content
		if len(toolParts) != 1 || toolParts[0].ToolResultPart == nil {
			t.Fatalf("expected one cancelled tool result, got %#v", toolParts)
		}
		if toolParts[0].ToolResultPart.Status != llmsdk.ToolResultStatusCancelled || len(toolParts[0].ToolResultPart.Content) != 0 {
			t.Fatalf("unexpected cancelled tool result: %#v", toolParts[0].ToolResultPart)
		}
	})

	t.Run("does not start later tools after a non-cooperative tool finishes", func(t *testing.T) {
		model := llmsdktest.NewMockLanguageModel()
		model.EnqueueGenerateResult(llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
			Content: []llmsdk.Part{
				llmsdk.NewToolCallPart("call_1", "first", map[string]any{}),
				llmsdk.NewToolCallPart("call_2", "second", map[string]any{}),
			},
		}))
		started := make(chan struct{})
		releaseFirst := make(chan struct{})
		first := NewMockTool[struct{}]("first", llmagent.AgentToolResult{}, func(context.Context, json.RawMessage, struct{}, *llmagent.RunState) (llmagent.AgentToolResult, error) {
			close(started)
			<-releaseFirst
			return llmagent.AgentToolResult{
				Content: []llmsdk.Part{llmsdk.NewTextPart("first finished")},
			}, nil
		})
		second := NewMockTool[struct{}]("second", llmagent.AgentToolResult{
			Content: []llmsdk.Part{llmsdk.NewTextPart("second finished")},
		}, nil)
		agent := llmagent.NewAgent(
			"test_agent",
			model,
			llmagent.WithTools[struct{}](
				llmagent.NewAgentFunctionTool[struct{}](first),
				llmagent.NewAgentFunctionTool[struct{}](second),
			),
		)
		ctx, cancel := context.WithCancel(t.Context())
		type runResult struct {
			response *llmagent.AgentResponse
			err      error
		}
		result := make(chan runResult, 1)
		go func() {
			response, err := agent.Run(ctx, llmagent.AgentRequest[struct{}]{
				Context: struct{}{},
				Input: []llmagent.AgentItem{
					llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Run both tools"))),
				},
			})
			result <- runResult{response: response, err: err}
		}()

		<-started
		cancel()
		close(releaseFirst)
		got := <-result
		if got.err != nil {
			t.Fatalf("expected cancelled response, got %v", got.err)
		}
		if got.response.Status != llmagent.AgentResponseStatusCancelled {
			t.Fatalf("expected cancelled status, got %q", got.response.Status)
		}
		if len(got.response.Output) != 3 || got.response.Output[1].Tool == nil || got.response.Output[2].Tool == nil {
			t.Fatalf("expected model, completed tool, and cancelled tool output; got %#v", got.response.Output)
		}
		if got.response.Output[1].Tool.Status != llmsdk.ToolResultStatusCompleted {
			t.Fatalf("expected first tool to remain completed, got %q", got.response.Output[1].Tool.Status)
		}
		if got.response.Output[2].Tool.Status != llmsdk.ToolResultStatusCancelled {
			t.Fatalf("expected second tool to be cancelled, got %q", got.response.Output[2].Tool.Status)
		}
		if len(second.AllCalls) != 0 {
			t.Fatalf("second tool executed after cancellation: %#v", second.AllCalls)
		}
	})

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
			Status: llmagent.AgentResponseStatusCompleted,
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
	t.Run("emits cleanup error instead of response", func(t *testing.T) {
		model := llmsdktest.NewMockLanguageModel()
		model.EnqueueStreamResult(llmsdktest.NewMockStreamResultPartials([]llmsdk.PartialModelResponse{
			{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta("done")}},
		}))
		toolkitSession := &mockToolkitSession[struct{}]{closeErr: errors.New("cleanup failed")}
		agent := llmagent.NewAgent(
			"test_agent",
			model,
			llmagent.WithToolkits[struct{}](&mockToolkit[struct{}]{createFn: func(context.Context, struct{}) (llmagent.ToolkitSession[struct{}], error) {
				return toolkitSession, nil
			}}),
		)
		stream, err := agent.RunStream(t.Context(), llmagent.AgentRequest[struct{}]{
			Context: struct{}{},
			Input: []llmagent.AgentItem{
				llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Hello"))),
			},
		})
		if err != nil {
			t.Fatalf("create stream: %v", err)
		}
		var events []*llmagent.AgentStreamEvent
		for stream.Next() {
			events = append(events, stream.Current())
		}
		var agentErr *llmagent.AgentError
		if !errors.As(stream.Err(), &agentErr) || agentErr.Kind != llmagent.CleanupErrorKind {
			t.Fatalf("expected cleanup error, got %v", stream.Err())
		}
		if agentErr.Snapshot != nil {
			t.Fatalf("unexpected cleanup snapshot: %#v", agentErr.Snapshot)
		}
		if len(events) != 2 || events[0].Partial == nil || events[1].Item == nil {
			t.Fatalf("expected partial and item but no response event, got %#v", events)
		}
	})

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
					Status: llmagent.AgentResponseStatusCompleted,
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

func TestAgent_ForwardsCompletePublicConfiguration(t *testing.T) {
	type testContext struct{ Tenant string }
	model := llmsdktest.NewMockLanguageModel()
	model.EnqueueGenerateResult(llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
		Content: []llmsdk.Part{llmsdk.NewTextPart("configured")},
	}))
	functionTool := NewMockTool[testContext]("lookup", llmagent.AgentToolResult{}, nil)
	functionTool.description = "Look up a record"
	functionTool.parameters = llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string"},
		},
		"required":             []any{"query"},
		"additionalProperties": false,
	}
	responseSchema := llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"answer": map[string]any{"type": "string"},
		},
		"required":             []any{"answer"},
		"additionalProperties": false,
	}
	responseFormat := llmsdk.NewResponseFormatJSON(
		"answer",
		ptr.To("A configured answer"),
		&responseSchema,
	)
	audio := llmsdk.AudioOptions{
		Format:       ptr.To(llmsdk.AudioFormatMP3),
		Voice:        ptr.To("alloy"),
		LanguageCode: ptr.To("en"),
	}
	reasoning := llmsdk.ReasoningOptions{Enabled: true, BudgetTokens: ptr.To(uint32(256))}
	webSearch := llmsdk.WebSearchTool{AllowedDomains: []string{"example.com"}}

	agent := llmagent.NewAgent(
		"configured-agent",
		model,
		llmagent.WithInstructions[testContext](
			llmagent.InstructionParam[testContext]{String: ptr.To("Static")},
			llmagent.InstructionParam[testContext]{Func: func(_ context.Context, ctx testContext) (string, error) {
				return "Tenant: " + ctx.Tenant, nil
			}},
		),
		llmagent.WithTools[testContext](
			llmagent.NewAgentFunctionTool[testContext](functionTool),
			llmagent.NewAgentWebSearchTool[testContext](webSearch),
		),
		llmagent.WithResponseFormat[testContext](*responseFormat),
		llmagent.WithMaxTurns[testContext](3),
		llmagent.WithTemperature[testContext](0.2),
		llmagent.WithTopP[testContext](0.8),
		llmagent.WithTopK[testContext](12),
		llmagent.WithPresencePenalty[testContext](0.1),
		llmagent.WithFrequencyPenalty[testContext](0.3),
		llmagent.WithModalities[testContext](llmsdk.ModalityText, llmsdk.ModalityAudio),
		llmagent.WithAudio[testContext](audio),
		llmagent.WithReasoning[testContext](reasoning),
	)

	_, err := agent.Run(t.Context(), llmagent.AgentRequest[testContext]{
		Context: testContext{Tenant: "acme"},
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Configure this"))),
		},
	})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}

	inputs := model.TrackedGenerateInputs()
	if len(inputs) != 1 {
		t.Fatalf("expected one model call, got %d", len(inputs))
	}
	expected := llmsdk.LanguageModelInput{
		SystemPrompt: ptr.To("Static\nTenant: acme"),
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(llmsdk.NewTextPart("Configure this")),
		},
		Tools: []llmsdk.Tool{
			llmsdk.NewFunctionTool("lookup", "Look up a record", functionTool.parameters),
			{WebSearchTool: &webSearch},
		},
		ResponseFormat:   responseFormat,
		Temperature:      ptr.To(0.2),
		TopP:             ptr.To(0.8),
		TopK:             ptr.To(int32(12)),
		PresencePenalty:  ptr.To(0.1),
		FrequencyPenalty: ptr.To(0.3),
		Modalities:       []llmsdk.Modality{llmsdk.ModalityText, llmsdk.ModalityAudio},
		Audio:            &audio,
		Reasoning:        &reasoning,
	}
	if diff := cmp.Diff(expected, inputs[0]); diff != "" {
		t.Fatalf("model input mismatch (-want +got):\n%s", diff)
	}
}
