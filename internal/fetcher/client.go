package fetcher

import (
	"cmp"
	"context"
	"fmt"
	"io"
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

// HFBaseURL trims s and its trailing slashes, defaulting to "https://huggingface.co".
func HFBaseURL(s string) string {
	return cmp.Or(strings.TrimRight(strings.TrimSpace(s), "/"), "https://huggingface.co")
}

// httpClient returns c, or http.DefaultClient when c is nil.
func httpClient(c *http.Client) *http.Client {
	if c == nil {
		return http.DefaultClient
	}
	return c
}

// fetchFirstOK GETs each URL in turn and returns the body of the first 200 response,
// or the last error.
func fetchFirstOK(client *http.Client, urls []string) (string, error) {
	var lastErr error
	for _, url := range urls {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Accept", "text/markdown, text/plain, */*")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		bodyBytes, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = &HFError{StatusCode: resp.StatusCode}
			continue
		}
		return string(bodyBytes), nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("unable to fetch README")
	}
	return "", lastErr
}
