package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/clientutils"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/tracing"
	"github.com/hoangvvo/llm-sdk/sdk-go/openai/openaiapi"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/partutil"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/randutil"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/stream"
)

// OpenAIModel implements the LanguageModel interface for OpenAI using Responses API
type OpenAIModel struct {
	modelID  string
	apiKey   string
	baseURL  string
	client   *http.Client
	metadata *llmsdk.LanguageModelMetadata
	headers  map[string]string
}

// OpenAIModelOptions represents configuration options for OpenAI model
type OpenAIModelOptions struct {
	BaseURL    string
	APIKey     string
	Headers    map[string]string
	HTTPClient *http.Client
}

type OpenAIReasoningEffort uint32

const (
	OpenAIReasoningEffortMinimal OpenAIReasoningEffort = 1000
	OpenAIReasoningEffortLow     OpenAIReasoningEffort = 2000
	OpenAIReasoningEffortMedium  OpenAIReasoningEffort = 3000
	OpenAIReasoningEffortHigh    OpenAIReasoningEffort = 4000
)

func NewOpenAIModel(modelID string, options OpenAIModelOptions) *OpenAIModel {
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

	return &OpenAIModel{
		modelID: modelID,
		apiKey:  options.APIKey,
		baseURL: baseURL,
		client:  client,
		headers: headers,
	}
}

func (m *OpenAIModel) WithMetadata(metadata *llmsdk.LanguageModelMetadata) *OpenAIModel {
	m.metadata = metadata
	return m
}

// Provider returns the provider name
func (m *OpenAIModel) Provider() string {
	return Provider
}

// ModelID returns the model ID
func (m *OpenAIModel) ModelID() string {
	return m.modelID
}

// Metadata returns the model capabilities
func (m *OpenAIModel) Metadata() *llmsdk.LanguageModelMetadata {
	return m.metadata
}

func (m *OpenAIModel) requestHeaders() map[string]string {
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", m.apiKey),
	}

	for k, v := range m.headers {
		headers[k] = v
	}

	return headers
}

