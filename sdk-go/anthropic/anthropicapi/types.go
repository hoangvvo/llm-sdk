package anthropicapi

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ServiceTier type for service tier enum
type ServiceTier string

const (
	ServiceTierAuto         ServiceTier = "auto"
	ServiceTierStandardOnly ServiceTier = "standard_only"
)

// Model type for model enum
type Model string

const (
	ModelClaude37SonnetLatest   Model = "claude-3-7-sonnet-latest"
	ModelClaude37Sonnet20250219 Model = "claude-3-7-sonnet-20250219"
	ModelClaude35HaikuLatest    Model = "claude-3-5-haiku-latest"
	ModelClaude35Haiku20241022  Model = "claude-3-5-haiku-20241022"
	ModelClaudeSonnet420250514  Model = "claude-sonnet-4-20250514"
	ModelClaudeSonnet40         Model = "claude-sonnet-4-0"
	ModelClaude4Sonnet20250514  Model = "claude-4-sonnet-20250514"
	ModelClaude35SonnetLatest   Model = "claude-3-5-sonnet-latest"
	ModelClaude35Sonnet20241022 Model = "claude-3-5-sonnet-20241022"
	ModelClaude35Sonnet20240620 Model = "claude-3-5-sonnet-20240620"
	ModelClaudeOpus40           Model = "claude-opus-4-0"
	ModelClaudeOpus420250514    Model = "claude-opus-4-20250514"
	ModelClaude4Opus20250514    Model = "claude-4-opus-20250514"
	ModelClaudeOpus4120250805   Model = "claude-opus-4-1-20250805"
	ModelClaude3OpusLatest      Model = "claude-3-opus-latest"
	ModelClaude3Opus20240229    Model = "claude-3-opus-20240229"
	ModelClaude3Haiku20240307   Model = "claude-3-haiku-20240307"
)

// StopReason type for stop reason enum
type StopReason string

const (
	StopReasonEndTurn      StopReason = "end_turn"
	StopReasonMaxTokens    StopReason = "max_tokens"
	StopReasonStopSequence StopReason = "stop_sequence"
	StopReasonToolUse      StopReason = "tool_use"
	StopReasonPauseTurn    StopReason = "pause_turn"
	StopReasonRefusal      StopReason = "refusal"
)

// Role type for message role enum
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// WebSearchToolResultErrorCode type for error code enum
type WebSearchToolResultErrorCode string

const (
	WebSearchToolResultErrorCodeInvalidToolInput WebSearchToolResultErrorCode = "invalid_tool_input"
	WebSearchToolResultErrorCodeUnavailable      WebSearchToolResultErrorCode = "unavailable"
	WebSearchToolResultErrorCodeMaxUsesExceeded  WebSearchToolResultErrorCode = "max_uses_exceeded"
	WebSearchToolResultErrorCodeTooManyRequests  WebSearchToolResultErrorCode = "too_many_requests"
	WebSearchToolResultErrorCodeQueryTooLong     WebSearchToolResultErrorCode = "query_too_long"
)

