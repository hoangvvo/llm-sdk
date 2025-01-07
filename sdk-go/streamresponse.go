package llmsdk

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
	return false
}

// Current gets the most recent item after Next() returns true.
func (s *StreamResponse) Current() *PartialModelResponse {
	return s.curr
}

// Err returns a terminal error (after channel closes or Next() returns false).
func (s *StreamResponse) Err() error {
	return s.err
}
