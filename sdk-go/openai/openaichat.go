package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/clientutils"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/tracing"
	"github.com/hoangvvo/llm-sdk/sdk-go/openai/openaichatapi"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/partutil"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/stream"
)

// OpenAIChatModel implements the LanguageModel interface for the Chat Completions API.
type OpenAIChatModel struct {
	modelID  string
	apiKey   string
	baseURL  string
	client   *http.Client
	metadata *llmsdk.LanguageModelMetadata
	headers  map[string]string
}

// OpenAIChatModelOptions represents configuration options for OpenAI chat models.
type OpenAIChatModelOptions struct {
	BaseURL    string
	APIKey     string
	Headers    map[string]string
	HTTPClient *http.Client
}

// NewOpenAIChatModel constructs a new OpenAIChatModel instance.
func NewOpenAIChatModel(modelID string, options OpenAIChatModelOptions) *OpenAIChatModel {
	baseURL := options.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	client := options.HTTPClient
	if client == nil {
		client = &http.Client{}
	}

	headers := map[string]string{}
	for k, v := range options.Headers {
		headers[k] = v
	}

	return &OpenAIChatModel{
		modelID: modelID,
		apiKey:  options.APIKey,
		baseURL: baseURL,
		client:  client,
		headers: headers,
	}
}

// WithMetadata attaches metadata to the model instance.
func (m *OpenAIChatModel) WithMetadata(metadata *llmsdk.LanguageModelMetadata) *OpenAIChatModel {
	m.metadata = metadata
	return m
}

// Provider returns the provider identifier.
func (m *OpenAIChatModel) Provider() string {
	return Provider
}

// ModelID returns the model identifier.
func (m *OpenAIChatModel) ModelID() string {
	return m.modelID
}

// Metadata returns the configured metadata, if any.
func (m *OpenAIChatModel) Metadata() *llmsdk.LanguageModelMetadata {
	return m.metadata
}

func (m *OpenAIChatModel) requestHeaders() map[string]string {
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", m.apiKey),
	}

	for k, v := range m.headers {
		headers[k] = v
	}

	return headers
}

