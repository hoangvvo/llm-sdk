package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/anthropic/anthropicapi"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/clientutils"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/toolresultutils"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/tracing"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/partutil"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/stream"
)

const (
	Provider          = "anthropic"
	DefaultBaseURL    = "https://api.anthropic.com"
	DefaultAPIVersion = "2023-06-01"
)

type AnthropicModelOptions struct {
	BaseURL    string
	APIKey     string
	APIVersion string
	Headers    map[string]string
	HTTPClient *http.Client
}

type AnthropicModel struct {
	modelID    string
	apiKey     string
	baseURL    string
	apiVersion string
	client     *http.Client
	metadata   *llmsdk.LanguageModelMetadata
	headers    map[string]string
}

func NewAnthropicModel(modelID string, options AnthropicModelOptions) *AnthropicModel {
	baseURL := options.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	apiVersion := options.APIVersion
	if apiVersion == "" {
		apiVersion = DefaultAPIVersion
	}

	client := options.HTTPClient
	if client == nil {
		client = &http.Client{}
	}

	headers := map[string]string{}
	for k, v := range options.Headers {
		headers[k] = v
	}

	return &AnthropicModel{
		modelID:    modelID,
		apiKey:     options.APIKey,
		baseURL:    baseURL,
		apiVersion: apiVersion,
		client:     client,
		headers:    headers,
	}
}

func (m *AnthropicModel) WithMetadata(metadata *llmsdk.LanguageModelMetadata) *AnthropicModel {
	m.metadata = metadata
	return m
}

func (m *AnthropicModel) Provider() string {
	return Provider
}

func (m *AnthropicModel) ModelID() string {
	return m.modelID
}

func (m *AnthropicModel) Metadata() *llmsdk.LanguageModelMetadata {
	return m.metadata
}

func (m *AnthropicModel) Generate(ctx context.Context, input *llmsdk.LanguageModelInput) (*llmsdk.ModelResponse, error) {
	return tracing.TraceGenerate(ctx, Provider, m.modelID, input, func(ctx context.Context) (*llmsdk.ModelResponse, error) {
		params, err := convertToAnthropicCreateParams(input, m.modelID, false)
		if err != nil {
			return nil, err
		}

		response, err := clientutils.DoJSON[anthropicapi.Message](ctx, m.client, clientutils.JSONRequestConfig{
			URL:     fmt.Sprintf("%s/v1/messages", m.baseURL),
			Body:    params,
			Headers: m.requestHeaders(),
		})
		if err != nil {
			return nil, err
		}
		if response.StopReason != nil && *response.StopReason == anthropicapi.StopReasonRefusal {
			return nil, llmsdk.NewRefusalError(anthropicRefusalMessage(response.StopDetails))
		}

		content, err := mapAnthropicMessage(response.Content)
		if err != nil {
			return nil, err
		}

		usage := mapAnthropicUsage(anthropicUsageFromMessage(response.Usage))

		result := &llmsdk.ModelResponse{
			Content: content,
			Usage:   usage,
		}

		if m.metadata != nil && m.metadata.Pricing != nil {
			cost := usage.CalculateCost(m.metadata.Pricing, llmsdk.ModelUsageCostOptions{InputCacheTokensAreAdditional: true, OutputReasoningTokensAreAdditional: false})
			result.Cost = &cost
		}

		return result, nil
	})
}

