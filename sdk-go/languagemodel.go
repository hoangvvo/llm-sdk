package llmsdk

import "context"

type ProviderName string

type LanguageModelMetadata struct {
	Pricing      *LanguageModelPricing     `json:"pricing"`
	Capabilities []LanguageModelCapability `json:"capabilities,omitempty"`
}

type LanguageModel interface {
	Provider() ProviderName
	ModelID() string
	Metadata() *LanguageModelMetadata
	Generate(ctx context.Context, input *LanguageModelInput) (*ModelResponse, error)
	Stream(ctx context.Context, input *LanguageModelInput) (*StreamResponse, error)
}

type StreamResponse struct {
	C    <-chan *PartialModelResponse
	errC <-chan error

	curr *PartialModelResponse
	err  error
}

func NewStreamResponse(c <-chan *PartialModelResponse, errC <-chan error) *StreamResponse {
	return &StreamResponse{
		C:    c,
		errC: errC,
		curr: nil,
		err:  nil,
	}
}

// Next advances the stream and returns true if there is a next item.
func (s *StreamResponse) Next() bool {
	select {
	case partial, ok := <-s.C:
		if !ok {
			// Channel closed, check for error (non-blocking)
			select {
			case err := <-s.errC:
				s.err = err
			default:
				// No error available
			}
			return false
		}
		s.curr = partial
		return true
	case err := <-s.errC:
		s.err = err
		return false
	}
}

// Current gets the most recent item after Next() returns true.
func (s *StreamResponse) Current() *PartialModelResponse {
	return s.curr
}

// Err returns a terminal error (after channel closes or Next() returns false).
func (s *StreamResponse) Err() error {
	return s.err
}
