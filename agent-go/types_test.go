package llmagent_test

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

func TestAgentResponse_TextReturnsOnlyNonEmptyText(t *testing.T) {
	response := &llmagent.AgentResponse{Content: []llmsdk.Part{
		llmsdk.NewTextPart("Hello"),
		llmsdk.NewImagePart("AAEC", "image/png"),
		llmsdk.NewTextPart(""),
		llmsdk.NewTextPart("world"),
	}}

	if got := response.Text(); got != "Hello world" {
		t.Fatalf("expected joined text, got %q", got)
	}
}

func TestAgentItem_JSONContractAndRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		item     llmagent.AgentItem
		expected string
	}{
		{
			name: "message",
			item: llmagent.NewAgentItemMessage(
				llmsdk.NewUserMessage(llmsdk.NewTextPart("Hello")),
			),
			expected: `{"type":"message","role":"user","content":[{"type":"text","text":"Hello"}]}`,
		},
		{
			name: "model",
			item: llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{llmsdk.NewTextPart("Hi")},
			}),
			expected: `{"type":"model","content":[{"type":"text","text":"Hi"}]}`,
		},
		{
			name: "tool",
			item: llmagent.NewAgentItemTool(
				"call_1",
				"lookup",
				json.RawMessage(`{"id":42}`),
				[]llmsdk.Part{llmsdk.NewTextPart("found")},
				llmsdk.ToolResultStatusCompleted,
			),
			expected: `{"type":"tool","tool_call_id":"call_1","tool_name":"lookup","input":{"id":42},"output":[{"type":"text","text":"found"}],"status":"completed"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.item)
			if err != nil {
				t.Fatalf("marshal item: %v", err)
			}
			assertJSONEqual(t, tt.expected, data)

			var decoded llmagent.AgentItem
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("unmarshal item: %v", err)
			}
			if diff := cmp.Diff(tt.item, decoded); diff != "" {
				t.Fatalf("round-trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAgentItem_UnmarshalReplacesPreviousVariant(t *testing.T) {
	item := llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("old")))
	if err := json.Unmarshal(
		[]byte(`{"type":"tool","tool_call_id":"call_1","tool_name":"lookup","input":{},"output":[],"status":"completed"}`),
		&item,
	); err != nil {
		t.Fatalf("unmarshal replacement: %v", err)
	}

	if item.Type() != llmagent.AgentItemTypeTool || item.Message != nil {
		t.Fatalf("expected only the replacement tool variant, got %#v", item)
	}
}

func TestAgentStreamEvent_JSONContractAndRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		event    *llmagent.AgentStreamEvent
		expected string
	}{
		{
			name: "partial",
			event: llmagent.NewAgentStreamEventPartial(&llmsdk.PartialModelResponse{
				Delta: &llmsdk.ContentDelta{Index: 0, Part: llmsdk.NewTextPartDelta("Hi")},
			}),
			expected: `{"event":"partial","delta":{"index":0,"part":{"type":"text","text":"Hi"}}}`,
		},
		{
			name: "item",
			event: llmagent.NewAgentStreamItemEvent(
				2,
				llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
					Content: []llmsdk.Part{llmsdk.NewTextPart("Hi")},
				}),
			),
			expected: `{"event":"item","index":2,"item":{"type":"model","content":[{"type":"text","text":"Hi"}]}}`,
		},
		{
			name: "response",
			event: llmagent.NewAgentStreamEventResponse(&llmagent.AgentResponse{
				Status:  llmagent.AgentResponseStatusCompleted,
				Output:  []llmagent.AgentItem{},
				Content: []llmsdk.Part{llmsdk.NewTextPart("Done")},
			}),
			expected: `{"event":"response","output":[],"content":[{"type":"text","text":"Done"}],"status":"completed"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("marshal event: %v", err)
			}
			assertJSONEqual(t, tt.expected, data)

			decoded := llmagent.NewAgentStreamEventResponse(&llmagent.AgentResponse{})
			if err := json.Unmarshal(data, decoded); err != nil {
				t.Fatalf("unmarshal event: %v", err)
			}
			if diff := cmp.Diff(tt.event, decoded); diff != "" {
				t.Fatalf("round-trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func assertJSONEqual(t *testing.T, expected string, actual []byte) {
	t.Helper()
	var expectedValue any
	if err := json.Unmarshal([]byte(expected), &expectedValue); err != nil {
		t.Fatalf("decode expected JSON: %v", err)
	}
	var actualValue any
	if err := json.Unmarshal(actual, &actualValue); err != nil {
		t.Fatalf("decode actual JSON: %v", err)
	}
	if diff := cmp.Diff(expectedValue, actualValue); diff != "" {
		t.Fatalf("JSON mismatch (-want +got):\n%s", diff)
	}
}
