package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// DatasetTreeFetcher fetches the file tree with security metadata from the HF Hub.
// datasets API (/api/datasets/{id}/tree/main).
type DatasetTreeFetcher struct {
	Client  *http.Client
	BaseURL string // optional; defaults to "https://huggingface.co"
}

// Fetch returns all file entries for the given datasetID from the HF datasets tree.
// API (branch: main, expand=true, recursive=true). It follows cursor-based.
// pagination up to maxTreePages pages.
func (f *DatasetTreeFetcher) Fetch(datasetID string) ([]SecurityFileEntry, error) {
	base := strings.TrimRight(f.BaseURL, "/")
	if base == "" {
		base = "https://huggingface.co"
	}
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}

	var all []SecurityFileEntry
	cursor := ""

	for page := 0; page < maxTreePages; page++ {
		apiURL := fmt.Sprintf("%s/api/datasets/%s/tree/main", base, datasetID)
		u, err := url.Parse(apiURL)
		if err != nil {
			return nil, fmt.Errorf("parse dataset tree url: %w", err)
		}
		q := u.Query()
		q.Set("expand", "true")
		q.Set("recursive", "true")
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("build dataset tree request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch dataset tree: %w", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read dataset tree body: %w", readErr)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, &HFError{StatusCode: resp.StatusCode}
		}

		var entries []SecurityFileEntry
		if err := json.Unmarshal(body, &entries); err != nil {
			return nil, fmt.Errorf("decode dataset tree response: %w", err)
		}
		all = append(all, entries...)

		next := parseLinkNext(resp.Header.Get("Link"))
		if next == "" {
			break
		}
		nextURL, err := url.Parse(next)
		if err != nil {
			break
		}
		cursor = nextURL.Query().Get("cursor")
		if cursor == "" {
			break
		}
	}

	return all, nil
}