// Generate performs a synchronous inference request.
func (m *OpenAIChatModel) Generate(ctx context.Context, input *llmsdk.LanguageModelInput) (*llmsdk.ModelResponse, error) {
	return tracing.TraceGenerate(ctx, Provider, m.modelID, input, func(ctx context.Context) (*llmsdk.ModelResponse, error) {
		params, err := convertToOpenAIChatCreateParams(input, m.modelID)
		if err != nil {
			return nil, err
		}

		body, err := buildChatCompletionRequestBody(params, input.Extra)
		if err != nil {
			return nil, err
		}

		response, err := clientutils.DoJSON[openaichatapi.CreateChatCompletionResponse](ctx, m.client, clientutils.JSONRequestConfig{
			URL:     fmt.Sprintf("%s/chat/completions", m.baseURL),
			Body:    body,
			Headers: m.requestHeaders(),
		})
		if err != nil {
			return nil, err
		}

		if len(response.Choices) == 0 {
			return nil, llmsdk.NewInvariantError(Provider, "no choices in response")
		}

		choice := response.Choices[0]
		if choice.Message.Refusal != nil && *choice.Message.Refusal != "" {
			return nil, llmsdk.NewRefusalError(*choice.Message.Refusal)
		}

		content, err := mapOpenAIChatMessage(choice.Message, params)
		if err != nil {
			return nil, err
		}

		var usage *llmsdk.ModelUsage
		if response.Usage != nil {
			usage = mapOpenAIChatUsage(*response.Usage, input)
		}

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

// Stream performs a streaming inference request.
func (m *OpenAIChatModel) Stream(ctx context.Context, input *llmsdk.LanguageModelInput) (*llmsdk.LanguageModelStream, error) {
	return tracing.TraceStream(ctx, Provider, m.modelID, input, func(ctx context.Context) (*llmsdk.LanguageModelStream, error) {
		params, err := convertToOpenAIChatCreateParams(input, m.modelID)
		if err != nil {
			return nil, err
		}
		params.Stream = ptr.To(true)
		params.StreamOptions = &openaichatapi.ChatCompletionStreamOptions{
			IncludeUsage: ptr.To(true),
		}

		body, err := buildChatCompletionRequestBody(params, input.Extra)
		if err != nil {
			return nil, err
		}

		sseStream, err := clientutils.DoSSE[openaichatapi.CreateChatCompletionStreamResponse](ctx, m.client, clientutils.SSERequestConfig{
			URL:     fmt.Sprintf("%s/chat/completions", m.baseURL),
			Body:    body,
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

			refusal := ""
			var allContentDeltas []llmsdk.ContentDelta

			for sseStream.Next() {
				streamEvent, err := sseStream.Current()
				if err != nil {
					errCh <- fmt.Errorf("failed to get sse event: %w", err)
					return
				}
				if streamEvent == nil {
					continue
				}

				if len(streamEvent.Choices) > 0 {
					choice := streamEvent.Choices[0]

					if choice.Delta.Refusal != nil {
						refusal += *choice.Delta.Refusal
					}

					incomingDeltas, err := mapOpenAIChatDelta(choice.Delta, allContentDeltas, params)
					if err != nil {
						errCh <- fmt.Errorf("failed to map stream delta: %w", err)
						return
					}

					allContentDeltas = append(allContentDeltas, incomingDeltas...)

					for _, delta := range incomingDeltas {
						d := delta
						responseCh <- &llmsdk.PartialModelResponse{Delta: &d}
					}
				}

				if streamEvent.Usage != nil {
					usage := mapOpenAIChatUsage(*streamEvent.Usage, input)
					partial := &llmsdk.PartialModelResponse{Usage: usage}
					if m.metadata != nil && m.metadata.Pricing != nil {
						partial.Cost = ptr.To(usage.CalculateCost(m.metadata.Pricing))
					}
					responseCh <- partial
				}
			}

			if err := sseStream.Err(); err != nil {
				errCh <- fmt.Errorf("scanner error: %w", err)
				return
			}

			if refusal != "" {
				errCh <- llmsdk.NewRefusalError(refusal)
			}
		}()

		return stream.New(responseCh, errCh), nil
	})
}

func convertToOpenAIChatCreateParams(input *llmsdk.LanguageModelInput, modelID string) (*openaichatapi.CreateChatCompletionRequest, error) {
	messages, err := convertToOpenAIChatMessages(input.Messages, input.SystemPrompt)
	if err != nil {
		return nil, err
	}

	params := &openaichatapi.CreateChatCompletionRequest{
		Model:    openaichatapi.ModelIdsShared(modelID),
		Messages: messages,
	}

	params.Temperature = input.Temperature
	params.TopP = input.TopP
	params.PresencePenalty = input.PresencePenalty
	params.FrequencyPenalty = input.FrequencyPenalty
	params.Seed = input.Seed

	if input.MaxTokens != nil {
		maxTokens := int(*input.MaxTokens)
		params.MaxTokens = &maxTokens
	}

	if len(input.Modalities) > 0 {
		modalities := make(openaichatapi.ResponseModalities, 0, len(input.Modalities))
		for _, modality := range input.Modalities {
			openaiModality, err := convertToOpenAIChatModality(modality)
			if err != nil {
				return nil, err
			}
			modalities = append(modalities, openaiModality)
		}
		params.Modalities = modalities
	}

	if input.Audio != nil {
		audio, err := convertToOpenAIChatAudio(*input.Audio)
		if err != nil {
			return nil, err
		}
		params.Audio = audio
	}

	if input.Tools != nil {
		var tools []interface{}
		for _, tool := range input.Tools {
			openAITool := openaichatapi.ChatCompletionTool{
				Type: "function",
				Function: openaichatapi.FunctionObject{
					Name: tool.Name,
				},
			}
			if tool.Description != "" {
				openAITool.Function.Description = &tool.Description
			}
			if tool.Parameters != nil {
				paramsCopy := openaichatapi.FunctionParameters(tool.Parameters)
				openAITool.Function.Parameters = &paramsCopy
			}
			strict := true
			openAITool.Function.Strict = &strict
			tools = append(tools, openAITool)
		}
		params.Tools = tools
	}

	if input.ToolChoice != nil {
		toolChoice, err := convertToOpenAIChatToolChoice(*input.ToolChoice)
		if err != nil {
			return nil, err
		}
		params.ToolChoice = toolChoice
	}

	if input.ResponseFormat != nil {
		responseFormat, err := convertToOpenAIChatResponseFormat(*input.ResponseFormat)
		if err != nil {
			return nil, err
		}
		params.ResponseFormat = responseFormat
	}

	if input.Reasoning != nil && input.Reasoning.BudgetTokens != nil {
		effort, err := convertToOpenAIChatReasoningEffort(*input.Reasoning.BudgetTokens)
		if err != nil {
			return nil, err
		}
		params.ReasoningEffort = effort
	}

	if len(input.Metadata) > 0 {
		metadata := openaichatapi.Metadata{}
		for k, v := range input.Metadata {
			metadata[k] = v
		}
		params.Metadata = &metadata
	}

	return params, nil
}

func convertToOpenAIChatMessages(messages []llmsdk.Message, systemPrompt *string) ([]openaichatapi.ChatCompletionRequestMessage, error) {
	var result []openaichatapi.ChatCompletionRequestMessage

	if systemPrompt != nil && *systemPrompt != "" {
		result = append(result, openaichatapi.ChatCompletionRequestMessage{
			System: &openaichatapi.ChatCompletionRequestSystemMessage{
				Role:    "system",
				Content: *systemPrompt,
			},
		})
	}

	for _, message := range messages {
		switch {
		case message.UserMessage != nil:
			userMessage, err := convertUserMessageToOpenAIChatMessage(message.UserMessage)
			if err != nil {
				return nil, err
			}
			result = append(result, userMessage)

		case message.AssistantMessage != nil:
			assistantMessages, err := convertAssistantMessageToOpenAIChatMessages(message.AssistantMessage)
			if err != nil {
				return nil, err
			}
			result = append(result, assistantMessages...)

		case message.ToolMessage != nil:
			toolMessages, err := convertToolMessageToOpenAIChatMessages(message.ToolMessage)
			if err != nil {
				return nil, err
			}
			result = append(result, toolMessages...)
		}
	}

	return result, nil
}

func convertUserMessageToOpenAIChatMessage(message *llmsdk.UserMessage) (openaichatapi.ChatCompletionRequestMessage, error) {
	contentParts := partutil.GetCompatiblePartsWithoutSourceParts(message.Content)

	var openAIContent []openaichatapi.ChatCompletionRequestUserMessageContentPart

	for _, part := range contentParts {
		switch {
		case part.TextPart != nil:
			openAIContent = append(openAIContent, openaichatapi.ChatCompletionRequestUserMessageContentPart{
				Text: &openaichatapi.ChatCompletionRequestMessageContentPartText{
					Type: "text",
					Text: part.TextPart.Text,
				},
			})
		case part.ImagePart != nil:
			openAIContent = append(openAIContent, openaichatapi.ChatCompletionRequestUserMessageContentPart{
				Image: &openaichatapi.ChatCompletionRequestMessageContentPartImage{
					Type: "image_url",
					ImageUrl: struct {
						Detail *string `json:"detail,omitempty"`
						Url    string  `json:"url"`
					}{
						Url: fmt.Sprintf("data:%s;base64,%s", part.ImagePart.MimeType, part.ImagePart.Data),
					},
				},
			})
		case part.AudioPart != nil:
			format, err := convertUserAudioFormat(part.AudioPart.Format)
			if err != nil {
				return openaichatapi.ChatCompletionRequestMessage{}, err
			}
			openAIContent = append(openAIContent, openaichatapi.ChatCompletionRequestUserMessageContentPart{
				Audio: &openaichatapi.ChatCompletionRequestMessageContentPartAudio{
					Type: "input_audio",
					InputAudio: struct {
						Data   string `json:"data"`
						Format string `json:"format"`
					}{
						Data:   part.AudioPart.Data,
						Format: format,
					},
				},
			})
		default:
			return openaichatapi.ChatCompletionRequestMessage{}, llmsdk.NewUnsupportedError(Provider, fmt.Sprintf("cannot convert user part of type %s to OpenAI chat message", part.Type()))
		}
	}

	return openaichatapi.ChatCompletionRequestMessage{
		User: &openaichatapi.ChatCompletionRequestUserMessage{
			Role:    "user",
			Content: openAIContent,
		},
	}, nil
}

func convertAssistantMessageToOpenAIChatMessages(message *llmsdk.AssistantMessage) ([]openaichatapi.ChatCompletionRequestMessage, error) {
	parts := partutil.GetCompatiblePartsWithoutSourceParts(message.Content)

	assistantMessage := openaichatapi.ChatCompletionRequestAssistantMessage{
		Role:    "assistant",
		Content: nil,
	}

	var contentParts []openaichatapi.ChatCompletionRequestAssistantMessageContentPart
	var toolCalls openaichatapi.ChatCompletionMessageToolCalls

	for _, part := range parts {
		switch {
		case part.TextPart != nil:
			contentParts = append(contentParts, openaichatapi.ChatCompletionRequestAssistantMessageContentPart{
				Text: &openaichatapi.ChatCompletionRequestMessageContentPartText{
					Type: "text",
					Text: part.TextPart.Text,
				},
			})
		case part.ToolCallPart != nil:
			if part.ToolCallPart == nil {
				continue
			}
			toolCall := openaichatapi.ChatCompletionMessageToolCall{
				ID:   part.ToolCallPart.ToolCallID,
				Type: "function",
			}
			toolCall.Function.Name = part.ToolCallPart.ToolName
			toolCall.Function.Arguments = string(part.ToolCallPart.Args)
			toolCalls = append(toolCalls, toolCall)
		case part.AudioPart != nil:
			if part.AudioPart.ID == nil {
				return nil, llmsdk.NewUnsupportedError(Provider, "assistant audio parts must include an ID for OpenAI chat API")
			}
			assistantMessage.Audio = &struct {
				ID string `json:"id"`
			}{
				ID: *part.AudioPart.ID,
			}
		default:
			return nil, llmsdk.NewUnsupportedError(Provider, fmt.Sprintf("cannot convert assistant part of type %s to OpenAI chat message", part.Type()))
		}
	}

	if len(contentParts) > 0 {
		assistantMessage.Content = contentParts
	}
	if len(toolCalls) > 0 {
		assistantMessage.ToolCalls = toolCalls
	}

	return []openaichatapi.ChatCompletionRequestMessage{{
		Assistant: &assistantMessage,
	}}, nil
}

func convertToolMessageToOpenAIChatMessages(message *llmsdk.ToolMessage) ([]openaichatapi.ChatCompletionRequestMessage, error) {
	var result []openaichatapi.ChatCompletionRequestMessage

	for _, part := range message.Content {
		if part.ToolResultPart == nil {
			return nil, llmsdk.NewInvalidInputError("tool messages must contain only tool result parts")
		}

		toolResultParts := partutil.GetCompatiblePartsWithoutSourceParts(part.ToolResultPart.Content)
		if len(toolResultParts) == 0 {
			continue
		}

		var contentParts []openaichatapi.ChatCompletionRequestMessageContentPartText
		for _, toolResultPart := range toolResultParts {
			if toolResultPart.TextPart == nil {
				return nil, llmsdk.NewInvalidInputError("tool result parts must contain only text parts for OpenAI chat API")
			}
			contentParts = append(contentParts, openaichatapi.ChatCompletionRequestMessageContentPartText{
				Type: "text",
				Text: toolResultPart.TextPart.Text,
			})
		}

		result = append(result, openaichatapi.ChatCompletionRequestMessage{
			Tool: &openaichatapi.ChatCompletionRequestToolMessage{
				Role:       "tool",
				ToolCallID: part.ToolResultPart.ToolCallID,
				Content:    contentParts,
			},
		})
	}

	return result, nil
}

func buildChatCompletionRequestBody(params *openaichatapi.CreateChatCompletionRequest, extra map[string]any) (map[string]any, error) {
	data, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat completion params: %w", err)
	}

	body := map[string]any{}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("failed to unmarshal chat completion params: %w", err)
	}

	for k, v := range extra {
		body[k] = v
	}

	return body, nil
}

func mapOpenAIChatMessage(message openaichatapi.ChatCompletionResponseMessage, params *openaichatapi.CreateChatCompletionRequest) ([]llmsdk.Part, error) {
	var parts []llmsdk.Part

	if message.Content != nil && *message.Content != "" {
		parts = append(parts, llmsdk.NewTextPart(*message.Content))
	}

	if message.Audio != nil {
		if params.Audio == nil {
			return nil, llmsdk.NewInvariantError(Provider, "audio returned from OpenAI API but no audio parameter was provided")
		}

		format, err := mapOpenAIChatAudioFormat(params.Audio.Format)
		if err != nil {
			return nil, err
		}

		audioOpts := []llmsdk.AudioPartOption{llmsdk.WithAudioID(message.Audio.ID)}
		if message.Audio.Transcript != "" {
			audioOpts = append(audioOpts, llmsdk.WithAudioTranscript(message.Audio.Transcript))
		}
		if format == llmsdk.AudioFormatLinear16 {
			audioOpts = append(audioOpts,
				llmsdk.WithAudioSampleRate(OpenAIAudioSampleRate),
				llmsdk.WithAudioChannels(OpenAIAudioChannels),
			)
		}
		parts = append(parts, llmsdk.NewAudioPart(message.Audio.Data, format, audioOpts...))
	}

	if message.ToolCalls != nil {
		for _, call := range message.ToolCalls {
			raw, err := json.Marshal(call)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal tool call: %w", err)
			}

			var toolCall openaichatapi.ChatCompletionMessageToolCall
			if err := json.Unmarshal(raw, &toolCall); err != nil {
				return nil, fmt.Errorf("failed to parse tool call: %w", err)
			}

			if toolCall.Type != "function" {
				return nil, llmsdk.NewNotImplementedError(Provider, fmt.Sprintf("cannot map OpenAI tool call of type %s", toolCall.Type))
			}

			var args map[string]any
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				return nil, fmt.Errorf("failed to parse tool call arguments: %w", err)
			}

			toolPart := llmsdk.NewToolCallPart(toolCall.ID, toolCall.Function.Name, args)
			parts = append(parts, toolPart)
		}
	}

	if message.FunctionCall != nil {
		var args map[string]any
		if err := json.Unmarshal([]byte(message.FunctionCall.Arguments), &args); err != nil {
			return nil, fmt.Errorf("failed to parse function call arguments: %w", err)
		}
		toolPart := llmsdk.NewToolCallPart(message.FunctionCall.Name, message.FunctionCall.Name, args)
		parts = append(parts, toolPart)
	}

	return parts, nil
}

