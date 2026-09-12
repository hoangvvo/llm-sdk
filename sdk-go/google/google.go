package google

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/google/googleapi"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/clientutils"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/sliceutils"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/toolresultutils"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/tracing"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/partutil"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/randutil"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/stream"
)

const Provider = "google"

type GoogleModelOptions struct {
	BaseURL    string
	APIKey     string
	APIVersion string
	Headers    map[string]string
	HTTPClient *http.Client
}

type GoogleModel struct {
	baseURL    string
	apiKey     string
	apiVersion string
	modelID    string
	client     *http.Client
	metadata   *llmsdk.LanguageModelMetadata
	headers    map[string]string
}

func NewGoogleModel(modelID string, options GoogleModelOptions) *GoogleModel {
	baseURL := "https://generativelanguage.googleapis.com"
	if options.BaseURL != "" {
		baseURL = options.BaseURL
	}
	apiVersion := "v1beta"
	if options.APIVersion != "" {
		apiVersion = options.APIVersion
	}

	client := options.HTTPClient
	if client == nil {
		client = &http.Client{}
	}

	headers := map[string]string{}
	for k, v := range options.Headers {
		headers[k] = v
	}

	return &GoogleModel{
		baseURL:    baseURL,
		apiKey:     options.APIKey,
		apiVersion: apiVersion,
		modelID:    modelID,
		client:     client,
		headers:    headers,
	}
}

func (m *GoogleModel) WithMetadata(metadata *llmsdk.LanguageModelMetadata) *GoogleModel {
	m.metadata = metadata
	return m
}

func (m *GoogleModel) Provider() string {
	return Provider
}

func (m *GoogleModel) ModelID() string {
	return m.modelID
}

func (m *GoogleModel) Metadata() *llmsdk.LanguageModelMetadata {
	return m.metadata
}

func (m *GoogleModel) requestHeaders() map[string]string {
	headers := map[string]string{
		"x-goog-api-key": m.apiKey,
	}

	for k, v := range m.headers {
		headers[k] = v
	}

	return headers
}

func (m *GoogleModel) Generate(ctx context.Context, input *llmsdk.LanguageModelInput) (*llmsdk.ModelResponse, error) {
	return tracing.TraceGenerate(ctx, string(Provider), m.modelID, input, func(ctx context.Context) (*llmsdk.ModelResponse, error) {
		params, err := convertToGenerateContentParameters(input, m.modelID)
		if err != nil {
			return nil, err
		}

		response, err := clientutils.DoJSON[googleapi.GenerateContentResponse](ctx, m.client, clientutils.JSONRequestConfig{
			URL:     fmt.Sprintf("%s/%s/models/%s:generateContent", m.baseURL, m.apiVersion, m.modelID),
			Headers: m.requestHeaders(),
			Body:    params,
		})
		if err != nil {
			return nil, err
		}

		if len(response.Candidates) == 0 {
			return nil, llmsdk.NewInvariantError(Provider, "no candidates returned")
		}

		var candidateParts []googleapi.Part
		if response.Candidates[0].Content != nil {
			candidateParts = response.Candidates[0].Content.Parts
		}

		content, err := mapGoogleContent(candidateParts, response.Candidates[0].GroundingMetadata)
		if err != nil {
			return nil, err
		}

		var usage *llmsdk.ModelUsage
		if response.UsageMetadata != nil {
			webSearchRequests := 0
			if response.Candidates[0].GroundingMetadata != nil {
				webSearchRequests = len(response.Candidates[0].GroundingMetadata.WebSearchQueries)
			}
			usage = mapGoogleUsageMetadata(*response.UsageMetadata, webSearchRequests)
		}

		result := &llmsdk.ModelResponse{
			Content: content,
			Usage:   usage,
		}

		if m.metadata != nil && m.metadata.Pricing != nil && usage != nil {
			cost := usage.CalculateCost(m.metadata.Pricing, llmsdk.ModelUsageCostOptions{InputCacheTokensAreAdditional: false, OutputReasoningTokensAreAdditional: true})
			result.Cost = &cost
		}

		return result, nil
	})
}

