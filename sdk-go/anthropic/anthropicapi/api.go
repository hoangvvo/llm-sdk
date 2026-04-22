package anthropicapi

import (
	"encoding/json"
	"errors"
	"fmt"
)

type CreateMessageParams struct {
	// Top-level cache control automatically applies a cache_control marker to the last cacheable block in the request.
	CacheControl *CreateMessageParamsCacheControl `json:"cache_control,omitempty"`
	// Container identifier for reuse across requests.
	Container *string `json:"container,omitempty"`
	// Specifies the geographic region for inference processing. If not specified, the workspace's `default_inference_geo` is used.
	InferenceGeo *string `json:"inference_geo,omitempty"`
	// The maximum number of tokens to generate before stopping.
	//
	// Note that our models may stop _before_ reaching this maximum. This parameter only specifies the absolute maximum number of tokens to generate.
	//
	// Different models have different maximum values for this parameter.  See [models](https://docs.claude.com/en/docs/models-overview) for details.
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
	// See [input examples](https://docs.claude.com/en/api/messages-examples).
	//
	// Note that if you want to include a [system prompt](https://docs.claude.com/en/docs/system-prompts), you can use the top-level `system` parameter — there is no `"system"` role for input messages in the Messages API.
	//
	// There is a limit of 100,000 messages in a single request.
	Messages []InputMessage `json:"messages"`
	// An object describing metadata about the request.
	Metadata *Metadata `json:"metadata,omitempty"`
	Model    Model     `json:"model"`
	// Configuration options for the model's output, such as the output format.
	OutputConfig *OutputConfig `json:"output_config,omitempty"`
	// Determines whether to use priority capacity (if available) or standard capacity for this request.
	//
	// Anthropic offers different levels of service for your API requests. See [service-tiers](https://docs.claude.com/en/api/service-tiers) for details.
	ServiceTier *CreateMessageParamsServiceTier `json:"service_tier,omitempty"`
	// Custom text sequences that will cause the model to stop generating.
	//
	// Our models will normally stop when they have naturally completed their turn, which will result in a response `stop_reason` of `"end_turn"`.
	//
	// If you want the model to stop generating when it encounters custom strings of text, you can use the `stop_sequences` parameter. If the model encounters one of the custom sequences, the response `stop_reason` value will be `"stop_sequence"` and the response `stop_sequence` value will contain the matched stop sequence.
	StopSequences []string `json:"stop_sequences,omitempty"`
	// Whether to incrementally stream the response using server-sent events.
	//
	// See [streaming](https://docs.claude.com/en/api/messages-streaming) for details.
	Stream *bool `json:"stream,omitempty"`
	// System prompt.
	//
	// A system prompt is a way of providing context and instructions to Claude, such as specifying a particular goal or role. See our [guide to system prompts](https://docs.claude.com/en/docs/system-prompts).
	System *CreateMessageParamsSystem `json:"system,omitempty"`
	// Amount of randomness injected into the response.
	//
	// Defaults to `1.0`. Ranges from `0.0` to `1.0`. Use `temperature` closer to `0.0` for analytical / multiple choice, and closer to `1.0` for creative and generative tasks.
	//
	// Note that even with `temperature` of `0.0`, the results will not be fully deterministic.
	Temperature *float64             `json:"temperature,omitempty"`
	Thinking    *ThinkingConfigParam `json:"thinking,omitempty"`
	ToolChoice  *ToolChoice          `json:"tool_choice,omitempty"`
	// Definitions of tools that the model may use.
	//
	// If you include `tools` in your API request, the model may return `tool_use` content blocks that represent the model's use of those tools. You can then run those tools using the tool input generated by the model and then optionally return results back to the model using `tool_result` content blocks.
	//
	// There are two types of tools: **client tools** and **server tools**. The behavior described below applies to client tools. For [server tools](https://docs.claude.com/en/docs/agents-and-tools/tool-use/overview\#server-tools), see their individual documentation as each has its own behavior (e.g., the [web search tool](https://docs.claude.com/en/docs/agents-and-tools/tool-use/web-search-tool)).
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
	// See our [guide](https://docs.claude.com/en/docs/tool-use) for more details.
	Tools []CreateMessageParamsToolsItem `json:"tools,omitempty"`
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

// Top-level cache control automatically applies a cache_control marker to the last cacheable block in the request.
type CreateMessageParamsCacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *CreateMessageParamsCacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid CreateMessageParamsCacheControl: all variants are nil")
}

func (u *CreateMessageParamsCacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in CreateMessageParamsCacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = CreateMessageParamsCacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in CreateMessageParamsCacheControl: %q", discriminator)
	}
	return nil
}

// Determines whether to use priority capacity (if available) or standard capacity for this request.
//
// Anthropic offers different levels of service for your API requests. See [service-tiers](https://docs.claude.com/en/api/service-tiers) for details.
type CreateMessageParamsServiceTier string

const (
	CreateMessageParamsServiceTierAuto         CreateMessageParamsServiceTier = "auto"
	CreateMessageParamsServiceTierStandardOnly CreateMessageParamsServiceTier = "standard_only"
)

type CreateMessageParamsSystemString *string

type CreateMessageParamsSystemArray []RequestTextBlock

// System prompt.
//
// A system prompt is a way of providing context and instructions to Claude, such as specifying a particular goal or role. See our [guide to system prompts](https://docs.claude.com/en/docs/system-prompts).
type CreateMessageParamsSystem struct {
	CreateMessageParamsSystemString *CreateMessageParamsSystemString
	CreateMessageParamsSystemArray  *CreateMessageParamsSystemArray
}

func (u *CreateMessageParamsSystem) MarshalJSON() ([]byte, error) {
	if u == nil {
		return []byte("null"), nil
	}
	if u.CreateMessageParamsSystemString != nil {
		return json.Marshal(u.CreateMessageParamsSystemString)
	}
	if u.CreateMessageParamsSystemArray != nil {
		return json.Marshal(u.CreateMessageParamsSystemArray)
	}
	return nil, errors.New("invalid CreateMessageParamsSystem: all variants are nil")
}

func (u *CreateMessageParamsSystem) UnmarshalJSON(data []byte) error {
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*u = CreateMessageParamsSystem{}
	switch raw.(type) {
	case string:
		var v CreateMessageParamsSystemString
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		u.CreateMessageParamsSystemString = &v
		return nil
	case []interface{}:
		var v CreateMessageParamsSystemArray
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		u.CreateMessageParamsSystemArray = &v
		return nil
	}
	return errors.New("invalid CreateMessageParamsSystem")
}

type CreateMessageParamsToolsItem struct {
	Tool                        *Tool
	BashTool20250124            *BashTool20250124
	CodeExecutionTool20250522   *CodeExecutionTool20250522
	CodeExecutionTool20250825   *CodeExecutionTool20250825
	CodeExecutionTool20260120   *CodeExecutionTool20260120
	MemoryTool20250818          *MemoryTool20250818
	TextEditor20250124          *TextEditor20250124
	TextEditor20250429          *TextEditor20250429
	TextEditor20250728          *TextEditor20250728
	WebSearchTool20250305       *WebSearchTool20250305
	WebFetchTool20250910        *WebFetchTool20250910
	WebSearchTool20260209       *WebSearchTool20260209
	WebFetchTool20260209        *WebFetchTool20260209
	WebFetchTool20260309        *WebFetchTool20260309
	ToolSearchToolBM2520251119  *ToolSearchToolBM2520251119
	ToolSearchToolRegex20251119 *ToolSearchToolRegex20251119
}

func (u *CreateMessageParamsToolsItem) MarshalJSON() ([]byte, error) {
	if u == nil {
		return []byte("null"), nil
	}
	if u.Tool != nil {
		return json.Marshal(u.Tool)
	}
	if u.BashTool20250124 != nil {
		return json.Marshal(u.BashTool20250124)
	}
	if u.CodeExecutionTool20250522 != nil {
		return json.Marshal(u.CodeExecutionTool20250522)
	}
	if u.CodeExecutionTool20250825 != nil {
		return json.Marshal(u.CodeExecutionTool20250825)
	}
	if u.CodeExecutionTool20260120 != nil {
		return json.Marshal(u.CodeExecutionTool20260120)
	}
	if u.MemoryTool20250818 != nil {
		return json.Marshal(u.MemoryTool20250818)
	}
	if u.TextEditor20250124 != nil {
		return json.Marshal(u.TextEditor20250124)
	}
	if u.TextEditor20250429 != nil {
		return json.Marshal(u.TextEditor20250429)
	}
	if u.TextEditor20250728 != nil {
		return json.Marshal(u.TextEditor20250728)
	}
	if u.WebSearchTool20250305 != nil {
		return json.Marshal(u.WebSearchTool20250305)
	}
	if u.WebFetchTool20250910 != nil {
		return json.Marshal(u.WebFetchTool20250910)
	}
	if u.WebSearchTool20260209 != nil {
		return json.Marshal(u.WebSearchTool20260209)
	}
	if u.WebFetchTool20260209 != nil {
		return json.Marshal(u.WebFetchTool20260209)
	}
	if u.WebFetchTool20260309 != nil {
		return json.Marshal(u.WebFetchTool20260309)
	}
	if u.ToolSearchToolBM2520251119 != nil {
		return json.Marshal(u.ToolSearchToolBM2520251119)
	}
	if u.ToolSearchToolRegex20251119 != nil {
		return json.Marshal(u.ToolSearchToolRegex20251119)
	}
	return nil, errors.New("invalid CreateMessageParamsToolsItem: all variants are nil")
}

func (u *CreateMessageParamsToolsItem) UnmarshalJSON(data []byte) error {
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*u = CreateMessageParamsToolsItem{}
	switch value := raw.(type) {
	case map[string]interface{}:
		if rawType, ok := value["type"]; !ok || rawType == "custom" && value["name"] != nil && value["input_schema"] != nil {
			var v Tool
			if err := json.Unmarshal(data, &v); err != nil {
				return err
			}
			u.Tool = &v
			return nil
		}
		if rawName, ok := value["name"]; !ok || rawName == "bash" && value["name"] != nil && value["type"] != nil {
			var v BashTool20250124
			if err := json.Unmarshal(data, &v); err != nil {
				return err
			}
			u.BashTool20250124 = &v
			return nil
		}
		if rawName, ok := value["name"]; !ok || rawName == "code_execution" && value["name"] != nil && value["type"] != nil {
			var v CodeExecutionTool20250522
			if err := json.Unmarshal(data, &v); err != nil {
				return err
			}
			u.CodeExecutionTool20250522 = &v
			return nil
		}
		if rawName, ok := value["name"]; !ok || rawName == "code_execution" && value["name"] != nil && value["type"] != nil {
			var v CodeExecutionTool20250825
			if err := json.Unmarshal(data, &v); err != nil {
				return err
			}
			u.CodeExecutionTool20250825 = &v
			return nil
		}
		if rawName, ok := value["name"]; !ok || rawName == "code_execution" && value["name"] != nil && value["type"] != nil {
			var v CodeExecutionTool20260120
			if err := json.Unmarshal(data, &v); err != nil {
				return err
			}
			u.CodeExecutionTool20260120 = &v
			return nil
		}
		if rawName, ok := value["name"]; !ok || rawName == "memory" && value["name"] != nil && value["type"] != nil {
			var v MemoryTool20250818
			if err := json.Unmarshal(data, &v); err != nil {
				return err
			}
			u.MemoryTool20250818 = &v
			return nil
		}
		if rawName, ok := value["name"]; !ok || rawName == "str_replace_editor" && value["name"] != nil && value["type"] != nil {
			var v TextEditor20250124
			if err := json.Unmarshal(data, &v); err != nil {
				return err
			}
			u.TextEditor20250124 = &v
			return nil
		}
		if rawName, ok := value["name"]; !ok || rawName == "str_replace_based_edit_tool" && value["name"] != nil && value["type"] != nil {
			var v TextEditor20250429
			if err := json.Unmarshal(data, &v); err != nil {
				return err
			}
			u.TextEditor20250429 = &v
			return nil
		}
		if rawName, ok := value["name"]; !ok || rawName == "str_replace_based_edit_tool" && value["name"] != nil && value["type"] != nil {
			var v TextEditor20250728
			if err := json.Unmarshal(data, &v); err != nil {
				return err
			}
			u.TextEditor20250728 = &v
			return nil
		}
		if rawName, ok := value["name"]; !ok || rawName == "web_search" && value["name"] != nil && value["type"] != nil {
			var v WebSearchTool20250305
			if err := json.Unmarshal(data, &v); err != nil {
				return err
			}
			u.WebSearchTool20250305 = &v
			return nil
		}
		if rawName, ok := value["name"]; !ok || rawName == "web_fetch" && value["name"] != nil && value["type"] != nil {
			var v WebFetchTool20250910
			if err := json.Unmarshal(data, &v); err != nil {
				return err
			}
			u.WebFetchTool20250910 = &v
			return nil
		}
		if rawName, ok := value["name"]; !ok || rawName == "web_search" && value["name"] != nil && value["type"] != nil {
			var v WebSearchTool20260209
			if err := json.Unmarshal(data, &v); err != nil {
				return err
			}
			u.WebSearchTool20260209 = &v
			return nil
		}
		if rawName, ok := value["name"]; !ok || rawName == "web_fetch" && value["name"] != nil && value["type"] != nil {
			var v WebFetchTool20260209
			if err := json.Unmarshal(data, &v); err != nil {
				return err
			}
			u.WebFetchTool20260209 = &v
			return nil
		}
		if rawName, ok := value["name"]; !ok || rawName == "web_fetch" && value["name"] != nil && value["type"] != nil {
			var v WebFetchTool20260309
			if err := json.Unmarshal(data, &v); err != nil {
				return err
			}
			u.WebFetchTool20260309 = &v
			return nil
		}
		if rawName, ok := value["name"]; !ok || rawName == "tool_search_tool_bm25" && value["name"] != nil && value["type"] != nil {
			var v ToolSearchToolBM2520251119
			if err := json.Unmarshal(data, &v); err != nil {
				return err
			}
			u.ToolSearchToolBM2520251119 = &v
			return nil
		}
		if rawName, ok := value["name"]; !ok || rawName == "tool_search_tool_regex" && value["name"] != nil && value["type"] != nil {
			var v ToolSearchToolRegex20251119
			if err := json.Unmarshal(data, &v); err != nil {
				return err
			}
			u.ToolSearchToolRegex20251119 = &v
			return nil
		}
		return errors.New("invalid CreateMessageParamsToolsItem")
	}
	return errors.New("invalid CreateMessageParamsToolsItem")
}

type Message struct {
	// Information about the container used in this request.
	//
	// This will be non-null if a container tool (e.g. code execution) was used.
	Container *Container `json:"container"`
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
	Id    string `json:"id"`
	Model Model  `json:"model"`
	// Conversational role of the generated message.
	//
	// This will always be `"assistant"`.
	Role string `json:"role"`
	// Structured information about why model output stopped.
	//
	// This is `null` when the `stop_reason` has no additional detail to report.
	StopDetails *RefusalStopDetails `json:"stop_details"`
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
	MessageStart      *MessageStartEvent
	MessageDelta      *MessageDeltaEvent
	MessageStop       *MessageStopEvent
	ContentBlockStart *ContentBlockStartEvent
	ContentBlockDelta *ContentBlockDeltaEvent
	ContentBlockStop  *ContentBlockStopEvent
	Ping              *PingEvent
}

func (u *MessageStreamEvent) MarshalJSON() ([]byte, error) {
	if u.MessageStart != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*MessageStartEvent
		}{
			Type:              "message_start",
			MessageStartEvent: u.MessageStart,
		})
	}
	if u.MessageDelta != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*MessageDeltaEvent
		}{
			Type:              "message_delta",
			MessageDeltaEvent: u.MessageDelta,
		})
	}
	if u.MessageStop != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*MessageStopEvent
		}{
			Type:             "message_stop",
			MessageStopEvent: u.MessageStop,
		})
	}
	if u.ContentBlockStart != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ContentBlockStartEvent
		}{
			Type:                   "content_block_start",
			ContentBlockStartEvent: u.ContentBlockStart,
		})
	}
	if u.ContentBlockDelta != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ContentBlockDeltaEvent
		}{
			Type:                   "content_block_delta",
			ContentBlockDeltaEvent: u.ContentBlockDelta,
		})
	}
	if u.ContentBlockStop != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ContentBlockStopEvent
		}{
			Type:                  "content_block_stop",
			ContentBlockStopEvent: u.ContentBlockStop,
		})
	}
	if u.Ping != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*PingEvent
		}{
			Type:      "ping",
			PingEvent: u.Ping,
		})
	}
	return nil, errors.New("invalid MessageStreamEvent: all variants are nil")
}

func (u *MessageStreamEvent) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in MessageStreamEvent")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = MessageStreamEvent{}
	switch discriminator {
	case "message_start":
		var value MessageStartEvent
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.MessageStart = &value
	case "message_delta":
		var value MessageDeltaEvent
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.MessageDelta = &value
	case "message_stop":
		var value MessageStopEvent
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.MessageStop = &value
	case "content_block_start":
		var value ContentBlockStartEvent
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ContentBlockStart = &value
	case "content_block_delta":
		var value ContentBlockDeltaEvent
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ContentBlockDelta = &value
	case "content_block_stop":
		var value ContentBlockStopEvent
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ContentBlockStop = &value
	case "ping":
		var value PingEvent
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ping = &value
	default:
		return fmt.Errorf("invalid type field in MessageStreamEvent: %q", discriminator)
	}
	return nil
}

type PingEvent struct {
}

type CacheControlEphemeral struct {
	// The time-to-live for the cache control breakpoint.
	//
	// This may be one the following values:
	// - `5m`: 5 minutes
	// - `1h`: 1 hour
	//
	// Defaults to `5m`.
	Ttl *CacheControlEphemeralTtl `json:"ttl,omitempty"`
}

// The time-to-live for the cache control breakpoint.
//
// This may be one the following values:
// - `5m`: 5 minutes
// - `1h`: 1 hour
//
// Defaults to `5m`.
type CacheControlEphemeralTtl string

const (
	CacheControlEphemeralTtlN5M CacheControlEphemeralTtl = "5m"
	CacheControlEphemeralTtlN1H CacheControlEphemeralTtl = "1h"
)

type InputMessage struct {
	Content InputMessageContent `json:"content"`
	Role    InputMessageRole    `json:"role"`
}

type InputMessageContentString *string

type InputMessageContentArray []InputContentBlock

type InputMessageContent struct {
	InputMessageContentString *InputMessageContentString
	InputMessageContentArray  *InputMessageContentArray
}

func (u *InputMessageContent) MarshalJSON() ([]byte, error) {
	if u == nil {
		return []byte("null"), nil
	}
	if u.InputMessageContentString != nil {
		return json.Marshal(u.InputMessageContentString)
	}
	if u.InputMessageContentArray != nil {
		return json.Marshal(u.InputMessageContentArray)
	}
	return nil, errors.New("invalid InputMessageContent: all variants are nil")
}

