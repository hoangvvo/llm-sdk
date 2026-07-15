package mcp_test

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	llmmcp "github.com/hoangvvo/llm-sdk/agent-go/mcp"
)

func TestMCPParams_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		params   llmmcp.MCPParams
		expected string
	}{
		{
			name:     "stdio",
			params:   llmmcp.NewMCPStdioParams("uvx", []string{"server.py"}),
			expected: `{"type":"stdio","command":"uvx","args":["server.py"]}`,
		},
		{
			name:     "streamable HTTP",
			params:   llmmcp.NewMCPStreamableHTTPParams("https://example.com/mcp", "Bearer token"),
			expected: `{"type":"streamable-http","url":"https://example.com/mcp","authorization":"Bearer token"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.params)
			if err != nil {
				t.Fatalf("marshal params: %v", err)
			}
			if string(data) != tt.expected {
				t.Fatalf("unexpected JSON: %s", data)
			}

			var decoded llmmcp.MCPParams
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("unmarshal params: %v", err)
			}
			roundTrip, err := json.Marshal(decoded)
			if err != nil {
				t.Fatalf("marshal decoded params: %v", err)
			}
			if diff := cmp.Diff(data, roundTrip); diff != "" {
				t.Fatalf("round-trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMCPParams_RejectsInvalidVariants(t *testing.T) {
	for _, input := range []string{
		`{"type":"stdio"}`,
		`{"type":"streamable-http"}`,
		`{"type":"websocket","url":"https://example.com"}`,
	} {
		var params llmmcp.MCPParams
		if err := json.Unmarshal([]byte(input), &params); err == nil {
			t.Fatalf("expected invalid MCP params %s to fail", input)
		}
	}
}