func mapOpenAIChatDelta(delta openaichatapi.ChatCompletionStreamResponseDelta, existing []llmsdk.ContentDelta, params *openaichatapi.CreateChatCompletionRequest) ([]llmsdk.ContentDelta, error) {
	var result []llmsdk.ContentDelta

	if delta.Content != nil && *delta.Content != "" {
		part := llmsdk.NewTextPartDelta(*delta.Content)
		index := partutil.GuessDeltaIndex(part, append(existing, result...), nil)
		result = append(result, llmsdk.ContentDelta{Index: index, Part: part})
	}

	if delta.Audio != nil {
		if params.Audio == nil {
			return nil, llmsdk.NewInvariantError(Provider, "audio delta received without audio request params")
		}

		audioDelta := llmsdk.AudioPartDelta{}
		if delta.Audio.ID != nil {
			audioDelta.ID = delta.Audio.ID
		}
		if delta.Audio.Data != nil {
			format, err := mapOpenAIChatAudioFormat(params.Audio.Format)
			if err != nil {
				return nil, err
			}
			audioDelta.Data = delta.Audio.Data
			audioDelta.Format = ptr.To(format)
			if format == llmsdk.AudioFormatLinear16 {
				audioDelta.SampleRate = ptr.To(OpenAIAudioSampleRate)
				audioDelta.Channels = ptr.To(OpenAIAudioChannels)
			}
		}
		if delta.Audio.Transcript != nil {
			audioDelta.Transcript = delta.Audio.Transcript
		}

		part := llmsdk.PartDelta{AudioPartDelta: &audioDelta}
		index := partutil.GuessDeltaIndex(part, append(existing, result...), nil)
		result = append(result, llmsdk.ContentDelta{Index: index, Part: part})
	}

	for _, toolCall := range delta.ToolCalls {
		if toolCall.Type != nil && *toolCall.Type != "function" {
			return nil, llmsdk.NewNotImplementedError(Provider, fmt.Sprintf("cannot map OpenAI tool call delta of type %s", *toolCall.Type))
		}

		toolCallDelta := llmsdk.ToolCallPartDelta{}
		if toolCall.ID != nil {
			toolCallDelta.ToolCallID = toolCall.ID
		}
		if toolCall.Function != nil {
			if toolCall.Function.Name != nil {
				toolCallDelta.ToolName = toolCall.Function.Name
			}
			if toolCall.Function.Arguments != nil {
				toolCallDelta.Args = toolCall.Function.Arguments
			}
		}

		part := llmsdk.PartDelta{ToolCallPartDelta: &toolCallDelta}
		index := partutil.GuessDeltaIndex(part, append(existing, result...), ptr.To(toolCall.Index))
		result = append(result, llmsdk.ContentDelta{Index: index, Part: part})
	}

	if delta.FunctionCall != nil {
		part := llmsdk.PartDelta{ToolCallPartDelta: &llmsdk.ToolCallPartDelta{}}
		if delta.FunctionCall.Name != nil {
			part.ToolCallPartDelta.ToolName = delta.FunctionCall.Name
		}
		if delta.FunctionCall.Arguments != nil {
			part.ToolCallPartDelta.Args = delta.FunctionCall.Arguments
		}
		index := partutil.GuessDeltaIndex(part, append(existing, result...), nil)
		result = append(result, llmsdk.ContentDelta{Index: index, Part: part})
	}

	return result, nil
}

