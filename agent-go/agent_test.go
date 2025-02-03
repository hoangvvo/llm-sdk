package llmagent_test

import (
	"context"
	"testing"

	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

func createMockLanguageModelForAgent() *MockLanguageModel {
	return &MockLanguageModel{
		responses: []*llmsdk.ModelResponse{
			{
				Content: []llmsdk.Part{
					{TextPart: &llmsdk.TextPart{Text: "Mock response"}},
				},
			},
		},
		partialResponses: [][]llmsdk.PartialModelResponse{
			{
				{Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.PartDelta{TextPartDelta: &llmsdk.TextPartDelta{Text: "Mock"}}}},
			},
		},
		generateCalls: []*llmsdk.LanguageModelInput{},
		streamCalls:   []*llmsdk.LanguageModelInput{},
	}
}

func TestAgent_Run(t *testing.T) {
	t.Run("creates session, runs, and finishes", func(t *testing.T) {
		model := createMockLanguageModelForAgent()
		agent := llmagent.NewAgent[map[string]interface{}]("test-agent", model)

		response, err := agent.Run(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
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

		expectedResponse := &llmagent.AgentResponse{
			Content: []llmsdk.Part{
				{TextPart: &llmsdk.TextPart{Text: "Mock response"}},
			},
			Output: []llmagent.AgentItem{
				llmagent.NewMessageAgentItem(llmsdk.Message{
					AssistantMessage: &llmsdk.AssistantMessage{
						Content: []llmsdk.Part{
							{TextPart: &llmsdk.TextPart{Text: "Mock response"}},
						},
					},
				}),
			},
		}

		if response.Content[0].TextPart.Text != expectedResponse.Content[0].TextPart.Text {
			t.Errorf("expected content %q, got %q", expectedResponse.Content[0].TextPart.Text, response.Content[0].TextPart.Text)
		}

		actualContent := response.Output[0].Message.AssistantMessage.Content[0].TextPart.Text
		expectedContent := expectedResponse.Output[0].Message.AssistantMessage.Content[0].TextPart.Text
		if actualContent != expectedContent {
			t.Errorf("expected output content %q, got %q", expectedContent, actualContent)
		}
	})
}

func TestAgent_RunStream(t *testing.T) {
	t.Run("creates session, streams, and finishes", func(t *testing.T) {
		model := createMockLanguageModelForAgent()
		agent := llmagent.NewAgent[map[string]interface{}]("test-agent", model)

		stream, err := agent.RunStream(context.Background(), llmagent.AgentRequest[map[string]interface{}]{
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

		events := []*llmagent.AgentStreamEvent{}
		for stream.Next() {
			events = append(events, stream.Current())
		}

		if err := stream.Err(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Check for partial event
		var foundPartial bool
		var foundMessage bool
		var foundResponse bool

		for _, event := range events {
			if event.Partial != nil {
				foundPartial = true
				// Verify partial event structure
				if event.Partial.Delta == nil {
					t.Error("expected delta in partial event")
				} else {
					if event.Partial.Delta.Index != 0 {
						t.Errorf("expected delta index 0, got %d", event.Partial.Delta.Index)
					}
					if event.Partial.Delta.Part.TextPartDelta == nil {
						t.Error("expected text part delta")
					} else if event.Partial.Delta.Part.TextPartDelta.Text != "Mock" {
						t.Errorf("expected text 'Mock', got %q", event.Partial.Delta.Part.TextPartDelta.Text)
					}
				}
			}

			if event.Message != nil {
				foundMessage = true
				// Verify message event structure
				if event.Message.AssistantMessage.Content[0].TextPart.Text != "Mock" {
					t.Errorf("expected message text 'Mock', got %q", event.Message.AssistantMessage.Content[0].TextPart.Text)
				}
			}

			if event.Response != nil {
				foundResponse = true
				// Verify response event structure
				if event.Response.Content[0].TextPart.Text != "Mock" {
					t.Errorf("expected response text 'Mock', got %q", event.Response.Content[0].TextPart.Text)
				}

				outputMessage := event.Response.Output[0].Message
				if outputMessage.AssistantMessage.Content[0].TextPart.Text != "Mock" {
					t.Errorf("expected output message text 'Mock', got %q", outputMessage.AssistantMessage.Content[0].TextPart.Text)
				}
			}
		}

		if !foundPartial {
			t.Error("expected to find partial event")
		}
		if !foundMessage {
			t.Error("expected to find message event")
		}
		if !foundResponse {
			t.Error("expected to find response event")
		}
	})
}
