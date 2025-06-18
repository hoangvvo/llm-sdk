package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/clientutils"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/sliceutils"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/tracing"
	"github.com/hoangvvo/llm-sdk/sdk-go/openai/openaiapi"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/partutil"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/stream"
)

// OpenAIChatModel implements the LanguageModel interface for OpenAI
type OpenAIChatModel struct {
	modelID  string
	apiKey   string
	baseURL  string
	client   *http.Client
	metadata *llmsdk.LanguageModelMetadata
}

type OpenAIChatModelOptions struct {
	BaseURL string
	APIKey  string
}

// NewOpenAIChatModel creates a new OpenAI model instance
func NewOpenAIChatModel(modelID string, options OpenAIChatModelOptions) *OpenAIChatModel {
	baseURL := options.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	return &OpenAIChatModel{
		modelID: modelID,
		apiKey:  options.APIKey,
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

func (m *OpenAIChatModel) WithMetadata(metadata *llmsdk.LanguageModelMetadata) *OpenAIChatModel {
	m.metadata = metadata
	return m
}

// Provider returns the provider name
func (m *OpenAIChatModel) Provider() string {
	return Provider
}

// ModelID returns the model ID
func (m *OpenAIChatModel) ModelID() string {
	return m.modelID
}

// Metadata returns the model capabilities
func (m *OpenAIChatModel) Metadata() *llmsdk.LanguageModelMetadata {
	return m.metadata
}

// Generate implements synchronous generation
func (m *OpenAIChatModel) Generate(ctx context.Context, input *llmsdk.LanguageModelInput) (*llmsdk.ModelResponse, error) {
	return tracing.TraceGenerate(ctx, Provider, m.modelID, input, func(ctx context.Context) (*llmsdk.ModelResponse, error) {
		params, err := convertToOpenAICreateParams(input, m.modelID)
		if err != nil {
			return nil, err
		}

		completion, err := clientutils.DoJSON[openaiapi.ChatCompletion](ctx, m.client, clientutils.JSONRequestConfig{
			URL:  fmt.Sprintf("%s/chat/completions", m.baseURL),
			Body: params,
			Headers: map[string]string{
				"Authorization": fmt.Sprintf("Bearer %s", m.apiKey),
			},
		})
		if err != nil {
			return nil, err
		}

		if len(completion.Choices) == 0 {
			return nil, llmsdk.NewInvariantError(m.Provider(), "no choices in response")
		}

		choice := completion.Choices[0]
		if choice.Message.Refusal != nil && *choice.Message.Refusal != "" {
			return nil, fmt.Errorf("request was refused: %s", *choice.Message.Refusal)
		}

		content, err := m.mapOpenAIMessage(choice.Message, params)
		if err != nil {
			return nil, err
		}

		var usage *llmsdk.ModelUsage
		if completion.Usage != nil {
			usage = &llmsdk.ModelUsage{
				InputTokens:  int(completion.Usage.PromptTokens),
				OutputTokens: int(completion.Usage.CompletionTokens),
			}
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

// Stream implements streaming generation
func (m *OpenAIChatModel) Stream(ctx context.Context, input *llmsdk.LanguageModelInput) (*llmsdk.LanguageModelStream, error) {
	return tracing.TraceStream(ctx, Provider, m.modelID, input, func(ctx context.Context) (*llmsdk.LanguageModelStream, error) {
		params, err := convertToOpenAICreateParams(input, m.modelID)
		if err != nil {
			return nil, err
		}
		params.Stream = ptr.To(true)

		sseStream, err := clientutils.DoSSE[openaiapi.ChatCompletionChunk](ctx, m.client, clientutils.SSERequestConfig{
			URL:  fmt.Sprintf("%s/chat/completions", m.baseURL),
			Body: params,
			Headers: map[string]string{
				"Authorization": fmt.Sprintf("Bearer %s", m.apiKey),
			},
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

			var allContentDeltas []llmsdk.ContentDelta

			for sseStream.Next() {
				chunk, err := sseStream.Current()
				if err != nil {
					errCh <- fmt.Errorf("failed to get sse chunk: %w", err)
					return
				}
				if chunk == nil {
					continue
				}

				if len(chunk.Choices) > 0 {
					choice := chunk.Choices[0]

					if choice.Delta.Refusal != nil && *choice.Delta.Refusal != "" {
						errCh <- llmsdk.NewRefusalError(*choice.Delta.Refusal)
						return
					}

					incomingDeltas := m.mapOpenAIDelta(choice.Delta, allContentDeltas, params)
					allContentDeltas = slices.Concat(allContentDeltas, incomingDeltas)

					for _, delta := range incomingDeltas {
						partial := &llmsdk.PartialModelResponse{Delta: &delta}
						responseCh <- partial
					}
				}

				if chunk.Usage != nil {
					usage := &llmsdk.ModelUsage{
						InputTokens:  int(chunk.Usage.PromptTokens),
						OutputTokens: int(chunk.Usage.CompletionTokens),
					}
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

// MARK: - Convert To OpenAI API Types

// convertToOpenAICreateParams converts LanguageModelInput to OpenAI parameters
func convertToOpenAICreateParams(input *llmsdk.LanguageModelInput, modelID string) (*openaiapi.ChatCompletionCreateParams, error) {
	messages, err := convertToOpenAIMessages(input.Messages, input.SystemPrompt)
	if err != nil {
		return nil, err
	}

	params := &openaiapi.ChatCompletionCreateParams{
		Model:            modelID,
		Messages:         messages,
		Temperature:      input.Temperature,
		TopP:             input.TopP,
		PresencePenalty:  input.PresencePenalty,
		FrequencyPenalty: input.FrequencyPenalty,
		Seed:             input.Seed,
		Extra:            input.Extra,
	}

	if input.MaxTokens != nil {
		params.MaxCompletionTokens = input.MaxTokens
	}

	if input.Tools != nil {
		var tools []openaiapi.ChatCompletionTool
		for _, tool := range input.Tools {
			openAITool := openaiapi.ChatCompletionTool{
				Function: &openaiapi.ChatCompletionFunctionTool{
					Function: openaiapi.FunctionDefinition{
						Name:        tool.Name,
						Description: &tool.Description,
						Parameters:  &tool.Parameters,
						Strict:      ptr.To(true),
					},
				},
			}
			tools = append(tools, openAITool)
		}
		params.Tools = tools
	}

	if input.ToolChoice != nil {
		params.ToolChoice = convertToOpenAIToolChoice(*input.ToolChoice)
	}

	if input.ResponseFormat != nil {
		params.ResponseFormat = convertToOpenAIResponseFormat(*input.ResponseFormat)
	}

	if input.Modalities != nil {
		modalities, err := sliceutils.MapErr(input.Modalities, func(modality llmsdk.Modality) (openaiapi.OpenAIModality, error) {
			return convertToOpenAIModality(modality)
		})
		if err != nil {
			return nil, err
		}
		params.Modalities = modalities
	}

	if input.Audio != nil {
		params.Audio, err = convertToOpenAIAudio(*input.Audio)
		if err != nil {
			return nil, err
		}
	}

	if input.Reasoning != nil && input.Reasoning.BudgetTokens != nil {
		switch OpenAIReasoningEffort(*input.Reasoning.BudgetTokens) {
		case OpenAIReasoningEffortMinimal:
			params.ReasoningEffort = ptr.To(openaiapi.ReasoningEffortMinimal)
		case OpenAIReasoningEffortLow:
			params.ReasoningEffort = ptr.To(openaiapi.ReasoningEffortLow)
		case OpenAIReasoningEffortMedium:
			params.ReasoningEffort = ptr.To(openaiapi.ReasoningEffortMedium)
		case OpenAIReasoningEffortHigh:
			params.ReasoningEffort = ptr.To(openaiapi.ReasoningEffortHigh)
		default:
			return nil, llmsdk.NewUnsupportedError(Provider, "Budget tokens property is not supported for OpenAI reasoning. You may use OpenAIChatCompletionReasoningEffort enum values to map it to OpenAI reasoning effort levels.")
		}
	}

	return params, nil
}

// MARK: - To Provider Messages

// convertToOpenAIMessages converts messages to OpenAI format
func convertToOpenAIMessages(messages []llmsdk.Message, systemPrompt *string) ([]openaiapi.ChatCompletionMessageParam, error) {
	var openAIMessages []openaiapi.ChatCompletionMessageParam

	// Add system prompt if provided
	if systemPrompt != nil && *systemPrompt != "" {
		openAIMessages = append(openAIMessages, openaiapi.ChatCompletionMessageParam{
			System: &openaiapi.ChatCompletionSystemMessageParam{
				Content: []openaiapi.SystemContentPart{
					{
						Text: &openaiapi.ChatCompletionContentPartText{
							Text: *systemPrompt,
						},
					},
				},
			},
		})
	}

	for _, message := range messages {
		switch {
		case message.UserMessage != nil:
			var content []openaiapi.ChatCompletionContentPart

			messageParts := partutil.GetCompatiblePartsWithoutSourceParts(message.UserMessage.Content)

			for _, part := range messageParts {
				openAIPart, err := convertToOpenAIContentPart(part)
				if err != nil {
					return nil, err
				}
				content = append(content, *openAIPart)
			}

			openAIMessages = append(openAIMessages, openaiapi.ChatCompletionMessageParam{
				User: &openaiapi.ChatCompletionUserMessageParam{
					Content: content,
				},
			})

		case message.AssistantMessage != nil:
			assistantMsg := &openaiapi.ChatCompletionAssistantMessageParam{}

			messageParts := partutil.GetCompatiblePartsWithoutSourceParts(message.AssistantMessage.Content)

			for _, part := range messageParts {
				switch {
				case part.TextPart != nil:
					if assistantMsg.Content == nil {
						assistantMsg.Content = []openaiapi.AssistantContentPart{}
					}
					assistantMsg.Content = append(assistantMsg.Content, openaiapi.AssistantContentPart{
						Text: &openaiapi.ChatCompletionContentPartText{
							Text: part.TextPart.Text,
						},
					})

				case part.ToolCallPart != nil:
					if assistantMsg.ToolCalls == nil {
						assistantMsg.ToolCalls = []openaiapi.ChatCompletionMessageToolCall{}
					}

					args, _ := json.Marshal(part.ToolCallPart.Args)
					toolCall := openaiapi.ChatCompletionMessageToolCall{
						Function: &openaiapi.ChatCompletionMessageFunctionToolCall{
							ID: part.ToolCallPart.ToolCallID,
							Function: openaiapi.ChatCompletionMessageFunctionToolCallFunction{
								Name:      part.ToolCallPart.ToolName,
								Arguments: string(args),
							},
						},
					}
					assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, toolCall)

				case part.AudioPart != nil:
					if part.AudioPart.ID != nil {
						assistantMsg.Audio = &openaiapi.ChatCompletionAssistantMessageParamAudio{
							ID: *part.AudioPart.ID,
						}
					}
				}
			}

			openAIMessages = append(openAIMessages, openaiapi.ChatCompletionMessageParam{
				Assistant: assistantMsg,
			})

		case message.ToolMessage != nil:
			for _, part := range message.ToolMessage.Content {
				if part.ToolResultPart == nil {
					return nil, fmt.Errorf("tool message must only contain tool result parts")
				}

				toolResultPartContent := partutil.GetCompatiblePartsWithoutSourceParts(part.ToolResultPart.Content)

				var content []openaiapi.ChatCompletionToolMessageParamToolContentPart
				for _, contentPart := range toolResultPartContent {
					toolContentPart, err := convertToOpenAIToolMessageParamContent(contentPart)
					if err != nil {
						return nil, err
					}
					content = append(content, *toolContentPart)
				}

				openAIMessages = append(openAIMessages, openaiapi.ChatCompletionMessageParam{
					Tool: &openaiapi.ChatCompletionToolMessageParam{
						ToolCallID: part.ToolResultPart.ToolCallID,
						Content:    content,
					},
				})
			}
		}
	}

	return openAIMessages, nil
}

// convertToOpenAIContentPart converts a Part to OpenAI ChatCompletionContentPart
func convertToOpenAIContentPart(part llmsdk.Part) (*openaiapi.ChatCompletionContentPart, error) {
	switch {
	case part.TextPart != nil:
		return &openaiapi.ChatCompletionContentPart{
			Text: convertToOpenAIContentPartText(part.TextPart),
		}, nil

	case part.ImagePart != nil:
		return &openaiapi.ChatCompletionContentPart{
			Image: convertToOpenAIContentPartImage(part.ImagePart),
		}, nil

	case part.AudioPart != nil:
		inputAudio, err := convertToOpenAIContentPartInputAudio(part.AudioPart)
		if err != nil {
			return nil, err
		}
		return &openaiapi.ChatCompletionContentPart{
			InputAudio: inputAudio,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported part type for OpenAI: %s", part.Type())
	}
}

func convertToOpenAIContentPartText(textPart *llmsdk.TextPart) *openaiapi.ChatCompletionContentPartText {
	return &openaiapi.ChatCompletionContentPartText{
		Text: textPart.Text,
	}
}

func convertToOpenAIContentPartImage(imagePart *llmsdk.ImagePart) *openaiapi.ChatCompletionContentPartImage {
	return &openaiapi.ChatCompletionContentPartImage{
		ImageURL: openaiapi.ChatCompletionContentPartImageImageURL{
			URL: fmt.Sprintf("data:%s;base64,%s", imagePart.MimeType, imagePart.ImageData),
		},
	}
}

func convertToOpenAIContentPartInputAudio(audioPart *llmsdk.AudioPart) (*openaiapi.ChatCompletionContentPartInputAudio, error) {
	var format openaiapi.AudioInputFormat
	switch audioPart.Format {
	case llmsdk.AudioFormatWav:
		format = openaiapi.AudioInputFormatWav
	case llmsdk.AudioFormatMP3:
		format = openaiapi.AudioInputFormatMp3
	default:
		return nil, fmt.Errorf("unsupported audio format for OpenAI: %s", audioPart.Format)
	}

	return &openaiapi.ChatCompletionContentPartInputAudio{
		InputAudio: openaiapi.ChatCompletionContentPartInputAudioInputAudio{
			Data:   audioPart.AudioData,
			Format: format,
		},
	}, nil
}

func convertToOpenAIToolMessageParamContent(part llmsdk.Part) (*openaiapi.ChatCompletionToolMessageParamToolContentPart, error) {
	if part.TextPart != nil {
		return &openaiapi.ChatCompletionToolMessageParamToolContentPart{
			Text: convertToOpenAIContentPartText(part.TextPart),
		}, nil
	}
	return nil, fmt.Errorf("cannot convert part to OpenAI tool message for type: %s", part.Type())
}

// MARK: - To Provider Tools

// convertToOpenAIToolChoice converts ToolChoiceOption to OpenAI format
func convertToOpenAIToolChoice(toolChoice llmsdk.ToolChoiceOption) *openaiapi.ChatCompletionToolChoiceOption {
	if toolChoice.None != nil {
		return &openaiapi.ChatCompletionToolChoiceOption{None: ptr.To(true)}
	}
	if toolChoice.Auto != nil {
		return &openaiapi.ChatCompletionToolChoiceOption{Auto: ptr.To(true)}
	}
	if toolChoice.Required != nil {
		return &openaiapi.ChatCompletionToolChoiceOption{Required: ptr.To(true)}
	}
	if toolChoice.Tool != nil {
		return &openaiapi.ChatCompletionToolChoiceOption{
			Named: &openaiapi.ChatCompletionNamedToolChoice{
				Function: openaiapi.ChatCompletionNamedToolChoiceFunction{
					Name: toolChoice.Tool.ToolName,
				},
				Type: "function",
			},
		}
	}
	return nil
}

// MARK: - To Provider Response Format

// convertToOpenAIResponseFormat converts ResponseFormatOption to OpenAI format
func convertToOpenAIResponseFormat(responseFormat llmsdk.ResponseFormatOption) *openaiapi.OpenAIResponseFormat {
	if responseFormat.Text != nil {
		return &openaiapi.OpenAIResponseFormat{Text: ptr.To(true)}
	}

	if responseFormat.JSON != nil {
		if responseFormat.JSON.Schema != nil {
			return &openaiapi.OpenAIResponseFormat{
				JsonSchema: &openaiapi.ResponseFormatJSONSchema{
					JsonSchema: openaiapi.ResponseFormatJSONSchemaJSONSchema{
						Name:        responseFormat.JSON.Name,
						Description: responseFormat.JSON.Description,
						Schema:      responseFormat.JSON.Schema,
						Strict:      ptr.To(true),
					},
				},
			}
		} else {
			return &openaiapi.OpenAIResponseFormat{JsonObject: ptr.To(true)}
		}
	}
	return nil
}

// MARK: - To Provider Modality

// convertToOpenAIModality converts SDK modality to OpenAI format
func convertToOpenAIModality(modality llmsdk.Modality) (openaiapi.OpenAIModality, error) {
	switch modality {
	case llmsdk.ModalityText:
		return openaiapi.OpenAIModalityText, nil
	case llmsdk.ModalityAudio:
		return openaiapi.OpenAIModalityAudio, nil
	default:
		return "", llmsdk.NewUnsupportedError(Provider, fmt.Sprintf("cannot convert modality to OpenAI modality for modality %s", modality))
	}
}

func convertToOpenAIAudio(audio llmsdk.AudioOptions) (*openaiapi.ChatCompletionAudioParam, error) {
	if audio.Voice == nil {
		return nil, llmsdk.NewInvalidInputError("Audio voice is required for OpenAI audio generation")
	}

	var format openaiapi.AudioOutputFormat
	switch *audio.Format {
	case llmsdk.AudioFormatWav:
		format = openaiapi.AudioOutputFormatWav
	case llmsdk.AudioFormatMP3:
		format = openaiapi.AudioOutputFormatMp3
	case llmsdk.AudioFormatFLAC:
		format = openaiapi.AudioOutputFormatFlac
	case llmsdk.AudioFormatAAC:
		format = openaiapi.AudioOutputFormatAac
	case llmsdk.AudioFormatOpus:
		format = openaiapi.AudioOutputFormatOpus
	case llmsdk.AudioFormatLinear16:
		format = openaiapi.AudioOutputFormatPcm16
	default:
		return nil, llmsdk.NewInvalidInputError(fmt.Sprintf("unsupported audio format for OpenAI: %s", *audio.Format))
	}

	audioParam := &openaiapi.ChatCompletionAudioParam{
		Format: format,
		Voice:  *audio.Voice,
	}

	return audioParam, nil
}

// MARK: - Map From OpenAI API Types

// mapOpenAIMessage converts OpenAI message to SDK parts
func (m *OpenAIChatModel) mapOpenAIMessage(message openaiapi.ChatCompletionMessage, createParams *openaiapi.ChatCompletionCreateParams) ([]llmsdk.Part, error) {
	var parts []llmsdk.Part

	if message.Content != nil && *message.Content != "" {
		parts = append(parts, llmsdk.NewTextPart(*message.Content))
	}

	for _, toolCall := range message.ToolCalls {
		if toolCall.Function != nil {
			var args map[string]any
			if err := json.Unmarshal([]byte(toolCall.Function.Function.Arguments), &args); err != nil {
				return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
			}

			parts = append(parts, llmsdk.NewToolCallPart(
				toolCall.Function.ID,
				toolCall.Function.Function.Name,
				args,
			))
		}
	}

	if message.Audio != nil {
		audioFormat := llmsdk.AudioFormatLinear16
		if createParams.Audio != nil {
			audioFormat = mapOpenAIAudioFormat(createParams.Audio.Format)
		}

		audioPart := llmsdk.NewAudioPart(
			message.Audio.Data,
			audioFormat,
			llmsdk.WithAudioSampleRate(OpenAIAudioSampleRate),
			llmsdk.WithAudioChannels(OpenAIAudioChannels),
			llmsdk.WithAudioTranscript(message.Audio.Transcript),
			llmsdk.WithAudioID(message.Audio.ID),
		)
		parts = append(parts, audioPart)
	}

	return parts, nil
}

// mapOpenAIAudioFormat converts OpenAI audio format to SDK format
func mapOpenAIAudioFormat(format openaiapi.AudioOutputFormat) llmsdk.AudioFormat {
	switch format {
	case openaiapi.AudioOutputFormatWav:
		return llmsdk.AudioFormatWav
	case openaiapi.AudioOutputFormatMp3:
		return llmsdk.AudioFormatMP3
	case openaiapi.AudioOutputFormatAac:
		return llmsdk.AudioFormatAAC
	case openaiapi.AudioOutputFormatFlac:
		return llmsdk.AudioFormatFLAC
	case openaiapi.AudioOutputFormatOpus:
		return llmsdk.AudioFormatOpus
	case openaiapi.AudioOutputFormatPcm16:
		return llmsdk.AudioFormatLinear16
	default:
		return llmsdk.AudioFormatLinear16
	}
}

// MARK: - To SDK Delta

// mapOpenAIDelta converts OpenAI delta to SDK content deltas
func (m *OpenAIChatModel) mapOpenAIDelta(delta openaiapi.ChatCompletionChunkChoiceDelta, existingDeltas []llmsdk.ContentDelta, createParams *openaiapi.ChatCompletionCreateParams) []llmsdk.ContentDelta {
	var contentDeltas []llmsdk.ContentDelta

	if delta.Content != nil && *delta.Content != "" {
		textDelta := llmsdk.TextPartDelta{
			Text: *delta.Content,
		}

		// Find the appropriate index for text content
		index := partutil.GuessDeltaIndex(llmsdk.PartDelta{TextPartDelta: &textDelta}, slices.Concat(existingDeltas, contentDeltas), nil)

		contentDeltas = append(contentDeltas, llmsdk.ContentDelta{
			Index: index,
			Part:  llmsdk.PartDelta{TextPartDelta: &textDelta},
		})
	}

	if delta.Audio != nil {
		audioDelta := llmsdk.AudioPartDelta{}

		if delta.Audio.ID != nil {
			audioDelta.ID = delta.Audio.ID
		}
		if delta.Audio.Data != nil {
			audioDelta.AudioData = delta.Audio.Data
			if createParams.Audio != nil {
				format := mapOpenAIAudioFormat(createParams.Audio.Format)
				audioDelta.Format = &format
			}
			if audioDelta.Format != nil && *audioDelta.Format == llmsdk.AudioFormatLinear16 {
				audioDelta.SampleRate = ptr.To(OpenAIAudioSampleRate)
				audioDelta.Channels = ptr.To(OpenAIAudioChannels)
			}
		}
		if delta.Audio.Transcript != nil {
			audioDelta.Transcript = delta.Audio.Transcript
		}

		index := partutil.GuessDeltaIndex(llmsdk.PartDelta{AudioPartDelta: &audioDelta}, slices.Concat(existingDeltas, contentDeltas), nil)

		contentDeltas = append(contentDeltas, llmsdk.ContentDelta{
			Index: index,
			Part:  llmsdk.PartDelta{AudioPartDelta: &audioDelta},
		})
	}

	for _, toolCall := range delta.ToolCalls {
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

		var indexHint *int
		if toolCall.Index >= 0 {
			indexHint = &toolCall.Index
		}

		index := partutil.GuessDeltaIndex(llmsdk.PartDelta{ToolCallPartDelta: &toolCallDelta}, slices.Concat(existingDeltas, contentDeltas), indexHint)

		contentDeltas = append(contentDeltas, llmsdk.ContentDelta{
			Index: index,
			Part:  llmsdk.PartDelta{ToolCallPartDelta: &toolCallDelta},
		})
	}

	return contentDeltas
}