func (m *AnthropicModel) Stream(ctx context.Context, input *llmsdk.LanguageModelInput) (*llmsdk.LanguageModelStream, error) {
	return tracing.TraceStream(ctx, Provider, m.modelID, input, func(ctx context.Context) (*llmsdk.LanguageModelStream, error) {
		params, err := convertToAnthropicCreateParams(input, m.modelID, true)
		if err != nil {
			return nil, err
		}

		sseStream, err := clientutils.DoSSE[anthropicapi.MessageStreamEvent](ctx, m.client, clientutils.SSERequestConfig{
			URL:     fmt.Sprintf("%s/v1/messages", m.baseURL),
			Body:    params,
			Headers: m.requestHeaders(),
		})
		if err != nil {
			return nil, err
		}

		responseCh := make(chan *llmsdk.PartialModelResponse)
		errCh := make(chan error, 1)

		go func() {
			defer close(responseCh)
			defer close(errCh)
			defer sseStream.Close()

			type serverToolBlock struct {
				id    string
				input string
			}
			providerToolBlockIndexes := map[int]bool{}
			serverToolBlocks := map[int]*serverToolBlock{}
			serverToolCallIndexes := map[string]int{}
			var streamUsage *anthropicUsage
			for sseStream.Next() {
				event, err := sseStream.Current()
				if err != nil {
					errCh <- fmt.Errorf("failed to get sse event: %w", err)
					return
				}
				if event == nil {
					continue
				}

				if event.MessageStart != nil {
					usage := anthropicUsageFromMessage(event.MessageStart.Message.Usage)
					streamUsage = &usage
					if event.MessageStart.Message.StopReason != nil && *event.MessageStart.Message.StopReason == anthropicapi.StopReasonRefusal {
						errCh <- llmsdk.NewRefusalError(anthropicRefusalMessage(event.MessageStart.Message.StopDetails))
						return
					}
					continue
				}

				if event.MessageDelta != nil {
					if streamUsage != nil {
						mergeAnthropicMessageDeltaUsage(streamUsage, event.MessageDelta.Usage)
					}
					if event.MessageDelta.Delta.StopReason != nil && *event.MessageDelta.Delta.StopReason == anthropicapi.StopReasonRefusal {
						errCh <- llmsdk.NewRefusalError(anthropicRefusalMessage(event.MessageDelta.Delta.StopDetails))
						return
					}
					continue
				}

				if event.ContentBlockStart != nil {
					block := event.ContentBlockStart.ContentBlock
					if block.ServerToolUse != nil {
						providerToolBlockIndexes[event.ContentBlockStart.Index] = true
						if block.ServerToolUse.Name == anthropicapi.ResponseServerToolUseBlockNameWebSearch {
							serverToolBlocks[event.ContentBlockStart.Index] = &serverToolBlock{id: block.ServerToolUse.Id}
							serverToolCallIndexes[block.ServerToolUse.Id] = event.ContentBlockStart.Index
						}
					}
					if block.WebSearchToolResult != nil {
						if callIndex, ok := serverToolCallIndexes[block.WebSearchToolResult.ToolUseId]; ok {
							status := anthropicWebSearchResultStatus(block.WebSearchToolResult)
							id := block.WebSearchToolResult.ToolUseId
							responseCh <- &llmsdk.PartialModelResponse{Delta: &llmsdk.ContentDelta{Index: callIndex, Part: llmsdk.PartDelta{ToolCallPartDelta: &llmsdk.ToolCallPartDelta{ToolCallID: &id, Call: llmsdk.ToolCallDelta{WebSearch: &llmsdk.WebSearchToolCallDelta{Status: &status}}}}}}
						}
					}
					deltas, err := mapAnthropicRawContentBlockStartEvent(*event.ContentBlockStart)
					if err != nil {
						errCh <- fmt.Errorf("failed to map content block start: %w", err)
						return
					}
					for _, delta := range deltas {
						d := delta
						responseCh <- &llmsdk.PartialModelResponse{Delta: &d}
					}
					continue
				}

				if event.ContentBlockDelta != nil {
					if block := serverToolBlocks[event.ContentBlockDelta.Index]; block != nil && event.ContentBlockDelta.Delta.InputJsonDelta != nil {
						block.input += event.ContentBlockDelta.Delta.InputJsonDelta.PartialJson
						continue
					}
					if providerToolBlockIndexes[event.ContentBlockDelta.Index] {
						continue
					}
					deltas, err := mapAnthropicRawContentBlockDeltaEvent(*event.ContentBlockDelta)
					if err != nil {
						errCh <- fmt.Errorf("failed to map content block delta: %w", err)
						return
					}
					for _, delta := range deltas {
						d := delta
						responseCh <- &llmsdk.PartialModelResponse{Delta: &d}
					}
					continue
				}

				if event.ContentBlockStop != nil {
					block := serverToolBlocks[event.ContentBlockStop.Index]
					if block == nil {
						continue
					}
					delete(serverToolBlocks, event.ContentBlockStop.Index)
					var input struct {
						Query string `json:"query"`
					}
					if json.Unmarshal([]byte(block.input), &input) == nil && input.Query != "" {
						action := &llmsdk.WebSearchAction{Type: "search", Queries: []string{input.Query}}
						id := block.id
						responseCh <- &llmsdk.PartialModelResponse{Delta: &llmsdk.ContentDelta{Index: event.ContentBlockStop.Index, Part: llmsdk.PartDelta{ToolCallPartDelta: &llmsdk.ToolCallPartDelta{ToolCallID: &id, Call: llmsdk.ToolCallDelta{WebSearch: &llmsdk.WebSearchToolCallDelta{Action: action}}}}}}
					}
					continue
				}
			}

			if err := sseStream.Err(); err != nil {
				errCh <- fmt.Errorf("scanner error: %w", err)
				return
			}
			if streamUsage != nil {
				partial := &llmsdk.PartialModelResponse{Usage: mapAnthropicUsage(*streamUsage)}
				if m.metadata != nil && m.metadata.Pricing != nil {
					cost := partial.Usage.CalculateCost(m.metadata.Pricing, llmsdk.ModelUsageCostOptions{InputCacheTokensAreAdditional: true, OutputReasoningTokensAreAdditional: false})
					partial.Cost = &cost
				}
				responseCh <- partial
			}
		}()

		return stream.New(responseCh, errCh), nil
	})
}

