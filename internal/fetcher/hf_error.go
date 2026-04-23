package fetcher

import (
	"errors"
	"fmt"
	"net/http"
)

// HFError is returned when the Hugging Face Hub responds with a non-2xx HTTP status.
// Using a typed error allows callers to distinguish "not found" (404) from transient.
// failures without string matching.
type HFError struct {
	StatusCode int
}

func (e *HFError) Error() string {
	return fmt.Sprintf("huggingface api status %d", e.StatusCode)
}

// IsNotFound reports whether err is an HFError with HTTP 404.
func IsNotFound(err error) bool {
	var e *HFError
	return errors.As(err, &e) && e.StatusCode == http.StatusNotFound
}

// IsUnauthorized reports whether err is an HFError with HTTP 401 or 403.
// This typically means the repo is private and no (or an invalid) token was provided.
func IsUnauthorized(err error) bool {
	var e *HFError
	return errors.As(err, &e) && (e.StatusCode == http.StatusUnauthorized || e.StatusCode == http.StatusForbidden)
}
