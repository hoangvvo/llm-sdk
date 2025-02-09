package llmagent

import (
	"context"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

type Agent[C any] struct {
	Name             string
	model            llmsdk.LanguageModel
	instructions     []InstructionParam[C]
	tools            []AgentTool[C]
	responseFormat   llmsdk.ResponseFormatOption
	maxTurns         uint
	temperature      *float64
	topP             *float64
	topK             *float64
	presencePenalty  *float64
	frequencyPenalty *float64
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

// WithTemperature sets the sampling temperature for the model.
// Amount of randomness injected into the response. Ranges from 0.0 to 1.0
func WithTemperature[C any](temperature float64) AgentOption[C] {
	return func(a *Agent[C]) {
		a.temperature = &temperature
	}
}

// WithTopP sets the nucleus sampling parameter for the model.
// An alternative to sampling with temperature, called nucleus sampling,
// where the model considers the results of the tokens with top_p probability mass.
// Ranges from 0.0 to 1.0
func WithTopP[C any](topP float64) AgentOption[C] {
	return func(a *Agent[C]) {
		a.topP = &topP
	}
}

// WithTopK sets the top-k sampling parameter for the model.
// Only sample from the top K options for each subsequent token.
// Used to remove 'long tail' low probability responses.
func WithTopK[C any](topK float64) AgentOption[C] {
	return func(a *Agent[C]) {
		a.topK = &topK
	}
}

// WithPresencePenalty sets the presence penalty for the model.
// Positive values penalize new tokens based on whether they appear in the text so far,
// increasing the model's likelihood to talk about new topics.
func WithPresencePenalty[C any](presencePenalty float64) AgentOption[C] {
	return func(a *Agent[C]) {
		a.presencePenalty = &presencePenalty
	}
}

// WithFrequencyPenalty sets the frequency penalty for the model.
// Positive values penalize new tokens based on their existing frequency in the text so far,
// decreasing the model's likelihood to repeat the same line verbatim.
func WithFrequencyPenalty[C any](frequencyPenalty float64) AgentOption[C] {
	return func(a *Agent[C]) {
		a.frequencyPenalty = &frequencyPenalty
	}
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
	stream := session.RunStream(ctx, request)

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
	return NewRunSession(
		a.Name,
		a.model,
		a.instructions,
		a.tools,
		a.responseFormat,
		a.maxTurns,
		a.temperature,
		a.topP,
		a.topK,
		a.presencePenalty,
		a.frequencyPenalty,
	)
}
