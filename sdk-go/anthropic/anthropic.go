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

		content, err := mapAnthropicMessage(response.Content)
		if err != nil {
			return nil, err
		}

		usage := mapAnthropicUsage(response.Usage)

		result := &llmsdk.ModelResponse{
			Content: content,
			Usage:   usage,
		}

		if m.metadata != nil && m.metadata.Pricing != nil && usage != nil {
			cost := usage.CalculateCost(m.metadata.Pricing)
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

			for sseStream.Next() {
				event, err := sseStream.Current()
				if err != nil {
					errCh <- fmt.Errorf("failed to get sse event: %w", err)
					return
				}
				if event == nil {
					continue
				}

				if event.MessageStartEvent != nil {
					usage := mapAnthropicUsage(event.MessageStartEvent.Message.Usage)
					partial := &llmsdk.PartialModelResponse{Usage: usage}
					if m.metadata != nil && m.metadata.Pricing != nil && usage != nil {
						cost := usage.CalculateCost(m.metadata.Pricing)
						partial.Cost = &cost
					}
					responseCh <- partial
					continue
				}

				if event.MessageDeltaEvent != nil {
					usage := mapAnthropicMessageDeltaUsage(event.MessageDeltaEvent.Usage)
					partial := &llmsdk.PartialModelResponse{Usage: usage}
					if m.metadata != nil && m.metadata.Pricing != nil && usage != nil {
						cost := usage.CalculateCost(m.metadata.Pricing)
						partial.Cost = &cost
					}
					responseCh <- partial
					continue
				}

				if event.ContentBlockStartEvent != nil {
					deltas, err := mapAnthropicRawContentBlockStartEvent(*event.ContentBlockStartEvent)
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

				if event.ContentBlockDeltaEvent != nil {
					deltas, err := mapAnthropicRawContentBlockDeltaEvent(*event.ContentBlockDeltaEvent)
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
			}

			if err := sseStream.Err(); err != nil {
				errCh <- fmt.Errorf("scanner error: %w", err)
			}
		}()

		return stream.New(responseCh, errCh), nil
	})
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
		Model:       anthropicapi.Model(modelID),
		Messages:    messages,
		MaxTokens:   maxTokens,
		Stream:      ptr.To(stream),
		Temperature: input.Temperature,
		TopP:        input.TopP,
	}

	if input.SystemPrompt != nil {
		params.System = *input.SystemPrompt
	}

	if input.TopK != nil {
		topK := int(*input.TopK)
		params.TopK = &topK
	}

	if input.ToolChoice != nil {
		params.ToolChoice = convertToAnthropicToolChoice(*input.ToolChoice)
	}

	if len(input.Tools) > 0 {
		tools := make([]interface{}, 0, len(input.Tools))
		for _, tool := range input.Tools {
			toolMap := map[string]any{
				"name":         tool.Name,
				"input_schema": tool.Parameters,
			}
			if tool.Description != "" {
				toolMap["description"] = tool.Description
			}
			tools = append(tools, toolMap)
		}
		params.Tools = tools
	}

	if input.Reasoning != nil {
		params.Thinking = convertToAnthropicThinkingConfigParam(*input.Reasoning, maxTokens)
	}

	if len(input.Metadata) > 0 {
		if userID, ok := input.Metadata["user_id"]; ok {
			params.Metadata = &anthropicapi.Metadata{UserID: ptr.To(userID)}
		}
	}

	return params, nil
}

