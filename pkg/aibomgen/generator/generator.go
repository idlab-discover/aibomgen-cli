package generator

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/idlab-discover/aibomgen-cli/internal/builder"
	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/scanner"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

// DiscoveredBOM pairs a scanner discovery with the CycloneDX BOM generated.
// from it.
type DiscoveredBOM struct {
	Discovery scanner.Discovery
	BOM       *cdx.BOM
}

type bomBuilder interface {
	Build(builder.BuildContext) (*cdx.BOM, error)
	BuildDataset(builder.DatasetBuildContext) (*cdx.Component, error)
}

var newBOMBuilder = func() bomBuilder {
	return builder.BOMBuilder{}
}

// Fetcher factory functions for testing.
type fetcherSet struct {
	modelAPI interface {
		FetchRevision(id, revision string) (*fetcher.ModelAPIResponse, error)
	}
	modelReadme interface {
		FetchRevision(id, revision string) (*fetcher.ModelReadmeCard, error)
	}
	datasetAPI interface {
		Fetch(string) (*fetcher.DatasetAPIResponse, error)
	}
	datasetReadme interface {
		Fetch(string) (*fetcher.DatasetReadmeCard, error)
	}
	modelTree interface {
		FetchRevision(id, revision string) ([]fetcher.SecurityFileEntry, error)
	}
}

// ModelRef is a Hugging Face model ID with an optional revision (branch, tag or commit).
type ModelRef struct {
	ID       string
	Revision string
}

// String returns "id" or "id@revision".
func (r ModelRef) String() string {
	if r.Revision == "" {
		return r.ID
	}
	return r.ID + "@" + r.Revision
}

// ParseModelRef parses "org/name" or "org/name@revision". '@' cannot occur in HF repo IDs,.
// so the first '@' separates the revision.
func ParseModelRef(s string) (ModelRef, error) {
	s = strings.TrimSpace(s)
	id, rev, hasRev := strings.Cut(s, "@")
	id, rev = strings.TrimSpace(id), strings.TrimSpace(rev)
	if id == "" {
		return ModelRef{}, fmt.Errorf("invalid model reference %q: empty model ID", s)
	}
	if hasRev && rev == "" {
		return ModelRef{}, fmt.Errorf("invalid model reference %q: empty revision after '@'", s)
	}
	return ModelRef{ID: id, Revision: rev}, nil
}

var newFetcherSet = func(httpClient *http.Client) fetcherSet {
	return fetcherSet{
		modelAPI:      &fetcher.ModelAPIFetcher{Client: httpClient},
		modelReadme:   &fetcher.ModelReadmeFetcher{Client: httpClient},
		datasetAPI:    &fetcher.DatasetAPIFetcher{Client: httpClient},
		datasetReadme: &fetcher.DatasetReadmeFetcher{Client: httpClient},
		modelTree:     &fetcher.ModelTreeFetcher{Client: httpClient},
	}
}

func newHTTPClient(opts GenerateOptions) *http.Client {
	return fetcher.NewHFClient(opts.Timeout, opts.HFToken)
}

// Dummy fetcher factory for BuildDummyBOM testing.
var newDummyFetcherSet = func() fetcherSet {
	return fetcherSet{
		modelAPI:      &fetcher.DummyModelAPIFetcher{},
		modelReadme:   &fetcher.DummyModelReadmeFetcher{},
		datasetAPI:    &fetcher.DummyDatasetAPIFetcher{},
		datasetReadme: &fetcher.DummyDatasetReadmeFetcher{},
		modelTree:     &fetcher.DummyModelTreeFetcher{},
	}
}

// ProgressCallback is called during generation to report progress.
type ProgressCallback func(event ProgressEvent)

// ProgressEvent represents a progress update.
type ProgressEvent struct {
	Type     ProgressEventType
	ModelID  string
	Message  string
	Index    int
	Total    int
	Datasets int
	Error    error
}

// ProgressEventType identifies the type of progress event.
type ProgressEventType int

