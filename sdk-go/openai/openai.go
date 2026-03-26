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

				if streamEvent.ResponseRefusalDelta != nil {
					refusal += streamEvent.ResponseRefusalDelta.Delta
				}

				partDelta, err := mapOpenAIStreamEvent(*streamEvent)
				if err != nil {
					errCh <- fmt.Errorf("failed to map stream event: %w", err)
					return
				}

				if partDelta != nil {
					responseCh <- &llmsdk.PartialModelResponse{Delta: partDelta}
				}

				if streamEvent.ResponseCompleted != nil {
					if streamEvent.ResponseCompleted.Response.Usage != nil {
						usage := mapOpenAIUsage(*streamEvent.ResponseCompleted.Response.Usage)
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

func convertToResponseCreateParams(input *llmsdk.LanguageModelInput, modelID string) (*openaiapi.CreateResponse, error) {
	inputItems, err := convertToOpenAIInputs(input.Messages)
	if err != nil {
		return nil, err
	}

	params := &openaiapi.CreateResponse{}
	params.Store = ptr.To(false)
	params.Instructions = input.SystemPrompt
	params.Temperature = input.Temperature
	params.TopP = input.TopP
	params.Reasoning = &openaiapi.Reasoning{
		Summary: ptr.To(openaiapi.ReasoningSummaryAuto),
	}
	params.Model = ptr.To(openaiapi.ModelIdsResponses(ptr.To(modelID)))
	params.Input = &openaiapi.InputParam{
		InputParamArray: (*openaiapi.InputParamArray)(&inputItems),
	}
	if input.MaxTokens != nil {
		maxTokens := int(*input.MaxTokens)
		params.MaxOutputTokens = &maxTokens
	}

	if input.Tools != nil {
		tools := openaiapi.ToolsArray{}
		for _, tool := range input.Tools {
			openAITool := openaiapi.Tool{
				FunctionTool: &openaiapi.FunctionTool{
					Name:        tool.Name,
					Description: &tool.Description,
					Parameters:  tool.Parameters,
					Strict:      ptr.To(true),
					Type:        openaiapi.FunctionToolTypeFunction,
				},
			}
			tools = append(tools, openAITool)
		}
		params.Tools = &tools
	}

	if input.ToolChoice != nil {
		params.ToolChoice = convertToOpenAIResponseToolChoice(*input.ToolChoice)
	}

	if input.ResponseFormat != nil {
		params.Text = convertToOpenAIResponseTextConfig(*input.ResponseFormat)
	}

	if input.Modalities != nil {
		if slices.Contains(input.Modalities, llmsdk.ModalityImage) {
			if params.Tools == nil {
				tools := openaiapi.ToolsArray{}
				params.Tools = &tools
			}
			*params.Tools = append(*params.Tools, openaiapi.Tool{
				ImageGenTool: &openaiapi.ImageGenTool{
					Type: openaiapi.ImageGenToolTypeImageGeneration,
				},
			})
		}
	}

	if input.Reasoning != nil {
		params.Include = []openaiapi.IncludeEnum{
			openaiapi.IncludeEnumReasoningEncryptedContent,
		}
		params.Reasoning, err = convertToOpenAIReasoning(*input.Reasoning)
		if err != nil {
			return nil, err
		}
	}

	return params, nil
}

// MARK: - To Provider Messages

func convertToOpenAIInputs(messages []llmsdk.Message) ([]openaiapi.InputItem, error) {
	var inputItems []openaiapi.InputItem

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

func convertUserMessageToOpenAIInputItem(userMessage *llmsdk.UserMessage) (openaiapi.InputItem, error) {
	messageParts := partutil.GetCompatiblePartsWithoutSourceParts(userMessage.Content)
	var content []openaiapi.InputContent

	for _, part := range messageParts {
		inputContent, err := convertToOpenAIResponseInputContent(part)
		if err != nil {
			return openaiapi.InputItem{}, err
		}
		content = append(content, *inputContent)
	}

	return openaiapi.InputItem{
		Item: &openaiapi.Item{
			InputMessage: &openaiapi.InputMessage{
				Role:    openaiapi.InputMessageRoleUser,
				Type:    ptr.To(openaiapi.InputMessageTypeMessage),
				Content: openaiapi.InputMessageContentList(content),
			},
		},
	}, nil
}

func convertAssistantMessageToOpenAIInputItems(assistantMessage *llmsdk.AssistantMessage) ([]openaiapi.InputItem, error) {
	messageParts := partutil.GetCompatiblePartsWithoutSourceParts(assistantMessage.Content)
	var inputItems []openaiapi.InputItem

	for _, part := range messageParts {
		switch {
		case part.TextPart != nil:
			inputItems = append(inputItems, openaiapi.InputItem{
				Item: &openaiapi.Item{
					OutputMessage: &openaiapi.OutputMessage{
						Id:     "msg_" + randutil.String(15),
						Role:   openaiapi.OutputMessageRoleAssistant,
						Status: openaiapi.OutputMessageStatusCompleted,
						Content: []openaiapi.OutputMessageContent{
							{
								OutputText: &openaiapi.OutputTextContent{
									Text:        part.TextPart.Text,
									Annotations: []openaiapi.Annotation{},
									Logprobs:    []openaiapi.LogProb{},
								},
							},
						},
					},
				},
			})

		case part.ReasoningPart != nil:
			id := ""
			if part.ReasoningPart.ID != nil {
				id = *part.ReasoningPart.ID
			}
			inputItems = append(inputItems, openaiapi.InputItem{
				Item: &openaiapi.Item{
					ReasoningItem: &openaiapi.ReasoningItem{
						// Similar to assistant message parts, we generate a unique ID for each reasoning part.
						Id: id,
						Summary: []openaiapi.SummaryTextContent{
							openaiapi.SummaryTextContent{
								Text: part.ReasoningPart.Text,
								Type: openaiapi.SummaryTextContentTypeSummaryText,
							},
						},
						EncryptedContent: part.ReasoningPart.Signature,
						Status:           ptr.To(openaiapi.ReasoningItemStatusCompleted),
					},
				},
			})

		case part.ImagePart != nil:
			id := ""
			if part.ImagePart.ID != nil {
				id = *part.ImagePart.ID
			}
			inputItems = append(inputItems, openaiapi.InputItem{
				Item: &openaiapi.Item{
					ImageGenToolCall: &openaiapi.ImageGenToolCall{
						Id:     id,
						Status: "completed",
						Result: ptr.To(fmt.Sprintf("data:%s;base64,%s", part.ImagePart.MimeType, part.ImagePart.Data)),
					},
				},
			})

		case part.ToolCallPart != nil:
			args, _ := json.Marshal(part.ToolCallPart.Args)
			inputItems = append(inputItems, openaiapi.InputItem{
				Item: &openaiapi.Item{
					FunctionToolCall: &openaiapi.FunctionToolCall{
						Arguments: string(args),
						CallId:    part.ToolCallPart.ToolCallID,
						Name:      part.ToolCallPart.ToolName,
						Id:        part.ToolCallPart.ID,
						Status:    ptr.To(openaiapi.FunctionToolCallStatusCompleted),
					},
				},
			})

		default:
			return nil, llmsdk.NewUnsupportedError(Provider, fmt.Sprintf("cannot convert assistant message part to OpenAI ResponseInputItem for type %s", part.Type()))
		}
	}

	return inputItems, nil
}

func convertToolMessageToOpenAIInputItems(toolMessage *llmsdk.ToolMessage) ([]openaiapi.InputItem, error) {
	var inputItems []openaiapi.InputItem
	for _, part := range toolMessage.Content {
		if part.ToolResultPart == nil {
			return nil, fmt.Errorf("tool messages must contain only tool result parts")
		}

		toolResultPartContent := partutil.GetCompatiblePartsWithoutSourceParts(part.ToolResultPart.Content)
		for _, toolResultPart := range toolResultPartContent {
			switch {
			case toolResultPart.TextPart != nil:
				inputItems = append(inputItems, openaiapi.InputItem{
					Item: &openaiapi.Item{
						FunctionCallOutputItemParam: &openaiapi.FunctionCallOutputItemParam{
							CallId: part.ToolResultPart.ToolCallID,
							Output: openaiapi.FunctionCallOutputItemParamOutput{
								FunctionCallOutputItemParamOutputArray: &openaiapi.FunctionCallOutputItemParamOutputArray{
									openaiapi.FunctionCallOutputItemParamOutputArrayItem{
										InputText: &openaiapi.InputTextContentParam{
											Text: toolResultPart.TextPart.Text,
										},
									},
								},
							},
							Type: openaiapi.FunctionCallOutputItemParamTypeFunctionCallOutput,
						},
					},
				})
			case toolResultPart.ImagePart != nil:
				inputItems = append(inputItems, openaiapi.InputItem{
					Item: &openaiapi.Item{
						FunctionCallOutputItemParam: &openaiapi.FunctionCallOutputItemParam{
							CallId: part.ToolResultPart.ToolCallID,
							Output: openaiapi.FunctionCallOutputItemParamOutput{
								FunctionCallOutputItemParamOutputArray: &openaiapi.FunctionCallOutputItemParamOutputArray{
									openaiapi.FunctionCallOutputItemParamOutputArrayItem{
										InputImage: &openaiapi.InputImageContentParamAutoParam{
											ImageUrl: ptr.To(fmt.Sprintf("data:%s;base64,%s", toolResultPart.ImagePart.MimeType, toolResultPart.ImagePart.Data)),
											Detail:   ptr.To(openaiapi.DetailEnumAuto),
										},
									},
								},
							},
							Type: openaiapi.FunctionCallOutputItemParamTypeFunctionCallOutput,
						},
					},
				})
			default:
				return nil, fmt.Errorf("cannot convert tool result part to OpenAI ResponseInputItem for type %s", toolResultPart.Type())
			}
		}
	}
	return inputItems, nil
}

func convertToOpenAIResponseInputContent(part llmsdk.Part) (*openaiapi.InputContent, error) {
	switch {
	case part.TextPart != nil:
		return &openaiapi.InputContent{
			InputText: &openaiapi.InputTextContent{
				Text: part.TextPart.Text,
			},
		}, nil

	case part.ImagePart != nil:
		return &openaiapi.InputContent{
			InputImage: &openaiapi.InputImageContent{
				Detail:   openaiapi.ImageDetailAuto,
				ImageUrl: ptr.To(fmt.Sprintf("data:%s;base64,%s", part.ImagePart.MimeType, part.ImagePart.Data)),
			},
		}, nil

	// case part.AudioPart != nil:
	// var format string
	// switch part.AudioPart.Format {
	// case llmsdk.AudioFormatMP3:
	// 	format = "mp3"
	// case llmsdk.AudioFormatWav:
	// 	format = "wav"
	// default:
	// 	return nil, llmsdk.NewUnsupportedError(Provider, fmt.Sprintf("cannot convert audio format to OpenAI InputAudio format for format %s", part.AudioPart.Format))
	// }

	// return &openaiapi.InputContent{
	// 	ResponseInputAudio: &openaiapi.ResponseInputAudio{
	// 		InputAudio: openaiapi.ResponseInputAudioInputAudio{
	// 			Data:   part.AudioPart.Data,
	// 			Format: format,
	// 		},
	// 	},
	// }, nil

	default:
		return nil, llmsdk.NewUnsupportedError(Provider, fmt.Sprintf("cannot convert part to OpenAI content part for type %s", part.Type()))
	}
}

// MARK: - To Provider Tools

func convertToOpenAIResponseToolChoice(toolChoice llmsdk.ToolChoiceOption) *openaiapi.ToolChoiceParam {
	choice := &openaiapi.ToolChoiceParam{}
	if toolChoice.Auto != nil {
		choice.ToolChoiceOptions = ptr.To(openaiapi.ToolChoiceOptionsAuto)
		return choice
	}
	if toolChoice.None != nil {
		choice.ToolChoiceOptions = ptr.To(openaiapi.ToolChoiceOptionsNone)
		return choice
	}
	if toolChoice.Required != nil {
		choice.ToolChoiceOptions = ptr.To(openaiapi.ToolChoiceOptionsRequired)
		return choice
	}
	if toolChoice.Tool != nil {
		choice.ToolChoiceFunction = &openaiapi.ToolChoiceFunction{
			Type: openaiapi.ToolChoiceFunctionTypeFunction,
			Name: toolChoice.Tool.ToolName,
		}
		return choice
	}
	return nil
}

// MARK: - To Provider Response Format

func convertToOpenAIResponseTextConfig(responseFormat llmsdk.ResponseFormatOption) *openaiapi.ResponseTextParam {
	if responseFormat.Text != nil {
		return &openaiapi.ResponseTextParam{
			Format: &openaiapi.TextResponseFormatConfiguration{
				Text: &openaiapi.ResponseFormatText{},
			},
		}
	}

	if responseFormat.JSON != nil {
		if responseFormat.JSON.Schema != nil {
			return &openaiapi.ResponseTextParam{
				Format: &openaiapi.TextResponseFormatConfiguration{
					JsonSchema: &openaiapi.TextResponseFormatJsonSchema{
						Name:        responseFormat.JSON.Name,
						Schema:      *responseFormat.JSON.Schema,
						Description: responseFormat.JSON.Description,
						Strict:      ptr.To(true),
					},
				},
			}
		}
		return &openaiapi.ResponseTextParam{
			Format: &openaiapi.TextResponseFormatConfiguration{
				JsonObject: &openaiapi.ResponseFormatJsonObject{},
			},
		}
	}
	return nil
}

func convertToOpenAIReasoning(reasoning llmsdk.ReasoningOptions) (*openaiapi.Reasoning, error) {
	openaiReasoning := &openaiapi.Reasoning{}
	if reasoning.Enabled {
		openaiReasoning.Summary = ptr.To(openaiapi.ReasoningSummaryAuto)
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

func mapOpenAIOutputItems(items []openaiapi.OutputItem) ([]llmsdk.Part, error) {
	var parts []llmsdk.Part

	for _, item := range items {
		switch {
		case item.Message != nil:
			for _, content := range item.Message.Content {
				switch {
				case content.OutputText != nil:
					parts = append(parts, llmsdk.NewTextPart(content.OutputText.Text))
				case content.Refusal != nil:
					return nil, llmsdk.NewRefusalError(content.Refusal.Refusal)
				}
			}

		case item.FunctionCall != nil:
			var args map[string]any
			if err := json.Unmarshal([]byte(item.FunctionCall.Arguments), &args); err != nil {
				return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
			}

			toolCallPart := llmsdk.NewToolCallPart(
				item.FunctionCall.CallId,
				item.FunctionCall.Name,
				args,
			)
			toolCallPart.ToolCallPart.ID = item.FunctionCall.Id
			parts = append(parts, toolCallPart)

		case item.ImageGenerationCall != nil:
			responseOutputItemImageGenerationCall := item.ImageGenerationCall
			if responseOutputItemImageGenerationCall.Result == nil {
				return nil, llmsdk.NewInvariantError(Provider, "image generation call did not return a result")
			}

			var width, height *int
			if responseOutputItemImageGenerationCall.Size != nil {
				width, height = parseOpenAIImageSize(string(*responseOutputItemImageGenerationCall.Size))
			}

			mimeType := ""
			if responseOutputItemImageGenerationCall.OutputFormat != nil {
				mimeType = "image/" + string(*responseOutputItemImageGenerationCall.OutputFormat)
			}

			imageOpts := []llmsdk.ImagePartOption{}
			imageOpts = append(imageOpts, llmsdk.WithImageID(responseOutputItemImageGenerationCall.Id))
			if width != nil {
				imageOpts = append(imageOpts, llmsdk.WithImageWidth(*width))
			}
			if height != nil {
				imageOpts = append(imageOpts, llmsdk.WithImageHeight(*height))
			}
			parts = append(parts, llmsdk.NewImagePart(
				*responseOutputItemImageGenerationCall.Result,
				mimeType,
				imageOpts...,
			))

		case item.Reasoning != nil:
			var summary = ""
			for _, s := range item.Reasoning.Summary {
				summary += s.Text + "\n"
			}

			reasoningOpts := []llmsdk.ReasoingPartOption{}
			if item.Reasoning.EncryptedContent != nil {
				reasoningOpts = append(reasoningOpts, llmsdk.WithReasoningSignature(*item.Reasoning.EncryptedContent))
			}
			reasoningOpts = append(reasoningOpts, llmsdk.WithReasoningID(item.Reasoning.Id))
			parts = append(parts, llmsdk.NewReasoningPart(summary, reasoningOpts...))
		}
	}

	return parts, nil
}

// MARK: - To SDK Delta

func mapOpenAIStreamEvent(event openaiapi.ResponseStreamEvent) (*llmsdk.ContentDelta, error) {
	switch {
	case event.ResponseFailed != nil:
		// Handle failed response - convert to error
		return nil, llmsdk.NewInvariantError(Provider, "stream event failed")

	case event.ResponseOutputItemAdded != nil:
		item := event.ResponseOutputItemAdded.Item

		if item.FunctionCall != nil {
			return &llmsdk.ContentDelta{
				Index: event.ResponseOutputItemAdded.OutputIndex,
				Part: llmsdk.PartDelta{
					ToolCallPartDelta: &llmsdk.ToolCallPartDelta{
						Args:       ptr.To(item.FunctionCall.Arguments),
						ToolName:   ptr.To(item.FunctionCall.Name),
						ToolCallID: ptr.To(item.FunctionCall.CallId),
						ID:         item.FunctionCall.Id,
					},
				},
			}, nil
		}

		if item.Reasoning != nil {
			if item.Reasoning.EncryptedContent != nil {
				return &llmsdk.ContentDelta{
					Index: event.ResponseOutputItemAdded.OutputIndex,
					Part: llmsdk.PartDelta{
						ReasoningPartDelta: &llmsdk.ReasoningPartDelta{
							Signature: item.Reasoning.EncryptedContent,
							ID:        ptr.To(item.Reasoning.Id),
						},
					},
				}, nil
			}
		}

		return nil, nil

	case event.ResponseOutputTextDelta != nil:
		return &llmsdk.ContentDelta{
			Index: event.ResponseOutputTextDelta.OutputIndex,
			Part:  llmsdk.NewTextPartDelta(event.ResponseOutputTextDelta.Delta),
		}, nil

	case event.ResponseFunctionCallArgumentsDelta != nil:
		// Note: function name is added in "response.output_item.added"
		return &llmsdk.ContentDelta{
			Index: event.ResponseFunctionCallArgumentsDelta.OutputIndex,
			Part:  llmsdk.NewToolCallPartDelta(llmsdk.WithToolCallPartDeltaArgs(event.ResponseFunctionCallArgumentsDelta.Delta)),
		}, nil

	case event.ResponseImageGenerationCallPartialImage != nil:
		responseImageGenCallPartialImageEvent := event.ResponseImageGenerationCallPartialImage
		var width, height *int
		if responseImageGenCallPartialImageEvent.Size != nil {
			width, height = parseOpenAIImageSize(string(*responseImageGenCallPartialImageEvent.Size))
		}
		mimeType := ""
		if responseImageGenCallPartialImageEvent.OutputFormat != nil {
			mimeType = "image/" + string(*responseImageGenCallPartialImageEvent.OutputFormat)
		}

		return &llmsdk.ContentDelta{
			Index: responseImageGenCallPartialImageEvent.OutputIndex,
			Part: llmsdk.PartDelta{
				ImagePartDelta: &llmsdk.ImagePartDelta{
					Data:     ptr.To(responseImageGenCallPartialImageEvent.PartialImageB64),
					Width:    width,
					Height:   height,
					MimeType: ptr.To(mimeType),
					ID:       &responseImageGenCallPartialImageEvent.ItemId,
				},
			},
		}, nil

	case event.ResponseReasoningSummaryTextDelta != nil:
		return &llmsdk.ContentDelta{
			Index: event.ResponseReasoningSummaryTextDelta.OutputIndex,
			Part:  llmsdk.NewReasoningPartDelta(event.ResponseReasoningSummaryTextDelta.Delta),
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