// Purpose: Fetches multiple CRM objects of the same object type in a single request.
//
// Returns: A list of CRM objects with their properties, identified by their unique IDs, containing:
// - id: Unique identifier for the CRM object
// - properties: Key-value pairs of property names and their values
// - createdAt: Timestamp when the object was created
// - updatedAt: Timestamp when the object was last updated
// - url: URL to view the object in HubSpot
//
// Usage Guidance:
//  1. Use the `search_crm_objects` tool to list a few objects first without a filter criteria.
//  2. Then use the `get_crm_objects` tool to retrieve those objects by their IDs without any properties in the tool input to understand the data model.
//  3. This will help you understand the structure of the objects and their properties.
type CreateMessageParams struct {
	// The maximum number of tokens to generate before stopping.
	//
	// Note that our models may stop _before_ reaching this maximum. This parameter only specifies the absolute maximum number of tokens to generate.
	//
	// Different models have different maximum values for this parameter.  See [models](https://docs.anthropic.com/en/docs/models-overview) for details.
	MaxTokens int `json:"max_tokens"`

	// Input messages.
	//
	// Our models are trained to operate on alternating `user` and `assistant` conversational turns. When creating a new `Message`, you specify the prior conversational turns with the `messages` parameter, and the model then generates the next `Message` in the conversation. Consecutive `user` or `assistant` turns in your request will be combined into a single turn.
	//
	// Each input message must be an object with a `role` and `content`. You can specify a single `user`-role message, or you can include multiple `user` and `assistant` messages.
	//
	// If the final message uses the `assistant` role, the response content will continue immediately from the content in that message. This can be used to constrain part of the model's response.
	//
	// Example with a single `user` message:
	//
	// ```json
	// [{"role": "user", "content": "Hello, Claude"}]
	// ```
	//
	// Example with multiple conversational turns:
	//
	// ```json
	// [
	//   {"role": "user", "content": "Hello there."},
	//   {"role": "assistant", "content": "Hi, I'm Claude. How can I help you?"},
	//   {"role": "user", "content": "Can you explain LLMs in plain English?"},
	// ]
	// ```
	//
	// Example with a partially-filled response from Claude:
	//
	// ```json
	// [
	//   {"role": "user", "content": "What's the Greek name for Sun? (A) Sol (B) Helios (C) Sun"},
	//   {"role": "assistant", "content": "The best answer is ("},
	// ]
	// ```
	//
	// Each input message `content` may be either a single `string` or an array of content blocks, where each block has a specific `type`. Using a `string` for `content` is shorthand for an array of one content block of type `"text"`. The following input messages are equivalent:
	//
	// ```json
	// {"role": "user", "content": "Hello, Claude"}
	// ```
	//
	// ```json
	// {"role": "user", "content": [{"type": "text", "text": "Hello, Claude"}]}
	// ```
	//
	// See [input examples](https://docs.anthropic.com/en/api/messages-examples).
	//
	// Note that if you want to include a [system prompt](https://docs.anthropic.com/en/docs/system-prompts), you can use the top-level `system` parameter — there is no `"system"` role for input messages in the Messages API.
	//
	// There is a limit of 100,000 messages in a single request.
	Messages []InputMessage `json:"messages"`

	// An object describing metadata about the request.
	Metadata *Metadata `json:"metadata,omitempty"`

	Model Model `json:"model"`

	// Determines whether to use priority capacity (if available) or standard capacity for this request.
	//
	// Anthropic offers different levels of service for your API requests. See [service-tiers](https://docs.anthropic.com/en/api/service-tiers) for details.
	ServiceTier *ServiceTier `json:"service_tier,omitempty"`

	// Custom text sequences that will cause the model to stop generating.
	//
	// Our models will normally stop when they have naturally completed their turn, which will result in a response `stop_reason` of `"end_turn"`.
	//
	// If you want the model to stop generating when it encounters custom strings of text, you can use the `stop_sequences` parameter. If the model encounters one of the custom sequences, the response `stop_reason` value will be `"stop_sequence"` and the response `stop_sequence` value will contain the matched stop sequence.
	StopSequences []string `json:"stop_sequences,omitempty"`

	// Whether to incrementally stream the response using server-sent events.
	//
	// See [streaming](https://docs.anthropic.com/en/api/messages-streaming) for details.
	Stream *bool `json:"stream,omitempty"`

	// System prompt.
	//
	// A system prompt is a way of providing context and instructions to Claude, such as specifying a particular goal or role. See our [guide to system prompts](https://docs.anthropic.com/en/docs/system-prompts).
	System interface{} `json:"system,omitempty"` // string or []RequestTextBlock

	// Amount of randomness injected into the response.
	//
	// Defaults to `1.0`. Ranges from `0.0` to `1.0`. Use `temperature` closer to `0.0` for analytical / multiple choice, and closer to `1.0` for creative and generative tasks.
	//
	// Note that even with `temperature` of `0.0`, the results will not be fully deterministic.
	Temperature *float64 `json:"temperature,omitempty"`

	Thinking *ThinkingConfigParam `json:"thinking,omitempty"`

	ToolChoice *ToolChoice `json:"tool_choice,omitempty"`

	// Definitions of tools that the model may use.
	//
	// If you include `tools` in your API request, the model may return `tool_use` content blocks that represent the model's use of those tools. You can then run those tools using the tool input generated by the model and then optionally return results back to the model using `tool_result` content blocks.
	//
	// There are two types of tools: **client tools** and **server tools**. The behavior described below applies to client tools. For [server tools](https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/overview\#server-tools), see their individual documentation as each has its own behavior (e.g., the [web search tool](https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/web-search-tool)).
	//
	// Each tool definition includes:
	//
	// * `name`: Name of the tool.
	// * `description`: Optional, but strongly-recommended description of the tool.
	// * `input_schema`: [JSON schema](https://json-schema.org/draft/2020-12) for the tool `input` shape that the model will produce in `tool_use` output content blocks.
	//
	// For example, if you defined `tools` as:
	//
	// ```json
	// [
	//   {
	//     "name": "get_stock_price",
	//     "description": "Get the current stock price for a given ticker symbol.",
	//     "input_schema": {
	//       "type": "object",
	//       "properties": {
	//         "ticker": {
	//           "type": "string",
	//           "description": "The stock ticker symbol, e.g. AAPL for Apple Inc."
	//         }
	//       },
	//       "required": ["ticker"]
	//     }
	//   }
	// ]
	// ```
	//
	// And then asked the model "What's the S&P 500 at today?", the model might produce `tool_use` content blocks in the response like this:
	//
	// ```json
	// [
	//   {
	//     "type": "tool_use",
	//     "id": "toolu_01D7FLrfh4GYq7yT1ULFeyMV",
	//     "name": "get_stock_price",
	//     "input": { "ticker": "^GSPC" }
	//   }
	// ]
	// ```
	//
	// You might then run your `get_stock_price` tool with `{"ticker": "^GSPC"}` as an input, and return the following back to the model in a subsequent `user` message:
	//
	// ```json
	// [
	//   {
	//     "type": "tool_result",
	//     "tool_use_id": "toolu_01D7FLrfh4GYq7yT1ULFeyMV",
	//     "content": "259.75 USD"
	//   }
	// ]
	// ```
	//
	// Tools can be used for workflows that include running client-side tools and functions, or more generally whenever you want the model to produce a particular JSON structure of output.
	//
	// See our [guide](https://docs.anthropic.com/en/docs/tool-use) for more details.
	Tools []interface{} `json:"tools,omitempty"` // oneOf Tool, BashTool_20250124, TextEditor_20250124, TextEditor_20250429, TextEditor_20250728, WebSearchTool_20250305

	// Only sample from the top K options for each subsequent token.
	//
	// Used to remove "long tail" low probability responses. [Learn more technical details here](https://towardsdatascience.com/how-to-sample-from-language-models-682bceb97277).
	//
	// Recommended for advanced use cases only. You usually only need to use `temperature`.
	TopK *int `json:"top_k,omitempty"`

	// Use nucleus sampling.
	//
	// In nucleus sampling, we compute the cumulative distribution over all the options for each subsequent token in decreasing probability order and cut it off once it reaches a particular probability specified by `top_p`. You should either alter `temperature` or `top_p`, but not both.
	//
	// Recommended for advanced use cases only. You usually only need to use `temperature`.
	TopP *float64 `json:"top_p,omitempty"`
}

