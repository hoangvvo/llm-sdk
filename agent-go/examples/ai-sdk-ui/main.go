package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	"github.com/hoangvvo/llm-sdk/agent-go/examples"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/partutil"
)

// ==== Vercel AI SDK types ====

type uiMessageRole string

const (
	uiRoleSystem    uiMessageRole = "system"
	uiRoleUser      uiMessageRole = "user"
	uiRoleAssistant uiMessageRole = "assistant"
)

type providerMetadata = any

type uiMessage struct {
	ID       string        `json:"id"`
	Role     uiMessageRole `json:"role"`
	Parts    []uiPart      `json:"parts"`
	Metadata any           `json:"metadata,omitempty"`
}

type uiPart struct {
	Text        *textUIPart
	Reasoning   *reasoningUIPart
	DynamicTool *dynamicToolUIPart
	Tool        *toolUIPart
	File        *fileUIPart
}

func (p *uiPart) UnmarshalJSON(data []byte) error {
	var base baseUIPart
	if err := json.Unmarshal(data, &base); err != nil {
		return err
	}
	switch {
	case base.Type == "text":
		var part textUIPart
		if err := json.Unmarshal(data, &part); err != nil {
			return err
		}
		p.Text = &part
	case base.Type == "reasoning":
		var part reasoningUIPart
		if err := json.Unmarshal(data, &part); err != nil {
			return err
		}
		p.Reasoning = &part
	case base.Type == "dynamic-tool":
		var part dynamicToolUIPart
		if err := json.Unmarshal(data, &part); err != nil {
			return err
		}
		p.DynamicTool = &part
	case base.Type == "file":
		var part fileUIPart
		if err := json.Unmarshal(data, &part); err != nil {
			return err
		}
		p.File = &part
	default:
		if strings.HasPrefix(base.Type, "tool-") {
			var part toolUIPart
			if err := json.Unmarshal(data, &part); err != nil {
				return err
			}
			part.rawType = base.Type
			if part.ToolName == "" {
				part.ToolName = strings.TrimPrefix(base.Type, "tool-")
			}
			p.Tool = &part
		} else {
			p.Tool = nil
		}
	}
	return nil
}

func (p uiPart) MarshalJSON() ([]byte, error) {
	switch {
	case p.Text != nil:
		return json.Marshal(struct {
			Type string `json:"type"`
			*textUIPart
		}{
			Type:       "text",
			textUIPart: p.Text,
		})
	case p.Reasoning != nil:
		return json.Marshal(struct {
			Type string `json:"type"`
			*reasoningUIPart
		}{
			Type:            "reasoning",
			reasoningUIPart: p.Reasoning,
		})
	case p.DynamicTool != nil:
		return json.Marshal(struct {
			Type string `json:"type"`
			*dynamicToolUIPart
		}{
			Type:              "dynamic-tool",
			dynamicToolUIPart: p.DynamicTool,
		})
	case p.File != nil:
		return json.Marshal(struct {
			Type string `json:"type"`
			*fileUIPart
		}{
			Type:       "file",
			fileUIPart: p.File,
		})
	case p.Tool != nil:
		typeValue := p.Tool.rawType
		if typeValue == "" {
			if p.Tool.ToolName != "" {
				typeValue = "tool-" + p.Tool.ToolName
			} else {
				typeValue = "tool"
			}
		}
		return json.Marshal(struct {
			Type string `json:"type"`
			*toolUIPart
		}{
			Type:       typeValue,
			toolUIPart: p.Tool,
		})
	default:
		return nil, fmt.Errorf("uiPart marshal: no variant populated")
	}
}

type chatRequestBody struct {
	ID        string                        `json:"id,omitempty"`
	Trigger   string                        `json:"trigger,omitempty"`
	MessageID string                        `json:"messageId,omitempty"`
	Messages  []uiMessage                   `json:"messages"`
	Provider  string                        `json:"provider,omitempty"`
	ModelID   string                        `json:"modelId,omitempty"`
	Metadata  *llmsdk.LanguageModelMetadata `json:"metadata,omitempty"`
}

type baseUIPart struct {
	Type string `json:"type"`
}

type textUIPart struct {
	Text             string           `json:"text"`
	State            *string          `json:"state,omitempty"`
	ProviderMetadata providerMetadata `json:"providerMetadata,omitempty"`
}

