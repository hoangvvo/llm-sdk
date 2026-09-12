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
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/toolresultutils"
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
			webSearchRequests := 0
			for _, item := range response.Output {
				if item.WebSearchToolCall != nil && item.WebSearchToolCall.Action.Search != nil {
					webSearchRequests++
				}
			}
			usage = mapOpenAIUsage(*response.Usage, webSearchRequests)
		}

		result := &llmsdk.ModelResponse{
			Content: content,
			Usage:   usage,
		}

		if m.metadata != nil && m.metadata.Pricing != nil && usage != nil {
			cost := usage.CalculateCost(m.metadata.Pricing, llmsdk.ModelUsageCostOptions{InputCacheTokensAreAdditional: false, OutputReasoningTokensAreAdditional: false})
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
			normalizedOutputIndexes := map[int]int{}
			nextContentIndex := 0
			webSearchRequests := 0
			streamState := &openAIStreamState{}

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
				if done := streamEvent.ResponseOutputItemDone; done != nil && done.Item.WebSearchToolCall != nil && done.Item.WebSearchToolCall.Action.Search != nil {
					webSearchRequests++
				}

				partDelta, err := mapOpenAIStreamEvent(*streamEvent, streamState)
				if err != nil {
					errCh <- fmt.Errorf("failed to map stream event: %w", err)
					return
				}

				if partDelta != nil {
					providerOutputIndex := partDelta.Index
					normalizedOutputIndex, ok := normalizedOutputIndexes[providerOutputIndex]
					if !ok {
						normalizedOutputIndex = nextContentIndex
						nextContentIndex++
						normalizedOutputIndexes[providerOutputIndex] = normalizedOutputIndex
					}
					partDelta.Index = normalizedOutputIndex
					if !stream.Send(ctx, responseCh, &llmsdk.PartialModelResponse{Delta: partDelta}) {
						return
					}
				}

				if resultDelta := mapOpenAIStreamWebSearchResult(*streamEvent, nextContentIndex); resultDelta != nil {
					nextContentIndex++
					if !stream.Send(ctx, responseCh, &llmsdk.PartialModelResponse{Delta: resultDelta}) {
						return
					}
				}

				if streamEvent.ResponseCompleted != nil {
					if streamEvent.ResponseCompleted.Response.Usage != nil {
						usage := mapOpenAIUsage(*streamEvent.ResponseCompleted.Response.Usage, webSearchRequests)
						partial := &llmsdk.PartialModelResponse{Usage: usage}
						if m.metadata != nil && m.metadata.Pricing != nil {
							partial.Cost = ptr.To(usage.CalculateCost(m.metadata.Pricing, llmsdk.ModelUsageCostOptions{InputCacheTokensAreAdditional: false, OutputReasoningTokensAreAdditional: false}))
						}
						if !stream.Send(ctx, responseCh, partial) {
							return
						}
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
	inputItems, err := convertToOpenAIInputs(input.Messages, input.Tools)
	if err != nil {
		return nil, err
	}

	params := &openaiapi.CreateResponse{}
	params.Store = ptr.To(false)
	if input.CacheRetention != nil {
		retention := openaiapi.CreateResponsePromptCacheRetentionInMemory
		if *input.CacheRetention == llmsdk.CacheRetentionExtended {
			retention = openaiapi.CreateResponsePromptCacheRetentionN24H
		}
		params.PromptCacheRetention = &retention
	}
	if len(input.Metadata) > 0 {
		metadata := openaiapi.Metadata{}
		for k, v := range input.Metadata {
			metadata[k] = v
		}
		params.Metadata = &metadata
	}
	params.Instructions = input.SystemPrompt
	params.Temperature = input.Temperature
	params.TopP = input.TopP
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
		hasWebSearchTool := false
		for _, tool := range input.Tools {
			if tool.WebSearchTool != nil {
				hasWebSearchTool = true
				webSearch := tool.WebSearchTool
				openAIWebSearch := &openaiapi.WebSearchTool{
					Type: openaiapi.WebSearchToolTypeWebSearch,
				}
				if len(webSearch.AllowedDomains) > 0 {
					openAIWebSearch.Filters = &openaiapi.WebSearchToolFilters{AllowedDomains: webSearch.AllowedDomains}
				}
				if webSearch.UserLocation != nil {
					locationType := openaiapi.WebSearchApproximateLocationTypeApproximate
					openAIWebSearch.UserLocation = &openaiapi.WebSearchApproximateLocation{
						City: webSearch.UserLocation.City, Country: webSearch.UserLocation.Country,
						Region: webSearch.UserLocation.Region, Timezone: webSearch.UserLocation.Timezone,
						Type: &locationType,
					}
				}
				tools = append(tools, openaiapi.Tool{WebSearchTool: openAIWebSearch})
				continue
			}
			if tool.ToolSearchTool != nil {
				// OpenAI's hosted search has a single algorithm, so strategy is ignored.
				tools = append(tools, openaiapi.Tool{ToolSearchToolParam: &openaiapi.ToolSearchToolParam{
					Type: openaiapi.ToolSearchToolParamTypeToolSearch,
				}})
				continue
			}
			if tool.FunctionTool == nil {
				continue
			}
			tools = append(tools, convertToOpenAIFunctionTool(tool.FunctionTool))
		}
		params.Tools = &tools
		if hasWebSearchTool {
			params.Include = append(params.Include, openaiapi.IncludeEnumWebSearchCallActionSources)
		}
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
		params.Include = append(params.Include,
			openaiapi.IncludeEnumReasoningEncryptedContent,
		)
		params.Reasoning, err = convertToOpenAIReasoning(*input.Reasoning)
		if err != nil {
			return nil, err
		}
	}

	return params, nil
}

func convertToOpenAIFunctionTool(functionTool *llmsdk.FunctionTool) openaiapi.Tool {
	openAIFunctionTool := &openaiapi.FunctionTool{
		Name:        functionTool.Name,
		Description: ptr.To(functionTool.Description),
		Parameters:  functionTool.Parameters,
		Strict:      ptr.To(true),
		Type:        openaiapi.FunctionToolTypeFunction,
	}
	if functionTool.DeferLoading != nil && *functionTool.DeferLoading {
		openAIFunctionTool.DeferLoading = ptr.To(true)
	}
	return openaiapi.Tool{FunctionTool: openAIFunctionTool}
}

// MARK: - To Provider Messages

func convertToOpenAIInputs(messages []llmsdk.Message, tools []llmsdk.Tool) ([]openaiapi.InputItem, error) {
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
			items, err := convertAssistantMessageToOpenAIInputItems(message.AssistantMessage, tools)
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

func convertAssistantMessageToOpenAIInputItems(assistantMessage *llmsdk.AssistantMessage, tools []llmsdk.Tool) ([]openaiapi.InputItem, error) {
	messageParts := partutil.GetCompatiblePartsWithoutSourceParts(assistantMessage.Content)
	var inputItems []openaiapi.InputItem

	for _, part := range messageParts {
		switch {
		// OpenAI replays hosted search results through the web_search_call item.
		case part.ToolResultPart != nil && part.ToolResultPart.Result.WebSearch != nil:
			continue

		case part.ToolResultPart != nil && part.ToolResultPart.Result.ToolSearch != nil:
			inputItems = append(inputItems, convertToOpenAIToolSearchOutput(part.ToolResultPart, part.ToolResultPart.Result.ToolSearch, tools))

		case part.TextPart != nil:
			inputItems = append(inputItems, openaiapi.InputItem{
				Item: &openaiapi.Item{
					OutputMessage: &openaiapi.OutputMessage{
						Id:     "msg_" + randutil.String(15),
						Role:   openaiapi.OutputMessageRoleAssistant,
						Status: openaiapi.OutputMessageStatusCompleted,
						Type:   openaiapi.OutputMessageTypeMessage,
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
						Id: id,
						Summary: []openaiapi.SummaryTextContent{
							{
								Text: part.ReasoningPart.Text,
								Type: openaiapi.SummaryTextContentTypeSummaryText,
							},
						},
						EncryptedContent: part.ReasoningPart.Signature,
						Type:             openaiapi.ReasoningItemTypeReasoning,
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
						Type:   openaiapi.ImageGenToolCallTypeImageGenerationCall,
					},
				},
			})

		case part.ToolCallPart != nil:
			if toolSearch := part.ToolCallPart.Call.ToolSearch; toolSearch != nil {
				id := part.ToolCallPart.ToolCallID
				if part.ToolCallPart.ID != nil {
					id = *part.ToolCallPart.ID
				}
				args := toolSearch.Args
				if len(args) == 0 {
					args = json.RawMessage(`{}`)
				}
				inputItems = append(inputItems, openaiapi.InputItem{Item: &openaiapi.Item{ToolSearchCallItemParam: &openaiapi.ToolSearchCallItemParam{
					Type:      openaiapi.ToolSearchCallItemParamTypeToolSearchCall,
					Id:        &id,
					Arguments: args,
					Status:    ptr.To(openaiapi.FunctionCallItemStatusCompleted),
					Execution: ptr.To(openaiapi.ToolSearchExecutionTypeServer),
				}}})
				continue
			}
			if part.ToolCallPart.Call.WebSearch != nil {
				web := part.ToolCallPart.Call.WebSearch
				if web.Action == nil {
					return nil, llmsdk.NewInvalidInputError("OpenAI web-search history requires an action")
				}
				status := openaiapi.WebSearchToolCallStatusCompleted
				if web.Status != nil {
					status = openaiapi.WebSearchToolCallStatus(*web.Status)
				}
				inputItems = append(inputItems, openaiapi.InputItem{Item: &openaiapi.Item{WebSearchToolCall: &openaiapi.WebSearchToolCall{
					Id: part.ToolCallPart.ToolCallID, Status: status,
					Type:   openaiapi.WebSearchToolCallTypeWebSearchCall,
					Action: convertToOpenAIWebSearchAction(*web.Action),
				}}})
				continue
			}
			call := part.ToolCallPart.Call.Function
			if call == nil {
				return nil, llmsdk.NewUnsupportedError(Provider, "tool call has no supported payload")
			}
			args, _ := json.Marshal(call.Args)
			functionCall := &openaiapi.FunctionToolCall{
				Arguments: string(args),
				CallId:    part.ToolCallPart.ToolCallID,
				Name:      call.Name,
				Id:        part.ToolCallPart.ID,
				Type:      openaiapi.FunctionToolCallTypeFunctionCall,
			}
			// Calls to deferred tools must be replayed with the namespace OpenAI
			// assigned them, which for top-level functions is the function name.
			for _, tool := range tools {
				if tool.FunctionTool != nil && tool.FunctionTool.Name == call.Name && tool.FunctionTool.DeferLoading != nil && *tool.FunctionTool.DeferLoading {
					functionCall.Namespace = ptr.To(call.Name)
					break
				}
			}
			inputItems = append(inputItems, openaiapi.InputItem{Item: &openaiapi.Item{FunctionToolCall: functionCall}})

		default:
			return nil, llmsdk.NewUnsupportedError(Provider, fmt.Sprintf("cannot convert assistant message part to OpenAI ResponseInputItem for type %s", part.Type()))
		}
	}

	return inputItems, nil
}

func convertToOpenAIToolSearchOutput(part *llmsdk.ToolResultPart, result *llmsdk.ToolSearchToolResult, tools []llmsdk.Tool) openaiapi.InputItem {
	// OpenAI needs the full definitions of the discovered tools, which the
	// request already declares as deferred function tools.
	discoveredTools := make([]openaiapi.Tool, 0, len(result.ToolNames))
	for _, toolName := range result.ToolNames {
		for _, tool := range tools {
			if tool.FunctionTool != nil && tool.FunctionTool.Name == toolName {
				discoveredTools = append(discoveredTools, convertToOpenAIFunctionTool(tool.FunctionTool))
				break
			}
		}
	}
	status := openaiapi.FunctionCallItemStatusIncomplete
	if part.Status == llmsdk.ToolResultStatusCompleted {
		status = openaiapi.FunctionCallItemStatusCompleted
	}
	return openaiapi.InputItem{Item: &openaiapi.Item{ToolSearchOutputItemParam: &openaiapi.ToolSearchOutputItemParam{
		Type:      openaiapi.ToolSearchOutputItemParamTypeToolSearchOutput,
		Execution: ptr.To(openaiapi.ToolSearchExecutionTypeServer),
		Status:    &status,
		Tools:     discoveredTools,
	}}}
}

func convertToolMessageToOpenAIInputItems(toolMessage *llmsdk.ToolMessage) ([]openaiapi.InputItem, error) {
	var inputItems []openaiapi.InputItem
	for _, part := range toolMessage.Content {
		if part.ToolResultPart == nil {
			return nil, llmsdk.NewInvalidInputError("tool messages must contain only tool result parts")
		}
		// Hosted tool results are replayed through their assistant-message items.
		result := part.ToolResultPart.Result.Function
		if result == nil {
			continue
		}

		toolResultPartContent := partutil.GetCompatiblePartsWithoutSourceParts(result.Content)
		if len(toolResultPartContent) == 0 {
			content := ""
			if part.ToolResultPart.Status == llmsdk.ToolResultStatusCancelled {
				content = toolresultutils.CancelledFallbackContent
			}
			empty := openaiapi.FunctionCallOutputItemParamOutputString(ptr.To(content))
			inputItems = append(inputItems, openaiapi.InputItem{
				Item: &openaiapi.Item{
					FunctionCallOutputItemParam: &openaiapi.FunctionCallOutputItemParam{
						CallId: part.ToolResultPart.ToolCallID,
						Output: openaiapi.FunctionCallOutputItemParamOutput{
							FunctionCallOutputItemParamOutputString: &empty,
						},
						Type: openaiapi.FunctionCallOutputItemParamTypeFunctionCallOutput,
					},
				},
			})
			continue
		}
		// A call has exactly one output item, so every result part becomes an
		// entry of the same output list.
		output := make(openaiapi.FunctionCallOutputItemParamOutputArray, 0, len(toolResultPartContent))
		for _, toolResultPart := range toolResultPartContent {
			switch {
			case toolResultPart.TextPart != nil:
				output = append(output, openaiapi.FunctionCallOutputItemParamOutputArrayItem{
					InputText: &openaiapi.InputTextContentParam{Text: toolResultPart.TextPart.Text},
				})
			case toolResultPart.ImagePart != nil:
				output = append(output, openaiapi.FunctionCallOutputItemParamOutputArrayItem{
					InputImage: &openaiapi.InputImageContentParamAutoParam{
						ImageUrl: ptr.To(convertToOpenAIInputImageURL(toolResultPart.ImagePart)),
						Detail:   ptr.To(openaiapi.DetailEnumAuto),
					},
				})
			case toolResultPart.FilePart != nil:
				file := convertToOpenAIInputFile(toolResultPart.FilePart)
				output = append(output, openaiapi.FunctionCallOutputItemParamOutputArrayItem{
					InputFile: &openaiapi.InputFileContentParam{
						FileData: file.fileData, FileUrl: file.fileURL, Filename: file.filename,
					},
				})
			default:
				return nil, llmsdk.NewUnsupportedError(Provider, fmt.Sprintf("cannot convert tool result part to OpenAI ResponseInputItem for type %s", toolResultPart.Type()))
			}
		}
		inputItems = append(inputItems, openaiapi.InputItem{
			Item: &openaiapi.Item{
				FunctionCallOutputItemParam: &openaiapi.FunctionCallOutputItemParam{
					CallId: part.ToolResultPart.ToolCallID,
					Output: openaiapi.FunctionCallOutputItemParamOutput{
						FunctionCallOutputItemParamOutputArray: &output,
					},
					Type: openaiapi.FunctionCallOutputItemParamTypeFunctionCallOutput,
				},
			},
		})
	}
	return inputItems, nil
}

func convertToOpenAIResponseInputContent(part llmsdk.Part) (*openaiapi.InputContent, error) {
	switch {
	case part.TextPart != nil:
		return &openaiapi.InputContent{
			InputText: &openaiapi.InputTextContent{
				Text: part.TextPart.Text,
				Type: openaiapi.InputTextContentTypeInputText,
			},
		}, nil

	case part.ImagePart != nil:
		return &openaiapi.InputContent{
			InputImage: &openaiapi.InputImageContent{
				Detail:   ptr.To(openaiapi.ImageDetailAuto),
				ImageUrl: ptr.To(convertToOpenAIInputImageURL(part.ImagePart)),
				Type:     openaiapi.InputImageContentTypeInputImage,
			},
		}, nil

	case part.FilePart != nil:
		file := convertToOpenAIInputFile(part.FilePart)
		return &openaiapi.InputContent{
			InputFile: &openaiapi.InputFileContent{
				FileData: file.fileData, FileUrl: file.fileURL, Filename: file.filename,
				Type: openaiapi.InputFileContentTypeInputFile,
			},
		}, nil

	default:
		return nil, llmsdk.NewUnsupportedError(Provider, fmt.Sprintf("cannot convert part to OpenAI content part for type %s", part.Type()))
	}
}

// convertToOpenAIInputImageURL returns the part URL, or a data URL of the inline data.
func convertToOpenAIInputImageURL(part *llmsdk.ImagePart) string {
	if part.URL != nil {
		return *part.URL
	}
	return fmt.Sprintf("data:%s;base64,%s", part.MimeType, part.Data)
}

type openAIInputFile struct {
	fileData *string
	fileURL  *string
	filename *string
}

func convertToOpenAIInputFile(part *llmsdk.FilePart) openAIInputFile {
	file := openAIInputFile{filename: part.Filename}
	if part.URL != nil {
		file.fileURL = part.URL
	} else {
		file.fileData = ptr.To(fmt.Sprintf("data:%s;base64,%s", part.MimeType, part.Data))
	}
	return file
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
	parts := make([]llmsdk.Part, 0, len(items))
	// Hosted tool searches have no call_id, so their output is matched to the
	// preceding search call.
	var lastToolSearchCallID *string

	for _, item := range items {
		switch {
		case item.ToolSearchCall != nil:
			lastToolSearchCallID = ptr.To(openAIToolSearchCallID(item.ToolSearchCall.CallId, item.ToolSearchCall.Id))
			parts = append(parts, mapOpenAIToolSearchCall(item.ToolSearchCall))

		case item.ToolSearchOutput != nil:
			toolCallID := item.ToolSearchOutput.CallId
			if toolCallID == nil {
				toolCallID = lastToolSearchCallID
			}
			parts = append(parts, mapOpenAIToolSearchOutput(item.ToolSearchOutput, toolCallID))

		case item.OutputMessage != nil:
			for _, content := range item.OutputMessage.Content {
				switch {
				case content.OutputText != nil:
					textPart := llmsdk.NewTextPart(content.OutputText.Text)
					for _, annotation := range content.OutputText.Annotations {
						if annotation.UrlCitation != nil {
							textPart.TextPart.Citations = append(textPart.TextPart.Citations, mapOpenAIURLCitation(*annotation.UrlCitation))
						}
					}
					parts = append(parts, textPart)
				case content.Refusal != nil:
					return nil, llmsdk.NewRefusalError(content.Refusal.Refusal)
				}
			}

		case item.FunctionToolCall != nil:
			var args map[string]any
			if err := json.Unmarshal([]byte(item.FunctionToolCall.Arguments), &args); err != nil {
				return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
			}

			toolCallPart := llmsdk.NewToolCallPart(
				item.FunctionToolCall.CallId,
				item.FunctionToolCall.Name,
				args,
			)
			toolCallPart.ToolCallPart.ID = item.FunctionToolCall.Id
			parts = append(parts, toolCallPart)

		case item.WebSearchToolCall != nil:
			web := item.WebSearchToolCall
			status := mapOpenAIWebSearchCallStatus(web.Status)
			call := llmsdk.Part{ToolCallPart: &llmsdk.ToolCallPart{
				ToolCallID: web.Id,
				Call: llmsdk.ToolCall{WebSearch: &llmsdk.WebSearchToolCall{
					Status: &status, Action: mapOpenAIWebSearchAction(web.Action),
				}},
			}}
			parts = append(parts, call)
			if web.Action.Search != nil && len(web.Action.Search.Sources) > 0 {
				sources := make([]llmsdk.WebSearchSource, 0, len(web.Action.Search.Sources))
				for _, source := range web.Action.Search.Sources {
					sources = append(sources, llmsdk.WebSearchSource{URL: source.Url})
				}
				parts = append(parts, llmsdk.Part{ToolResultPart: &llmsdk.ToolResultPart{
					ToolCallID: web.Id, Result: llmsdk.ToolResult{WebSearch: &llmsdk.WebSearchToolResult{Sources: sources}},
					Status: llmsdk.ToolResultStatusCompleted,
				}})
			}

		case item.ImageGenToolCall != nil:
			responseOutputItemImageGenerationCall := item.ImageGenToolCall
			if responseOutputItemImageGenerationCall.Result == nil {
				return nil, llmsdk.NewInvariantError(Provider, "image generation call did not return a result")
			}

			var width, height *int
			if responseOutputItemImageGenerationCall.Size != nil {
				width, height = parseOpenAIImageSize(string(*responseOutputItemImageGenerationCall.Size))
			}

			mimeType := "image/png"
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

		case item.ReasoningItem != nil:
			summaryTexts := make([]string, 0, len(item.ReasoningItem.Summary))
			for _, s := range item.ReasoningItem.Summary {
				summaryTexts = append(summaryTexts, s.Text)
			}
			summary := strings.Join(summaryTexts, "\n")

			reasoningOpts := []llmsdk.ReasoningPartOption{}
			if item.ReasoningItem.EncryptedContent != nil {
				reasoningOpts = append(reasoningOpts, llmsdk.WithReasoningSignature(*item.ReasoningItem.EncryptedContent))
			}
			reasoningOpts = append(reasoningOpts, llmsdk.WithReasoningID(item.ReasoningItem.Id))
			parts = append(parts, llmsdk.NewReasoningPart(summary, reasoningOpts...))
		}
	}

	return parts, nil
}

// mapOpenAIWebSearchCallStatus maps the provider status, which reports a
// failed search as "incomplete".
func mapOpenAIWebSearchCallStatus(status openaiapi.WebSearchToolCallStatus) llmsdk.WebSearchToolCallStatus {
	if string(status) == "incomplete" {
		return llmsdk.WebSearchToolCallStatusFailed
	}
	return llmsdk.WebSearchToolCallStatus(status)
}

func mapOpenAIToolSearchStatus(status openaiapi.FunctionCallStatus) llmsdk.ToolSearchToolCallStatus {
	if status == openaiapi.FunctionCallStatusIncomplete {
		return llmsdk.ToolSearchToolCallStatusFailed
	}
	return llmsdk.ToolSearchToolCallStatus(status)
}

func openAIToolSearchCallID(callID *string, id string) string {
	if callID != nil {
		return *callID
	}
	return id
}

// openAIToolSearchArgs encodes the search arguments object, falling back to an
// empty object for anything that is not one.
func openAIToolSearchArgs(arguments any) json.RawMessage {
	object, ok := arguments.(map[string]any)
	if !ok || object == nil {
		return json.RawMessage(`{}`)
	}
	data, err := json.Marshal(object)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}

func mapOpenAIToolSearchCall(item *openaiapi.ToolSearchCall) llmsdk.Part {
	status := mapOpenAIToolSearchStatus(item.Status)
	return llmsdk.Part{ToolCallPart: &llmsdk.ToolCallPart{
		ToolCallID: openAIToolSearchCallID(item.CallId, item.Id),
		ID:         ptr.To(item.Id),
		Call: llmsdk.ToolCall{ToolSearch: &llmsdk.ToolSearchToolCall{
			Args:   openAIToolSearchArgs(item.Arguments),
			Status: &status,
		}},
	}}
}

func mapOpenAIToolSearchOutput(item *openaiapi.ToolSearchOutput, toolCallID *string) llmsdk.Part {
	status := llmsdk.ToolResultStatusCompleted
	if item.Status == openaiapi.FunctionCallOutputStatusEnumIncomplete {
		status = llmsdk.ToolResultStatusFailed
	}
	return llmsdk.Part{ToolResultPart: &llmsdk.ToolResultPart{
		ToolCallID: openAIToolSearchCallID(toolCallID, item.Id),
		Result: llmsdk.ToolResult{ToolSearch: &llmsdk.ToolSearchToolResult{
			ToolNames: mapOpenAIDiscoveredToolNames(item.Tools),
		}},
		Status: status,
	}}
}

func mapOpenAIDiscoveredToolNames(tools []openaiapi.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		switch {
		case tool.FunctionTool != nil:
			names = append(names, tool.FunctionTool.Name)
		case tool.CustomToolParam != nil:
			names = append(names, tool.CustomToolParam.Name)
		case tool.NamespaceToolParam != nil:
			for _, member := range tool.NamespaceToolParam.Tools {
				switch {
				case member.Function != nil:
					names = append(names, member.Function.Name)
				case member.Custom != nil:
					names = append(names, member.Custom.Name)
				}
			}
		}
	}
	return names
}

// MARK: - To SDK Delta

// openAIStreamState carries the stream-wide context needed to map events.
type openAIStreamState struct {
	lastToolSearchCallID *string
}

func mapOpenAIStreamEvent(event openaiapi.ResponseStreamEvent, state *openAIStreamState) (*llmsdk.ContentDelta, error) {
	switch {
	case event.ResponseFailed != nil:
		message := "OpenAI Response Stream failed"
		if event.ResponseFailed.Response.Error.Message != "" {
			message += ": " + event.ResponseFailed.Response.Error.Message
		}
		return nil, llmsdk.NewInvariantError(Provider, message)

	case event.ResponseOutputItemAdded != nil:
		item := event.ResponseOutputItemAdded.Item

		if item.FunctionToolCall != nil {
			return &llmsdk.ContentDelta{
				Index: event.ResponseOutputItemAdded.OutputIndex,
				Part: llmsdk.PartDelta{
					ToolCallPartDelta: &llmsdk.ToolCallPartDelta{
						Call:       llmsdk.ToolCallDelta{Function: &llmsdk.FunctionToolCallDelta{Name: ptr.To(item.FunctionToolCall.Name), Args: ptr.To(item.FunctionToolCall.Arguments)}},
						ToolCallID: ptr.To(item.FunctionToolCall.CallId),
						ID:         item.FunctionToolCall.Id,
					},
				},
			}, nil
		}
		if item.WebSearchToolCall != nil {
			status := mapOpenAIWebSearchCallStatus(item.WebSearchToolCall.Status)
			return &llmsdk.ContentDelta{Index: event.ResponseOutputItemAdded.OutputIndex, Part: llmsdk.PartDelta{
				ToolCallPartDelta: &llmsdk.ToolCallPartDelta{
					ToolCallID: ptr.To(item.WebSearchToolCall.Id),
					Call:       llmsdk.ToolCallDelta{WebSearch: &llmsdk.WebSearchToolCallDelta{Status: &status, Action: mapOpenAIWebSearchAction(item.WebSearchToolCall.Action)}},
				},
			}}, nil
		}
		if item.ToolSearchCall != nil {
			toolCallID := openAIToolSearchCallID(item.ToolSearchCall.CallId, item.ToolSearchCall.Id)
			state.lastToolSearchCallID = &toolCallID
			status := mapOpenAIToolSearchStatus(item.ToolSearchCall.Status)
			return &llmsdk.ContentDelta{Index: event.ResponseOutputItemAdded.OutputIndex, Part: llmsdk.PartDelta{
				ToolCallPartDelta: &llmsdk.ToolCallPartDelta{
					ToolCallID: &toolCallID,
					ID:         ptr.To(item.ToolSearchCall.Id),
					Call:       llmsdk.ToolCallDelta{ToolSearch: &llmsdk.ToolSearchToolCallDelta{Status: &status}},
				},
			}}, nil
		}

		if item.ReasoningItem != nil && item.ReasoningItem.EncryptedContent != nil {
			return &llmsdk.ContentDelta{
				Index: event.ResponseOutputItemAdded.OutputIndex,
				Part: llmsdk.PartDelta{
					ReasoningPartDelta: &llmsdk.ReasoningPartDelta{
						Signature: item.ReasoningItem.EncryptedContent,
						ID:        ptr.To(item.ReasoningItem.Id),
					},
				},
			}, nil
		}

		return nil, nil

	case event.ResponseOutputItemDone != nil:
		item := event.ResponseOutputItemDone.Item
		if item.ToolSearchCall != nil {
			// Search arguments arrive whole with the completed item.
			toolCallID := openAIToolSearchCallID(item.ToolSearchCall.CallId, item.ToolSearchCall.Id)
			state.lastToolSearchCallID = &toolCallID
			status := mapOpenAIToolSearchStatus(item.ToolSearchCall.Status)
			args := string(openAIToolSearchArgs(item.ToolSearchCall.Arguments))
			return &llmsdk.ContentDelta{Index: event.ResponseOutputItemDone.OutputIndex, Part: llmsdk.PartDelta{
				ToolCallPartDelta: &llmsdk.ToolCallPartDelta{
					ToolCallID: &toolCallID,
					ID:         ptr.To(item.ToolSearchCall.Id),
					Call:       llmsdk.ToolCallDelta{ToolSearch: &llmsdk.ToolSearchToolCallDelta{Args: &args, Status: &status}},
				},
			}}, nil
		}
		if item.ToolSearchOutput != nil {
			toolCallID := item.ToolSearchOutput.CallId
			if toolCallID == nil {
				toolCallID = state.lastToolSearchCallID
			}
			part := mapOpenAIToolSearchOutput(item.ToolSearchOutput, toolCallID)
			return &llmsdk.ContentDelta{Index: event.ResponseOutputItemDone.OutputIndex, Part: partutil.LooselyConvertPartToPartDelta(part)}, nil
		}
		if item.WebSearchToolCall == nil {
			return nil, nil
		}
		status := mapOpenAIWebSearchCallStatus(item.WebSearchToolCall.Status)
		return &llmsdk.ContentDelta{Index: event.ResponseOutputItemDone.OutputIndex, Part: llmsdk.PartDelta{
			ToolCallPartDelta: &llmsdk.ToolCallPartDelta{
				ToolCallID: ptr.To(item.WebSearchToolCall.Id),
				Call: llmsdk.ToolCallDelta{WebSearch: &llmsdk.WebSearchToolCallDelta{
					Status: &status, Action: mapOpenAIWebSearchAction(item.WebSearchToolCall.Action),
				}},
			},
		}}, nil

	case event.ResponseOutputTextDelta != nil:
		return &llmsdk.ContentDelta{
			Index: event.ResponseOutputTextDelta.OutputIndex,
			Part:  llmsdk.NewTextPartDelta(event.ResponseOutputTextDelta.Delta),
		}, nil

	case event.ResponseOutputTextAnnotationAdded != nil:
		// The generated API leaves streaming annotations untyped, so decode the
		// tagged annotation before mapping a citation delta.
		data, err := json.Marshal(event.ResponseOutputTextAnnotationAdded.Annotation)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal OpenAI citation annotation: %w", err)
		}
		var annotation openaiapi.Annotation
		if err := json.Unmarshal(data, &annotation); err != nil {
			return nil, fmt.Errorf("failed to parse OpenAI citation annotation: %w", err)
		}
		if annotation.UrlCitation == nil {
			return nil, nil
		}
		citation := mapOpenAIURLCitation(*annotation.UrlCitation)
		return &llmsdk.ContentDelta{
			Index: event.ResponseOutputTextAnnotationAdded.OutputIndex,
			Part: llmsdk.PartDelta{TextPartDelta: &llmsdk.TextPartDelta{
				Citation: &llmsdk.CitationDelta{
					Source: &citation.Source, Title: citation.Title, CitedText: citation.CitedText,
					StartIndex: citation.StartIndex, EndIndex: citation.EndIndex, Signature: citation.Signature,
				},
			}},
		}, nil

	case event.ResponseFunctionCallArgumentsDelta != nil:
		// Note: function name is added in "response.output_item.added"
		return &llmsdk.ContentDelta{
			Index: event.ResponseFunctionCallArgumentsDelta.OutputIndex,
			Part:  llmsdk.NewToolCallPartDelta(llmsdk.WithToolCallPartDeltaArgs(event.ResponseFunctionCallArgumentsDelta.Delta)),
		}, nil

	case event.ResponseWebSearchCallInProgress != nil:
		return mapOpenAIWebSearchStatus(event.ResponseWebSearchCallInProgress.OutputIndex, event.ResponseWebSearchCallInProgress.ItemId, llmsdk.WebSearchToolCallStatusInProgress), nil
	case event.ResponseWebSearchCallSearching != nil:
		return mapOpenAIWebSearchStatus(event.ResponseWebSearchCallSearching.OutputIndex, event.ResponseWebSearchCallSearching.ItemId, llmsdk.WebSearchToolCallStatusSearching), nil
	case event.ResponseWebSearchCallCompleted != nil:
		return mapOpenAIWebSearchStatus(event.ResponseWebSearchCallCompleted.OutputIndex, event.ResponseWebSearchCallCompleted.ItemId, llmsdk.WebSearchToolCallStatusCompleted), nil

	case event.ResponseImageGenerationCallPartialImage != nil:
		responseImageGenCallPartialImageEvent := event.ResponseImageGenerationCallPartialImage
		var width, height *int
		if responseImageGenCallPartialImageEvent.Size != nil {
			width, height = parseOpenAIImageSize(string(*responseImageGenCallPartialImageEvent.Size))
		}
		mimeType := "image/png"
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

	case event.ResponseReasoningTextDelta != nil:
		return &llmsdk.ContentDelta{
			Index: event.ResponseReasoningTextDelta.OutputIndex,
			Part:  llmsdk.NewReasoningPartDelta(event.ResponseReasoningTextDelta.Delta),
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

func mapOpenAIStreamWebSearchResult(event openaiapi.ResponseStreamEvent, index int) *llmsdk.ContentDelta {
	if event.ResponseOutputItemDone == nil || event.ResponseOutputItemDone.Item.WebSearchToolCall == nil {
		return nil
	}
	web := event.ResponseOutputItemDone.Item.WebSearchToolCall
	if web.Action.Search == nil || len(web.Action.Search.Sources) == 0 {
		return nil
	}
	sources := make([]llmsdk.WebSearchSource, 0, len(web.Action.Search.Sources))
	for _, source := range web.Action.Search.Sources {
		sources = append(sources, llmsdk.WebSearchSource{URL: source.Url})
	}
	return &llmsdk.ContentDelta{Index: index, Part: llmsdk.PartDelta{
		ToolResultPartDelta: &llmsdk.ToolResultPartDelta{
			ToolCallID: web.Id,
			Result: llmsdk.ToolResult{WebSearch: &llmsdk.WebSearchToolResult{
				Sources: sources,
			}},
			Status: llmsdk.ToolResultStatusCompleted,
		},
	}}
}

func convertToOpenAIWebSearchAction(action llmsdk.WebSearchAction) openaiapi.WebSearchToolCallAction {
	switch action.Type {
	case "search":
		return openaiapi.WebSearchToolCallAction{Search: &openaiapi.WebSearchActionSearch{Queries: action.Queries}}
	case "open_page":
		return openaiapi.WebSearchToolCallAction{OpenPage: &openaiapi.WebSearchActionOpenPage{Url: ptr.To(action.URL)}}
	case "find_in_page":
		return openaiapi.WebSearchToolCallAction{FindInPage: &openaiapi.WebSearchActionFind{Url: action.URL, Pattern: action.Pattern}}
	default:
		return openaiapi.WebSearchToolCallAction{}
	}
}

func mapOpenAIWebSearchAction(action openaiapi.WebSearchToolCallAction) *llmsdk.WebSearchAction {
	if action.Search != nil {
		queries := action.Search.Queries
		if len(queries) == 0 && action.Search.Query != "" {
			queries = []string{action.Search.Query}
		}
		return &llmsdk.WebSearchAction{Type: "search", Queries: queries}
	}
	if action.OpenPage != nil {
		url := ""
		if action.OpenPage.Url != nil {
			url = *action.OpenPage.Url
		}
		return &llmsdk.WebSearchAction{Type: "open_page", URL: url}
	}
	if action.FindInPage != nil {
		return &llmsdk.WebSearchAction{Type: "find_in_page", URL: action.FindInPage.Url, Pattern: action.FindInPage.Pattern}
	}
	return nil
}

func mapOpenAIWebSearchStatus(index int, id string, status llmsdk.WebSearchToolCallStatus) *llmsdk.ContentDelta {
	return &llmsdk.ContentDelta{Index: index, Part: llmsdk.PartDelta{ToolCallPartDelta: &llmsdk.ToolCallPartDelta{
		ToolCallID: &id, Call: llmsdk.ToolCallDelta{WebSearch: &llmsdk.WebSearchToolCallDelta{Status: &status}},
	}}}
}

func mapOpenAIURLCitation(value openaiapi.UrlCitationBody) llmsdk.Citation {
	return llmsdk.Citation{
		Source: value.Url, Title: &value.Title,
		StartIndex: &value.StartIndex, EndIndex: &value.EndIndex,
	}
}

// MARK: - To SDK Usage

func mapOpenAIUsage(usage openaiapi.ResponseUsage, webSearchRequests int) *llmsdk.ModelUsage {
	result := &llmsdk.ModelUsage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		InputTokensDetails: &llmsdk.ModelTokensDetails{
			CachedTokens:     ptr.To(usage.InputTokensDetails.CachedTokens),
			CacheWriteTokens: usage.InputTokensDetails.CacheWriteTokens,
		},
		OutputTokensDetails: &llmsdk.ModelTokensDetails{
			ReasoningTokens: ptr.To(usage.OutputTokensDetails.ReasoningTokens),
		},
	}
	if webSearchRequests > 0 {
		result.ServerToolUse = &llmsdk.ModelServerToolUsage{WebSearchRequests: ptr.To(webSearchRequests)}
	}
	return result
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
