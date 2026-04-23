package generator

import (
	"fmt"
	"net/http"
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
	return builder.NewBOMBuilder(builder.DefaultOptions())
}

// Fetcher factory functions for testing.
type fetcherSet struct {
	modelAPI interface {
		Fetch(string) (*fetcher.ModelAPIResponse, error)
	}
	modelReadme interface {
		Fetch(string) (*fetcher.ModelReadmeCard, error)
	}
	datasetAPI interface {
		Fetch(string) (*fetcher.DatasetAPIResponse, error)
	}
	datasetReadme interface {
		Fetch(string) (*fetcher.DatasetReadmeCard, error)
	}
	modelTree interface {
		Fetch(string) ([]fetcher.SecurityFileEntry, error)
	}
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
	apiResp, err := fetchers.modelAPI.Fetch("dummy-org/dummy-model")
	if err != nil {
		return nil, err
	}

	readme, err := fetchers.modelReadme.Fetch("dummy-org/dummy-model")
	if err != nil {
		return nil, err
	}

	var securityTree []fetcher.SecurityFileEntry
	if fetchers.modelTree != nil {
		securityTree, _ = fetchers.modelTree.Fetch("dummy-org/dummy-model")
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
	buildDatasetComponents(fetchers, bom, extractDatasetsFromModel(apiResp, readme), "dummy-org/dummy-model", noProgress)

	// Add dependencies from model to datasets.
	builder.AddDependencies(bom)

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

		progress(ProgressEvent{Type: EventFetchStart, ModelID: modelID, Index: i, Total: len(discoveries)})

		var resp *fetcher.ModelAPIResponse
		var readme *fetcher.ModelReadmeCard

		if modelID != "" {
			if r, err := fetchers.modelAPI.Fetch(modelID); err == nil {
				resp = r
				progress(ProgressEvent{Type: EventFetchAPIComplete, ModelID: modelID})
			} else {
				progress(ProgressEvent{Type: EventError, ModelID: modelID, Error: err, Message: fetchErrMessage("API", err)})
			}

			if c, err := fetchers.modelReadme.Fetch(modelID); err == nil {
				readme = c
				progress(ProgressEvent{Type: EventFetchReadmeComplete, ModelID: modelID})
			} else {
				progress(ProgressEvent{Type: EventError, ModelID: modelID, Error: err, Message: fetchErrMessage("README", err)})
			}
		}

		var securityTree []fetcher.SecurityFileEntry
		if modelID != "" && !opts.SkipSecurityScan && fetchers.modelTree != nil {
			if tree, err := fetchers.modelTree.Fetch(modelID); err == nil {
				securityTree = tree
				progress(ProgressEvent{Type: EventFetchSecurityScanComplete, ModelID: modelID})
			} else {
				// Non-fatal: security scan failure should not abort BOM generation.
				progress(ProgressEvent{Type: EventError, ModelID: modelID, Error: err, Message: fetchErrMessage("security scan", err)})
			}
		}

		progress(ProgressEvent{Type: EventBuildStart, ModelID: modelID})

		bctx := builder.BuildContext{
			ModelID:      modelID,
			Scan:         d,
			HF:           resp,
			Readme:       readme,
			SecurityTree: securityTree,
		}

		bom, err := bomBuilder.Build(bctx)
		if err != nil {
			progress(ProgressEvent{Type: EventError, ModelID: modelID, Error: err, Message: "BOM build failed"})
			continue
		}

		progress(ProgressEvent{Type: EventBuildComplete, ModelID: modelID})

		datasetCount := buildDatasetComponents(fetchers, bom, extractDatasetsFromModel(resp, readme), modelID, progress)

		// Add dependencies from model to datasets.
		builder.AddDependencies(bom)

		progress(ProgressEvent{Type: EventModelComplete, ModelID: modelID, Datasets: datasetCount})

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
// It appends each successfully built dataset component to bom.Components and returns.
// the number of datasets that were successfully added.
// Dataset references that fail to fetch (e.g. not on HuggingFace) are silently skipped;.
// the references are still preserved in the model's modelCard metadata.
func buildDatasetComponents(fetchers fetcherSet, bom *cdx.BOM, datasets []string, modelID string, progress ProgressCallback) int {
	count := 0
	for _, dsID := range datasets {
		progress(ProgressEvent{Type: EventDatasetStart, ModelID: modelID, Message: dsID})

		dsResp, err := fetchers.datasetAPI.Fetch(dsID)
		if err != nil {
			progress(ProgressEvent{Type: EventDatasetError, ModelID: modelID, Message: dsID, Error: err})
			continue
		}

		dsReadme, _ := fetchers.datasetReadme.Fetch(dsID)

		dsCtx := builder.DatasetBuildContext{
			DatasetID: dsID,
			Scan:      scanner.Discovery{ID: dsID, Name: dsID, Type: "dataset"},
			HF:        dsResp,
			Readme:    dsReadme,
		}

		dsComp, err := newBOMBuilder().BuildDataset(dsCtx)
		if err != nil {
			continue
		}

		if bom.Components == nil {
			bom.Components = &[]cdx.Component{}
		}
		*bom.Components = append(*bom.Components, *dsComp)
		count++

		progress(ProgressEvent{Type: EventDatasetComplete, ModelID: modelID, Message: dsID})
	}
	return count
}

// BuildFromModelIDs generates an AIBOM for each of the provided Hugging Face model IDs.
// Use opts.OnProgress to receive progress events; pass a nil callback to disable.
func BuildFromModelIDs(modelIDs []string, opts GenerateOptions) ([]DiscoveredBOM, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Second
	}

	progress := opts.OnProgress
	if progress == nil {
		progress = func(ProgressEvent) {} // no-op
	}

	results := make([]DiscoveredBOM, 0, len(modelIDs))

	fetchers := newFetcherSet(newHTTPClient(opts))

	for i, modelID := range modelIDs {
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			continue
		}

		bomBuilder := newBOMBuilder()

		progress(ProgressEvent{Type: EventFetchStart, ModelID: modelID, Index: i, Total: len(modelIDs)})

		// Fetch API metadata.
		resp, err := fetchers.modelAPI.Fetch(modelID)
		if err != nil {
			progress(ProgressEvent{Type: EventError, ModelID: modelID, Error: err, Message: "API fetch failed"})
			resp = nil
		} else {
			progress(ProgressEvent{Type: EventFetchAPIComplete, ModelID: modelID})
		}

		// Fetch README.
		readme, err := fetchers.modelReadme.Fetch(modelID)
		if err != nil {
			progress(ProgressEvent{Type: EventError, ModelID: modelID, Error: err, Message: "README fetch failed"})
			readme = nil
		} else {
			progress(ProgressEvent{Type: EventFetchReadmeComplete, ModelID: modelID})
		}

		// Fetch security scan tree (non-fatal).
		var securityTree []fetcher.SecurityFileEntry
		if !opts.SkipSecurityScan && fetchers.modelTree != nil {
			if tree, err := fetchers.modelTree.Fetch(modelID); err == nil {
				securityTree = tree
				progress(ProgressEvent{Type: EventFetchSecurityScanComplete, ModelID: modelID})
			} else {
				progress(ProgressEvent{Type: EventError, ModelID: modelID, Error: err, Message: fetchErrMessage("security scan", err)})
			}
		}

		progress(ProgressEvent{Type: EventBuildStart, ModelID: modelID})

		discovery := scanner.Discovery{
			ID:       modelID,
			Name:     modelID,
			Type:     "huggingface",
			Path:     "",
			Evidence: fmt.Sprintf("from model-id: %s", modelID),
		}

		bctx := builder.BuildContext{
			ModelID:      modelID,
			Scan:         discovery,
			HF:           resp,
			Readme:       readme,
			SecurityTree: securityTree,
		}

		bom, err := bomBuilder.Build(bctx)
		if err != nil {
			progress(ProgressEvent{Type: EventError, ModelID: modelID, Error: err, Message: "BOM build failed"})
			continue
		}

		progress(ProgressEvent{Type: EventBuildComplete, ModelID: modelID})

		datasetCount := buildDatasetComponents(fetchers, bom, extractDatasetsFromModel(resp, readme), modelID, progress)

		// Add dependencies from model to datasets.
		builder.AddDependencies(bom)

		progress(ProgressEvent{Type: EventModelComplete, ModelID: modelID, Datasets: datasetCount})

		results = append(results, DiscoveredBOM{
			Discovery: discovery,
			BOM:       bom,
		})
	}

	return results, nil
}