type reasoningUIPart struct {
	Text             string           `json:"text"`
	State            *string          `json:"state,omitempty"`
	ProviderMetadata providerMetadata `json:"providerMetadata,omitempty"`
}

type fileUIPart struct {
	URL              string           `json:"url"`
	MediaType        string           `json:"mediaType"`
	Filename         *string          `json:"filename,omitempty"`
	ProviderMetadata providerMetadata `json:"providerMetadata,omitempty"`
}

type dynamicToolUIPart struct {
	ToolName   string `json:"toolName"`
	ToolCallID string `json:"toolCallId"`
	Input      any    `json:"input,omitempty"`
}

type toolUIPart struct {
	State            string           `json:"state"`
	ToolCallID       string           `json:"toolCallId"`
	ToolName         string           `json:"toolName,omitempty"`
	Input            any              `json:"input,omitempty"`
	Output           any              `json:"output,omitempty"`
	ErrorText        string           `json:"errorText,omitempty"`
	ProviderMetadata providerMetadata `json:"providerMetadata,omitempty"`
	rawType          string           `json:"-"`
}

func (p *toolUIPart) resolvedToolName() string {
	if p.ToolName != "" {
		return p.ToolName
	}
	return strings.TrimPrefix(p.rawType, "tool-")
}

// ==== Agent setup ====

type chatContext struct{}

type timeTool struct{}

func (t *timeTool) Name() string {
	return "get_current_time"
}

func (t *timeTool) Description() string {
	return "Get the current server time in ISO 8601 format."
}

func (t *timeTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type":                 "object",
		"properties":           map[string]any{},
		"additionalProperties": false,
	}
}

func (t *timeTool) Execute(_ context.Context, _ json.RawMessage, _ chatContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{llmsdk.NewTextPart(now)},
		IsError: false,
	}, nil
}

type weatherTool struct{}

func (t *weatherTool) Name() string {
	return "get_local_weather"
}

func (t *weatherTool) Description() string {
	return "Return a lightweight weather forecast for a given city using mock data."
}

func (t *weatherTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"location": map[string]any{
				"type":        "string",
				"description": "City name to look up weather for.",
			},
		},
		"required":             []string{"location"},
		"additionalProperties": false,
	}
}

type weatherParams struct {
	Location string `json:"location"`
}

func (t *weatherTool) Execute(_ context.Context, paramsJSON json.RawMessage, _ chatContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var params weatherParams
	if len(paramsJSON) > 0 {
		if err := json.Unmarshal(paramsJSON, &params); err != nil {
			return llmagent.AgentToolResult{}, err
		}
	}

	location := strings.TrimSpace(params.Location)
	conditions := []string{"sunny", "cloudy", "rainy", "breezy"}
	condition := conditions[len(location)%len(conditions)]
	result := map[string]any{
		"location":     location,
		"condition":    condition,
		"temperatureC": 18 + (len(location) % 10),
	}

	payload, err := json.Marshal(result)
	if err != nil {
		return llmagent.AgentToolResult{}, err
	}

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{llmsdk.NewTextPart(string(payload))},
		IsError: false,
	}, nil
}

func createAgent(provider, modelID string, metadata llmsdk.LanguageModelMetadata) *llmagent.Agent[chatContext] {
	model, err := examples.GetModel(provider, modelID, metadata, "")
	if err != nil {
		panic(err)
	}

	instruction1 := "You are an assistant orchestrated by the llm-agent SDK."
	instruction2 := "Use the available tools when they can provide better answers."

	return llmagent.NewAgent("UIExampleAgent", model,
		llmagent.WithInstructions(
			llmagent.InstructionParam[chatContext]{String: &instruction1},
			llmagent.InstructionParam[chatContext]{String: &instruction2},
		),
		llmagent.WithTools(&timeTool{}, &weatherTool{}),
	)
}

// ==== Streaming helpers ====

type textStreamState struct {
	id string
}

type reasoningStreamState struct {
	id string
}

type toolCallStreamState struct {
	toolCallID   string
	toolName     string
	argsBuilder  strings.Builder
	didEmitStart bool
}

type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	mu      sync.Mutex
}

