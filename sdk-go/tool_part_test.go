package llmsdk_test

import (
	"encoding/json"
	"testing"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

func TestWebSearchToolPartsUseNestedDiscriminators(t *testing.T) {
	status := llmsdk.WebSearchToolCallStatusCompleted
	part := llmsdk.Part{ToolCallPart: &llmsdk.ToolCallPart{
		ToolCallID: "ws_1",
		Call: llmsdk.ToolCall{WebSearch: &llmsdk.WebSearchToolCall{
			Status: &status,
			Action: &llmsdk.WebSearchAction{Type: "search", Queries: []string{"sdk docs"}},
		}},
	}}
	data, err := json.Marshal(part)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	call, ok := raw["call"].(map[string]any)
	if raw["type"] != "tool-call" || !ok || call["type"] != "web_search" {
		t.Fatalf("unexpected wire shape: %s", data)
	}
	var decoded llmsdk.Part
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ToolCallPart == nil || decoded.ToolCallPart.Call.WebSearch == nil {
		t.Fatalf("failed to decode web-search call: %#v", decoded)
	}

	errorCode := "unavailable"
	resultData, err := json.Marshal(llmsdk.Part{ToolResultPart: &llmsdk.ToolResultPart{
		ToolCallID: "ws_1",
		Result: llmsdk.ToolResult{WebSearch: &llmsdk.WebSearchToolResult{
			ErrorCode: &errorCode,
		}},
		Status: llmsdk.ToolResultStatusFailed,
	}})
	if err != nil {
		t.Fatal(err)
	}
	var rawResult struct {
		Result struct {
			Sources json.RawMessage `json:"sources"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resultData, &rawResult); err != nil {
		t.Fatal(err)
	}
	if string(rawResult.Result.Sources) != "[]" {
		t.Fatalf("empty web-search sources must be an array: %s", resultData)
	}
	var decodedResult llmsdk.Part
	if err := json.Unmarshal(resultData, &decodedResult); err != nil {
		t.Fatal(err)
	}
	if decodedResult.ToolResultPart == nil || decodedResult.ToolResultPart.Result.WebSearch == nil || decodedResult.ToolResultPart.Result.WebSearch.ErrorCode == nil || *decodedResult.ToolResultPart.Result.WebSearch.ErrorCode != errorCode {
		t.Fatalf("failed to round-trip web-search error: %s", resultData)
	}
}

func TestFunctionToolCallMarshalPreservesRawNumbers(t *testing.T) {
	wire := []byte(`{"type":"tool-call","tool_call_id":"call_1","call":{"type":"function","name":"lookup","args":{"id":9007199254740993}}}`)
	var part llmsdk.Part
	if err := json.Unmarshal(wire, &part); err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(part)
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Call struct {
			Args json.RawMessage `json:"args"`
		} `json:"call"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if string(raw.Call.Args) != `{"id":9007199254740993}` {
		t.Fatalf("function arguments changed during round trip: %s", data)
	}
}
