package google

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"maps"
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

type ProviderType string

const (
	ProviderTypeGoogleAI ProviderType = "google-ai"
	ProviderTypeVertexAI ProviderType = "vertex-ai"
)

type GoogleModelOptions struct {
	BaseURL      string
	APIKey       string
	APIVersion   string
	Headers      map[string]string
	HTTPClient   *http.Client
	ProviderType ProviderType
	AccessToken  string
	ProjectID    string
	Location     string
}

type GoogleModel struct {
	baseURL      string
	apiKey       string
	apiVersion   string
	modelID      string
	client       *http.Client
	metadata     *llmsdk.LanguageModelMetadata
	headers      map[string]string
	providerType ProviderType
	accessToken  string
	projectID    string
	location     string
}

func NewGoogleModel(modelID string, options GoogleModelOptions) *GoogleModel {
	providerType := cmp.Or(options.ProviderType, ProviderTypeGoogleAI)

	baseURL := options.BaseURL
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
		if providerType == ProviderTypeVertexAI {
			if options.Location == "" || options.Location == "global" {
				baseURL = "https://aiplatform.googleapis.com"
			} else {
				baseURL = "https://" + options.Location + "-aiplatform.googleapis.com"
			}
		}
	}

	apiVersion := options.APIVersion
	if apiVersion == "" {
		if providerType == ProviderTypeVertexAI {
			apiVersion = "v1beta1"
		} else {
			apiVersion = "v1beta"
		}
	}

	return &GoogleModel{
		baseURL:    baseURL,
		apiKey:     options.APIKey,
		apiVersion: apiVersion,
		modelID:    modelID,
		client:     cmp.Or(options.HTTPClient, &http.Client{}),
		headers:    maps.Clone(options.Headers),

		providerType: providerType,
		accessToken:  options.AccessToken,
		projectID:    options.ProjectID,
		location:     options.Location,
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
	headers := maps.Clone(m.headers)
	if headers == nil {
		headers = make(map[string]string, 1)
	}

	if m.accessToken != "" {
		headers["Authorization"] = "Bearer " + m.accessToken
	} else if m.apiKey != "" {
		headers["x-goog-api-key"] = m.apiKey
	}

	return headers
}

func (m *GoogleModel) buildEndpointURL(action string) string {
	var b strings.Builder
	b.WriteString(m.baseURL)
	b.WriteByte('/')
	b.WriteString(m.apiVersion)

	if m.providerType == ProviderTypeVertexAI {
		if m.projectID != "" {
			b.WriteString("/projects/")
			b.WriteString(m.projectID)
			b.WriteString("/locations/")
			b.WriteString(cmp.Or(m.location, "global"))
		}
		b.WriteString("/publishers/google")
	}

	b.WriteString("/models/")
	b.WriteString(m.modelID)
	b.WriteByte(':')
	b.WriteString(action)

	return b.String()
}