type Message struct {
	// Content generated by the model.
	//
	// This is an array of content blocks, each of which has a `type` that determines its shape.
	//
	// Example:
	//
	// ```json
	// [{"type": "text", "text": "Hi, I'm Claude."}]
	// ```
	//
	// If the request input `messages` ended with an `assistant` turn, then the response `content` will continue directly from that last turn. You can use this to constrain the model's output.
	//
	// For example, if the input `messages` were:
	// ```json
	// [
	//   {"role": "user", "content": "What's the Greek name for Sun? (A) Sol (B) Helios (C) Sun"},
	//   {"role": "assistant", "content": "The best answer is ("}
	// ]
	// ```
	//
	// Then the response `content` might be:
	//
	// ```json
	// [{"type": "text", "text": "B)"}]
	// ```
	Content []ContentBlock `json:"content"`

	// Unique object identifier.
	//
	// The format and length of IDs may change over time.
	ID string `json:"id"`

	Model Model `json:"model"`

	// Conversational role of the generated message.
	//
	// This will always be `"assistant"`.
	Role string `json:"role"`

	// The reason that we stopped.
	//
	// This may be one the following values:
	// * `"end_turn"`: the model reached a natural stopping point
	// * `"max_tokens"`: we exceeded the requested `max_tokens` or the model's maximum
	// * `"stop_sequence"`: one of your provided custom `stop_sequences` was generated
	// * `"tool_use"`: the model invoked one or more tools
	// * `"pause_turn"`: we paused a long-running turn. You may provide the response back as-is in a subsequent request to let the model continue.
	// * `"refusal"`: when streaming classifiers intervene to handle potential policy violations
	//
	// In non-streaming mode this value is always non-null. In streaming mode, it is null in the `message_start` event and non-null otherwise.
	StopReason *StopReason `json:"stop_reason"`

	// Which custom stop sequence was generated, if any.
	//
	// This value will be a non-null string if one of your custom stop sequences was generated.
	StopSequence *string `json:"stop_sequence"`

	// Object type.
	//
	// For Messages, this is always `"message"`.
	Type string `json:"type"`

	// Billing and rate-limit usage.
	//
	// Anthropic's API bills and rate-limits by token counts, as tokens represent the underlying cost to our systems.
	//
	// Under the hood, the API transforms requests into a format suitable for the model. The model's output then goes through a parsing stage before becoming an API response. As a result, the token counts in `usage` will not match one-to-one with the exact visible content of an API request or response.
	//
	// For example, `output_tokens` will be non-zero, even for an empty string response from Claude.
	//
	// Total input tokens in a request is the summation of `input_tokens`, `cache_creation_input_tokens`, and `cache_read_input_tokens`.
	Usage Usage `json:"usage"`
}

type MessageStreamEvent struct {
	MessageStartEvent      *MessageStartEvent
	MessageDeltaEvent      *MessageDeltaEvent
	MessageStopEvent       *MessageStopEvent
	ContentBlockStartEvent *ContentBlockStartEvent
	ContentBlockDeltaEvent *ContentBlockDeltaEvent
	ContentBlockStopEvent  *ContentBlockStopEvent
}

func (m *MessageStreamEvent) MarshalJSON() ([]byte, error) {
	if m.MessageStartEvent != nil {
		type Alias struct {
			Type string `json:"type"`
			*MessageStartEvent
		}
		return json.Marshal(&Alias{
			Type:              "message_start",
			MessageStartEvent: m.MessageStartEvent,
		})
	}
	if m.MessageDeltaEvent != nil {
		type Alias struct {
			Type string `json:"type"`
			*MessageDeltaEvent
		}
		return json.Marshal(&Alias{
			Type:              "message_delta",
			MessageDeltaEvent: m.MessageDeltaEvent,
		})
	}
	if m.MessageStopEvent != nil {
		type Alias struct {
			Type string `json:"type"`
			*MessageStopEvent
		}
		return json.Marshal(&Alias{
			Type:             "message_stop",
			MessageStopEvent: m.MessageStopEvent,
		})
	}
	if m.ContentBlockStartEvent != nil {
		type Alias struct {
			Type string `json:"type"`
			*ContentBlockStartEvent
		}
		return json.Marshal(&Alias{
			Type:                   "content_block_start",
			ContentBlockStartEvent: m.ContentBlockStartEvent,
		})
	}
	if m.ContentBlockDeltaEvent != nil {
		type Alias struct {
			Type string `json:"type"`
			*ContentBlockDeltaEvent
		}
		return json.Marshal(&Alias{
			Type:                   "content_block_delta",
			ContentBlockDeltaEvent: m.ContentBlockDeltaEvent,
		})
	}
	if m.ContentBlockStopEvent != nil {
		type Alias struct {
			Type string `json:"type"`
			*ContentBlockStopEvent
		}
		return json.Marshal(&Alias{
			Type:                  "content_block_stop",
			ContentBlockStopEvent: m.ContentBlockStopEvent,
		})
	}
	return nil, errors.New("MessageStreamEvent must have one variant set")
}

func (m *MessageStreamEvent) UnmarshalJSON(data []byte) error {
	var aux struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	switch aux.Type {
	case "message_start":
		var event MessageStartEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return err
		}
		m.MessageStartEvent = &event
	case "message_delta":
		var event MessageDeltaEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return err
		}
		m.MessageDeltaEvent = &event
	case "message_stop":
		var event MessageStopEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return err
		}
		m.MessageStopEvent = &event
	case "content_block_start":
		var event ContentBlockStartEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return err
		}
		m.ContentBlockStartEvent = &event
	case "content_block_delta":
		var event ContentBlockDeltaEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return err
		}
		m.ContentBlockDeltaEvent = &event
	case "content_block_stop":
		var event ContentBlockStopEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return err
		}
		m.ContentBlockStopEvent = &event
	default:
	}
	return nil
}

type InputMessage struct {
	Content interface{} `json:"content"` // string or []InputContentBlock
	Role    Role        `json:"role"`
}