func (u *InputMessageContent) UnmarshalJSON(data []byte) error {
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*u = InputMessageContent{}
	switch raw.(type) {
	case string:
		var v InputMessageContentString
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		u.InputMessageContentString = &v
		return nil
	case []interface{}:
		var v InputMessageContentArray
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		u.InputMessageContentArray = &v
		return nil
	}
	return errors.New("invalid InputMessageContent")
}

type InputMessageRole string

const (
	InputMessageRoleUser      InputMessageRole = "user"
	InputMessageRoleAssistant InputMessageRole = "assistant"
)

type Metadata struct {
	// An external identifier for the user who is associated with the request.
	//
	// This should be a uuid, hash value, or other opaque identifier. Anthropic may use this id to help detect abuse. Do not include any identifying information such as name, email address, or phone number.
	UserId *string `json:"user_id,omitempty"`
}

// The model that will complete your prompt.\n\nSee [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options.
type Model *string

type OutputConfig struct {
	// How much effort the model should put into its response. Higher effort levels may result in more thorough analysis but take longer.
	//
	// Valid values are `low`, `medium`, `high`, or `max`.
	Effort *EffortLevel `json:"effort,omitempty"`
	// A schema to specify Claude's output format in responses. See [structured outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs)
	Format *JsonOutputFormat `json:"format,omitempty"`
}

type RequestTextBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *RequestTextBlockCacheControl   `json:"cache_control,omitempty"`
	Citations    []RequestTextBlockCitationsItem `json:"citations,omitempty"`
	Text         string                          `json:"text"`
	Type         string                          `json:"type"`
}

// Create a cache control breakpoint at this content block.
type RequestTextBlockCacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *RequestTextBlockCacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid RequestTextBlockCacheControl: all variants are nil")
}

func (u *RequestTextBlockCacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestTextBlockCacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestTextBlockCacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in RequestTextBlockCacheControl: %q", discriminator)
	}
	return nil
}

type RequestTextBlockCitationsItem struct {
	CharLocation            *RequestCharLocationCitation
	PageLocation            *RequestPageLocationCitation
	ContentBlockLocation    *RequestContentBlockLocationCitation
	WebSearchResultLocation *RequestWebSearchResultLocationCitation
	SearchResultLocation    *RequestSearchResultLocationCitation
}

func (u *RequestTextBlockCitationsItem) MarshalJSON() ([]byte, error) {
	if u.CharLocation != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestCharLocationCitation
		}{
			Type:                        "char_location",
			RequestCharLocationCitation: u.CharLocation,
		})
	}
	if u.PageLocation != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestPageLocationCitation
		}{
			Type:                        "page_location",
			RequestPageLocationCitation: u.PageLocation,
		})
	}
	if u.ContentBlockLocation != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestContentBlockLocationCitation
		}{
			Type:                                "content_block_location",
			RequestContentBlockLocationCitation: u.ContentBlockLocation,
		})
	}
	if u.WebSearchResultLocation != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestWebSearchResultLocationCitation
		}{
			Type:                                   "web_search_result_location",
			RequestWebSearchResultLocationCitation: u.WebSearchResultLocation,
		})
	}
	if u.SearchResultLocation != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestSearchResultLocationCitation
		}{
			Type:                                "search_result_location",
			RequestSearchResultLocationCitation: u.SearchResultLocation,
		})
	}
	return nil, errors.New("invalid RequestTextBlockCitationsItem: all variants are nil")
}

func (u *RequestTextBlockCitationsItem) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestTextBlockCitationsItem")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestTextBlockCitationsItem{}
	switch discriminator {
	case "char_location":
		var value RequestCharLocationCitation
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CharLocation = &value
	case "page_location":
		var value RequestPageLocationCitation
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.PageLocation = &value
	case "content_block_location":
		var value RequestContentBlockLocationCitation
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ContentBlockLocation = &value
	case "web_search_result_location":
		var value RequestWebSearchResultLocationCitation
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.WebSearchResultLocation = &value
	case "search_result_location":
		var value RequestSearchResultLocationCitation
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.SearchResultLocation = &value
	default:
		return fmt.Errorf("invalid type field in RequestTextBlockCitationsItem: %q", discriminator)
	}
	return nil
}

// Configuration for enabling Claude's extended thinking.
//
// When enabled, responses include `thinking` content blocks showing Claude's thinking process before the final answer. Requires a minimum budget of 1,024 tokens and counts towards your `max_tokens` limit.
//
// See [extended thinking](https://docs.claude.com/en/docs/build-with-claude/extended-thinking) for details.
type ThinkingConfigParam struct {
	Enabled  *ThinkingConfigEnabled
	Disabled *ThinkingConfigDisabled
	Adaptive *ThinkingConfigAdaptive
}

func (u *ThinkingConfigParam) MarshalJSON() ([]byte, error) {
	if u.Enabled != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ThinkingConfigEnabled
		}{
			Type:                  "enabled",
			ThinkingConfigEnabled: u.Enabled,
		})
	}
	if u.Disabled != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ThinkingConfigDisabled
		}{
			Type:                   "disabled",
			ThinkingConfigDisabled: u.Disabled,
		})
	}
	if u.Adaptive != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ThinkingConfigAdaptive
		}{
			Type:                   "adaptive",
			ThinkingConfigAdaptive: u.Adaptive,
		})
	}
	return nil, errors.New("invalid ThinkingConfigParam: all variants are nil")
}

func (u *ThinkingConfigParam) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in ThinkingConfigParam")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = ThinkingConfigParam{}
	switch discriminator {
	case "enabled":
		var value ThinkingConfigEnabled
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Enabled = &value
	case "disabled":
		var value ThinkingConfigDisabled
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Disabled = &value
	case "adaptive":
		var value ThinkingConfigAdaptive
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Adaptive = &value
	default:
		return fmt.Errorf("invalid type field in ThinkingConfigParam: %q", discriminator)
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

func (u *ToolChoice) MarshalJSON() ([]byte, error) {
	if u.Auto != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ToolChoiceAuto
		}{
			Type:           "auto",
			ToolChoiceAuto: u.Auto,
		})
	}
	if u.Any != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ToolChoiceAny
		}{
			Type:          "any",
			ToolChoiceAny: u.Any,
		})
	}
	if u.Tool != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ToolChoiceTool
		}{
			Type:           "tool",
			ToolChoiceTool: u.Tool,
		})
	}
	if u.None != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ToolChoiceNone
		}{
			Type:           "none",
			ToolChoiceNone: u.None,
		})
	}
	return nil, errors.New("invalid ToolChoice: all variants are nil")
}

func (u *ToolChoice) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in ToolChoice")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = ToolChoice{}
	switch discriminator {
	case "auto":
		var value ToolChoiceAuto
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Auto = &value
	case "any":
		var value ToolChoiceAny
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Any = &value
	case "tool":
		var value ToolChoiceTool
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Tool = &value
	case "none":
		var value ToolChoiceNone
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.None = &value
	default:
		return fmt.Errorf("invalid type field in ToolChoice: %q", discriminator)
	}
	return nil
}

type Tool struct {
	AllowedCallers []AllowedCaller `json:"allowed_callers,omitempty"`
	// Create a cache control breakpoint at this content block.
	CacheControl *ToolCacheControl `json:"cache_control,omitempty"`
	// If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.
	DeferLoading *bool `json:"defer_loading,omitempty"`
	// Description of what this tool does.
	//
	// Tool descriptions should be as detailed as possible. The more information that the model has about what the tool is and how to use it, the better it will perform. You can use natural language descriptions to reinforce important aspects of the tool input JSON schema.
	Description *string `json:"description,omitempty"`
	// Enable eager input streaming for this tool. When true, tool input parameters will be streamed incrementally as they are generated, and types will be inferred on-the-fly rather than buffering the full JSON output. When false, streaming is disabled for this tool even if the fine-grained-tool-streaming beta is active. When null (default), uses the default behavior based on beta headers.
	EagerInputStreaming *bool                  `json:"eager_input_streaming,omitempty"`
	InputExamples       []map[string]JsonValue `json:"input_examples,omitempty"`
	// [JSON schema](https://json-schema.org/draft/2020-12) for this tool's input.
	//
	// This defines the shape of the `input` that your tool accepts and that the model will produce.
	InputSchema InputSchema `json:"input_schema"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string `json:"name"`
	// When true, guarantees schema validation on tool names and inputs
	Strict *bool   `json:"strict,omitempty"`
	Type   *string `json:"type,omitempty"`
}

// Create a cache control breakpoint at this content block.
type ToolCacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *ToolCacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid ToolCacheControl: all variants are nil")
}

func (u *ToolCacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in ToolCacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = ToolCacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in ToolCacheControl: %q", discriminator)
	}
	return nil
}

type BashTool20250124 struct {
	AllowedCallers []AllowedCaller `json:"allowed_callers,omitempty"`
	// Create a cache control breakpoint at this content block.
	CacheControl *BashTool20250124CacheControl `json:"cache_control,omitempty"`
	// If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.
	DeferLoading  *bool                  `json:"defer_loading,omitempty"`
	InputExamples []map[string]JsonValue `json:"input_examples,omitempty"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string `json:"name"`
	// When true, guarantees schema validation on tool names and inputs
	Strict *bool  `json:"strict,omitempty"`
	Type   string `json:"type"`
}

// Create a cache control breakpoint at this content block.
type BashTool20250124CacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *BashTool20250124CacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid BashTool20250124CacheControl: all variants are nil")
}

func (u *BashTool20250124CacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in BashTool20250124CacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = BashTool20250124CacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in BashTool20250124CacheControl: %q", discriminator)
	}
	return nil
}

type CodeExecutionTool20250522 struct {
	AllowedCallers []AllowedCaller `json:"allowed_callers,omitempty"`
	// Create a cache control breakpoint at this content block.
	CacheControl *CodeExecutionTool20250522CacheControl `json:"cache_control,omitempty"`
	// If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.
	DeferLoading *bool `json:"defer_loading,omitempty"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string `json:"name"`
	// When true, guarantees schema validation on tool names and inputs
	Strict *bool  `json:"strict,omitempty"`
	Type   string `json:"type"`
}

// Create a cache control breakpoint at this content block.
type CodeExecutionTool20250522CacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *CodeExecutionTool20250522CacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid CodeExecutionTool20250522CacheControl: all variants are nil")
}

func (u *CodeExecutionTool20250522CacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in CodeExecutionTool20250522CacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = CodeExecutionTool20250522CacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in CodeExecutionTool20250522CacheControl: %q", discriminator)
	}
	return nil
}

type CodeExecutionTool20250825 struct {
	AllowedCallers []AllowedCaller `json:"allowed_callers,omitempty"`
	// Create a cache control breakpoint at this content block.
	CacheControl *CodeExecutionTool20250825CacheControl `json:"cache_control,omitempty"`
	// If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.
	DeferLoading *bool `json:"defer_loading,omitempty"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string `json:"name"`
	// When true, guarantees schema validation on tool names and inputs
	Strict *bool  `json:"strict,omitempty"`
	Type   string `json:"type"`
}

// Create a cache control breakpoint at this content block.
type CodeExecutionTool20250825CacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *CodeExecutionTool20250825CacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid CodeExecutionTool20250825CacheControl: all variants are nil")
}

func (u *CodeExecutionTool20250825CacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in CodeExecutionTool20250825CacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = CodeExecutionTool20250825CacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in CodeExecutionTool20250825CacheControl: %q", discriminator)
	}
	return nil
}

// Code execution tool with REPL state persistence (daemon mode + gVisor checkpoint).
type CodeExecutionTool20260120 struct {
	AllowedCallers []AllowedCaller `json:"allowed_callers,omitempty"`
	// Create a cache control breakpoint at this content block.
	CacheControl *CodeExecutionTool20260120CacheControl `json:"cache_control,omitempty"`
	// If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.
	DeferLoading *bool `json:"defer_loading,omitempty"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string `json:"name"`
	// When true, guarantees schema validation on tool names and inputs
	Strict *bool  `json:"strict,omitempty"`
	Type   string `json:"type"`
}

// Create a cache control breakpoint at this content block.
type CodeExecutionTool20260120CacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *CodeExecutionTool20260120CacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid CodeExecutionTool20260120CacheControl: all variants are nil")
}

func (u *CodeExecutionTool20260120CacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in CodeExecutionTool20260120CacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = CodeExecutionTool20260120CacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in CodeExecutionTool20260120CacheControl: %q", discriminator)
	}
	return nil
}

type MemoryTool20250818 struct {
	AllowedCallers []AllowedCaller `json:"allowed_callers,omitempty"`
	// Create a cache control breakpoint at this content block.
	CacheControl *MemoryTool20250818CacheControl `json:"cache_control,omitempty"`
	// If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.
	DeferLoading  *bool                  `json:"defer_loading,omitempty"`
	InputExamples []map[string]JsonValue `json:"input_examples,omitempty"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string `json:"name"`
	// When true, guarantees schema validation on tool names and inputs
	Strict *bool  `json:"strict,omitempty"`
	Type   string `json:"type"`
}

// Create a cache control breakpoint at this content block.
type MemoryTool20250818CacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *MemoryTool20250818CacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid MemoryTool20250818CacheControl: all variants are nil")
}

func (u *MemoryTool20250818CacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in MemoryTool20250818CacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = MemoryTool20250818CacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in MemoryTool20250818CacheControl: %q", discriminator)
	}
	return nil
}

type TextEditor20250124 struct {
	AllowedCallers []AllowedCaller `json:"allowed_callers,omitempty"`
	// Create a cache control breakpoint at this content block.
	CacheControl *TextEditor20250124CacheControl `json:"cache_control,omitempty"`
	// If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.
	DeferLoading  *bool                  `json:"defer_loading,omitempty"`
	InputExamples []map[string]JsonValue `json:"input_examples,omitempty"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string `json:"name"`
	// When true, guarantees schema validation on tool names and inputs
	Strict *bool  `json:"strict,omitempty"`
	Type   string `json:"type"`
}

// Create a cache control breakpoint at this content block.
type TextEditor20250124CacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *TextEditor20250124CacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid TextEditor20250124CacheControl: all variants are nil")
}

func (u *TextEditor20250124CacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in TextEditor20250124CacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = TextEditor20250124CacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in TextEditor20250124CacheControl: %q", discriminator)
	}
	return nil
}

type TextEditor20250429 struct {
	AllowedCallers []AllowedCaller `json:"allowed_callers,omitempty"`
	// Create a cache control breakpoint at this content block.
	CacheControl *TextEditor20250429CacheControl `json:"cache_control,omitempty"`
	// If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.
	DeferLoading  *bool                  `json:"defer_loading,omitempty"`
	InputExamples []map[string]JsonValue `json:"input_examples,omitempty"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string `json:"name"`
	// When true, guarantees schema validation on tool names and inputs
	Strict *bool  `json:"strict,omitempty"`
	Type   string `json:"type"`
}

// Create a cache control breakpoint at this content block.
type TextEditor20250429CacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *TextEditor20250429CacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid TextEditor20250429CacheControl: all variants are nil")
}

func (u *TextEditor20250429CacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in TextEditor20250429CacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = TextEditor20250429CacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in TextEditor20250429CacheControl: %q", discriminator)
	}
	return nil
}

type TextEditor20250728 struct {
	AllowedCallers []AllowedCaller `json:"allowed_callers,omitempty"`
	// Create a cache control breakpoint at this content block.
	CacheControl *TextEditor20250728CacheControl `json:"cache_control,omitempty"`
	// If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.
	DeferLoading  *bool                  `json:"defer_loading,omitempty"`
	InputExamples []map[string]JsonValue `json:"input_examples,omitempty"`
	// Maximum number of characters to display when viewing a file. If not specified, defaults to displaying the full file.
	MaxCharacters *int `json:"max_characters,omitempty"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string `json:"name"`
	// When true, guarantees schema validation on tool names and inputs
	Strict *bool  `json:"strict,omitempty"`
	Type   string `json:"type"`
}

// Create a cache control breakpoint at this content block.
type TextEditor20250728CacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *TextEditor20250728CacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid TextEditor20250728CacheControl: all variants are nil")
}

func (u *TextEditor20250728CacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in TextEditor20250728CacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = TextEditor20250728CacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in TextEditor20250728CacheControl: %q", discriminator)
	}
	return nil
}

type WebSearchTool20250305 struct {
	AllowedCallers []AllowedCaller `json:"allowed_callers,omitempty"`
	// If provided, only these domains will be included in results. Cannot be used alongside `blocked_domains`.
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	// If provided, these domains will never appear in results. Cannot be used alongside `allowed_domains`.
	BlockedDomains []string `json:"blocked_domains,omitempty"`
	// Create a cache control breakpoint at this content block.
	CacheControl *WebSearchTool20250305CacheControl `json:"cache_control,omitempty"`
	// If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.
	DeferLoading *bool `json:"defer_loading,omitempty"`
	// Maximum number of times the tool can be used in the API request.
	MaxUses *int `json:"max_uses,omitempty"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string `json:"name"`
	// When true, guarantees schema validation on tool names and inputs
	Strict *bool  `json:"strict,omitempty"`
	Type   string `json:"type"`
	// Parameters for the user's location. Used to provide more relevant search results.
	UserLocation *UserLocation `json:"user_location,omitempty"`
}

// Create a cache control breakpoint at this content block.
type WebSearchTool20250305CacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *WebSearchTool20250305CacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid WebSearchTool20250305CacheControl: all variants are nil")
}

func (u *WebSearchTool20250305CacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in WebSearchTool20250305CacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = WebSearchTool20250305CacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in WebSearchTool20250305CacheControl: %q", discriminator)
	}
	return nil
}

