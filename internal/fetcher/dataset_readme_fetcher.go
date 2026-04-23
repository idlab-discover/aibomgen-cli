package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DatasetReadmeCard represents metadata extracted from a Hugging Face dataset README.
// Analog to ModelReadmeCard but for datasets.
type DatasetReadmeCard struct {
	Raw         string
	FrontMatter map[string]any
	Body        string

	// BOM-relevant front matter fields.
	License            string   // BOM.metadata.component.licenses
	Tags               []string // BOM.metadata.component.tags
	Language           []string // BOM.metadata.component.data.classification / tags
	AnnotationCreators []string // BOM.metadata.component.manufacturer, author, group

	// Configs with data_files (for attachments/contents).
	Configs []DatasetConfig // BOM.metadata.component.data.contents.attachment

	// BOM-relevant Markdown body fields.
	DatasetDescription    string // BOM.metadata.component.data.description
	CuratedBy             string // BOM.metadata.component.data.governance.stewards.organization.name / custodians
	FundedBy              string // BOM.metadata.component.data.governance.owners.organization.name
	SharedBy              string // BOM.metadata.component.data.governance.custodians.organization.name
	RepositoryURL         string // BOM.metadata.component.externalRef (url to huggingface)
	PaperURL              string // BOM.metadata.component.externalRef (url to paper)
	DemoURL               string // BOM.metadata.component.externalRef (url to demo)
	OutOfScopeUse         string // BOM.metadata.component.data.sensitive data
	PersonalSensitiveInfo string // BOM.metadata.component.data.sensitive data
	BiasRisksLimitations  string // BOM.metadata.component.data.sensitive data
	DatasetCardContact    string // BOM.metadata.component.properties (datasetcardcontact)
}

// DatasetConfig represents a configuration with data files splits.
type DatasetConfig struct {
	Name      string
	DataFiles []DatasetDataFile
}

// DatasetDataFile represents a single data file entry with split info.
type DatasetDataFile struct {
	Split string
	Path  string
}

// DatasetReadmeFetcher fetches the README.md (dataset card) for a dataset repo.
type DatasetReadmeFetcher struct {
	Client  *http.Client
	Token   string
	BaseURL string // optional; defaults to "https://huggingface.co"
}

func (f *DatasetReadmeFetcher) Fetch(datasetID string) (*DatasetReadmeCard, error) {
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}

	trimmedDatasetID := strings.TrimPrefix(strings.TrimSpace(datasetID), "/")
	if trimmedDatasetID == "" {
		return nil, fmt.Errorf("empty dataset id")
	}

	baseURL := strings.TrimRight(strings.TrimSpace(f.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://huggingface.co"
	}

	// Try main then master.
	candidates := []string{
		fmt.Sprintf("%s/datasets/%s/resolve/main/README.md", baseURL, trimmedDatasetID),
		fmt.Sprintf("%s/datasets/%s/resolve/master/README.md", baseURL, trimmedDatasetID),
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
		card := parseDatasetReadmeCard(raw)

		return card, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("unable to fetch README")
	}

	return nil, lastErr
}

func parseDatasetReadmeCard(raw string) *DatasetReadmeCard {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	card := &DatasetReadmeCard{Raw: raw}

	fm, body := splitFrontMatter(raw)
	card.FrontMatter = fm
	card.Body = body

	// BOM-relevant front matter fields.
	card.License = strings.TrimSpace(stringFromAny(fm["license"]))
	card.Tags = stringSliceFromAny(fm["tags"])
	card.Language = stringSliceFromAny(fm["language"])
	card.AnnotationCreators = stringSliceFromAny(fm["annotations_creators"])

	// Parse configs with data_files.
	if cfgs, ok := fm["configs"]; ok {
		card.Configs = parseDatasetConfigs(cfgs)
	}

	// BOM-relevant Markdown body fields.
	card.DatasetDescription = strings.TrimSpace(extractSection(body, "Dataset Description"))
	card.CuratedBy = strings.TrimSpace(extractBulletValue(body, "Curated by"))
	card.FundedBy = strings.TrimSpace(extractBulletValue(body, "Funded by"))
	card.SharedBy = strings.TrimSpace(extractBulletValue(body, "Shared by"))
	card.RepositoryURL = strings.TrimSpace(extractBulletValue(body, "Repository"))
	card.PaperURL = strings.TrimSpace(extractBulletValue(body, "Paper"))
	card.DemoURL = strings.TrimSpace(extractBulletValue(body, "Demo"))
	card.OutOfScopeUse = strings.TrimSpace(extractSection(body, "Out-of-Scope Use"))
	card.PersonalSensitiveInfo = strings.TrimSpace(extractSection(body, "Personal and Sensitive Information"))
	card.BiasRisksLimitations = strings.TrimSpace(extractSection(body, "Bias, Risks, and Limitations"))
	card.DatasetCardContact = strings.TrimSpace(extractSection(body, "Dataset Card Contact"))

	return card
}

// parseDatasetConfigs parses dataset configurations from the front matter.
func parseDatasetConfigs(cfgs any) []DatasetConfig {
	var result []DatasetConfig

	cfgList, ok := cfgs.([]any)
	if !ok {
		return result
	}

	for _, cfgItem := range cfgList {
		cfgMap, ok := cfgItem.(map[string]any)
		if !ok {
			continue
		}

		cfg := DatasetConfig{
			Name: strings.TrimSpace(stringFromAny(cfgMap["config_name"])),
		}

		// Parse data_files.
		if dfAny, ok := cfgMap["data_files"]; ok {
			dfList, ok := dfAny.([]any)
			if ok {
				for _, dfItem := range dfList {
					dfMap, ok := dfItem.(map[string]any)
					if !ok {
						continue
					}
					df := DatasetDataFile{
						Split: strings.TrimSpace(stringFromAny(dfMap["split"])),
						Path:  strings.TrimSpace(stringFromAny(dfMap["path"])),
					}
					if df.Split != "" || df.Path != "" {
						cfg.DataFiles = append(cfg.DataFiles, df)
					}
				}
			}
		}

		if cfg.Name != "" || len(cfg.DataFiles) > 0 {
			result = append(result, cfg)
		}
	}

	return result
}
