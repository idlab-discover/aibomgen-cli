package fetcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	neturl "net/url"
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
	// BaseModels is the Hub's lineage for models whose card declares a base_model.
	// It comes from a separate ?expand[]=baseModels request (see FetchRevision).
	BaseModels *ModelBaseModels `json:"baseModels,omitempty"`
}

// ModelBaseModels is the Hub's view of a model's lineage: its base models and the
// relation to them (finetune, adapter, quantized or merge), inferred by the Hub when
// the card doesn't set base_model_relation.
type ModelBaseModels struct {
	Relation string `json:"relation"`
	Models   []struct {
		ID string `json:"id"`
	} `json:"models"`
}

func (f *ModelAPIFetcher) Fetch(modelID string) (*ModelAPIResponse, error) {
	return f.FetchRevision(modelID, "")
}

// FetchRevision fetches model metadata at a revision (branch, tag or commit).
// An empty revision uses the default branch (GET /api/models/:id); otherwise.
// GET /api/models/:id/revision/:revision, whose sha is the resolved commit.
func (f *ModelAPIFetcher) FetchRevision(modelID, revision string) (*ModelAPIResponse, error) {
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
	if rev := strings.TrimSpace(revision); rev != "" {
		url += "/revision/" + neturl.PathEscape(rev)
	}
	var parsed ModelAPIResponse
	if err := getJSON(client, url, &parsed); err != nil {
		return nil, err
	}

	// expand[] limits the response to the expanded fields, so lineage needs its own
	// request. Only make it for models that declare a base model; it is best-effort.
	if hasBaseModel(parsed.CardData) {
		var lineage struct {
			BaseModels *ModelBaseModels `json:"baseModels"`
		}
		if err := getJSON(client, url+"?expand[]=baseModels", &lineage); err == nil {
			parsed.BaseModels = lineage.BaseModels
		}
	}
	return &parsed, nil
}

// hasBaseModel reports whether the card data declares a base_model.
func hasBaseModel(cardData map[string]any) bool {
	return len(stringSliceFromAny(cardData["base_model"])) > 0
}

// getJSON GETs url and decodes the JSON body into out.
func getJSON(client *http.Client, url string, out any) error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &HFError{StatusCode: resp.StatusCode}
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