// newSSEWriter wraps the ResponseWriter with helpers for emitting Server-Sent
// Events. The Vercel AI SDK data stream protocol uses SSE, so isolating the
// transport details keeps the adapter focused on payload translation.
func newSSEWriter(w http.ResponseWriter) (*sseWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming unsupported by response writer")
	}
	return &sseWriter{w: w, flusher: flusher}, nil
}

func (w *sseWriter) Write(event any) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.w.Write([]byte("data: ")); err != nil {
		return err
	}
	if _, err := w.w.Write(payload); err != nil {
		return err
	}
	if _, err := w.w.Write([]byte("\n\n")); err != nil {
		return err
	}
	w.flusher.Flush()
	return nil
}

func (w *sseWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.w.Write([]byte("data: [DONE]\n\n")); err != nil {
		return err
	}
	w.flusher.Flush()
	return nil
}

type startChunk struct {
	MessageID string `json:"messageId,omitempty"`
}

func (c startChunk) MarshalJSON() ([]byte, error) {
	type alias struct {
		Type      string `json:"type"`
		MessageID string `json:"messageId,omitempty"`
	}
	return json.Marshal(alias{Type: "start", MessageID: c.MessageID})
}

type startStepChunk struct{}

func (startStepChunk) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string `json:"type"`
	}{Type: "start-step"})
}

type finishStepChunk struct{}

func (finishStepChunk) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string `json:"type"`
	}{Type: "finish-step"})
}

type textStartChunk struct {
	ID string `json:"id"`
}

func (c textStartChunk) MarshalJSON() ([]byte, error) {
	type alias struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}
	return json.Marshal(alias{Type: "text-start", ID: c.ID})
}

type textDeltaChunk struct {
	ID    string `json:"id"`
	Delta string `json:"delta"`
}

func (c textDeltaChunk) MarshalJSON() ([]byte, error) {
	type alias struct {
		Type  string `json:"type"`
		ID    string `json:"id"`
		Delta string `json:"delta"`
	}
	return json.Marshal(alias{Type: "text-delta", ID: c.ID, Delta: c.Delta})
}

type textEndChunk struct {
	ID string `json:"id"`
}

func (c textEndChunk) MarshalJSON() ([]byte, error) {
	type alias struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}
	return json.Marshal(alias{Type: "text-end", ID: c.ID})
}

type reasoningStartChunk struct {
	ID string `json:"id"`
}

func (c reasoningStartChunk) MarshalJSON() ([]byte, error) {
	type alias struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}
	return json.Marshal(alias{Type: "reasoning-start", ID: c.ID})
}

type reasoningDeltaChunk struct {
	ID    string `json:"id"`
	Delta string `json:"delta"`
}

func (c reasoningDeltaChunk) MarshalJSON() ([]byte, error) {
	type alias struct {
		Type  string `json:"type"`
		ID    string `json:"id"`
		Delta string `json:"delta"`
	}
	return json.Marshal(alias{Type: "reasoning-delta", ID: c.ID, Delta: c.Delta})
}

type reasoningEndChunk struct {
	ID string `json:"id"`
}

func (c reasoningEndChunk) MarshalJSON() ([]byte, error) {
	type alias struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}
	return json.Marshal(alias{Type: "reasoning-end", ID: c.ID})
}

type toolInputStartChunk struct {
	ToolCallID string `json:"toolCallId"`
	ToolName   string `json:"toolName"`
}

func (c toolInputStartChunk) MarshalJSON() ([]byte, error) {
	type alias struct {
		Type       string `json:"type"`
		ToolCallID string `json:"toolCallId"`
		ToolName   string `json:"toolName"`
	}
	return json.Marshal(alias{
		Type:       "tool-input-start",
		ToolCallID: c.ToolCallID,
		ToolName:   c.ToolName,
	})
}

type toolInputDeltaChunk struct {
	ToolCallID     string `json:"toolCallId"`
	InputTextDelta string `json:"inputTextDelta"`
}

func (c toolInputDeltaChunk) MarshalJSON() ([]byte, error) {
	type alias struct {
		Type           string `json:"type"`
		ToolCallID     string `json:"toolCallId"`
		InputTextDelta string `json:"inputTextDelta"`
	}
	return json.Marshal(alias{
		Type:           "tool-input-delta",
		ToolCallID:     c.ToolCallID,
		InputTextDelta: c.InputTextDelta,
	})
}