func (m *GoogleModel) Generate(ctx context.Context, input *llmsdk.LanguageModelInput) (*llmsdk.ModelResponse, error) {
	return tracing.TraceGenerate(ctx, string(Provider), m.modelID, input, func(ctx context.Context) (*llmsdk.ModelResponse, error) {
		params, err := convertToGenerateContentParameters(input, m.modelID)
		if err != nil {
			return nil, err
		}

		response, err := clientutils.DoJSON[googleapi.GenerateContentResponse](ctx, m.client, clientutils.JSONRequestConfig{
			URL:     m.buildEndpointURL("generateContent"),
			Headers: m.requestHeaders(),
			Body:    params,
		})
		if err != nil {
			return nil, err
		}

		if len(response.Candidates) == 0 {
			return nil, llmsdk.NewInvariantError(Provider, "no candidates returned")
		}

		content, err := mapGoogleContent(response.Candidates[0].Content.Parts)
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
			URL:     m.buildEndpointURL("streamGenerateContent?alt=sse"),
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
				if candidate.Content != nil {
					incomingContentDeltas, err := mapGoogleContentToDelta(*candidate.Content, allContentDeltas)
					if err != nil {
						errCh <- fmt.Errorf("failed to map content delta: %w", err)
						return
					}

					allContentDeltas = append(allContentDeltas, incomingContentDeltas...)

					for _, delta := range incomingContentDeltas {
						partial := &llmsdk.PartialModelResponse{Delta: &delta}
						responseCh <- partial
					}
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

func convertToGenerateContentParameters(input *llmsdk.LanguageModelInput, modelID string) (*googleapi.GenerateContentParameters, error) {
	contents, err := convertToGoogleContents(input.Messages)
	if err != nil {
		return nil, err
	}

	params := &googleapi.GenerateContentParameters{
		Contents: contents,
		Model:    modelID,
		GenerationConfig: &googleapi.GenerateContentConfig{
			Temperature:      input.Temperature,
			TopP:             input.TopP,
			TopK:             input.TopK,
			PresencePenalty:  input.PresencePenalty,
			FrequencyPenalty: input.FrequencyPenalty,
			Seed:             input.Seed,
			MaxOutputTokens:  input.MaxTokens,
		},
	}

	if input.SystemPrompt != nil {
		params.SystemInstruction = &googleapi.Content{
			Role:  "system", // ignored by Google API
			Parts: []googleapi.Part{{Text: input.SystemPrompt}},
		}
	}

	if input.Tools != nil {
		params.Tools = []googleapi.Tool{
			{
				FunctionDeclarations: sliceutils.Map(input.Tools, convertToGoogleTool),
			},
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
				Role:  "user",
				Parts: sliceutils.Flat(parts),
			}
		case message.AssistantMessage != nil:
			parts, err := sliceutils.MapErr(message.AssistantMessage.Content, convertToGoogleParts)
			if err != nil {
				return nil, err
			}
			contents[i] = googleapi.Content{
				Role:  "model",
				Parts: sliceutils.Flat(parts),
			}
		case message.ToolMessage != nil:
			parts, err := sliceutils.MapErr(message.ToolMessage.Content, convertToGoogleParts)
			if err != nil {
				return nil, err
			}
			contents[i] = googleapi.Content{
				Role:  "user",
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
		return []googleapi.Part{{Text: &part.TextPart.Text}}, nil
	case part.ImagePart != nil:
		return []googleapi.Part{{
			InlineData: &googleapi.Blob2{
				Data:     &part.ImagePart.Data,
				MimeType: &part.ImagePart.MimeType,
			},
		}}, nil
	case part.AudioPart != nil:
		return []googleapi.Part{{
			InlineData: &googleapi.Blob2{
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
		googlePart := googleapi.Part{
			FunctionCall: &googleapi.FunctionCall{
				Name: &part.ToolCallPart.ToolName,
				Args: part.ToolCallPart.Args,
				Id:   &part.ToolCallPart.ToolCallID,
			},
		}
		if part.ToolCallPart.ThoughtSignature != nil {
			googlePart.ThoughtSignature = part.ToolCallPart.ThoughtSignature
		}
		return []googleapi.Part{googlePart}, nil
	case part.ToolResultPart != nil:
		response, err := convertToGoogleFunctionResponseResponse(part.ToolResultPart.Content, part.ToolResultPart.IsError)
		if err != nil {
			return nil, err
		}
		return []googleapi.Part{{
			FunctionResponse: &googleapi.FunctionResponse{
				Id:       &part.ToolResultPart.ToolCallID,
				Name:     &part.ToolResultPart.ToolName,
				Response: response,
			},
		}}, nil
	}
	return []googleapi.Part{}, nil
}

func convertToGoogleFunctionResponseResponse(parts []llmsdk.Part, isError bool) (map[string]any, error) {
	compatibleParts := partutil.GetCompatiblePartsWithoutSourceParts(parts)
	textParts := []llmsdk.TextPart{}
	for _, part := range compatibleParts {
		if part.TextPart != nil {
			textParts = append(textParts, *part.TextPart)
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

	if len(responses) == 0 {
		return nil, llmsdk.NewInvalidInputError("Google model tool result must have at least one text part")
	}

	// Use "output" key to specify function output and "error" key to specify error details,
	// as per Google API specification
	key := "output"
	if isError {
		key = "error"
	}
	return map[string]any{
		key: func() any {
			if len(responses) == 1 {
				return responses[0]
			}
			return responses
		}(),
	}, nil
}

func convertToGoogleTool(tool llmsdk.Tool) googleapi.FunctionDeclaration {
	return googleapi.FunctionDeclaration{
		Name:                 &tool.Name,
		Description:          &tool.Description,
		ParametersJsonSchema: tool.Parameters,
	}
}

func convertToGoogleFunctionCallingConfig(choice *llmsdk.ToolChoiceOption) *googleapi.FunctionCallingConfig {
	switch {
	case choice.Auto != nil:
		return &googleapi.FunctionCallingConfig{Mode: ptr.To(googleapi.FunctionCallingConfigModeAuto)}
	case choice.Tool != nil:
		return &googleapi.FunctionCallingConfig{
			Mode: ptr.To(googleapi.FunctionCallingConfigModeAny),
			AllowedFunctionNames: []string{
				choice.Tool.ToolName,
			},
		}
	case choice.Required != nil:
		return &googleapi.FunctionCallingConfig{
			Mode: ptr.To(googleapi.FunctionCallingConfigModeAny),
		}
	case choice.None != nil:
		return &googleapi.FunctionCallingConfig{Mode: ptr.To(googleapi.FunctionCallingConfigModeNone)}
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

func convertToGoogleModality(modality llmsdk.Modality) string {
	switch modality {
	case llmsdk.ModalityText:
		return "TEXT"
	case llmsdk.ModalityImage:
		return "IMAGE"
	case llmsdk.ModalityAudio:
		return "AUDIO"
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
func mapGoogleContent(parts []googleapi.Part) ([]llmsdk.Part, error) {
	var result []llmsdk.Part

	for _, part := range parts {
		if part.Thought != nil && *part.Thought {
			text := ""
			if part.Text != nil {
				text = *part.Text
			}
			opts := []llmsdk.ReasoningPartOption{}
			if part.ThoughtSignature != nil {
				opts = append(opts, llmsdk.WithReasoningSignature(*part.ThoughtSignature))
			}
			result = append(result, llmsdk.NewReasoningPart(text, opts...))
			continue
		}

		if part.Text != nil {
			result = append(result, llmsdk.NewTextPart(*part.Text))
			continue
		}

		if part.InlineData != nil {
			if part.InlineData.MimeType != nil && part.InlineData.Data != nil {
				if strings.HasPrefix(*part.InlineData.MimeType, "image/") {
					result = append(result, llmsdk.NewImagePart(*part.InlineData.Data, *part.InlineData.MimeType))
					continue
				}

				if strings.HasPrefix(*part.InlineData.MimeType, "audio/") {
					format, err := partutil.MapMimeTypeToAudioFormat(*part.InlineData.MimeType)
					if err != nil {
						return nil, llmsdk.NewInvariantError(Provider, fmt.Sprintf("unsupported audio mime type: %s", *part.InlineData.MimeType))
					}
					result = append(result, llmsdk.NewAudioPart(*part.InlineData.Data, format))
					continue
				}
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
			opts := []llmsdk.ToolCallPartOption{}
			if part.ThoughtSignature != nil {
				opts = append(opts, llmsdk.WithToolCallThoughtSignature(*part.ThoughtSignature))
			}
			toolCallPart := llmsdk.NewToolCallPart(toolCallID, *part.FunctionCall.Name, json.RawMessage(part.FunctionCall.Args), opts...)
			toolCallPart.ToolCallPart.Args = part.FunctionCall.Args
			result = append(result, toolCallPart)
			continue
		}
	}

	return result, nil
}

// mapGoogleContentToDelta maps Google API content to content deltas for streaming
func mapGoogleContentToDelta(content googleapi.Content, existingContentDeltas []llmsdk.ContentDelta) ([]llmsdk.ContentDelta, error) {
	if len(content.Parts) == 0 {
		return []llmsdk.ContentDelta{}, nil
	}

	contentDeltas := []llmsdk.ContentDelta{}
	parts, err := mapGoogleContent(content.Parts)
	if err != nil {
		return nil, err
	}

	for _, part := range parts {
		partDelta := partutil.LooselyConvertPartToPartDelta(part)
		index := partutil.GuessDeltaIndex(partDelta, append(existingContentDeltas, contentDeltas...), nil)
		contentDeltas = append(contentDeltas, llmsdk.ContentDelta{
			Index: index,
			Part:  partDelta,
		})
	}

	return contentDeltas, nil
}

// mapGoogleUsageMetadata maps Google usage metadata to SDK usage
func mapGoogleUsageMetadata(usageMetadata googleapi.GenerateContentResponseUsageMetadata) *llmsdk.ModelUsage {
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
			case googleapi.MediaModalityText:
				if details.TextTokens == nil {
					details.TextTokens = ptr.To(0)
				}
				*details.TextTokens += *modalityTokenCount.TokenCount
			case googleapi.MediaModalityImage:
				if details.ImageTokens == nil {
					details.ImageTokens = ptr.To(0)
				}
				*details.ImageTokens += *modalityTokenCount.TokenCount
			case googleapi.MediaModalityAudio:
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
			case googleapi.MediaModalityText:
				if details.CachedTextTokens == nil {
					details.CachedTextTokens = ptr.To(0)
				}
				*details.CachedTextTokens += *cachedTokenCount.TokenCount
			case googleapi.MediaModalityImage:
				if details.CachedImageTokens == nil {
					details.CachedImageTokens = ptr.To(0)
				}
				*details.CachedImageTokens += *cachedTokenCount.TokenCount
			case googleapi.MediaModalityAudio:
				if details.CachedAudioTokens == nil {
					details.CachedAudioTokens = ptr.To(0)
				}
				*details.CachedAudioTokens += *cachedTokenCount.TokenCount
			}
		}
	}

	return &details
}