func mapOpenAIChatUsage(usage openaichatapi.CompletionUsage, input *llmsdk.LanguageModelInput) *llmsdk.ModelUsage {
	result := &llmsdk.ModelUsage{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
	}

	if usage.PromptTokensDetails != nil {
		details := usage.PromptTokensDetails
		tokensDetails := &llmsdk.ModelTokensDetails{}

		if details.TextTokens != nil {
			tokensDetails.TextTokens = ptr.To(*details.TextTokens)
		}
		if details.AudioTokens > 0 {
			tokensDetails.AudioTokens = ptr.To(details.AudioTokens)
		}
		if details.ImageTokens != nil {
			tokensDetails.ImageTokens = ptr.To(*details.ImageTokens)
		}

		if details.CachedTokensDetails != nil {
			if details.CachedTokensDetails.TextTokens != nil {
				tokensDetails.CachedTextTokens = ptr.To(*details.CachedTokensDetails.TextTokens)
			}
			if details.CachedTokensDetails.AudioTokens != nil {
				tokensDetails.CachedAudioTokens = ptr.To(*details.CachedTokensDetails.AudioTokens)
			}
		} else if details.CachedTokens > 0 {
			if hasUserTextPart(input.Messages) {
				tokensDetails.CachedTextTokens = ptr.To(details.CachedTokens)
			}
			if hasUserAudioPart(input.Messages) {
				tokensDetails.CachedAudioTokens = ptr.To(details.CachedTokens)
			}
		}

		if isModelTokensDetailsEmpty(tokensDetails) {
			tokensDetails = nil
		}
		result.InputTokensDetails = tokensDetails
	}

	if usage.CompletionTokensDetails != nil {
		details := usage.CompletionTokensDetails
		tokensDetails := &llmsdk.ModelTokensDetails{}
		if details.TextTokens != nil {
			tokensDetails.TextTokens = ptr.To(*details.TextTokens)
		}
		if details.AudioTokens > 0 {
			tokensDetails.AudioTokens = ptr.To(details.AudioTokens)
		}
		if isModelTokensDetailsEmpty(tokensDetails) {
			tokensDetails = nil
		}
		result.OutputTokensDetails = tokensDetails
	}

	return result
}

