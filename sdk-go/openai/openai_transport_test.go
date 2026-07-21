package openai_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/openai"
)

func recordedInput() *llmsdk.LanguageModelInput {
	systemPrompt := "Be exact"
	maxTokens := uint32(17)
	temperature := 0.2
	topP := 0.8
	return &llmsdk.LanguageModelInput{
		SystemPrompt: &systemPrompt,
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(llmsdk.NewTextPart("Hello")),
		},
		MaxTokens:   &maxTokens,
		Temperature: &temperature,
		TopP:        &topP,
	}
}

func recordedModel(server *httptest.Server) *openai.OpenAIModel {
	return openai.NewOpenAIModel("recorded-model", openai.OpenAIModelOptions{
		APIKey:  "test-token",
		BaseURL: server.URL + "/v1",
	})
}

func readRequestJSON(t *testing.T, request *http.Request) map[string]any {
	t.Helper()
	defer request.Body.Close()
	body, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	var value map[string]any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	return value
}

func TestOpenAIRecordedTransportGenerateRequestAndResponse(t *testing.T) {
	var requestBody map[string]any
	var authorization string
	var requestPath string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requestPath = request.URL.Path
		authorization = request.Header.Get("Authorization")
		requestBody = readRequestJSON(t, request)
		response.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(response, `{
			"output":[{"type":"web_search_call","id":"ws_1","status":"completed","action":{"type":"search","query":"recorded query","queries":["recorded query"],"sources":[{"type":"url","url":"https://example.com/source"}]}},{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"Recorded response","annotations":[],"logprobs":[]}]}],
			"usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6,"input_tokens_details":{"cached_tokens":1},"output_tokens_details":{"reasoning_tokens":0}}
		}`)
	}))
	t.Cleanup(server.Close)

	result, err := recordedModel(server).Generate(t.Context(), recordedInput())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	expectedRequest := map[string]any{
		"model": "recorded-model",
		"input": []any{
			map[string]any{
				"type": "message",
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": "Hello"},
				},
			},
		},
		"instructions":      "Be exact",
		"max_output_tokens": float64(17),
		"store":             false,
		"stream":            false,
		"temperature":       0.2,
		"top_p":             0.8,
	}
	if !reflect.DeepEqual(requestBody, expectedRequest) {
		t.Fatalf("unexpected request body:\nactual:   %#v\nexpected: %#v", requestBody, expectedRequest)
	}
	if requestPath != "/v1/responses" {
		t.Fatalf("unexpected request path: %q", requestPath)
	}
	if authorization != "Bearer test-token" {
		t.Fatalf("unexpected authorization: %q", authorization)
	}
	if len(result.Content) != 3 || result.Content[0].ToolCallPart == nil || result.Content[0].ToolCallPart.Call.WebSearch == nil || result.Content[1].ToolResultPart == nil || result.Content[1].ToolResultPart.Result.WebSearch == nil || result.Content[2].TextPart == nil {
		t.Fatalf("unexpected content: %#v", result.Content)
	}
	if result.Usage == nil || result.Usage.InputTokens != 4 || result.Usage.OutputTokens != 2 {
		t.Fatalf("unexpected usage: %#v", result.Usage)
	}
}

func TestOpenAIRecordedTransportCancelledToolResult(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requestBody = readRequestJSON(t, request)
		response.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(response, `{"output":[]}`)
	}))
	t.Cleanup(server.Close)

	input := &llmsdk.LanguageModelInput{Messages: []llmsdk.Message{
		llmsdk.NewAssistantMessage(llmsdk.NewToolCallPart("call_1", "wait", map[string]any{})),
		llmsdk.NewToolMessage(llmsdk.NewToolResultPart(
			"call_1",
			"wait",
			[]llmsdk.Part{},
			llmsdk.WithToolResultStatus(llmsdk.ToolResultStatusCancelled),
		)),
	}}
	if _, err := recordedModel(server).Generate(t.Context(), input); err != nil {
		t.Fatalf("generate: %v", err)
	}

	items, ok := requestBody["input"].([]any)
	if !ok {
		t.Fatalf("unexpected input: %#v", requestBody["input"])
	}
	var outputs []any
	for _, item := range items {
		object, ok := item.(map[string]any)
		if ok && object["type"] == "function_call_output" {
			outputs = append(outputs, object)
		}
	}
	expected := []any{map[string]any{
		"type":    "function_call_output",
		"call_id": "call_1",
		"output":  "cancelled",
	}}
	if !reflect.DeepEqual(outputs, expected) {
		t.Fatalf("unexpected tool outputs: %#v", outputs)
	}
}

func TestOpenAIRecordedTransportFragmentedStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if stream, _ := readRequestJSON(t, request)["stream"].(bool); !stream {
			t.Error("stream request did not set stream=true")
		}
		response.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := response.(http.Flusher)
		first := `{"type":"response.output_item.added","output_index":0,"sequence_number":0,"item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"lookup","arguments":"","status":"in_progress"}}`
		_, _ = io.WriteString(response, "data: "+first[:31])
		flusher.Flush()
		_, _ = io.WriteString(response, first[31:]+"\n\n")
		_, _ = io.WriteString(response, `data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":0,"sequence_number":1,"delta":"{\"city\":"}`+"\n\n")
		_, _ = io.WriteString(response, `data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":0,"sequence_number":2,"delta":"\"Hanoi\"}"}`+"\n\n")
		_, _ = io.WriteString(response, `data: {"type":"response.output_item.added","output_index":1,"sequence_number":3,"item":{"type":"web_search_call","id":"ws_1","status":"in_progress"}}`+"\n\n")
		_, _ = io.WriteString(response, `data: {"type":"response.web_search_call.in_progress","item_id":"ws_1","output_index":1,"sequence_number":4}`+"\n\n")
		_, _ = io.WriteString(response, `data: {"type":"response.web_search_call.searching","item_id":"ws_1","output_index":1,"sequence_number":5}`+"\n\n")
		_, _ = io.WriteString(response, `data: {"type":"response.web_search_call.completed","item_id":"ws_1","output_index":1,"sequence_number":6}`+"\n\n")
		_, _ = io.WriteString(response, `data: {"type":"response.output_item.done","output_index":1,"sequence_number":7,"item":{"type":"web_search_call","id":"ws_1","status":"completed","action":{"type":"search","queries":["recorded query"]}}}`+"\n\n")
		_, _ = io.WriteString(response, `data: {"type":"response.completed","sequence_number":8,"response":{"usage":{"input_tokens":7,"output_tokens":3,"total_tokens":10,"input_tokens_details":{"cached_tokens":0},"output_tokens_details":{"reasoning_tokens":0}}}}`+"\n\n")
		_, _ = io.WriteString(response, "data: [DONE]\n\n")
	}))
	t.Cleanup(server.Close)

	stream, err := recordedModel(server).Stream(t.Context(), recordedInput())
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	var partials []*llmsdk.PartialModelResponse
	for stream.Next() {
		partials = append(partials, stream.Current())
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("consume stream: %v", err)
	}
	if len(partials) != 9 {
		t.Fatalf("expected 9 partials, got %d: %#v", len(partials), partials)
	}
	initial := partials[0].Delta.Part.ToolCallPartDelta
	if initial == nil || initial.ToolCallID == nil || *initial.ToolCallID != "call_1" || initial.Call.Function == nil || initial.Call.Function.Name == nil || *initial.Call.Function.Name != "lookup" || initial.Call.Function.Args == nil || *initial.Call.Function.Args != "" {
		t.Fatalf("unexpected initial tool delta: %#v", partials[0])
	}
	if args := partials[1].Delta.Part.ToolCallPartDelta.Call.Function.Args; args == nil || *args != `{"city":` {
		t.Fatalf("unexpected first argument delta: %#v", partials[1])
	}
	if args := partials[2].Delta.Part.ToolCallPartDelta.Call.Function.Args; args == nil || *args != `"Hanoi"}` {
		t.Fatalf("unexpected second argument delta: %#v", partials[2])
	}
	initialWeb := partials[3].Delta.Part.ToolCallPartDelta
	if initialWeb == nil || initialWeb.Call.WebSearch == nil || initialWeb.Call.WebSearch.Action != nil || initialWeb.Call.WebSearch.Status == nil || *initialWeb.Call.WebSearch.Status != llmsdk.WebSearchToolCallStatusInProgress {
		t.Fatalf("unexpected initial web-search delta: %#v", partials[3])
	}
	completedWeb := partials[7].Delta.Part.ToolCallPartDelta
	if completedWeb == nil || completedWeb.Call.WebSearch == nil || completedWeb.Call.WebSearch.Action == nil || completedWeb.Call.WebSearch.Action.Type != "search" || len(completedWeb.Call.WebSearch.Action.Queries) != 1 || completedWeb.Call.WebSearch.Action.Queries[0] != "recorded query" {
		t.Fatalf("unexpected completed web-search delta: %#v", partials[7])
	}
	if partials[8].Delta != nil || partials[8].Usage == nil || partials[8].Usage.InputTokens != 7 || partials[8].Usage.OutputTokens != 3 {
		t.Fatalf("unexpected usage-only partial: %#v", partials[8])
	}
}

func TestOpenAIRecordedTransportFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body := readRequestJSON(t, request)
		if stream, _ := body["stream"].(bool); !stream {
			response.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(response, `{"error":{"message":"rate limited"}}`)
			return
		}
		response.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(response, "data: {\"type\":\n\n")
	}))
	t.Cleanup(server.Close)
	model := recordedModel(server)

	_, err := model.Generate(t.Context(), recordedInput())
	if err == nil || !strings.Contains(err.Error(), "API error (429)") {
		t.Fatalf("expected 429 error, got %v", err)
	}
	stream, err := model.Stream(t.Context(), recordedInput())
	if err != nil {
		t.Fatalf("create malformed stream: %v", err)
	}
	for stream.Next() {
	}
	if err := stream.Err(); err == nil || !strings.Contains(err.Error(), "failed to unmarshal SSE data") {
		t.Fatalf("expected malformed stream error, got %v", err)
	}
}
