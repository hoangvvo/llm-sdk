package mcp

import (
	"encoding/base64"
	"fmt"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/partutil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// convertMCPContentToParts maps MCP content blocks to the agent toolkit primitive.
// Unsupported content types are ignored so the agent can still surface partial results.
func convertMCPContentToParts(contents []mcp.Content) ([]llmsdk.Part, error) {
	parts := make([]llmsdk.Part, 0, len(contents))

	for _, content := range contents {
		switch c := content.(type) {
		case *mcp.TextContent:
			parts = append(parts, llmsdk.Part{TextPart: &llmsdk.TextPart{Text: c.Text}})
		case *mcp.ImageContent:
			encoded := base64.StdEncoding.EncodeToString(c.Data)
			parts = append(parts, llmsdk.Part{ImagePart: &llmsdk.ImagePart{
				MimeType:  c.MIMEType,
				ImageData: encoded,
			}})
		case *mcp.AudioContent:
			format, err := partutil.MapMimeTypeToAudioFormat(c.MIMEType)
			if err != nil {
				return nil, fmt.Errorf("unsupported MCP audio format %q: %w", c.MIMEType, err)
			}
			encoded := base64.StdEncoding.EncodeToString(c.Data)
			parts = append(parts, llmsdk.Part{AudioPart: &llmsdk.AudioPart{
				AudioData: encoded,
				Format:    format,
			}})
		default:
			// Skip content we cannot represent (e.g., resource links or embedded blobs).
		}
	}

	return parts, nil
}