type Metadata struct {
	// An external identifier for the user who is associated with the request.
	//
	// This should be a uuid, hash value, or other opaque identifier. Anthropic may use this id to help detect abuse. Do not include any identifying information such as name, email address, or phone number.
	UserID *string `json:"user_id,omitempty"`
}

type RequestTextBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *CacheControlEphemeral `json:"cache_control,omitempty"`
	Citations    []interface{}          `json:"citations,omitempty"` // Various citation types
	Text         string                 `json:"text"`
	Type         string                 `json:"type"`
}

// Configuration for enabling Claude's extended thinking.
//
// When enabled, responses include `thinking` content blocks showing Claude's thinking process before the final answer. Requires a minimum budget of 1,024 tokens and counts towards your `max_tokens` limit.
//
// See [extended thinking](https://docs.anthropic.com/en/docs/build-with-claude/extended-thinking) for details.
type ThinkingConfigParam struct {
	Enabled  *ThinkingConfigEnabled
	Disabled *ThinkingConfigDisabled
}

func (t *ThinkingConfigParam) MarshalJSON() ([]byte, error) {
	if t.Enabled != nil {
		type Alias struct {
			Type string `json:"type"`
			*ThinkingConfigEnabled
		}
		return json.Marshal(&Alias{
			Type:                  "enabled",
			ThinkingConfigEnabled: t.Enabled,
		})
	}
	if t.Disabled != nil {
		type Alias struct {
			Type string `json:"type"`
			*ThinkingConfigDisabled
		}
		return json.Marshal(&Alias{
			Type:                   "disabled",
			ThinkingConfigDisabled: t.Disabled,
		})
	}
	return nil, errors.New("ThinkingConfigParam must have either Enabled or Disabled set")
}

func (t *ThinkingConfigParam) UnmarshalJSON(data []byte) error {
	var aux struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	switch aux.Type {
	case "enabled":
		var enabled ThinkingConfigEnabled
		if err := json.Unmarshal(data, &enabled); err != nil {
			return err
		}
		t.Enabled = &enabled
	case "disabled":
		var disabled ThinkingConfigDisabled
		if err := json.Unmarshal(data, &disabled); err != nil {
			return err
		}
		t.Disabled = &disabled
	default:
		return fmt.Errorf("unknown type: %s", aux.Type)
	}
	return nil
}

// How the model should use the provided tools. The model can use a specific tool, any available tool, decide by itself, or not use tools at all.
type ToolChoice struct {
	Auto *ToolChoiceAuto
	Any  *ToolChoiceAny
	Tool *ToolChoiceTool
	None *ToolChoiceNone
}

func (t *ToolChoice) MarshalJSON() ([]byte, error) {
	if t.Auto != nil {
		type Alias struct {
			Type string `json:"type"`
			*ToolChoiceAuto
		}
		return json.Marshal(&Alias{
			Type:           "auto",
			ToolChoiceAuto: t.Auto,
		})
	}
	if t.Any != nil {
		type Alias struct {
			Type string `json:"type"`
			*ToolChoiceAny
		}
		return json.Marshal(&Alias{
			Type:          "any",
			ToolChoiceAny: t.Any,
		})
	}
	if t.Tool != nil {
		type Alias struct {
			Type string `json:"type"`
			*ToolChoiceTool
		}
		return json.Marshal(&Alias{
			Type:           "tool",
			ToolChoiceTool: t.Tool,
		})
	}
	if t.None != nil {
		type Alias struct {
			Type string `json:"type"`
			*ToolChoiceNone
		}
		return json.Marshal(&Alias{
			Type:           "none",
			ToolChoiceNone: t.None,
		})
	}
	return nil, errors.New("ToolChoice must have one variant set")
}

func (t *ToolChoice) UnmarshalJSON(data []byte) error {
	var aux struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	switch aux.Type {
	case "auto":
		var auto ToolChoiceAuto
		if err := json.Unmarshal(data, &auto); err != nil {
			return err
		}
		t.Auto = &auto
	case "any":
		var any ToolChoiceAny
		if err := json.Unmarshal(data, &any); err != nil {
			return err
		}
		t.Any = &any
	case "tool":
		var tool ToolChoiceTool
		if err := json.Unmarshal(data, &tool); err != nil {
			return err
		}
		t.Tool = &tool
	case "none":
		var none ToolChoiceNone
		if err := json.Unmarshal(data, &none); err != nil {
			return err
		}
		t.None = &none
	default:
		return fmt.Errorf("unknown type: %s", aux.Type)
	}
	return nil
}

type Tool struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *CacheControlEphemeral `json:"cache_control,omitempty"`
	// Description of what this tool does.
	//
	// Tool descriptions should be as detailed as possible. The more information that the model has about what the tool is and how to use it, the better it will perform. You can use natural language descriptions to reinforce important aspects of the tool input JSON schema.
	Description *string `json:"description,omitempty"`
	// [JSON schema](https://json-schema.org/draft/2020-12) for this tool's input.
	//
	// This defines the shape of the `input` that your tool accepts and that the model will produce.
	InputSchema InputSchema `json:"input_schema"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string  `json:"name"`
	Type *string `json:"type,omitempty"`
}

type BashTool20250124 struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *CacheControlEphemeral `json:"cache_control,omitempty"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string `json:"name"`
	Type string `json:"type"`
}

type TextEditor20250124 struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *CacheControlEphemeral `json:"cache_control,omitempty"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string `json:"name"`
	Type string `json:"type"`
}

type TextEditor20250429 struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *CacheControlEphemeral `json:"cache_control,omitempty"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string `json:"name"`
	Type string `json:"type"`
}

