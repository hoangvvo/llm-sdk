package google_test

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/google"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/testcommon"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"
	"github.com/joho/godotenv"
)

var model *google.GoogleModel
var audioModel *google.GoogleModel
var imageModel *google.GoogleModel
var reasoningModel *google.GoogleModel
var vertexGlobalModel *google.GoogleModel
var vertexProjectModel *google.GoogleModel

func TestMain(m *testing.M) {
	godotenv.Load("../../.env")
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		panic("GOOGLE_API_KEY must be set")
	}

	model = google.NewGoogleModel("gemini-3-flash-preview", google.GoogleModelOptions{
		APIKey: apiKey,
	})
	audioModel = google.NewGoogleModel("gemini-2.5-flash-preview-tts", google.GoogleModelOptions{
		APIKey: apiKey,
	})
	imageModel = google.NewGoogleModel("gemini-2.5-flash-image-preview", google.GoogleModelOptions{
		APIKey: apiKey,
	})
	reasoningModel = google.NewGoogleModel("gemini-3-flash-preview", google.GoogleModelOptions{
		APIKey: apiKey,
	})

	vertexAccessToken := os.Getenv("VERTEX_ACCESS_TOKEN")
	vertexAPIKey := os.Getenv("VERTEX_API_KEY")
	if vertexAccessToken != "" || vertexAPIKey != "" {
		globalOptions := google.GoogleModelOptions{
			ProviderType: google.ProviderTypeVertexAI,
		}
		if vertexAccessToken != "" {
			globalOptions.AccessToken = vertexAccessToken
		} else if vertexAPIKey != "" {
			globalOptions.APIKey = vertexAPIKey
		}
		vertexGlobalModel = google.NewGoogleModel("gemini-2.5-flash-lite", globalOptions)

		vertexProjectID := os.Getenv("VERTEX_PROJECT_ID")
		vertexLocation := os.Getenv("VERTEX_LOCATION")
		if vertexProjectID != "" && vertexLocation != "" && vertexAccessToken != "" {
			globalOptions.ProjectID = vertexProjectID
			globalOptions.Location = vertexLocation
			vertexProjectModel = google.NewGoogleModel("gemini-2.5-flash-lite", globalOptions)
		}
	}

	m.Run()
}

func TestGenerateText(t *testing.T) {
	testcommon.RunTestCase(t, model, "generate_text")
}

func TestStreamText(t *testing.T) {
	testcommon.RunTestCase(t, model, "stream_text")
}

func TestGenerateWithSystemPrompt(t *testing.T) {
	testcommon.RunTestCase(t, model, "generate_with_system_prompt")
}

func TestGenerateToolCall(t *testing.T) {
	testcommon.RunTestCase(t, model, "generate_tool_call")
}

func TestStreamToolCall(t *testing.T) {
	testcommon.RunTestCase(t, model, "stream_tool_call")
}

func TestGenerateTextWithToolResult(t *testing.T) {
	testcommon.RunTestCase(t, model, "generate_text_from_tool_result")
}

func TestStreamTextWithToolResult(t *testing.T) {
	testcommon.RunTestCase(t, model, "stream_text_from_tool_result")
}

func TestGenerateParallelToolCalls(t *testing.T) {
	testcommon.RunTestCase(t, model, "generate_parallel_tool_calls")
}

func TestStreamParallelToolCalls(t *testing.T) {
	testcommon.RunTestCase(t, model, "stream_parallel_tool_calls")
}

func TestStreamParallelToolCallsOfSameName(t *testing.T) {
	testcommon.RunTestCase(t, model, "stream_parallel_tool_calls_of_same_name")
}

func TestStructuredResponseFormat(t *testing.T) {
	testcommon.RunTestCase(t, model, "structured_response_format")
}

func TestSourcePartInput(t *testing.T) {
	testcommon.RunTestCase(t, model, "source_part_input")
}

func TestGenerateImage(t *testing.T) {
	testcommon.RunTestCase(t, imageModel, "generate_image")
}

func TestStreamImage(t *testing.T) {
	testcommon.RunTestCase(t, imageModel, "stream_image")
}

func TestGenerateImageInput(t *testing.T) {
	testcommon.RunTestCase(t, imageModel, "generate_image_input")
}

func TestStreamImageInput(t *testing.T) {
	testcommon.RunTestCase(t, imageModel, "stream_image_input")
}

func TestGenerateAudio(t *testing.T) {
	testcommon.RunTestCase(t, audioModel, "generate_audio",
		testcommon.WithAdditionalInput(func(lmi *llmsdk.LanguageModelInput) {
			lmi.Modalities = []llmsdk.Modality{llmsdk.ModalityAudio}
			lmi.Audio = &llmsdk.AudioOptions{Voice: ptr.To("Zephyr")}
		}),
		testcommon.WithCustomOutputContent(func(content []testcommon.PartAssertion) []testcommon.PartAssertion {
			newContent := []testcommon.PartAssertion{}
			for _, part := range content {
				if part.AudioPart != nil {
					part.AudioPart.AudioID = false
					part.AudioPart.Transcript = nil
				}
				newContent = append(newContent, part)
			}
			return newContent
		}),
	)
}

