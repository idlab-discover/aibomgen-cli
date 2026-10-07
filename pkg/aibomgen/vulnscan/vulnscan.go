package vulnscan

import (
	"fmt"
	"strings"
	"time"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/internal/builder"
	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
)

// ComponentScanResult holds the security tree entries and any derived vulnerabilities.
// for a single BOM component (model or dataset).
type ComponentScanResult struct {
	// ComponentRef is the BOM-ref of the affected component.
	ComponentRef string
	// ModelID is the HuggingFace model/dataset ID used to call the tree API.
	ModelID string
	// Entries is the raw per-file security data returned by the HF tree API.
	Entries []fetcher.SecurityFileEntry
	// Vulnerabilities are the CycloneDX vulnerability objects derived from Entries.
	Vulnerabilities []cdx.Vulnerability
	// Err is non-nil when the tree API call failed (non-fatal; other components continue).
	Err error
}

// Options configures a vulnerability scan run.
type Options struct {
	HFToken string
	Timeout time.Duration
	// BaseURL overrides the HuggingFace base URL (empty = default).
	BaseURL string
}

// ScanBOM fetches security scan results for every ML-model and dataset component.
// found in the given BOM and returns one ComponentScanResult per component.
// Errors per component are recorded in ComponentScanResult.Err and do not abort.
// the overall scan.
func ScanBOM(bom *cdx.BOM, opts Options) []ComponentScanResult {
	if opts.Timeout <= 0 {
		opts.Timeout = 15 * time.Second
	}

	httpClient := fetcher.NewHFClient(opts.Timeout, opts.HFToken)

	modelFetcher := &fetcher.ModelTreeFetcher{
		Client:  httpClient,
		BaseURL: opts.BaseURL,
	}
	datasetFetcher := &fetcher.DatasetTreeFetcher{
		Client:  httpClient,
		BaseURL: opts.BaseURL,
	}

	return scanComponents(bom, modelFetcher, datasetFetcher)
}

// treeFetcherIface allows injection of a test double.
type treeFetcherIface interface {
	Fetch(id string) ([]fetcher.SecurityFileEntry, error)
}

func scanComponents(bom *cdx.BOM, modelTF treeFetcherIface, datasetTF treeFetcherIface) []ComponentScanResult {
	var results []ComponentScanResult

	// Scan the primary metadata component (always a model).
	if bom.Metadata != nil && bom.Metadata.Component != nil {
		c := bom.Metadata.Component
		if id := hfIDFromComponent(c); id != "" {
			results = append(results, scanOne(c, id, modelTF))
		}
	}

	// Scan dataset and model components in BOM.components.
	if bom.Components != nil {
		for i := range *bom.Components {
			c := &(*bom.Components)[i]
			switch c.Type {
			case cdx.ComponentTypeData:
				if id := datasetIDFromComponent(c); id != "" {
					results = append(results, scanOne(c, id, datasetTF))
				}
			case cdx.ComponentTypeMachineLearningModel:
				if id := hfIDFromComponent(c); id != "" {
					results = append(results, scanOne(c, id, modelTF))
				}
			}
		}
	}

	return results
}

func scanOne(comp *cdx.Component, modelID string, tf treeFetcherIface) ComponentScanResult {
	res := ComponentScanResult{
		ComponentRef: comp.BOMRef,
		ModelID:      modelID,
	}

	entries, err := tf.Fetch(modelID)
	if err != nil {
		res.Err = fmt.Errorf("security scan for %q failed: %w", modelID, err)
		return res
	}

	res.Entries = entries

	// Derive vulnerabilities (reuse the same builder logic used during generate/scan).
	// We need a temporary BOM to collect them.
	tmpBOM := cdx.NewBOM()
	builder.InjectSecurityData(tmpBOM, comp, entries, modelID, "")
	if tmpBOM.Vulnerabilities != nil {
		res.Vulnerabilities = *tmpBOM.Vulnerabilities
	}

	return res
}

// hfIDFromComponent extracts the HuggingFace model ID (owner/repo) from a component.
// It checks the PURL, then falls back to the component name.
func hfIDFromComponent(c *cdx.Component) string {
	if c.PackageURL != "" {
		if id := idFromPURL(c.PackageURL); id != "" {
			return id
		}
	}
	return strings.TrimSpace(c.Name)
}

