package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ModelReadmeCard represents metadata extracted from a Hugging Face model README.
// .
// Hugging Face model cards usually contain a YAML front matter block (--- ... ---).
// followed by Markdown sections. We parse both:.
// - YAML front matter for structured fields (license, tags, datasets, metrics, base_model, model-index).
// - Markdown sections/bullets using regex (e.g. Direct Use, Bias/Risks, Paper/Demo links).
type ModelReadmeCard struct {
	Raw         string
	FrontMatter map[string]any
	Body        string

	// Common front matter fields.
	License   string
	Tags      []string
	Datasets  []string
	Metrics   []string
	BaseModel string

	// Extracted from Markdown body (template-based).
	DevelopedBy          string
	PaperURL             string
	DemoURL              string
	DirectUse            string
	OutOfScopeUse        string
	BiasRisksLimitations string
	BiasRecommendations  string
	ModelCardContact     string

	// Environmental Impact (from Markdown body).
	EnvironmentalHardwareType  string
	EnvironmentalHoursUsed     string
	EnvironmentalCloudProvider string
	EnvironmentalComputeRegion string
	EnvironmentalCarbonEmitted string

	// From model-index (if present).
	TaskType string
	TaskName string
	// Metrics with optional values (best-effort).
	ModelIndexMetrics []ModelIndexMetric

	// Quantitative Analysis sections (from Markdown body).
	TestingMetrics string
	Results        string
}

type ModelIndexMetric struct {
	Type  string
	Value string
}

// ModelReadmeFetcher fetches the README.md (model card) for a model repo.
// .
// It uses URLs like:.
// .
//
//	GET https://huggingface.co/{modelID}/resolve/main/README.md.
//
// .
// and falls back to /resolve/master/README.md.
type ModelReadmeFetcher struct {
	Client  *http.Client
	Token   string
	BaseURL string // optional; defaults to "https://huggingface.co"
}

func (f *ModelReadmeFetcher) Fetch(modelID string) (*ModelReadmeCard, error) {
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}

	trimmedModelID := strings.TrimPrefix(strings.TrimSpace(modelID), "/")
	if trimmedModelID == "" {
		return nil, fmt.Errorf("empty model id")
	}

	baseURL := strings.TrimRight(strings.TrimSpace(f.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://huggingface.co"
	}

	// Try main then master.
	candidates := []string{
		fmt.Sprintf("%s/%s/resolve/main/README.md", baseURL, trimmedModelID),
		fmt.Sprintf("%s/%s/resolve/master/README.md", baseURL, trimmedModelID),
	}

	var lastErr error
	for _, url := range candidates {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
		if err != nil {
			return nil, err
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

		raw := string(bodyBytes)
		card := parseReadmeCard(raw)

		return card, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("unable to fetch README")
	}

	return nil, lastErr
}

func parseReadmeCard(raw string) *ModelReadmeCard {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	card := &ModelReadmeCard{Raw: raw}

	fm, body := splitFrontMatter(raw)
	card.FrontMatter = fm
	card.Body = body

	// Front matter fields (best effort).
	card.License = strings.TrimSpace(stringFromAny(fm["license"]))
	card.Tags = stringSliceFromAny(fm["tags"])
	card.Datasets = stringSliceFromAny(fm["datasets"])
	card.Metrics = stringSliceFromAny(fm["metrics"])
	card.BaseModel = strings.TrimSpace(stringFromAny(fm["base_model"]))

	// model-index task + metrics (best effort).
	if mi, ok := fm["model-index"]; ok {
		parseModelIndex(mi, card)
	}

	// Markdown extraction (template-based).
	card.DevelopedBy = strings.TrimSpace(extractBulletValue(body, "Developed by"))
	card.PaperURL = strings.TrimSpace(extractBulletValue(body, "Paper"))
	card.DemoURL = strings.TrimSpace(extractBulletValue(body, "Demo"))
	card.DirectUse = strings.TrimSpace(extractSection(body, "Direct Use"))
	card.OutOfScopeUse = strings.TrimSpace(extractSection(body, "Out-of-Scope Use"))
	card.BiasRisksLimitations = strings.TrimSpace(extractSection(body, "Bias, Risks, and Limitations"))
	card.BiasRecommendations = strings.TrimSpace(extractSection(body, "Recommendations"))
	card.ModelCardContact = strings.TrimSpace(extractSection(body, "Model Card Contact"))

	// Quantitative Analysis sections.
	card.TestingMetrics = strings.TrimSpace(extractSection(body, "Metrics"))
	card.Results = strings.TrimSpace(extractSection(body, "Results"))

	// Environmental Impact.
	card.EnvironmentalHardwareType = strings.TrimSpace(extractBulletValue(body, "Hardware Type"))
	card.EnvironmentalHoursUsed = strings.TrimSpace(extractBulletValue(body, "Hours used"))
	card.EnvironmentalCloudProvider = strings.TrimSpace(extractBulletValue(body, "Cloud Provider"))
	card.EnvironmentalComputeRegion = strings.TrimSpace(extractBulletValue(body, "Compute Region"))
	card.EnvironmentalCarbonEmitted = strings.TrimSpace(extractBulletValue(body, "Carbon Emitted"))

	// Note: We keep placeholders in the card structure. (for templates/model-card-example).
	// The fieldspecs layer can decide whether to use them or filter them out.

	return card
}
