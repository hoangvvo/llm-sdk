package llmagent

import "context"

// Toolkit produces a per-session toolkit session that can provide dynamic prompt and tool data.
type Toolkit[C any] interface {
	// CreateSession creates a new toolkit session for the supplied context value.
	// Implementations should also initialize the session with any instructions or tools.
	CreateSession(ctx context.Context, contextVal C) (ToolkitSession[C], error)
}

// ToolkitSession exposes dynamically resolved tools and system prompt data for a run session.
type ToolkitSession[C any] interface {
	// SystemPrompt returns the current system prompt for the session if available.
	SystemPrompt() *string
	// Tools returns the current set of tools that should be available to the session.
	Tools() []AgentTool[C]
	// Close releases any resources that were allocated for the session.
	Close(ctx context.Context) error
}