type toolInputAvailableChunk struct {
	ToolCallID string `json:"toolCallId"`
	ToolName   string `json:"toolName"`
	Input      any    `json:"input"`
}

func (c toolInputAvailableChunk) MarshalJSON() ([]byte, error) {
	type alias struct {
		Type       string `json:"type"`
		ToolCallID string `json:"toolCallId"`
		ToolName   string `json:"toolName"`
		Input      any    `json:"input"`
	}
	return json.Marshal(alias{
		Type:       "tool-input-available",
		ToolCallID: c.ToolCallID,
		ToolName:   c.ToolName,
		Input:      c.Input,
	})
}

type toolOutputAvailableChunk struct {
	ToolCallID string `json:"toolCallId"`
	Output     any    `json:"output"`
}

func (c toolOutputAvailableChunk) MarshalJSON() ([]byte, error) {
	type alias struct {
		Type       string `json:"type"`
		ToolCallID string `json:"toolCallId"`
		Output     any    `json:"output"`
	}
	return json.Marshal(alias{
		Type:       "tool-output-available",
		ToolCallID: c.ToolCallID,
		Output:     c.Output,
	})
}

type errorChunk struct {
	ErrorText string `json:"errorText"`
}

func (c errorChunk) MarshalJSON() ([]byte, error) {
	type alias struct {
		Type      string `json:"type"`
		ErrorText string `json:"errorText"`
	}
	return json.Marshal(alias{Type: "error", ErrorText: c.ErrorText})
}

type finishChunk struct{}

func (finishChunk) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string `json:"type"`
	}{Type: "finish"})
}

// dataStreamProtocolAdapter bridges AgentStreamEvent values to the Vercel AI
// SDK data stream protocol. Feed every event emitted by Agent.RunStream into
// Write so the frontend receives the expected stream chunks.
type dataStreamProtocolAdapter struct {
	writer            *sseWriter
	textStateMap      map[int]textStreamState
	reasoningStateMap map[int]reasoningStreamState
	toolCallStateMap  map[int]*toolCallStreamState
	stepStarted       bool
	closed            bool
}

func newDataStreamProtocolAdapter(w http.ResponseWriter) (*dataStreamProtocolAdapter, error) {
	writer, err := newSSEWriter(w)
	if err != nil {
		return nil, err
	}

	adapter := &dataStreamProtocolAdapter{
		writer:            writer,
		textStateMap:      make(map[int]textStreamState),
		reasoningStateMap: make(map[int]reasoningStreamState),
		toolCallStateMap:  make(map[int]*toolCallStreamState),
	}

	messageID := "msg_" + uuid.NewString()
	if err := adapter.writer.Write(startChunk{MessageID: messageID}); err != nil {
		return nil, err
	}

	return adapter, nil
}

func (a *dataStreamProtocolAdapter) Write(event *llmagent.AgentStreamEvent) error {
	if a.closed {
		return nil
	}

	switch {
	case event.Partial != nil:
		if event.Partial.Delta == nil {
			return nil
		}
		if err := a.ensureStepStarted(); err != nil {
			return err
		}
		return a.writeDelta(event.Partial.Delta)
	case event.Item != nil:
		if err := a.finishStep(); err != nil {
			return err
		}
		if event.Item.Item.Tool != nil {
			if err := a.ensureStepStarted(); err != nil {
				return err
			}
			if err := a.writeForToolItem(event.Item.Item.Tool); err != nil {
				return err
			}
			return a.finishStep()
		}
	case event.Response != nil:
		// Final agent response does not emit an extra stream part.
		return nil
	}

	return nil
}

func (a *dataStreamProtocolAdapter) EmitError(errorText string) error {
	if a.closed {
		return nil
	}
	return a.writer.Write(errorChunk{ErrorText: errorText})
}

func (a *dataStreamProtocolAdapter) Close() error {
	if a.closed {
		return nil
	}
	if err := a.finishStep(); err != nil {
		return err
	}
	if err := a.writer.Write(finishChunk{}); err != nil {
		return err
	}
	if err := a.writer.Close(); err != nil {
		return err
	}
	a.closed = true
	return nil
}