const (
	EventScanStart ProgressEventType = iota
	EventScanComplete
	EventFetchStart
	EventFetchAPIComplete
	EventFetchReadmeComplete
	EventFetchSecurityScanComplete
	EventBuildStart
	EventBuildComplete
	EventDatasetStart
	EventDatasetComplete
	EventDatasetError // dataset fetch/build failed (non-fatal; model processing continues)
	EventModelComplete
	EventError
)

// GenerateOptions configures the generation process.
type GenerateOptions struct {
	HFToken          string
	Timeout          time.Duration
	OnProgress       ProgressCallback
	SkipSecurityScan bool // when true, the HF tree security scan is not fetched
}

// BuildDummyBOM builds a single comprehensive dummy BOM with all fields populated.
// This is used in dummy mode for testing/demo purposes without scanning or fetching real data.
func BuildDummyBOM() ([]DiscoveredBOM, error) {
	fetchers := newDummyFetcherSet()

	// Create a dummy discovery.
	dummyDiscovery := scanner.Discovery{
		ID:       "dummy-org/dummy-model",
		Name:     "dummy-model",
		Type:     "huggingface",
		Path:     "/dummy/path",
		Evidence: "from_pretrained('dummy-org/dummy-model')",
	}

	// Fetch dummy metadata.
	apiResp, err := fetchers.modelAPI.FetchRevision("dummy-org/dummy-model", "")
	if err != nil {
		return nil, err
	}

	readme, err := fetchers.modelReadme.FetchRevision("dummy-org/dummy-model", "")
	if err != nil {
		return nil, err
	}

	var securityTree []fetcher.SecurityFileEntry
	if fetchers.modelTree != nil {
		securityTree, _ = fetchers.modelTree.FetchRevision("dummy-org/dummy-model", "")
	}

	// Build the BOM with all dummy data.
	bctx := builder.BuildContext{
		ModelID:      "dummy-org/dummy-model",
		Scan:         dummyDiscovery,
		HF:           apiResp,
		Readme:       readme,
		SecurityTree: securityTree,
	}

	bomBuilder := newBOMBuilder()
	bom, err := bomBuilder.Build(bctx)
	if err != nil {
		return nil, err
	}

	// Build dataset components for any datasets referenced in the model's training metadata.
	noProgress := func(ProgressEvent) {}
	_, resolved := buildDatasetComponents(fetchers, bom, extractDatasetsFromModel(apiResp, readme), "dummy-org/dummy-model", noProgress)
	finalizeModelBOM(bom, resolved)

	return []DiscoveredBOM{
		{
			Discovery: dummyDiscovery,
			BOM:       bom,
		},
	}, nil
}

