package llmagent

import (
	"context"
	"encoding/json"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

// Agent tool that can be used by the agent to perform specific tasks. Any
// struct that implements the `AgentTool` trait can be used as a tool.
type AgentTool[C any] interface {
	// Name of the tool.
	Name() string
	// A description of the tool to instruct the model how and when to use it.
	Description() string
	// The JSON schema of the parameters that the tool accepts. The type must
	// be "object".
	Parameters() llmsdk.JSONSchema
	// The function that will be called to execute the tool with given
	// parameters and context.
	//
	// If the tool returns an error, the agent will be interrupted and the error
	// will be propagated. To avoid interrupting the agent, the tool must
	// return an `AgentToolResult` with `is_error` set to true.
	Execute(ctx context.Context, params json.RawMessage, contextVal C, runState *RunState) (AgentToolResult, error)
}

type AgentToolResult struct {
	Content []llmsdk.Part `json:"content"`
	IsError bool          `json:"is_error"`
}