// Generate implements synchronous generation
func (m *OpenAIModel) Generate(ctx context.Context, input *llmsdk.LanguageModelInput) (*llmsdk.ModelResponse, error) {
	return tracing.TraceGenerate(ctx, Provider, m.modelID, input, func(ctx context.Context) (*llmsdk.ModelResponse, error) {
		params, err := convertToResponseCreateParams(input, m.modelID)
		if err != nil {
			return nil, err
		}

		params.Stream = ptr.To(false)

		response, err := clientutils.DoJSON[openaiapi.Response](ctx, m.client, clientutils.JSONRequestConfig{
			URL:     fmt.Sprintf("%s/responses", m.baseURL),
			Body:    params,
			Headers: m.requestHeaders(),
		})
		if err != nil {
			return nil, err
		}

		content, err := mapOpenAIOutputItems(response.Output)
		if err != nil {
			return nil, err
		}

		var usage *llmsdk.ModelUsage
		if response.Usage != nil {
			usage = mapOpenAIUsage(*response.Usage)
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
func (m *OpenAIModel) Stream(ctx context.Context, input *llmsdk.LanguageModelInput) (*llmsdk.LanguageModelStream, error) {
	return tracing.TraceStream(ctx, Provider, m.modelID, input, func(ctx context.Context) (*llmsdk.LanguageModelStream, error) {
		params, err := convertToResponseCreateParams(input, m.modelID)
		if err != nil {
			return nil, err
		}
		params.Stream = ptr.To(true)

		sseStream, err := clientutils.DoSSE[openaiapi.ResponseStreamEvent](ctx, m.client, clientutils.SSERequestConfig{
			URL:     fmt.Sprintf("%s/responses", m.baseURL),
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

			refusal := ""

			for sseStream.Next() {
				streamEvent, err := sseStream.Current()
				if err != nil {
					errCh <- fmt.Errorf("failed to get sse event: %w", err)
					return
				}
				if streamEvent == nil {
					continue
				}

				if streamEvent.ResponseRefusalDeltaEvent != nil {
					refusal += streamEvent.ResponseRefusalDeltaEvent.Delta
				}

				partDelta, err := mapOpenAIStreamEvent(*streamEvent)
				if err != nil {
					errCh <- fmt.Errorf("failed to map stream event: %w", err)
					return
				}

				if partDelta != nil {
					responseCh <- &llmsdk.PartialModelResponse{Delta: partDelta}
				}

				if streamEvent.ResponseCompletedEvent != nil {
					if streamEvent.ResponseCompletedEvent.Response.Usage != nil {
						usage := mapOpenAIUsage(*streamEvent.ResponseCompletedEvent.Response.Usage)
						partial := &llmsdk.PartialModelResponse{Usage: usage}
						if m.metadata != nil && m.metadata.Pricing != nil {
							partial.Cost = ptr.To(usage.CalculateCost(m.metadata.Pricing))
						}
						responseCh <- partial
					}
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

// MARK: - Convert To OpenAI API Types

func convertToResponseCreateParams(input *llmsdk.LanguageModelInput, modelID string) (*openaiapi.ResponseCreateParams, error) {
	inputItems, err := convertToOpenAIInputs(input.Messages)
	if err != nil {
		return nil, err
	}

	params := &openaiapi.ResponseCreateParams{
		Store:           ptr.To(false),
		Model:           ptr.To(modelID),
		Instructions:    input.SystemPrompt,
		Input:           inputItems,
		Temperature:     input.Temperature,
		TopP:            input.TopP,
		MaxOutputTokens: input.MaxTokens,
		Reasoning: &openaiapi.Reasoning{
			Summary: ptr.To("auto"),
		},
	}

	if input.Tools != nil {
		var tools []openaiapi.Tool
		for _, tool := range input.Tools {
			openAITool := openaiapi.Tool{
				FunctionTool: &openaiapi.FunctionTool{
					Name:        tool.Name,
					Description: &tool.Description,
					Parameters:  tool.Parameters,
					Strict:      ptr.To(true),
				},
			}
			tools = append(tools, openAITool)
		}
		params.Tools = tools
	}

	if input.ToolChoice != nil {
		params.ToolChoice = convertToOpenAIResponseToolChoice(*input.ToolChoice)
	}

	if input.ResponseFormat != nil {
		params.Text = convertToOpenAIResponseTextConfig(*input.ResponseFormat)
	}

	if input.Modalities != nil {
		if slices.Contains(input.Modalities, llmsdk.ModalityImage) {
			params.Tools = append(params.Tools, openaiapi.Tool{
				ToolImageGeneration: &openaiapi.ToolImageGeneration{},
			})
		}
	}

	if input.Reasoning != nil {
		params.Include = []openaiapi.ResponseIncludable{
			openaiapi.ResponseIncludableReasoningEncryptedContent,
		}
		params.Reasoning, err = convertToOpenAIReasoning(*input.Reasoning)
		if err != nil {
			return nil, err
		}
	}

	return params, nil
}

// MARK: - To Provider Messages

func convertToOpenAIInputs(messages []llmsdk.Message) ([]openaiapi.ResponseInputItem, error) {
	var inputItems []openaiapi.ResponseInputItem

	for _, message := range messages {
		switch {
		case message.UserMessage != nil:
			inputItem, err := convertUserMessageToOpenAIInputItem(message.UserMessage)
			if err != nil {
				return nil, err
			}
			inputItems = append(inputItems, inputItem)

		case message.AssistantMessage != nil:
			items, err := convertAssistantMessageToOpenAIInputItems(message.AssistantMessage)
			if err != nil {
				return nil, err
			}
			inputItems = append(inputItems, items...)

		case message.ToolMessage != nil:
			items, err := convertToolMessageToOpenAIInputItems(message.ToolMessage)
			if err != nil {
				return nil, err
			}
			inputItems = append(inputItems, items...)
		}
	}

	return inputItems, nil
}

func convertUserMessageToOpenAIInputItem(userMessage *llmsdk.UserMessage) (openaiapi.ResponseInputItem, error) {
	messageParts := partutil.GetCompatiblePartsWithoutSourceParts(userMessage.Content)
	var content []openaiapi.ResponseInputContent

	for _, part := range messageParts {
		inputContent, err := convertToOpenAIResponseInputContent(part)
		if err != nil {
			return openaiapi.ResponseInputItem{}, err
		}
		content = append(content, *inputContent)
	}

	return openaiapi.ResponseInputItem{
		ResponseInputItemMessage: &openaiapi.ResponseInputItemMessage{
			Role:    "user",
			Content: openaiapi.ResponseInputMessageContentList(content),
		},
	}, nil
}

func convertAssistantMessageToOpenAIInputItems(assistantMessage *llmsdk.AssistantMessage) ([]openaiapi.ResponseInputItem, error) {
	messageParts := partutil.GetCompatiblePartsWithoutSourceParts(assistantMessage.Content)
	var inputItems []openaiapi.ResponseInputItem

	for _, part := range messageParts {
		switch {
		case part.TextPart != nil:
			inputItems = append(inputItems, openaiapi.ResponseInputItem{
				ResponseOutputMessage: &openaiapi.ResponseOutputMessage{
					// Response output item requires an ID.
					// This usually applies if we enable OpenAI "store".
					// or that we propogate the message ID in output.
					// For compatibility, we want to avoid doing that, so we use a generated ID
					// to avoid the API from returning an error.
					ID:     fmt.Sprintf("msg_%s", randutil.String(15)),
					Role:   "assistant",
					Status: "completed",
					Content: []openaiapi.ResponseOutputContent{{
						ResponseOutputText: &openaiapi.ResponseOutputText{
							Text:        part.TextPart.Text,
							Annotations: []openaiapi.ResponseOutputTextAnnotation{},
						},
					}},
				},
			})

		case part.ReasoningPart != nil:
			id := ""
			if part.ReasoningPart.ID != nil {
				id = *part.ReasoningPart.ID
			}
			inputItems = append(inputItems, openaiapi.ResponseInputItem{
				ResponseReasoningItem: &openaiapi.ResponseReasoningItem{
					// Similar to assistant message parts, we generate a unique ID for each reasoning part.
					ID: id,
					Summary: []openaiapi.ResponseReasoningItemSummaryUnion{
						{
							ResponseReasoningItemSummary: &openaiapi.ResponseReasoningItemSummary{
								Text: part.ReasoningPart.Text,
							},
						},
					},
					EncryptedContent: part.ReasoningPart.Signature,
				},
			})

		case part.ImagePart != nil:
			id := ""
			if part.ImagePart.ID != nil {
				id = *part.ImagePart.ID
			}
			inputItems = append(inputItems, openaiapi.ResponseInputItem{
				ResponseOutputItemImageGenerationCall: &openaiapi.ResponseOutputItemImageGenerationCall{
					ID:     id,
					Status: "completed",
					Result: ptr.To(fmt.Sprintf("data:%s;base64,%s", part.ImagePart.MimeType, part.ImagePart.Data)),
				},
			})

		case part.ToolCallPart != nil:
			args, _ := json.Marshal(part.ToolCallPart.Args)
			inputItems = append(inputItems, openaiapi.ResponseInputItem{
				ResponseFunctionToolCall: &openaiapi.ResponseFunctionToolCall{
					Arguments: string(args),
					CallID:    part.ToolCallPart.ToolCallID,
					Name:      part.ToolCallPart.ToolName,
					ID:        part.ToolCallPart.ID,
				},
			})

		default:
			return nil, llmsdk.NewUnsupportedError(Provider, fmt.Sprintf("cannot convert assistant message part to OpenAI ResponseInputItem for type %s", part.Type()))
		}
	}

	return inputItems, nil
}

func convertToolMessageToOpenAIInputItems(toolMessage *llmsdk.ToolMessage) ([]openaiapi.ResponseInputItem, error) {
	var inputItems []openaiapi.ResponseInputItem
	for _, part := range toolMessage.Content {
		if part.ToolResultPart == nil {
			return nil, fmt.Errorf("tool messages must contain only tool result parts")
		}

		toolResultPartContent := partutil.GetCompatiblePartsWithoutSourceParts(part.ToolResultPart.Content)
		for _, toolResultPart := range toolResultPartContent {
			switch {
			case toolResultPart.TextPart != nil:
				inputItems = append(inputItems, openaiapi.ResponseInputItem{
					ResponseInputItemFunctionCallOutput: &openaiapi.ResponseInputItemFunctionCallOutput{
						CallID: part.ToolResultPart.ToolCallID,
						Output: toolResultPart.TextPart.Text,
					},
				})
			default:
				return nil, fmt.Errorf("cannot convert tool result part to OpenAI ResponseInputItem for type %s", toolResultPart.Type())
			}
		}
	}
	return inputItems, nil
}

func convertToOpenAIResponseInputContent(part llmsdk.Part) (*openaiapi.ResponseInputContent, error) {
	switch {
	case part.TextPart != nil:
		return &openaiapi.ResponseInputContent{
			ResponseInputText: &openaiapi.ResponseInputText{
				Text: part.TextPart.Text,
			},
		}, nil

	case part.ImagePart != nil:
		return &openaiapi.ResponseInputContent{
			ResponseInputImage: &openaiapi.ResponseInputImage{
				Detail:   "auto",
				ImageURL: ptr.To(fmt.Sprintf("data:%s;base64,%s", part.ImagePart.MimeType, part.ImagePart.Data)),
			},
		}, nil

	case part.AudioPart != nil:
		var format string
		switch part.AudioPart.Format {
		case llmsdk.AudioFormatMP3:
			format = "mp3"
		case llmsdk.AudioFormatWav:
			format = "wav"
		default:
			return nil, llmsdk.NewUnsupportedError(Provider, fmt.Sprintf("cannot convert audio format to OpenAI InputAudio format for format %s", part.AudioPart.Format))
		}

		return &openaiapi.ResponseInputContent{
			ResponseInputAudio: &openaiapi.ResponseInputAudio{
				InputAudio: openaiapi.ResponseInputAudioInputAudio{
					Data:   part.AudioPart.Data,
					Format: format,
				},
			},
		}, nil

	default:
		return nil, llmsdk.NewUnsupportedError(Provider, fmt.Sprintf("cannot convert part to OpenAI content part for type %s", part.Type()))
	}
}

// MARK: - To Provider Tools

func convertToOpenAIResponseToolChoice(toolChoice llmsdk.ToolChoiceOption) *openaiapi.ToolChoice {
	choice := &openaiapi.ToolChoice{}
	if toolChoice.Auto != nil {
		opt := openaiapi.ToolChoiceOptionsAuto
		choice.Options = &opt
		return choice
	}
	if toolChoice.None != nil {
		opt := openaiapi.ToolChoiceOptionsNone
		choice.Options = &opt
		return choice
	}
	if toolChoice.Required != nil {
		opt := openaiapi.ToolChoiceOptionsRequired
		choice.Options = &opt
		return choice
	}
	if toolChoice.Tool != nil {
		choice.Function = &openaiapi.ToolChoiceFunction{
			Type: "function",
			Name: toolChoice.Tool.ToolName,
		}
		return choice
	}
	return nil
}

// MARK: - To Provider Response Format

func convertToOpenAIResponseTextConfig(responseFormat llmsdk.ResponseFormatOption) *openaiapi.ResponseTextConfig {
	if responseFormat.Text != nil {
		return &openaiapi.ResponseTextConfig{
			Format: &openaiapi.ResponseFormatTextConfig{
				ResponseFormatText: &openaiapi.ResponseFormatText{},
			},
		}
	}

	if responseFormat.JSON != nil {
		if responseFormat.JSON.Schema != nil {
			return &openaiapi.ResponseTextConfig{
				Format: &openaiapi.ResponseFormatTextConfig{
					ResponseFormatTextJSONSchemaConfig: &openaiapi.ResponseFormatTextJSONSchemaConfig{
						Name:        responseFormat.JSON.Name,
						Schema:      *responseFormat.JSON.Schema,
						Description: responseFormat.JSON.Description,
						Strict:      ptr.To(true),
					},
				},
			}
		}
		return &openaiapi.ResponseTextConfig{
			Format: &openaiapi.ResponseFormatTextConfig{
				ResponseFormatJSONObject: &openaiapi.ResponseFormatJSONObject{},
			},
		}
	}
	return nil
}

func convertToOpenAIReasoning(reasoning llmsdk.ReasoningOptions) (*openaiapi.Reasoning, error) {
	openaiReasoning := &openaiapi.Reasoning{}
	if reasoning.Enabled {
		openaiReasoning.Summary = ptr.To("auto")
	}
	if reasoning.BudgetTokens != nil {
		switch OpenAIReasoningEffort(*reasoning.BudgetTokens) {
		case OpenAIReasoningEffortMinimal:
			openaiReasoning.Effort = ptr.To(openaiapi.ReasoningEffortMinimal)
		case OpenAIReasoningEffortLow:
			openaiReasoning.Effort = ptr.To(openaiapi.ReasoningEffortLow)
		case OpenAIReasoningEffortMedium:
			openaiReasoning.Effort = ptr.To(openaiapi.ReasoningEffortMedium)
		case OpenAIReasoningEffortHigh:
			openaiReasoning.Effort = ptr.To(openaiapi.ReasoningEffortHigh)
		default:
			return nil, llmsdk.NewUnsupportedError(Provider, "Budget tokens property is not supported for OpenAI reasoning. You may use OpenAIReasoningEffort enum values to map it to OpenAI reasoning effort levels.")
		}
	}
	return openaiReasoning, nil
}

// MARK: - To SDK Message

func mapOpenAIOutputItems(items []openaiapi.ResponseOutputItem) ([]llmsdk.Part, error) {
	var parts []llmsdk.Part

	for _, item := range items {
		switch {
		case item.ResponseOutputMessage != nil:
			for _, content := range item.ResponseOutputMessage.Content {
				switch {
				case content.ResponseOutputText != nil:
					parts = append(parts, llmsdk.NewTextPart(content.ResponseOutputText.Text))
				case content.ResponseOutputRefusal != nil:
					return nil, llmsdk.NewRefusalError(content.ResponseOutputRefusal.Refusal)
				}
			}

		case item.ResponseFunctionToolCall != nil:
			var args map[string]any
			if err := json.Unmarshal([]byte(item.ResponseFunctionToolCall.Arguments), &args); err != nil {
				return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
			}

			toolCallPart := llmsdk.NewToolCallPart(
				item.ResponseFunctionToolCall.CallID,
				item.ResponseFunctionToolCall.Name,
				args,
			)
			toolCallPart.ToolCallPart.ID = item.ResponseFunctionToolCall.ID
			parts = append(parts, toolCallPart)

		case item.ResponseOutputItemImageGenerationCall != nil:
			responseOutputItemImageGenerationCall := item.ResponseOutputItemImageGenerationCall
			if responseOutputItemImageGenerationCall.Result == nil {
				return nil, llmsdk.NewInvariantError(Provider, "image generation call did not return a result")
			}

			var width, height *int
			if responseOutputItemImageGenerationCall.Size != "" {
				width, height = parseOpenAIImageSize(responseOutputItemImageGenerationCall.Size)
			}

			imageOpts := []llmsdk.ImagePartOption{}
			if width != nil {
				imageOpts = append(imageOpts, llmsdk.WithImageWidth(*width))
			}
			if height != nil {
				imageOpts = append(imageOpts, llmsdk.WithImageHeight(*height))
			}
			imageOpts = append(imageOpts, llmsdk.WithImageID(responseOutputItemImageGenerationCall.ID))
			parts = append(parts, llmsdk.NewImagePart(
				*responseOutputItemImageGenerationCall.Result,
				fmt.Sprintf("image/%s", responseOutputItemImageGenerationCall.OutputFormat),
				imageOpts...,
			))

		case item.ResponseReasoningItem != nil:
			var summary = ""
			for _, s := range item.ResponseReasoningItem.Summary {
				if s.ResponseReasoningItemSummary != nil {
					summary += s.ResponseReasoningItemSummary.Text + "\n"
				}
			}

			reasoningOpts := []llmsdk.ReasoningPartOption{}
			if item.ResponseReasoningItem.EncryptedContent != nil {
				reasoningOpts = append(reasoningOpts, llmsdk.WithReasoningSignature(*item.ResponseReasoningItem.EncryptedContent))
			}
			reasoningOpts = append(reasoningOpts, llmsdk.WithReasoningID(item.ResponseReasoningItem.ID))
			parts = append(parts, llmsdk.NewReasoningPart(summary, reasoningOpts...))
		}
	}

	return parts, nil
}

// MARK: - To SDK Delta

func mapOpenAIStreamEvent(event openaiapi.ResponseStreamEvent) (*llmsdk.ContentDelta, error) {
	switch {
	case event.ResponseFailedEvent != nil:
		// Handle failed response - convert to error
		return nil, llmsdk.NewInvariantError(Provider, "stream event failed")

	case event.ResponseOutputItemAddedEvent != nil:
		item := event.ResponseOutputItemAddedEvent.Item

		if item.ResponseFunctionToolCall != nil {
			return &llmsdk.ContentDelta{
				Index: event.ResponseOutputItemAddedEvent.OutputIndex,
				Part: llmsdk.PartDelta{
					ToolCallPartDelta: &llmsdk.ToolCallPartDelta{
						Args:       ptr.To(item.ResponseFunctionToolCall.Arguments),
						ToolName:   ptr.To(item.ResponseFunctionToolCall.Name),
						ToolCallID: ptr.To(item.ResponseFunctionToolCall.CallID),
						ID:         item.ResponseFunctionToolCall.ID,
					},
				},
			}, nil
		}

		if item.ResponseReasoningItem != nil {
			if item.ResponseReasoningItem.EncryptedContent != nil {
				return &llmsdk.ContentDelta{
					Index: event.ResponseOutputItemAddedEvent.OutputIndex,
					Part: llmsdk.PartDelta{
						ReasoningPartDelta: &llmsdk.ReasoningPartDelta{
							Signature: item.ResponseReasoningItem.EncryptedContent,
							ID:        ptr.To(item.ResponseReasoningItem.ID),
						},
					},
				}, nil
			}
		}

		return nil, nil

	case event.ResponseTextDeltaEvent != nil:
		return &llmsdk.ContentDelta{
			Index: event.ResponseTextDeltaEvent.OutputIndex,
			Part:  llmsdk.NewTextPartDelta(event.ResponseTextDeltaEvent.Delta),
		}, nil

	case event.ResponseFunctionCallArgumentsDeltaEvent != nil:
		// Note: function name is added in "response.output_item.added"
		return &llmsdk.ContentDelta{
			Index: event.ResponseFunctionCallArgumentsDeltaEvent.OutputIndex,
			Part:  llmsdk.NewToolCallPartDelta(llmsdk.WithToolCallPartDeltaArgs(event.ResponseFunctionCallArgumentsDeltaEvent.Delta)),
		}, nil

	case event.ResponseImageGenCallPartialImageEvent != nil:
		var width, height *int

		responseImageGenCallPartialImageEvent := event.ResponseImageGenCallPartialImageEvent

		if responseImageGenCallPartialImageEvent.Size != "" {
			width, height = parseOpenAIImageSize(responseImageGenCallPartialImageEvent.Size)
		}

		var mimeType *string
		if responseImageGenCallPartialImageEvent.OutputFormat != "" {
			mimeType = ptr.To(fmt.Sprintf("image/%s", responseImageGenCallPartialImageEvent.OutputFormat))
		}

		return &llmsdk.ContentDelta{
			Index: responseImageGenCallPartialImageEvent.OutputIndex,
			Part: llmsdk.PartDelta{
				ImagePartDelta: &llmsdk.ImagePartDelta{
					Data:     ptr.To(responseImageGenCallPartialImageEvent.PartialImageB64),
					MimeType: mimeType,
					Width:    width,
					Height:   height,
					ID:       &responseImageGenCallPartialImageEvent.ItemID,
				},
			},
		}, nil

	case event.ResponseReasoningSummaryTextDeltaEvent != nil:
		return &llmsdk.ContentDelta{
			Index: event.ResponseReasoningSummaryTextDeltaEvent.OutputIndex,
			Part:  llmsdk.NewReasoningPartDelta(event.ResponseReasoningSummaryTextDeltaEvent.Delta),
		}, nil

	default:
		return nil, nil
	}
}

// MARK: - To SDK Usage

func mapOpenAIUsage(usage openaiapi.ResponseUsage) *llmsdk.ModelUsage {
	return &llmsdk.ModelUsage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
	}
}

// image size from openai is in the format of {number}x{number}, we parse it into width, height if available
func parseOpenAIImageSize(sizeDim string) (width, height *int) {
	dims := strings.Split(sizeDim, "x")
	if len(dims) == 2 {
		if w, err := strconv.ParseInt(dims[0], 10, 0); err == nil {
			width = ptr.To(int(w))
		}
		if h, err := strconv.ParseInt(dims[1], 10, 0); err == nil {
			height = ptr.To(int(h))
		}
	}
	return
}