func convertToOpenAIChatModality(modality llmsdk.Modality) (string, error) {
	switch modality {
	case llmsdk.ModalityText:
		return "text", nil
	case llmsdk.ModalityAudio:
		return "audio", nil
	default:
		return "", llmsdk.NewUnsupportedError(Provider, fmt.Sprintf("cannot convert modality %s to OpenAI chat modality", modality))
	}
}

func convertToOpenAIChatAudio(options llmsdk.AudioOptions) (*struct {
	Format string                       `json:"format"`
	Voice  openaichatapi.VoiceIdsShared `json:"voice"`
}, error) {
	if options.Voice == nil || *options.Voice == "" {
		return nil, llmsdk.NewInvalidInputError("audio voice is required for OpenAI audio")
	}

	if options.Format == nil {
		return nil, llmsdk.NewInvalidInputError("audio format is required for OpenAI audio")
	}

	format, err := convertToOpenAIChatAudioFormat(*options.Format)
	if err != nil {
		return nil, err
	}

	return &struct {
		Format string                       `json:"format"`
		Voice  openaichatapi.VoiceIdsShared `json:"voice"`
	}{
		Format: format,
		Voice:  openaichatapi.VoiceIdsShared(*options.Voice),
	}, nil
}

func convertToOpenAIChatAudioFormat(format llmsdk.AudioFormat) (string, error) {
	switch format {
	case llmsdk.AudioFormatWav:
		return "wav", nil
	case llmsdk.AudioFormatMP3:
		return "mp3", nil
	case llmsdk.AudioFormatFLAC:
		return "flac", nil
	case llmsdk.AudioFormatOpus:
		return "opus", nil
	case llmsdk.AudioFormatLinear16:
		return "pcm16", nil
	case llmsdk.AudioFormatAAC:
		return "aac", nil
	default:
		return "", llmsdk.NewUnsupportedError(Provider, fmt.Sprintf("cannot convert audio format %s to OpenAI audio format", format))
	}
}

