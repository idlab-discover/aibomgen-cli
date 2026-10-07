package fetcher

import (
	"fmt"
	"net/http"
	"strings"
)

// DatasetAPIResponse is the decoded response from GET https://huggingface.co/api/datasets/:id.
type DatasetAPIResponse struct {
	ID          string         `json:"id"`
	Author      string         `json:"author"`
	SHA         string         `json:"sha"`
	LastMod     string         `json:"lastModified"`
	CreatedAt   string         `json:"createdAt"`
	Private     bool           `json:"private"`
	Gated       BoolOrString   `json:"gated"`
	Disabled    bool           `json:"disabled"`
	Tags        []string       `json:"tags"`
	Description string         `json:"description"`
	Downloads   int            `json:"downloads"`
	Likes       int            `json:"likes"`
	UsedStorage int64          `json:"usedStorage"`
	CardData    map[string]any `json:"cardData"`
}

// DatasetAPIFetcher fetches dataset metadata from the Hugging Face Hub API.
type DatasetAPIFetcher struct {
	Client  *http.Client
	BaseURL string // optional; defaults to "https://huggingface.co"
}

// Fetch fetches dataset metadata for the given datasetID.
func (f *DatasetAPIFetcher) Fetch(datasetID string) (*DatasetAPIResponse, error) {
	trimmedDatasetID := strings.TrimPrefix(strings.TrimSpace(datasetID), "/")
	url := fmt.Sprintf("%s/api/datasets/%s", HFBaseURL(f.BaseURL), trimmedDatasetID)
	var parsed DatasetAPIResponse
	if err := getJSON(httpClient(f.Client), url, &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}
