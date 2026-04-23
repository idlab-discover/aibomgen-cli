package enricher

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/internal/apperr"
	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
	"github.com/idlab-discover/aibomgen-cli/internal/metadata"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/completeness"
)

// Config holds enrichment configuration.
type Config struct {
	Strategy     string  // "interactive" or "file"
	ConfigFile   string  // path to config file (for file strategy)
	RequiredOnly bool    // only enrich required fields
	MinWeight    float64 // minimum weight threshold
	Refetch      bool    // refetch from Hugging Face
	NoPreview    bool    // skip preview
	SpecVersion  string  // CycloneDX spec version
	HFToken      string  // Hugging Face token
	HFBaseURL    string  // Hugging Face base URL
	HFTimeout    int     // timeout in seconds
}

// Options for creating an Enricher.
type Options struct {
	Reader io.Reader
	Writer io.Writer
	Config Config
}

// Enricher handles AIBOM enrichment.
type Enricher struct {
	reader io.Reader
	writer io.Writer
	config Config
	scan   *bufio.Scanner
}

// New creates a new Enricher.
func New(opts Options) *Enricher {
	return &Enricher{
		reader: opts.Reader,
		writer: opts.Writer,
		config: opts.Config,
		scan:   bufio.NewScanner(opts.Reader),
	}
}

// Enrich enriches a BOM with additional metadata.
func (e *Enricher) Enrich(bom *cdx.BOM, configViper interface{}) (*cdx.BOM, error) {
	if bom == nil {
		return nil, fmt.Errorf("nil BOM")
	}

	// Get model ID from BOM.
	modelID := extractModelID(bom)
	if modelID == "" {
		fmt.Fprintf(e.writer, "warning: no model ID found in BOM; enrichment will proceed without a model ID\n")
	}

	// Run initial completeness check.
	initialResult := completeness.Check(bom)

	// Refetch metadata if requested and apply it to BOM.
	var hfAPI *fetcher.ModelAPIResponse
	var hfReadme *fetcher.ModelReadmeCard
	var postRefetchResult completeness.Result
	if e.config.Refetch && modelID != "" {
		hfAPI, hfReadme = e.refetchMetadata(modelID)

		// Apply refetched metadata to BOM.
		if hfAPI != nil || hfReadme != nil {
			e.applyRefetchedMetadata(bom, modelID, hfAPI, hfReadme)

			// Check completeness after refetch.
			postRefetchResult = completeness.Check(bom)
		} else {
			postRefetchResult = initialResult
		}
	} else {
		postRefetchResult = initialResult
	}

	// STEP 1: Enrich model fields.
	modelChanges, err := e.enrichModel(bom, modelID, hfAPI, hfReadme, postRefetchResult, configViper)
	if err != nil {
		return nil, fmt.Errorf("failed to enrich model: %w", err)
	}

	// STEP 2: Enrich dataset components if they exist.
	datasetChanges := make(map[string]map[metadata.DatasetKey]string)
	if bom.Components != nil {
		for i := range *bom.Components {
			comp := &(*bom.Components)[i]
			if comp.Type == cdx.ComponentTypeData {
				dsChanges, err := e.enrichDataset(bom, comp, configViper)
				if err != nil {
					fmt.Fprintf(e.writer, "warning: failed to enrich dataset %q: %v\n", comp.Name, err)
					continue
				}
				if len(dsChanges) > 0 {
					datasetChanges[comp.Name] = dsChanges
				}
			}
		}
	}

	// Show preview if requested.
	if !e.config.NoPreview && (len(modelChanges) > 0 || len(datasetChanges) > 0) {
		confirm, err := ShowPreviewWithConfirm(initialResult, postRefetchResult, bom, modelChanges, datasetChanges)
		if err != nil {
			return nil, fmt.Errorf("preview error: %w", err)
		}
		if !confirm {
			return nil, apperr.ErrCancelled
		}
	}

	return bom, nil
}

