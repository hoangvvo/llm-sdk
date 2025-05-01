package llmsdk

import (
	"context"
	"errors"

	"github.com/hoangvvo/llm-sdk/sdk-go/utils/stream"
)

// MockGenerateResult is a result for a mocked `generate` call.
// It can either be a full response or an error.
type MockGenerateResult struct {
	Response *ModelResponse
	Error    error
}

// NewMockGenerateResultResponse constructs a generate result with a response.
func NewMockGenerateResultResponse(response ModelResponse) MockGenerateResult {
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
	Partials []PartialModelResponse
	Error    error
}

// NewMockStreamResultPartials constructs a stream result with partial responses.
func NewMockStreamResultPartials(partials []PartialModelResponse) MockStreamResult {
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

	TrackedGenerateInputs []*LanguageModelInput
	TrackedStreamInputs   []*LanguageModelInput

	provider ProviderName
	modelID  string
	metadata *LanguageModelMetadata
}

// NewMockLanguageModel constructs a mock language model instance.
func NewMockLanguageModel() *MockLanguageModel {
	return &MockLanguageModel{
		mockedGenerateResults: []MockGenerateResult{},
		mockedStreamResults:   []MockStreamResult{},
		TrackedGenerateInputs: []*LanguageModelInput{},
		TrackedStreamInputs:   []*LanguageModelInput{},
		provider:              ProviderName("mock"),
		modelID:               "mock-model",
	}
}

// Provider returns the provider name of the mock language model.
func (m *MockLanguageModel) Provider() ProviderName {
	return m.provider
}

// SetProvider overrides the provider name returned by the mock model.
func (m *MockLanguageModel) SetProvider(provider ProviderName) {
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
func (m *MockLanguageModel) Metadata() *LanguageModelMetadata {
	return m.metadata
}

// SetMetadata overrides the metadata returned by the mock language model.
func (m *MockLanguageModel) SetMetadata(metadata *LanguageModelMetadata) {
	m.metadata = metadata
}

// Generate returns the next mocked generate result, tracking the provided input.
func (m *MockLanguageModel) Generate(_ context.Context, input *LanguageModelInput) (*ModelResponse, error) {
	if len(m.mockedGenerateResults) == 0 {
		return nil, errors.New("no mocked generate results available")
	}

	result := m.mockedGenerateResults[0]
	m.mockedGenerateResults = m.mockedGenerateResults[1:]
	m.TrackedGenerateInputs = append(m.TrackedGenerateInputs, input)

	if result.Error != nil {
		return nil, result.Error
	}

	return result.Response, nil
}

// Stream returns the next mocked stream result as a LanguageModelStream, tracking the input.
func (m *MockLanguageModel) Stream(_ context.Context, input *LanguageModelInput) (*LanguageModelStream, error) {
	if len(m.mockedStreamResults) == 0 {
		return nil, errors.New("no mocked stream results available")
	}

	result := m.mockedStreamResults[0]
	m.mockedStreamResults = m.mockedStreamResults[1:]
	m.TrackedStreamInputs = append(m.TrackedStreamInputs, input)

	if result.Error != nil {
		return nil, result.Error
	}

	m.TrackedStreamInputs = append(m.TrackedStreamInputs, input)

	eventChan := make(chan *PartialModelResponse)
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

// Reset clears tracked inputs without touching enqueued results.
func (m *MockLanguageModel) Reset() {
	m.TrackedGenerateInputs = []*LanguageModelInput{}
	m.TrackedStreamInputs = []*LanguageModelInput{}
}

// Restore clears enqueued results and tracked inputs, returning the mock to its initial state.
func (m *MockLanguageModel) Restore() {
	m.mockedGenerateResults = []MockGenerateResult{}
	m.mockedStreamResults = []MockStreamResult{}
	m.Reset()
}
