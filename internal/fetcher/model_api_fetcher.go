package fetcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// BoolOrString unmarshals JSON that may be either a boolean (true/false).
// or a string (e.g. "auto").
type BoolOrString struct {
	Bool   *bool
	String *string
}

func (v *BoolOrString) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		v.Bool = nil
		v.String = nil
		return nil
	}

	// string case: "auto".
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		s = strings.TrimSpace(s)
		v.String = &s
		v.Bool = nil
		return nil
	}

	// bool case: true/false.
	var bo bool
	if err := json.Unmarshal(b, &bo); err != nil {
		return err
	}
	v.Bool = &bo
	v.String = nil
	return nil
}

// ModelAPIFetcher fetches model metadata from the Hugging Face Hub API.
type ModelAPIFetcher struct {
	Client  *http.Client
	BaseURL string // optional; defaults to "https://huggingface.co"
}

// ModelAPIResponse is the decoded response from GET https://huggingface.co/api/models/:id.
type ModelAPIResponse struct {
	ID          string         `json:"id"`
	ModelID     string         `json:"modelId"`
	Author      string         `json:"author"`
	PipelineTag string         `json:"pipeline_tag"`
	LibraryName string         `json:"library_name"`
	Tags        []string       `json:"tags"`
	License     string         `json:"license"`
	SHA         string         `json:"sha"`
	Downloads   int            `json:"downloads"`
	Likes       int            `json:"likes"`
	LastMod     string         `json:"lastModified"`
	CreatedAt   string         `json:"createdAt"`
	Gated       BoolOrString   `json:"gated"` // <- changed from bool
	Private     bool           `json:"private"`
	Inference   string         `json:"inference"`
	UsedStorage int64          `json:"usedStorage"`
	CardData    map[string]any `json:"cardData"`
	Config      struct {
		ModelType     string   `json:"model_type"`
		Architectures []string `json:"architectures"`
	} `json:"config"`
}

func (f *ModelAPIFetcher) Fetch(modelID string) (*ModelAPIResponse, error) {
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}

	trimmedModelID := strings.TrimPrefix(strings.TrimSpace(modelID), "/")

	baseURL := strings.TrimRight(strings.TrimSpace(f.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://huggingface.co"
	}

	url := fmt.Sprintf("%s/api/models/%s", baseURL, trimmedModelID)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
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
		return nil, &HFError{StatusCode: resp.StatusCode}
	}

	var parsed ModelAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}
