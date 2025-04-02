package llmagent

import (
	"context"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
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
	session := a.CreateSession()
	defer session.Finish()
	result, err := session.Run(ctx, request)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// RunStream creates a one-time streaming run of the agent and generates a response.
// A session is created for the run and cleaned up afterwards.
func (a *Agent[C]) RunStream(ctx context.Context, request AgentRequest[C]) (*AgentStream, error) {
	session := a.CreateSession()
	stream, err := session.RunStream(ctx, request)
	if err != nil {
		return nil, err
	}

	eventChan := make(chan *AgentStreamEvent)
	errChan := make(chan error, 1)

	go func() {
		defer close(eventChan)
		defer close(errChan)
		defer session.Finish()

		for {
			select {
			case event, ok := <-stream.C:
				if !ok {
					return
				}
				eventChan <- event
			case err, ok := <-stream.errC:
				if !ok {
					return
				}
				errChan <- err
				return
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			}
		}
	}()
	return NewAgentStream(eventChan, errChan), nil
}

func (a *Agent[C]) CreateSession() *RunSession[C] {
	return NewRunSession(a.params)
}