// BuildPerDiscovery generates an AIBOM for each scanned discovery.
// Fetches HF API metadata → builds BOM per model via registry-driven builder.
// When building a model, if datasets are referenced in the model's training metadata, builds dataset components too.
// Use opts.OnProgress to receive progress events; pass a nil callback to disable.
func BuildPerDiscovery(discoveries []scanner.Discovery, opts GenerateOptions) ([]DiscoveredBOM, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Second
	}

	progress := opts.OnProgress
	if progress == nil {
		progress = func(ProgressEvent) {}
	}

	results := make([]DiscoveredBOM, 0, len(discoveries))

	fetchers := newFetcherSet(newHTTPClient(opts))
	bomBuilder := newBOMBuilder()

	for i, d := range discoveries {
		modelID := strings.TrimSpace(d.ID)
		if modelID == "" {
			modelID = strings.TrimSpace(d.Name)
		}
		revision := strings.TrimSpace(d.Revision)
		label := ModelRef{ID: modelID, Revision: revision}.String() // progress key

		progress(ProgressEvent{Type: EventFetchStart, ModelID: label, Index: i, Total: len(discoveries)})

		var resp *fetcher.ModelAPIResponse
		var readme *fetcher.ModelReadmeCard
		var apiNotFound bool

		if modelID != "" {
			if r, err := fetchers.modelAPI.FetchRevision(modelID, revision); err == nil {
				resp = r
				progress(ProgressEvent{Type: EventFetchAPIComplete, ModelID: label})
			} else {
				if fetcher.IsNotFound(err) || fetcher.IsUnauthorized(err) {
					apiNotFound = true
				}
				progress(ProgressEvent{Type: EventError, ModelID: label, Error: err, Message: fetchErrMessage(apiKind(revision), err)})
			}

			// Skip BOM generation if API fetch returned not found or unauthorized (model not accessible on HF)
			if apiNotFound {
				progress(ProgressEvent{Type: EventModelComplete, ModelID: label, Message: "model skipped: API not found or unauthorized"})
				continue
			}

			if c, err := fetchers.modelReadme.FetchRevision(modelID, revision); err == nil {
				readme = c
				progress(ProgressEvent{Type: EventFetchReadmeComplete, ModelID: label})
			} else {
				progress(ProgressEvent{Type: EventError, ModelID: label, Error: err, Message: fetchErrMessage("README", err)})
			}
		}

		var securityTree []fetcher.SecurityFileEntry
		if modelID != "" && !opts.SkipSecurityScan && fetchers.modelTree != nil {
			if tree, err := fetchers.modelTree.FetchRevision(modelID, revision); err == nil {
				securityTree = tree
				progress(ProgressEvent{Type: EventFetchSecurityScanComplete, ModelID: label})
			} else {
				// Non-fatal: security scan failure should not abort BOM generation.
				progress(ProgressEvent{Type: EventError, ModelID: label, Error: err, Message: fetchErrMessage("security scan", err)})
			}
		}

		progress(ProgressEvent{Type: EventBuildStart, ModelID: label})

		bctx := builder.BuildContext{
			ModelID:      modelID,
			Revision:     revision,
			Scan:         d,
			HF:           resp,
			Readme:       readme,
			SecurityTree: securityTree,
		}

		bom, err := bomBuilder.Build(bctx)
		if err != nil {
			progress(ProgressEvent{Type: EventError, ModelID: label, Error: err, Message: "BOM build failed"})
			continue
		}

		progress(ProgressEvent{Type: EventBuildComplete, ModelID: label})

		datasetCount, resolved := buildDatasetComponents(fetchers, bom, extractDatasetsFromModel(resp, readme), label, progress)
		finalizeModelBOM(bom, resolved)

		progress(ProgressEvent{Type: EventModelComplete, ModelID: label, Datasets: datasetCount})

		results = append(results, DiscoveredBOM{
			Discovery: d,
			BOM:       bom,
		})
	}

	return results, nil
}

// fetchErrMessage returns a user-facing message for a Hugging Face fetch error,.
// distinguishing "not found" (404) from other failures.
func fetchErrMessage(kind string, err error) string {
	if fetcher.IsNotFound(err) {
		return kind + ": not found on Hugging Face Hub"
	}
	return kind + " fetch failed: " + err.Error()
}

// apiKind names the API fetch in error messages, mentioning revision lookups.
func apiKind(revision string) string {
	if revision != "" {
		return "API (revision " + revision + ")"
	}
	return "API"
}

// finalizeModelBOM links model-card dataset references to the built data components.
// and adds the model -> dataset dependency graph.
func finalizeModelBOM(bom *cdx.BOM, resolved map[string]string) {
	builder.LinkDatasetRefs(bom, resolved)
	builder.AddDependencies(bom)
}

// extractDatasetsFromModel extracts dataset IDs from model's training metadata.
func extractDatasetsFromModel(modelResp *fetcher.ModelAPIResponse, readme *fetcher.ModelReadmeCard) []string {
	var datasets []string

	// Check model API response for datasets field.
	if modelResp != nil && modelResp.CardData != nil {
		if datasetsVal, ok := modelResp.CardData["datasets"]; ok {
			// Could be a slice or a single value.
			switch v := datasetsVal.(type) {
			case []interface{}:
				for _, item := range v {
					if dsID, ok := item.(string); ok && strings.TrimSpace(dsID) != "" {
						datasets = append(datasets, strings.TrimSpace(dsID))
					}
				}
			case string:
				if strings.TrimSpace(v) != "" {
					datasets = append(datasets, strings.TrimSpace(v))
				}
			}
		}
	}

	// Check readme for dataset references.
	if readme != nil && readme.Datasets != nil {
		for _, dsID := range readme.Datasets {
			if strings.TrimSpace(dsID) != "" {
				datasets = append(datasets, strings.TrimSpace(dsID))
			}
		}
	}

	// Fallback: datasets known only from dataset:<id> tags.
	if len(datasets) == 0 && modelResp != nil {
		for _, tag := range modelResp.Tags {
			tag = strings.TrimSpace(tag)
			if id, ok := strings.CutPrefix(tag, "dataset:"); ok && strings.TrimSpace(id) != "" {
				datasets = append(datasets, strings.TrimSpace(id))
			}
		}
	}

	// Deduplicate.
	if len(datasets) > 0 {
		seen := make(map[string]struct{})
		unique := make([]string, 0)
		for _, ds := range datasets {
			if _, ok := seen[ds]; !ok {
				seen[ds] = struct{}{}
				unique = append(unique, ds)
			}
		}
		return unique
	}

	return nil
}