func convertUserAudioFormat(format llmsdk.AudioFormat) (string, error) {
	switch format {
	case llmsdk.AudioFormatMP3:
		return "mp3", nil
	case llmsdk.AudioFormatWav:
		return "wav", nil
	default:
		return "", llmsdk.NewUnsupportedError(Provider, fmt.Sprintf("cannot convert audio format %s to OpenAI input audio format", format))
	}
}

func mapOpenAIChatAudioFormat(format string) (llmsdk.AudioFormat, error) {
	switch format {
	case "wav":
		return llmsdk.AudioFormatWav, nil
	case "mp3":
		return llmsdk.AudioFormatMP3, nil
	case "flac":
		return llmsdk.AudioFormatFLAC, nil
	case "opus":
		return llmsdk.AudioFormatOpus, nil
	case "pcm16":
		return llmsdk.AudioFormatLinear16, nil
	case "aac":
		return llmsdk.AudioFormatAAC, nil
	default:
		return "", llmsdk.NewUnsupportedError(Provider, fmt.Sprintf("unknown OpenAI audio format %s", format))
	}
}

func convertToOpenAIChatToolChoice(choice llmsdk.ToolChoiceOption) (interface{}, error) {
	switch {
	case choice.Auto != nil:
		return "auto", nil
	case choice.None != nil:
		return "none", nil
	case choice.Required != nil:
		return "required", nil
	case choice.Tool != nil:
		return openaichatapi.ChatCompletionNamedToolChoice{
			Type: "function",
			Function: struct {
				Name string `json:"name"`
			}{
				Name: choice.Tool.ToolName,
			},
		}, nil
	default:
		return nil, llmsdk.NewInvalidInputError("tool choice has no content")
	}
}

