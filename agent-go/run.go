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
	agentName        string
	tools            []AgentTool[C]
	model            llmsdk.LanguageModel
	responseFormat   *llmsdk.ResponseFormatOption
	instructions     []InstructionParam[C]
	maxTurns         uint
	temperature      *float64
	topP             *float64
	topK             *float64
	presencePenalty  *float64
	frequencyPenalty *float64
}

// NewRunSession creates a new run session and initializes dependencies
func NewRunSession[C any](
	agentName string,
	model llmsdk.LanguageModel,
	instructions []InstructionParam[C],
	tools []AgentTool[C],
	responseFormat *llmsdk.ResponseFormatOption,
	maxTurns uint,
	temperature *float64,
	topP *float64,
	topK *float64,
	presencePenalty *float64,
	frequencyPenalty *float64,
) *RunSession[C] {
	return &RunSession[C]{
		agentName:        agentName,
		tools:            tools,
		model:            model,
		responseFormat:   responseFormat,
		instructions:     instructions,
		maxTurns:         maxTurns,
		temperature:      temperature,
		topP:             topP,
		topK:             topK,
		presencePenalty:  presencePenalty,
		frequencyPenalty: frequencyPenalty,
	}
}

// process processes the model response and decide whether to continue the loop or
// return the response
func (s *RunSession[C]) process(
	ctx context.Context,
	contextVal C,
	runState *RunState,
	content []llmsdk.Part,
) (ProcessResult, error) {
	var toolCallParts []*llmsdk.ToolCallPart
	for _, part := range content {
		if part.ToolCallPart != nil {
			toolCallParts = append(toolCallParts, part.ToolCallPart)
		}
	}

	// If no tool calls were found, return the model response as is
	if len(toolCallParts) == 0 {
		return ProcessResult{
			Response: &content,
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

		toolRes, err := startActiveToolSpan(
			ctx,
			toolCallPart.ToolCallID,
			toolCallPart.ToolName,
			agentTool.Description(),
			func(ctx context.Context) (AgentToolResult, error) {
				res, err := agentTool.Execute(ctx, toolCallPart.Args, contextVal, runState)
				if err != nil {
					return AgentToolResult{}, NewToolExecutionError(err)
				}
				return res, nil
			},
		)
		if err != nil {
			return ProcessResult{}, err
		}

		toolMessage.Content = append(toolMessage.Content, llmsdk.Part{
			ToolResultPart: &llmsdk.ToolResultPart{
				ToolCallID: toolCallPart.ToolCallID,
				ToolName:   toolCallPart.ToolName,
				Content:    toolRes.Content,
				IsError:    toolRes.IsError,
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
	span, ctx := NewAgentSpan(ctx, s.agentName, "run")
	var err error
	defer func() {
		if err != nil {
			span.OnError(err)
		}
		span.OnEnd()
	}()

	state := NewRunState(request.Input, s.maxTurns)

	input, err := s.getLLMInput(ctx, request)
	if err != nil {
		return nil, err
	}
	contextVal := request.Context

	for {
		input.Messages = state.GetTurnMessages()
		var modelResponse *llmsdk.ModelResponse
		modelResponse, err = s.model.Generate(ctx, input)
		if err != nil {
			err = NewLanguageModelError(err)
			return nil, err
		}

		content := modelResponse.Content

		state.AppendMessages([]llmsdk.Message{llmsdk.NewAssistantMessage(content...)})

		state.AppendModelCall(ModelCallInfo{
			Usage:    modelResponse.Usage,
			Cost:     modelResponse.Cost,
			ModelID:  s.model.ModelID(),
			Provider: s.model.Provider(),
		})

		var result ProcessResult
		result, err = s.process(ctx, contextVal, state, content)
		if err != nil {
			return nil, err
		}

		if result.Response != nil {
			response := state.CreateResponse(*result.Response)
			span.OnResponse(response)
			return response, nil
		}

		if result.Next != nil {
			state.AppendMessages(*result.Next)
		}

		if err = state.Turn(); err != nil {
			return nil, err
		}
	}
}

// Run a streaming execution of the agent.
func (s *RunSession[C]) RunStream(ctx context.Context, request AgentRequest[C]) (*AgentStream, error) {
	span, ctx := NewAgentSpan(ctx, s.agentName, "run_stream")

	state := NewRunState(request.Input, s.maxTurns)

	input, err := s.getLLMInput(ctx, request)
	if err != nil {
		return nil, err
	}
	contextVal := request.Context

	eventChan := make(chan *AgentStreamEvent)
	errChan := make(chan error, 1)

	go func() {
		var err error
		defer close(eventChan)
		defer close(errChan)
		defer func() {
			if err != nil {
				span.OnError(err)
			}
			span.OnEnd()
		}()

		for {
			input.Messages = state.GetTurnMessages()

			var modelStream *llmsdk.LanguageModelStream
			modelStream, err = s.model.Stream(ctx, input)
			if err != nil {
				err := NewLanguageModelError(err)
				errChan <- err
				return
			}

			accumulator := llmsdk.NewStreamAccumulator()

			for modelStream.Next() {
				partial := modelStream.Current()

				if err = accumulator.AddPartial(*partial); err != nil {
					err = NewInvariantError(fmt.Sprintf("failed to accumulate stream: %v", err))
					errChan <- err
					return
				}

				eventChan <- &AgentStreamEvent{
					Partial: partial,
				}
			}

			if err = modelStream.Err(); err != nil {
				err := NewLanguageModelError(err)
				errChan <- err
				return
			}

			var modelResponse llmsdk.ModelResponse
			modelResponse, err = accumulator.ComputeResponse()
			if err != nil {
				errChan <- err
				return
			}

			content := modelResponse.Content

			assistantMessage := llmsdk.Message{
				AssistantMessage: &llmsdk.AssistantMessage{
					Content: content,
				},
			}

			state.AppendMessages([]llmsdk.Message{assistantMessage})

			state.AppendModelCall(ModelCallInfo{
				Usage:    modelResponse.Usage,
				Cost:     modelResponse.Cost,
				ModelID:  s.model.ModelID(),
				Provider: s.model.Provider(),
			})

			eventChan <- &AgentStreamEvent{
				Message: &assistantMessage,
			}

			var result ProcessResult
			result, err = s.process(ctx, contextVal, state, content)
			if err != nil {
				errChan <- err
				return
			}

			if result.Response != nil {
				response := state.CreateResponse(*result.Response)
				span.OnResponse(response)
				eventChan <- &AgentStreamEvent{
					Response: response,
				}
				return
			}

			if result.Next != nil {
				state.AppendMessages(*result.Next)
				for _, message := range *result.Next {
					eventChan <- &AgentStreamEvent{
						Message: &message,
					}
				}
			}

			if err = state.Turn(); err != nil {
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

func (s *RunSession[C]) getLLMInput(ctx context.Context, request AgentRequest[C]) (*llmsdk.LanguageModelInput, error) {
	// Convert AgentTool to SDK Tool
	var tools []llmsdk.Tool
	for _, tool := range s.tools {
		tools = append(tools, llmsdk.Tool{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  tool.Parameters(),
		})
	}

	systemPrompt, err := getPrompt(ctx, s.instructions, request.Context)
	if err != nil {
		return nil, NewInitError(err)
	}

	return &llmsdk.LanguageModelInput{
		// messages will be computed from getTurnMessages
		Messages:         nil,
		SystemPrompt:     &systemPrompt,
		Tools:            tools,
		ResponseFormat:   s.responseFormat,
		Temperature:      s.temperature,
		TopP:             s.topP,
		TopK:             s.topK,
		PresencePenalty:  s.presencePenalty,
		FrequencyPenalty: s.frequencyPenalty,
	}, nil
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
	maxTurns uint
	input    []AgentItem

	// CurrentTurn is the current turn number in the run.
	CurrentTurn uint
	// output contains all items generated during the run, such as new `ToolMessage` and
	// `AssistantMessage`
	output []AgentItem

	// modelCalls contain information about the LLM calls made during the run
	modelCalls []ModelCallInfo

	mu sync.RWMutex
}

func NewRunState(input []AgentItem, maxTurns uint) *RunState {
	return &RunState{
		maxTurns:    maxTurns,
		input:       input,
		CurrentTurn: 0,
		output:      []AgentItem{},
		modelCalls:  []ModelCallInfo{},
	}
}

// Turn marks a new turn in the conversation and throw an error if max turns
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

// AppendMessages adds messages to the run state.
func (s *RunState) AppendMessages(messages []llmsdk.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, message := range messages {
		s.output = append(s.output, AgentItem{
			Message: &message,
		})
	}
}

// AppendModelCall adds a model call to the run state.
func (s *RunState) AppendModelCall(call ModelCallInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.modelCalls = append(s.modelCalls, call)
}

// GetTurnMessages gets LLM messages to use in the `LanguageModelInput` for the turn
func (s *RunState) GetTurnMessages() []llmsdk.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	messages := make([]llmsdk.Message, 0, len(s.input)+len(s.output))
	for _, item := range s.input {
		if item.Message != nil {
			messages = append(messages, *item.Message)
		}
	}

	for _, item := range s.output {
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
		Content:    finalContent,
		Output:     s.output,
		ModelCalls: s.modelCalls,
	}
}
