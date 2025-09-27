package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// toolkit wires the MCP integration into the llmagent toolkit primitive so the agent can hydrate remote tools on demand.
type toolkit[C any] struct {
	init MCPInit[C]
}

// NewMCPToolkit returns an implementation of llmagent.Toolkit that sources tools from the Model Context Protocol.
// The init resolver can inspect the agent context (e.g., pull user-specific auth data) before the session connects.
func NewMCPToolkit[C any](init MCPInit[C]) llmagent.Toolkit[C] {
	return &toolkit[C]{init: init}
}

// CreateSession resolves the per-run MCP configuration and bootstraps an MCP-backed toolkit session.
func (t *toolkit[C]) CreateSession(ctx context.Context, contextVal C) (llmagent.ToolkitSession[C], error) {
	params, err := t.init(ctx, contextVal)
	if err != nil {
		return nil, fmt.Errorf("resolve MCP params: %w", err)
	}
	if params.isZero() {
		return nil, errors.New("mcp params missing variant")
	}

	session, err := newToolkitSession[C](ctx, params)
	if err != nil {
		return nil, err
	}
	return session, nil
}

// toolkitSession bridges an MCP client session into the agent runtime.
type toolkitSession[C any] struct {
	client    *mcp.Client
	transport mcp.Transport
	session   *mcp.ClientSession

	mu sync.RWMutex
	// tools caches the latest snapshot surfaced to the agent runtime.
	tools []llmagent.AgentTool[C]
	// toolListErr records asynchronous discovery failures when the MCP server publishes on-demand tool list changes.
	toolListErr error
}

// newToolkitSession prepares transport + client scaffolding and completes the MCP handshake.
func newToolkitSession[C any](ctx context.Context, params MCPParams) (*toolkitSession[C], error) {
	transport, err := buildTransport(params)
	if err != nil {
		return nil, err
	}
	s := &toolkitSession[C]{
		transport: transport,
		tools:     make([]llmagent.AgentTool[C], 0),
	}
	clientOpts := &mcp.ClientOptions{
		ToolListChangedHandler: func(ctx context.Context, _ *mcp.ToolListChangedRequest) {
			_ = s.reloadTools(ctx)
		},
	}
	s.client = mcp.NewClient(&mcp.Implementation{Name: "llm-agent-go", Version: "0.1.0"}, clientOpts)

	if err := s.initialize(ctx); err != nil {
		_ = s.Close(ctx)
		return nil, err
	}

	return s, nil
}

// initialize connects to the MCP server, hydrates the initial tool snapshot, and wires change notifications.
func (s *toolkitSession[C]) initialize(ctx context.Context) error {
	// TODO: mcp.ClientSession uses the same context for the lifetime of the session.
	// so we need to use context.Background() here.
	clientSession, err := s.client.Connect(context.Background(), s.transport, nil)
	if err != nil {
		return fmt.Errorf("connect MCP client: %w", err)
	}
	s.session = clientSession

	if err := s.reloadTools(ctx); err != nil {
		return err
	}
	return nil
}

// SystemPrompt keeps parity with the Toolkit contract; MCP does not expose instructions so we return nil.
func (s *toolkitSession[C]) SystemPrompt() *string {
	return nil
}

// Tools exposes the latest cached tool list, surfacing asynchronous discovery errors immediately.
func (s *toolkitSession[C]) Tools() []llmagent.AgentTool[C] {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.toolListErr != nil {
		panic(fmt.Errorf("mcp tool discovery failed: %w", s.toolListErr))
	}

	out := make([]llmagent.AgentTool[C], len(s.tools))
	// Copy the tools into the output slice so we don't leak the internal slice.
	copy(out, s.tools)
	return out
}

// Close tears down the MCP client session when the toolkit session ends.
func (s *toolkitSession[C]) Close(ctx context.Context) error {
	if s.session != nil {
		if err := s.session.Close(); err != nil {
			return fmt.Errorf("close MCP session: %w", err)
		}
	}
	return nil
}

// reloadTools refreshes the cached tool list and records any discovery failures.
func (s *toolkitSession[C]) reloadTools(ctx context.Context) error {
	if s.session == nil {
		return fmt.Errorf("mcp session not initialised")
	}

	tools, err := s.fetchTools(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()

	if err != nil {
		s.toolListErr = err
		return err
	}

	s.tools = tools
	s.toolListErr = nil
	return nil
}

// fetchTools walks the MCP pagination API to build the full AgentTool collection.
func (s *toolkitSession[C]) fetchTools(ctx context.Context) ([]llmagent.AgentTool[C], error) {
	var (
		cursor    *string
		collected []llmagent.AgentTool[C]
	)

	for {
		var params *mcp.ListToolsParams
		if cursor != nil {
			params = &mcp.ListToolsParams{Cursor: *cursor}
		}

		result, err := s.session.ListTools(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("list MCP tools: %w", err)
		}

		for _, tool := range result.Tools {
			agentTool, convErr := s.toAgentTool(tool)
			if convErr != nil {
				return nil, convErr
			}
			collected = append(collected, agentTool)
		}

		if result.NextCursor == "" {
			break
		}
		cursor = &result.NextCursor
	}

	return collected, nil
}

// toAgentTool converts an MCP tool definition into the llmagent AgentTool abstraction.
func (s *toolkitSession[C]) toAgentTool(tool *mcp.Tool) (llmagent.AgentTool[C], error) {
	schema := llmsdk.JSONSchema{}
	if tool.InputSchema != nil {
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("serialise MCP tool schema for %s: %w", tool.Name, err)
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			return nil, fmt.Errorf("decode MCP tool schema for %s: %w", tool.Name, err)
		}
	}

	return &agentTool[C]{
		session:     s.session,
		name:        tool.Name,
		description: tool.Description,
		parameters:  schema,
	}, nil
}

type agentTool[C any] struct {
	session     *mcp.ClientSession
	name        string
	description string
	parameters  llmsdk.JSONSchema
}

// Name returns the remote tool identifier.
func (t *agentTool[C]) Name() string {
	return t.name
}

// Description surfaces the remote description to the model.
func (t *agentTool[C]) Description() string {
	return t.description
}

// Parameters returns the JSON schema the remote tool provided.
func (t *agentTool[C]) Parameters() llmsdk.JSONSchema {
	return t.parameters
}

// Execute forwards the agent call to the MCP server and adapts the response into llmagent parts.
func (t *agentTool[C]) Execute(ctx context.Context, params json.RawMessage, _ C, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var arguments map[string]any
	if len(params) == 0 {
		arguments = map[string]any{}
	} else {
		if err := json.Unmarshal(params, &arguments); err != nil {
			return llmagent.AgentToolResult{}, fmt.Errorf("decode MCP tool args for %s: %w", t.name, err)
		}
	}

	result, err := t.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      t.name,
		Arguments: arguments,
	})
	if err != nil {
		return llmagent.AgentToolResult{}, fmt.Errorf("call MCP tool %s: %w", t.name, err)
	}

	parts, err := convertMCPContentToParts(result.Content)
	if err != nil {
		return llmagent.AgentToolResult{}, err
	}

	return llmagent.AgentToolResult{
		Content: parts,
		IsError: result.IsError,
	}, nil
}