func convertToOpenAIChatResponseFormat(option llmsdk.ResponseFormatOption) (interface{}, error) {
	switch {
	case option.Text != nil:
		return openaichatapi.ResponseFormatText{Type: "text"}, nil
	case option.JSON != nil:
		if option.JSON.Schema != nil {
			schema := openaichatapi.ResponseFormatJsonSchemaSchema(*option.JSON.Schema)
			strict := true
			jsonSchema := openaichatapi.ResponseFormatJsonSchema{}
			jsonSchema.Type = "json_schema"
			jsonSchema.JsonSchema.Name = option.JSON.Name
			jsonSchema.JsonSchema.Schema = &schema
			jsonSchema.JsonSchema.Strict = &strict
			if option.JSON.Description != nil {
				jsonSchema.JsonSchema.Description = option.JSON.Description
			}
			return jsonSchema, nil
		}
		return openaichatapi.ResponseFormatJsonObject{Type: "json_object"}, nil
	default:
		return nil, llmsdk.NewInvalidInputError("response format has no content")
	}
}

func convertToOpenAIChatReasoningEffort(budgetTokens uint32) (*openaichatapi.ReasoningEffort, error) {
	switch OpenAIReasoningEffort(budgetTokens) {
	case OpenAIReasoningEffortMinimal:
		effort := openaichatapi.ReasoningEffortMinimal
		return &effort, nil
	case OpenAIReasoningEffortLow:
		effort := openaichatapi.ReasoningEffortLow
		return &effort, nil
	case OpenAIReasoningEffortMedium:
		effort := openaichatapi.ReasoningEffortMedium
		return &effort, nil
	case OpenAIReasoningEffortHigh:
		effort := openaichatapi.ReasoningEffortHigh
		return &effort, nil
	default:
		return nil, llmsdk.NewUnsupportedError(Provider, "Budget tokens property is not supported for OpenAI reasoning. You may use OpenAIReasoningEffort enum values to map it to OpenAI reasoning effort levels.")
	}
}

func hasUserTextPart(messages []llmsdk.Message) bool {
	for _, message := range messages {
		if message.UserMessage == nil {
			continue
		}
		for _, part := range message.UserMessage.Content {
			if part.TextPart != nil {
				return true
			}
		}
	}
	return false
}

func hasUserAudioPart(messages []llmsdk.Message) bool {
	for _, message := range messages {
		if message.UserMessage == nil {
			continue
		}
		for _, part := range message.UserMessage.Content {
			if part.AudioPart != nil {
				return true
			}
		}
	}
	return false
}

func isModelTokensDetailsEmpty(details *llmsdk.ModelTokensDetails) bool {
	if details == nil {
		return true
	}
	return details.TextTokens == nil &&
		details.CachedTextTokens == nil &&
		details.AudioTokens == nil &&
		details.CachedAudioTokens == nil &&
		details.ImageTokens == nil &&
		details.CachedImageTokens == nil
}