func anthropicRefusalMessage(details *anthropicapi.RefusalStopDetails) string {
	if details != nil {
		if details.Explanation != nil {
			return *details.Explanation
		}
		if details.Category != nil {
			return fmt.Sprintf("Anthropic policy category: %s", *details.Category)
		}
	}
	return "Anthropic refused the request"
}

func (m *AnthropicModel) requestHeaders() map[string]string {
	headers := map[string]string{
		"x-api-key":         m.apiKey,
		"anthropic-version": m.apiVersion,
	}

	for k, v := range m.headers {
		headers[k] = v
	}

	return headers
}

func convertToAnthropicCreateParams(input *llmsdk.LanguageModelInput, modelID string, stream bool) (*anthropicapi.CreateMessageParams, error) {
	maxTokens := 4096
	if input.MaxTokens != nil {
		maxTokens = int(*input.MaxTokens)
	}

	messages, err := convertToAnthropicMessages(input.Messages)
	if err != nil {
		return nil, err
	}

	params := &anthropicapi.CreateMessageParams{
		Messages:    messages,
		MaxTokens:   maxTokens,
		Stream:      ptr.To(stream),
		Temperature: input.Temperature,
		TopP:        input.TopP,
	}
	model := anthropicapi.Model(ptr.To(modelID))
	params.Model = model

	if input.SystemPrompt != nil {
		systemPrompt := anthropicapi.CreateMessageParamsSystemString(input.SystemPrompt)
		params.System = &anthropicapi.CreateMessageParamsSystem{
			CreateMessageParamsSystemString: &systemPrompt,
		}
	}

	if input.TopK != nil {
		topK := int(*input.TopK)
		params.TopK = &topK
	}

	if input.ToolChoice != nil {
		params.ToolChoice = convertToAnthropicToolChoice(*input.ToolChoice)
	}

	if input.ResponseFormat != nil {
		params.OutputConfig = convertToAnthropicOutputConfig(*input.ResponseFormat)
	}

	if len(input.Tools) > 0 {
		tools := make([]anthropicapi.CreateMessageParamsToolsItem, 0, len(input.Tools))
		for _, tool := range input.Tools {
			if tool.WebSearchTool != nil {
				// The basic version supports both common options without enabling
				// Anthropic's newer code-execution filtering flow.
				webSearch := tool.WebSearchTool
				anthropicWebSearch := &anthropicapi.WebSearchTool20250305{
					Name: "web_search", Type: "web_search_20250305",
					AllowedDomains: webSearch.AllowedDomains,
					MaxUses:        webSearch.MaxUses,
				}
				if webSearch.UserLocation != nil {
					anthropicWebSearch.UserLocation = &anthropicapi.UserLocation{
						City: webSearch.UserLocation.City, Country: webSearch.UserLocation.Country,
						Region: webSearch.UserLocation.Region, Timezone: webSearch.UserLocation.Timezone,
						Type: "approximate",
					}
				}
				tools = append(tools, anthropicapi.CreateMessageParamsToolsItem{WebSearchTool20250305: anthropicWebSearch})
				continue
			}
			if tool.FunctionTool == nil {
				continue
			}
			functionTool := tool.FunctionTool
			strict := true
			anthropicTool := anthropicapi.Tool{
				Name:        functionTool.Name,
				InputSchema: anthropicapi.InputSchema(functionTool.Parameters),
				Strict:      &strict,
			}
			if functionTool.Description != "" {
				anthropicTool.Description = ptr.To(functionTool.Description)
			}
			tools = append(tools, anthropicapi.CreateMessageParamsToolsItem{
				Tool: &anthropicTool,
			})
		}
		params.Tools = tools
	}

	if input.Reasoning != nil {
		params.Thinking = convertToAnthropicThinkingConfigParam(*input.Reasoning)
	}

	if input.CacheRetention != nil {
		// Top-level cache_control caches through the last cacheable block.
		ephemeral := &anthropicapi.CacheControlEphemeral{}
		if *input.CacheRetention == llmsdk.CacheRetentionExtended {
			ephemeral.Ttl = ptr.To(anthropicapi.CacheControlEphemeralTtlN1H)
		}
		params.CacheControl = &anthropicapi.CreateMessageParamsCacheControl{Ephemeral: ephemeral}
	}

	return params, nil
}

func convertToAnthropicOutputConfig(option llmsdk.ResponseFormatOption) *anthropicapi.OutputConfig {
	switch {
	case option.Text != nil:
		return nil
	case option.JSON != nil && option.JSON.Schema != nil:
		return &anthropicapi.OutputConfig{
			Format: &anthropicapi.JsonOutputFormat{
				Type:   "json_schema",
				Schema: option.JSON.Schema,
			},
		}
	default:
		return nil
	}
}

