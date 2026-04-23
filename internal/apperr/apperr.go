// Package apperr defines the two sentinel error categories used across aibomgen-cli.
//.
// Error taxonomy.
//.
//	UserError  – caused by missing or invalid user input (wrong flag, bad value, …).
//	             The CLI prints only the message; usage help is NOT repeated.
//	             Exit code: 1.
//.
//	ErrCancelled – the user deliberately aborted an interactive flow (confirmation.
//	               prompt, model-selector, …).
//	               Exit code: 0 (not a failure).
//.
// Everything else is a plain Go error (I/O, network, BOM parsing, …) and is.
// propagated with fmt.Errorf("context: %w", err) wrapping.
package apperr

import (
	"errors"
	"fmt"
)

// ErrCancelled is returned when the user explicitly aborts an interactive.
// operation.  The CLI should exit 0 rather than 1 when it sees this error.
var ErrCancelled = errors.New("operation cancelled")

// UserError represents an error caused by invalid or missing user input.
// Cobra command handlers return this instead of a bare fmt.Errorf so that.
// the root command can suppress repeated usage output and format the message.
// in a user-friendly way.
type UserError struct {
	Message string
}

func (e *UserError) Error() string { return e.Message }

// User creates a UserError with the given message.
func User(msg string) error { return &UserError{Message: msg} }

// Userf creates a formatted UserError.
func Userf(format string, args ...any) error {
	return &UserError{Message: fmt.Sprintf(format, args...)}
}

// IsUser reports whether err is (or wraps) a *UserError.
func IsUser(err error) bool {
	var u *UserError
	return errors.As(err, &u)
}
