package llmagent

import (
	"context"
	"encoding/json"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

// Agent function tool that can be executed by the agent runtime.
type AgentFunctionTool[C any] interface {
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

// AgentTool is the union of agent-executed function tools and provider-hosted
// tools forwarded to the model.
type AgentTool[C any] struct {
	FunctionTool AgentFunctionTool[C] `json:"-"`
	ProviderTool *llmsdk.ProviderTool `json:"-"`
}

func NewAgentFunctionTool[C any](tool AgentFunctionTool[C]) AgentTool[C] {
	return AgentTool[C]{FunctionTool: tool}
}

func NewAgentProviderTool[C any](tool llmsdk.ProviderTool) AgentTool[C] {
	return AgentTool[C]{ProviderTool: &tool}
}

func FunctionTools[C any](tools ...AgentFunctionTool[C]) []AgentTool[C] {
	agentTools := make([]AgentTool[C], 0, len(tools))
	for _, tool := range tools {
		agentTools = append(agentTools, NewAgentFunctionTool(tool))
	}
	return agentTools
}

func ProviderTools[C any](tools ...llmsdk.ProviderTool) []AgentTool[C] {
	agentTools := make([]AgentTool[C], 0, len(tools))
	for _, tool := range tools {
		agentTools = append(agentTools, NewAgentProviderTool[C](tool))
	}
	return agentTools
}

func (t AgentTool[C]) ToLanguageModelTool() llmsdk.Tool {
	if t.FunctionTool != nil {
		return llmsdk.NewFunctionTool(
			t.FunctionTool.Name(),
			t.FunctionTool.Description(),
			t.FunctionTool.Parameters(),
		)
	}
	if t.ProviderTool != nil {
		return llmsdk.Tool{ProviderTool: t.ProviderTool}
	}
	return llmsdk.Tool{}
}

func (t AgentTool[C]) AsFunctionTool() AgentFunctionTool[C] {
	return t.FunctionTool
}

type AgentToolResult struct {
	Content []llmsdk.Part `json:"content"`
	IsError bool          `json:"is_error"`
}