func convertToAnthropicMessages(messages []llmsdk.Message) ([]anthropicapi.InputMessage, error) {
	result := make([]anthropicapi.InputMessage, 0, len(messages))

	for _, message := range messages {
		var parts []llmsdk.Part
		var role anthropicapi.InputMessageRole

		switch {
		case message.UserMessage != nil:
			parts = message.UserMessage.Content
			role = anthropicapi.InputMessageRoleUser
		case message.AssistantMessage != nil:
			parts = message.AssistantMessage.Content
			role = anthropicapi.InputMessageRoleAssistant
		case message.ToolMessage != nil:
			parts = message.ToolMessage.Content
			role = anthropicapi.InputMessageRoleUser
		default:
			continue
		}

		contentBlocks, err := convertPartsToAnthropicContentBlocks(parts)
		if err != nil {
			return nil, err
		}

		result = append(result, anthropicapi.InputMessage{
			Role: role,
			Content: anthropicapi.InputMessageContent{
				InputMessageContentArray: (*anthropicapi.InputMessageContentArray)(&contentBlocks),
			},
		})
	}

	return result, nil
}

func convertPartsToAnthropicContentBlocks(parts []llmsdk.Part) ([]anthropicapi.InputContentBlock, error) {
	blocks := make([]anthropicapi.InputContentBlock, 0, len(parts))

	for _, part := range parts {
		block, err := convertPartToAnthropicContentBlock(part)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}

	return blocks, nil
}