type WebFetchTool20250910 struct {
	AllowedCallers []AllowedCaller `json:"allowed_callers,omitempty"`
	// List of domains to allow fetching from
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	// List of domains to block fetching from
	BlockedDomains []string `json:"blocked_domains,omitempty"`
	// Create a cache control breakpoint at this content block.
	CacheControl *WebFetchTool20250910CacheControl `json:"cache_control,omitempty"`
	// Citations configuration for fetched documents. Citations are disabled by default.
	Citations *RequestCitationsConfig `json:"citations,omitempty"`
	// If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.
	DeferLoading *bool `json:"defer_loading,omitempty"`
	// Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.
	MaxContentTokens *int `json:"max_content_tokens,omitempty"`
	// Maximum number of times the tool can be used in the API request.
	MaxUses *int `json:"max_uses,omitempty"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string `json:"name"`
	// When true, guarantees schema validation on tool names and inputs
	Strict *bool  `json:"strict,omitempty"`
	Type   string `json:"type"`
}

// Create a cache control breakpoint at this content block.
type WebFetchTool20250910CacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *WebFetchTool20250910CacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid WebFetchTool20250910CacheControl: all variants are nil")
}

func (u *WebFetchTool20250910CacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in WebFetchTool20250910CacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = WebFetchTool20250910CacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in WebFetchTool20250910CacheControl: %q", discriminator)
	}
	return nil
}

type WebSearchTool20260209 struct {
	AllowedCallers []AllowedCaller `json:"allowed_callers,omitempty"`
	// If provided, only these domains will be included in results. Cannot be used alongside `blocked_domains`.
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	// If provided, these domains will never appear in results. Cannot be used alongside `allowed_domains`.
	BlockedDomains []string `json:"blocked_domains,omitempty"`
	// Create a cache control breakpoint at this content block.
	CacheControl *WebSearchTool20260209CacheControl `json:"cache_control,omitempty"`
	// If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.
	DeferLoading *bool `json:"defer_loading,omitempty"`
	// Maximum number of times the tool can be used in the API request.
	MaxUses *int `json:"max_uses,omitempty"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string `json:"name"`
	// When true, guarantees schema validation on tool names and inputs
	Strict *bool  `json:"strict,omitempty"`
	Type   string `json:"type"`
	// Parameters for the user's location. Used to provide more relevant search results.
	UserLocation *UserLocation `json:"user_location,omitempty"`
}

// Create a cache control breakpoint at this content block.
type WebSearchTool20260209CacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *WebSearchTool20260209CacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid WebSearchTool20260209CacheControl: all variants are nil")
}

func (u *WebSearchTool20260209CacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in WebSearchTool20260209CacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = WebSearchTool20260209CacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in WebSearchTool20260209CacheControl: %q", discriminator)
	}
	return nil
}

type WebFetchTool20260209 struct {
	AllowedCallers []AllowedCaller `json:"allowed_callers,omitempty"`
	// List of domains to allow fetching from
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	// List of domains to block fetching from
	BlockedDomains []string `json:"blocked_domains,omitempty"`
	// Create a cache control breakpoint at this content block.
	CacheControl *WebFetchTool20260209CacheControl `json:"cache_control,omitempty"`
	// Citations configuration for fetched documents. Citations are disabled by default.
	Citations *RequestCitationsConfig `json:"citations,omitempty"`
	// If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.
	DeferLoading *bool `json:"defer_loading,omitempty"`
	// Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.
	MaxContentTokens *int `json:"max_content_tokens,omitempty"`
	// Maximum number of times the tool can be used in the API request.
	MaxUses *int `json:"max_uses,omitempty"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string `json:"name"`
	// When true, guarantees schema validation on tool names and inputs
	Strict *bool  `json:"strict,omitempty"`
	Type   string `json:"type"`
}

// Create a cache control breakpoint at this content block.
type WebFetchTool20260209CacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *WebFetchTool20260209CacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid WebFetchTool20260209CacheControl: all variants are nil")
}

func (u *WebFetchTool20260209CacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in WebFetchTool20260209CacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = WebFetchTool20260209CacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in WebFetchTool20260209CacheControl: %q", discriminator)
	}
	return nil
}

// Web fetch tool with use_cache parameter for bypassing cached content.
type WebFetchTool20260309 struct {
	AllowedCallers []AllowedCaller `json:"allowed_callers,omitempty"`
	// List of domains to allow fetching from
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	// List of domains to block fetching from
	BlockedDomains []string `json:"blocked_domains,omitempty"`
	// Create a cache control breakpoint at this content block.
	CacheControl *WebFetchTool20260309CacheControl `json:"cache_control,omitempty"`
	// Citations configuration for fetched documents. Citations are disabled by default.
	Citations *RequestCitationsConfig `json:"citations,omitempty"`
	// If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.
	DeferLoading *bool `json:"defer_loading,omitempty"`
	// Maximum number of tokens used by including web page text content in the context. The limit is approximate and does not apply to binary content such as PDFs.
	MaxContentTokens *int `json:"max_content_tokens,omitempty"`
	// Maximum number of times the tool can be used in the API request.
	MaxUses *int `json:"max_uses,omitempty"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string `json:"name"`
	// When true, guarantees schema validation on tool names and inputs
	Strict *bool  `json:"strict,omitempty"`
	Type   string `json:"type"`
	// Whether to use cached content. Set to false to bypass the cache and fetch fresh content. Only set to false when the user explicitly requests fresh content or when fetching rapidly-changing sources.
	UseCache *bool `json:"use_cache,omitempty"`
}

// Create a cache control breakpoint at this content block.
type WebFetchTool20260309CacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *WebFetchTool20260309CacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid WebFetchTool20260309CacheControl: all variants are nil")
}

func (u *WebFetchTool20260309CacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in WebFetchTool20260309CacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = WebFetchTool20260309CacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in WebFetchTool20260309CacheControl: %q", discriminator)
	}
	return nil
}

type ToolSearchToolBM2520251119 struct {
	AllowedCallers []AllowedCaller `json:"allowed_callers,omitempty"`
	// Create a cache control breakpoint at this content block.
	CacheControl *ToolSearchToolBM2520251119CacheControl `json:"cache_control,omitempty"`
	// If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.
	DeferLoading *bool `json:"defer_loading,omitempty"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string `json:"name"`
	// When true, guarantees schema validation on tool names and inputs
	Strict *bool                          `json:"strict,omitempty"`
	Type   ToolSearchToolBM2520251119Type `json:"type"`
}

// Create a cache control breakpoint at this content block.
type ToolSearchToolBM2520251119CacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *ToolSearchToolBM2520251119CacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid ToolSearchToolBM2520251119CacheControl: all variants are nil")
}

func (u *ToolSearchToolBM2520251119CacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in ToolSearchToolBM2520251119CacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = ToolSearchToolBM2520251119CacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in ToolSearchToolBM2520251119CacheControl: %q", discriminator)
	}
	return nil
}

type ToolSearchToolBM2520251119Type string

const (
	ToolSearchToolBM2520251119TypeToolSearchToolBm2520251119 ToolSearchToolBM2520251119Type = "tool_search_tool_bm25_20251119"
	ToolSearchToolBM2520251119TypeToolSearchToolBm25         ToolSearchToolBM2520251119Type = "tool_search_tool_bm25"
)

type ToolSearchToolRegex20251119 struct {
	AllowedCallers []AllowedCaller `json:"allowed_callers,omitempty"`
	// Create a cache control breakpoint at this content block.
	CacheControl *ToolSearchToolRegex20251119CacheControl `json:"cache_control,omitempty"`
	// If true, tool will not be included in initial system prompt. Only loaded when returned via tool_reference from tool search.
	DeferLoading *bool `json:"defer_loading,omitempty"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string `json:"name"`
	// When true, guarantees schema validation on tool names and inputs
	Strict *bool                           `json:"strict,omitempty"`
	Type   ToolSearchToolRegex20251119Type `json:"type"`
}

// Create a cache control breakpoint at this content block.
type ToolSearchToolRegex20251119CacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *ToolSearchToolRegex20251119CacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid ToolSearchToolRegex20251119CacheControl: all variants are nil")
}

func (u *ToolSearchToolRegex20251119CacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in ToolSearchToolRegex20251119CacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = ToolSearchToolRegex20251119CacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in ToolSearchToolRegex20251119CacheControl: %q", discriminator)
	}
	return nil
}

type ToolSearchToolRegex20251119Type string

const (
	ToolSearchToolRegex20251119TypeToolSearchToolRegex20251119 ToolSearchToolRegex20251119Type = "tool_search_tool_regex_20251119"
	ToolSearchToolRegex20251119TypeToolSearchToolRegex         ToolSearchToolRegex20251119Type = "tool_search_tool_regex"
)

// Information about the container used in the request (for the code execution tool)
type Container struct {
	// The time at which the container will expire.
	ExpiresAt string `json:"expires_at"`
	// Identifier for the container used in this request
	Id string `json:"id"`
}

type ContentBlock struct {
	Text                              *ResponseTextBlock
	Thinking                          *ResponseThinkingBlock
	RedactedThinking                  *ResponseRedactedThinkingBlock
	ToolUse                           *ResponseToolUseBlock
	ServerToolUse                     *ResponseServerToolUseBlock
	WebSearchToolResult               *ResponseWebSearchToolResultBlock
	WebFetchToolResult                *ResponseWebFetchToolResultBlock
	CodeExecutionToolResult           *ResponseCodeExecutionToolResultBlock
	BashCodeExecutionToolResult       *ResponseBashCodeExecutionToolResultBlock
	TextEditorCodeExecutionToolResult *ResponseTextEditorCodeExecutionToolResultBlock
	ToolSearchToolResult              *ResponseToolSearchToolResultBlock
	ContainerUpload                   *ResponseContainerUploadBlock
}

func (u *ContentBlock) MarshalJSON() ([]byte, error) {
	if u.Text != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseTextBlock
		}{
			Type:              "text",
			ResponseTextBlock: u.Text,
		})
	}
	if u.Thinking != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseThinkingBlock
		}{
			Type:                  "thinking",
			ResponseThinkingBlock: u.Thinking,
		})
	}
	if u.RedactedThinking != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseRedactedThinkingBlock
		}{
			Type:                          "redacted_thinking",
			ResponseRedactedThinkingBlock: u.RedactedThinking,
		})
	}
	if u.ToolUse != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseToolUseBlock
		}{
			Type:                 "tool_use",
			ResponseToolUseBlock: u.ToolUse,
		})
	}
	if u.ServerToolUse != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseServerToolUseBlock
		}{
			Type:                       "server_tool_use",
			ResponseServerToolUseBlock: u.ServerToolUse,
		})
	}
	if u.WebSearchToolResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseWebSearchToolResultBlock
		}{
			Type:                             "web_search_tool_result",
			ResponseWebSearchToolResultBlock: u.WebSearchToolResult,
		})
	}
	if u.WebFetchToolResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseWebFetchToolResultBlock
		}{
			Type:                            "web_fetch_tool_result",
			ResponseWebFetchToolResultBlock: u.WebFetchToolResult,
		})
	}
	if u.CodeExecutionToolResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseCodeExecutionToolResultBlock
		}{
			Type:                                 "code_execution_tool_result",
			ResponseCodeExecutionToolResultBlock: u.CodeExecutionToolResult,
		})
	}
	if u.BashCodeExecutionToolResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseBashCodeExecutionToolResultBlock
		}{
			Type:                                     "bash_code_execution_tool_result",
			ResponseBashCodeExecutionToolResultBlock: u.BashCodeExecutionToolResult,
		})
	}
	if u.TextEditorCodeExecutionToolResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseTextEditorCodeExecutionToolResultBlock
		}{
			Type: "text_editor_code_execution_tool_result",
			ResponseTextEditorCodeExecutionToolResultBlock: u.TextEditorCodeExecutionToolResult,
		})
	}
	if u.ToolSearchToolResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseToolSearchToolResultBlock
		}{
			Type:                              "tool_search_tool_result",
			ResponseToolSearchToolResultBlock: u.ToolSearchToolResult,
		})
	}
	if u.ContainerUpload != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseContainerUploadBlock
		}{
			Type:                         "container_upload",
			ResponseContainerUploadBlock: u.ContainerUpload,
		})
	}
	return nil, errors.New("invalid ContentBlock: all variants are nil")
}

func (u *ContentBlock) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in ContentBlock")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = ContentBlock{}
	switch discriminator {
	case "text":
		var value ResponseTextBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Text = &value
	case "thinking":
		var value ResponseThinkingBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Thinking = &value
	case "redacted_thinking":
		var value ResponseRedactedThinkingBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.RedactedThinking = &value
	case "tool_use":
		var value ResponseToolUseBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ToolUse = &value
	case "server_tool_use":
		var value ResponseServerToolUseBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ServerToolUse = &value
	case "web_search_tool_result":
		var value ResponseWebSearchToolResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.WebSearchToolResult = &value
	case "web_fetch_tool_result":
		var value ResponseWebFetchToolResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.WebFetchToolResult = &value
	case "code_execution_tool_result":
		var value ResponseCodeExecutionToolResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecutionToolResult = &value
	case "bash_code_execution_tool_result":
		var value ResponseBashCodeExecutionToolResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.BashCodeExecutionToolResult = &value
	case "text_editor_code_execution_tool_result":
		var value ResponseTextEditorCodeExecutionToolResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.TextEditorCodeExecutionToolResult = &value
	case "tool_search_tool_result":
		var value ResponseToolSearchToolResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ToolSearchToolResult = &value
	case "container_upload":
		var value ResponseContainerUploadBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ContainerUpload = &value
	default:
		return fmt.Errorf("invalid type field in ContentBlock: %q", discriminator)
	}
	return nil
}

// Structured information about a refusal.
type RefusalStopDetails struct {
	// The policy category that triggered the refusal.
	//
	// `null` when the refusal doesn't map to a named category.
	Category *RefusalStopDetailsCategory `json:"category"`
	// Human-readable explanation of the refusal.
	//
	// This text is not guaranteed to be stable. `null` when no explanation is available for the category.
	Explanation *string `json:"explanation"`
	Type        string  `json:"type"`
}

// The policy category that triggered the refusal.
//
// `null` when the refusal doesn't map to a named category.
type RefusalStopDetailsCategory string

const (
	RefusalStopDetailsCategoryCyber RefusalStopDetailsCategory = "cyber"
	RefusalStopDetailsCategoryBio   RefusalStopDetailsCategory = "bio"
)

type StopReason string

const (
	StopReasonEndTurn      StopReason = "end_turn"
	StopReasonMaxTokens    StopReason = "max_tokens"
	StopReasonStopSequence StopReason = "stop_sequence"
	StopReasonToolUse      StopReason = "tool_use"
	StopReasonPauseTurn    StopReason = "pause_turn"
	StopReasonRefusal      StopReason = "refusal"
)

type Usage struct {
	// Breakdown of cached tokens by TTL
	CacheCreation *CacheCreation `json:"cache_creation"`
	// The number of input tokens used to create the cache entry.
	CacheCreationInputTokens *int `json:"cache_creation_input_tokens"`
	// The number of input tokens read from the cache.
	CacheReadInputTokens *int `json:"cache_read_input_tokens"`
	// The geographic region where inference was performed for this request.
	InferenceGeo *string `json:"inference_geo"`
	// The number of input tokens which were used.
	InputTokens int `json:"input_tokens"`
	// The number of output tokens which were used.
	OutputTokens int `json:"output_tokens"`
	// The number of server tool requests.
	ServerToolUse *ServerToolUsage `json:"server_tool_use"`
	// If the request used the priority, standard, or batch tier.
	ServiceTier *UsageServiceTier `json:"service_tier"`
}

// If the request used the priority, standard, or batch tier.
type UsageServiceTier string

const (
	UsageServiceTierStandard UsageServiceTier = "standard"
	UsageServiceTierPriority UsageServiceTier = "priority"
	UsageServiceTierBatch    UsageServiceTier = "batch"
)

type ContentBlockDeltaEvent struct {
	Delta ContentBlockDeltaEventDelta `json:"delta"`
	Index int                         `json:"index"`
}

type ContentBlockDeltaEventDelta struct {
	TextDelta      *TextContentBlockDelta
	InputJsonDelta *InputJsonContentBlockDelta
	CitationsDelta *CitationsDelta
	ThinkingDelta  *ThinkingContentBlockDelta
	SignatureDelta *SignatureContentBlockDelta
}

func (u *ContentBlockDeltaEventDelta) MarshalJSON() ([]byte, error) {
	if u.TextDelta != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*TextContentBlockDelta
		}{
			Type:                  "text_delta",
			TextContentBlockDelta: u.TextDelta,
		})
	}
	if u.InputJsonDelta != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*InputJsonContentBlockDelta
		}{
			Type:                       "input_json_delta",
			InputJsonContentBlockDelta: u.InputJsonDelta,
		})
	}
	if u.CitationsDelta != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CitationsDelta
		}{
			Type:           "citations_delta",
			CitationsDelta: u.CitationsDelta,
		})
	}
	if u.ThinkingDelta != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ThinkingContentBlockDelta
		}{
			Type:                      "thinking_delta",
			ThinkingContentBlockDelta: u.ThinkingDelta,
		})
	}
	if u.SignatureDelta != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*SignatureContentBlockDelta
		}{
			Type:                       "signature_delta",
			SignatureContentBlockDelta: u.SignatureDelta,
		})
	}
	return nil, errors.New("invalid ContentBlockDeltaEventDelta: all variants are nil")
}

func (u *ContentBlockDeltaEventDelta) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in ContentBlockDeltaEventDelta")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = ContentBlockDeltaEventDelta{}
	switch discriminator {
	case "text_delta":
		var value TextContentBlockDelta
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.TextDelta = &value
	case "input_json_delta":
		var value InputJsonContentBlockDelta
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.InputJsonDelta = &value
	case "citations_delta":
		var value CitationsDelta
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CitationsDelta = &value
	case "thinking_delta":
		var value ThinkingContentBlockDelta
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ThinkingDelta = &value
	case "signature_delta":
		var value SignatureContentBlockDelta
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.SignatureDelta = &value
	default:
		return fmt.Errorf("invalid type field in ContentBlockDeltaEventDelta: %q", discriminator)
	}
	return nil
}

type ContentBlockStartEvent struct {
	ContentBlock ContentBlockStartEventContentBlock `json:"content_block"`
	Index        int                                `json:"index"`
}

type ContentBlockStartEventContentBlock struct {
	Text                              *ResponseTextBlock
	Thinking                          *ResponseThinkingBlock
	RedactedThinking                  *ResponseRedactedThinkingBlock
	ToolUse                           *ResponseToolUseBlock
	ServerToolUse                     *ResponseServerToolUseBlock
	WebSearchToolResult               *ResponseWebSearchToolResultBlock
	WebFetchToolResult                *ResponseWebFetchToolResultBlock
	CodeExecutionToolResult           *ResponseCodeExecutionToolResultBlock
	BashCodeExecutionToolResult       *ResponseBashCodeExecutionToolResultBlock
	TextEditorCodeExecutionToolResult *ResponseTextEditorCodeExecutionToolResultBlock
	ToolSearchToolResult              *ResponseToolSearchToolResultBlock
	ContainerUpload                   *ResponseContainerUploadBlock
}

