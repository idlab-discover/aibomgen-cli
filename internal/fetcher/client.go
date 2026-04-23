package fetcher

import (
	"net/http"
	"strings"
	"time"
)

// hfTransport injects a Bearer token into every request when a token is set.
type hfTransport struct {
	base  http.RoundTripper
	token string
}

func (t *hfTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.token != "" {
		req = req.Clone(req.Context())
		req.Header.Set("Authorization", "Bearer "+t.token)
	}
	return t.base.RoundTrip(req)
}

// NewHFClient creates an *http.Client configured for Hugging Face API calls.
// timeout is the per-request deadline (0 = no timeout).
// token is automatically injected as a Bearer token on every request when non-empty.
func NewHFClient(timeout time.Duration, token string) *http.Client {
	token = strings.TrimSpace(token)
	base := http.DefaultTransport
	transport := base
	if token != "" {
		transport = &hfTransport{base: base, token: token}
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}