func convertPartToAnthropicContentBlock(part llmsdk.Part) (anthropicapi.InputContentBlock, error) {
	switch {
	case part.TextPart != nil:
		citations := make([]anthropicapi.RequestTextBlockCitationsItem, 0, len(part.TextPart.Citations))
		for _, citation := range part.TextPart.Citations {
			if citation.Signature == nil {
				continue
			}
			citedText := ""
			if citation.CitedText != nil {
				citedText = *citation.CitedText
			}
			citations = append(citations, anthropicapi.RequestTextBlockCitationsItem{
				WebSearchResultLocation: &anthropicapi.RequestWebSearchResultLocationCitation{
					CitedText: citedText, EncryptedIndex: *citation.Signature,
					Title: citation.Title, Url: citation.Source,
				},
			})
		}
		return anthropicapi.InputContentBlock{
			Text: &anthropicapi.RequestTextBlock{
				Type: "text", Text: part.TextPart.Text,
				// EncryptedIndex is the provider state Anthropic accepts when a
				// web-search citation is returned in a later assistant message.
				Citations: citations,
			},
		}, nil

	case part.ImagePart != nil:
		return anthropicapi.InputContentBlock{
			Image: &anthropicapi.RequestImageBlock{
				Source: anthropicapi.RequestImageBlockSource{
					Base64: &anthropicapi.Base64ImageSource{
						Data:      part.ImagePart.Data,
						MediaType: anthropicapi.Base64ImageSourceMediaType(part.ImagePart.MimeType),
					},
				},
			},
		}, nil

	case part.SourcePart != nil:
		textBlocks := make([]anthropicapi.RequestTextBlock, 0, len(part.SourcePart.Content))
		for _, subPart := range part.SourcePart.Content {
			if subPart.TextPart == nil {
				return anthropicapi.InputContentBlock{}, llmsdk.NewUnsupportedError(Provider, fmt.Sprintf("cannot convert source part content to anthropic search result for type %s", subPart.Type()))
			}
			textBlocks = append(textBlocks, anthropicapi.RequestTextBlock{Text: subPart.TextPart.Text, Type: "text"})
		}
		return anthropicapi.InputContentBlock{
			SearchResult: &anthropicapi.RequestSearchResultBlock{
				Source:  part.SourcePart.Source,
				Title:   part.SourcePart.Title,
				Content: textBlocks,
				Citations: &anthropicapi.RequestCitationsConfig{
					Enabled: ptr.To(true),
				},
			},
		}, nil

	case part.ToolCallPart != nil:
		if part.ToolCallPart.Call.WebSearch != nil {
			input := map[string]any{}
			if action := part.ToolCallPart.Call.WebSearch.Action; action != nil && action.Type == "search" && len(action.Queries) > 0 {
				input["query"] = action.Queries[0]
			}
			return anthropicapi.InputContentBlock{ServerToolUse: &anthropicapi.RequestServerToolUseBlock{
				Id: part.ToolCallPart.ToolCallID, Name: anthropicapi.RequestServerToolUseBlockNameWebSearch, Input: input,
			}}, nil
		}
		call := part.ToolCallPart.Call.Function
		if call == nil {
			return anthropicapi.InputContentBlock{}, llmsdk.NewUnsupportedError(Provider, "tool call has no supported payload")
		}
		var inputMap map[string]any
		if len(call.Args) > 0 {
			if err := json.Unmarshal(call.Args, &inputMap); err != nil {
				return anthropicapi.InputContentBlock{}, fmt.Errorf("failed to unmarshal tool call args: %w", err)
			}
		}
		if inputMap == nil {
			inputMap = map[string]any{}
		}
		return anthropicapi.InputContentBlock{
			ToolUse: &anthropicapi.RequestToolUseBlock{
				Id:    part.ToolCallPart.ToolCallID,
				Name:  call.Name,
				Input: inputMap,
			},
		}, nil

	case part.ToolResultPart != nil:
		if part.ToolResultPart.Result.WebSearch != nil {
			result := part.ToolResultPart.Result.WebSearch
			content := anthropicapi.RequestWebSearchToolResultBlockContent{}
			if result.ErrorCode != nil {
				content.RequestWebSearchToolResultError = &anthropicapi.RequestWebSearchToolResultError{Type: "web_search_tool_result_error", ErrorCode: anthropicapi.WebSearchToolResultErrorCode(*result.ErrorCode)}
			} else {
				items := make(anthropicapi.RequestWebSearchToolResultBlockContentArray, 0, len(result.Sources))
				for _, source := range result.Sources {
					title, signature := "", ""
					if source.Title != nil {
						title = *source.Title
					}
					if source.Signature != nil {
						signature = *source.Signature
					}
					items = append(items, anthropicapi.RequestWebSearchResultBlock{Type: "web_search_result", Url: source.URL, Title: title, PageAge: source.PageAge, EncryptedContent: signature})
				}
				content.RequestWebSearchToolResultBlockContentArray = &items
			}
			return anthropicapi.InputContentBlock{WebSearchToolResult: &anthropicapi.RequestWebSearchToolResultBlock{ToolUseId: part.ToolResultPart.ToolCallID, Content: content}}, nil
		}
		functionResult := part.ToolResultPart.Result.Function
		if functionResult == nil {
			return anthropicapi.InputContentBlock{}, llmsdk.NewUnsupportedError(Provider, "tool result has no supported payload")
		}
		contentBlocks := make([]anthropicapi.InputContentBlock, 0, len(functionResult.Content))
		for _, subPart := range functionResult.Content {
			block, err := convertPartToAnthropicContentBlock(subPart)
			if err != nil {
				return anthropicapi.InputContentBlock{}, err
			}
			if block.Text == nil && block.Image == nil && block.SearchResult == nil {
				return anthropicapi.InputContentBlock{}, llmsdk.NewUnsupportedError(Provider, fmt.Sprintf("cannot convert tool result part to anthropic content for type %s", subPart.Type()))
			}
			contentBlocks = append(contentBlocks, block)
		}
		content := make(anthropicapi.RequestToolResultBlockContentArray, 0, len(contentBlocks))
		for _, block := range contentBlocks {
			content = append(content, anthropicapi.RequestToolResultBlockContentArrayItem{
				Text:         block.Text,
				Image:        block.Image,
				SearchResult: block.SearchResult,
			})
		}
		toolResultContent := &anthropicapi.RequestToolResultBlockContent{
			RequestToolResultBlockContentArray: &content,
		}
		if len(contentBlocks) == 0 && part.ToolResultPart.Status == llmsdk.ToolResultStatusCancelled {
			emptyContent := toolresultutils.CancelledFallbackContent
			contentString := anthropicapi.RequestToolResultBlockContentString(&emptyContent)
			toolResultContent = &anthropicapi.RequestToolResultBlockContent{
				RequestToolResultBlockContentString: &contentString,
			}
		}
		toolResult := anthropicapi.RequestToolResultBlock{
			ToolUseId: part.ToolResultPart.ToolCallID,
			Content:   toolResultContent,
			IsError:   ptr.To(part.ToolResultPart.Status != llmsdk.ToolResultStatusCompleted),
		}
		return anthropicapi.InputContentBlock{
			ToolResult: &toolResult,
		}, nil

	case part.ReasoningPart != nil:
		if part.ReasoningPart.Text == "" && part.ReasoningPart.Signature != nil {
			return anthropicapi.InputContentBlock{
				RedactedThinking: &anthropicapi.RequestRedactedThinkingBlock{
					Data: *part.ReasoningPart.Signature,
				},
			}, nil
		}
		block := anthropicapi.RequestThinkingBlock{Thinking: part.ReasoningPart.Text}
		if part.ReasoningPart.Signature != nil {
			block.Signature = *part.ReasoningPart.Signature
		}
		return anthropicapi.InputContentBlock{
			Thinking: &block,
		}, nil
	}

	return anthropicapi.InputContentBlock{}, llmsdk.NewUnsupportedError(Provider, fmt.Sprintf("cannot convert part to anthropic content for type %s", part.Type()))
}

func convertToAnthropicToolChoice(option llmsdk.ToolChoiceOption) *anthropicapi.ToolChoice {
	if option.Auto != nil {
		return &anthropicapi.ToolChoice{Auto: &anthropicapi.ToolChoiceAuto{}}
	}
	if option.None != nil {
		return &anthropicapi.ToolChoice{None: &anthropicapi.ToolChoiceNone{}}
	}
	if option.Required != nil {
		return &anthropicapi.ToolChoice{Any: &anthropicapi.ToolChoiceAny{}}
	}
	if option.Tool != nil {
		return &anthropicapi.ToolChoice{Tool: &anthropicapi.ToolChoiceTool{Name: option.Tool.ToolName}}
	}
	return nil
}