func TestStreamAudio(t *testing.T) {
	testcommon.RunTestCase(
		t, audioModel, "stream_audio",
		testcommon.WithAdditionalInput(func(lmi *llmsdk.LanguageModelInput) {
			lmi.Modalities = []llmsdk.Modality{llmsdk.ModalityAudio}
			lmi.Audio = &llmsdk.AudioOptions{Voice: ptr.To("Zephyr")}
		}),
		testcommon.WithCustomOutputContent(func(content []testcommon.PartAssertion) []testcommon.PartAssertion {
			newContent := []testcommon.PartAssertion{}
			for _, part := range content {
				if part.AudioPart != nil {
					part.AudioPart.AudioID = false
					part.AudioPart.Transcript = nil
				}
				newContent = append(newContent, part)
			}
			return newContent
		}),
	)
}

func TestGenerateReasoning(t *testing.T) {
	testcommon.RunTestCase(t, reasoningModel, "generate_reasoning")
}

func TestStreamReasoning(t *testing.T) {
	testcommon.RunTestCase(t, reasoningModel, "stream_reasoning")
}

func TestGenerateThoughtSignatures(t *testing.T) {
	ctx := t.Context()

	input := &llmsdk.LanguageModelInput{
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(llmsdk.NewTextPart("What's the weather like in San Francisco?")),
		},
		Tools: []llmsdk.Tool{testcommon.GetWeatherTool()},
	}

	result, err := model.Generate(ctx, input)
	if err != nil {
		t.Fatalf("Initial generate failed: %v", err)
	}

	var toolCallPart *llmsdk.ToolCallPart
	for _, part := range result.Content {
		if part.ToolCallPart != nil {
			toolCallPart = part.ToolCallPart
			break
		}
	}

	if toolCallPart == nil {
		t.Fatal("Expected a tool call in the response")
	}

	if toolCallPart.ThoughtSignature == nil {
		t.Fatal("Expected thought signature on tool call for Gemini 3 model")
	}

	input2 := &llmsdk.LanguageModelInput{
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(llmsdk.NewTextPart("What's the weather like in San Francisco?")),
			llmsdk.NewAssistantMessage(llmsdk.Part{ToolCallPart: toolCallPart}),
			llmsdk.NewToolMessage(llmsdk.NewToolResultPart(
				toolCallPart.ToolCallID,
				toolCallPart.ToolName,
				[]llmsdk.Part{llmsdk.NewTextPart(`{"temperature": 65, "unit": "f", "description": "Foggy"}`)},
			)),
		},
		Tools: []llmsdk.Tool{testcommon.GetWeatherTool()},
	}

	result2, err := model.Generate(ctx, input2)
	if err != nil {
		t.Fatalf("Generate with tool result failed: %v", err)
	}

	var hasTextPart bool
	for _, part := range result2.Content {
		if part.TextPart != nil {
			hasTextPart = true
			break
		}
	}

	if !hasTextPart {
		t.Fatal("Expected text response after providing tool result")
	}
}