type TextEditor20250728 struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *CacheControlEphemeral `json:"cache_control,omitempty"`
	// Maximum number of characters to display when viewing a file. If not specified, defaults to displaying the full file.
	MaxCharacters *int `json:"max_characters,omitempty"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string `json:"name"`
	Type string `json:"type"`
}

type WebSearchTool20250305 struct {
	// If provided, only these domains will be included in results. Cannot be used alongside `blocked_domains`.
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	// If provided, these domains will never appear in results. Cannot be used alongside `allowed_domains`.
	BlockedDomains []string `json:"blocked_domains,omitempty"`
	// Create a cache control breakpoint at this content block.
	CacheControl *CacheControlEphemeral `json:"cache_control,omitempty"`
	// Maximum number of times the tool can be used in the API request.
	MaxUses *int `json:"max_uses,omitempty"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string `json:"name"`
	Type string `json:"type"`
	// Parameters for the user's location. Used to provide more relevant search results.
	UserLocation *UserLocation `json:"user_location,omitempty"`
}

type ContentBlock struct {
	ResponseTextBlock                *ResponseTextBlock
	ResponseThinkingBlock            *ResponseThinkingBlock
	ResponseRedactedThinkingBlock    *ResponseRedactedThinkingBlock
	ResponseToolUseBlock             *ResponseToolUseBlock
	ResponseServerToolUseBlock       *ResponseServerToolUseBlock
	ResponseWebSearchToolResultBlock *ResponseWebSearchToolResultBlock
}

func (c *ContentBlock) MarshalJSON() ([]byte, error) {
	if c.ResponseTextBlock != nil {
		type Alias struct {
			Type string `json:"type"`
			*ResponseTextBlock
		}
		return json.Marshal(&Alias{
			Type:              "text",
			ResponseTextBlock: c.ResponseTextBlock,
		})
	}
	if c.ResponseThinkingBlock != nil {
		type Alias struct {
			Type string `json:"type"`
			*ResponseThinkingBlock
		}
		return json.Marshal(&Alias{
			Type:                  "thinking",
			ResponseThinkingBlock: c.ResponseThinkingBlock,
		})
	}
	if c.ResponseRedactedThinkingBlock != nil {
		type Alias struct {
			Type string `json:"type"`
			*ResponseRedactedThinkingBlock
		}
		return json.Marshal(&Alias{
			Type:                          "redacted_thinking",
			ResponseRedactedThinkingBlock: c.ResponseRedactedThinkingBlock,
		})
	}
	if c.ResponseToolUseBlock != nil {
		type Alias struct {
			Type string `json:"type"`
			*ResponseToolUseBlock
		}
		return json.Marshal(&Alias{
			Type:                 "tool_use",
			ResponseToolUseBlock: c.ResponseToolUseBlock,
		})
	}
	if c.ResponseServerToolUseBlock != nil {
		type Alias struct {
			Type string `json:"type"`
			*ResponseServerToolUseBlock
		}
		return json.Marshal(&Alias{
			Type:                       "server_tool_use",
			ResponseServerToolUseBlock: c.ResponseServerToolUseBlock,
		})
	}
	if c.ResponseWebSearchToolResultBlock != nil {
		type Alias struct {
			Type string `json:"type"`
			*ResponseWebSearchToolResultBlock
		}
		return json.Marshal(&Alias{
			Type:                             "web_search_tool_result",
			ResponseWebSearchToolResultBlock: c.ResponseWebSearchToolResultBlock,
		})
	}
	return nil, errors.New("ContentBlock must have one variant set")
}

func (c *ContentBlock) UnmarshalJSON(data []byte) error {
	var aux struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	switch aux.Type {
	case "text":
		var block ResponseTextBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return err
		}
		c.ResponseTextBlock = &block
	case "thinking":
		var block ResponseThinkingBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return err
		}
		c.ResponseThinkingBlock = &block
	case "redacted_thinking":
		var block ResponseRedactedThinkingBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return err
		}
		c.ResponseRedactedThinkingBlock = &block
	case "tool_use":
		var block ResponseToolUseBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return err
		}
		c.ResponseToolUseBlock = &block
	case "server_tool_use":
		var block ResponseServerToolUseBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return err
		}
		c.ResponseServerToolUseBlock = &block
	case "web_search_tool_result":
		var block ResponseWebSearchToolResultBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return err
		}
		c.ResponseWebSearchToolResultBlock = &block
	default:
		return fmt.Errorf("unknown type: %s", aux.Type)
	}
	return nil
}

type Usage struct {
	// Breakdown of cached tokens by TTL
	CacheCreation *CacheCreation `json:"cache_creation"`
	// The number of input tokens used to create the cache entry.
	CacheCreationInputTokens *int `json:"cache_creation_input_tokens"`
	// The number of input tokens read from the cache.
	CacheReadInputTokens *int `json:"cache_read_input_tokens"`
	// The number of input tokens which were used.
	InputTokens int `json:"input_tokens"`
	// The number of output tokens which were used.
	OutputTokens int `json:"output_tokens"`
	// The number of server tool requests.
	ServerToolUse *ServerToolUsage `json:"server_tool_use"`
	// If the request used the priority, standard, or batch tier.
	ServiceTier *string `json:"service_tier"`
}

type MessageStartEvent struct {
	Message Message `json:"message"`
}

type MessageDeltaEvent struct {
	Delta MessageDelta `json:"delta"`
	// Billing and rate-limit usage.
	//
	// Anthropic's API bills and rate-limits by token counts, as tokens represent the underlying cost to our systems.
	//
	// Under the hood, the API transforms requests into a format suitable for the model. The model's output then goes through a parsing stage before becoming an API response. As a result, the token counts in `usage` will not match one-to-one with the exact visible content of an API request or response.
	//
	// For example, `output_tokens` will be non-zero, even for an empty string response from Claude.
	//
	// Total input tokens in a request is the summation of `input_tokens`, `cache_creation_input_tokens`, and `cache_read_input_tokens`.
	Usage MessageDeltaUsage `json:"usage"`
}