func (u *ContentBlockStartEventContentBlock) MarshalJSON() ([]byte, error) {
	if u.Text != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseTextBlock
		}{
			Type:              "text",
			ResponseTextBlock: u.Text,
		})
	}
	if u.Thinking != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseThinkingBlock
		}{
			Type:                  "thinking",
			ResponseThinkingBlock: u.Thinking,
		})
	}
	if u.RedactedThinking != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseRedactedThinkingBlock
		}{
			Type:                          "redacted_thinking",
			ResponseRedactedThinkingBlock: u.RedactedThinking,
		})
	}
	if u.ToolUse != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseToolUseBlock
		}{
			Type:                 "tool_use",
			ResponseToolUseBlock: u.ToolUse,
		})
	}
	if u.ServerToolUse != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseServerToolUseBlock
		}{
			Type:                       "server_tool_use",
			ResponseServerToolUseBlock: u.ServerToolUse,
		})
	}
	if u.WebSearchToolResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseWebSearchToolResultBlock
		}{
			Type:                             "web_search_tool_result",
			ResponseWebSearchToolResultBlock: u.WebSearchToolResult,
		})
	}
	if u.WebFetchToolResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseWebFetchToolResultBlock
		}{
			Type:                            "web_fetch_tool_result",
			ResponseWebFetchToolResultBlock: u.WebFetchToolResult,
		})
	}
	if u.CodeExecutionToolResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseCodeExecutionToolResultBlock
		}{
			Type:                                 "code_execution_tool_result",
			ResponseCodeExecutionToolResultBlock: u.CodeExecutionToolResult,
		})
	}
	if u.BashCodeExecutionToolResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseBashCodeExecutionToolResultBlock
		}{
			Type:                                     "bash_code_execution_tool_result",
			ResponseBashCodeExecutionToolResultBlock: u.BashCodeExecutionToolResult,
		})
	}
	if u.TextEditorCodeExecutionToolResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseTextEditorCodeExecutionToolResultBlock
		}{
			Type: "text_editor_code_execution_tool_result",
			ResponseTextEditorCodeExecutionToolResultBlock: u.TextEditorCodeExecutionToolResult,
		})
	}
	if u.ToolSearchToolResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseToolSearchToolResultBlock
		}{
			Type:                              "tool_search_tool_result",
			ResponseToolSearchToolResultBlock: u.ToolSearchToolResult,
		})
	}
	if u.ContainerUpload != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseContainerUploadBlock
		}{
			Type:                         "container_upload",
			ResponseContainerUploadBlock: u.ContainerUpload,
		})
	}
	return nil, errors.New("invalid ContentBlockStartEventContentBlock: all variants are nil")
}

func (u *ContentBlockStartEventContentBlock) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in ContentBlockStartEventContentBlock")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = ContentBlockStartEventContentBlock{}
	switch discriminator {
	case "text":
		var value ResponseTextBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Text = &value
	case "thinking":
		var value ResponseThinkingBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Thinking = &value
	case "redacted_thinking":
		var value ResponseRedactedThinkingBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.RedactedThinking = &value
	case "tool_use":
		var value ResponseToolUseBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ToolUse = &value
	case "server_tool_use":
		var value ResponseServerToolUseBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ServerToolUse = &value
	case "web_search_tool_result":
		var value ResponseWebSearchToolResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.WebSearchToolResult = &value
	case "web_fetch_tool_result":
		var value ResponseWebFetchToolResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.WebFetchToolResult = &value
	case "code_execution_tool_result":
		var value ResponseCodeExecutionToolResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecutionToolResult = &value
	case "bash_code_execution_tool_result":
		var value ResponseBashCodeExecutionToolResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.BashCodeExecutionToolResult = &value
	case "text_editor_code_execution_tool_result":
		var value ResponseTextEditorCodeExecutionToolResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.TextEditorCodeExecutionToolResult = &value
	case "tool_search_tool_result":
		var value ResponseToolSearchToolResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ToolSearchToolResult = &value
	case "container_upload":
		var value ResponseContainerUploadBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ContainerUpload = &value
	default:
		return fmt.Errorf("invalid type field in ContentBlockStartEventContentBlock: %q", discriminator)
	}
	return nil
}

type ContentBlockStopEvent struct {
	Index int `json:"index"`
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

type MessageStartEvent struct {
	Message Message `json:"message"`
}

type MessageStopEvent struct {
}

type InputContentBlock struct {
	Text                              *RequestTextBlock
	Image                             *RequestImageBlock
	Document                          *RequestDocumentBlock
	SearchResult                      *RequestSearchResultBlock
	Thinking                          *RequestThinkingBlock
	RedactedThinking                  *RequestRedactedThinkingBlock
	ToolUse                           *RequestToolUseBlock
	ToolResult                        *RequestToolResultBlock
	ServerToolUse                     *RequestServerToolUseBlock
	WebSearchToolResult               *RequestWebSearchToolResultBlock
	WebFetchToolResult                *RequestWebFetchToolResultBlock
	CodeExecutionToolResult           *RequestCodeExecutionToolResultBlock
	BashCodeExecutionToolResult       *RequestBashCodeExecutionToolResultBlock
	TextEditorCodeExecutionToolResult *RequestTextEditorCodeExecutionToolResultBlock
	ToolSearchToolResult              *RequestToolSearchToolResultBlock
	ContainerUpload                   *RequestContainerUploadBlock
}

func (u *InputContentBlock) MarshalJSON() ([]byte, error) {
	if u.Text != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestTextBlock
		}{
			Type:             "text",
			RequestTextBlock: u.Text,
		})
	}
	if u.Image != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestImageBlock
		}{
			Type:              "image",
			RequestImageBlock: u.Image,
		})
	}
	if u.Document != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestDocumentBlock
		}{
			Type:                 "document",
			RequestDocumentBlock: u.Document,
		})
	}
	if u.SearchResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestSearchResultBlock
		}{
			Type:                     "search_result",
			RequestSearchResultBlock: u.SearchResult,
		})
	}
	if u.Thinking != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestThinkingBlock
		}{
			Type:                 "thinking",
			RequestThinkingBlock: u.Thinking,
		})
	}
	if u.RedactedThinking != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestRedactedThinkingBlock
		}{
			Type:                         "redacted_thinking",
			RequestRedactedThinkingBlock: u.RedactedThinking,
		})
	}
	if u.ToolUse != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestToolUseBlock
		}{
			Type:                "tool_use",
			RequestToolUseBlock: u.ToolUse,
		})
	}
	if u.ToolResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestToolResultBlock
		}{
			Type:                   "tool_result",
			RequestToolResultBlock: u.ToolResult,
		})
	}
	if u.ServerToolUse != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestServerToolUseBlock
		}{
			Type:                      "server_tool_use",
			RequestServerToolUseBlock: u.ServerToolUse,
		})
	}
	if u.WebSearchToolResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestWebSearchToolResultBlock
		}{
			Type:                            "web_search_tool_result",
			RequestWebSearchToolResultBlock: u.WebSearchToolResult,
		})
	}
	if u.WebFetchToolResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestWebFetchToolResultBlock
		}{
			Type:                           "web_fetch_tool_result",
			RequestWebFetchToolResultBlock: u.WebFetchToolResult,
		})
	}
	if u.CodeExecutionToolResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestCodeExecutionToolResultBlock
		}{
			Type:                                "code_execution_tool_result",
			RequestCodeExecutionToolResultBlock: u.CodeExecutionToolResult,
		})
	}
	if u.BashCodeExecutionToolResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestBashCodeExecutionToolResultBlock
		}{
			Type:                                    "bash_code_execution_tool_result",
			RequestBashCodeExecutionToolResultBlock: u.BashCodeExecutionToolResult,
		})
	}
	if u.TextEditorCodeExecutionToolResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestTextEditorCodeExecutionToolResultBlock
		}{
			Type: "text_editor_code_execution_tool_result",
			RequestTextEditorCodeExecutionToolResultBlock: u.TextEditorCodeExecutionToolResult,
		})
	}
	if u.ToolSearchToolResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestToolSearchToolResultBlock
		}{
			Type:                             "tool_search_tool_result",
			RequestToolSearchToolResultBlock: u.ToolSearchToolResult,
		})
	}
	if u.ContainerUpload != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestContainerUploadBlock
		}{
			Type:                        "container_upload",
			RequestContainerUploadBlock: u.ContainerUpload,
		})
	}
	return nil, errors.New("invalid InputContentBlock: all variants are nil")
}

func (u *InputContentBlock) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in InputContentBlock")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = InputContentBlock{}
	switch discriminator {
	case "text":
		var value RequestTextBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Text = &value
	case "image":
		var value RequestImageBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Image = &value
	case "document":
		var value RequestDocumentBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Document = &value
	case "search_result":
		var value RequestSearchResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.SearchResult = &value
	case "thinking":
		var value RequestThinkingBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Thinking = &value
	case "redacted_thinking":
		var value RequestRedactedThinkingBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.RedactedThinking = &value
	case "tool_use":
		var value RequestToolUseBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ToolUse = &value
	case "tool_result":
		var value RequestToolResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ToolResult = &value
	case "server_tool_use":
		var value RequestServerToolUseBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ServerToolUse = &value
	case "web_search_tool_result":
		var value RequestWebSearchToolResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.WebSearchToolResult = &value
	case "web_fetch_tool_result":
		var value RequestWebFetchToolResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.WebFetchToolResult = &value
	case "code_execution_tool_result":
		var value RequestCodeExecutionToolResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecutionToolResult = &value
	case "bash_code_execution_tool_result":
		var value RequestBashCodeExecutionToolResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.BashCodeExecutionToolResult = &value
	case "text_editor_code_execution_tool_result":
		var value RequestTextEditorCodeExecutionToolResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.TextEditorCodeExecutionToolResult = &value
	case "tool_search_tool_result":
		var value RequestToolSearchToolResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ToolSearchToolResult = &value
	case "container_upload":
		var value RequestContainerUploadBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ContainerUpload = &value
	default:
		return fmt.Errorf("invalid type field in InputContentBlock: %q", discriminator)
	}
	return nil
}

// All possible effort levels.
type EffortLevel string

const (
	EffortLevelLow    EffortLevel = "low"
	EffortLevelMedium EffortLevel = "medium"
	EffortLevelHigh   EffortLevel = "high"
	EffortLevelXhigh  EffortLevel = "xhigh"
	EffortLevelMax    EffortLevel = "max"
)

type JsonOutputFormat struct {
	// The JSON schema of the format
	Schema any    `json:"schema"`
	Type   string `json:"type"`
}

type RequestCharLocationCitation struct {
	CitedText      string  `json:"cited_text"`
	DocumentIndex  int     `json:"document_index"`
	DocumentTitle  *string `json:"document_title"`
	EndCharIndex   int     `json:"end_char_index"`
	StartCharIndex int     `json:"start_char_index"`
}

type RequestContentBlockLocationCitation struct {
	CitedText       string  `json:"cited_text"`
	DocumentIndex   int     `json:"document_index"`
	DocumentTitle   *string `json:"document_title"`
	EndBlockIndex   int     `json:"end_block_index"`
	StartBlockIndex int     `json:"start_block_index"`
}

type RequestPageLocationCitation struct {
	CitedText       string  `json:"cited_text"`
	DocumentIndex   int     `json:"document_index"`
	DocumentTitle   *string `json:"document_title"`
	EndPageNumber   int     `json:"end_page_number"`
	StartPageNumber int     `json:"start_page_number"`
}

type RequestSearchResultLocationCitation struct {
	CitedText         string  `json:"cited_text"`
	EndBlockIndex     int     `json:"end_block_index"`
	SearchResultIndex int     `json:"search_result_index"`
	Source            string  `json:"source"`
	StartBlockIndex   int     `json:"start_block_index"`
	Title             *string `json:"title"`
}

type RequestWebSearchResultLocationCitation struct {
	CitedText      string  `json:"cited_text"`
	EncryptedIndex string  `json:"encrypted_index"`
	Title          *string `json:"title"`
	Url            string  `json:"url"`
}

type ThinkingConfigAdaptive struct {
	// Controls how thinking content appears in the response. When set to `summarized`, thinking is returned normally. When set to `omitted`, thinking content is redacted but a signature is returned for multi-turn continuity. Defaults to `summarized`.
	Display *ThinkingDisplayMode `json:"display,omitempty"`
}

type ThinkingConfigDisabled struct {
}

type ThinkingConfigEnabled struct {
	// Determines how many tokens Claude can use for its internal reasoning process. Larger budgets can enable more thorough analysis for complex problems, improving response quality.
	//
	// Must be ≥1024 and less than `max_tokens`.
	//
	// See [extended thinking](https://docs.claude.com/en/docs/build-with-claude/extended-thinking) for details.
	BudgetTokens int `json:"budget_tokens"`
	// Controls how thinking content appears in the response. When set to `summarized`, thinking is returned normally. When set to `omitted`, thinking content is redacted but a signature is returned for multi-turn continuity. Defaults to `summarized`.
	Display *ThinkingDisplayMode `json:"display,omitempty"`
}

// The model will use any available tools.
type ToolChoiceAny struct {
	// Whether to disable parallel tool use.
	//
	// Defaults to `false`. If set to `true`, the model will output exactly one tool use.
	DisableParallelToolUse *bool `json:"disable_parallel_tool_use,omitempty"`
}

// The model will automatically decide whether to use tools.
type ToolChoiceAuto struct {
	// Whether to disable parallel tool use.
	//
	// Defaults to `false`. If set to `true`, the model will output at most one tool use.
	DisableParallelToolUse *bool `json:"disable_parallel_tool_use,omitempty"`
}

// The model will not be allowed to use tools.
type ToolChoiceNone struct {
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

// Specifies who can invoke a tool.
//
// Values:
//
//	direct: The model can call this tool directly.
//	code_execution_20250825: The tool can be called from the code execution environment (v1).
//	code_execution_20260120: The tool can be called from the code execution environment (v2 with persistence).
type AllowedCaller string

const (
	AllowedCallerDirect                AllowedCaller = "direct"
	AllowedCallerCodeExecution20250825 AllowedCaller = "code_execution_20250825"
	AllowedCallerCodeExecution20260120 AllowedCaller = "code_execution_20260120"
)

type JsonValue any

type InputSchema any

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

type RequestCitationsConfig struct {
	Enabled *bool `json:"enabled,omitempty"`
}

type ResponseBashCodeExecutionToolResultBlock struct {
	Content   ResponseBashCodeExecutionToolResultBlockContent `json:"content"`
	ToolUseId string                                          `json:"tool_use_id"`
}

type ResponseBashCodeExecutionToolResultBlockContent struct {
	BashCodeExecutionToolResultError *ResponseBashCodeExecutionToolResultError
	BashCodeExecutionResult          *ResponseBashCodeExecutionResultBlock
}

func (u *ResponseBashCodeExecutionToolResultBlockContent) MarshalJSON() ([]byte, error) {
	if u.BashCodeExecutionToolResultError != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseBashCodeExecutionToolResultError
		}{
			Type:                                     "bash_code_execution_tool_result_error",
			ResponseBashCodeExecutionToolResultError: u.BashCodeExecutionToolResultError,
		})
	}
	if u.BashCodeExecutionResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseBashCodeExecutionResultBlock
		}{
			Type:                                 "bash_code_execution_result",
			ResponseBashCodeExecutionResultBlock: u.BashCodeExecutionResult,
		})
	}
	return nil, errors.New("invalid ResponseBashCodeExecutionToolResultBlockContent: all variants are nil")
}

func (u *ResponseBashCodeExecutionToolResultBlockContent) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in ResponseBashCodeExecutionToolResultBlockContent")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = ResponseBashCodeExecutionToolResultBlockContent{}
	switch discriminator {
	case "bash_code_execution_tool_result_error":
		var value ResponseBashCodeExecutionToolResultError
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.BashCodeExecutionToolResultError = &value
	case "bash_code_execution_result":
		var value ResponseBashCodeExecutionResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.BashCodeExecutionResult = &value
	default:
		return fmt.Errorf("invalid type field in ResponseBashCodeExecutionToolResultBlockContent: %q", discriminator)
	}
	return nil
}

type ResponseCodeExecutionToolResultBlock struct {
	Content   ResponseCodeExecutionToolResultBlockContent `json:"content"`
	ToolUseId string                                      `json:"tool_use_id"`
}

type ResponseCodeExecutionToolResultBlockContent struct {
	CodeExecutionToolResultError *ResponseCodeExecutionToolResultError
	CodeExecutionResult          *ResponseCodeExecutionResultBlock
	EncryptedCodeExecutionResult *ResponseEncryptedCodeExecutionResultBlock
}

func (u *ResponseCodeExecutionToolResultBlockContent) MarshalJSON() ([]byte, error) {
	if u.CodeExecutionToolResultError != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseCodeExecutionToolResultError
		}{
			Type:                                 "code_execution_tool_result_error",
			ResponseCodeExecutionToolResultError: u.CodeExecutionToolResultError,
		})
	}
	if u.CodeExecutionResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseCodeExecutionResultBlock
		}{
			Type:                             "code_execution_result",
			ResponseCodeExecutionResultBlock: u.CodeExecutionResult,
		})
	}
	if u.EncryptedCodeExecutionResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseEncryptedCodeExecutionResultBlock
		}{
			Type: "encrypted_code_execution_result",
			ResponseEncryptedCodeExecutionResultBlock: u.EncryptedCodeExecutionResult,
		})
	}
	return nil, errors.New("invalid ResponseCodeExecutionToolResultBlockContent: all variants are nil")
}

func (u *ResponseCodeExecutionToolResultBlockContent) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in ResponseCodeExecutionToolResultBlockContent")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = ResponseCodeExecutionToolResultBlockContent{}
	switch discriminator {
	case "code_execution_tool_result_error":
		var value ResponseCodeExecutionToolResultError
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecutionToolResultError = &value
	case "code_execution_result":
		var value ResponseCodeExecutionResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecutionResult = &value
	case "encrypted_code_execution_result":
		var value ResponseEncryptedCodeExecutionResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.EncryptedCodeExecutionResult = &value
	default:
		return fmt.Errorf("invalid type field in ResponseCodeExecutionToolResultBlockContent: %q", discriminator)
	}
	return nil
}

// Response model for a file uploaded to the container.
type ResponseContainerUploadBlock struct {
	FileId string `json:"file_id"`
}

type ResponseRedactedThinkingBlock struct {
	Data string `json:"data"`
}

type ResponseServerToolUseBlock struct {
	Caller ResponseServerToolUseBlockCaller `json:"caller"`
	Id     string                           `json:"id"`
	Input  any                              `json:"input"`
	Name   ResponseServerToolUseBlockName   `json:"name"`
}

type ResponseServerToolUseBlockCaller struct {
	Direct                *DirectCaller
	CodeExecution20250825 *ServerToolCaller
	CodeExecution20260120 *ServerToolCaller20260120
}