func TestStreamThoughtSignatures(t *testing.T) {
	ctx := t.Context()

	input := &llmsdk.LanguageModelInput{
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(llmsdk.NewTextPart("What's the stock price of GOOG?")),
		},
		Tools: []llmsdk.Tool{testcommon.GetStockPriceTool()},
	}

	stream, err := model.Stream(ctx, input)
	if err != nil {
		t.Fatalf("Initial stream failed: %v", err)
	}

	accumulator := llmsdk.NewStreamAccumulator()
	for stream.Next() {
		partial := stream.Current()
		if err := accumulator.AddPartial(*partial); err != nil {
			t.Fatalf("Failed to add partial: %v", err)
		}
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("Stream error: %v", err)
	}

	result, err := accumulator.ComputeResponse()
	if err != nil {
		t.Fatalf("Failed to compute response: %v", err)
	}

	var toolCallPart *llmsdk.ToolCallPart
	for _, part := range result.Content {
		if part.ToolCallPart != nil {
			toolCallPart = part.ToolCallPart
			break
		}
	}

	if toolCallPart == nil {
		t.Fatal("Expected a tool call in the streamed response")
	}

	if toolCallPart.ThoughtSignature == nil {
		t.Fatal("Expected thought signature on streamed tool call for Gemini 3 model")
	}

	input2 := &llmsdk.LanguageModelInput{
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(llmsdk.NewTextPart("What's the stock price of GOOG?")),
			llmsdk.NewAssistantMessage(llmsdk.Part{ToolCallPart: toolCallPart}),
			llmsdk.NewToolMessage(llmsdk.NewToolResultPart(
				toolCallPart.ToolCallID,
				toolCallPart.ToolName,
				[]llmsdk.Part{llmsdk.NewTextPart(`{"price": 175.50, "currency": "USD"}`)},
			)),
		},
		Tools: []llmsdk.Tool{testcommon.GetStockPriceTool()},
	}

	stream2, err := model.Stream(ctx, input2)
	if err != nil {
		t.Fatalf("Stream with tool result failed: %v", err)
	}

	accumulator2 := llmsdk.NewStreamAccumulator()
	for stream2.Next() {
		partial := stream2.Current()
		if err := accumulator2.AddPartial(*partial); err != nil {
			t.Fatalf("Failed to add partial: %v", err)
		}
	}
	if err := stream2.Err(); err != nil {
		t.Fatalf("Stream error: %v", err)
	}

	result2, err := accumulator2.ComputeResponse()
	if err != nil {
		t.Fatalf("Failed to compute response: %v", err)
	}

	var hasTextPart bool
	for _, part := range result2.Content {
		if part.TextPart != nil {
			hasTextPart = true
			break
		}
	}

	if !hasTextPart {
		t.Fatal("Expected text response after providing tool result in stream")
	}
}

type roundTripFunc func(req *http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func TestVertexAIGlobalGenerateText(t *testing.T) {
	if vertexGlobalModel == nil {
		t.Skip("VERTEX_ACCESS_TOKEN not set")
	}
	testcommon.RunTestCase(t, vertexGlobalModel, "generate_text")
}

func TestVertexAIGlobalStreamText(t *testing.T) {
	if vertexGlobalModel == nil {
		t.Skip("VERTEX_ACCESS_TOKEN not set")
	}
	testcommon.RunTestCase(t, vertexGlobalModel, "stream_text")
}

func TestVertexAIProjectGenerateText(t *testing.T) {
	if vertexProjectModel == nil {
		t.Skip("VERTEX_PROJECT_ID, VERTEX_LOCATION or VERTEX_ACCESS_TOKEN not set")
	}
	testcommon.RunTestCase(t, vertexProjectModel, "generate_text")
}

func TestVertexAIProjectStreamText(t *testing.T) {
	if vertexProjectModel == nil {
		t.Skip("VERTEX_PROJECT_ID, VERTEX_LOCATION or VERTEX_ACCESS_TOKEN not set")
	}
	testcommon.RunTestCase(t, vertexProjectModel, "stream_text")
}

func TestVertexAIURLAndHeaders(t *testing.T) {
	type vertexURLHeaderTestCase struct {
		name        string
		modelName   string
		options     google.GoogleModelOptions
		expectedURL string
		headers     map[string]string
	}
	testCases := []vertexURLHeaderTestCase{
		{
			name:      "Global",
			modelName: "gemini-3-flash",
			options: google.GoogleModelOptions{
				APIKey:       "test-api-key",
				ProviderType: google.ProviderTypeVertexAI,
				APIVersion:   "v1",
			},
			expectedURL: "https://aiplatform.googleapis.com/v1/publishers/google/models/gemini-3-flash:generateContent",
			headers:     map[string]string{"x-goog-api-key": "test-api-key"},
		},
		{
			name:      "Project",
			modelName: "gemini-3-flash",
			options: google.GoogleModelOptions{
				AccessToken:  "test-key",
				ProjectID:    "test-project",
				Location:     "us-central1",
				ProviderType: google.ProviderTypeVertexAI,
			},
			expectedURL: "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/test-project/locations/us-central1/publishers/google/models/gemini-3-flash:generateContent",
			headers:     map[string]string{"Authorization": "Bearer test-key"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := tc.options
			opts.HTTPClient = &http.Client{
				Transport: roundTripFunc(func(req *http.Request) *http.Response {
					if req.URL.String() != tc.expectedURL {
						t.Errorf("expected URL %s, got %s", tc.expectedURL, req.URL.String())
					}
					for header, expected := range tc.headers {
						if got := req.Header.Get(header); got != expected {
							t.Errorf("expected header %s=%q, got %q", header, expected, got)
						}
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"Hello"}]}}]}`)),
						Header:     make(http.Header),
					}
				}),
			}

			m := google.NewGoogleModel(tc.modelName, opts)
			_, err := m.Generate(context.Background(), &llmsdk.LanguageModelInput{
				Messages: []llmsdk.Message{
					llmsdk.NewUserMessage(llmsdk.NewTextPart("Hi")),
				},
			})
			if err != nil {
				t.Fatalf("Generate failed: %v", err)
			}
		})
	}
}