// datasetIDFromComponent extracts the HuggingFace dataset ID from a data component.
// Dataset PURLs are pkg:huggingface/datasets/{owner/name}@sha, so the namespace.
// segment "datasets" must be stripped to get the actual API identifier.
func datasetIDFromComponent(c *cdx.Component) string {
	if c.PackageURL != "" {
		if id := datasetIDFromPURL(c.PackageURL); id != "" {
			return id
		}
	}
	// Fall back: strip a leading "datasets/" prefix if present, otherwise use as-is.
	name := strings.TrimSpace(c.Name)
	if after, ok := strings.CutPrefix(name, "datasets/"); ok {
		return after
	}
	return name
}

// idFromPURL extracts "namespace/name" from a pkg:huggingface/... model PURL.
func idFromPURL(purl string) string {
	rest, ok := strings.CutPrefix(purl, "pkg:huggingface/")
	if !ok {
		return ""
	}
	rest, _, _ = strings.Cut(rest, "@")
	if !strings.Contains(rest, "/") {
		return ""
	}
	return rest
}

// datasetIDFromPURL extracts the dataset owner/name from a pkg:huggingface/datasets/... PURL.
// e.g. "pkg:huggingface/datasets/bookcorpus@sha" → "bookcorpus".
// .
//
//	"pkg:huggingface/datasets/allenai/c4@sha"  → "allenai/c4".
func datasetIDFromPURL(purl string) string {
	rest, ok := strings.CutPrefix(purl, "pkg:huggingface/datasets/")
	if !ok {
		return ""
	}
	rest, _, _ = strings.Cut(rest, "@")
	return rest
}

// ApplyToDOM merges the vulnerability scan results into the BOM in-place.
// For every successfully scanned component, its previous Hugging Face findings
// ("hfsec-<component-ref>-..." bom-refs) are replaced by the fresh ones, so
// findings that were resolved upstream disappear. Other vulnerabilities (e.g.
// CVEs merged from an SBOM) and findings of components whose scan failed are kept.
// It returns the number of previous findings that were removed.
func ApplyToDOM(bom *cdx.BOM, results []ComponentScanResult) int {
	var prefixes []string
	var fresh []cdx.Vulnerability
	for _, r := range results {
		if r.Err != nil {
			continue
		}
		prefixes = append(prefixes, "hfsec-"+r.ComponentRef+"-")
		fresh = append(fresh, r.Vulnerabilities...)
	}

	var kept []cdx.Vulnerability
	removed := 0
	if bom.Vulnerabilities != nil {
		for _, v := range *bom.Vulnerabilities {
			if hasAnyPrefix(v.BOMRef, prefixes) {
				removed++
				continue
			}
			kept = append(kept, v)
		}
	}

	kept = append(kept, fresh...)
	if len(kept) == 0 {
		bom.Vulnerabilities = nil
	} else {
		bom.Vulnerabilities = &kept
	}
	return removed
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// severityRank orders the rated CycloneDX severities; unknown or unset is absent.
var severityRank = map[cdx.Severity]int{
	cdx.SeverityCritical: 5,
	cdx.SeverityHigh:     4,
	cdx.SeverityMedium:   3,
	cdx.SeverityLow:      2,
	cdx.SeverityInfo:     1,
	cdx.SeverityNone:     0,
}

// HighestSeverity returns the highest severity among a vulnerability's ratings,
// or cdx.SeverityUnknown when it has no rated severity.
func HighestSeverity(v cdx.Vulnerability) cdx.Severity {
	best, bestRank := cdx.SeverityUnknown, -1
	if v.Ratings != nil {
		for _, r := range *v.Ratings {
			if rank, ok := severityRank[r.Severity]; ok && rank > bestRank {
				best, bestRank = r.Severity, rank
			}
		}
	}
	return best
}

// SeverityAtLeast reports whether sev is rated at or above threshold.
// Unknown or unset severities never meet a threshold.
func SeverityAtLeast(sev, threshold cdx.Severity) bool {
	rank, ok := severityRank[sev]
	return ok && rank >= severityRank[threshold]
}