func (u *ResponseServerToolUseBlockCaller) MarshalJSON() ([]byte, error) {
	if u.Direct != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*DirectCaller
		}{
			Type:         "direct",
			DirectCaller: u.Direct,
		})
	}
	if u.CodeExecution20250825 != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ServerToolCaller
		}{
			Type:             "code_execution_20250825",
			ServerToolCaller: u.CodeExecution20250825,
		})
	}
	if u.CodeExecution20260120 != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ServerToolCaller20260120
		}{
			Type:                     "code_execution_20260120",
			ServerToolCaller20260120: u.CodeExecution20260120,
		})
	}
	return nil, errors.New("invalid ResponseServerToolUseBlockCaller: all variants are nil")
}

func (u *ResponseServerToolUseBlockCaller) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in ResponseServerToolUseBlockCaller")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = ResponseServerToolUseBlockCaller{}
	switch discriminator {
	case "direct":
		var value DirectCaller
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Direct = &value
	case "code_execution_20250825":
		var value ServerToolCaller
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecution20250825 = &value
	case "code_execution_20260120":
		var value ServerToolCaller20260120
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecution20260120 = &value
	default:
		return fmt.Errorf("invalid type field in ResponseServerToolUseBlockCaller: %q", discriminator)
	}
	return nil
}

type ResponseServerToolUseBlockName string

const (
	ResponseServerToolUseBlockNameWebSearch               ResponseServerToolUseBlockName = "web_search"
	ResponseServerToolUseBlockNameWebFetch                ResponseServerToolUseBlockName = "web_fetch"
	ResponseServerToolUseBlockNameCodeExecution           ResponseServerToolUseBlockName = "code_execution"
	ResponseServerToolUseBlockNameBashCodeExecution       ResponseServerToolUseBlockName = "bash_code_execution"
	ResponseServerToolUseBlockNameTextEditorCodeExecution ResponseServerToolUseBlockName = "text_editor_code_execution"
	ResponseServerToolUseBlockNameToolSearchToolRegex     ResponseServerToolUseBlockName = "tool_search_tool_regex"
	ResponseServerToolUseBlockNameToolSearchToolBm25      ResponseServerToolUseBlockName = "tool_search_tool_bm25"
)

type ResponseTextBlock struct {
	// Citations supporting the text block.
	//
	// The type of citation returned will depend on the type of document being cited. Citing a PDF results in `page_location`, plain text results in `char_location`, and content document results in `content_block_location`.
	Citations []ResponseTextBlockCitationsItem `json:"citations"`
	Text      string                           `json:"text"`
}

type ResponseTextBlockCitationsItem struct {
	CharLocation            *ResponseCharLocationCitation
	PageLocation            *ResponsePageLocationCitation
	ContentBlockLocation    *ResponseContentBlockLocationCitation
	WebSearchResultLocation *ResponseWebSearchResultLocationCitation
	SearchResultLocation    *ResponseSearchResultLocationCitation
}

func (u *ResponseTextBlockCitationsItem) MarshalJSON() ([]byte, error) {
	if u.CharLocation != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseCharLocationCitation
		}{
			Type:                         "char_location",
			ResponseCharLocationCitation: u.CharLocation,
		})
	}
	if u.PageLocation != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponsePageLocationCitation
		}{
			Type:                         "page_location",
			ResponsePageLocationCitation: u.PageLocation,
		})
	}
	if u.ContentBlockLocation != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseContentBlockLocationCitation
		}{
			Type:                                 "content_block_location",
			ResponseContentBlockLocationCitation: u.ContentBlockLocation,
		})
	}
	if u.WebSearchResultLocation != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseWebSearchResultLocationCitation
		}{
			Type:                                    "web_search_result_location",
			ResponseWebSearchResultLocationCitation: u.WebSearchResultLocation,
		})
	}
	if u.SearchResultLocation != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseSearchResultLocationCitation
		}{
			Type:                                 "search_result_location",
			ResponseSearchResultLocationCitation: u.SearchResultLocation,
		})
	}
	return nil, errors.New("invalid ResponseTextBlockCitationsItem: all variants are nil")
}

func (u *ResponseTextBlockCitationsItem) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in ResponseTextBlockCitationsItem")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = ResponseTextBlockCitationsItem{}
	switch discriminator {
	case "char_location":
		var value ResponseCharLocationCitation
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CharLocation = &value
	case "page_location":
		var value ResponsePageLocationCitation
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.PageLocation = &value
	case "content_block_location":
		var value ResponseContentBlockLocationCitation
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ContentBlockLocation = &value
	case "web_search_result_location":
		var value ResponseWebSearchResultLocationCitation
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.WebSearchResultLocation = &value
	case "search_result_location":
		var value ResponseSearchResultLocationCitation
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.SearchResultLocation = &value
	default:
		return fmt.Errorf("invalid type field in ResponseTextBlockCitationsItem: %q", discriminator)
	}
	return nil
}

type ResponseTextEditorCodeExecutionToolResultBlock struct {
	Content   ResponseTextEditorCodeExecutionToolResultBlockContent `json:"content"`
	ToolUseId string                                                `json:"tool_use_id"`
}

type ResponseTextEditorCodeExecutionToolResultBlockContent struct {
	TextEditorCodeExecutionToolResultError  *ResponseTextEditorCodeExecutionToolResultError
	TextEditorCodeExecutionViewResult       *ResponseTextEditorCodeExecutionViewResultBlock
	TextEditorCodeExecutionCreateResult     *ResponseTextEditorCodeExecutionCreateResultBlock
	TextEditorCodeExecutionStrReplaceResult *ResponseTextEditorCodeExecutionStrReplaceResultBlock
}

func (u *ResponseTextEditorCodeExecutionToolResultBlockContent) MarshalJSON() ([]byte, error) {
	if u.TextEditorCodeExecutionToolResultError != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseTextEditorCodeExecutionToolResultError
		}{
			Type: "text_editor_code_execution_tool_result_error",
			ResponseTextEditorCodeExecutionToolResultError: u.TextEditorCodeExecutionToolResultError,
		})
	}
	if u.TextEditorCodeExecutionViewResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseTextEditorCodeExecutionViewResultBlock
		}{
			Type: "text_editor_code_execution_view_result",
			ResponseTextEditorCodeExecutionViewResultBlock: u.TextEditorCodeExecutionViewResult,
		})
	}
	if u.TextEditorCodeExecutionCreateResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseTextEditorCodeExecutionCreateResultBlock
		}{
			Type: "text_editor_code_execution_create_result",
			ResponseTextEditorCodeExecutionCreateResultBlock: u.TextEditorCodeExecutionCreateResult,
		})
	}
	if u.TextEditorCodeExecutionStrReplaceResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseTextEditorCodeExecutionStrReplaceResultBlock
		}{
			Type: "text_editor_code_execution_str_replace_result",
			ResponseTextEditorCodeExecutionStrReplaceResultBlock: u.TextEditorCodeExecutionStrReplaceResult,
		})
	}
	return nil, errors.New("invalid ResponseTextEditorCodeExecutionToolResultBlockContent: all variants are nil")
}

func (u *ResponseTextEditorCodeExecutionToolResultBlockContent) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in ResponseTextEditorCodeExecutionToolResultBlockContent")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = ResponseTextEditorCodeExecutionToolResultBlockContent{}
	switch discriminator {
	case "text_editor_code_execution_tool_result_error":
		var value ResponseTextEditorCodeExecutionToolResultError
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.TextEditorCodeExecutionToolResultError = &value
	case "text_editor_code_execution_view_result":
		var value ResponseTextEditorCodeExecutionViewResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.TextEditorCodeExecutionViewResult = &value
	case "text_editor_code_execution_create_result":
		var value ResponseTextEditorCodeExecutionCreateResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.TextEditorCodeExecutionCreateResult = &value
	case "text_editor_code_execution_str_replace_result":
		var value ResponseTextEditorCodeExecutionStrReplaceResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.TextEditorCodeExecutionStrReplaceResult = &value
	default:
		return fmt.Errorf("invalid type field in ResponseTextEditorCodeExecutionToolResultBlockContent: %q", discriminator)
	}
	return nil
}

type ResponseThinkingBlock struct {
	Signature string `json:"signature"`
	Thinking  string `json:"thinking"`
}

type ResponseToolSearchToolResultBlock struct {
	Content   ResponseToolSearchToolResultBlockContent `json:"content"`
	ToolUseId string                                   `json:"tool_use_id"`
}

type ResponseToolSearchToolResultBlockContent struct {
	ToolSearchToolResultError  *ResponseToolSearchToolResultError
	ToolSearchToolSearchResult *ResponseToolSearchToolSearchResultBlock
}

func (u *ResponseToolSearchToolResultBlockContent) MarshalJSON() ([]byte, error) {
	if u.ToolSearchToolResultError != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseToolSearchToolResultError
		}{
			Type:                              "tool_search_tool_result_error",
			ResponseToolSearchToolResultError: u.ToolSearchToolResultError,
		})
	}
	if u.ToolSearchToolSearchResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseToolSearchToolSearchResultBlock
		}{
			Type:                                    "tool_search_tool_search_result",
			ResponseToolSearchToolSearchResultBlock: u.ToolSearchToolSearchResult,
		})
	}
	return nil, errors.New("invalid ResponseToolSearchToolResultBlockContent: all variants are nil")
}

func (u *ResponseToolSearchToolResultBlockContent) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in ResponseToolSearchToolResultBlockContent")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = ResponseToolSearchToolResultBlockContent{}
	switch discriminator {
	case "tool_search_tool_result_error":
		var value ResponseToolSearchToolResultError
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ToolSearchToolResultError = &value
	case "tool_search_tool_search_result":
		var value ResponseToolSearchToolSearchResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ToolSearchToolSearchResult = &value
	default:
		return fmt.Errorf("invalid type field in ResponseToolSearchToolResultBlockContent: %q", discriminator)
	}
	return nil
}

type ResponseToolUseBlock struct {
	Caller ResponseToolUseBlockCaller `json:"caller"`
	Id     string                     `json:"id"`
	Input  any                        `json:"input"`
	Name   string                     `json:"name"`
}

type ResponseToolUseBlockCaller struct {
	Direct                *DirectCaller
	CodeExecution20250825 *ServerToolCaller
	CodeExecution20260120 *ServerToolCaller20260120
}

func (u *ResponseToolUseBlockCaller) MarshalJSON() ([]byte, error) {
	if u.Direct != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*DirectCaller
		}{
			Type:         "direct",
			DirectCaller: u.Direct,
		})
	}
	if u.CodeExecution20250825 != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ServerToolCaller
		}{
			Type:             "code_execution_20250825",
			ServerToolCaller: u.CodeExecution20250825,
		})
	}
	if u.CodeExecution20260120 != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ServerToolCaller20260120
		}{
			Type:                     "code_execution_20260120",
			ServerToolCaller20260120: u.CodeExecution20260120,
		})
	}
	return nil, errors.New("invalid ResponseToolUseBlockCaller: all variants are nil")
}

func (u *ResponseToolUseBlockCaller) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in ResponseToolUseBlockCaller")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = ResponseToolUseBlockCaller{}
	switch discriminator {
	case "direct":
		var value DirectCaller
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Direct = &value
	case "code_execution_20250825":
		var value ServerToolCaller
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecution20250825 = &value
	case "code_execution_20260120":
		var value ServerToolCaller20260120
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecution20260120 = &value
	default:
		return fmt.Errorf("invalid type field in ResponseToolUseBlockCaller: %q", discriminator)
	}
	return nil
}

type ResponseWebFetchToolResultBlock struct {
	Caller    ResponseWebFetchToolResultBlockCaller  `json:"caller"`
	Content   ResponseWebFetchToolResultBlockContent `json:"content"`
	ToolUseId string                                 `json:"tool_use_id"`
}

type ResponseWebFetchToolResultBlockCaller struct {
	Direct                *DirectCaller
	CodeExecution20250825 *ServerToolCaller
	CodeExecution20260120 *ServerToolCaller20260120
}

func (u *ResponseWebFetchToolResultBlockCaller) MarshalJSON() ([]byte, error) {
	if u.Direct != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*DirectCaller
		}{
			Type:         "direct",
			DirectCaller: u.Direct,
		})
	}
	if u.CodeExecution20250825 != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ServerToolCaller
		}{
			Type:             "code_execution_20250825",
			ServerToolCaller: u.CodeExecution20250825,
		})
	}
	if u.CodeExecution20260120 != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ServerToolCaller20260120
		}{
			Type:                     "code_execution_20260120",
			ServerToolCaller20260120: u.CodeExecution20260120,
		})
	}
	return nil, errors.New("invalid ResponseWebFetchToolResultBlockCaller: all variants are nil")
}

func (u *ResponseWebFetchToolResultBlockCaller) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in ResponseWebFetchToolResultBlockCaller")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = ResponseWebFetchToolResultBlockCaller{}
	switch discriminator {
	case "direct":
		var value DirectCaller
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Direct = &value
	case "code_execution_20250825":
		var value ServerToolCaller
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecution20250825 = &value
	case "code_execution_20260120":
		var value ServerToolCaller20260120
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecution20260120 = &value
	default:
		return fmt.Errorf("invalid type field in ResponseWebFetchToolResultBlockCaller: %q", discriminator)
	}
	return nil
}

type ResponseWebFetchToolResultBlockContent struct {
	WebFetchToolResultError *ResponseWebFetchToolResultError
	WebFetchResult          *ResponseWebFetchResultBlock
}

func (u *ResponseWebFetchToolResultBlockContent) MarshalJSON() ([]byte, error) {
	if u.WebFetchToolResultError != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseWebFetchToolResultError
		}{
			Type:                            "web_fetch_tool_result_error",
			ResponseWebFetchToolResultError: u.WebFetchToolResultError,
		})
	}
	if u.WebFetchResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseWebFetchResultBlock
		}{
			Type:                        "web_fetch_result",
			ResponseWebFetchResultBlock: u.WebFetchResult,
		})
	}
	return nil, errors.New("invalid ResponseWebFetchToolResultBlockContent: all variants are nil")
}

func (u *ResponseWebFetchToolResultBlockContent) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in ResponseWebFetchToolResultBlockContent")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = ResponseWebFetchToolResultBlockContent{}
	switch discriminator {
	case "web_fetch_tool_result_error":
		var value ResponseWebFetchToolResultError
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.WebFetchToolResultError = &value
	case "web_fetch_result":
		var value ResponseWebFetchResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.WebFetchResult = &value
	default:
		return fmt.Errorf("invalid type field in ResponseWebFetchToolResultBlockContent: %q", discriminator)
	}
	return nil
}

type ResponseWebSearchToolResultBlock struct {
	Caller    ResponseWebSearchToolResultBlockCaller  `json:"caller"`
	Content   ResponseWebSearchToolResultBlockContent `json:"content"`
	ToolUseId string                                  `json:"tool_use_id"`
}

type ResponseWebSearchToolResultBlockCaller struct {
	Direct                *DirectCaller
	CodeExecution20250825 *ServerToolCaller
	CodeExecution20260120 *ServerToolCaller20260120
}

func (u *ResponseWebSearchToolResultBlockCaller) MarshalJSON() ([]byte, error) {
	if u.Direct != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*DirectCaller
		}{
			Type:         "direct",
			DirectCaller: u.Direct,
		})
	}
	if u.CodeExecution20250825 != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ServerToolCaller
		}{
			Type:             "code_execution_20250825",
			ServerToolCaller: u.CodeExecution20250825,
		})
	}
	if u.CodeExecution20260120 != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ServerToolCaller20260120
		}{
			Type:                     "code_execution_20260120",
			ServerToolCaller20260120: u.CodeExecution20260120,
		})
	}
	return nil, errors.New("invalid ResponseWebSearchToolResultBlockCaller: all variants are nil")
}

func (u *ResponseWebSearchToolResultBlockCaller) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in ResponseWebSearchToolResultBlockCaller")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = ResponseWebSearchToolResultBlockCaller{}
	switch discriminator {
	case "direct":
		var value DirectCaller
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Direct = &value
	case "code_execution_20250825":
		var value ServerToolCaller
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecution20250825 = &value
	case "code_execution_20260120":
		var value ServerToolCaller20260120
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecution20260120 = &value
	default:
		return fmt.Errorf("invalid type field in ResponseWebSearchToolResultBlockCaller: %q", discriminator)
	}
	return nil
}

type ResponseWebSearchToolResultBlockContentArray []ResponseWebSearchResultBlock

type ResponseWebSearchToolResultBlockContent struct {
	ResponseWebSearchToolResultError             *ResponseWebSearchToolResultError
	ResponseWebSearchToolResultBlockContentArray *ResponseWebSearchToolResultBlockContentArray
}

func (u *ResponseWebSearchToolResultBlockContent) MarshalJSON() ([]byte, error) {
	if u == nil {
		return []byte("null"), nil
	}
	if u.ResponseWebSearchToolResultError != nil {
		return json.Marshal(u.ResponseWebSearchToolResultError)
	}
	if u.ResponseWebSearchToolResultBlockContentArray != nil {
		return json.Marshal(u.ResponseWebSearchToolResultBlockContentArray)
	}
	return nil, errors.New("invalid ResponseWebSearchToolResultBlockContent: all variants are nil")
}

func (u *ResponseWebSearchToolResultBlockContent) UnmarshalJSON(data []byte) error {
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*u = ResponseWebSearchToolResultBlockContent{}
	switch value := raw.(type) {
	case []interface{}:
		var v ResponseWebSearchToolResultBlockContentArray
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		u.ResponseWebSearchToolResultBlockContentArray = &v
		return nil
	case map[string]interface{}:
		if rawType, ok := value["type"]; !ok || rawType == "web_search_tool_result_error" && value["error_code"] != nil && value["type"] != nil {
			var v ResponseWebSearchToolResultError
			if err := json.Unmarshal(data, &v); err != nil {
				return err
			}
			u.ResponseWebSearchToolResultError = &v
			return nil
		}
		return errors.New("invalid ResponseWebSearchToolResultBlockContent")
	}
	return errors.New("invalid ResponseWebSearchToolResultBlockContent")
}

type CacheCreation struct {
	// The number of input tokens used to create the 1 hour cache entry.
	Ephemeral1HInputTokens int `json:"ephemeral_1h_input_tokens"`
	// The number of input tokens used to create the 5 minute cache entry.
	Ephemeral5MInputTokens int `json:"ephemeral_5m_input_tokens"`
}

type ServerToolUsage struct {
	// The number of web fetch tool requests.
	WebFetchRequests int `json:"web_fetch_requests"`
	// The number of web search tool requests.
	WebSearchRequests int `json:"web_search_requests"`
}

type CitationsDelta struct {
	Citation CitationsDeltaCitation `json:"citation"`
}

type CitationsDeltaCitation struct {
	CharLocation            *ResponseCharLocationCitation
	PageLocation            *ResponsePageLocationCitation
	ContentBlockLocation    *ResponseContentBlockLocationCitation
	WebSearchResultLocation *ResponseWebSearchResultLocationCitation
	SearchResultLocation    *ResponseSearchResultLocationCitation
}

