package llmagent

import llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"

// Parameters required to create a new agent.
type AgentParams[C any] struct {
	Name string
	// The default language model to use for the agent.
	Model llmsdk.LanguageModel
	// Instructions to be added to system messages when executing the agent.
	// This can include formatting instructions or other guidance for the
	// agent.
	Instructions []InstructionParam[C]
	// The tools that the agent can use to perform tasks.
	Tools []AgentTool[C]
	// Optional toolkits that can provide dynamic tools and system prompts for each session.
	Toolkits []Toolkit[C]
	// The expected format of the response. Either text or structured output.
	ResponseFormat *llmsdk.ResponseFormatOption
	// Max number of turns for agent to run to protect against infinite loops.
	MaxTurns uint
	// Amount of randomness injected into the response.
	Temperature *float64
	// An alternative to sampling with temperature, called nucleus sampling,
	// where the model considers the results of the tokens with `top_p`
	// probability mass.
	TopP *float64
	// Only sample from the top K options for each subsequent token.
	// Used to remove 'long tail' low probability responses.
	// Must be a non-negative integer.
	TopK *int32
	// Positive values penalize new tokens based on whether they appear in the
	// text so far, increasing the model's likelihood to talk about new
	// topics.
	PresencePenalty *float64
	// Positive values penalize new tokens based on their existing frequency in
	// the text so far, decreasing the model's likelihood to repeat the
	// same line verbatim.
	FrequencyPenalty *float64
	// The modalities that the model should support.
	Modalities []llmsdk.Modality
	// Options for audio generation.
	Audio *llmsdk.AudioOptions
	// Options for reasoning generation.
	Reasoning *llmsdk.ReasoningOptions
}

type AgentParamsOption[C any] func(*AgentParams[C])

// WithInstructions sets the instructions to be added to system messages when executing the agent.
// This can include formatting instructions or other guidance for the agent.
func WithInstructions[C any](instructions ...InstructionParam[C]) AgentParamsOption[C] {
	return func(p *AgentParams[C]) {
		p.Instructions = instructions
	}
}

// WithTools sets the tools that the agent can use to perform tasks.
func WithTools[C any](tools ...AgentTool[C]) AgentParamsOption[C] {
	return func(p *AgentParams[C]) {
		p.Tools = tools
	}
}

// WithToolkits sets the toolkits that can provide dynamic tools and prompts per session.
func WithToolkits[C any](toolkits ...Toolkit[C]) AgentParamsOption[C] {
	return func(p *AgentParams[C]) {
		p.Toolkits = toolkits
	}
}

// WithResponseFormat sets the expected format of the response. Either text or structured output.
func WithResponseFormat[C any](format llmsdk.ResponseFormatOption) AgentParamsOption[C] {
	return func(p *AgentParams[C]) {
		p.ResponseFormat = &format
	}
}

// WithMaxTurns sets the max number of turns for agent to run to protect against infinite loops.
func WithMaxTurns[C any](maxTurns uint) AgentParamsOption[C] {
	return func(p *AgentParams[C]) {
		p.MaxTurns = maxTurns
	}
}

// WithTemperature sets the sampling temperature for the model.
// Amount of randomness injected into the response. Ranges from 0.0 to 1.0
func WithTemperature[C any](temperature float64) AgentParamsOption[C] {
	return func(p *AgentParams[C]) {
		p.Temperature = &temperature
	}
}

// WithTopP sets the nucleus sampling parameter for the model.
// An alternative to sampling with temperature, called nucleus sampling,
// where the model considers the results of the tokens with top_p probability mass.
// Ranges from 0.0 to 1.0
func WithTopP[C any](topP float64) AgentParamsOption[C] {
	return func(p *AgentParams[C]) {
		p.TopP = &topP
	}
}

// WithTopK sets the top-k sampling parameter for the model.
// Only sample from the top K options for each subsequent token.
// Used to remove 'long tail' low probability responses.
// Must be a non-negative integer.
func WithTopK[C any](topK int32) AgentParamsOption[C] {
	return func(p *AgentParams[C]) {
		p.TopK = &topK
	}
}

// WithPresencePenalty sets the presence penalty for the model.
// Positive values penalize new tokens based on whether they appear in the text so far,
// increasing the model's likelihood to talk about new topics.
func WithPresencePenalty[C any](presencePenalty float64) AgentParamsOption[C] {
	return func(p *AgentParams[C]) {
		p.PresencePenalty = &presencePenalty
	}
}

// WithFrequencyPenalty sets the frequency penalty for the model.
// Positive values penalize new tokens based on their existing frequency in the text so far,
// decreasing the model's likelihood to repeat the same line verbatim.
func WithFrequencyPenalty[C any](frequencyPenalty float64) AgentParamsOption[C] {
	return func(p *AgentParams[C]) {
		p.FrequencyPenalty = &frequencyPenalty
	}
}

// WithModalities sets the modalities that the model should support.
func WithModalities[C any](modalities ...llmsdk.Modality) AgentParamsOption[C] {
	return func(p *AgentParams[C]) {
		p.Modalities = modalities
	}
}

// WithAudio sets the options for audio generation.
func WithAudio[C any](audioOptions llmsdk.AudioOptions) AgentParamsOption[C] {
	return func(p *AgentParams[C]) {
		p.Audio = &audioOptions
	}
}

// WithReasoning sets the options for reasoning generation.
func WithReasoning[C any](reasoningOptions llmsdk.ReasoningOptions) AgentParamsOption[C] {
	return func(p *AgentParams[C]) {
		p.Reasoning = &reasoningOptions
	}
}
