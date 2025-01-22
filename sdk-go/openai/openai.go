package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/ptr"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/sse"
)

const (
	Provider              = "openai"
	OpenAIAudioSampleRate = 24000
	OpenAIAudioChannels   = 1
	DefaultBaseURL        = "https://api.openai.com/v1"
)

// OpenAIModelOptions represents configuration options for OpenAI model
type OpenAIModelOptions struct {
	BaseURL string
	APIKey  string
	ModelID string
}

// OpenAIModel implements the LanguageModel interface for OpenAI
type OpenAIModel struct {
	modelID  string
	apiKey   string
	baseURL  string
	client   *http.Client
	metadata *llmsdk.LanguageModelMetadata
}

// NewOpenAIModel creates a new OpenAI model instance
func NewOpenAIModel(options OpenAIModelOptions) *OpenAIModel {
	baseURL := options.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	return &OpenAIModel{
		modelID: options.ModelID,
		apiKey:  options.APIKey,
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

func (m *OpenAIModel) WithMetadata(metadata *llmsdk.LanguageModelMetadata) *OpenAIModel {
	m.metadata = metadata
	return m
}

// Provider returns the provider name
func (m *OpenAIModel) Provider() llmsdk.ProviderName {
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

// Generate implements synchronous generation
func (m *OpenAIModel) Generate(ctx context.Context, input *llmsdk.LanguageModelInput) (*llmsdk.ModelResponse, error) {
	params, err := convertToOpenAICreateParams(input, m.modelID)
	if err != nil {
		return nil, err
	}

	reqBody, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/chat/completions", m.baseURL), bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", m.apiKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var completion ChatCompletion
	if err := json.Unmarshal(respBody, &completion); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
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

	return &llmsdk.ModelResponse{
		Content: content,
		Usage:   usage,
	}, nil
}

// Stream implements streaming generation
func (m *OpenAIModel) Stream(ctx context.Context, input *llmsdk.LanguageModelInput) (*llmsdk.StreamResponse, error) {
	params, err := convertToOpenAICreateParams(input, m.modelID)
	if err != nil {
		return nil, err
	}

	// Enable streaming
	params.Stream = ptr.To(true)

	reqBody, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/chat/completions", m.baseURL), bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", m.apiKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	responseCh := make(chan *llmsdk.PartialModelResponse)
	errCh := make(chan error, 1)

	go func() {
		defer close(responseCh)
		defer close(errCh)
		defer resp.Body.Close()

		var allContentDeltas []llmsdk.ContentDelta
		scanner := sse.NewScanner(resp.Body)

		for scanner.Scan() {
			line := scanner.Text()

			if line == "" || strings.HasPrefix(line, ":") {
				continue // Skip empty lines and comments
			}

			if data, ok := sse.IsDataLine(line); ok {
				if data == "[DONE]" {
					break // End of stream
				}

				var chunk ChatCompletionChunk
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					errCh <- fmt.Errorf("failed to parse chunk: %w", err)
					return
				}

				if len(chunk.Choices) > 0 {
					choice := chunk.Choices[0]

					if choice.Delta.Refusal != nil && *choice.Delta.Refusal != "" {
						errCh <- fmt.Errorf("request was refused: %s", *choice.Delta.Refusal)
						return
					}

					incomingDeltas := m.mapOpenAIDelta(choice.Delta, allContentDeltas, params)
					allContentDeltas = slices.Concat(allContentDeltas, incomingDeltas)

					for _, delta := range incomingDeltas {
						responseCh <- &llmsdk.PartialModelResponse{
							Delta: &delta,
						}
					}
				}

				if chunk.Usage != nil {
					usage := &llmsdk.ModelUsage{
						InputTokens:  int(chunk.Usage.PromptTokens),
						OutputTokens: int(chunk.Usage.CompletionTokens),
					}
					responseCh <- &llmsdk.PartialModelResponse{
						Usage: usage,
					}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			errCh <- fmt.Errorf("scanner error: %w", err)
		}
	}()

	return llmsdk.NewStreamResponse(responseCh, errCh), nil
}

// MARK: - Convert To OpenAI API Types

// convertToOpenAICreateParams converts LanguageModelInput to OpenAI parameters
func convertToOpenAICreateParams(input *llmsdk.LanguageModelInput, modelID string) (*ChatCompletionCreateParams, error) {
	messages, err := convertToOpenAIMessages(input.Messages, input.SystemPrompt)
	if err != nil {
		return nil, err
	}

	params := &ChatCompletionCreateParams{
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
		var tools []ChatCompletionTool
		for _, tool := range input.Tools {
			openAITool := ChatCompletionTool{
				Function: &ChatCompletionFunctionTool{
					Function: FunctionDefinition{
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
		var modalities []OpenAIModality
		for _, modality := range input.Modalities {
			modalities = append(modalities, convertToOpenAIModality(modality))
		}
		params.Modalities = modalities
	}

	// Handle audio parameter from extra
	if input.Extra != nil {
		if audioValue, exists := input.Extra["audio"]; exists {
			audioBytes, _ := json.Marshal(audioValue)
			var audioParam ChatCompletionAudioParam
			if json.Unmarshal(audioBytes, &audioParam) == nil {
				params.Audio = &audioParam
			}
		}
	}

	return params, nil
}

// MARK: - To Provider Messages

// convertToOpenAIMessages converts messages to OpenAI format
func convertToOpenAIMessages(messages []llmsdk.Message, systemPrompt *string) ([]ChatCompletionMessageParam, error) {
	var openAIMessages []ChatCompletionMessageParam

	// Add system prompt if provided
	if systemPrompt != nil && *systemPrompt != "" {
		openAIMessages = append(openAIMessages, ChatCompletionMessageParam{
			System: &ChatCompletionSystemMessageParam{
				Content: []SystemContentPart{
					{
						Text: &ChatCompletionContentPartText{
							Text: *systemPrompt,
						},
					},
				},
			},
		})
	}

	for _, message := range messages {
		switch message.Role() {
		case llmsdk.RoleUser:
			if message.UserMessage == nil {
				return nil, fmt.Errorf("invalid user message")
			}

			var content []ChatCompletionContentPart
			for _, part := range message.UserMessage.Content {
				openAIPart, err := convertToOpenAIContentPart(part)
				if err != nil {
					return nil, err
				}
				content = append(content, *openAIPart)
			}

			openAIMessages = append(openAIMessages, ChatCompletionMessageParam{
				User: &ChatCompletionUserMessageParam{
					Content: content,
				},
			})

		case llmsdk.RoleAssistant:
			if message.AssistantMessage == nil {
				return nil, fmt.Errorf("invalid assistant message")
			}

			assistantMsg := &ChatCompletionAssistantMessageParam{}

			for _, part := range message.AssistantMessage.Content {
				switch part.Type() {
				case llmsdk.PartTypeText:
					if assistantMsg.Content == nil {
						assistantMsg.Content = []AssistantContentPart{}
					}
					assistantMsg.Content = append(assistantMsg.Content, AssistantContentPart{
						Text: &ChatCompletionContentPartText{
							Text: part.TextPart.Text,
						},
					})

				case llmsdk.PartTypeToolCall:
					if assistantMsg.ToolCalls == nil {
						assistantMsg.ToolCalls = []ChatCompletionMessageToolCall{}
					}

					args, _ := json.Marshal(part.ToolCallPart.Args)
					toolCall := ChatCompletionMessageToolCall{
						Function: &ChatCompletionMessageFunctionToolCall{
							ID: part.ToolCallPart.ToolCallID,
							Function: ChatCompletionMessageFunctionToolCallFunction{
								Name:      part.ToolCallPart.ToolName,
								Arguments: string(args),
							},
						},
					}
					assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, toolCall)

				case llmsdk.PartTypeAudio:
					if part.AudioPart.ID != nil {
						assistantMsg.Audio = &ChatCompletionAssistantMessageParamAudio{
							ID: *part.AudioPart.ID,
						}
					}
				}
			}

			openAIMessages = append(openAIMessages, ChatCompletionMessageParam{
				Assistant: assistantMsg,
			})

		case llmsdk.RoleTool:
			if message.ToolMessage == nil {
				return nil, fmt.Errorf("invalid tool message")
			}

			for _, part := range message.ToolMessage.Content {
				if part.Type() != llmsdk.PartTypeToolResult {
					return nil, fmt.Errorf("tool message must only contain tool result parts")
				}

				var content []ToolContentPart
				for _, contentPart := range part.ToolResultPart.Content {
					if contentPart.Type() == llmsdk.PartTypeText {
						content = append(content, ToolContentPart{
							Text: &ChatCompletionContentPartText{
								Text: contentPart.TextPart.Text,
							},
						})
					}
				}

				openAIMessages = append(openAIMessages, ChatCompletionMessageParam{
					Tool: &ChatCompletionToolMessageParam{
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
func convertToOpenAIContentPart(part llmsdk.Part) (*ChatCompletionContentPart, error) {
	switch part.Type() {
	case llmsdk.PartTypeText:
		return &ChatCompletionContentPart{
			Text: &ChatCompletionContentPartText{
				Text: part.TextPart.Text,
			},
		}, nil

	case llmsdk.PartTypeImage:
		return &ChatCompletionContentPart{
			Image: &ChatCompletionContentPartImage{
				ImageURL: ChatCompletionContentPartImageImageURL{
					URL: fmt.Sprintf("data:%s;base64,%s", part.ImagePart.MimeType, part.ImagePart.ImageData),
				},
			},
		}, nil

	case llmsdk.PartTypeAudio:
		var format AudioInputFormat
		switch part.AudioPart.Format {
		case llmsdk.AudioFormatWav:
			format = AudioInputFormatWav
		case llmsdk.AudioFormatMP3:
			format = AudioInputFormatMp3
		default:
			return nil, fmt.Errorf("unsupported audio format for OpenAI: %s", part.AudioPart.Format)
		}

		return &ChatCompletionContentPart{
			InputAudio: &ChatCompletionContentPartInputAudio{
				InputAudio: ChatCompletionContentPartInputAudioInputAudio{
					Data:   part.AudioPart.AudioData,
					Format: format,
				},
			},
		}, nil

	default:
		return nil, fmt.Errorf("unsupported part type for OpenAI: %s", part.Type())
	}
}

// MARK: - To Provider Tools

// convertToOpenAIToolChoice converts ToolChoiceOption to OpenAI format
func convertToOpenAIToolChoice(toolChoice llmsdk.ToolChoiceOption) *ChatCompletionToolChoiceOption {
	if toolChoice.None != nil {
		return &ChatCompletionToolChoiceOption{None: ptr.To(true)}
	}
	if toolChoice.Auto != nil {
		return &ChatCompletionToolChoiceOption{Auto: ptr.To(true)}
	}
	if toolChoice.Required != nil {
		return &ChatCompletionToolChoiceOption{Required: ptr.To(true)}
	}
	if toolChoice.Tool != nil {
		return &ChatCompletionToolChoiceOption{
			Named: &ChatCompletionNamedToolChoice{
				Function: ChatCompletionNamedToolChoiceFunction{
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
func convertToOpenAIResponseFormat(responseFormat llmsdk.ResponseFormatOption) *OpenAIResponseFormat {
	if responseFormat.Text != nil {
		return &OpenAIResponseFormat{Text: ptr.To(true)}
	}
	if responseFormat.JSON != nil {
		if responseFormat.JSON.Schema != nil {
			return &OpenAIResponseFormat{
				JsonSchema: &ResponseFormatJSONSchema{
					JsonSchema: ResponseFormatJSONSchemaJSONSchema{
						Name:        responseFormat.JSON.Name,
						Description: responseFormat.JSON.Description,
						Schema:      responseFormat.JSON.Schema,
						Strict:      ptr.To(true),
					},
				},
			}
		} else {
			return &OpenAIResponseFormat{JsonObject: ptr.To(true)}
		}
	}
	return nil
}

// MARK: - To Provider Modality

// convertToOpenAIModality converts SDK modality to OpenAI format
func convertToOpenAIModality(modality llmsdk.Modality) OpenAIModality {
	switch modality {
	case llmsdk.ModalityText:
		return OpenAIModalityText
	case llmsdk.ModalityAudio:
		return OpenAIModalityAudio
	default:
		return OpenAIModalityText
	}
}

// MARK: - Map From OpenAI API Types

// mapOpenAIMessage converts OpenAI message to SDK parts
func (m *OpenAIModel) mapOpenAIMessage(message ChatCompletionMessage, createParams *ChatCompletionCreateParams) ([]llmsdk.Part, error) {
	var parts []llmsdk.Part

	if message.Content != nil && *message.Content != "" {
		parts = append(parts, llmsdk.NewTextPart(*message.Content, nil))
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
				nil,
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
			ptr.To(OpenAIAudioSampleRate),
			ptr.To(OpenAIAudioChannels),
			&message.Audio.Transcript,
			&message.Audio.ID,
		)
		parts = append(parts, audioPart)
	}

	return parts, nil
}

// mapOpenAIAudioFormat converts OpenAI audio format to SDK format
func mapOpenAIAudioFormat(format AudioOutputFormat) llmsdk.AudioFormat {
	switch format {
	case AudioOutputFormatWav:
		return llmsdk.AudioFormatWav
	case AudioOutputFormatMp3:
		return llmsdk.AudioFormatMP3
	case AudioOutputFormatAac:
		return llmsdk.AudioFormatAAC
	case AudioOutputFormatFlac:
		return llmsdk.AudioFormatFLAC
	case AudioOutputFormatOpus:
		return llmsdk.AudioFormatOpus
	case AudioOutputFormatPcm16:
		return llmsdk.AudioFormatLinear16
	default:
		return llmsdk.AudioFormatLinear16
	}
}

// MARK: - To SDK Delta

// mapOpenAIDelta converts OpenAI delta to SDK content deltas
func (m *OpenAIModel) mapOpenAIDelta(delta ChatCompletionChunkChoiceDelta, existingDeltas []llmsdk.ContentDelta, createParams *ChatCompletionCreateParams) []llmsdk.ContentDelta {
	var contentDeltas []llmsdk.ContentDelta

	if delta.Content != nil && *delta.Content != "" {
		textDelta := llmsdk.TextPartDelta{
			Text: *delta.Content,
		}

		// Find the appropriate index for text content
		index := llmsdk.GuessDeltaIndex(llmsdk.PartDelta{TextPartDelta: &textDelta}, slices.Concat(existingDeltas, contentDeltas), nil)

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
			audioDelta.SampleRate = ptr.To(OpenAIAudioSampleRate)
			audioDelta.Channels = ptr.To(OpenAIAudioChannels)
		}
		if delta.Audio.Transcript != nil {
			audioDelta.Transcript = delta.Audio.Transcript
		}

		index := llmsdk.GuessDeltaIndex(llmsdk.PartDelta{AudioPartDelta: &audioDelta}, slices.Concat(existingDeltas, contentDeltas), nil)

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

		index := llmsdk.GuessDeltaIndex(llmsdk.PartDelta{ToolCallPartDelta: &toolCallDelta}, slices.Concat(existingDeltas, contentDeltas), indexHint)

		contentDeltas = append(contentDeltas, llmsdk.ContentDelta{
			Index: index,
			Part:  llmsdk.PartDelta{ToolCallPartDelta: &toolCallDelta},
		})
	}

	return contentDeltas
}