type MessageStopEvent struct {
}

type ContentBlockStartEvent struct {
	ContentBlock interface{} `json:"content_block"` // Union of content blocks
	Index        int         `json:"index"`
}

type ContentBlockDeltaEvent struct {
	Delta interface{} `json:"delta"` // Union of deltas
	Index int         `json:"index"`
}

type ContentBlockStopEvent struct {
	Index int `json:"index"`
}

type InputContentBlock struct {
	RequestTextBlock                *RequestTextBlock
	RequestImageBlock               *RequestImageBlock
	RequestDocumentBlock            *RequestDocumentBlock
	RequestSearchResultBlock        *RequestSearchResultBlock
	RequestThinkingBlock            *RequestThinkingBlock
	RequestRedactedThinkingBlock    *RequestRedactedThinkingBlock
	RequestToolUseBlock             *RequestToolUseBlock
	RequestToolResultBlock          *RequestToolResultBlock
	RequestServerToolUseBlock       *RequestServerToolUseBlock
	RequestWebSearchToolResultBlock *RequestWebSearchToolResultBlock
}

func (i *InputContentBlock) MarshalJSON() ([]byte, error) {
	if i.RequestTextBlock != nil {
		type Alias struct {
			Type string `json:"type"`
			*RequestTextBlock
		}
		return json.Marshal(&Alias{
			Type:             "text",
			RequestTextBlock: i.RequestTextBlock,
		})
	}
	if i.RequestImageBlock != nil {
		type Alias struct {
			Type string `json:"type"`
			*RequestImageBlock
		}
		return json.Marshal(&Alias{
			Type:              "image",
			RequestImageBlock: i.RequestImageBlock,
		})
	}
	if i.RequestDocumentBlock != nil {
		type Alias struct {
			Type string `json:"type"`
			*RequestDocumentBlock
		}
		return json.Marshal(&Alias{
			Type:                 "document",
			RequestDocumentBlock: i.RequestDocumentBlock,
		})
	}
	if i.RequestSearchResultBlock != nil {
		type Alias struct {
			Type string `json:"type"`
			*RequestSearchResultBlock
		}
		return json.Marshal(&Alias{
			Type:                     "search_result",
			RequestSearchResultBlock: i.RequestSearchResultBlock,
		})
	}
	if i.RequestThinkingBlock != nil {
		type Alias struct {
			Type string `json:"type"`
			*RequestThinkingBlock
		}
		return json.Marshal(&Alias{
			Type:                 "thinking",
			RequestThinkingBlock: i.RequestThinkingBlock,
		})
	}
	if i.RequestRedactedThinkingBlock != nil {
		type Alias struct {
			Type string `json:"type"`
			*RequestRedactedThinkingBlock
		}
		return json.Marshal(&Alias{
			Type:                         "redacted_thinking",
			RequestRedactedThinkingBlock: i.RequestRedactedThinkingBlock,
		})
	}
	if i.RequestToolUseBlock != nil {
		type Alias struct {
			Type string `json:"type"`
			*RequestToolUseBlock
		}
		return json.Marshal(&Alias{
			Type:                "tool_use",
			RequestToolUseBlock: i.RequestToolUseBlock,
		})
	}
	if i.RequestToolResultBlock != nil {
		type Alias struct {
			Type string `json:"type"`
			*RequestToolResultBlock
		}
		return json.Marshal(&Alias{
			Type:                   "tool_result",
			RequestToolResultBlock: i.RequestToolResultBlock,
		})
	}
	if i.RequestServerToolUseBlock != nil {
		type Alias struct {
			Type string `json:"type"`
			*RequestServerToolUseBlock
		}
		return json.Marshal(&Alias{
			Type:                      "server_tool_use",
			RequestServerToolUseBlock: i.RequestServerToolUseBlock,
		})
	}
	if i.RequestWebSearchToolResultBlock != nil {
		type Alias struct {
			Type string `json:"type"`
			*RequestWebSearchToolResultBlock
		}
		return json.Marshal(&Alias{
			Type:                            "web_search_tool_result",
			RequestWebSearchToolResultBlock: i.RequestWebSearchToolResultBlock,
		})
	}
	return nil, errors.New("InputContentBlock must have one variant set")
}

func (i *InputContentBlock) UnmarshalJSON(data []byte) error {
	var aux struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	switch aux.Type {
	case "text":
		var block RequestTextBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return err
		}
		i.RequestTextBlock = &block
	case "image":
		var block RequestImageBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return err
		}
		i.RequestImageBlock = &block
	case "document":
		var block RequestDocumentBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return err
		}
		i.RequestDocumentBlock = &block
	case "search_result":
		var block RequestSearchResultBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return err
		}
		i.RequestSearchResultBlock = &block
	case "thinking":
		var block RequestThinkingBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return err
		}
		i.RequestThinkingBlock = &block
	case "redacted_thinking":
		var block RequestRedactedThinkingBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return err
		}
		i.RequestRedactedThinkingBlock = &block
	case "tool_use":
		var block RequestToolUseBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return err
		}
		i.RequestToolUseBlock = &block
	case "tool_result":
		var block RequestToolResultBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return err
		}
		i.RequestToolResultBlock = &block
	case "server_tool_use":
		var block RequestServerToolUseBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return err
		}
		i.RequestServerToolUseBlock = &block
	case "web_search_tool_result":
		var block RequestWebSearchToolResultBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return err
		}
		i.RequestWebSearchToolResultBlock = &block
	default:
		return fmt.Errorf("unknown type: %s", aux.Type)
	}
	return nil
}

