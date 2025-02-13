package llmagent

import "fmt"

type AgentError struct {
	Kind    ErrorKind
	Message string
	Err     error
}

func (e *AgentError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *AgentError) Unwrap() error {
	return e.Err
}

type ErrorKind string

const (
	LanguageModelErrorKind         ErrorKind = "language_model_error"
	InvariantErrorKind             ErrorKind = "invariant_error"
	ToolExecutionErrorKind         ErrorKind = "tool_execution_error"
	AgentErrorKindMaxTurnsExceeded ErrorKind = "max_turns_exceeded"
	InitErrorKind                  ErrorKind = "init_error"
)

func NewLanguageModelError(err error) *AgentError {
	return &AgentError{
		Kind:    LanguageModelErrorKind,
		Message: fmt.Sprintf("language model error: %v", err),
		Err:     err,
	}
}

func NewInvariantError(msg string) *AgentError {
	return &AgentError{
		Kind:    InvariantErrorKind,
		Message: fmt.Sprintf("invariant: %s", msg),
	}
}

func NewToolExecutionError(err error) *AgentError {
	return &AgentError{
		Kind:    ToolExecutionErrorKind,
		Message: fmt.Sprintf("tool execution error: %v", err),
		Err:     err,
	}
}

func NewMaxTurnsExceededError(turns int) *AgentError {
	return &AgentError{
		Kind:    AgentErrorKindMaxTurnsExceeded,
		Message: fmt.Sprintf("the maximum number of turns (%d) has been exceeded.", turns),
	}
}

func NewInitError(err error) *AgentError {
	return &AgentError{
		Kind:    InitErrorKind,
		Message: fmt.Sprintf("run initialization error: %v", err),
		Err:     err,
	}
}
