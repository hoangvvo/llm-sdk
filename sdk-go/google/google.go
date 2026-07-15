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

		if response.Candidates[0].Content == nil {
			return nil, llmsdk.NewInvariantError(Provider, "candidate content is missing")
		}

		content, err := mapGoogleContent(response.Candidates[0].Content.Parts, response.Candidates[0].GroundingMetadata)
		if err != nil {
			return nil, err
		}

		var usage *llmsdk.ModelUsage
		if response.UsageMetadata != nil {
			usage = mapGoogleUsageMetadata(*response.UsageMetadata)
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
			streamTextPartMappings := map[int]int{}

			for sseStream.Next() {
				streamEvent, err := sseStream.Current()
				if err != nil {
					errCh <- fmt.Errorf("failed to get sse event: %w", err)
					return
				}
				if streamEvent == nil || len(streamEvent.Candidates) == 0 {
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
					responseCh <- partial
				}

				if streamEvent.UsageMetadata != nil {
					usage := mapGoogleUsageMetadata(*streamEvent.UsageMetadata)
					partial := &llmsdk.PartialModelResponse{Usage: usage}
					if m.metadata != nil && m.metadata.Pricing != nil {
						partial.Cost = ptr.To(usage.CalculateCost(m.metadata.Pricing))
					}
					responseCh <- partial
				}
			}

			if err := sseStream.Err(); err != nil {
				errCh <- fmt.Errorf("scanner error: %w", err)
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
	}

	if input.ToolChoice != nil {
		params.ToolConfig = &googleapi.ToolConfig{
			FunctionCallingConfig: convertToGoogleFunctionCallingConfig(input.ToolChoice),
		}
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
	contents := make([]googleapi.Content, len(messages))
	for i, message := range messages {
		switch {
		case message.UserMessage != nil:
			parts, err := sliceutils.MapErr(message.UserMessage.Content, convertToGoogleParts)
			if err != nil {
				return nil, err
			}
			contents[i] = googleapi.Content{
				Role:  ptr.To("user"),
				Parts: sliceutils.Flat(parts),
			}
		case message.AssistantMessage != nil:
			parts, err := sliceutils.MapErr(message.AssistantMessage.Content, convertToGoogleParts)
			if err != nil {
				return nil, err
			}
			contents[i] = googleapi.Content{
				Role:  ptr.To("model"),
				Parts: sliceutils.Flat(parts),
			}
		case message.ToolMessage != nil:
			parts, err := sliceutils.MapErr(message.ToolMessage.Content, convertToGoogleParts)
			if err != nil {
				return nil, err
			}
			contents[i] = googleapi.Content{
				Role:  ptr.To("user"),
				Parts: sliceutils.Flat(parts),
			}
		default:
			return nil, llmsdk.NewInvalidInputError(fmt.Sprintf("unknown message type: %T", message))
		}
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
		var args map[string]any
		if err := json.Unmarshal(part.ToolCallPart.Args, &args); err != nil {
			return nil, llmsdk.NewInvalidInputError(fmt.Sprintf("invalid Google function arguments: %v", err))
		}
		googlePart := googleapi.Part{
			FunctionCall: &googleapi.FunctionCall{
				Name: &part.ToolCallPart.ToolName,
				Args: args,
				Id:   &part.ToolCallPart.ToolCallID,
			},
		}
		if part.ToolCallPart.Signature != nil {
			googlePart.ThoughtSignature = part.ToolCallPart.Signature
		}
		return []googleapi.Part{googlePart}, nil
	case part.ToolResultPart != nil:
		response, parts, err := convertToGoogleFunctionResponse(part.ToolResultPart.Content, part.ToolResultPart.IsError)
		if err != nil {
			return nil, err
		}
		return []googleapi.Part{{
			FunctionResponse: &googleapi.FunctionResponse{
				Id:       &part.ToolResultPart.ToolCallID,
				Name:     &part.ToolResultPart.ToolName,
				Response: response,
				Parts:    parts,
			},
		}}, nil
	}
	return []googleapi.Part{}, nil
}

func convertToGoogleFunctionResponse(parts []llmsdk.Part, isError bool) (map[string]any, []googleapi.FunctionResponsePart, error) {
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
	if isError {
		key = "error"
	}
	response := func() any {
		if len(responses) == 0 {
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
		case tool.FunctionTool != nil:
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
		LanguageCode: audio.LanguageCode,
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
			toolCallID = fmt.Sprintf("call_%s", randutil.String(10))
		}
		args, err := json.Marshal(part.FunctionCall.Args)
		if err != nil {
			return nil, llmsdk.NewInvariantError(Provider, fmt.Sprintf("invalid function call arguments: %v", err))
		}
		mapped := llmsdk.NewToolCallPart(toolCallID, *part.FunctionCall.Name, args)
		mapped.ToolCallPart.Args = args
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
func mapGoogleUsageMetadata(usageMetadata googleapi.UsageMetadata) *llmsdk.ModelUsage {
	usage := &llmsdk.ModelUsage{
		InputTokens:  0,
		OutputTokens: 0,
	}

	if usageMetadata.PromptTokenCount != nil {
		usage.InputTokens = *usageMetadata.PromptTokenCount
	}

	if usageMetadata.CandidatesTokenCount != nil {
		usage.OutputTokens = *usageMetadata.CandidatesTokenCount
	}

	if len(usageMetadata.PromptTokensDetails) > 0 {
		usage.InputTokensDetails =
			mapGoogleModalityTokenCountToUsageDetails(usageMetadata.PromptTokensDetails, usageMetadata.CacheTokensDetails)
	}

	if len(usageMetadata.CandidatesTokensDetails) > 0 {
		usage.OutputTokensDetails =
			mapGoogleModalityTokenCountToUsageDetails(usageMetadata.CandidatesTokensDetails, nil)
	}

	return usage
}

// mapGoogleModalityTokenCountToUsageDetails maps Google modality token counts to usage details
func mapGoogleModalityTokenCountToUsageDetails(
	modalityTokenCounts []googleapi.ModalityTokenCount,
	cachedTokenCounts []googleapi.ModalityTokenCount,
) *llmsdk.ModelTokensDetails {
	var details llmsdk.ModelTokensDetails

	for _, modalityTokenCount := range modalityTokenCounts {
		if modalityTokenCount.TokenCount == nil {
			continue
		}

		if modalityTokenCount.Modality != nil {
			switch *modalityTokenCount.Modality {
			case googleapi.ModalityTokenCountModalityTEXT:
				if details.TextTokens == nil {
					details.TextTokens = ptr.To(0)
				}
				*details.TextTokens += *modalityTokenCount.TokenCount
			case googleapi.ModalityTokenCountModalityIMAGE:
				if details.ImageTokens == nil {
					details.ImageTokens = ptr.To(0)
				}
				*details.ImageTokens += *modalityTokenCount.TokenCount
			case googleapi.ModalityTokenCountModalityAUDIO:
				if details.AudioTokens == nil {
					details.AudioTokens = ptr.To(0)
				}
				*details.AudioTokens += *modalityTokenCount.TokenCount
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
				if details.CachedTextTokens == nil {
					details.CachedTextTokens = ptr.To(0)
				}
				*details.CachedTextTokens += *cachedTokenCount.TokenCount
			case googleapi.ModalityTokenCountModalityIMAGE:
				if details.CachedImageTokens == nil {
					details.CachedImageTokens = ptr.To(0)
				}
				*details.CachedImageTokens += *cachedTokenCount.TokenCount
			case googleapi.ModalityTokenCountModalityAUDIO:
				if details.CachedAudioTokens == nil {
					details.CachedAudioTokens = ptr.To(0)
				}
				*details.CachedAudioTokens += *cachedTokenCount.TokenCount
			}
		}
	}

	return &details
}