func convertToAnthropicThinkingConfigParam(reasoning llmsdk.ReasoningOptions) *anthropicapi.ThinkingConfigParam {
	if !reasoning.Enabled {
		return &anthropicapi.ThinkingConfigParam{Disabled: &anthropicapi.ThinkingConfigDisabled{}}
	}

	// Without an explicit token budget, let Anthropic choose the thinking depth.
	if reasoning.BudgetTokens == nil {
		return &anthropicapi.ThinkingConfigParam{Adaptive: &anthropicapi.ThinkingConfigAdaptive{}}
	}

	return &anthropicapi.ThinkingConfigParam{
		Enabled: &anthropicapi.ThinkingConfigEnabled{
			BudgetTokens: int(*reasoning.BudgetTokens),
		},
	}
}

func mapAnthropicMessage(content []anthropicapi.ContentBlock) ([]llmsdk.Part, error) {
	parts := make([]llmsdk.Part, 0, len(content))
	callStatuses := map[string]llmsdk.WebSearchToolCallStatus{}
	for _, block := range content {
		if block.WebSearchToolResult != nil {
			callStatuses[block.WebSearchToolResult.ToolUseId] = anthropicWebSearchResultStatus(block.WebSearchToolResult)
		}
	}

	for _, block := range content {
		part, err := mapAnthropicContentBlock(block)
		if err != nil {
			return nil, err
		}
		if part != nil {
			if part.ToolCallPart != nil && part.ToolCallPart.Call.WebSearch != nil {
				status, ok := callStatuses[part.ToolCallPart.ToolCallID]
				if ok {
					part.ToolCallPart.Call.WebSearch.Status = &status
				}
			}
			parts = append(parts, *part)
		}
	}

	return parts, nil
}

func anthropicWebSearchResultStatus(block *anthropicapi.ResponseWebSearchToolResultBlock) llmsdk.WebSearchToolCallStatus {
	if block.Content.ResponseWebSearchToolResultError != nil {
		return llmsdk.WebSearchToolCallStatusFailed
	}
	return llmsdk.WebSearchToolCallStatusCompleted
}

func mapAnthropicContentBlock(block anthropicapi.ContentBlock) (*llmsdk.Part, error) {
	switch {
	case block.Text != nil:
		citations, err := mapAnthropicTextCitations(block.Text.Citations)
		if err != nil {
			return nil, err
		}
		opts := []llmsdk.TextPartOption{}
		if len(citations) > 0 {
			opts = append(opts, llmsdk.WithTextCitations(citations))
		}
		part := llmsdk.NewTextPart(block.Text.Text, opts...)
		return &part, nil

	case block.ToolUse != nil:
		args, err := json.Marshal(block.ToolUse.Input)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tool use input: %w", err)
		}
		part := llmsdk.NewToolCallPart(block.ToolUse.Id, block.ToolUse.Name, json.RawMessage(args))
		return &part, nil

	case block.ServerToolUse != nil:
		if block.ServerToolUse.Name != anthropicapi.ResponseServerToolUseBlockNameWebSearch {
			return nil, nil
		}
		status := llmsdk.WebSearchToolCallStatusInProgress
		webCall := &llmsdk.WebSearchToolCall{Status: &status}
		if input, ok := block.ServerToolUse.Input.(map[string]any); ok {
			if query, ok := input["query"].(string); ok {
				webCall.Action = &llmsdk.WebSearchAction{Type: "search", Queries: []string{query}}
			}
		}
		part := llmsdk.Part{ToolCallPart: &llmsdk.ToolCallPart{ToolCallID: block.ServerToolUse.Id, Call: llmsdk.ToolCall{WebSearch: webCall}}}
		return &part, nil

	case block.WebSearchToolResult != nil:
		result := &llmsdk.WebSearchToolResult{Sources: []llmsdk.WebSearchSource{}}
		status := llmsdk.ToolResultStatusCompleted
		if values := block.WebSearchToolResult.Content.ResponseWebSearchToolResultBlockContentArray; values != nil {
			for _, source := range *values {
				result.Sources = append(result.Sources, llmsdk.WebSearchSource{URL: source.Url, Title: ptr.To(source.Title), PageAge: source.PageAge, Signature: ptr.To(source.EncryptedContent)})
			}
		} else if value := block.WebSearchToolResult.Content.ResponseWebSearchToolResultError; value != nil {
			code := string(value.ErrorCode)
			result.ErrorCode = &code
			status = llmsdk.ToolResultStatusFailed
		}
		part := llmsdk.Part{ToolResultPart: &llmsdk.ToolResultPart{ToolCallID: block.WebSearchToolResult.ToolUseId, Result: llmsdk.ToolResult{WebSearch: result}, Status: status}}
		return &part, nil

	case block.Thinking != nil:
		opts := []llmsdk.ReasoningPartOption{}
		if block.Thinking.Signature != "" {
			opts = append(opts, llmsdk.WithReasoningSignature(block.Thinking.Signature))
		}
		part := llmsdk.NewReasoningPart(block.Thinking.Thinking, opts...)
		return &part, nil

	case block.RedactedThinking != nil:
		part := llmsdk.NewReasoningPart("", llmsdk.WithReasoningSignature(block.RedactedThinking.Data))
		return &part, nil
	}

	return nil, nil
}