type CacheControlEphemeral struct {
	// The time-to-live for the cache control breakpoint.
	//
	// This may be one the following values:
	// - `5m`: 5 minutes
	// - `1h`: 1 hour
	//
	// Defaults to `5m`.
	TTL *string `json:"ttl,omitempty"`
}

type RequestCharLocationCitation struct {
	CitedText      string  `json:"cited_text"`
	DocumentIndex  int     `json:"document_index"`
	DocumentTitle  *string `json:"document_title"`
	EndCharIndex   int     `json:"end_char_index"`
	StartCharIndex int     `json:"start_char_index"`
}

type RequestPageLocationCitation struct {
	CitedText       string  `json:"cited_text"`
	DocumentIndex   int     `json:"document_index"`
	DocumentTitle   *string `json:"document_title"`
	EndPageNumber   int     `json:"end_page_number"`
	StartPageNumber int     `json:"start_page_number"`
}

type RequestContentBlockLocationCitation struct {
	CitedText       string  `json:"cited_text"`
	DocumentIndex   int     `json:"document_index"`
	DocumentTitle   *string `json:"document_title"`
	EndBlockIndex   int     `json:"end_block_index"`
	StartBlockIndex int     `json:"start_block_index"`
}

type RequestWebSearchResultLocationCitation struct {
	CitedText      string  `json:"cited_text"`
	EncryptedIndex string  `json:"encrypted_index"`
	Title          *string `json:"title"`
	URL            string  `json:"url"`
}

type RequestSearchResultLocationCitation struct {
	CitedText         string  `json:"cited_text"`
	EndBlockIndex     int     `json:"end_block_index"`
	SearchResultIndex int     `json:"search_result_index"`
	Source            string  `json:"source"`
	StartBlockIndex   int     `json:"start_block_index"`
	Title             *string `json:"title"`
}

type ThinkingConfigEnabled struct {
	// Determines how many tokens Claude can use for its internal reasoning process. Larger budgets can enable more thorough analysis for complex problems, improving response quality.
	//
	// Must be ≥1024 and less than `max_tokens`.
	//
	// See [extended thinking](https://docs.anthropic.com/en/docs/build-with-claude/extended-thinking) for details.
	BudgetTokens int `json:"budget_tokens"`
}

type ThinkingConfigDisabled struct {
}

// The model will automatically decide whether to use tools.
type ToolChoiceAuto struct {
	// Whether to disable parallel tool use.
	//
	// Defaults to `false`. If set to `true`, the model will output at most one tool use.
	DisableParallelToolUse *bool `json:"disable_parallel_tool_use,omitempty"`
}

// The model will use any available tools.
type ToolChoiceAny struct {
	// Whether to disable parallel tool use.
	//
	// Defaults to `false`. If set to `true`, the model will output exactly one tool use.
	DisableParallelToolUse *bool `json:"disable_parallel_tool_use,omitempty"`
}

// The model will use the specified tool with `tool_choice.name`.
type ToolChoiceTool struct {
	// Whether to disable parallel tool use.
	//
	// Defaults to `false`. If set to `true`, the model will output exactly one tool use.
	DisableParallelToolUse *bool `json:"disable_parallel_tool_use,omitempty"`
	// The name of the tool to use.
	Name string `json:"name"`
}

// The model will not be allowed to use tools.
type ToolChoiceNone struct {
}