func (u *CitationsDeltaCitation) MarshalJSON() ([]byte, error) {
	if u.CharLocation != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseCharLocationCitation
		}{
			Type:                         "char_location",
			ResponseCharLocationCitation: u.CharLocation,
		})
	}
	if u.PageLocation != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponsePageLocationCitation
		}{
			Type:                         "page_location",
			ResponsePageLocationCitation: u.PageLocation,
		})
	}
	if u.ContentBlockLocation != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseContentBlockLocationCitation
		}{
			Type:                                 "content_block_location",
			ResponseContentBlockLocationCitation: u.ContentBlockLocation,
		})
	}
	if u.WebSearchResultLocation != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseWebSearchResultLocationCitation
		}{
			Type:                                    "web_search_result_location",
			ResponseWebSearchResultLocationCitation: u.WebSearchResultLocation,
		})
	}
	if u.SearchResultLocation != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ResponseSearchResultLocationCitation
		}{
			Type:                                 "search_result_location",
			ResponseSearchResultLocationCitation: u.SearchResultLocation,
		})
	}
	return nil, errors.New("invalid CitationsDeltaCitation: all variants are nil")
}

func (u *CitationsDeltaCitation) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in CitationsDeltaCitation")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = CitationsDeltaCitation{}
	switch discriminator {
	case "char_location":
		var value ResponseCharLocationCitation
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CharLocation = &value
	case "page_location":
		var value ResponsePageLocationCitation
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.PageLocation = &value
	case "content_block_location":
		var value ResponseContentBlockLocationCitation
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ContentBlockLocation = &value
	case "web_search_result_location":
		var value ResponseWebSearchResultLocationCitation
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.WebSearchResultLocation = &value
	case "search_result_location":
		var value ResponseSearchResultLocationCitation
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.SearchResultLocation = &value
	default:
		return fmt.Errorf("invalid type field in CitationsDeltaCitation: %q", discriminator)
	}
	return nil
}

type InputJsonContentBlockDelta struct {
	PartialJson string `json:"partial_json"`
}

type SignatureContentBlockDelta struct {
	Signature string `json:"signature"`
}

type TextContentBlockDelta struct {
	Text string `json:"text"`
}

type ThinkingContentBlockDelta struct {
	Thinking string `json:"thinking"`
}

type MessageDelta struct {
	// Information about the container used in this request.
	//
	// This will be non-null if a container tool (e.g. code execution) was used.
	Container *Container `json:"container"`
	// Structured information about why model output stopped.
	//
	// This is `null` when the `stop_reason` has no additional detail to report.
	StopDetails  *RefusalStopDetails `json:"stop_details"`
	StopReason   *StopReason         `json:"stop_reason"`
	StopSequence *string             `json:"stop_sequence"`
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

type RequestBashCodeExecutionToolResultBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *RequestBashCodeExecutionToolResultBlockCacheControl `json:"cache_control,omitempty"`
	Content      RequestBashCodeExecutionToolResultBlockContent       `json:"content"`
	ToolUseId    string                                               `json:"tool_use_id"`
}

// Create a cache control breakpoint at this content block.
type RequestBashCodeExecutionToolResultBlockCacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *RequestBashCodeExecutionToolResultBlockCacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid RequestBashCodeExecutionToolResultBlockCacheControl: all variants are nil")
}

func (u *RequestBashCodeExecutionToolResultBlockCacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestBashCodeExecutionToolResultBlockCacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestBashCodeExecutionToolResultBlockCacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in RequestBashCodeExecutionToolResultBlockCacheControl: %q", discriminator)
	}
	return nil
}

type RequestBashCodeExecutionToolResultBlockContent struct {
	BashCodeExecutionToolResultError *RequestBashCodeExecutionToolResultError
	BashCodeExecutionResult          *RequestBashCodeExecutionResultBlock
}

func (u *RequestBashCodeExecutionToolResultBlockContent) MarshalJSON() ([]byte, error) {
	if u.BashCodeExecutionToolResultError != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestBashCodeExecutionToolResultError
		}{
			Type:                                    "bash_code_execution_tool_result_error",
			RequestBashCodeExecutionToolResultError: u.BashCodeExecutionToolResultError,
		})
	}
	if u.BashCodeExecutionResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestBashCodeExecutionResultBlock
		}{
			Type:                                "bash_code_execution_result",
			RequestBashCodeExecutionResultBlock: u.BashCodeExecutionResult,
		})
	}
	return nil, errors.New("invalid RequestBashCodeExecutionToolResultBlockContent: all variants are nil")
}

func (u *RequestBashCodeExecutionToolResultBlockContent) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestBashCodeExecutionToolResultBlockContent")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestBashCodeExecutionToolResultBlockContent{}
	switch discriminator {
	case "bash_code_execution_tool_result_error":
		var value RequestBashCodeExecutionToolResultError
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.BashCodeExecutionToolResultError = &value
	case "bash_code_execution_result":
		var value RequestBashCodeExecutionResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.BashCodeExecutionResult = &value
	default:
		return fmt.Errorf("invalid type field in RequestBashCodeExecutionToolResultBlockContent: %q", discriminator)
	}
	return nil
}

type RequestCodeExecutionToolResultBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *RequestCodeExecutionToolResultBlockCacheControl `json:"cache_control,omitempty"`
	Content      RequestCodeExecutionToolResultBlockContent       `json:"content"`
	ToolUseId    string                                           `json:"tool_use_id"`
}

// Create a cache control breakpoint at this content block.
type RequestCodeExecutionToolResultBlockCacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *RequestCodeExecutionToolResultBlockCacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid RequestCodeExecutionToolResultBlockCacheControl: all variants are nil")
}

func (u *RequestCodeExecutionToolResultBlockCacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestCodeExecutionToolResultBlockCacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestCodeExecutionToolResultBlockCacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in RequestCodeExecutionToolResultBlockCacheControl: %q", discriminator)
	}
	return nil
}

type RequestCodeExecutionToolResultBlockContent struct {
	CodeExecutionToolResultError *RequestCodeExecutionToolResultError
	CodeExecutionResult          *RequestCodeExecutionResultBlock
	EncryptedCodeExecutionResult *RequestEncryptedCodeExecutionResultBlock
}

func (u *RequestCodeExecutionToolResultBlockContent) MarshalJSON() ([]byte, error) {
	if u.CodeExecutionToolResultError != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestCodeExecutionToolResultError
		}{
			Type:                                "code_execution_tool_result_error",
			RequestCodeExecutionToolResultError: u.CodeExecutionToolResultError,
		})
	}
	if u.CodeExecutionResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestCodeExecutionResultBlock
		}{
			Type:                            "code_execution_result",
			RequestCodeExecutionResultBlock: u.CodeExecutionResult,
		})
	}
	if u.EncryptedCodeExecutionResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestEncryptedCodeExecutionResultBlock
		}{
			Type:                                     "encrypted_code_execution_result",
			RequestEncryptedCodeExecutionResultBlock: u.EncryptedCodeExecutionResult,
		})
	}
	return nil, errors.New("invalid RequestCodeExecutionToolResultBlockContent: all variants are nil")
}

func (u *RequestCodeExecutionToolResultBlockContent) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestCodeExecutionToolResultBlockContent")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestCodeExecutionToolResultBlockContent{}
	switch discriminator {
	case "code_execution_tool_result_error":
		var value RequestCodeExecutionToolResultError
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecutionToolResultError = &value
	case "code_execution_result":
		var value RequestCodeExecutionResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecutionResult = &value
	case "encrypted_code_execution_result":
		var value RequestEncryptedCodeExecutionResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.EncryptedCodeExecutionResult = &value
	default:
		return fmt.Errorf("invalid type field in RequestCodeExecutionToolResultBlockContent: %q", discriminator)
	}
	return nil
}

// A content block that represents a file to be uploaded to the container
// Files uploaded via this block will be available in the container's input directory.
type RequestContainerUploadBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *RequestContainerUploadBlockCacheControl `json:"cache_control,omitempty"`
	FileId       string                                   `json:"file_id"`
}

// Create a cache control breakpoint at this content block.
type RequestContainerUploadBlockCacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *RequestContainerUploadBlockCacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid RequestContainerUploadBlockCacheControl: all variants are nil")
}

func (u *RequestContainerUploadBlockCacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestContainerUploadBlockCacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestContainerUploadBlockCacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in RequestContainerUploadBlockCacheControl: %q", discriminator)
	}
	return nil
}

type RequestDocumentBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *RequestDocumentBlockCacheControl `json:"cache_control,omitempty"`
	Citations    *RequestCitationsConfig           `json:"citations,omitempty"`
	Context      *string                           `json:"context,omitempty"`
	Source       RequestDocumentBlockSource        `json:"source"`
	Title        *string                           `json:"title,omitempty"`
	Type         string                            `json:"type"`
}

// Create a cache control breakpoint at this content block.
type RequestDocumentBlockCacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *RequestDocumentBlockCacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid RequestDocumentBlockCacheControl: all variants are nil")
}

func (u *RequestDocumentBlockCacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestDocumentBlockCacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestDocumentBlockCacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in RequestDocumentBlockCacheControl: %q", discriminator)
	}
	return nil
}

type RequestDocumentBlockSource struct {
	Base64  *Base64PDFSource
	Text    *PlainTextSource
	Content *ContentBlockSource
	Url     *URLPDFSource
}

func (u *RequestDocumentBlockSource) MarshalJSON() ([]byte, error) {
	if u.Base64 != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*Base64PDFSource
		}{
			Type:            "base64",
			Base64PDFSource: u.Base64,
		})
	}
	if u.Text != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*PlainTextSource
		}{
			Type:            "text",
			PlainTextSource: u.Text,
		})
	}
	if u.Content != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ContentBlockSource
		}{
			Type:               "content",
			ContentBlockSource: u.Content,
		})
	}
	if u.Url != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*URLPDFSource
		}{
			Type:         "url",
			URLPDFSource: u.Url,
		})
	}
	return nil, errors.New("invalid RequestDocumentBlockSource: all variants are nil")
}

func (u *RequestDocumentBlockSource) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestDocumentBlockSource")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestDocumentBlockSource{}
	switch discriminator {
	case "base64":
		var value Base64PDFSource
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Base64 = &value
	case "text":
		var value PlainTextSource
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Text = &value
	case "content":
		var value ContentBlockSource
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Content = &value
	case "url":
		var value URLPDFSource
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Url = &value
	default:
		return fmt.Errorf("invalid type field in RequestDocumentBlockSource: %q", discriminator)
	}
	return nil
}

type RequestImageBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *RequestImageBlockCacheControl `json:"cache_control,omitempty"`
	Source       RequestImageBlockSource        `json:"source"`
}

// Create a cache control breakpoint at this content block.
type RequestImageBlockCacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *RequestImageBlockCacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid RequestImageBlockCacheControl: all variants are nil")
}

func (u *RequestImageBlockCacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestImageBlockCacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestImageBlockCacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in RequestImageBlockCacheControl: %q", discriminator)
	}
	return nil
}

type RequestImageBlockSource struct {
	Base64 *Base64ImageSource
	Url    *URLImageSource
}

func (u *RequestImageBlockSource) MarshalJSON() ([]byte, error) {
	if u.Base64 != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*Base64ImageSource
		}{
			Type:              "base64",
			Base64ImageSource: u.Base64,
		})
	}
	if u.Url != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*URLImageSource
		}{
			Type:           "url",
			URLImageSource: u.Url,
		})
	}
	return nil, errors.New("invalid RequestImageBlockSource: all variants are nil")
}

func (u *RequestImageBlockSource) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestImageBlockSource")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestImageBlockSource{}
	switch discriminator {
	case "base64":
		var value Base64ImageSource
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Base64 = &value
	case "url":
		var value URLImageSource
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Url = &value
	default:
		return fmt.Errorf("invalid type field in RequestImageBlockSource: %q", discriminator)
	}
	return nil
}

type RequestRedactedThinkingBlock struct {
	Data string `json:"data"`
}

type RequestSearchResultBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *RequestSearchResultBlockCacheControl `json:"cache_control,omitempty"`
	Citations    *RequestCitationsConfig               `json:"citations,omitempty"`
	Content      []RequestTextBlock                    `json:"content"`
	Source       string                                `json:"source"`
	Title        string                                `json:"title"`
}

// Create a cache control breakpoint at this content block.
type RequestSearchResultBlockCacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *RequestSearchResultBlockCacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid RequestSearchResultBlockCacheControl: all variants are nil")
}

func (u *RequestSearchResultBlockCacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestSearchResultBlockCacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestSearchResultBlockCacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in RequestSearchResultBlockCacheControl: %q", discriminator)
	}
	return nil
}

type RequestServerToolUseBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *RequestServerToolUseBlockCacheControl `json:"cache_control,omitempty"`
	Caller       *RequestServerToolUseBlockCaller       `json:"caller,omitempty"`
	Id           string                                 `json:"id"`
	Input        any                                    `json:"input"`
	Name         RequestServerToolUseBlockName          `json:"name"`
}

// Create a cache control breakpoint at this content block.
type RequestServerToolUseBlockCacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *RequestServerToolUseBlockCacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid RequestServerToolUseBlockCacheControl: all variants are nil")
}

func (u *RequestServerToolUseBlockCacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestServerToolUseBlockCacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestServerToolUseBlockCacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in RequestServerToolUseBlockCacheControl: %q", discriminator)
	}
	return nil
}

type RequestServerToolUseBlockCaller struct {
	Direct                *DirectCaller
	CodeExecution20250825 *ServerToolCaller
	CodeExecution20260120 *ServerToolCaller20260120
}

func (u *RequestServerToolUseBlockCaller) MarshalJSON() ([]byte, error) {
	if u.Direct != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*DirectCaller
		}{
			Type:         "direct",
			DirectCaller: u.Direct,
		})
	}
	if u.CodeExecution20250825 != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ServerToolCaller
		}{
			Type:             "code_execution_20250825",
			ServerToolCaller: u.CodeExecution20250825,
		})
	}
	if u.CodeExecution20260120 != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ServerToolCaller20260120
		}{
			Type:                     "code_execution_20260120",
			ServerToolCaller20260120: u.CodeExecution20260120,
		})
	}
	return nil, errors.New("invalid RequestServerToolUseBlockCaller: all variants are nil")
}

func (u *RequestServerToolUseBlockCaller) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestServerToolUseBlockCaller")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestServerToolUseBlockCaller{}
	switch discriminator {
	case "direct":
		var value DirectCaller
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Direct = &value
	case "code_execution_20250825":
		var value ServerToolCaller
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecution20250825 = &value
	case "code_execution_20260120":
		var value ServerToolCaller20260120
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecution20260120 = &value
	default:
		return fmt.Errorf("invalid type field in RequestServerToolUseBlockCaller: %q", discriminator)
	}
	return nil
}

type RequestServerToolUseBlockName string

const (
	RequestServerToolUseBlockNameWebSearch               RequestServerToolUseBlockName = "web_search"
	RequestServerToolUseBlockNameWebFetch                RequestServerToolUseBlockName = "web_fetch"
	RequestServerToolUseBlockNameCodeExecution           RequestServerToolUseBlockName = "code_execution"
	RequestServerToolUseBlockNameBashCodeExecution       RequestServerToolUseBlockName = "bash_code_execution"
	RequestServerToolUseBlockNameTextEditorCodeExecution RequestServerToolUseBlockName = "text_editor_code_execution"
	RequestServerToolUseBlockNameToolSearchToolRegex     RequestServerToolUseBlockName = "tool_search_tool_regex"
	RequestServerToolUseBlockNameToolSearchToolBm25      RequestServerToolUseBlockName = "tool_search_tool_bm25"
)

type RequestTextEditorCodeExecutionToolResultBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *RequestTextEditorCodeExecutionToolResultBlockCacheControl `json:"cache_control,omitempty"`
	Content      RequestTextEditorCodeExecutionToolResultBlockContent       `json:"content"`
	ToolUseId    string                                                     `json:"tool_use_id"`
}

// Create a cache control breakpoint at this content block.
type RequestTextEditorCodeExecutionToolResultBlockCacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *RequestTextEditorCodeExecutionToolResultBlockCacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid RequestTextEditorCodeExecutionToolResultBlockCacheControl: all variants are nil")
}

func (u *RequestTextEditorCodeExecutionToolResultBlockCacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestTextEditorCodeExecutionToolResultBlockCacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestTextEditorCodeExecutionToolResultBlockCacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in RequestTextEditorCodeExecutionToolResultBlockCacheControl: %q", discriminator)
	}
	return nil
}

type RequestTextEditorCodeExecutionToolResultBlockContent struct {
	TextEditorCodeExecutionToolResultError  *RequestTextEditorCodeExecutionToolResultError
	TextEditorCodeExecutionViewResult       *RequestTextEditorCodeExecutionViewResultBlock
	TextEditorCodeExecutionCreateResult     *RequestTextEditorCodeExecutionCreateResultBlock
	TextEditorCodeExecutionStrReplaceResult *RequestTextEditorCodeExecutionStrReplaceResultBlock
}

func (u *RequestTextEditorCodeExecutionToolResultBlockContent) MarshalJSON() ([]byte, error) {
	if u.TextEditorCodeExecutionToolResultError != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestTextEditorCodeExecutionToolResultError
		}{
			Type: "text_editor_code_execution_tool_result_error",
			RequestTextEditorCodeExecutionToolResultError: u.TextEditorCodeExecutionToolResultError,
		})
	}
	if u.TextEditorCodeExecutionViewResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestTextEditorCodeExecutionViewResultBlock
		}{
			Type: "text_editor_code_execution_view_result",
			RequestTextEditorCodeExecutionViewResultBlock: u.TextEditorCodeExecutionViewResult,
		})
	}
	if u.TextEditorCodeExecutionCreateResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestTextEditorCodeExecutionCreateResultBlock
		}{
			Type: "text_editor_code_execution_create_result",
			RequestTextEditorCodeExecutionCreateResultBlock: u.TextEditorCodeExecutionCreateResult,
		})
	}
	if u.TextEditorCodeExecutionStrReplaceResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestTextEditorCodeExecutionStrReplaceResultBlock
		}{
			Type: "text_editor_code_execution_str_replace_result",
			RequestTextEditorCodeExecutionStrReplaceResultBlock: u.TextEditorCodeExecutionStrReplaceResult,
		})
	}
	return nil, errors.New("invalid RequestTextEditorCodeExecutionToolResultBlockContent: all variants are nil")
}

