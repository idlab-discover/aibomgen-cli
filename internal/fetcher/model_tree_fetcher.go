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

// SecurityFileEntry represents one file entry from the HF tree API when expand=true.
type SecurityFileEntry struct {
	Type               string              `json:"type"`
	OID                string              `json:"oid"`
	Size               int64               `json:"size"`
	Path               string              `json:"path"`
	LastCommit         SecurityCommit      `json:"lastCommit"`
	SecurityFileStatus *SecurityFileStatus `json:"securityFileStatus"`
}

// SecurityCommit is the last-commit metadata returned per file in the expanded tree.
type SecurityCommit struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Date  string `json:"date"`
}

// SecurityFileStatus holds per-file, per-scanner security results.
type SecurityFileStatus struct {
	// Status is the aggregated file status: "safe", "caution", or "unsafe".
	Status           string           `json:"status"`
	ProtectAiScan    ScannerResult    `json:"protectAiScan"`
	AvScan           ScannerResult    `json:"avScan"`
	PickleImportScan PickleScanResult `json:"pickleImportScan"`
	VirusTotalScan   ScannerResult    `json:"virusTotalScan"`
	JFrogScan        ScannerResult    `json:"jFrogScan"`
}

// ScannerResult is the result from a single security scanner.
type ScannerResult struct {
	// Status is one of: "safe", "caution", "unsafe", "suspicious", "unscanned".
	Status      string `json:"status"`
	Message     string `json:"message"`
	ReportLink  string `json:"reportLink"`
	ReportLabel string `json:"reportLabel"`
}

// PickleScanResult extends ScannerResult with pickle-specific import analysis.
type PickleScanResult struct {
	Status        string         `json:"status"`
	PickleImports []PickleImport `json:"pickleImports"`
	Message       string         `json:"message"`
	Version       string         `json:"version"`
}

// PickleImport describes a single Python import found inside a pickle file.
type PickleImport struct {
	Module string `json:"module"`
	Name   string `json:"name"`
	// Safety is one of: "safe", "suspicious", "dangerous".
	Safety string `json:"safety"`
}

// ModelTreeFetcher fetches the file tree with security metadata from the HF Hub API.
type ModelTreeFetcher struct {
	Client  *http.Client
	BaseURL string // optional; defaults to "https://huggingface.co"
}

// maxTreePages caps the number of paginated requests to avoid unbounded fetching.
const maxTreePages = 10

// Fetch returns all file entries for the given modelID from the HF tree API.
// (branch: main, expand=true, recursive=true). It follows cursor-based.
// pagination up to maxTreePages pages (1 000 files maximum).
func (f *ModelTreeFetcher) Fetch(modelID string) ([]SecurityFileEntry, error) {
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
		// Construct paginated URL: /api/models/{modelID}/tree/main.
		apiURL := fmt.Sprintf("%s/api/models/%s/tree/main", base, modelID)
		u, err := url.Parse(apiURL)
		if err != nil {
			return nil, fmt.Errorf("parse tree url: %w", err)
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
			return nil, fmt.Errorf("build tree request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch tree: %w", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read tree body: %w", readErr)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, &HFError{StatusCode: resp.StatusCode}
		}

		var entries []SecurityFileEntry
		if err := json.Unmarshal(body, &entries); err != nil {
			return nil, fmt.Errorf("decode tree response: %w", err)
		}
		all = append(all, entries...)

		// Follow pagination via the Link header (<url>; rel="next").
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

// parseLinkNext extracts the URL from a Link header's rel="next" entry.
// Example: `<https://huggingface.co/...?cursor=abc>; rel="next"`.
func parseLinkNext(header string) string {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		segments := strings.Split(part, ";")
		if len(segments) < 2 {
			continue
		}
		rel := strings.TrimSpace(segments[1])
		if strings.EqualFold(rel, `rel="next"`) {
			u := strings.TrimSpace(segments[0])
			return strings.Trim(u, "<>")
		}
	}
	return ""
}