// enrichModel enriches the main model component.
func (e *Enricher) enrichModel(bom *cdx.BOM, modelID string, hfAPI *fetcher.ModelAPIResponse, hfReadme *fetcher.ModelReadmeCard, result completeness.Result, configViper interface{}) (map[metadata.Key]string, error) {
	// Collect missing fields based on config (using post-refetch state).
	missingFields := e.collectMissingFields(result)
	if len(missingFields) == 0 {
		return nil, nil
	}

	// Prepare enrichment source.
	src := metadata.Source{
		ModelID: modelID,
		HF:      hfAPI,
		Readme:  hfReadme,
	}

	// Prepare enrichment target - modify the BOM directly.
	tgt := metadata.Target{
		BOM:                bom,
		Component:          bomComponent(bom),
		ModelCard:          bomModelCard(bom),
		HuggingFaceBaseURL: e.config.HFBaseURL,
	}

	var changes map[metadata.Key]string
	var err error

	switch e.config.Strategy {
	case "file":
		changes, err = e.enrichModelFromFile(missingFields, src, tgt, configViper)
	case "interactive":
		// Use new interactive enricher.
		ie := NewInteractiveEnricher(e)
		changes, err = ie.EnrichInteractive(bom, missingFields, src, tgt)
	default:
		return nil, fmt.Errorf("unknown strategy: %s", e.config.Strategy)
	}

	if err != nil {
		return nil, err
	}

	return changes, nil
}

// enrichModelFromFile handles file-based enrichment.
func (e *Enricher) enrichModelFromFile(
	missingFields []metadata.FieldSpec,
	src metadata.Source,
	tgt metadata.Target,
	configViper interface{},
) (map[metadata.Key]string, error) {
	changes := make(map[metadata.Key]string)

	for _, spec := range missingFields {
		value, err := e.getValueFromFile(spec, configViper)
		if err != nil {
			return nil, fmt.Errorf("failed to get value for %s: %w", spec.Key, err)
		}

		if value != nil {
			err = e.applyValue(spec, &src, &tgt, value)
			if err != nil {
				return nil, err
			}
			changes[spec.Key] = formatValue(value)
		}
	}

	return changes, nil
}

// enrichDatasetFromFile handles file-based dataset enrichment.
func (e *Enricher) enrichDatasetFromFile(
	missingFields []metadata.DatasetFieldSpec,
	src metadata.DatasetSource,
	tgt metadata.DatasetTarget,
	configViper interface{},
) (map[metadata.DatasetKey]string, error) {
	changes := make(map[metadata.DatasetKey]string)

	for _, spec := range missingFields {
		value, err := e.getDatasetValueFromFile(spec, configViper)
		if err != nil {
			return nil, fmt.Errorf("failed to get value for %s: %w", spec.Key, err)
		}

		if value != nil {
			err = e.applyDatasetValue(spec, &src, &tgt, value)
			if err != nil {
				return nil, fmt.Errorf("failed to apply value for %s: %w", spec.Key, err)
			}
			changes[spec.Key] = formatValue(value)
		}
	}

	return changes, nil
}

// enrichDataset enriches a single dataset component.
func (e *Enricher) enrichDataset(bom *cdx.BOM, comp *cdx.Component, configViper interface{}) (map[metadata.DatasetKey]string, error) {
	datasetID := comp.Name

	// Check dataset completeness.
	dsReport := completeness.CheckDataset(comp)

	// Collect missing dataset fields.
	missingFields := e.collectMissingDatasetFields(dsReport)
	if len(missingFields) == 0 {
		return nil, nil
	}

	// Refetch dataset metadata if requested.
	var hfAPI *fetcher.DatasetAPIResponse
	var hfReadme *fetcher.DatasetReadmeCard
	if e.config.Refetch && datasetID != "" {
		hfAPI, hfReadme = e.refetchDatasetMetadata(datasetID)
	}

	// Prepare enrichment source.
	src := metadata.DatasetSource{
		DatasetID: datasetID,
		HF:        hfAPI,
		Readme:    hfReadme,
	}

	// Prepare enrichment target.
	tgt := metadata.DatasetTarget{
		Component:                 comp,
		IncludeEvidenceProperties: false,
		HuggingFaceBaseURL:        e.config.HFBaseURL,
	}

	var changes map[metadata.DatasetKey]string
	var err error

	switch e.config.Strategy {
	case "file":
		changes, err = e.enrichDatasetFromFile(missingFields, src, tgt, configViper)
	case "interactive":
		// Use new interactive enricher.
		ie := NewInteractiveEnricher(e)
		changes, err = ie.EnrichDatasetInteractive(comp, missingFields, src, tgt)
	default:
		return nil, fmt.Errorf("unknown strategy: %s", e.config.Strategy)
	}

	if err != nil {
		return nil, err
	}

	return changes, nil
}