func (a *dataStreamProtocolAdapter) ensureStepStarted() error {
	if a.stepStarted {
		return nil
	}
	if err := a.writer.Write(startStepChunk{}); err != nil {
		return err
	}
	a.stepStarted = true
	return nil
}

func (a *dataStreamProtocolAdapter) finishStep() error {
	if !a.stepStarted {
		return nil
	}
	if err := a.flushStates(); err != nil {
		return err
	}
	if err := a.writer.Write(finishStepChunk{}); err != nil {
		return err
	}
	a.stepStarted = false
	return nil
}

func (a *dataStreamProtocolAdapter) flushStates() error {
	for index, state := range a.textStateMap {
		if err := a.writer.Write(textEndChunk{ID: state.id}); err != nil {
			return err
		}
		delete(a.textStateMap, index)
	}

	for index, state := range a.reasoningStateMap {
		if err := a.writer.Write(reasoningEndChunk{ID: state.id}); err != nil {
			return err
		}
		delete(a.reasoningStateMap, index)
	}

	for index, state := range a.toolCallStateMap {
		if state.toolCallID != "" && state.toolName != "" && state.argsBuilder.Len() > 0 {
			input := safeJSONParse(state.argsBuilder.String())
			if err := a.writer.Write(toolInputAvailableChunk{
				ToolCallID: state.toolCallID,
				ToolName:   state.toolName,
				Input:      input,
			}); err != nil {
				return err
			}
		}
		delete(a.toolCallStateMap, index)
	}

	return nil
}

func (a *dataStreamProtocolAdapter) writeDelta(delta *llmsdk.ContentDelta) error {
	switch {
	case delta.Part.TextPartDelta != nil:
		return a.writeForTextPartDelta(delta.Index, delta.Part.TextPartDelta)
	case delta.Part.ReasoningPartDelta != nil:
		return a.writeForReasoningPartDelta(delta.Index, delta.Part.ReasoningPartDelta)
	case delta.Part.ToolCallPartDelta != nil:
		return a.writeForToolCallPartDelta(delta.Index, delta.Part.ToolCallPartDelta)
	case delta.Part.AudioPartDelta != nil:
		return a.flushStates()
	case delta.Part.ImagePartDelta != nil:
		return a.flushStates()
	default:
		return nil
	}
}

func (a *dataStreamProtocolAdapter) writeForTextPartDelta(index int, part *llmsdk.TextPartDelta) error {
	state, ok := a.textStateMap[index]
	if !ok {
		if err := a.flushStates(); err != nil {
			return err
		}
		state = textStreamState{id: "text_" + uuid.NewString()}
		a.textStateMap[index] = state
		if err := a.writer.Write(textStartChunk{ID: state.id}); err != nil {
			return err
		}
	}

	return a.writer.Write(textDeltaChunk{ID: state.id, Delta: part.Text})
}

func (a *dataStreamProtocolAdapter) writeForReasoningPartDelta(index int, part *llmsdk.ReasoningPartDelta) error {
	state, ok := a.reasoningStateMap[index]
	if !ok {
		if err := a.flushStates(); err != nil {
			return err
		}
		id := "reasoning_" + uuid.NewString()
		if part.ID != nil && *part.ID != "" {
			id = "reasoning_" + *part.ID
		}
		state = reasoningStreamState{id: id}
		a.reasoningStateMap[index] = state
		if err := a.writer.Write(reasoningStartChunk{ID: state.id}); err != nil {
			return err
		}
	}

	return a.writer.Write(reasoningDeltaChunk{ID: state.id, Delta: part.Text})
}

func (a *dataStreamProtocolAdapter) writeForToolCallPartDelta(index int, part *llmsdk.ToolCallPartDelta) error {
	state, ok := a.toolCallStateMap[index]
	if !ok {
		if err := a.flushStates(); err != nil {
			return err
		}
		state = &toolCallStreamState{}
		a.toolCallStateMap[index] = state
	}

	if part.ToolCallID != nil && *part.ToolCallID != "" {
		state.toolCallID = *part.ToolCallID
	}
	if part.ToolName != nil && *part.ToolName != "" {
		state.toolName = *part.ToolName
	}

	if !state.didEmitStart && state.toolCallID != "" && state.toolName != "" {
		state.didEmitStart = true
		if err := a.writer.Write(toolInputStartChunk{
			ToolCallID: state.toolCallID,
			ToolName:   state.toolName,
		}); err != nil {
			return err
		}
	}

	if part.Args != nil && *part.Args != "" {
		state.argsBuilder.WriteString(*part.Args)
		return a.writer.Write(toolInputDeltaChunk{
			ToolCallID:     state.toolCallID,
			InputTextDelta: *part.Args,
		})
	}

	return nil
}

