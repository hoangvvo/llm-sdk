package stream

// Stream represents a generic stream of data.
type Stream[T any] struct {
	C    <-chan T
	errC <-chan error

	curr T
	err  error
}

func New[T any](c <-chan T, errC <-chan error) *Stream[T] {
	return &Stream[T]{
		C:    c,
		errC: errC,
	}
}

// Next advances the stream to the next item.
// It returns false if there are no more items or an error occurred.
func (s *Stream[T]) Next() bool {
	for s.C != nil || s.errC != nil {
		select {
		case event, ok := <-s.C:
			if !ok {
				s.C = nil
				continue
			}
			s.curr = event
			return true
		case err, ok := <-s.errC:
			if !ok {
				s.errC = nil
				continue
			}
			if err != nil {
				s.err = err
				return false
			}
		}
	}
	return false
}

// Current returns the current item in the stream.
func (s *Stream[T]) Current() T {
	return s.curr
}

// Err returns the error encountered during streaming, if any.
func (s *Stream[T]) Err() error {
	return s.err
}
