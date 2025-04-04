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
	select {
	case event, ok := <-s.C:
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
		s.curr = event
		return true
	case err := <-s.errC:
		s.err = err
		return false
	}
}

// Current returns the current item in the stream.
func (s *Stream[T]) Current() T {
	return s.curr
}

// Err returns the error encountered during streaming, if any.
func (s *Stream[T]) Err() error {
	return s.err
}