func (a *dataStreamProtocolAdapter) writeForToolItem(item *llmagent.AgentItemTool) error {
	if err := a.flushStates(); err != nil {
		return err
	}

	var textBuffer strings.Builder
	for _, part := range item.Output {
		if part.TextPart != nil {
			textBuffer.WriteString(part.TextPart.Text)
		}
	}

	var output any
	if textBuffer.Len() > 0 {
		output = safeJSONParse(textBuffer.String())
	} else {
		output = item.Output
	}

	return a.writer.Write(toolOutputAvailableChunk{
		ToolCallID: item.ToolCallID,
		Output:     output,
	})
}

// ==== Adapter layers ====

// uiPartToParts converts UI message parts produced by the Vercel AI SDK back
// into llm-sdk Part values so the agent can reconstruct history, tool calls,
// and intermediate reasoning steps.
func uiPartToParts(part uiPart) []llmsdk.Part {
	switch {
	case part.Text != nil:
		return []llmsdk.Part{llmsdk.NewTextPart(part.Text.Text)}
	case part.Reasoning != nil:
		return []llmsdk.Part{llmsdk.NewReasoningPart(part.Reasoning.Text)}
	case part.DynamicTool != nil:
		if part.DynamicTool.ToolCallID == "" || part.DynamicTool.ToolName == "" {
			return nil
		}
		return []llmsdk.Part{llmsdk.NewToolCallPart(part.DynamicTool.ToolCallID, part.DynamicTool.ToolName, part.DynamicTool.Input)}
	case part.File != nil:
		return convertFilePart(part.File)
	case part.Tool != nil:
		return convertToolPart(part.Tool)
	default:
		return nil
	}
}

func convertFilePart(part *fileUIPart) []llmsdk.Part {
	data := extractDataPayload(part.URL)
	switch {
	case strings.HasPrefix(part.MediaType, "image/"):
		imagePart := llmsdk.NewImagePart(data, part.MediaType)
		return []llmsdk.Part{imagePart}
	case strings.HasPrefix(part.MediaType, "audio/"):
		format, err := partutil.MapMimeTypeToAudioFormat(part.MediaType)
		if err != nil {
			return nil
		}
		return []llmsdk.Part{llmsdk.NewAudioPart(data, format)}
	case strings.HasPrefix(part.MediaType, "text/"):
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return nil
		}
		return []llmsdk.Part{llmsdk.NewTextPart(string(decoded))}
	default:
		return nil
	}
}

func convertToolPart(part *toolUIPart) []llmsdk.Part {
	name := part.resolvedToolName()
	if part.ToolCallID == "" || name == "" {
		return nil
	}
	switch part.State {
	case "input-available":
		return []llmsdk.Part{llmsdk.NewToolCallPart(part.ToolCallID, name, part.Input)}
	case "output-available":
		call := llmsdk.NewToolCallPart(part.ToolCallID, name, part.Input)
		result := llmsdk.NewToolResultPart(part.ToolCallID, name, []llmsdk.Part{
			llmsdk.NewTextPart(safeJSONMarshal(part.Output)),
		})
		return []llmsdk.Part{call, result}
	case "output-error":
		call := llmsdk.NewToolCallPart(part.ToolCallID, name, part.Input)
		result := llmsdk.NewToolResultPart(part.ToolCallID, name, []llmsdk.Part{
			llmsdk.NewTextPart(part.ErrorText),
		})
		return []llmsdk.Part{call, result}
	default:
		return nil
	}
}