// collectMissingFields returns fields that need enrichment based on config.
func (e *Enricher) collectMissingFields(result completeness.Result) []metadata.FieldSpec {
	var fields []metadata.FieldSpec

	for _, spec := range metadata.Registry() {
		// Skip if weight is 0 or below threshold.
		if spec.Weight <= 0 || spec.Weight < e.config.MinWeight {
			continue
		}

		// Check if field is missing.
		isMissing := false
		for _, k := range result.MissingRequired {
			if k == spec.Key {
				isMissing = true
				break
			}
		}
		if !isMissing && !e.config.RequiredOnly {
			for _, k := range result.MissingOptional {
				if k == spec.Key {
					isMissing = true
					break
				}
			}
		}

		if isMissing {
			fields = append(fields, spec)
		}
	}

	return fields
}

// refetchMetadata fetches fresh metadata from Hugging Face.
func (e *Enricher) refetchMetadata(modelID string) (*fetcher.ModelAPIResponse, *fetcher.ModelReadmeCard) {
	client := fetcher.NewHFClient(time.Duration(e.config.HFTimeout)*time.Second, e.config.HFToken)

	apiResp, err := (&fetcher.ModelAPIFetcher{
		Client:  client,
		BaseURL: e.config.HFBaseURL,
	}).Fetch(modelID)
	if err != nil {
		fmt.Fprintf(e.writer, "warning: failed to fetch model API metadata for %q: %v\n", modelID, err)
		apiResp = nil
	}

	readme, err := (&fetcher.ModelReadmeFetcher{
		Client:  client,
		BaseURL: e.config.HFBaseURL,
	}).Fetch(modelID)
	if err != nil {
		fmt.Fprintf(e.writer, "warning: failed to fetch model README for %q: %v\n", modelID, err)
		readme = nil
	}

	return apiResp, readme
}

// applyRefetchedMetadata applies all available metadata from HuggingFace to the BOM.
func (e *Enricher) applyRefetchedMetadata(bom *cdx.BOM, modelID string, hfAPI *fetcher.ModelAPIResponse, hfReadme *fetcher.ModelReadmeCard) {
	src := metadata.Source{
		ModelID: modelID,
		HF:      hfAPI,
		Readme:  hfReadme,
	}

	tgt := metadata.Target{
		BOM:                bom,
		Component:          bomComponent(bom),
		ModelCard:          bomModelCard(bom),
		HuggingFaceBaseURL: e.config.HFBaseURL,
	}

	// Apply all field specs that have Apply functions.
	totalSpecs := 0
	specsWithWeight := 0
	for _, spec := range metadata.Registry() {
		metadata.ApplyFromSources(spec, src, tgt)
		totalSpecs++
		if spec.Weight > 0 {
			specsWithWeight++
		}
	}

}

// getValueFromFile extracts a value from the config file.
func (e *Enricher) getValueFromFile(spec metadata.FieldSpec, configViper interface{}) (interface{}, error) {
	// If no config provided, return nil.
	if configViper == nil {
		return nil, nil
	}

	// Type assert to viper.Viper.
	type viperGetter interface {
		Get(key string) interface{}
	}

	v, ok := configViper.(viperGetter)
	if !ok {
		return nil, fmt.Errorf("invalid config type")
	}

	// Use the full key - viper will handle the nested lookup and lowercasing.
	key := string(spec.Key)
	val := v.Get(key)

	if val != nil {
		return val, nil
	}

	return nil, nil
}