// buildDatasetComponents fetches and builds dataset components for a model BOM.
// It appends each successfully built dataset component to bom.Components and returns
// the number of distinct datasets added, plus a map from builder.DatasetRefKey(card
// dataset) to the bom-ref of the component built for it. Because Hugging Face
// redirects renamed datasets, several card names can resolve to one component; it is
// added once. Dataset references that fail to fetch (e.g. not on HuggingFace) are
// skipped here and kept as inline entries by builder.LinkDatasetRefs.
func buildDatasetComponents(fetchers fetcherSet, bom *cdx.BOM, datasets []string, modelID string, progress ProgressCallback) (int, map[string]string) {
	count := 0
	resolved := make(map[string]string)
	for _, dsID := range datasets {
		progress(ProgressEvent{Type: EventDatasetStart, ModelID: modelID, Message: dsID})

		dsResp, err := fetchers.datasetAPI.Fetch(dsID)
		if err != nil {
			progress(ProgressEvent{Type: EventDatasetError, ModelID: modelID, Message: dsID, Error: err})
			continue
		}

		// Use the resolved ID (after redirects) for follow-up requests.
		resolvedID := dsID
		if id := strings.TrimSpace(dsResp.ID); id != "" {
			resolvedID = id
		}
		dsReadme, _ := fetchers.datasetReadme.Fetch(resolvedID)

		dsCtx := builder.DatasetBuildContext{
			DatasetID: resolvedID,
			Scan:      scanner.Discovery{ID: dsID, Name: dsID, Type: "dataset"},
			HF:        dsResp,
			Readme:    dsReadme,
		}

		dsComp, err := newBOMBuilder().BuildDataset(dsCtx)
		if err != nil {
			continue
		}

		if dsComp.BOMRef != "" {
			resolved[builder.DatasetRefKey(dsID)] = dsComp.BOMRef
		}
		if bom.Components == nil {
			bom.Components = &[]cdx.Component{}
		}
		if dsComp.BOMRef == "" || !slices.ContainsFunc(*bom.Components, func(c cdx.Component) bool { return c.BOMRef == dsComp.BOMRef }) {
			*bom.Components = append(*bom.Components, *dsComp)
			count++
		}

		progress(ProgressEvent{Type: EventDatasetComplete, ModelID: modelID, Message: dsID})
	}
	return count, resolved
}

// BuildFromModelIDs generates an AIBOM for each of the provided Hugging Face model IDs.
// Each ID may carry a revision as "org/name@revision" (see ParseModelRef).
// Use opts.OnProgress to receive progress events; pass a nil callback to disable.
func BuildFromModelIDs(modelIDs []string, opts GenerateOptions) ([]DiscoveredBOM, error) {
	discoveries := make([]scanner.Discovery, 0, len(modelIDs))
	for _, rawID := range modelIDs {
		if strings.TrimSpace(rawID) == "" {
			continue
		}
		ref, err := ParseModelRef(rawID)
		if err != nil {
			if opts.OnProgress != nil {
				opts.OnProgress(ProgressEvent{Type: EventError, ModelID: rawID, Error: err, Message: err.Error()})
			}
			continue
		}
		discoveries = append(discoveries, scanner.Discovery{
			ID:       ref.ID,
			Name:     ref.ID,
			Type:     "huggingface",
			Evidence: "from model-id: " + ref.String(),
			Revision: ref.Revision,
		})
	}
	return BuildPerDiscovery(discoveries, opts)
}
