package llmagent

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/stream"
	"golang.org/x/sync/errgroup"
)

func attachRunSnapshot(err error, snapshot *AgentRunSnapshot) error {
	var agentErr *AgentError
	if errors.As(err, &agentErr) {
		agentErr.withSnapshot(snapshot)
		return err
	}
	return (&AgentError{
		Kind:    InvariantErrorKind,
		Message: "agent run error",
		Err:     err,
	}).withSnapshot(snapshot)
}

func emitToolCancellationEvents(currCh chan<- ProcessEvent, pendingToolCalls []*llmsdk.ToolCallPart) {
	for _, toolCall := range pendingToolCalls {
		item := NewAgentItemTool(
			toolCall.ToolCallID,
			toolCall.ToolName,
			toolCall.Args,
			[]llmsdk.Part{},
			llmsdk.ToolResultStatusCancelled,
		)
		currCh <- ProcessEvent{Item: &item}
	}
	currCh <- ProcessEvent{Response: &ProcessResponse{
		Content: []llmsdk.Part{},
		Status:  AgentResponseStatusCancelled,
	}}
}

// RunSession manages the run session for an agent.
// It initializes all necessary components for the agent to run
// and handles the execution of the agent's tasks.
// Once finished, the session cleans up any resources used during the run.
//
// The session can be reused in multiple runs. RunSession binds to a specific
// context value that is used when resolving instructions and invoking tools,
// while input items remain per run and are supplied to each invocation.
type RunSession[C any] struct {
	params             *AgentParams[C]     // params stores the agent configuration used during the run.
	contextVal         C                   // contextVal is the bound context value used for instructions and tool executions.
	staticSystemPrompt *string             // systemPrompt caches the resolved instructions as a system prompt.
	staticTools        []AgentTool[C]      // staticTools holds the tools provided directly in the agent params.
	toolkitSessions    []ToolkitSession[C] // toolkitSessions keeps the toolkit-provided sessions for this run session.
	initialized        bool                // initialized ensures the session is ready before running.
}

