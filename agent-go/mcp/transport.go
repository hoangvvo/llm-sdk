package mcp

import (
	"errors"
	"net/http"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// buildTransport constructs the MCP transport (stdio or streamable HTTP) based on the resolved params.
func buildTransport(params MCPParams) (mcp.Transport, error) {
	if stdio, ok := params.StdioParams(); ok {
		if stdio.Command == "" {
			return nil, errors.New("mcp stdio command cannot be empty")
		}
		cmd := exec.Command(stdio.Command, stdio.Args...)
		return &mcp.CommandTransport{Command: cmd}, nil
	}

	if httpParams, ok := params.StreamableHTTPParams(); ok {
		if httpParams.URL == "" {
			return nil, errors.New("mcp streamable-http url cannot be empty")
		}
		transport := &mcp.StreamableClientTransport{
			Endpoint: httpParams.URL,
		}
		if token := strings.TrimSpace(httpParams.Authorization); token != "" {
			client := &http.Client{Transport: &authHeaderRoundTripper{
				base:  http.DefaultTransport,
				value: ensureBearerPrefix(token),
			}}
			transport.HTTPClient = client
		}
		return transport, nil
	}

	return nil, errors.New("unsupported mcp params variant")
}

// authHeaderRoundTripper injects an Authorization header because the Go MCP SDK does not yet expose
// a helper for bearer tokens on the streamable HTTP client.
type authHeaderRoundTripper struct {
	base  http.RoundTripper
	value string
}

func (rt *authHeaderRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", rt.value)
	base := rt.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(clone)
}

// ensureBearerPrefix normalises tokens so the Authorization header always carries the Bearer prefix.
func ensureBearerPrefix(token string) string {
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return token
	}
	return "Bearer " + token
}
