// Package apperr defines the sentinel error used across aibomgen-cli.
// .
// Error taxonomy.
// .
//
//	ErrCancelled – the user deliberately aborted an interactive flow (confirmation.
//	               prompt, model-selector, …).
//	               Exit code: 0 (not a failure).
//
// .
// Everything else is a plain Go error (I/O, network, BOM parsing, …) and is.
// propagated with fmt.Errorf("context: %w", err) wrapping.
package apperr

import "errors"

// ErrCancelled is returned when the user explicitly aborts an interactive.
// operation.  The CLI should exit 0 rather than 1 when it sees this error.
var ErrCancelled = errors.New("operation cancelled")
