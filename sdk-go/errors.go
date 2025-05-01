package llmsdk

import (
	"fmt"
)

// Kind is a classification of error type.
type Kind string

const (
	InvalidInput   Kind = "invalid_input"
	Transport      Kind = "transport"
	StatusCode     Kind = "status_code"
	Unsupported    Kind = "unsupported"
	NotImplemented Kind = "not_implemented"
	Invariant      Kind = "invariant"
	Refusal        Kind = "refusal"
)

// LanguageModelError represents errors from the language model layer.
type LanguageModelError struct {
	Kind    Kind
	Message string
	Err     error
	// The provider name
	Provider string
	// The status for the StatusCode error kind
	Status int
}

func (e *LanguageModelError) Error() string {
	switch e.Kind {
	case InvalidInput:
		return fmt.Sprintf("invalid input: %s", e.Message)
	case Transport:
		return fmt.Sprintf("transport error: %s", e.Err)
	case StatusCode:
		return fmt.Sprintf("status error: %s (status %d)", e.Message, e.Status)
	case Unsupported:
		return fmt.Sprintf("unsupported by %s: %s", e.Provider, e.Message)
	case NotImplemented:
		return fmt.Sprintf("not implemented for %s: %s", e.Provider, e.Message)
	case Invariant:
		return fmt.Sprintf("invariant from %s: %s", e.Provider, e.Message)
	case Refusal:
		return fmt.Sprintf("refusal: %s", e.Message)
	default:
		return e.Message
	}
}

// Unwrap allows errors.Is / errors.As to work with wrapped errors.
func (e *LanguageModelError) Unwrap() error {
	return e.Err
}

// Helper constructors
func NewInvalidInputError(msg string) *LanguageModelError {
	return &LanguageModelError{Kind: InvalidInput, Message: msg}
}

func NewTransportError(err error) *LanguageModelError {
	return &LanguageModelError{Kind: Transport, Err: err}
}

func NewStatusCodeError(status int, body string) *LanguageModelError {
	return &LanguageModelError{Kind: StatusCode, Message: body, Status: status}
}

func NewUnsupportedError(provider string, msg string) *LanguageModelError {
	return &LanguageModelError{Kind: Unsupported, Message: msg, Provider: provider}
}

func NewNotImplementedError(provider string, msg string) *LanguageModelError {
	return &LanguageModelError{Kind: NotImplemented, Message: msg, Provider: provider}
}

func NewInvariantError(provider string, msg string) *LanguageModelError {
	return &LanguageModelError{Kind: Invariant, Message: msg, Provider: provider}
}

func NewRefusalError(msg string) *LanguageModelError {
	return &LanguageModelError{Kind: Refusal, Message: msg}
}