func (u *RequestTextEditorCodeExecutionToolResultBlockContent) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestTextEditorCodeExecutionToolResultBlockContent")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestTextEditorCodeExecutionToolResultBlockContent{}
	switch discriminator {
	case "text_editor_code_execution_tool_result_error":
		var value RequestTextEditorCodeExecutionToolResultError
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.TextEditorCodeExecutionToolResultError = &value
	case "text_editor_code_execution_view_result":
		var value RequestTextEditorCodeExecutionViewResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.TextEditorCodeExecutionViewResult = &value
	case "text_editor_code_execution_create_result":
		var value RequestTextEditorCodeExecutionCreateResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.TextEditorCodeExecutionCreateResult = &value
	case "text_editor_code_execution_str_replace_result":
		var value RequestTextEditorCodeExecutionStrReplaceResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.TextEditorCodeExecutionStrReplaceResult = &value
	default:
		return fmt.Errorf("invalid type field in RequestTextEditorCodeExecutionToolResultBlockContent: %q", discriminator)
	}
	return nil
}

type RequestThinkingBlock struct {
	Signature string `json:"signature"`
	Thinking  string `json:"thinking"`
}

type RequestToolResultBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *RequestToolResultBlockCacheControl `json:"cache_control,omitempty"`
	Content      *RequestToolResultBlockContent      `json:"content,omitempty"`
	IsError      *bool                               `json:"is_error,omitempty"`
	ToolUseId    string                              `json:"tool_use_id"`
}

// Create a cache control breakpoint at this content block.
type RequestToolResultBlockCacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *RequestToolResultBlockCacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid RequestToolResultBlockCacheControl: all variants are nil")
}

func (u *RequestToolResultBlockCacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestToolResultBlockCacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestToolResultBlockCacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in RequestToolResultBlockCacheControl: %q", discriminator)
	}
	return nil
}

type RequestToolResultBlockContentString *string

type RequestToolResultBlockContentArrayItem struct {
	Text          *RequestTextBlock
	Image         *RequestImageBlock
	SearchResult  *RequestSearchResultBlock
	Document      *RequestDocumentBlock
	ToolReference *RequestToolReferenceBlock
}

func (u *RequestToolResultBlockContentArrayItem) MarshalJSON() ([]byte, error) {
	if u.Text != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestTextBlock
		}{
			Type:             "text",
			RequestTextBlock: u.Text,
		})
	}
	if u.Image != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestImageBlock
		}{
			Type:              "image",
			RequestImageBlock: u.Image,
		})
	}
	if u.SearchResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestSearchResultBlock
		}{
			Type:                     "search_result",
			RequestSearchResultBlock: u.SearchResult,
		})
	}
	if u.Document != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestDocumentBlock
		}{
			Type:                 "document",
			RequestDocumentBlock: u.Document,
		})
	}
	if u.ToolReference != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestToolReferenceBlock
		}{
			Type:                      "tool_reference",
			RequestToolReferenceBlock: u.ToolReference,
		})
	}
	return nil, errors.New("invalid RequestToolResultBlockContentArrayItem: all variants are nil")
}

func (u *RequestToolResultBlockContentArrayItem) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestToolResultBlockContentArrayItem")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestToolResultBlockContentArrayItem{}
	switch discriminator {
	case "text":
		var value RequestTextBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Text = &value
	case "image":
		var value RequestImageBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Image = &value
	case "search_result":
		var value RequestSearchResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.SearchResult = &value
	case "document":
		var value RequestDocumentBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Document = &value
	case "tool_reference":
		var value RequestToolReferenceBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ToolReference = &value
	default:
		return fmt.Errorf("invalid type field in RequestToolResultBlockContentArrayItem: %q", discriminator)
	}
	return nil
}

type RequestToolResultBlockContentArray []RequestToolResultBlockContentArrayItem

type RequestToolResultBlockContent struct {
	RequestToolResultBlockContentString *RequestToolResultBlockContentString
	RequestToolResultBlockContentArray  *RequestToolResultBlockContentArray
}

func (u *RequestToolResultBlockContent) MarshalJSON() ([]byte, error) {
	if u == nil {
		return []byte("null"), nil
	}
	if u.RequestToolResultBlockContentString != nil {
		return json.Marshal(u.RequestToolResultBlockContentString)
	}
	if u.RequestToolResultBlockContentArray != nil {
		return json.Marshal(u.RequestToolResultBlockContentArray)
	}
	return nil, errors.New("invalid RequestToolResultBlockContent: all variants are nil")
}

func (u *RequestToolResultBlockContent) UnmarshalJSON(data []byte) error {
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*u = RequestToolResultBlockContent{}
	switch raw.(type) {
	case string:
		var v RequestToolResultBlockContentString
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		u.RequestToolResultBlockContentString = &v
		return nil
	case []interface{}:
		var v RequestToolResultBlockContentArray
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		u.RequestToolResultBlockContentArray = &v
		return nil
	}
	return errors.New("invalid RequestToolResultBlockContent")
}

type RequestToolSearchToolResultBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *RequestToolSearchToolResultBlockCacheControl `json:"cache_control,omitempty"`
	Content      RequestToolSearchToolResultBlockContent       `json:"content"`
	ToolUseId    string                                        `json:"tool_use_id"`
}

// Create a cache control breakpoint at this content block.
type RequestToolSearchToolResultBlockCacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *RequestToolSearchToolResultBlockCacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid RequestToolSearchToolResultBlockCacheControl: all variants are nil")
}

func (u *RequestToolSearchToolResultBlockCacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestToolSearchToolResultBlockCacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestToolSearchToolResultBlockCacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in RequestToolSearchToolResultBlockCacheControl: %q", discriminator)
	}
	return nil
}

type RequestToolSearchToolResultBlockContent struct {
	ToolSearchToolResultError  *RequestToolSearchToolResultError
	ToolSearchToolSearchResult *RequestToolSearchToolSearchResultBlock
}

func (u *RequestToolSearchToolResultBlockContent) MarshalJSON() ([]byte, error) {
	if u.ToolSearchToolResultError != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestToolSearchToolResultError
		}{
			Type:                             "tool_search_tool_result_error",
			RequestToolSearchToolResultError: u.ToolSearchToolResultError,
		})
	}
	if u.ToolSearchToolSearchResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestToolSearchToolSearchResultBlock
		}{
			Type:                                   "tool_search_tool_search_result",
			RequestToolSearchToolSearchResultBlock: u.ToolSearchToolSearchResult,
		})
	}
	return nil, errors.New("invalid RequestToolSearchToolResultBlockContent: all variants are nil")
}

func (u *RequestToolSearchToolResultBlockContent) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestToolSearchToolResultBlockContent")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestToolSearchToolResultBlockContent{}
	switch discriminator {
	case "tool_search_tool_result_error":
		var value RequestToolSearchToolResultError
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ToolSearchToolResultError = &value
	case "tool_search_tool_search_result":
		var value RequestToolSearchToolSearchResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.ToolSearchToolSearchResult = &value
	default:
		return fmt.Errorf("invalid type field in RequestToolSearchToolResultBlockContent: %q", discriminator)
	}
	return nil
}

type RequestToolUseBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *RequestToolUseBlockCacheControl `json:"cache_control,omitempty"`
	Caller       *RequestToolUseBlockCaller       `json:"caller,omitempty"`
	Id           string                           `json:"id"`
	Input        any                              `json:"input"`
	Name         string                           `json:"name"`
}

// Create a cache control breakpoint at this content block.
type RequestToolUseBlockCacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *RequestToolUseBlockCacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid RequestToolUseBlockCacheControl: all variants are nil")
}

func (u *RequestToolUseBlockCacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestToolUseBlockCacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestToolUseBlockCacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in RequestToolUseBlockCacheControl: %q", discriminator)
	}
	return nil
}

type RequestToolUseBlockCaller struct {
	Direct                *DirectCaller
	CodeExecution20250825 *ServerToolCaller
	CodeExecution20260120 *ServerToolCaller20260120
}

func (u *RequestToolUseBlockCaller) MarshalJSON() ([]byte, error) {
	if u.Direct != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*DirectCaller
		}{
			Type:         "direct",
			DirectCaller: u.Direct,
		})
	}
	if u.CodeExecution20250825 != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ServerToolCaller
		}{
			Type:             "code_execution_20250825",
			ServerToolCaller: u.CodeExecution20250825,
		})
	}
	if u.CodeExecution20260120 != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ServerToolCaller20260120
		}{
			Type:                     "code_execution_20260120",
			ServerToolCaller20260120: u.CodeExecution20260120,
		})
	}
	return nil, errors.New("invalid RequestToolUseBlockCaller: all variants are nil")
}

func (u *RequestToolUseBlockCaller) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestToolUseBlockCaller")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestToolUseBlockCaller{}
	switch discriminator {
	case "direct":
		var value DirectCaller
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Direct = &value
	case "code_execution_20250825":
		var value ServerToolCaller
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecution20250825 = &value
	case "code_execution_20260120":
		var value ServerToolCaller20260120
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecution20260120 = &value
	default:
		return fmt.Errorf("invalid type field in RequestToolUseBlockCaller: %q", discriminator)
	}
	return nil
}

type RequestWebFetchToolResultBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *RequestWebFetchToolResultBlockCacheControl `json:"cache_control,omitempty"`
	Caller       *RequestWebFetchToolResultBlockCaller       `json:"caller,omitempty"`
	Content      RequestWebFetchToolResultBlockContent       `json:"content"`
	ToolUseId    string                                      `json:"tool_use_id"`
}

// Create a cache control breakpoint at this content block.
type RequestWebFetchToolResultBlockCacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *RequestWebFetchToolResultBlockCacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid RequestWebFetchToolResultBlockCacheControl: all variants are nil")
}

func (u *RequestWebFetchToolResultBlockCacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestWebFetchToolResultBlockCacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestWebFetchToolResultBlockCacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in RequestWebFetchToolResultBlockCacheControl: %q", discriminator)
	}
	return nil
}

type RequestWebFetchToolResultBlockCaller struct {
	Direct                *DirectCaller
	CodeExecution20250825 *ServerToolCaller
	CodeExecution20260120 *ServerToolCaller20260120
}

func (u *RequestWebFetchToolResultBlockCaller) MarshalJSON() ([]byte, error) {
	if u.Direct != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*DirectCaller
		}{
			Type:         "direct",
			DirectCaller: u.Direct,
		})
	}
	if u.CodeExecution20250825 != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ServerToolCaller
		}{
			Type:             "code_execution_20250825",
			ServerToolCaller: u.CodeExecution20250825,
		})
	}
	if u.CodeExecution20260120 != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ServerToolCaller20260120
		}{
			Type:                     "code_execution_20260120",
			ServerToolCaller20260120: u.CodeExecution20260120,
		})
	}
	return nil, errors.New("invalid RequestWebFetchToolResultBlockCaller: all variants are nil")
}

func (u *RequestWebFetchToolResultBlockCaller) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestWebFetchToolResultBlockCaller")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestWebFetchToolResultBlockCaller{}
	switch discriminator {
	case "direct":
		var value DirectCaller
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Direct = &value
	case "code_execution_20250825":
		var value ServerToolCaller
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecution20250825 = &value
	case "code_execution_20260120":
		var value ServerToolCaller20260120
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecution20260120 = &value
	default:
		return fmt.Errorf("invalid type field in RequestWebFetchToolResultBlockCaller: %q", discriminator)
	}
	return nil
}

type RequestWebFetchToolResultBlockContent struct {
	WebFetchToolResultError *RequestWebFetchToolResultError
	WebFetchResult          *RequestWebFetchResultBlock
}

func (u *RequestWebFetchToolResultBlockContent) MarshalJSON() ([]byte, error) {
	if u.WebFetchToolResultError != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestWebFetchToolResultError
		}{
			Type:                           "web_fetch_tool_result_error",
			RequestWebFetchToolResultError: u.WebFetchToolResultError,
		})
	}
	if u.WebFetchResult != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestWebFetchResultBlock
		}{
			Type:                       "web_fetch_result",
			RequestWebFetchResultBlock: u.WebFetchResult,
		})
	}
	return nil, errors.New("invalid RequestWebFetchToolResultBlockContent: all variants are nil")
}

func (u *RequestWebFetchToolResultBlockContent) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestWebFetchToolResultBlockContent")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestWebFetchToolResultBlockContent{}
	switch discriminator {
	case "web_fetch_tool_result_error":
		var value RequestWebFetchToolResultError
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.WebFetchToolResultError = &value
	case "web_fetch_result":
		var value RequestWebFetchResultBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.WebFetchResult = &value
	default:
		return fmt.Errorf("invalid type field in RequestWebFetchToolResultBlockContent: %q", discriminator)
	}
	return nil
}

type RequestWebSearchToolResultBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *RequestWebSearchToolResultBlockCacheControl `json:"cache_control,omitempty"`
	Caller       *RequestWebSearchToolResultBlockCaller       `json:"caller,omitempty"`
	Content      RequestWebSearchToolResultBlockContent       `json:"content"`
	ToolUseId    string                                       `json:"tool_use_id"`
}

// Create a cache control breakpoint at this content block.
type RequestWebSearchToolResultBlockCacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *RequestWebSearchToolResultBlockCacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid RequestWebSearchToolResultBlockCacheControl: all variants are nil")
}

func (u *RequestWebSearchToolResultBlockCacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestWebSearchToolResultBlockCacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestWebSearchToolResultBlockCacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in RequestWebSearchToolResultBlockCacheControl: %q", discriminator)
	}
	return nil
}

type RequestWebSearchToolResultBlockCaller struct {
	Direct                *DirectCaller
	CodeExecution20250825 *ServerToolCaller
	CodeExecution20260120 *ServerToolCaller20260120
}

func (u *RequestWebSearchToolResultBlockCaller) MarshalJSON() ([]byte, error) {
	if u.Direct != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*DirectCaller
		}{
			Type:         "direct",
			DirectCaller: u.Direct,
		})
	}
	if u.CodeExecution20250825 != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ServerToolCaller
		}{
			Type:             "code_execution_20250825",
			ServerToolCaller: u.CodeExecution20250825,
		})
	}
	if u.CodeExecution20260120 != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*ServerToolCaller20260120
		}{
			Type:                     "code_execution_20260120",
			ServerToolCaller20260120: u.CodeExecution20260120,
		})
	}
	return nil, errors.New("invalid RequestWebSearchToolResultBlockCaller: all variants are nil")
}

func (u *RequestWebSearchToolResultBlockCaller) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestWebSearchToolResultBlockCaller")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestWebSearchToolResultBlockCaller{}
	switch discriminator {
	case "direct":
		var value DirectCaller
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Direct = &value
	case "code_execution_20250825":
		var value ServerToolCaller
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecution20250825 = &value
	case "code_execution_20260120":
		var value ServerToolCaller20260120
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.CodeExecution20260120 = &value
	default:
		return fmt.Errorf("invalid type field in RequestWebSearchToolResultBlockCaller: %q", discriminator)
	}
	return nil
}

type RequestWebSearchToolResultBlockContentArray []RequestWebSearchResultBlock

type RequestWebSearchToolResultBlockContent struct {
	RequestWebSearchToolResultBlockContentArray *RequestWebSearchToolResultBlockContentArray
	RequestWebSearchToolResultError             *RequestWebSearchToolResultError
}

func (u *RequestWebSearchToolResultBlockContent) MarshalJSON() ([]byte, error) {
	if u == nil {
		return []byte("null"), nil
	}
	if u.RequestWebSearchToolResultBlockContentArray != nil {
		return json.Marshal(u.RequestWebSearchToolResultBlockContentArray)
	}
	if u.RequestWebSearchToolResultError != nil {
		return json.Marshal(u.RequestWebSearchToolResultError)
	}
	return nil, errors.New("invalid RequestWebSearchToolResultBlockContent: all variants are nil")
}

func (u *RequestWebSearchToolResultBlockContent) UnmarshalJSON(data []byte) error {
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*u = RequestWebSearchToolResultBlockContent{}
	switch value := raw.(type) {
	case []interface{}:
		var v RequestWebSearchToolResultBlockContentArray
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		u.RequestWebSearchToolResultBlockContentArray = &v
		return nil
	case map[string]interface{}:
		if rawType, ok := value["type"]; !ok || rawType == "web_search_tool_result_error" && value["error_code"] != nil && value["type"] != nil {
			var v RequestWebSearchToolResultError
			if err := json.Unmarshal(data, &v); err != nil {
				return err
			}
			u.RequestWebSearchToolResultError = &v
			return nil
		}
		return errors.New("invalid RequestWebSearchToolResultBlockContent")
	}
	return errors.New("invalid RequestWebSearchToolResultBlockContent")
}

type ThinkingDisplayMode string

const (
	ThinkingDisplayModeSummarized ThinkingDisplayMode = "summarized"
	ThinkingDisplayModeOmitted    ThinkingDisplayMode = "omitted"
)

type ResponseBashCodeExecutionToolResultError struct {
	ErrorCode BashCodeExecutionToolResultErrorCode `json:"error_code"`
}

type ResponseBashCodeExecutionResultBlock struct {
	Content    []ResponseBashCodeExecutionOutputBlock `json:"content"`
	ReturnCode int                                    `json:"return_code"`
	Stderr     string                                 `json:"stderr"`
	Stdout     string                                 `json:"stdout"`
}

type ResponseCodeExecutionToolResultError struct {
	ErrorCode CodeExecutionToolResultErrorCode `json:"error_code"`
}

type ResponseCodeExecutionResultBlock struct {
	Content    []ResponseCodeExecutionOutputBlock `json:"content"`
	ReturnCode int                                `json:"return_code"`
	Stderr     string                             `json:"stderr"`
	Stdout     string                             `json:"stdout"`
}

// Code execution result with encrypted stdout for PFC + web_search results.
type ResponseEncryptedCodeExecutionResultBlock struct {
	Content         []ResponseCodeExecutionOutputBlock `json:"content"`
	EncryptedStdout string                             `json:"encrypted_stdout"`
	ReturnCode      int                                `json:"return_code"`
	Stderr          string                             `json:"stderr"`
}

// Tool invocation generated by a server-side tool.
type ServerToolCaller struct {
	ToolId string `json:"tool_id"`
}

type ServerToolCaller20260120 struct {
	ToolId string `json:"tool_id"`
}

// Tool invocation directly from the model.
type DirectCaller struct {
}

type ResponseCharLocationCitation struct {
	CitedText      string  `json:"cited_text"`
	DocumentIndex  int     `json:"document_index"`
	DocumentTitle  *string `json:"document_title"`
	EndCharIndex   int     `json:"end_char_index"`
	FileId         *string `json:"file_id"`
	StartCharIndex int     `json:"start_char_index"`
}

type ResponseContentBlockLocationCitation struct {
	CitedText       string  `json:"cited_text"`
	DocumentIndex   int     `json:"document_index"`
	DocumentTitle   *string `json:"document_title"`
	EndBlockIndex   int     `json:"end_block_index"`
	FileId          *string `json:"file_id"`
	StartBlockIndex int     `json:"start_block_index"`
}