// applyValue applies a user-provided value to the BOM using the FieldSpec's SetUserValue function.
func (e *Enricher) applyValue(spec metadata.FieldSpec, src *metadata.Source, tgt *metadata.Target, value interface{}) error {
	strValue := fmt.Sprintf("%v", value)

	// Use the FieldSpec's SetUserValue if available.
	err := metadata.ApplyUserValue(spec, strValue, *tgt)
	if err != nil {
		return fmt.Errorf("failed to set user value for %s: %w", spec.Key, err)
	}
	return nil
}

// Helper functions.

func extractModelID(bom *cdx.BOM) string {
	if c := bomComponent(bom); c != nil {
		return c.Name
	}
	return ""
}

func bomComponent(bom *cdx.BOM) *cdx.Component {
	if bom == nil || bom.Metadata == nil || bom.Metadata.Component == nil {
		return nil
	}
	return bom.Metadata.Component
}

func bomModelCard(bom *cdx.BOM) *cdx.MLModelCard {
	c := bomComponent(bom)
	if c == nil || c.ModelCard == nil {
		return nil
	}
	return c.ModelCard
}

func formatValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case int, int64, float64:
		return fmt.Sprintf("%v", val)
	case []string:
		return strings.Join(val, ", ")
	default:
		return fmt.Sprintf("%v", val)
	}
}

// Dataset-specific helper functions.

// collectMissingDatasetFields returns dataset fields that need enrichment.
func (e *Enricher) collectMissingDatasetFields(result completeness.DatasetResult) []metadata.DatasetFieldSpec {
	var fields []metadata.DatasetFieldSpec

	for _, spec := range metadata.DatasetRegistry() {
		// Skip if weight is 0 or below threshold.
		if spec.Weight <= 0 || spec.Weight < e.config.MinWeight {
			continue
		}

		// Check if field is missing.
		isMissing := false
		for _, k := range result.MissingRequired {
			if k == spec.Key {
				isMissing = true
				break
			}
		}
		if !isMissing && !e.config.RequiredOnly {
			for _, k := range result.MissingOptional {
				if k == spec.Key {
					isMissing = true
					break
				}
			}
		}

		if isMissing {
			fields = append(fields, spec)
		}
	}

	return fields
}

// refetchDatasetMetadata fetches fresh metadata for a dataset from Hugging Face.
func (e *Enricher) refetchDatasetMetadata(datasetID string) (*fetcher.DatasetAPIResponse, *fetcher.DatasetReadmeCard) {
	client := fetcher.NewHFClient(time.Duration(e.config.HFTimeout)*time.Second, e.config.HFToken)

	apiResp, err := (&fetcher.DatasetAPIFetcher{
		Client:  client,
		BaseURL: e.config.HFBaseURL,
	}).Fetch(datasetID)
	if err != nil {
		fmt.Fprintf(e.writer, "warning: failed to fetch dataset API metadata for %q: %v\n", datasetID, err)
		apiResp = nil
	}

	readme, err := (&fetcher.DatasetReadmeFetcher{
		Client:  client,
		BaseURL: e.config.HFBaseURL,
	}).Fetch(datasetID)
	if err != nil {
		fmt.Fprintf(e.writer, "warning: failed to fetch dataset README for %q: %v\n", datasetID, err)
		readme = nil
	}

	return apiResp, readme
}

// getDatasetValueFromFile extracts a dataset value from the config file.
func (e *Enricher) getDatasetValueFromFile(spec metadata.DatasetFieldSpec, configViper interface{}) (interface{}, error) {
	if configViper == nil {
		return nil, nil
	}

	type viperGetter interface {
		Get(key string) interface{}
	}

	v, ok := configViper.(viperGetter)
	if !ok {
		return nil, fmt.Errorf("invalid config type")
	}

	key := string(spec.Key)
	val := v.Get(key)

	if val != nil {
		return val, nil
	}

	return nil, nil
}

// applyDatasetValue applies a user-provided value to a dataset component.
func (e *Enricher) applyDatasetValue(spec metadata.DatasetFieldSpec, src *metadata.DatasetSource, tgt *metadata.DatasetTarget, value interface{}) error {
	strValue := fmt.Sprintf("%v", value)

	err := metadata.ApplyDatasetUserValue(spec, strValue, *tgt)
	if err != nil {
		return fmt.Errorf("failed to set user value for %s: %w", spec.Key, err)
	}
	return nil
}
