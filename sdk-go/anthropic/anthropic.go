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

				if event.MessageStart != nil {
					usage := mapAnthropicUsage(event.MessageStart.Message.Usage)
					partial := &llmsdk.PartialModelResponse{Usage: usage}
					if m.metadata != nil && m.metadata.Pricing != nil && usage != nil {
						cost := usage.CalculateCost(m.metadata.Pricing)
						partial.Cost = &cost
					}
					responseCh <- partial
					continue
				}

				if event.MessageDelta != nil {
					usage := mapAnthropicMessageDeltaUsage(event.MessageDelta.Usage)
					partial := &llmsdk.PartialModelResponse{Usage: usage}
					if m.metadata != nil && m.metadata.Pricing != nil && usage != nil {
						cost := usage.CalculateCost(m.metadata.Pricing)
						partial.Cost = &cost
					}
					responseCh <- partial
					continue
				}

				if event.ContentBlockStart != nil {
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
			strict := true
			anthropicTool := anthropicapi.Tool{
				Name:        tool.Name,
				InputSchema: anthropicapi.InputSchema(tool.Parameters),
				Strict:      &strict,
			}
			if tool.Description != "" {
				anthropicTool.Description = ptr.To(tool.Description)
			}
			tools = append(tools, anthropicapi.CreateMessageParamsToolsItem{
				Tool: &anthropicTool,
			})
		}
		params.Tools = tools
	}

	if input.Reasoning != nil {
		params.Thinking = convertToAnthropicThinkingConfigParam(*input.Reasoning, maxTokens)
	}

	if len(input.Metadata) > 0 {
		if userID, ok := input.Metadata["user_id"]; ok {
			params.Metadata = &anthropicapi.Metadata{UserId: ptr.To(userID)}
		}
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
		return anthropicapi.InputContentBlock{
			Text: &anthropicapi.RequestTextBlock{
				Type: "text",
				Text: part.TextPart.Text,
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
			textBlocks = append(textBlocks, anthropicapi.RequestTextBlock{Type: "text", Text: subPart.TextPart.Text})
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
			ToolUse: &anthropicapi.RequestToolUseBlock{
				Id:    part.ToolCallPart.ToolCallID,
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
		toolResult := anthropicapi.RequestToolResultBlock{
			ToolUseId: part.ToolResultPart.ToolCallID,
			Content: &anthropicapi.RequestToolResultBlockContent{
				RequestToolResultBlockContentArray: &content,
			},
		}
		if part.ToolResultPart.IsError {
			toolResult.IsError = ptr.To(true)
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
		part.ToolCallPart.Args = args
		return &part, nil

	case block.Thinking != nil:
		opts := []llmsdk.ReasoingPartOption{}
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
		if item.SearchResultLocation == nil {
			continue
		}
		source := item.SearchResultLocation.Source
		if source == "" {
			continue
		}

		citation := llmsdk.Citation{
			Source:     source,
			StartIndex: item.SearchResultLocation.StartBlockIndex,
			EndIndex:   item.SearchResultLocation.EndBlockIndex,
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
		Text:             event.ContentBlock.Text,
		Thinking:         event.ContentBlock.Thinking,
		RedactedThinking: event.ContentBlock.RedactedThinking,
		ToolUse:          event.ContentBlock.ToolUse,
		ServerToolUse:    event.ContentBlock.ServerToolUse,
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
