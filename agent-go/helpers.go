package llmagent

import (
	"encoding/json"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

func NewAgentItemMessage(message llmsdk.Message) AgentItem {
	return AgentItem{Message: &message}
}

func NewAgentItemModelResponse(response llmsdk.ModelResponse) AgentItem {
	return AgentItem{Model: &AgentItemModelResponse{ModelResponse: &response}}
}

func NewAgentItemTool(toolCallID, toolName string, input json.RawMessage, output []llmsdk.Part, isError bool) AgentItem {
	return AgentItem{Tool: &AgentItemTool{
		ToolCallID: toolCallID,
		ToolName:   toolName,
		Input:      input,
		Output:     output,
		IsError:    isError,
	}}
}

func NewAgentStreamItemEvent(index int, item AgentItem) *AgentStreamEvent {
	return &AgentStreamEvent{Item: &AgentStreamItemEvent{
		Index: index,
		Item:  item,
	}}
}

func NewAgentStreamEventPartial(partial *llmsdk.PartialModelResponse) *AgentStreamEvent {
	return &AgentStreamEvent{Partial: partial}
}

func NewAgentStreamEventResponse(response *AgentResponse) *AgentStreamEvent {
	return &AgentStreamEvent{Response: response}
}
