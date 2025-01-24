package llmagent

import (
	"context"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

type Agent[C any] struct {
	Name           string
	model          llmsdk.LanguageModel
	instructions   []InstructionParam[C]
	tools          []AgentTool[C]
	responseFormat llmsdk.ResponseFormatOption
	maxTurns       uint
}

// NewAgent creates a new agent with given name, language model, and options.
//
// Defaults:
// - `instructions`: empty
// - `tools`: empty
// - `responseFormat`: `llmsdk.NewResponseFormatText()`
// - `maxTurns`: 10
func NewAgent[C any](name string, model llmsdk.LanguageModel, options ...AgentOption[C]) *Agent[C] {
	agent := &Agent[C]{
		Name:           name,
		model:          model,
		instructions:   []InstructionParam[C]{},
		tools:          []AgentTool[C]{},
		responseFormat: llmsdk.NewResponseFormatText(),
		maxTurns:       10,
	}

	for _, option := range options {
		option(agent)
	}

	return agent
}

type AgentOption[C any] func(*Agent[C])

// WithInstructions sets the instructions to be added to system messages when executing the agent.
// This can include formatting instructions or other guidance for the agent.
func WithInstructions[C any](instructions ...InstructionParam[C]) AgentOption[C] {
	return func(a *Agent[C]) {
		a.instructions = instructions
	}
}

// WithTools sets the tools that the agent can use to perform tasks.
func WithTools[C any](tools ...AgentTool[C]) AgentOption[C] {
	return func(a *Agent[C]) {
		a.tools = tools
	}
}

// WithResponseFormat sets the expected format of the response. Either text or structured output.
func WithResponseFormat[C any](format llmsdk.ResponseFormatOption) AgentOption[C] {
	return func(a *Agent[C]) {
		a.responseFormat = format
	}
}

// WithMaxTurns sets the max number of turns for agent to run to protect against infinite loops.
func WithMaxTurns[C any](maxTurns uint) AgentOption[C] {
	return func(a *Agent[C]) {
		a.maxTurns = maxTurns
	}
}

// Run executes the agent with the given request and returns the response.
func (a *Agent[C]) Run(ctx context.Context, request AgentRequest[C]) (*AgentResponse, error) {
	session := NewRunSession(
		a.model,
		a.instructions,
		a.tools,
		a.responseFormat,
		a.maxTurns,
	)
	return session.Run(ctx, request)
}

// RunStream executes the agent with the given request and returns a stream of events.
func (a *Agent[C]) RunStream(ctx context.Context, request AgentRequest[C]) (*AgentStream, error) {
	session := NewRunSession(
		a.model,
		a.instructions,
		a.tools,
		a.responseFormat,
		a.maxTurns,
	)
	return session.RunStream(ctx, request)
}
