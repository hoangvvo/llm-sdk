package llmagent_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/llmsdktest"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"
)

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
