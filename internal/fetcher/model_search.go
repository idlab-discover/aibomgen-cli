package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ModelSearchResult represents a single model in search results.
type ModelSearchResult struct {
	ID          string   `json:"id"`
	ModelID     string   `json:"modelId"`
	Author      string   `json:"author"`
	PipelineTag string   `json:"pipeline_tag"`
	Tags        []string `json:"tags"`
	Downloads   int      `json:"downloads"`
	Likes       int      `json:"likes"`
	Private     bool     `json:"private"`
	Gated       bool     `json:"gated"`
}

// ModelSearcher searches for models on Hugging Face.
type ModelSearcher struct {
	Client  *http.Client
	BaseURL string // optional; defaults to "https://huggingface.co"
}

// Search queries Hugging Face for models matching the search term.
func (s *ModelSearcher) Search(query string, limit int) ([]ModelSearchResult, error) {
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}

	baseURL := strings.TrimRight(strings.TrimSpace(s.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://huggingface.co"
	}

	if limit <= 0 {
		limit = 20
	}

	// Build the search URL with parameters.
	searchURL := fmt.Sprintf("%s/api/models", baseURL)
	params := url.Values{}

	if query != "" {
		params.Add("search", query)
	}
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("sort", "downloads") // Sort by downloads by default

	if len(params) > 0 {
		searchURL = searchURL + "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var results []ModelSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	return results, nil
}