func uiMessagesToMessages(messages []uiMessage) ([]llmsdk.Message, error) {
	history := make([]llmsdk.Message, 0, len(messages))

	for _, message := range messages {
		switch message.Role {
		case uiRoleUser:
			var parts []llmsdk.Part
			for _, part := range message.Parts {
				parts = append(parts, uiPartToParts(part)...)
			}
			if len(parts) == 0 {
				continue
			}
			history = append(history, llmsdk.NewUserMessage(parts...))
		case uiRoleAssistant:
			for _, part := range message.Parts {
				for _, converted := range uiPartToParts(part) {
					switch converted.Type() {
					case llmsdk.PartTypeText, llmsdk.PartTypeReasoning, llmsdk.PartTypeAudio, llmsdk.PartTypeImage, llmsdk.PartTypeToolCall:
						appendAssistantMessage(&history, converted)
					case llmsdk.PartTypeToolResult:
						appendToolMessage(&history, converted)
					}
				}
			}
		default:
			// ignore unsupported roles
		}
	}

	return history, nil
}

func appendAssistantMessage(history *[]llmsdk.Message, part llmsdk.Part) {
	n := len(*history)
	if n > 0 {
		last := &(*history)[n-1]
		if msg := last.AssistantMessage; msg != nil {
			msg.Content = append(msg.Content, part)
			return
		}
		if last.ToolMessage != nil && n >= 2 {
			prev := &(*history)[n-2]
			if msg := prev.AssistantMessage; msg != nil {
				msg.Content = append(msg.Content, part)
				return
			}
		}
	}

	*history = append(*history, llmsdk.NewAssistantMessage(part))
}

func appendToolMessage(history *[]llmsdk.Message, part llmsdk.Part) {
	n := len(*history)
	if n > 0 {
		last := &(*history)[n-1]
		if msg := last.ToolMessage; msg != nil {
			msg.Content = append(msg.Content, part)
			return
		}
	}

	*history = append(*history, llmsdk.NewToolMessage(part))
}

// ==== HTTP handlers ====

func handleChatRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "content-type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	bodyBytes, err := readRequestBody(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var body chatRequestBody
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	provider := body.Provider
	if provider == "" {
		provider = "openai"
	}
	modelID := body.ModelID
	if modelID == "" {
		modelID = "gpt-4o-mini"
	}

	var metadata llmsdk.LanguageModelMetadata
	if body.Metadata != nil {
		metadata = *body.Metadata
	}

	agent := createAgent(provider, modelID, metadata)

	messages, err := uiMessagesToMessages(body.Messages)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid messages payload: %v", err), http.StatusBadRequest)
		return
	}
	items := make([]llmagent.AgentItem, 0, len(messages))
	for _, message := range messages {
		items = append(items, llmagent.NewAgentItemMessage(message))
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("x-vercel-ai-ui-message-stream", "v1")
	w.WriteHeader(http.StatusOK)

	adapter, err := newDataStreamProtocolAdapter(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	stream, err := agent.RunStream(ctx, llmagent.AgentRequest[chatContext]{
		Input:   items,
		Context: chatContext{},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	clientClosed := false

	for stream.Next() {
		event := stream.Current()
		if err := adapter.Write(event); err != nil {
			clientClosed = true
			cancel()
			break
		}
		if ctx.Err() != nil {
			clientClosed = true
			break
		}
	}

	if streamErr := stream.Err(); streamErr != nil && !errors.Is(streamErr, context.Canceled) {
		if err := adapter.EmitError(streamErr.Error()); err != nil {
			log.Printf("ai-sdk-ui: failed to emit error chunk: %v", err)
		}
	}

	if !clientClosed {
		if err := adapter.Close(); err != nil {
			log.Printf("ai-sdk-ui: failed to close stream: %v", err)
		}
	}
}

func readRequestBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}

// ==== Utility helpers ====

// safeJSONParse attempts to decode tool arguments or results as JSON. When a
// payload is not valid JSON we fall back to the original string so the UI can
// still surface something meaningful to the user.
func safeJSONParse(raw string) any {
	if strings.TrimSpace(raw) == "" {
		return raw
	}

	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return raw
	}
	if decoder.More() {
		return raw
	}
	return value
}

func safeJSONMarshal(value any) string {
	if value == nil {
		return "null"
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}

func extractDataPayload(dataURL string) string {
	if idx := strings.Index(dataURL, ","); idx != -1 {
		return dataURL[idx+1:]
	}
	return dataURL
}

// ==== Server bootstrap ====

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/chat", handleChatRequest)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Not found"}`))
	})

	port := "8000"
	log.Printf("AI SDK UI example server listening on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