func convertToAnthropicMessages(messages []llmsdk.Message) ([]anthropicapi.InputMessage, error) {
	result := make([]anthropicapi.InputMessage, 0, len(messages))

	for _, message := range messages {
		var parts []llmsdk.Part
		var role anthropicapi.Role

		switch {
		case message.UserMessage != nil:
			parts = message.UserMessage.Content
			role = anthropicapi.RoleUser
		case message.AssistantMessage != nil:
			parts = message.AssistantMessage.Content
			role = anthropicapi.RoleAssistant
		case message.ToolMessage != nil:
			parts = message.ToolMessage.Content
			role = anthropicapi.RoleUser
		default:
			continue
		}

		contentBlocks, err := convertPartsToAnthropicContentBlocks(parts)
		if err != nil {
			return nil, err
		}

		result = append(result, anthropicapi.InputMessage{
			Role:    role,
			Content: contentBlocks,
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
		return anthropicapi.InputContentBlock{
			RequestTextBlock: &anthropicapi.RequestTextBlock{
				Type: "text",
				Text: part.TextPart.Text,
			},
		}, nil

	case part.ImagePart != nil:
		source := map[string]any{
			"type":       "base64",
			"media_type": part.ImagePart.MimeType,
			"data":       part.ImagePart.Data,
		}
		return anthropicapi.InputContentBlock{
			RequestImageBlock: &anthropicapi.RequestImageBlock{
				Source: source,
			},
		}, nil

	case part.SourcePart != nil:
		textBlocks := make([]anthropicapi.RequestTextBlock, 0, len(part.SourcePart.Content))
		for _, subPart := range part.SourcePart.Content {
			if subPart.TextPart == nil {
				return anthropicapi.InputContentBlock{}, llmsdk.NewUnsupportedError(Provider, fmt.Sprintf("cannot convert source part content to anthropic search result for type %s", subPart.Type()))
			}
			textBlocks = append(textBlocks, anthropicapi.RequestTextBlock{Type: "text", Text: subPart.TextPart.Text})
		}
		return anthropicapi.InputContentBlock{
			RequestSearchResultBlock: &anthropicapi.RequestSearchResultBlock{
				Source:  part.SourcePart.Source,
				Title:   part.SourcePart.Title,
				Content: textBlocks,
				Citations: &anthropicapi.RequestCitationsConfig{
					Enabled: ptr.To(true),
				},
			},
		}, nil

	case part.ToolCallPart != nil:
		var inputMap map[string]any
		if len(part.ToolCallPart.Args) > 0 {
			if err := json.Unmarshal(part.ToolCallPart.Args, &inputMap); err != nil {
				return anthropicapi.InputContentBlock{}, fmt.Errorf("failed to unmarshal tool call args: %w", err)
			}
		}
		if inputMap == nil {
			inputMap = map[string]any{}
		}
		return anthropicapi.InputContentBlock{
			RequestToolUseBlock: &anthropicapi.RequestToolUseBlock{
				ID:    part.ToolCallPart.ToolCallID,
				Name:  part.ToolCallPart.ToolName,
				Input: inputMap,
			},
		}, nil

	case part.ToolResultPart != nil:
		contentBlocks := make([]anthropicapi.InputContentBlock, 0, len(part.ToolResultPart.Content))
		for _, subPart := range part.ToolResultPart.Content {
			block, err := convertPartToAnthropicContentBlock(subPart)
			if err != nil {
				return anthropicapi.InputContentBlock{}, err
			}
			if block.RequestTextBlock == nil && block.RequestImageBlock == nil && block.RequestSearchResultBlock == nil {
				return anthropicapi.InputContentBlock{}, llmsdk.NewUnsupportedError(Provider, fmt.Sprintf("cannot convert tool result part to anthropic content for type %s", subPart.Type()))
			}
			contentBlocks = append(contentBlocks, block)
		}
		toolResult := anthropicapi.RequestToolResultBlock{
			ToolUseID: part.ToolResultPart.ToolCallID,
			Content:   contentBlocks,
		}
		if part.ToolResultPart.IsError {
			toolResult.IsError = ptr.To(true)
		}
		return anthropicapi.InputContentBlock{
			RequestToolResultBlock: &toolResult,
		}, nil

	case part.ReasoningPart != nil:
		if part.ReasoningPart.Text == "" && part.ReasoningPart.Signature != nil {
			return anthropicapi.InputContentBlock{
				RequestRedactedThinkingBlock: &anthropicapi.RequestRedactedThinkingBlock{
					Data: *part.ReasoningPart.Signature,
				},
			}, nil
		}
		block := anthropicapi.RequestThinkingBlock{Thinking: part.ReasoningPart.Text}
		if part.ReasoningPart.Signature != nil {
			block.Signature = *part.ReasoningPart.Signature
		}
		return anthropicapi.InputContentBlock{
			RequestThinkingBlock: &block,
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

func convertToAnthropicThinkingConfigParam(reasoning llmsdk.ReasoningOptions, maxTokens int) *anthropicapi.ThinkingConfigParam {
	if !reasoning.Enabled {
		return &anthropicapi.ThinkingConfigParam{Disabled: &anthropicapi.ThinkingConfigDisabled{}}
	}

	budget := maxTokens - 1
	if reasoning.BudgetTokens != nil {
		budget = int(*reasoning.BudgetTokens)
	}
	if budget < 1 {
		budget = maxTokens - 1
	}

	return &anthropicapi.ThinkingConfigParam{
		Enabled: &anthropicapi.ThinkingConfigEnabled{
			BudgetTokens: budget,
		},
	}
}

func mapAnthropicMessage(content []anthropicapi.ContentBlock) ([]llmsdk.Part, error) {
	parts := make([]llmsdk.Part, 0, len(content))

	for _, block := range content {
		part, err := mapAnthropicContentBlock(block)
		if err != nil {
			return nil, err
		}
		if part != nil {
			parts = append(parts, *part)
		}
	}

	return parts, nil
}

func mapAnthropicContentBlock(block anthropicapi.ContentBlock) (*llmsdk.Part, error) {
	switch {
	case block.ResponseTextBlock != nil:
		citations, err := mapAnthropicTextCitations(block.ResponseTextBlock.Citations)
		if err != nil {
			return nil, err
		}
		opts := []llmsdk.TextPartOption{}
		if len(citations) > 0 {
			opts = append(opts, llmsdk.WithTextCitations(citations))
		}
		part := llmsdk.NewTextPart(block.ResponseTextBlock.Text, opts...)
		return &part, nil

	case block.ResponseToolUseBlock != nil:
		args, err := json.Marshal(block.ResponseToolUseBlock.Input)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tool use input: %w", err)
		}
		part := llmsdk.NewToolCallPart(block.ResponseToolUseBlock.ID, block.ResponseToolUseBlock.Name, json.RawMessage(args))
		part.ToolCallPart.Args = args
		return &part, nil

	case block.ResponseThinkingBlock != nil:
		opts := []llmsdk.ReasoningPartOption{}
		if block.ResponseThinkingBlock.Signature != "" {
			opts = append(opts, llmsdk.WithReasoningSignature(block.ResponseThinkingBlock.Signature))
		}
		part := llmsdk.NewReasoningPart(block.ResponseThinkingBlock.Thinking, opts...)
		return &part, nil

	case block.ResponseRedactedThinkingBlock != nil:
		part := llmsdk.NewReasoningPart("", llmsdk.WithReasoningSignature(block.ResponseRedactedThinkingBlock.Data))
		return &part, nil
	}

	return nil, nil
}

func mapAnthropicTextCitations(raw []interface{}) ([]llmsdk.Citation, error) {
	citations := make([]llmsdk.Citation, 0, len(raw))

	for _, item := range raw {
		citationMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		cType, _ := citationMap["type"].(string)
		if cType != "search_result_location" {
			continue
		}

		source, _ := citationMap["source"].(string)
		startIdx, okStart := toInt(citationMap["start_block_index"])
		endIdx, okEnd := toInt(citationMap["end_block_index"])
		if !okStart || !okEnd || source == "" {
			continue
		}

		citation := llmsdk.Citation{
			Source:     source,
			StartIndex: startIdx,
			EndIndex:   endIdx,
		}
		if citedText, ok := citationMap["cited_text"].(string); ok && citedText != "" {
			citation.CitedText = ptr.To(citedText)
		}
		if title, ok := citationMap["title"].(string); ok && title != "" {
			citation.Title = ptr.To(title)
		}
		citations = append(citations, citation)
	}

	return citations, nil
}

func mapAnthropicRawContentBlockStartEvent(event anthropicapi.ContentBlockStartEvent) ([]llmsdk.ContentDelta, error) {
	part, err := mapAnthropicRawContentBlock(event.ContentBlock)
	if err != nil {
		return nil, err
	}
	if part == nil {
		return nil, nil
	}

	delta := partutil.LooselyConvertPartToPartDelta(*part)
	if delta.ToolCallPartDelta != nil {
		empty := ""
		delta.ToolCallPartDelta.Args = &empty
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

func mapAnthropicRawContentBlock(raw interface{}) (*llmsdk.Part, error) {
	if raw == nil {
		return nil, nil
	}

	switch value := raw.(type) {
	case anthropicapi.ContentBlock:
		return mapAnthropicContentBlock(value)
	case *anthropicapi.ContentBlock:
		if value == nil {
			return nil, nil
		}
		return mapAnthropicContentBlock(*value)
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal raw content block: %w", err)
		}
		var block anthropicapi.ContentBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return nil, fmt.Errorf("failed to unmarshal raw content block: %w", err)
		}
		return mapAnthropicContentBlock(block)
	}
}

func mapAnthropicRawContentBlockDelta(raw interface{}) (*llmsdk.PartDelta, error) {
	if raw == nil {
		return nil, nil
	}

	deltaMap, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected delta type %T", raw)
	}

	deltaType, _ := deltaMap["type"].(string)

	switch deltaType {
	case "text_delta":
		text, _ := deltaMap["text"].(string)
		part := llmsdk.NewTextPartDelta(text)
		return &part, nil

	case "input_json_delta":
		partial, _ := deltaMap["partial_json"].(string)
		part := llmsdk.NewToolCallPartDelta(llmsdk.WithToolCallPartDeltaArgs(partial))
		return &part, nil

	case "thinking_delta":
		thinking, _ := deltaMap["thinking"].(string)
		part := llmsdk.NewReasoningPartDelta(thinking)
		return &part, nil

	case "signature_delta":
		signature, _ := deltaMap["signature"].(string)
		part := llmsdk.NewReasoningPartDelta("", llmsdk.WithReasoningPartDeltaSignature(signature))
		return &part, nil

	case "citations_delta":
		rawCitation, ok := deltaMap["citation"].(map[string]any)
		if !ok {
			return nil, nil
		}
		citationDelta, err := mapAnthropicCitationDelta(rawCitation)
		if err != nil || citationDelta == nil {
			return nil, err
		}
		part := llmsdk.NewTextPartDelta("", llmsdk.WithTextPartDeltaCitation(citationDelta))
		return &part, nil
	}

	return nil, nil
}

func mapAnthropicCitationDelta(raw map[string]any) (*llmsdk.CitationDelta, error) {
	cType, _ := raw["type"].(string)
	if cType != "search_result_location" {
		return nil, nil
	}

	citation := &llmsdk.CitationDelta{}

	if source, ok := raw["source"].(string); ok && source != "" {
		citation.Source = ptr.To(source)
	}
	if title, ok := raw["title"].(string); ok && title != "" {
		citation.Title = ptr.To(title)
	}
	if citedText, ok := raw["cited_text"].(string); ok && citedText != "" {
		citation.CitedText = ptr.To(citedText)
	}
	if start, ok := toInt(raw["start_block_index"]); ok {
		citation.StartIndex = ptr.To(start)
	}
	if end, ok := toInt(raw["end_block_index"]); ok {
		citation.EndIndex = ptr.To(end)
	}

	return citation, nil
}

func mapAnthropicUsage(usage anthropicapi.Usage) *llmsdk.ModelUsage {
	return &llmsdk.ModelUsage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
	}
}

func mapAnthropicMessageDeltaUsage(usage anthropicapi.MessageDeltaUsage) *llmsdk.ModelUsage {
	inputTokens := 0
	if usage.InputTokens != nil {
		inputTokens = *usage.InputTokens
	}
	return &llmsdk.ModelUsage{
		InputTokens:  inputTokens,
		OutputTokens: usage.OutputTokens,
	}
}

func toInt(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case uint:
		return int(v), true
	case uint32:
		return int(v), true
	case uint64:
		return int(v), true
	default:
		return 0, false
	}
}