func mapAnthropicTextCitations(raw []anthropicapi.ResponseTextBlockCitationsItem) ([]llmsdk.Citation, error) {
	citations := make([]llmsdk.Citation, 0, len(raw))

	for _, item := range raw {
		if item.WebSearchResultLocation != nil {
			web := item.WebSearchResultLocation
			if web.Url != "" {
				citation := llmsdk.Citation{Source: web.Url, Title: web.Title, Signature: ptr.To(web.EncryptedIndex)}
				if web.CitedText != "" {
					citation.CitedText = ptr.To(web.CitedText)
				}
				citations = append(citations, citation)
			}
			continue
		}
		if item.SearchResultLocation == nil {
			continue
		}
		source := item.SearchResultLocation.Source
		if source == "" {
			continue
		}

		citation := llmsdk.Citation{
			Source:     source,
			StartIndex: ptr.To(item.SearchResultLocation.StartBlockIndex),
			EndIndex:   ptr.To(item.SearchResultLocation.EndBlockIndex),
		}
		if item.SearchResultLocation.CitedText != "" {
			citation.CitedText = ptr.To(item.SearchResultLocation.CitedText)
		}
		if item.SearchResultLocation.Title != nil && *item.SearchResultLocation.Title != "" {
			citation.Title = item.SearchResultLocation.Title
		}
		citations = append(citations, citation)
	}

	return citations, nil
}

func mapAnthropicRawContentBlockStartEvent(event anthropicapi.ContentBlockStartEvent) ([]llmsdk.ContentDelta, error) {
	part, err := mapAnthropicContentBlock(anthropicapi.ContentBlock{
		Text:                event.ContentBlock.Text,
		Thinking:            event.ContentBlock.Thinking,
		RedactedThinking:    event.ContentBlock.RedactedThinking,
		ToolUse:             event.ContentBlock.ToolUse,
		ServerToolUse:       event.ContentBlock.ServerToolUse,
		WebSearchToolResult: event.ContentBlock.WebSearchToolResult,
	})
	if err != nil {
		return nil, err
	}
	if part == nil {
		return nil, nil
	}

	delta := partutil.LooselyConvertPartToPartDelta(*part)
	if delta.ToolCallPartDelta != nil {
		empty := ""
		if delta.ToolCallPartDelta.Call.Function != nil {
			delta.ToolCallPartDelta.Call.Function.Args = &empty
		}
	}

	return []llmsdk.ContentDelta{
		{Index: event.Index, Part: delta},
	}, nil
}

func mapAnthropicRawContentBlockDeltaEvent(event anthropicapi.ContentBlockDeltaEvent) ([]llmsdk.ContentDelta, error) {
	partDelta, err := mapAnthropicRawContentBlockDelta(event.Delta)
	if err != nil || partDelta == nil {
		return nil, err
	}

	return []llmsdk.ContentDelta{{Index: event.Index, Part: *partDelta}}, nil
}

func mapAnthropicRawContentBlockDelta(raw anthropicapi.ContentBlockDeltaEventDelta) (*llmsdk.PartDelta, error) {
	switch {
	case raw.TextDelta != nil:
		part := llmsdk.NewTextPartDelta(raw.TextDelta.Text)
		return &part, nil

	case raw.InputJsonDelta != nil:
		part := llmsdk.NewToolCallPartDelta(llmsdk.WithToolCallPartDeltaArgs(raw.InputJsonDelta.PartialJson))
		return &part, nil

	case raw.ThinkingDelta != nil:
		part := llmsdk.NewReasoningPartDelta(raw.ThinkingDelta.Thinking)
		return &part, nil

	case raw.SignatureDelta != nil:
		part := llmsdk.NewReasoningPartDelta("", llmsdk.WithReasoningPartDeltaSignature(raw.SignatureDelta.Signature))
		return &part, nil

	case raw.CitationsDelta != nil:
		citationDelta, err := mapAnthropicCitationDelta(raw.CitationsDelta.Citation)
		if err != nil || citationDelta == nil {
			return nil, err
		}
		part := llmsdk.NewTextPartDelta("", llmsdk.WithTextPartDeltaCitation(citationDelta))
		return &part, nil
	}

	return nil, nil
}