func (m *GoogleModel) Stream(ctx context.Context, input *llmsdk.LanguageModelInput) (*llmsdk.LanguageModelStream, error) {
	return tracing.TraceStream(ctx, string(Provider), m.modelID, input, func(ctx context.Context) (*llmsdk.LanguageModelStream, error) {
		params, err := convertToGenerateContentParameters(input, m.modelID)
		if err != nil {
			return nil, err
		}

		sseStream, err := clientutils.DoSSE[googleapi.GenerateContentResponse](ctx, m.client, clientutils.SSERequestConfig{
			URL:     fmt.Sprintf("%s/%s/models/%s:streamGenerateContent?alt=sse", m.baseURL, m.apiVersion, m.modelID),
			Headers: m.requestHeaders(),
			Body:    params,
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

			allContentDeltas := []llmsdk.ContentDelta{}
			// Streaming support indices address grounding chunks accumulated across
			// every response chunk, not only the current chunk.
			groundingChunks := []googleapi.GroundingChunk{}
			webSearchQueries := map[string]bool{}
			streamTextPartMappings := map[int]int{}
			var streamUsage *llmsdk.ModelUsage

			for sseStream.Next() {
				streamEvent, err := sseStream.Current()
				if err != nil {
					errCh <- fmt.Errorf("failed to get sse event: %w", err)
					return
				}
				if streamEvent == nil {
					continue
				}
				if streamEvent.UsageMetadata != nil {
					streamUsage = llmsdk.MergeModelUsageMax(
						streamUsage,
						mapGoogleUsageMetadata(*streamEvent.UsageMetadata, len(webSearchQueries)),
					)
				}
				if len(streamEvent.Candidates) == 0 {
					continue
				}

				candidate := streamEvent.Candidates[0]
				incomingContentDeltas := []llmsdk.ContentDelta{}
				if candidate.Content != nil {
					incomingContentDeltas, err = mapGoogleContentToDelta(
						*candidate.Content,
						allContentDeltas,
						streamTextPartMappings,
					)
					if err != nil {
						errCh <- fmt.Errorf("failed to map content delta: %w", err)
						return
					}

				}
				if candidate.GroundingMetadata != nil {
					for _, query := range candidate.GroundingMetadata.WebSearchQueries {
						webSearchQueries[query] = true
					}
					groundingChunks = append(groundingChunks, candidate.GroundingMetadata.GroundingChunks...)
					for _, support := range candidate.GroundingMetadata.GroundingSupports {
						sdkPartIndex, ok := streamTextPartMappings[googleGroundingSupportPartIndex(support)]
						if !ok {
							continue
						}
						for _, citation := range mapGoogleGroundingCitations(support, groundingChunks) {
							part := llmsdk.PartDelta{TextPartDelta: &llmsdk.TextPartDelta{Citation: &llmsdk.CitationDelta{
								Source: &citation.Source, Title: citation.Title, CitedText: citation.CitedText,
								StartIndex: citation.StartIndex, EndIndex: citation.EndIndex,
							}}}
							incomingContentDeltas = append(incomingContentDeltas, llmsdk.ContentDelta{
								Index: sdkPartIndex,
								Part:  part,
							})
						}
					}
				}
				allContentDeltas = append(allContentDeltas, incomingContentDeltas...)
				for _, delta := range incomingContentDeltas {
					partial := &llmsdk.PartialModelResponse{Delta: &delta}
					if !stream.Send(ctx, responseCh, partial) {
						return
					}
				}

			}

			if len(webSearchQueries) > 0 || len(groundingChunks) > 0 {
				maxIndex := -1
				for _, delta := range allContentDeltas {
					if delta.Index > maxIndex {
						maxIndex = delta.Index
					}
				}
				id := randutil.String(10)
				status := llmsdk.WebSearchToolCallStatusCompleted
				queries := make([]string, 0, len(webSearchQueries))
				for query := range webSearchQueries {
					queries = append(queries, query)
				}
				webCall := &llmsdk.WebSearchToolCall{Status: &status}
				if len(queries) > 0 {
					webCall.Action = &llmsdk.WebSearchAction{Type: "search", Queries: queries}
				}
				callDelta := &llmsdk.ContentDelta{Index: maxIndex + 1, Part: llmsdk.PartDelta{ToolCallPartDelta: &llmsdk.ToolCallPartDelta{ToolCallID: &id, Call: llmsdk.ToolCallDelta{WebSearch: &llmsdk.WebSearchToolCallDelta{Status: &status, Action: webCall.Action}}}}}
				if !stream.Send(ctx, responseCh, &llmsdk.PartialModelResponse{Delta: callDelta}) {
					return
				}
				sources := []llmsdk.WebSearchSource{}
				for _, chunk := range groundingChunks {
					if chunk.Web != nil && chunk.Web.Uri != nil {
						sources = append(sources, llmsdk.WebSearchSource{URL: *chunk.Web.Uri, Title: chunk.Web.Title})
					}
				}
				resultDelta := &llmsdk.ContentDelta{Index: maxIndex + 2, Part: llmsdk.PartDelta{ToolResultPartDelta: &llmsdk.ToolResultPartDelta{ToolCallID: id, Result: llmsdk.ToolResult{WebSearch: &llmsdk.WebSearchToolResult{Sources: sources}}, Status: llmsdk.ToolResultStatusCompleted}}}
				if !stream.Send(ctx, responseCh, &llmsdk.PartialModelResponse{Delta: resultDelta}) {
					return
				}
			}

			if err := sseStream.Err(); err != nil {
				errCh <- fmt.Errorf("scanner error: %w", err)
				return
			}
			if streamUsage != nil {
				// Search queries are only known once the stream ends.
				if len(webSearchQueries) > 0 {
					streamUsage.ServerToolUse = &llmsdk.ModelServerToolUsage{WebSearchRequests: ptr.To(len(webSearchQueries))}
				}
				partial := &llmsdk.PartialModelResponse{Usage: streamUsage}
				if m.metadata != nil && m.metadata.Pricing != nil {
					partial.Cost = ptr.To(streamUsage.CalculateCost(m.metadata.Pricing, llmsdk.ModelUsageCostOptions{InputCacheTokensAreAdditional: false, OutputReasoningTokensAreAdditional: true}))
				}
				if !stream.Send(ctx, responseCh, partial) {
					return
				}
			}
		}()

		return stream.New(responseCh, errCh), nil
	})
}

func convertToGenerateContentParameters(input *llmsdk.LanguageModelInput, modelID string) (*googleapi.GenerateContentRequest, error) {
	contents, err := convertToGoogleContents(input.Messages)
	if err != nil {
		return nil, err
	}

	generationConfig := &googleapi.GenerationConfig{
		Temperature:      input.Temperature,
		TopP:             input.TopP,
		PresencePenalty:  input.PresencePenalty,
		FrequencyPenalty: input.FrequencyPenalty,
	}
	if input.TopK != nil {
		generationConfig.TopK = ptr.To(int(*input.TopK))
	}
	if input.Seed != nil {
		generationConfig.Seed = ptr.To(int(*input.Seed))
	}
	if input.MaxTokens != nil {
		generationConfig.MaxOutputTokens = ptr.To(int(*input.MaxTokens))
	}

	params := &googleapi.GenerateContentRequest{
		Contents:         contents,
		Model:            &modelID,
		GenerationConfig: generationConfig,
	}

	if input.SystemPrompt != nil {
		params.SystemInstruction = &googleapi.Content{
			Role:  ptr.To("system"),
			Parts: []googleapi.Part{{Text: input.SystemPrompt}},
		}
	}

	if input.Tools != nil {
		params.Tools, err = convertToGoogleTools(input.Tools)
		if err != nil {
			return nil, err
		}
		// Google requires invocation data when web search and function tools are mixed.
		hasWebSearch := false
		hasFunction := false
		for _, tool := range input.Tools {
			if tool.WebSearchTool != nil {
				hasWebSearch = true
			}
			if tool.FunctionTool != nil {
				hasFunction = true
			}
		}
		if hasWebSearch && hasFunction {
			params.ToolConfig = &googleapi.ToolConfig{
				IncludeServerSideToolInvocations: ptr.To(true),
			}
		}
	}

	if input.ToolChoice != nil {
		if params.ToolConfig == nil {
			params.ToolConfig = &googleapi.ToolConfig{}
		}
		params.ToolConfig.FunctionCallingConfig = convertToGoogleFunctionCallingConfig(input.ToolChoice)
	}

	if input.ResponseFormat != nil {
		mimeType, schema := convertToGoogleResponseSchema(input.ResponseFormat)
		params.GenerationConfig.ResponseMimeType = &mimeType
		params.GenerationConfig.ResponseJsonSchema = schema
	}

	if input.Modalities != nil {
		params.GenerationConfig.ResponseModalities = sliceutils.Map(input.Modalities, convertToGoogleModality)
	}

	if input.Audio != nil {
		params.GenerationConfig.SpeechConfig = convertToGoogleSpeechConfig(*input.Audio)
	}

	if input.Reasoning != nil {
		params.GenerationConfig.ThinkingConfig = convertToGoogleThinkingConfig(*input.Reasoning)
	}

	return params, nil
}

func convertToGoogleContents(messages []llmsdk.Message) ([]googleapi.Content, error) {
	contents := make([]googleapi.Content, 0, len(messages))
	for _, message := range messages {
		var role string
		var messageParts []llmsdk.Part
		switch {
		case message.UserMessage != nil:
			role = "user"
			messageParts = message.UserMessage.Content
		case message.AssistantMessage != nil:
			role = "model"
			messageParts = message.AssistantMessage.Content
		case message.ToolMessage != nil:
			role = "user"
			messageParts = message.ToolMessage.Content
		default:
			return nil, llmsdk.NewInvalidInputError(fmt.Sprintf("unknown message type: %T", message))
		}

		parts, err := sliceutils.MapErr(messageParts, convertToGoogleParts)
		if err != nil {
			return nil, err
		}
		googleParts := sliceutils.Flat(parts)
		// Google hosted-tool metadata has no request part to replay.
		if len(googleParts) == 0 {
			continue
		}
		contents = append(contents, googleapi.Content{
			Role:  ptr.To(role),
			Parts: googleParts,
		})
	}
	return contents, nil
}

func convertToGoogleParts(part llmsdk.Part) ([]googleapi.Part, error) {
	switch {
	case part.TextPart != nil:
		return []googleapi.Part{{
			Text:             &part.TextPart.Text,
			ThoughtSignature: part.TextPart.Signature,
		}}, nil
	case part.ImagePart != nil:
		return []googleapi.Part{{
			InlineData: &googleapi.Blob{
				Data:     &part.ImagePart.Data,
				MimeType: &part.ImagePart.MimeType,
			},
		}}, nil
	case part.AudioPart != nil:
		return []googleapi.Part{{
			InlineData: &googleapi.Blob{
				Data:     &part.AudioPart.Data,
				MimeType: ptr.To(partutil.MapAudioFormatToMimeType(part.AudioPart.Format)),
			},
		}}, nil
	case part.ReasoningPart != nil:
		return []googleapi.Part{{
			Text:             &part.ReasoningPart.Text,
			Thought:          ptr.To(true),
			ThoughtSignature: part.ReasoningPart.Signature,
		}}, nil
	case part.SourcePart != nil:
		parts, err := sliceutils.MapErr(part.SourcePart.Content, convertToGoogleParts)
		if err != nil {
			return nil, err
		}
		return sliceutils.Flat(
			parts,
		), nil
	case part.ToolCallPart != nil:
		// Hosted tool history has no Gemini equivalent and is skipped.
		call := part.ToolCallPart.Call.Function
		if call == nil {
			return []googleapi.Part{}, nil
		}
		var args map[string]any
		if err := json.Unmarshal(call.Args, &args); err != nil {
			return nil, llmsdk.NewInvalidInputError(fmt.Sprintf("invalid Google function arguments: %v", err))
		}
		googlePart := googleapi.Part{
			FunctionCall: &googleapi.FunctionCall{
				Name: &call.Name,
				Args: args,
				Id:   &part.ToolCallPart.ToolCallID,
			},
		}
		if part.ToolCallPart.Signature != nil {
			googlePart.ThoughtSignature = part.ToolCallPart.Signature
		}
		return []googleapi.Part{googlePart}, nil
	case part.ToolResultPart != nil:
		result := part.ToolResultPart.Result.Function
		if result == nil {
			return []googleapi.Part{}, nil
		}
		response, parts, err := convertToGoogleFunctionResponse(result.Content, part.ToolResultPart.Status)
		if err != nil {
			return nil, err
		}
		return []googleapi.Part{{
			FunctionResponse: &googleapi.FunctionResponse{
				Id:       &part.ToolResultPart.ToolCallID,
				Name:     &result.Name,
				Response: response,
				Parts:    parts,
			},
		}}, nil
	}
	return []googleapi.Part{}, nil
}

func convertToGoogleFunctionResponse(parts []llmsdk.Part, status llmsdk.ToolResultStatus) (map[string]any, []googleapi.FunctionResponsePart, error) {
	compatibleParts := partutil.GetCompatiblePartsWithoutSourceParts(parts)
	textParts := []llmsdk.TextPart{}
	functionResponseParts := []googleapi.FunctionResponsePart{}
	for _, part := range compatibleParts {
		switch {
		case part.TextPart != nil:
			textParts = append(textParts, *part.TextPart)
		case part.ImagePart != nil:
			functionResponseParts = append(functionResponseParts, googleapi.FunctionResponsePart{
				InlineData: &googleapi.FunctionResponseBlob{
					Data:     &part.ImagePart.Data,
					MimeType: &part.ImagePart.MimeType,
				},
			})
		case part.AudioPart != nil:
			functionResponseParts = append(functionResponseParts, googleapi.FunctionResponsePart{
				InlineData: &googleapi.FunctionResponseBlob{
					Data:     &part.AudioPart.Data,
					MimeType: ptr.To(partutil.MapAudioFormatToMimeType(part.AudioPart.Format)),
				},
			})
		default:
			return nil, nil, llmsdk.NewInvalidInputError(fmt.Sprintf("Google model tool result does not support part type %q", part.Type()))
		}
	}

	responses := make([]map[string]any, len(textParts))
	for i, part := range textParts {
		// parse to map[string]any if possible
		var parsed map[string]any
		if err := json.Unmarshal([]byte(part.Text), &parsed); err != nil {
			responses[i] = map[string]any{"data": part.Text}
		} else {
			responses[i] = parsed
		}
	}

	// Use "output" key to specify function output and "error" key to specify error details,
	// as per Google API specification
	key := "output"
	if status != llmsdk.ToolResultStatusCompleted {
		key = "error"
	}
	response := func() any {
		if len(responses) == 0 {
			if status == llmsdk.ToolResultStatusCancelled {
				return toolresultutils.CancelledFallbackContent
			}
			return map[string]any{}
		}
		if len(responses) == 1 {
			return responses[0]
		}
		return responses
	}()
	return map[string]any{key: response}, functionResponseParts, nil
}

func convertToGoogleTools(tools []llmsdk.Tool) ([]googleapi.Tool, error) {
	functionDeclarations := make([]googleapi.FunctionDeclaration, 0, len(tools))
	googleTools := make([]googleapi.Tool, 0, len(tools))

	for _, tool := range tools {
		switch {
		case tool.ToolSearchTool != nil:
			return nil, llmsdk.NewUnsupportedError(Provider, "Google does not support hosted tool search")
		case tool.FunctionTool != nil:
			// Gemini has no deferred loading, so deferred tools are loaded eagerly.
			functionDeclarations = append(functionDeclarations, googleapi.FunctionDeclaration{
				Name:                 &tool.FunctionTool.Name,
				Description:          &tool.FunctionTool.Description,
				ParametersJsonSchema: tool.FunctionTool.Parameters,
			})
		case tool.WebSearchTool != nil:
			if len(tool.WebSearchTool.AllowedDomains) > 0 || tool.WebSearchTool.UserLocation != nil {
				// GoogleSearch has no equivalent fields. Reject these options instead
				// of silently broadening or de-localizing the search.
				return nil, llmsdk.NewUnsupportedError(Provider, "Google Search does not support allowed_domains or user_location")
			}
			googleTools = append(googleTools, googleapi.Tool{GoogleSearch: &googleapi.GoogleSearch{}})
		}
	}

	if len(functionDeclarations) > 0 {
		googleTools = append([]googleapi.Tool{{
			FunctionDeclarations: functionDeclarations,
		}}, googleTools...)
	}

	return googleTools, nil
}

func convertToGoogleFunctionCallingConfig(choice *llmsdk.ToolChoiceOption) *googleapi.FunctionCallingConfig {
	switch {
	case choice.Auto != nil:
		return &googleapi.FunctionCallingConfig{Mode: ptr.To(googleapi.FunctionCallingConfigModeAUTO)}
	case choice.Tool != nil:
		return &googleapi.FunctionCallingConfig{
			Mode: ptr.To(googleapi.FunctionCallingConfigModeANY),
			AllowedFunctionNames: []string{
				choice.Tool.ToolName,
			},
		}
	case choice.Required != nil:
		return &googleapi.FunctionCallingConfig{
			Mode: ptr.To(googleapi.FunctionCallingConfigModeANY),
		}
	case choice.None != nil:
		return &googleapi.FunctionCallingConfig{Mode: ptr.To(googleapi.FunctionCallingConfigModeNONE)}
	}
	return nil
}

func convertToGoogleResponseSchema(format *llmsdk.ResponseFormatOption) (string, any) {
	if format.JSON != nil {
		if format.JSON.Schema != nil {
			return "application/json", *format.JSON.Schema
		}
		return "application/json", nil
	}
	if format.Text != nil {
		return "text/plain", nil
	}
	return "", nil
}

func convertToGoogleModality(modality llmsdk.Modality) googleapi.GenerationConfigResponseModalitiesItem {
	switch modality {
	case llmsdk.ModalityText:
		return googleapi.GenerationConfigResponseModalitiesItemTEXT
	case llmsdk.ModalityImage:
		return googleapi.GenerationConfigResponseModalitiesItemIMAGE
	case llmsdk.ModalityAudio:
		return googleapi.GenerationConfigResponseModalitiesItemAUDIO
	}
	return ""
}

func convertToGoogleSpeechConfig(audio llmsdk.AudioOptions) *googleapi.SpeechConfig {
	return &googleapi.SpeechConfig{
		VoiceConfig: &googleapi.VoiceConfig{
			PrebuiltVoiceConfig: &googleapi.PrebuiltVoiceConfig{
				VoiceName: audio.Voice,
			},
		},
		LanguageCode: audio.Language,
	}
}

func convertToGoogleThinkingConfig(reasoning llmsdk.ReasoningOptions) *googleapi.ThinkingConfig {
	c := &googleapi.ThinkingConfig{
		IncludeThoughts: ptr.To(reasoning.Enabled),
	}
	if reasoning.BudgetTokens != nil {
		c.ThinkingBudget = ptr.To(int(*reasoning.BudgetTokens))
	}
	return c
}

// mapGoogleContent maps Google API parts to SDK parts
func mapGoogleContent(parts []googleapi.Part, groundingMetadata *googleapi.GroundingMetadata) ([]llmsdk.Part, error) {
	mappedParts := make([]*llmsdk.Part, len(parts))
	for i, part := range parts {
		mappedPart, err := mapGooglePart(part)
		if err != nil {
			return nil, err
		}
		mappedParts[i] = mappedPart
	}

	if groundingMetadata != nil {
		for _, support := range groundingMetadata.GroundingSupports {
			// Attach citations while provider part slots still exist. Filtering
			// unsupported Google parts first would shift segment.partIndex.
			partIndex := googleGroundingSupportPartIndex(support)
			if partIndex < 0 || partIndex >= len(mappedParts) {
				continue
			}
			part := mappedParts[partIndex]
			if part != nil && part.TextPart != nil {
				part.TextPart.Citations = append(
					part.TextPart.Citations,
					mapGoogleGroundingCitations(support, groundingMetadata.GroundingChunks)...,
				)
			}
		}
	}

	result := make([]llmsdk.Part, 0, len(mappedParts))
	for _, part := range mappedParts {
		if part != nil {
			result = append(result, *part)
		}
	}
	if groundingMetadata != nil && (len(groundingMetadata.WebSearchQueries) > 0 || len(groundingMetadata.GroundingChunks) > 0) {
		id := randutil.String(10)
		status := llmsdk.WebSearchToolCallStatusCompleted
		call := &llmsdk.WebSearchToolCall{Status: &status}
		if len(groundingMetadata.WebSearchQueries) > 0 {
			call.Action = &llmsdk.WebSearchAction{Type: "search", Queries: groundingMetadata.WebSearchQueries}
		}
		result = append(result, llmsdk.Part{ToolCallPart: &llmsdk.ToolCallPart{ToolCallID: id, Call: llmsdk.ToolCall{WebSearch: call}}})
		sources := []llmsdk.WebSearchSource{}
		for _, chunk := range groundingMetadata.GroundingChunks {
			if chunk.Web != nil && chunk.Web.Uri != nil {
				sources = append(sources, llmsdk.WebSearchSource{URL: *chunk.Web.Uri, Title: chunk.Web.Title})
			}
		}
		result = append(result, llmsdk.Part{ToolResultPart: &llmsdk.ToolResultPart{ToolCallID: id, Result: llmsdk.ToolResult{WebSearch: &llmsdk.WebSearchToolResult{Sources: sources}}, Status: llmsdk.ToolResultStatusCompleted}})
	}
	return result, nil
}

func mapGooglePart(part googleapi.Part) (*llmsdk.Part, error) {
	if part.Thought != nil && *part.Thought {
		text := ""
		if part.Text != nil {
			text = *part.Text
		}
		opts := []llmsdk.ReasoningPartOption{}
		if part.ThoughtSignature != nil {
			opts = append(opts, llmsdk.WithReasoningSignature(*part.ThoughtSignature))
		}
		mapped := llmsdk.NewReasoningPart(text, opts...)
		return &mapped, nil
	}

	if part.Text != nil {
		opts := []llmsdk.TextPartOption{}
		if part.ThoughtSignature != nil {
			opts = append(opts, llmsdk.WithTextSignature(*part.ThoughtSignature))
		}
		mapped := llmsdk.NewTextPart(*part.Text, opts...)
		return &mapped, nil
	}

	if part.InlineData != nil && part.InlineData.MimeType != nil && part.InlineData.Data != nil {
		if strings.HasPrefix(*part.InlineData.MimeType, "image/") {
			mapped := llmsdk.NewImagePart(*part.InlineData.Data, *part.InlineData.MimeType)
			return &mapped, nil
		}
		if strings.HasPrefix(*part.InlineData.MimeType, "audio/") {
			format, err := partutil.MapMimeTypeToAudioFormat(*part.InlineData.MimeType)
			if err != nil {
				return nil, llmsdk.NewInvariantError(Provider, fmt.Sprintf("unsupported audio mime type: %s", *part.InlineData.MimeType))
			}
			mapped := llmsdk.NewAudioPart(*part.InlineData.Data, format)
			return &mapped, nil
		}
	}

	if part.FunctionCall != nil {
		if part.FunctionCall.Name == nil {
			return nil, llmsdk.NewInvariantError(Provider, "function call name is missing")
		}
		toolCallID := ""
		if part.FunctionCall.Id != nil {
			toolCallID = *part.FunctionCall.Id
		} else {
			toolCallID = randutil.String(10)
		}
		args, err := json.Marshal(part.FunctionCall.Args)
		if err != nil {
			return nil, llmsdk.NewInvariantError(Provider, fmt.Sprintf("invalid function call arguments: %v", err))
		}
		mapped := llmsdk.NewToolCallPart(toolCallID, *part.FunctionCall.Name, args)
		mapped.ToolCallPart.Signature = part.ThoughtSignature
		return &mapped, nil
	}

	return nil, nil
}

func googleGroundingSupportPartIndex(support googleapi.GoogleAiGenerativelanguageV1BetaGroundingSupport) int {
	if support.Segment == nil || support.Segment.PartIndex == nil {
		return 0
	}
	return *support.Segment.PartIndex
}

func mapGoogleGroundingCitations(
	support googleapi.GoogleAiGenerativelanguageV1BetaGroundingSupport,
	chunks []googleapi.GroundingChunk,
) []llmsdk.Citation {
	if support.Segment == nil {
		return nil
	}
	var citations []llmsdk.Citation
	for _, chunkIndex := range support.GroundingChunkIndices {
		if chunkIndex < 0 || chunkIndex >= len(chunks) || chunks[chunkIndex].Web == nil || chunks[chunkIndex].Web.Uri == nil {
			continue
		}
		web := chunks[chunkIndex].Web
		citations = append(citations, llmsdk.Citation{
			Source: *web.Uri, Title: web.Title, CitedText: support.Segment.Text,
			StartIndex: support.Segment.StartIndex, EndIndex: support.Segment.EndIndex,
		})
	}
	return citations
}

// mapGoogleContentToDelta maps Google API content to content deltas for streaming
func mapGoogleContentToDelta(
	content googleapi.Content,
	existingContentDeltas []llmsdk.ContentDelta,
	streamTextPartMappings map[int]int,
) ([]llmsdk.ContentDelta, error) {
	if len(content.Parts) == 0 {
		return []llmsdk.ContentDelta{}, nil
	}

	contentDeltas := []llmsdk.ContentDelta{}
	for providerPartIndex, part := range content.Parts {
		mappedPart, err := mapGooglePart(part)
		if err != nil {
			return nil, err
		}
		if mappedPart == nil {
			continue
		}
		partDelta := partutil.LooselyConvertPartToPartDelta(*mappedPart)
		var index int
		if partDelta.TextPartDelta != nil {
			// Google's citation partIndex addresses the provider's parts array.
			// Keep a text-only mapping because provider slots are not stable for
			// separate tool calls, which retain the existing index matching.
			var ok bool
			index, ok = streamTextPartMappings[providerPartIndex]
			if !ok {
				hasIncomingText := false
				for _, delta := range contentDeltas {
					if delta.Part.TextPartDelta != nil {
						hasIncomingText = true
						break
					}
				}
				if hasIncomingText {
					// Multiple text parts in one chunk are distinct provider parts.
					index = nextGoogleDeltaIndex(existingContentDeltas, contentDeltas)
				} else {
					// Part indexes are local to an incremental chunk. Reuse the existing
					// text stream when a later chunk starts again at provider index zero.
					index = partutil.GuessDeltaIndex(partDelta, append(existingContentDeltas, contentDeltas...), nil)
				}
				streamTextPartMappings[providerPartIndex] = index
			}
		} else {
			index = partutil.GuessDeltaIndex(partDelta, append(existingContentDeltas, contentDeltas...), nil)
		}
		contentDeltas = append(contentDeltas, llmsdk.ContentDelta{
			Index: index,
			Part:  partDelta,
		})
	}

	return contentDeltas, nil
}

func nextGoogleDeltaIndex(existingContentDeltas, incomingContentDeltas []llmsdk.ContentDelta) int {
	maxIndex := -1
	for _, delta := range existingContentDeltas {
		if delta.Index > maxIndex {
			maxIndex = delta.Index
		}
	}
	for _, delta := range incomingContentDeltas {
		if delta.Index > maxIndex {
			maxIndex = delta.Index
		}
	}
	return maxIndex + 1
}

// mapGoogleUsageMetadata maps Google usage metadata to SDK usage
func mapGoogleUsageMetadata(usageMetadata googleapi.UsageMetadata, webSearchRequests int) *llmsdk.ModelUsage {
	value := func(value *int) (int, bool) {
		if value == nil {
			return 0, false
		}
		if *value < 0 {
			return 0, true
		}
		return *value, true
	}
	sumTokenCounts := func(details []googleapi.ModalityTokenCount) (int, bool) {
		total := 0
		hasCount := false
		for _, detail := range details {
			if count, ok := value(detail.TokenCount); ok {
				total += count
				hasCount = true
			}
		}
		return total, hasCount
	}

	promptTokens, hasPromptTokens := value(usageMetadata.PromptTokenCount)
	if !hasPromptTokens {
		promptTokens, hasPromptTokens = sumTokenCounts(usageMetadata.PromptTokensDetails)
	}
	toolUsePromptTokens, hasToolUsePromptTokens := value(usageMetadata.ToolUsePromptTokenCount)
	if !hasToolUsePromptTokens {
		toolUsePromptTokens, hasToolUsePromptTokens = sumTokenCounts(usageMetadata.ToolUsePromptTokensDetails)
	}
	outputTokens, hasOutputTokens := value(usageMetadata.CandidatesTokenCount)
	if !hasOutputTokens {
		outputTokens, hasOutputTokens = sumTokenCounts(usageMetadata.CandidatesTokensDetails)
	}
	reasoningTokens, hasReasoningTokens := value(usageMetadata.ThoughtsTokenCount)
	totalTokens, hasTotalTokens := value(usageMetadata.TotalTokenCount)

	if hasTotalTokens {
		if !hasPromptTokens && hasOutputTokens {
			promptTokens = max(0, totalTokens-outputTokens-toolUsePromptTokens-reasoningTokens)
			hasPromptTokens = true
		} else if !hasOutputTokens && hasPromptTokens {
			outputTokens = max(0, totalTokens-promptTokens-toolUsePromptTokens-reasoningTokens)
			hasOutputTokens = true
		}

		residual := max(0, totalTokens-promptTokens-toolUsePromptTokens-outputTokens-reasoningTokens)
		if residual > 0 {
			switch {
			case !hasReasoningTokens:
				reasoningTokens = residual
				hasReasoningTokens = true
			case !hasToolUsePromptTokens:
				toolUsePromptTokens = residual
				hasToolUsePromptTokens = true
			case !hasOutputTokens:
				outputTokens = residual
				hasOutputTokens = true
			default:
				promptTokens += residual
				hasPromptTokens = true
			}
		}
	}

	usage := &llmsdk.ModelUsage{
		InputTokens:  promptTokens + toolUsePromptTokens,
		OutputTokens: outputTokens,
	}
	if webSearchRequests > 0 {
		usage.ServerToolUse = &llmsdk.ModelServerToolUsage{WebSearchRequests: ptr.To(webSearchRequests)}
	}

	if len(usageMetadata.PromptTokensDetails) > 0 || len(usageMetadata.ToolUsePromptTokensDetails) > 0 || len(usageMetadata.CacheTokensDetails) > 0 {
		inputTokenDetails := append([]googleapi.ModalityTokenCount{}, usageMetadata.PromptTokensDetails...)
		inputTokenDetails = append(inputTokenDetails, usageMetadata.ToolUsePromptTokensDetails...)
		usage.InputTokensDetails =
			mapGoogleModalityTokenCountToUsageDetails(inputTokenDetails, usageMetadata.CacheTokensDetails)
	}
	if usageMetadata.CachedContentTokenCount != nil {
		if usage.InputTokensDetails == nil {
			usage.InputTokensDetails = &llmsdk.ModelTokensDetails{}
		}
		cachedTokens, _ := value(usageMetadata.CachedContentTokenCount)
		usage.InputTokensDetails.CachedTokens = ptr.To(cachedTokens)
	}

	if len(usageMetadata.CandidatesTokensDetails) > 0 {
		usage.OutputTokensDetails =
			mapGoogleModalityTokenCountToUsageDetails(usageMetadata.CandidatesTokensDetails, nil)
	}
	if usageMetadata.ThoughtsTokenCount != nil || reasoningTokens > 0 {
		if usage.OutputTokensDetails == nil {
			usage.OutputTokensDetails = &llmsdk.ModelTokensDetails{}
		}
		usage.OutputTokensDetails.ReasoningTokens = ptr.To(reasoningTokens)
	}

	return usage
}

// mapGoogleModalityTokenCountToUsageDetails maps Google modality token counts to usage details
func mapGoogleModalityTokenCountToUsageDetails(
	modalityTokenCounts []googleapi.ModalityTokenCount,
	cachedTokenCounts []googleapi.ModalityTokenCount,
) *llmsdk.ModelTokensDetails {
	var details llmsdk.ModelTokensDetails
	add := func(target **int, count *int) {
		if count == nil {
			return
		}
		value := max(0, *count)
		if *target == nil {
			*target = ptr.To(value)
		} else {
			**target += value
		}
	}

	for _, modalityTokenCount := range modalityTokenCounts {
		if modalityTokenCount.TokenCount == nil {
			continue
		}

		if modalityTokenCount.Modality != nil {
			switch *modalityTokenCount.Modality {
			case googleapi.ModalityTokenCountModalityTEXT:
				add(&details.TextTokens, modalityTokenCount.TokenCount)
			case googleapi.ModalityTokenCountModalityIMAGE:
				add(&details.ImageTokens, modalityTokenCount.TokenCount)
			case googleapi.ModalityTokenCountModalityAUDIO:
				add(&details.AudioTokens, modalityTokenCount.TokenCount)
			}
		}
	}

	for _, cachedTokenCount := range cachedTokenCounts {
		if cachedTokenCount.TokenCount == nil {
			continue
		}

		if cachedTokenCount.Modality != nil {
			switch *cachedTokenCount.Modality {
			case googleapi.ModalityTokenCountModalityTEXT:
				add(&details.CachedTextTokens, cachedTokenCount.TokenCount)
			case googleapi.ModalityTokenCountModalityIMAGE:
				add(&details.CachedImageTokens, cachedTokenCount.TokenCount)
			case googleapi.ModalityTokenCountModalityAUDIO:
				add(&details.CachedAudioTokens, cachedTokenCount.TokenCount)
			}
		}
	}

	if details.TextTokens == nil && details.AudioTokens == nil && details.ImageTokens == nil &&
		details.CachedTextTokens == nil && details.CachedAudioTokens == nil && details.CachedImageTokens == nil {
		return nil
	}
	return &details
}
