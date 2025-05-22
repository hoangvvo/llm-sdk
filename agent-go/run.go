package llmagent

import (
	"context"
	"fmt"
	"slices"
	"sync"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/stream"
)

// RunSession manages the run session for an agent.
// It initializes all necessary components for the agent to run
// and handles the execution of the agent's tasks.
// Once finished, the session cleans up any resources used during the run.
//
// The session can be reused in multiple runs. RunSession binds to a specific
// context value that is used when resolving instructions and invoking tools,
// while input items remain per run and are supplied to each invocation.
type RunSession[C any] struct {
	params       *AgentParams[C] // RunSession params stores the agent configuration used during the run.
	contextVal   C               // RunSession contextVal is the bound context value used for instructions and tool executions.
	systemPrompt *string         // RunSession systemPrompt caches the resolved instructions as a system prompt.
	initialized  bool            // RunSession initialized ensures the session is ready before running.
}

// NewRunSession creates a new run session, resolves instructions, and initializes dependencies.
func NewRunSession[C any](
	ctx context.Context,
	params *AgentParams[C],
	contextVal C,
) (*RunSession[C], error) {
	session := &RunSession[C]{
		params:     params,
		contextVal: contextVal,
	}

	if err := session.initialize(ctx); err != nil {
		return nil, err
	}

	return session, nil
}

func (s *RunSession[C]) initialize(ctx context.Context) error {
	if len(s.params.Instructions) > 0 {
		prompt, err := getPrompt(ctx, s.params.Instructions, s.contextVal)
		if err != nil {
			return NewInitError(err)
		}
		s.systemPrompt = &prompt
	}

	s.initialized = true
	return nil
}

// process processes the model response and decide whether to continue the loop or
// return the response
func (s *RunSession[C]) process(
	ctx context.Context,
	runState *RunState,
	content []llmsdk.Part,
) *stream.Stream[ProcessEvents] {
	var toolCallParts []*llmsdk.ToolCallPart
	for _, part := range content {
		if part.ToolCallPart != nil {
			toolCallParts = append(toolCallParts, part.ToolCallPart)
		}
	}

	currCh := make(chan ProcessEvents)
	errCh := make(chan error, 1)

	go func() {
		defer close(currCh)
		defer close(errCh)

		// If no tool calls were found, return the model response as is
		if len(toolCallParts) == 0 {
			currCh <- ProcessEvents{
				Response: &content,
			}
			return
		}

		// Build AgentItem tool results for each tool call

		for _, toolCallPart := range toolCallParts {
			var agentTool AgentTool[C]
			for _, tool := range s.params.Tools {
				if tool.Name() == toolCallPart.ToolName {
					agentTool = tool
					break
				}
			}

			if agentTool == nil {
				errCh <- NewInvariantError(
					fmt.Sprintf("tool %s not found for tool call", toolCallPart.ToolName),
				)
				return
			}

			toolRes, err := startActiveToolSpan(
				ctx,
				toolCallPart.ToolCallID,
				toolCallPart.ToolName,
				agentTool.Description(),
				func(ctx context.Context) (AgentToolResult, error) {
					res, err := agentTool.Execute(ctx, toolCallPart.Args, s.contextVal, runState)
					if err != nil {
						return AgentToolResult{}, NewToolExecutionError(err)
					}
					return res, nil
				},
			)
			if err != nil {
				errCh <- err
				return
			}

			item := NewAgentItemTool(
				toolCallPart.ToolCallID,
				toolCallPart.ToolName,
				toolCallPart.Args,
				toolRes.Content,
				toolRes.IsError,
			)
			currCh <- ProcessEvents{
				Item: &item,
			}
		}
	}()

	return stream.New(currCh, errCh)
}

// Run runs a non-streaming execution of the agent.
func (s *RunSession[C]) Run(ctx context.Context, request RunSessionRequest) (*AgentResponse, error) {
	if !s.initialized {
		return nil, NewInvariantError("run session not initialized")
	}

	span, ctx := NewAgentSpan(ctx, s.params.Name, "run")
	var err error
	defer func() {
		if err != nil {
			span.OnError(err)
		}
		span.OnEnd()
	}()

	state := NewRunState(request.Input, s.params.MaxTurns)

	input := s.getLLMInput()

	for {
		input.Messages = state.GetTurnMessages()
		var modelResponse *llmsdk.ModelResponse
		modelResponse, err = s.params.Model.Generate(ctx, input)
		if err != nil {
			err = NewLanguageModelError(err)
			return nil, err
		}

		content := modelResponse.Content

		state.AppendModelResponse(*modelResponse)

		processStream := s.process(ctx, state, content)
		for processStream.Next() {
			event := processStream.Current()
			if event.Response != nil {
				response := state.CreateResponse(*event.Response)
				span.OnResponse(response)
				return response, nil
			}
			if event.Item != nil {
				state.AppendItem(*event.Item)
			}
			if event.Next != nil {
				// continue to next iteration
				break
			}
		}
		if err = processStream.Err(); err != nil {
			return nil, err
		}

		if err = state.Turn(); err != nil {
			return nil, err
		}
	}
}