type ResponsePageLocationCitation struct {
	CitedText       string  `json:"cited_text"`
	DocumentIndex   int     `json:"document_index"`
	DocumentTitle   *string `json:"document_title"`
	EndPageNumber   int     `json:"end_page_number"`
	FileId          *string `json:"file_id"`
	StartPageNumber int     `json:"start_page_number"`
}

type ResponseSearchResultLocationCitation struct {
	CitedText         string  `json:"cited_text"`
	EndBlockIndex     int     `json:"end_block_index"`
	SearchResultIndex int     `json:"search_result_index"`
	Source            string  `json:"source"`
	StartBlockIndex   int     `json:"start_block_index"`
	Title             *string `json:"title"`
}

type ResponseWebSearchResultLocationCitation struct {
	CitedText      string  `json:"cited_text"`
	EncryptedIndex string  `json:"encrypted_index"`
	Title          *string `json:"title"`
	Url            string  `json:"url"`
}

type ResponseTextEditorCodeExecutionToolResultError struct {
	ErrorCode    TextEditorCodeExecutionToolResultErrorCode `json:"error_code"`
	ErrorMessage *string                                    `json:"error_message"`
}

type ResponseTextEditorCodeExecutionViewResultBlock struct {
	Content    string                                                 `json:"content"`
	FileType   ResponseTextEditorCodeExecutionViewResultBlockFileType `json:"file_type"`
	NumLines   *int                                                   `json:"num_lines"`
	StartLine  *int                                                   `json:"start_line"`
	TotalLines *int                                                   `json:"total_lines"`
}

type ResponseTextEditorCodeExecutionViewResultBlockFileType string

const (
	ResponseTextEditorCodeExecutionViewResultBlockFileTypeText  ResponseTextEditorCodeExecutionViewResultBlockFileType = "text"
	ResponseTextEditorCodeExecutionViewResultBlockFileTypeImage ResponseTextEditorCodeExecutionViewResultBlockFileType = "image"
	ResponseTextEditorCodeExecutionViewResultBlockFileTypePdf   ResponseTextEditorCodeExecutionViewResultBlockFileType = "pdf"
)

type ResponseTextEditorCodeExecutionCreateResultBlock struct {
	IsFileUpdate bool `json:"is_file_update"`
}

type ResponseTextEditorCodeExecutionStrReplaceResultBlock struct {
	Lines    []string `json:"lines"`
	NewLines *int     `json:"new_lines"`
	NewStart *int     `json:"new_start"`
	OldLines *int     `json:"old_lines"`
	OldStart *int     `json:"old_start"`
}

type ResponseToolSearchToolResultError struct {
	ErrorCode    ToolSearchToolResultErrorCode `json:"error_code"`
	ErrorMessage *string                       `json:"error_message"`
}

type ResponseToolSearchToolSearchResultBlock struct {
	ToolReferences []ResponseToolReferenceBlock `json:"tool_references"`
}

type ResponseWebFetchToolResultError struct {
	ErrorCode WebFetchToolResultErrorCode `json:"error_code"`
}

type ResponseWebFetchResultBlock struct {
	Content ResponseDocumentBlock `json:"content"`
	// ISO 8601 timestamp when the content was retrieved
	RetrievedAt *string `json:"retrieved_at"`
	// Fetched content URL
	Url string `json:"url"`
}

type ResponseWebSearchToolResultError struct {
	ErrorCode WebSearchToolResultErrorCode `json:"error_code"`
	Type      string                       `json:"type"`
}

type ResponseWebSearchResultBlock struct {
	EncryptedContent string  `json:"encrypted_content"`
	PageAge          *string `json:"page_age"`
	Title            string  `json:"title"`
	Type             string  `json:"type"`
	Url              string  `json:"url"`
}

type RequestBashCodeExecutionToolResultError struct {
	ErrorCode BashCodeExecutionToolResultErrorCode `json:"error_code"`
}

type RequestBashCodeExecutionResultBlock struct {
	Content    []RequestBashCodeExecutionOutputBlock `json:"content"`
	ReturnCode int                                   `json:"return_code"`
	Stderr     string                                `json:"stderr"`
	Stdout     string                                `json:"stdout"`
}

type RequestCodeExecutionToolResultError struct {
	ErrorCode CodeExecutionToolResultErrorCode `json:"error_code"`
}

type RequestCodeExecutionResultBlock struct {
	Content    []RequestCodeExecutionOutputBlock `json:"content"`
	ReturnCode int                               `json:"return_code"`
	Stderr     string                            `json:"stderr"`
	Stdout     string                            `json:"stdout"`
}

// Code execution result with encrypted stdout for PFC + web_search results.
type RequestEncryptedCodeExecutionResultBlock struct {
	Content         []RequestCodeExecutionOutputBlock `json:"content"`
	EncryptedStdout string                            `json:"encrypted_stdout"`
	ReturnCode      int                               `json:"return_code"`
	Stderr          string                            `json:"stderr"`
}

type Base64PDFSource struct {
	Data      string `json:"data"`
	MediaType string `json:"media_type"`
}

type ContentBlockSource struct {
	Content ContentBlockSourceContent `json:"content"`
}

type ContentBlockSourceContentString *string

type ContentBlockSourceContentArrayItem struct {
	Text  *RequestTextBlock
	Image *RequestImageBlock
}

func (u *ContentBlockSourceContentArrayItem) MarshalJSON() ([]byte, error) {
	if u.Text != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestTextBlock
		}{
			Type:             "text",
			RequestTextBlock: u.Text,
		})
	}
	if u.Image != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*RequestImageBlock
		}{
			Type:              "image",
			RequestImageBlock: u.Image,
		})
	}
	return nil, errors.New("invalid ContentBlockSourceContentArrayItem: all variants are nil")
}

func (u *ContentBlockSourceContentArrayItem) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in ContentBlockSourceContentArrayItem")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = ContentBlockSourceContentArrayItem{}
	switch discriminator {
	case "text":
		var value RequestTextBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Text = &value
	case "image":
		var value RequestImageBlock
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Image = &value
	default:
		return fmt.Errorf("invalid type field in ContentBlockSourceContentArrayItem: %q", discriminator)
	}
	return nil
}

type ContentBlockSourceContentArray []ContentBlockSourceContentArrayItem

type ContentBlockSourceContent struct {
	ContentBlockSourceContentString *ContentBlockSourceContentString
	ContentBlockSourceContentArray  *ContentBlockSourceContentArray
}

func (u *ContentBlockSourceContent) MarshalJSON() ([]byte, error) {
	if u == nil {
		return []byte("null"), nil
	}
	if u.ContentBlockSourceContentString != nil {
		return json.Marshal(u.ContentBlockSourceContentString)
	}
	if u.ContentBlockSourceContentArray != nil {
		return json.Marshal(u.ContentBlockSourceContentArray)
	}
	return nil, errors.New("invalid ContentBlockSourceContent: all variants are nil")
}

func (u *ContentBlockSourceContent) UnmarshalJSON(data []byte) error {
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*u = ContentBlockSourceContent{}
	switch raw.(type) {
	case string:
		var v ContentBlockSourceContentString
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		u.ContentBlockSourceContentString = &v
		return nil
	case []interface{}:
		var v ContentBlockSourceContentArray
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		u.ContentBlockSourceContentArray = &v
		return nil
	}
	return errors.New("invalid ContentBlockSourceContent")
}

type PlainTextSource struct {
	Data      string `json:"data"`
	MediaType string `json:"media_type"`
}

type URLPDFSource struct {
	Url string `json:"url"`
}

type Base64ImageSource struct {
	Data      string                     `json:"data"`
	MediaType Base64ImageSourceMediaType `json:"media_type"`
}

type Base64ImageSourceMediaType string

const (
	Base64ImageSourceMediaTypeImageJpeg Base64ImageSourceMediaType = "image/jpeg"
	Base64ImageSourceMediaTypeImagePng  Base64ImageSourceMediaType = "image/png"
	Base64ImageSourceMediaTypeImageGif  Base64ImageSourceMediaType = "image/gif"
	Base64ImageSourceMediaTypeImageWebp Base64ImageSourceMediaType = "image/webp"
)

type URLImageSource struct {
	Url string `json:"url"`
}

type RequestTextEditorCodeExecutionToolResultError struct {
	ErrorCode    TextEditorCodeExecutionToolResultErrorCode `json:"error_code"`
	ErrorMessage *string                                    `json:"error_message,omitempty"`
}

type RequestTextEditorCodeExecutionViewResultBlock struct {
	Content    string                                                `json:"content"`
	FileType   RequestTextEditorCodeExecutionViewResultBlockFileType `json:"file_type"`
	NumLines   *int                                                  `json:"num_lines,omitempty"`
	StartLine  *int                                                  `json:"start_line,omitempty"`
	TotalLines *int                                                  `json:"total_lines,omitempty"`
}

type RequestTextEditorCodeExecutionViewResultBlockFileType string

const (
	RequestTextEditorCodeExecutionViewResultBlockFileTypeText  RequestTextEditorCodeExecutionViewResultBlockFileType = "text"
	RequestTextEditorCodeExecutionViewResultBlockFileTypeImage RequestTextEditorCodeExecutionViewResultBlockFileType = "image"
	RequestTextEditorCodeExecutionViewResultBlockFileTypePdf   RequestTextEditorCodeExecutionViewResultBlockFileType = "pdf"
)

type RequestTextEditorCodeExecutionCreateResultBlock struct {
	IsFileUpdate bool `json:"is_file_update"`
}

type RequestTextEditorCodeExecutionStrReplaceResultBlock struct {
	Lines    []string `json:"lines,omitempty"`
	NewLines *int     `json:"new_lines,omitempty"`
	NewStart *int     `json:"new_start,omitempty"`
	OldLines *int     `json:"old_lines,omitempty"`
	OldStart *int     `json:"old_start,omitempty"`
}

// Tool reference block that can be included in tool_result content.
type RequestToolReferenceBlock struct {
	// Create a cache control breakpoint at this content block.
	CacheControl *RequestToolReferenceBlockCacheControl `json:"cache_control,omitempty"`
	ToolName     string                                 `json:"tool_name"`
	Type         string                                 `json:"type"`
}

// Create a cache control breakpoint at this content block.
type RequestToolReferenceBlockCacheControl struct {
	Ephemeral *CacheControlEphemeral
}

func (u *RequestToolReferenceBlockCacheControl) MarshalJSON() ([]byte, error) {
	if u.Ephemeral != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*CacheControlEphemeral
		}{
			Type:                  "ephemeral",
			CacheControlEphemeral: u.Ephemeral,
		})
	}
	return nil, errors.New("invalid RequestToolReferenceBlockCacheControl: all variants are nil")
}

func (u *RequestToolReferenceBlockCacheControl) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in RequestToolReferenceBlockCacheControl")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = RequestToolReferenceBlockCacheControl{}
	switch discriminator {
	case "ephemeral":
		var value CacheControlEphemeral
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Ephemeral = &value
	default:
		return fmt.Errorf("invalid type field in RequestToolReferenceBlockCacheControl: %q", discriminator)
	}
	return nil
}

type RequestToolSearchToolResultError struct {
	ErrorCode ToolSearchToolResultErrorCode `json:"error_code"`
}

type RequestToolSearchToolSearchResultBlock struct {
	ToolReferences []RequestToolReferenceBlock `json:"tool_references"`
}

type RequestWebFetchToolResultError struct {
	ErrorCode WebFetchToolResultErrorCode `json:"error_code"`
}

type RequestWebFetchResultBlock struct {
	Content RequestDocumentBlock `json:"content"`
	// ISO 8601 timestamp when the content was retrieved
	RetrievedAt *string `json:"retrieved_at,omitempty"`
	// Fetched content URL
	Url string `json:"url"`
}

type RequestWebSearchResultBlock struct {
	EncryptedContent string  `json:"encrypted_content"`
	PageAge          *string `json:"page_age,omitempty"`
	Title            string  `json:"title"`
	Type             string  `json:"type"`
	Url              string  `json:"url"`
}

type RequestWebSearchToolResultError struct {
	ErrorCode WebSearchToolResultErrorCode `json:"error_code"`
	Type      string                       `json:"type"`
}

type BashCodeExecutionToolResultErrorCode string

const (
	BashCodeExecutionToolResultErrorCodeInvalidToolInput      BashCodeExecutionToolResultErrorCode = "invalid_tool_input"
	BashCodeExecutionToolResultErrorCodeUnavailable           BashCodeExecutionToolResultErrorCode = "unavailable"
	BashCodeExecutionToolResultErrorCodeTooManyRequests       BashCodeExecutionToolResultErrorCode = "too_many_requests"
	BashCodeExecutionToolResultErrorCodeExecutionTimeExceeded BashCodeExecutionToolResultErrorCode = "execution_time_exceeded"
	BashCodeExecutionToolResultErrorCodeOutputFileTooLarge    BashCodeExecutionToolResultErrorCode = "output_file_too_large"
)

type ResponseBashCodeExecutionOutputBlock struct {
	FileId string `json:"file_id"`
	Type   string `json:"type"`
}

type CodeExecutionToolResultErrorCode string

const (
	CodeExecutionToolResultErrorCodeInvalidToolInput      CodeExecutionToolResultErrorCode = "invalid_tool_input"
	CodeExecutionToolResultErrorCodeUnavailable           CodeExecutionToolResultErrorCode = "unavailable"
	CodeExecutionToolResultErrorCodeTooManyRequests       CodeExecutionToolResultErrorCode = "too_many_requests"
	CodeExecutionToolResultErrorCodeExecutionTimeExceeded CodeExecutionToolResultErrorCode = "execution_time_exceeded"
)

type ResponseCodeExecutionOutputBlock struct {
	FileId string `json:"file_id"`
	Type   string `json:"type"`
}

type TextEditorCodeExecutionToolResultErrorCode string

const (
	TextEditorCodeExecutionToolResultErrorCodeInvalidToolInput      TextEditorCodeExecutionToolResultErrorCode = "invalid_tool_input"
	TextEditorCodeExecutionToolResultErrorCodeUnavailable           TextEditorCodeExecutionToolResultErrorCode = "unavailable"
	TextEditorCodeExecutionToolResultErrorCodeTooManyRequests       TextEditorCodeExecutionToolResultErrorCode = "too_many_requests"
	TextEditorCodeExecutionToolResultErrorCodeExecutionTimeExceeded TextEditorCodeExecutionToolResultErrorCode = "execution_time_exceeded"
	TextEditorCodeExecutionToolResultErrorCodeFileNotFound          TextEditorCodeExecutionToolResultErrorCode = "file_not_found"
)

type ToolSearchToolResultErrorCode string

const (
	ToolSearchToolResultErrorCodeInvalidToolInput      ToolSearchToolResultErrorCode = "invalid_tool_input"
	ToolSearchToolResultErrorCodeUnavailable           ToolSearchToolResultErrorCode = "unavailable"
	ToolSearchToolResultErrorCodeTooManyRequests       ToolSearchToolResultErrorCode = "too_many_requests"
	ToolSearchToolResultErrorCodeExecutionTimeExceeded ToolSearchToolResultErrorCode = "execution_time_exceeded"
)

type ResponseToolReferenceBlock struct {
	ToolName string `json:"tool_name"`
	Type     string `json:"type"`
}

type WebFetchToolResultErrorCode string

const (
	WebFetchToolResultErrorCodeInvalidToolInput       WebFetchToolResultErrorCode = "invalid_tool_input"
	WebFetchToolResultErrorCodeUrlTooLong             WebFetchToolResultErrorCode = "url_too_long"
	WebFetchToolResultErrorCodeUrlNotAllowed          WebFetchToolResultErrorCode = "url_not_allowed"
	WebFetchToolResultErrorCodeUrlNotAccessible       WebFetchToolResultErrorCode = "url_not_accessible"
	WebFetchToolResultErrorCodeUnsupportedContentType WebFetchToolResultErrorCode = "unsupported_content_type"
	WebFetchToolResultErrorCodeTooManyRequests        WebFetchToolResultErrorCode = "too_many_requests"
	WebFetchToolResultErrorCodeMaxUsesExceeded        WebFetchToolResultErrorCode = "max_uses_exceeded"
	WebFetchToolResultErrorCodeUnavailable            WebFetchToolResultErrorCode = "unavailable"
)

type ResponseDocumentBlock struct {
	// Citation configuration for the document
	Citations *ResponseCitationsConfig    `json:"citations"`
	Source    ResponseDocumentBlockSource `json:"source"`
	// The title of the document
	Title *string `json:"title"`
	Type  string  `json:"type"`
}

type ResponseDocumentBlockSource struct {
	Base64 *Base64PDFSource
	Text   *PlainTextSource
}

func (u *ResponseDocumentBlockSource) MarshalJSON() ([]byte, error) {
	if u.Base64 != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*Base64PDFSource
		}{
			Type:            "base64",
			Base64PDFSource: u.Base64,
		})
	}
	if u.Text != nil {
		return json.Marshal(struct {
			Type string `json:"type"`
			*PlainTextSource
		}{
			Type:            "text",
			PlainTextSource: u.Text,
		})
	}
	return nil, errors.New("invalid ResponseDocumentBlockSource: all variants are nil")
}

func (u *ResponseDocumentBlockSource) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawType, ok := raw["type"]
	if !ok {
		return errors.New("missing type field in ResponseDocumentBlockSource")
	}
	var discriminator string
	if err := json.Unmarshal(rawType, &discriminator); err != nil {
		return err
	}
	*u = ResponseDocumentBlockSource{}
	switch discriminator {
	case "base64":
		var value Base64PDFSource
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Base64 = &value
	case "text":
		var value PlainTextSource
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		u.Text = &value
	default:
		return fmt.Errorf("invalid type field in ResponseDocumentBlockSource: %q", discriminator)
	}
	return nil
}

type WebSearchToolResultErrorCode string

const (
	WebSearchToolResultErrorCodeInvalidToolInput WebSearchToolResultErrorCode = "invalid_tool_input"
	WebSearchToolResultErrorCodeUnavailable      WebSearchToolResultErrorCode = "unavailable"
	WebSearchToolResultErrorCodeMaxUsesExceeded  WebSearchToolResultErrorCode = "max_uses_exceeded"
	WebSearchToolResultErrorCodeTooManyRequests  WebSearchToolResultErrorCode = "too_many_requests"
	WebSearchToolResultErrorCodeQueryTooLong     WebSearchToolResultErrorCode = "query_too_long"
	WebSearchToolResultErrorCodeRequestTooLarge  WebSearchToolResultErrorCode = "request_too_large"
)

type RequestBashCodeExecutionOutputBlock struct {
	FileId string `json:"file_id"`
	Type   string `json:"type"`
}

type RequestCodeExecutionOutputBlock struct {
	FileId string `json:"file_id"`
	Type   string `json:"type"`
}

type ResponseCitationsConfig struct {
	Enabled bool `json:"enabled"`
}
