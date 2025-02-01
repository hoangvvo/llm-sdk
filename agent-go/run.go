package llmagent

import (
	"context"
	"fmt"
	"sync"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

// RunSession manages the run session for an agent.
// It initializes all necessary components for the agent to run
// and handles the execution of the agent's tasks.
// Once finish, the session cleans up any resources used during the run.
// The session can be reused in multiple runs.
type RunSession[C any] struct {
	tools          []AgentTool[C]
	model          llmsdk.LanguageModel
	responseFormat llmsdk.ResponseFormatOption
	instructions   []InstructionParam[C]
	maxTurns       uint
	temperature    *float64
	topP           *float64
	topK           *float64
	presencePenalty *float64
	frequencyPenalty *float64
}

// NewRunSession creates a new run session and initializes dependencies
func NewRunSession[C any](
	model llmsdk.LanguageModel,
	instructions []InstructionParam[C],
	tools []AgentTool[C],
	responseFormat llmsdk.ResponseFormatOption,
	maxTurns uint,
	temperature *float64,
	topP *float64,
	topK *float64,
	presencePenalty *float64,
	frequencyPenalty *float64,
) *RunSession[C] {
	return &RunSession[C]{
		tools:          tools,
		model:          model,
		responseFormat: responseFormat,
		instructions:   instructions,
		maxTurns:       maxTurns,
		temperature:    temperature,
		topP:           topP,
		topK:           topK,
		presencePenalty: presencePenalty,
		frequencyPenalty: frequencyPenalty,
	}
}

// process processes the model response and decide whether to continue the loop or
// return the response
func (s *RunSession[C]) process(
	ctx context.Context,
	contextVal C,
	runState *RunState,
	modelResponse *llmsdk.ModelResponse,
) (ProcessResult, error) {
	var toolCallParts []*llmsdk.ToolCallPart
	for _, part := range modelResponse.Content {
		if part.ToolCallPart != nil {
			toolCallParts = append(toolCallParts, part.ToolCallPart)
		}
	}

	// If no tool calls were found, return the model response as is
	if len(toolCallParts) == 0 {
		return ProcessResult{
			Response: &modelResponse.Content,
		}, nil
	}

	var nextMessages []llmsdk.Message

	// Process all tool calls
	toolMessage := llmsdk.ToolMessage{
		Content: []llmsdk.Part{},
	}

	for _, toolCallPart := range toolCallParts {
		var agentTool AgentTool[C]
		for _, tool := range s.tools {
			if tool.Name() == toolCallPart.ToolName {
				agentTool = tool
				break
			}
		}

		if agentTool == nil {
			return ProcessResult{}, NewInvariantError(
				fmt.Sprintf("tool %s not found for tool call", toolCallPart.ToolName),
			)
		}

		toolRes, err := agentTool.Execute(ctx, toolCallPart.Args, contextVal, runState)
		if err != nil {
			return ProcessResult{}, NewToolExecutionError(err)
		}

		toolMessage.Content = append(toolMessage.Content, llmsdk.Part{
			ToolResultPart: &llmsdk.ToolResultPart{
				ToolCallID: toolCallPart.ToolCallID,
				ToolName:   toolCallPart.ToolName,
				Content:    toolRes.Content,
				IsError:    &toolRes.IsError,
			},
		})
	}

	nextMessages = append(nextMessages, llmsdk.Message{
		ToolMessage: &toolMessage,
	})

	return ProcessResult{
		Next: &nextMessages,
	}, nil
}

// Run runs a non-streaming execution of the agent.
func (s *RunSession[C]) Run(ctx context.Context, request AgentRequest[C]) (*AgentResponse, error) {
	state := NewRunState(request.Messages, s.maxTurns)

	input := s.getLLMInput(request)
	contextVal := request.Context

	for {
		input.Messages = state.GetTurnMessages()
		modelResponse, err := s.model.Generate(ctx, input)
		if err != nil {
			return nil, NewLanguageModelError(err)
		}

		state.AppendMessage(llmsdk.Message{
			AssistantMessage: &llmsdk.AssistantMessage{
				Content: modelResponse.Content,
			},
		})

		result, err := s.process(ctx, contextVal, state, modelResponse)
		if err != nil {
			return nil, err
		}

		if result.Response != nil {
			return state.CreateResponse(*result.Response), nil
		}

		if result.Next != nil {
			for _, message := range *result.Next {
				state.AppendMessage(message)
			}
		}

		if err := state.Turn(); err != nil {
			return nil, err
		}
	}
}

// Run a streaming execution of the agent.
func (s *RunSession[C]) RunStream(ctx context.Context, request AgentRequest[C]) (*AgentStream, error) {
	state := NewRunState(request.Messages, s.maxTurns)

	input := s.getLLMInput(request)
	contextVal := request.Context

	eventChan := make(chan *AgentStreamEvent)
	errChan := make(chan error, 1)

	go func() {
		defer close(eventChan)
		defer close(errChan)

		for {
			input.Messages = state.GetTurnMessages()

			modelStream, err := s.model.Stream(ctx, input)
			if err != nil {
				errChan <- NewLanguageModelError(err)
				return
			}

			accumulator := llmsdk.NewStreamAccumulator()

			for modelStream.Next() {
				partial := modelStream.Current()

				if err := accumulator.AddPartial(*partial); err != nil {
					errChan <- NewInvariantError(fmt.Sprintf("failed to accumulate stream: %v", err))
					return
				}

				eventChan <- &AgentStreamEvent{
					Partial: partial,
				}
			}

			if err := modelStream.Err(); err != nil {
				errChan <- NewLanguageModelError(err)
				return
			}

			modelResponse, err := accumulator.ComputeResponse()
			if err != nil {
				errChan <- err
				return
			}

			assistantMessage := llmsdk.Message{
				AssistantMessage: &llmsdk.AssistantMessage{
					Content: modelResponse.Content,
				},
			}

			state.AppendMessage(assistantMessage)
			eventChan <- &AgentStreamEvent{
				Message: &assistantMessage,
			}

			result, err := s.process(ctx, contextVal, state, &modelResponse)
			if err != nil {
				errChan <- err
				return
			}

			if result.Response != nil {
				response := state.CreateResponse(*result.Response)
				eventChan <- &AgentStreamEvent{
					Response: response,
				}
				return
			}

			if result.Next != nil {
				for _, message := range *result.Next {
					state.AppendMessage(message)
					eventChan <- &AgentStreamEvent{
						Message: &message,
					}
				}
			}

			if err := state.Turn(); err != nil {
				errChan <- err
				return
			}
		}
	}()

	return NewAgentStream(eventChan, errChan), nil
}

func (s *RunSession[C]) Finish() {
	// Cleanup dependencies if needed
}

func (s *RunSession[C]) getLLMInput(request AgentRequest[C]) *llmsdk.LanguageModelInput {
	// Convert AgentTool to SDK Tool
	var tools []llmsdk.Tool
	for _, tool := range s.tools {
		tools = append(tools, llmsdk.Tool{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  tool.Parameters(),
		})
	}

	systemPrompt := getPrompt(s.instructions, request.Context)

	return &llmsdk.LanguageModelInput{
		Messages:       request.Messages,
		SystemPrompt:   &systemPrompt,
		Tools:          tools,
		ResponseFormat: &s.responseFormat,
		Temperature:    s.temperature,
		TopP:           s.topP,
		TopK:           s.topK,
		PresencePenalty: s.presencePenalty,
		FrequencyPenalty: s.frequencyPenalty,
	}
}

// ProcessResult represents the result of processing a model response
// to decide whether to continue the loop or return the response.
// Only one of Response or Next should be set.
type ProcessResult struct {
	Response *[]llmsdk.Part
	// Return when new messages need to be added to the input and continue processing
	Next *[]llmsdk.Message
}

type RunState struct {
	maxTurns      uint
	inputMessages []llmsdk.Message

	// The current turn number in the run.
	CurrentTurn uint
	// All items generated during the run, such as new `ToolMessage` and
	// `AssistantMessage`
	Items []RunItem

	mu sync.RWMutex
}

func NewRunState(inputMessages []llmsdk.Message, maxTurns uint) *RunState {
	return &RunState{
		maxTurns:      maxTurns,
		inputMessages: inputMessages,
		CurrentTurn:   0,
		Items:         []RunItem{},
	}
}

// Mark a new turn in the conversation and throw an error if max turns
// exceeded.
func (s *RunState) Turn() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.CurrentTurn++
	if s.CurrentTurn > s.maxTurns {
		return NewMaxTurnsExceededError(int(s.maxTurns))
	}
	return nil
}

// Add a message to the run state.
func (s *RunState) AppendMessage(message llmsdk.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Items = append(s.Items, RunItem{
		Message: &message,
	})
}

// Get LLM messages to use in the `LanguageModelInput` for the turn
func (s *RunState) GetTurnMessages() []llmsdk.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	messages := make([]llmsdk.Message, len(s.inputMessages))
	copy(messages, s.inputMessages)

	for _, item := range s.Items {
		if item.Message != nil {
			messages = append(messages, *item.Message)
		}
	}

	return messages
}

func (s *RunState) CreateResponse(finalContent []llmsdk.Part) *AgentResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return &AgentResponse{
		Content: finalContent,
		Items:   s.Items,
	}
}