// RunStream runs a streaming execution of the agent.
func (s *RunSession[C]) RunStream(ctx context.Context, request RunSessionRequest) (*AgentStream, error) {
	if !s.initialized {
		return nil, NewInvariantError("run session not initialized")
	}

	span, ctx := NewAgentSpan(ctx, s.params.Name, "run_stream")

	state := NewRunState(request.Input, s.params.MaxTurns)

	input := s.getLLMInput()

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
			modelStream, err = s.params.Model.Stream(ctx, input)
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

			item, index := state.AppendModelResponse(modelResponse)

			eventChan <- NewAgentStreamItemEvent(index, item)

			processStream := s.process(ctx, state, content)
			for processStream.Next() {
				event := processStream.Current()
				if event.Response != nil {
					response := state.CreateResponse(*event.Response)
					span.OnResponse(response)
					eventChan <- &AgentStreamEvent{
						Response: response,
					}
					return
				}
				if event.Item != nil {
					index := state.AppendItem(*event.Item)
					eventChan <- NewAgentStreamItemEvent(index, *event.Item)
				}
				if event.Next != nil {
					// continue to next iteration
					break
				}
			}
			if err = processStream.Err(); err != nil {
				errChan <- err
				return
			}

			if err = state.Turn(); err != nil {
				errChan <- err
				return
			}
		}
	}()

	return stream.New(eventChan, errChan), nil
}

func (s *RunSession[C]) Finish() {
	s.initialized = false
	s.systemPrompt = nil
}

func (s *RunSession[C]) getLLMInput() *llmsdk.LanguageModelInput {
	// Convert AgentTool to SDK Tool
	var tools []llmsdk.Tool
	for _, tool := range s.params.Tools {
		tools = append(tools, llmsdk.Tool{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  tool.Parameters(),
		})
	}

	return &llmsdk.LanguageModelInput{
		// messages will be computed from getTurnMessages
		Messages:         nil,
		SystemPrompt:     s.systemPrompt,
		Tools:            tools,
		ResponseFormat:   s.params.ResponseFormat,
		Temperature:      s.params.Temperature,
		TopP:             s.params.TopP,
		TopK:             s.params.TopK,
		PresencePenalty:  s.params.PresencePenalty,
		FrequencyPenalty: s.params.FrequencyPenalty,
		Modalities:       s.params.Modalities,
		Audio:            s.params.Audio,
		Reasoning:        s.params.Reasoning,
	}
}

// RunSessionRequest contains the input used for a run.
type RunSessionRequest struct {
	// Input holds the items to seed the run, such as LLM messages.
	Input []AgentItem
}

// ProcessEvents represents the sum type of events returned by the process function.
type ProcessEvents struct {
	// Emit when a new item is generated
	Item *AgentItem
	// Emit when the final response is ready
	Response *[]llmsdk.Part
	// Emit when the loop should continue to the next iteration
	Next *struct{}
}

type RunState struct {
	maxTurns uint
	input    []AgentItem

	// CurrentTurn is the current turn number in the run.
	CurrentTurn uint
	// output contains all items generated during the run, such as new `Tool` and `Model` items
	output []AgentItem

	mu sync.RWMutex
}

func NewRunState(input []AgentItem, maxTurns uint) *RunState {
	return &RunState{
		maxTurns:    maxTurns,
		input:       input,
		CurrentTurn: 0,
		output:      []AgentItem{},
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

// AppendItem adds an AgentItem to the run state and returns its index.
func (s *RunState) AppendItem(item AgentItem) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.output = append(s.output, item)
	return len(s.output) - 1
}

// AppendModelResponse appends a model response as a model AgentItem and returns it and its index.
func (s *RunState) AppendModelResponse(resp llmsdk.ModelResponse) (AgentItem, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := NewAgentItemModelResponse(resp)
	s.output = append(s.output, item)
	return item, len(s.output) - 1
}

func (s *RunState) GetItems() []AgentItem {
	return slices.Concat(s.input, s.output)
}

// GetTurnMessages gets LLM messages to use in the `LanguageModelInput` for the turn
func (s *RunState) GetTurnMessages() []llmsdk.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	messages := []llmsdk.Message{}
	items := s.GetItems()

	for _, it := range items {
		if msg := it.Message; msg != nil {
			messages = append(messages, *msg)
		}
		if modelResponse := it.Model; modelResponse != nil {
			messages = append(messages, llmsdk.NewAssistantMessage(modelResponse.Content...))
		}
		if tool := it.Tool; tool != nil {
			toolResultPart := llmsdk.NewToolResultPart(
				tool.ToolCallID,
				tool.ToolName,
				tool.Output,
				tool.IsError,
			)

			if len(messages) == 0 || messages[len(messages)-1].ToolMessage == nil {
				messages = append(messages, llmsdk.NewToolMessage(toolResultPart))
			} else {
				lastMessage := messages[len(messages)-1]
				lastMessage.ToolMessage.Content = append(lastMessage.ToolMessage.Content, toolResultPart)
				messages[len(messages)-1] = lastMessage
			}
		}
	}

	return messages
}

func (s *RunState) CreateResponse(finalContent []llmsdk.Part) *AgentResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return &AgentResponse{
		Content: finalContent,
		Output:  s.output,
	}
}
