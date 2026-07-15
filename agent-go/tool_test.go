package llmagent_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

func TestAgentTool_ConvertsPublicToolVariantsForModelUse(t *testing.T) {
	function := NewMockTool[struct{}]("lookup", llmagent.AgentToolResult{}, nil)
	function.description = "Look up a record"
	function.parameters = llmsdk.JSONSchema{
		"type":       "object",
		"properties": map[string]any{},
	}
	functionTool := llmagent.NewAgentFunctionTool[struct{}](function)
	if functionTool.Name() != "lookup" || functionTool.AsFunctionTool() != function {
		t.Fatalf("function tool identity was not preserved: %#v", functionTool)
	}
	expectedFunction := llmsdk.NewFunctionTool("lookup", "Look up a record", function.parameters)
	if diff := cmp.Diff(expectedFunction, functionTool.ToLanguageModelTool()); diff != "" {
		t.Fatalf("function conversion mismatch (-want +got):\n%s", diff)
	}

	webSearch := llmsdk.WebSearchTool{AllowedDomains: []string{"example.com"}}
	webSearchTool := llmagent.NewAgentWebSearchTool[struct{}](webSearch)
	if webSearchTool.Name() != "web_search" || webSearchTool.AsFunctionTool() != nil {
		t.Fatalf("unexpected hosted-tool behavior: %#v", webSearchTool)
	}
	if diff := cmp.Diff(llmsdk.Tool{WebSearchTool: &webSearch}, webSearchTool.ToLanguageModelTool()); diff != "" {
		t.Fatalf("web-search conversion mismatch (-want +got):\n%s", diff)
	}

	var empty llmagent.AgentTool[struct{}]
	if empty.Name() != "" || empty.AsFunctionTool() != nil || empty.ToLanguageModelTool().Type() != "" {
		t.Fatalf("zero-value tool should remain empty: %#v", empty)
	}
}
