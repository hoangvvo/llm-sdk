package llmagent

import (
	"context"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/stream"
)

type Agent[C any] struct {
	Name   string
	params *AgentParams[C]
}

// NewAgent creates a new agent with given name, language model, and options.
//
// Defaults:
// - `instructions`: empty
// - `tools`: empty
// - `responseFormat`: `llmsdk.NewResponseFormatText()`
// - `maxTurns`: 10
// - `temperature`: nil
// - `topP`: nil
// - `topK`: nil
// - `presencePenalty`: nil
// - `frequencyPenalty`: nil
func NewAgent[C any](name string, model llmsdk.LanguageModel, options ...AgentParamsOption[C]) *Agent[C] {
	params := &AgentParams[C]{
		Name:           name,
		Model:          model,
		Instructions:   []InstructionParam[C]{},
		Tools:          []AgentTool[C]{},
		Toolkits:       []Toolkit[C]{},
		ResponseFormat: llmsdk.NewResponseFormatText(),
		MaxTurns:       10,
	}

	for _, option := range options {
		option(params)
	}

	return &Agent[C]{Name: name, params: params}
}

// Run creates a one-time run of the agent and generates a response.
// A session is created for the run and cleaned up afterwards.
func (a *Agent[C]) Run(ctx context.Context, request AgentRequest[C]) (*AgentResponse, error) {
	session, err := a.CreateSession(ctx, request.Context)
	if err != nil {
		return nil, err
	}
	result, runErr := session.Run(ctx, RunSessionRequest{Input: request.Input})
	closeErr := session.Close(ctx)
	if runErr != nil {
		return nil, runErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return result, nil
}

// RunStream creates a one-time streaming run of the agent and generates a response.
// A session is created for the run and cleaned up afterwards.
func (a *Agent[C]) RunStream(ctx context.Context, request AgentRequest[C]) (*AgentStream, error) {
	session, err := a.CreateSession(ctx, request.Context)
	if err != nil {
		return nil, err
	}
	agentStream, err := session.RunStream(ctx, RunSessionRequest{Input: request.Input})
	if err != nil {
		_ = session.Close(ctx)
		return nil, err
	}

	eventChan := make(chan *AgentStreamEvent)
	errChan := make(chan error, 1)

	go func() {
		defer close(eventChan)
		defer close(errChan)

		var streamErr error
		defer func() {
			if closeErr := session.Close(ctx); closeErr != nil && streamErr == nil {
				errChan <- closeErr
			}
		}()

		for agentStream.Next() {
			event := agentStream.Current()
			eventChan <- event
		}
		if streamErr = agentStream.Err(); streamErr != nil {
			errChan <- streamErr
			return
		}
	}()
	return stream.New(eventChan, errChan), nil
}

// CreateSession creates an initialized session for stateful multiple runs of the agent using the provided context value.
func (a *Agent[C]) CreateSession(ctx context.Context, contextVal C) (*RunSession[C], error) {
	return NewRunSession(ctx, a.params, contextVal)
}