func mapAnthropicCitationDelta(raw anthropicapi.CitationsDeltaCitation) (*llmsdk.CitationDelta, error) {
	citation := &llmsdk.CitationDelta{}
	if raw.WebSearchResultLocation != nil {
		web := raw.WebSearchResultLocation
		if web.Url != "" {
			citation.Source = ptr.To(web.Url)
		}
		if web.Title != nil && *web.Title != "" {
			citation.Title = web.Title
		}
		if web.CitedText != "" {
			citation.CitedText = ptr.To(web.CitedText)
		}
		citation.Signature = ptr.To(web.EncryptedIndex)
		return citation, nil
	}
	if raw.SearchResultLocation != nil {
		if raw.SearchResultLocation.Source != "" {
			citation.Source = ptr.To(raw.SearchResultLocation.Source)
		}
		if raw.SearchResultLocation.Title != nil && *raw.SearchResultLocation.Title != "" {
			citation.Title = raw.SearchResultLocation.Title
		}
		if raw.SearchResultLocation.CitedText != "" {
			citation.CitedText = ptr.To(raw.SearchResultLocation.CitedText)
		}
		citation.StartIndex = ptr.To(raw.SearchResultLocation.StartBlockIndex)
		citation.EndIndex = ptr.To(raw.SearchResultLocation.EndBlockIndex)
		return citation, nil
	}

	// Unknown citation kinds carry no source, so drop them instead of emitting
	// an empty citation that the accumulator would reject.
	return nil, nil
}

// anthropicUsage holds the usage fields shared by message_start and the
// cumulative message_delta events. Fields of later events overwrite earlier
// values when present.
type anthropicUsage struct {
	InputTokens              *int
	OutputTokens             *int
	CacheReadInputTokens     *int
	CacheCreationInputTokens *int
	CacheCreation            *anthropicapi.CacheCreation
	OutputTokensDetails      *anthropicapi.OutputTokensDetails
	ServerToolUse            *anthropicapi.ServerToolUsage
}

func anthropicUsageFromMessage(usage anthropicapi.Usage) anthropicUsage {
	return anthropicUsage{
		InputTokens:              ptr.To(usage.InputTokens),
		OutputTokens:             ptr.To(usage.OutputTokens),
		CacheReadInputTokens:     usage.CacheReadInputTokens,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
		CacheCreation:            usage.CacheCreation,
		OutputTokensDetails:      usage.OutputTokensDetails,
		ServerToolUse:            usage.ServerToolUse,
	}
}

func mergeAnthropicMessageDeltaUsage(result *anthropicUsage, usage anthropicapi.MessageDeltaUsage) {
	if usage.InputTokens != nil {
		result.InputTokens = usage.InputTokens
	}
	result.OutputTokens = ptr.To(usage.OutputTokens)
	if usage.CacheReadInputTokens != nil {
		result.CacheReadInputTokens = usage.CacheReadInputTokens
	}
	if usage.CacheCreationInputTokens != nil {
		result.CacheCreationInputTokens = usage.CacheCreationInputTokens
	}
	if usage.OutputTokensDetails != nil {
		result.OutputTokensDetails = usage.OutputTokensDetails
	}
	if usage.ServerToolUse != nil {
		result.ServerToolUse = usage.ServerToolUse
	}
}

// mapAnthropicUsage maps the raw usage to the SDK usage. Totals stay as
// Anthropic reports them: input_tokens excludes cached and cache-write tokens.
func mapAnthropicUsage(usage anthropicUsage) *llmsdk.ModelUsage {
	value := func(value *int) int {
		if value == nil {
			return 0
		}
		return *value
	}
	result := &llmsdk.ModelUsage{
		InputTokens:  value(usage.InputTokens),
		OutputTokens: value(usage.OutputTokens),
	}
	inputDetails := &llmsdk.ModelTokensDetails{}
	hasInputDetails := false
	if usage.CacheReadInputTokens != nil {
		inputDetails.CachedTokens = ptr.To(*usage.CacheReadInputTokens)
		hasInputDetails = true
	}
	if usage.CacheCreationInputTokens != nil {
		inputDetails.CacheWriteTokens = ptr.To(*usage.CacheCreationInputTokens)
		hasInputDetails = true
	}
	if usage.CacheCreation != nil && usage.CacheCreation.Ephemeral1HInputTokens > 0 {
		inputDetails.ExtendedCacheWriteTokens = ptr.To(usage.CacheCreation.Ephemeral1HInputTokens)
		hasInputDetails = true
	}
	if hasInputDetails {
		result.InputTokensDetails = inputDetails
	}
	if usage.OutputTokensDetails != nil {
		result.OutputTokensDetails = &llmsdk.ModelTokensDetails{
			ReasoningTokens: ptr.To(usage.OutputTokensDetails.ThinkingTokens),
		}
	}
	if usage.ServerToolUse != nil && usage.ServerToolUse.WebSearchRequests > 0 {
		result.ServerToolUse = &llmsdk.ModelServerToolUsage{
			WebSearchRequests: ptr.To(usage.ServerToolUse.WebSearchRequests),
		}
	}
	return result
}
