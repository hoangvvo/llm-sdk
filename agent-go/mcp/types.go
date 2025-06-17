package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// MCPInit resolves per-session MCP configuration. Use StaticMCPInit when the
// target server is fixed, or provide a resolver that inspects the agent context
// (e.g. to inject user-specific credentials).
type MCPInit[C any] func(ctx context.Context, contextVal C) (MCPParams, error)

// StaticMCPInit returns an MCPInit that always yields the same parameters.
func StaticMCPInit[C any](params MCPParams) MCPInit[C] {
	return func(context.Context, C) (MCPParams, error) {
		return params, nil
	}
}

// MCPParams describes how to reach an MCP server. Exactly one variant should be set.
type MCPParams struct {
	stdio          *MCPStdioParams
	streamableHTTP *MCPStreamableHTTPParams
}

// MCPStdioParams executes a local MCP server over stdio.
type MCPStdioParams struct {
	// Command is the executable to launch (e.g. "uvx").
	Command string `json:"command"`
	// Args are optional arguments passed to the command.
	Args []string `json:"args,omitempty"`
}

// MCPStreamableHTTPParams connects to a remote MCP server using the streamable HTTP transport.
type MCPStreamableHTTPParams struct {
	// URL is the base endpoint of the MCP server.
	URL string `json:"url"`
	// Authorization is an optional header value. OAuth flows are not handled automatically,
	// so callers should supply a ready-to-use token when required.
	Authorization string `json:"authorization,omitempty"`
}

const (
	paramTypeStdio          = "stdio"
	paramTypeStreamableHTTP = "streamable-http"
)

// NewMCPStdioParams constructs an MCPParams pointing at a stdio server.
func NewMCPStdioParams(command string, args []string) MCPParams {
	return MCPParams{stdio: &MCPStdioParams{Command: command, Args: args}}
}

// NewMCPStreamableHTTPParams constructs an MCPParams pointing at a streamable HTTP server.
func NewMCPStreamableHTTPParams(url, authorization string) MCPParams {
	return MCPParams{streamableHTTP: &MCPStreamableHTTPParams{URL: url, Authorization: authorization}}
}

// StdioParams returns the stdio configuration if this MCPParams targets a local process.
func (p MCPParams) StdioParams() (*MCPStdioParams, bool) {
	if p.stdio == nil {
		return nil, false
	}
	return p.stdio, true
}

// StreamableHTTPParams returns the streamable HTTP configuration if applicable.
func (p MCPParams) StreamableHTTPParams() (*MCPStreamableHTTPParams, bool) {
	if p.streamableHTTP == nil {
		return nil, false
	}
	return p.streamableHTTP, true
}

func (p MCPParams) isZero() bool {
	return p.stdio == nil && p.streamableHTTP == nil
}

// MarshalJSON encodes the MCPParams using a discriminated union.
func (p MCPParams) MarshalJSON() ([]byte, error) {
	switch {
	case p.stdio != nil:
		type alias struct {
			Type string `json:"type"`
			*MCPStdioParams
		}
		return json.Marshal(alias{Type: paramTypeStdio, MCPStdioParams: p.stdio})
	case p.streamableHTTP != nil:
		type alias struct {
			Type string `json:"type"`
			*MCPStreamableHTTPParams
		}
		return json.Marshal(alias{Type: paramTypeStreamableHTTP, MCPStreamableHTTPParams: p.streamableHTTP})
	default:
		return nil, errors.New("mcp params missing variant")
	}
}

// UnmarshalJSON decodes the discriminated union into the appropriate MCPParams variant.
func (p *MCPParams) UnmarshalJSON(data []byte) error {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return fmt.Errorf("decode MCP params discriminator: %w", err)
	}

	switch probe.Type {
	case paramTypeStdio:
		var payload struct {
			*MCPStdioParams
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			return fmt.Errorf("decode MCP stdio params: %w", err)
		}
		if payload.MCPStdioParams == nil || payload.Command == "" {
			return errors.New("mcp stdio params missing command")
		}
		p.stdio = payload.MCPStdioParams
		p.streamableHTTP = nil
		return nil
	case paramTypeStreamableHTTP:
		var payload struct {
			*MCPStreamableHTTPParams
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			return fmt.Errorf("decode MCP streamable-http params: %w", err)
		}
		if payload.MCPStreamableHTTPParams == nil || payload.URL == "" {
			return errors.New("mcp streamable-http params missing url")
		}
		p.streamableHTTP = payload.MCPStreamableHTTPParams
		p.stdio = nil
		return nil
	default:
		return fmt.Errorf("unknown mcp params type %q", probe.Type)
	}
}