// NewRunSession creates a new run session, resolves instructions, and initializes dependencies.
func NewRunSession[C any](
	ctx context.Context,
	params *AgentParams[C],
	contextVal C,
) (*RunSession[C], error) {
	session := &RunSession[C]{
		params:      params,
		contextVal:  contextVal,
		staticTools: append([]AgentTool[C]{}, params.Tools...),
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
		s.staticSystemPrompt = &prompt
	}

	if len(s.params.Toolkits) > 0 {
		sessions := make([]ToolkitSession[C], len(s.params.Toolkits))
		cleanupCtx := context.WithoutCancel(ctx)
		g, ctx := errgroup.WithContext(ctx)
		for i, toolkit := range s.params.Toolkits {
			g.Go(func() error {
				toolkitSession, err := toolkit.CreateSession(ctx, s.contextVal)
				if err != nil {
					return fmt.Errorf("toolkit[%d].CreateSession: %w", i, err)
				}
				sessions[i] = toolkitSession
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			var cleanupGroup errgroup.Group
			for _, session := range sessions {
				if session != nil {
					cleanupGroup.Go(func() error { return session.Close(cleanupCtx) })
				}
			}
			_ = cleanupGroup.Wait()
			return NewInitError(err)
		}
		s.toolkitSessions = sessions
	}

	s.initialized = true
	return nil
}

// process flow:
//
//  1. Peek latest run item to locate assistant content.
//
//     1a. Tail is user message -> emit Next. Go to 3.
//
//     1b. Tail is tool/tool message -> gather processed ids, backtrack to assistant/model content. Go to 2.
//
//     1c. Tail is assistant/model -> use its content. Go to 2.
//
//  2. Scan assistant content for tool calls.
//
//     2a. Tool calls remaining -> execute unprocessed tools, emit each Item, then emit Next. Go to 3.
//
//     2b. No tool calls -> emit Response. Go to 4.
//
//  3. Outer loop: bump turn, refresh params, request model response, append it, then re-enter step 1.
//
//  4. Return final response to caller.
func (s *RunSession[C]) process(
	ctx context.Context,
	runState *RunState,
	tools []AgentFunctionTool[C],
) *stream.Stream[ProcessEvent] {
	currCh := make(chan ProcessEvent)
	errCh := make(chan error, 1)

	go func() {
		defer close(currCh)
		defer close(errCh)

		allItems := runState.Items()
		if len(allItems) == 0 {
			errCh <- NewInvariantError("no items in the run state")
			return
		}

		lastItem := allItems[len(allItems)-1]

		var content []llmsdk.Part
		// a set of tool call IDs that have been processed
		processedToolCallIDs := make(map[string]struct{})

		switch {
		case lastItem.Model != nil:
			// ========== Case: Assistant Message [from AgentItemModelResponse] ==========
			// Last item is a model response, process it
			content = lastItem.Model.Content
		case lastItem.Message != nil:
			switch {
			case lastItem.Message.AssistantMessage != nil:
				// ========== Case: Assistant Message [from AgentItemMessage] ==========
				// Last item is an assistant message, process it
				content = lastItem.Message.AssistantMessage.Content
			case lastItem.Message.UserMessage != nil:
				// ========== Case: User Message ==========
				// last item is a user message, so we need to generate a model response
				currCh <- ProcessEvent{Next: &struct{}{}}
				return
			case lastItem.Message.ToolMessage != nil:
				// ========== Case: Tool Results (from AgentItemMessage) ==========
				// Track the tool call ids that have been processed to avoid duplicate execution
				for _, part := range lastItem.Message.ToolMessage.Content {
					if part.ToolResultPart != nil {
						processedToolCallIDs[part.ToolResultPart.ToolCallID] = struct{}{}
					}
				}

				// We are in the middle of processing tool results, the 2nd last item should be a model response
				if len(allItems) < 2 {
					errCh <- NewInvariantError("no preceding assistant content found before tool results")
					return
				}

				previousItem := allItems[len(allItems)-2]
				switch {
				case previousItem.Model != nil:
					content = previousItem.Model.Content

				case previousItem.Message != nil && previousItem.Message.AssistantMessage != nil:
					content = previousItem.Message.AssistantMessage.Content

				default:
					errCh <- NewInvariantError("expected a model item or assistant message before tool results")
					return
				}
			default:
				errCh <- NewInvariantError("unsupported message role in run state")
				return
			}

		case lastItem.Tool != nil:
			// ========== Case: Tool Results (from AgentItemTool) ==========
			// Each tool result is an individual item in this representation, so there could be other
			// AgentItemTool before this one. We loop backwards to find the first non-tool item while also
			// tracking the called tool ids to avoid duplicate execution

			for i := len(allItems) - 1; i >= 0; i-- {
				item := allItems[i]

				if item.Tool != nil {
					processedToolCallIDs[item.Tool.ToolCallID] = struct{}{}
					// Continue searching for the originating model/assistant item
					continue
				} else if item.Model != nil {
					// Found the originating model response
					content = item.Model.Content
					break
				} else if item.Message != nil {
					if item.Message.ToolMessage != nil {
						// Collect all tool call ids in the tool message
						for _, part := range item.Message.ToolMessage.Content {
							if part.ToolResultPart != nil {
								processedToolCallIDs[part.ToolResultPart.ToolCallID] = struct{}{}
							}
						}
						// Continue searching for the originating model/assistant item
						continue
					}
					if item.Message.AssistantMessage != nil {
						// Found the originating assistant message
						content = item.Message.AssistantMessage.Content
						break
					}
					if item.Message.UserMessage != nil {
						errCh <- NewInvariantError("expected a model item or assistant message before tool results")
						return
					}
				} else {
					errCh <- NewInvariantError("invalid item type in run state")
				}
			}

			if content == nil {
				errCh <- NewInvariantError("no model or assistant message found before tool results")
				return
			}
		default:
			errCh <- NewInvariantError("unsupported item type in run state")
			return
		}

		if len(content) == 0 {
			errCh <- NewInvariantError("no assistant content found to process")
			return
		}

		var toolCallParts []*llmsdk.ToolCallPart
		for _, part := range content {
			if part.ToolCallPart != nil {
				toolCallParts = append(toolCallParts, part.ToolCallPart)
			}
		}

		if len(toolCallParts) == 0 {
			status := AgentResponseStatusCompleted
			if ctx.Err() != nil {
				status = AgentResponseStatusCancelled
			}
			currCh <- ProcessEvent{Response: &ProcessResponse{
				Content: content,
				Status:  status,
			}}
			return
		}

		seenToolCallIDs := make(map[string]struct{}, len(toolCallParts))
		for _, toolCallPart := range toolCallParts {
			if _, exists := seenToolCallIDs[toolCallPart.ToolCallID]; exists {
				errCh <- NewInvariantError(fmt.Sprintf("duplicate tool call ID: %s", toolCallPart.ToolCallID))
				return
			}
			seenToolCallIDs[toolCallPart.ToolCallID] = struct{}{}
		}

		pendingToolCalls := make([]*llmsdk.ToolCallPart, 0, len(toolCallParts))
		for _, toolCallPart := range toolCallParts {
			if _, exists := processedToolCallIDs[toolCallPart.ToolCallID]; !exists {
				pendingToolCalls = append(pendingToolCalls, toolCallPart)
			}
		}

		for index, toolCallPart := range pendingToolCalls {
			if ctx.Err() != nil {
				emitToolCancellationEvents(currCh, pendingToolCalls[index:])
				return
			}

			var agentTool AgentFunctionTool[C]
			for _, tool := range tools {
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
				if ctx.Err() != nil {
					emitToolCancellationEvents(currCh, pendingToolCalls[index:])
					return
				}
				errCh <- err
				return
			}

			status := llmsdk.ToolResultStatusCompleted
			if toolRes.IsError {
				status = llmsdk.ToolResultStatusFailed
			}
			item := NewAgentItemTool(
				toolCallPart.ToolCallID,
				toolCallPart.ToolName,
				toolCallPart.Args,
				toolRes.Content,
				status,
			)
			currCh <- ProcessEvent{Item: &item}

			if ctx.Err() != nil {
				emitToolCancellationEvents(currCh, pendingToolCalls[index+1:])
				return
			}
		}

		currCh <- ProcessEvent{Next: &struct{}{}}
	}()

	return stream.New(currCh, errCh)
}

// Run runs a non-streaming execution of the agent.
func (s *RunSession[C]) Run(ctx context.Context, request RunSessionRequest) (*AgentResponse, error) {
	if !s.initialized {
		return nil, NewInvariantError("run session not initialized")
	}

	return traceRun(ctx, s.params.Name, "run", func(ctx context.Context) (*AgentResponse, error) {
		state := NewRunState(request.Input, s.params.MaxTurns)
		tools := s.getFunctionTools()

		for {
			processStream := s.process(ctx, state, tools)
			for processStream.Next() {
				event := processStream.Current()
				if event.Response != nil {
					response := state.createResponse(event.Response.Content, event.Response.Status)
					return response, nil
				}
				if event.Item != nil {
					state.appendItem(*event.Item)
				}
				if event.Next != nil {
					if err := state.turn(); err != nil {
						return nil, attachRunSnapshot(err, state.createSnapshot())
					}
					break
				}
			}
			if err := processStream.Err(); err != nil {
				return nil, attachRunSnapshot(err, state.createSnapshot())
			}

			input, nextTools := s.getTurnParams(state)
			tools = nextTools

			if ctx.Err() != nil {
				return state.createCancelledResponse(), nil
			}
			modelResponse, err := s.params.Model.Generate(ctx, input)
			if err != nil {
				if ctx.Err() != nil {
					return state.createCancelledResponse(), nil
				}
				return nil, NewLanguageModelError(err).withSnapshot(state.createSnapshot())
			}

			state.appendModelResponse(*modelResponse)
		}
	})
}

// RunStream runs a streaming execution of the agent.
func (s *RunSession[C]) RunStream(ctx context.Context, request RunSessionRequest) (*AgentStream, error) {
	if !s.initialized {
		return nil, NewInvariantError("run session not initialized")
	}

	return traceRunStream(ctx, s.params.Name, "run_stream", func(ctx context.Context) (*AgentStream, error) {
		state := NewRunState(request.Input, s.params.MaxTurns)

		eventChan := make(chan *AgentStreamEvent)
		errChan := make(chan error, 1)

		go func() {
			defer close(eventChan)
			defer close(errChan)

			tools := s.getFunctionTools()

			for {
				processStream := s.process(ctx, state, tools)

				for processStream.Next() {
					event := processStream.Current()
					if event.Response != nil {
						response := state.createResponse(event.Response.Content, event.Response.Status)
						eventChan <- NewAgentStreamEventResponse(response)
						return
					}
					if event.Item != nil {
						index := state.appendItem(*event.Item)
						eventChan <- NewAgentStreamItemEvent(index, *event.Item)
					}
					if event.Next != nil {
						if err := state.turn(); err != nil {
							errChan <- attachRunSnapshot(err, state.createSnapshot())
							return
						}
						break
					}
				}
				if err := processStream.Err(); err != nil {
					errChan <- attachRunSnapshot(err, state.createSnapshot())
					return
				}

				input, nextTools := s.getTurnParams(state)
				tools = nextTools

				if ctx.Err() != nil {
					eventChan <- NewAgentStreamEventResponse(state.createCancelledResponse())
					return
				}
				modelStream, err := s.params.Model.Stream(ctx, input)
				if err != nil {
					if ctx.Err() != nil {
						eventChan <- NewAgentStreamEventResponse(state.createCancelledResponse())
						return
					}
					errChan <- NewLanguageModelError(err).withSnapshot(state.createSnapshot())
					return
				}

				accumulator := llmsdk.NewStreamAccumulator()
				commitModelSnapshot := func(snapshot llmsdk.ModelResponse) bool {
					if item, index, ok := state.appendModelSnapshot(snapshot); ok {
						eventChan <- NewAgentStreamItemEvent(index, item)
						return true
					}
					return false
				}

				for modelStream.Next() {
					partial := modelStream.Current()

					if err := accumulator.AddPartial(*partial); err != nil {
						snapshot := accumulator.Snapshot()
						commitModelSnapshot(snapshot)
						errChan <- NewInvariantError(fmt.Sprintf("failed to accumulate stream: %v", err)).withSnapshot(
							state.createSnapshot(),
						)
						return
					}

					eventChan <- NewAgentStreamEventPartial(partial)
				}

				if ctx.Err() != nil {
					snapshot := accumulator.Snapshot()
					if commitModelSnapshot(snapshot) {
						continue
					}
					eventChan <- NewAgentStreamEventResponse(
						state.createResponse(snapshot.Content, AgentResponseStatusCancelled),
					)
					return
				}

				if err := modelStream.Err(); err != nil {
					snapshot := accumulator.Snapshot()
					commitModelSnapshot(snapshot)
					errChan <- NewLanguageModelError(err).withSnapshot(
						state.createSnapshot(),
					)
					return
				}

				modelResponse, err := accumulator.ComputeResponse()
				if err != nil {
					snapshot := accumulator.Snapshot()
					commitModelSnapshot(snapshot)
					errChan <- NewInvariantError(fmt.Sprintf("failed to compute stream response: %v", err)).withSnapshot(
						state.createSnapshot(),
					)
					return
				}

				item, index := state.appendModelResponse(modelResponse)

				eventChan <- NewAgentStreamItemEvent(index, item)
			}
		}()

		return stream.New(eventChan, errChan), nil
	})
}

func (s *RunSession[C]) Close(ctx context.Context) error {
	if !s.initialized {
		return nil
	}
	s.staticSystemPrompt = nil
	s.staticTools = nil

	var g errgroup.Group
	for _, toolkitSession := range s.toolkitSessions {
		if toolkitSession == nil {
			continue
		}
		g.Go(func() error {
			return toolkitSession.Close(ctx)
		})
	}
	err := g.Wait()
	s.toolkitSessions = nil
	s.initialized = false

	if err != nil {
		return NewCleanupError(err)
	}
	return nil
}

func (s *RunSession[C]) getTurnParams(state *RunState) (*llmsdk.LanguageModelInput, []AgentFunctionTool[C]) {
	input := &llmsdk.LanguageModelInput{
		Messages:         state.getTurnMessages(),
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

	systemPrompts := []string{}
	if s.staticSystemPrompt != nil && *s.staticSystemPrompt != "" {
		systemPrompts = append(systemPrompts, *s.staticSystemPrompt)
	}

	for _, toolkitSession := range s.toolkitSessions {
		if toolkitSession == nil {
			continue
		}
		if prompt := toolkitSession.SystemPrompt(); prompt != nil && *prompt != "" {
			systemPrompts = append(systemPrompts, *prompt)
		}
	}

	tools := s.getTools()
	functionTools := s.getFunctionToolsFrom(tools)

	if len(systemPrompts) > 0 {
		input.SystemPrompt = ptr.To(strings.Join(systemPrompts, "\n"))
	}
	if len(tools) > 0 {
		sdkTools := make([]llmsdk.Tool, 0, len(tools))
		for _, tool := range tools {
			sdkTools = append(sdkTools, tool.ToLanguageModelTool())
		}
		input.Tools = sdkTools
	}

	return input, functionTools
}

func (s *RunSession[C]) getTools() []AgentTool[C] {
	tools := make([]AgentTool[C], len(s.staticTools))
	copy(tools, s.staticTools)
	for _, toolkitSession := range s.toolkitSessions {
		if toolkitSession == nil {
			continue
		}
		if toolkitTools := toolkitSession.Tools(); len(toolkitTools) > 0 {
			tools = append(tools, toolkitTools...)
		}
	}
	return tools
}

func (s *RunSession[C]) getFunctionTools() []AgentFunctionTool[C] {
	return s.getFunctionToolsFrom(s.getTools())
}

func (s *RunSession[C]) getFunctionToolsFrom(tools []AgentTool[C]) []AgentFunctionTool[C] {
	functionTools := make([]AgentFunctionTool[C], 0, len(tools))
	for _, tool := range tools {
		if functionTool := tool.AsFunctionTool(); functionTool != nil {
			functionTools = append(functionTools, functionTool)
		}
	}
	return functionTools
}

// RunSessionRequest contains the input used for a run.
type RunSessionRequest struct {
	// Input holds the items to seed the run, such as LLM messages.
	Input []AgentItem
}

// ProcessEvent represents an event returned by the process function.
type ProcessEvent struct {
	// Emit when a new item is generated
	Item *AgentItem
	// Emit when the final response is ready
	Response *ProcessResponse
	// Emit when the loop should continue to the next iteration
	Next *struct{}
}

type ProcessResponse struct {
	Content []llmsdk.Part
	Status  AgentResponseStatus
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

// turn marks a new turn in the conversation and throw an error if max turns
// exceeded.
func (s *RunState) turn() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.CurrentTurn++
	if s.CurrentTurn > s.maxTurns {
		return NewMaxTurnsExceededError(int(s.maxTurns))
	}
	return nil
}

// appendItem adds an AgentItem to the run state and returns its index.
func (s *RunState) appendItem(item AgentItem) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.output = append(s.output, item)
	return len(s.output) - 1
}

// appendModelResponse appends a model response as a model AgentItem and returns it and its index.
func (s *RunState) appendModelResponse(resp llmsdk.ModelResponse) (AgentItem, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := NewAgentItemModelResponse(resp)
	s.output = append(s.output, item)
	return item, len(s.output) - 1
}

// appendModelSnapshot appends the independently materializable portion of an
// interrupted model stream. An empty snapshot does not represent an output item.
func (s *RunState) appendModelSnapshot(resp llmsdk.ModelResponse) (AgentItem, int, bool) {
	if len(resp.Content) == 0 {
		return AgentItem{}, 0, false
	}
	item, index := s.appendModelResponse(resp)
	return item, index, true
}

func (s *RunState) Items() []AgentItem {
	return slices.Concat(s.input, s.output)
}

// getTurnMessages gets LLM messages to use in the `LanguageModelInput` for the turn
func (s *RunState) getTurnMessages() []llmsdk.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	messages := []llmsdk.Message{}
	items := s.Items()

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
				llmsdk.WithToolResultStatus(tool.Status),
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

func (s *RunState) createResponse(finalContent []llmsdk.Part, status AgentResponseStatus) *AgentResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return &AgentResponse{
		Content: finalContent,
		Output:  s.output,
		Status:  status,
	}
}

func (s *RunState) createSnapshot() *AgentRunSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return &AgentRunSnapshot{
		Output: slices.Clone(s.output),
	}
}

func (s *RunState) createCancelledResponse() *AgentResponse {
	return s.createResponse([]llmsdk.Part{}, AgentResponseStatusCancelled)
}
