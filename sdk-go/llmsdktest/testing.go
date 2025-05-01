package llmsdktest

import (
	"context"
	"errors"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/stream"
)

// MockGenerateResult is a result for a mocked `generate` call.
// It can either be a full response or an error.
type MockGenerateResult struct {
	Response *llmsdk.ModelResponse
	Error    error
}

// NewMockGenerateResultResponse constructs a generate result with a response.
func NewMockGenerateResultResponse(response llmsdk.ModelResponse) MockGenerateResult {
	return MockGenerateResult{
		Response: &response,
	}
}

// NewMockGenerateResultError constructs a generate result that yields an error.
func NewMockGenerateResultError(err error) MockGenerateResult {
	return MockGenerateResult{
		Error: err,
	}
}

// MockStreamResult is a result for a mocked `stream` call.
// It can either be a set of partial responses or an error.
type MockStreamResult struct {
	Partials []llmsdk.PartialModelResponse
	Error    error
}

// NewMockStreamResultPartials constructs a stream result with partial responses.
func NewMockStreamResultPartials(partials []llmsdk.PartialModelResponse) MockStreamResult {
	return MockStreamResult{
		Partials: partials,
	}
}

// NewMockStreamResultError constructs a stream result that yields an error.
func NewMockStreamResultError(err error) MockStreamResult {
	return MockStreamResult{
		Error: err,
	}
}

// MockLanguageModel is a mock language model for testing purposes
// that tracks inputs and returns predefined outputs.
type MockLanguageModel struct {
	mockedGenerateResults []MockGenerateResult
	mockedStreamResults   []MockStreamResult

	trackedGenerateInputs []llmsdk.LanguageModelInput
	trackedStreamInputs   []llmsdk.LanguageModelInput

	provider string
	modelID  string
	metadata *llmsdk.LanguageModelMetadata
}

// NewMockLanguageModel constructs a mock language model instance.
func NewMockLanguageModel() *MockLanguageModel {
	return &MockLanguageModel{
		mockedGenerateResults: []MockGenerateResult{},
		mockedStreamResults:   []MockStreamResult{},
		trackedGenerateInputs: []llmsdk.LanguageModelInput{},
		trackedStreamInputs:   []llmsdk.LanguageModelInput{},
		provider:              "mock",
		modelID:               "mock-model",
	}
}

// Provider returns the provider name of the mock language model.
func (m *MockLanguageModel) Provider() string {
	return m.provider
}

// SetProvider overrides the provider name returned by the mock model.
func (m *MockLanguageModel) SetProvider(provider string) {
	m.provider = provider
}

// ModelID returns the model identifier of the mock language model.
func (m *MockLanguageModel) ModelID() string {
	return m.modelID
}

// SetModelID overrides the model identifier returned by the mock model.
func (m *MockLanguageModel) SetModelID(modelID string) {
	m.modelID = modelID
}

// Metadata returns metadata associated with the mock language model.
func (m *MockLanguageModel) Metadata() *llmsdk.LanguageModelMetadata {
	return m.metadata
}

// SetMetadata overrides the metadata returned by the mock language model.
func (m *MockLanguageModel) SetMetadata(metadata *llmsdk.LanguageModelMetadata) {
	m.metadata = metadata
}

// Generate returns the next mocked generate result, tracking the provided input.
func (m *MockLanguageModel) Generate(_ context.Context, input *llmsdk.LanguageModelInput) (*llmsdk.ModelResponse, error) {
	if len(m.mockedGenerateResults) == 0 {
		return nil, errors.New("no mocked generate results available")
	}

	result := m.mockedGenerateResults[0]
	m.mockedGenerateResults = m.mockedGenerateResults[1:]
	m.trackedGenerateInputs = append(m.trackedGenerateInputs, *input)

	if result.Error != nil {
		return nil, result.Error
	}

	return result.Response, nil
}

// Stream returns the next mocked stream result as a LanguageModelStream, tracking the input.
func (m *MockLanguageModel) Stream(_ context.Context, input *llmsdk.LanguageModelInput) (*llmsdk.LanguageModelStream, error) {
	if len(m.mockedStreamResults) == 0 {
		return nil, errors.New("no mocked stream results available")
	}

	result := m.mockedStreamResults[0]
	m.mockedStreamResults = m.mockedStreamResults[1:]
	m.trackedStreamInputs = append(m.trackedStreamInputs, *input)

	if result.Error != nil {
		return nil, result.Error
	}

	eventChan := make(chan *llmsdk.PartialModelResponse)
	errChan := make(chan error)

	partials := result.Partials

	go func() {
		defer close(eventChan)
		defer close(errChan)

		for _, partial := range partials {
			p := partial
			eventChan <- &p
		}
	}()

	return stream.New(eventChan, errChan), nil
}

// EnqueueGenerateResult enqueues generate results to be returned sequentially.
func (m *MockLanguageModel) EnqueueGenerateResult(results ...MockGenerateResult) {
	m.mockedGenerateResults = append(m.mockedGenerateResults, results...)
}

// EnqueueStreamResult enqueues stream results to be returned sequentially.
func (m *MockLanguageModel) EnqueueStreamResult(results ...MockStreamResult) {
	m.mockedStreamResults = append(m.mockedStreamResults, results...)
}

// TrackedGenerateInputs returns the list of inputs tracked from Generate calls.
func (m *MockLanguageModel) TrackedGenerateInputs() []llmsdk.LanguageModelInput {
	return m.trackedGenerateInputs
}

// TrackedStreamInputs returns the list of inputs tracked from Stream calls.
func (m *MockLanguageModel) TrackedStreamInputs() []llmsdk.LanguageModelInput {
	return m.trackedStreamInputs
}

// Reset clears tracked inputs without touching enqueued results.
func (m *MockLanguageModel) Reset() {
	m.trackedGenerateInputs = []llmsdk.LanguageModelInput{}
	m.trackedStreamInputs = []llmsdk.LanguageModelInput{}
}

// Restore clears enqueued results and tracked inputs, returning the mock to its initial state.
func (m *MockLanguageModel) Restore() {
	m.mockedGenerateResults = []MockGenerateResult{}
	m.mockedStreamResults = []MockStreamResult{}
	m.Reset()
}