type InputSchema struct {
	Properties map[string]interface{} `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
	Type       string                 `json:"type"`
}

type UserLocation struct {
	// The city of the user.
	City *string `json:"city,omitempty"`
	// The two letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2) of the user.
	Country *string `json:"country,omitempty"`
	// The region of the user.
	Region *string `json:"region,omitempty"`
	// The [IANA timezone](https://nodatime.org/TimeZones) of the user.
	Timezone *string `json:"timezone,omitempty"`
	Type     string  `json:"type"`
}

type ResponseTextBlock struct {
	// Citations supporting the text block.
	//
	// The type of citation returned will depend on the type of document being cited. Citing a PDF results in `page_location`, plain text results in `char_location`, and content document results in `content_block_location`.
	Citations []interface{} `json:"citations"`
	Text      string        `json:"text"`
}

type ResponseThinkingBlock struct {
	Signature string `json:"signature"`
	Thinking  string `json:"thinking"`
}

type ResponseRedactedThinkingBlock struct {
	Data string `json:"data"`
}

type ResponseToolUseBlock struct {
	ID    string                 `json:"id"`
	Input map[string]interface{} `json:"input"`
	Name  string                 `json:"name"`
}

type ResponseServerToolUseBlock struct {
	ID    string                 `json:"id"`
	Input map[string]interface{} `json:"input"`
	Name  string                 `json:"name"`
}

type ResponseWebSearchToolResultBlock struct {
	Content   interface{} `json:"content"` // ResponseWebSearchToolResultError or []ResponseWebSearchResultBlock
	ToolUseID string      `json:"tool_use_id"`
}

type CacheCreation struct {
	// The number of input tokens used to create the 1 hour cache entry.
	Ephemeral1hInputTokens int `json:"ephemeral_1h_input_tokens"`
	// The number of input tokens used to create the 5 minute cache entry.
	Ephemeral5mInputTokens int `json:"ephemeral_5m_input_tokens"`
}

type ServerToolUsage struct {
	// The number of web search tool requests.
	WebSearchRequests int `json:"web_search_requests"`
}

type MessageDelta struct {
	StopReason   *StopReason `json:"stop_reason"`
	StopSequence *string     `json:"stop_sequence"`
}

type MessageDeltaUsage struct {
	// The cumulative number of input tokens used to create the cache entry.
	CacheCreationInputTokens *int `json:"cache_creation_input_tokens"`
	// The cumulative number of input tokens read from the cache.
	CacheReadInputTokens *int `json:"cache_read_input_tokens"`
	// The cumulative number of input tokens which were used.
	InputTokens *int `json:"input_tokens"`
	// The cumulative number of output tokens which were used.
	OutputTokens int `json:"output_tokens"`
	// The number of server tool requests.
	ServerToolUse *ServerToolUsage `json:"server_tool_use"`
}

type TextContentBlockDelta struct {
	Text string `json:"text"`
}

type InputJsonContentBlockDelta struct {
	PartialJSON string `json:"partial_json"`
}

type CitationsDelta struct {
	Citation interface{} `json:"citation"` // Various citation types
}

type ThinkingContentBlockDelta struct {
	Thinking string `json:"thinking"`
}

type SignatureContentBlockDelta struct {
	Signature string `json:"signature"`
}

// Regular text content.
type RequestImageBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *CacheControlEphemeral `json:"cache_control,omitempty"`
	Source       interface{}            `json:"source"` // Base64ImageSource or URLImageSource
}

type RequestDocumentBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *CacheControlEphemeral  `json:"cache_control,omitempty"`
	Citations    *RequestCitationsConfig `json:"citations,omitempty"`
	Context      *string                 `json:"context,omitempty"`
	Source       interface{}             `json:"source"` // Various source types
	Title        *string                 `json:"title,omitempty"`
}

type RequestSearchResultBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *CacheControlEphemeral  `json:"cache_control,omitempty"`
	Citations    *RequestCitationsConfig `json:"citations,omitempty"`
	Content      []RequestTextBlock      `json:"content"`
	Source       string                  `json:"source"`
	Title        string                  `json:"title"`
}

type RequestThinkingBlock struct {
	Signature string `json:"signature"`
	Thinking  string `json:"thinking"`
}

type RequestRedactedThinkingBlock struct {
	Data string `json:"data"`
}

type RequestToolUseBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *CacheControlEphemeral `json:"cache_control,omitempty"`
	ID           string                 `json:"id"`
	Input        map[string]interface{} `json:"input"`
	Name         string                 `json:"name"`
}

type RequestToolResultBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *CacheControlEphemeral `json:"cache_control,omitempty"`
	Content      interface{}            `json:"content,omitempty"` // string or []Block
	IsError      *bool                  `json:"is_error,omitempty"`
	ToolUseID    string                 `json:"tool_use_id"`
}

type RequestServerToolUseBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *CacheControlEphemeral `json:"cache_control,omitempty"`
	ID           string                 `json:"id"`
	Input        map[string]interface{} `json:"input"`
	Name         string                 `json:"name"`
}

type RequestWebSearchToolResultBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *CacheControlEphemeral `json:"cache_control,omitempty"`
	Content      interface{}            `json:"content"` // []RequestWebSearchResultBlock or RequestWebSearchToolResultError
	ToolUseID    string                 `json:"tool_use_id"`
}

type ResponseCharLocationCitation struct {
	CitedText      string  `json:"cited_text"`
	DocumentIndex  int     `json:"document_index"`
	DocumentTitle  *string `json:"document_title"`
	EndCharIndex   int     `json:"end_char_index"`
	FileID         *string `json:"file_id"`
	StartCharIndex int     `json:"start_char_index"`
}

type ResponsePageLocationCitation struct {
	CitedText       string  `json:"cited_text"`
	DocumentIndex   int     `json:"document_index"`
	DocumentTitle   *string `json:"document_title"`
	EndPageNumber   int     `json:"end_page_number"`
	FileID          *string `json:"file_id"`
	StartPageNumber int     `json:"start_page_number"`
}

type ResponseContentBlockLocationCitation struct {
	CitedText       string  `json:"cited_text"`
	DocumentIndex   int     `json:"document_index"`
	DocumentTitle   *string `json:"document_title"`
	EndBlockIndex   int     `json:"end_block_index"`
	FileID          *string `json:"file_id"`
	StartBlockIndex int     `json:"start_block_index"`
}

type ResponseWebSearchResultLocationCitation struct {
	CitedText      string  `json:"cited_text"`
	EncryptedIndex string  `json:"encrypted_index"`
	Title          *string `json:"title"`
	URL            string  `json:"url"`
}

type ResponseSearchResultLocationCitation struct {
	CitedText         string  `json:"cited_text"`
	EndBlockIndex     int     `json:"end_block_index"`
	SearchResultIndex int     `json:"search_result_index"`
	Source            string  `json:"source"`
	StartBlockIndex   int     `json:"start_block_index"`
	Title             *string `json:"title"`
}

type ResponseWebSearchToolResultError struct {
	ErrorCode WebSearchToolResultErrorCode `json:"error_code"`
}

type ResponseWebSearchResultBlock struct {
	EncryptedContent string  `json:"encrypted_content"`
	PageAge          *string `json:"page_age"`
	Title            string  `json:"title"`
	URL              string  `json:"url"`
}

type Base64ImageSource struct {
	Data      []byte `json:"data"`
	MediaType string `json:"media_type"`
}

type URLImageSource struct {
	URL string `json:"url"`
}

type RequestCitationsConfig struct {
	Enabled *bool `json:"enabled,omitempty"`
}

type Base64PDFSource struct {
	Data      []byte `json:"data"`
	MediaType string `json:"media_type"`
}

type PlainTextSource struct {
	Data      string `json:"data"`
	MediaType string `json:"media_type"`
}

type ContentBlockSource struct {
	Content interface{} `json:"content"` // string or []ContentBlockSourceContentItem
}

type URLPDFSource struct {
	URL string `json:"url"`
}

type RequestWebSearchResultBlock struct {
	EncryptedContent string  `json:"encrypted_content"`
	PageAge          *string `json:"page_age,omitempty"`
	Title            string  `json:"title"`
	URL              string  `json:"url"`
}

type RequestWebSearchToolResultError struct {
	ErrorCode WebSearchToolResultErrorCode `json:"error_code"`
}
