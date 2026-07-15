package mcp

import (
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestConvertMCPContentToParts_RejectsUnsupportedAudio(t *testing.T) {
	_, err := convertMCPContentToParts([]gomcp.Content{
		&gomcp.AudioContent{MIMEType: "audio/unknown", Data: []byte{0, 1, 2}},
	})
	if err == nil {
		t.Fatal("expected unsupported MCP audio format to fail")
	}
}
