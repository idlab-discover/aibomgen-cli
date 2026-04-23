package metadata

import (
	"strings"
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/scanner"
)

func specFor(t *testing.T, key Key) FieldSpec {
	t.Helper()
	for _, spec := range Registry() {
		if spec.Key == key {
			return spec
		}
	}
	t.Fatalf("missing spec %s", key)
	return FieldSpec{}
}

func TestRegistryApplyAndPresent(t *testing.T) {
	comp := &cdx.Component{ModelCard: &cdx.MLModelCard{}}
	bom := cdx.NewBOM()
	bom.Metadata = &cdx.Metadata{Component: comp}

	src := Source{
		ModelID: "org/model",
		Scan: scanner.Discovery{
			Name:     "scan-name",
			Type:     "model",
			Path:     "/tmp/model.py",
			Evidence: "pattern",
		},
		HF: &fetcher.ModelAPIResponse{
			ID:          "hf-org/hf-model",
			ModelID:     "hf-org/hf-model",
			Author:      "hf-author",
			PipelineTag: "classification",
			LibraryName: "transformers",
			Tags:        []string{"tag1", "license:apache-2.0", "dataset:ds1", "tag1"},
			License:     "mit",
			SHA:         "deadbeef",
			Downloads:   42,
			Likes:       7,
			LastMod:     "2024-01-01",
			CreatedAt:   "2023-01-01",
			Private:     true,
			UsedStorage: 9,
			CardData: map[string]any{
				"license":  "card-license",
				"language": []any{"en", "fr"},
				"datasets": []any{"ds2", "ds2"},
			},
		},
		Readme: &fetcher.ModelReadmeCard{
			BaseModel:                  "bert-base-uncased",
			Tags:                       []string{"tag-readme"},
			License:                    "apache-2.0",
			Datasets:                   []string{"glue"},
			Metrics:                    []string{"accuracy"},
			DirectUse:                  "Use for classification.",
			OutOfScopeUse:              "Do not use for medical.",
			BiasRisksLimitations:       "May be biased.",
			BiasRecommendations:        "Use with care.",
			ModelCardContact:           "contact@example.com",
			EnvironmentalHardwareType:  "NVIDIA A100",
			EnvironmentalHoursUsed:     "10",
			EnvironmentalCloudProvider: "AWS",
			EnvironmentalComputeRegion: "us-east-1",
			EnvironmentalCarbonEmitted: "123g",
			ModelIndexMetrics:          []fetcher.ModelIndexMetric{{Type: "accuracy", Value: "0.91"}},
		},
	}
	src.HF.Config.ModelType = "bert"
	src.HF.Config.Architectures = []string{"BertForSequenceClassification"}

	// Provide a minimal security tree so the security FieldSpecs have data to present.
	safeStatus := &fetcher.SecurityFileStatus{Status: "safe"}
	src.SecurityTree = []fetcher.SecurityFileEntry{
		{Type: "file", OID: "abc", Path: "model.safetensors", SecurityFileStatus: safeStatus},
	}

	tgt := Target{
		BOM:                       bom,
		Component:                 comp,
		ModelCard:                 comp.ModelCard,
		IncludeEvidenceProperties: true,
		HuggingFaceBaseURL:        "https://huggingface.co",
	}

	specs := Registry()
	for _, spec := range specs {
		ApplyFromSources(spec, src, tgt)
	}

	if comp.Name != "scan-name" {
		t.Fatalf("component name = %q", comp.Name)
	}
	if comp.ExternalReferences == nil || len(*comp.ExternalReferences) == 0 {
		t.Fatalf("expected external references")
	}
	if comp.Tags == nil || len(*comp.Tags) != 3 {
		t.Fatalf("expected normalized tags, got %v", comp.Tags)
	}
	if comp.Licenses == nil || len(*comp.Licenses) != 1 {
		t.Fatalf("expected licenses populated")
	}
	if comp.Hashes == nil || len(*comp.Hashes) != 1 {
		t.Fatalf("expected hashes populated")
	}
	if comp.Manufacturer == nil || comp.Manufacturer.Name != "hf-author" {
		t.Fatalf("manufacturer mismatch")
	}
	// Group is now extracted from ModelID (first part before /).
	if comp.Group != "hf-org" {
		t.Fatalf("group mismatch: expected 'hf-org', got %q", comp.Group)
	}
	if comp.Properties == nil || !hasProperty(comp, "huggingface:lastModified") {
		t.Fatalf("expected huggingface properties")
	}
	if !hasProperty(comp, "huggingface:baseModel") {
		t.Fatalf("expected modelcard baseModel property")
	}
	if comp.ModelCard == nil || comp.ModelCard.ModelParameters == nil {
		t.Fatalf("model parameters missing")
	}
	mp := comp.ModelCard.ModelParameters
	if mp.Task != "classification" || mp.ArchitectureFamily != "bert" || mp.ModelArchitecture != "BertForSequenceClassification" {
		t.Fatalf("model parameters not populated: %#v", mp)
	}
	if mp.Datasets == nil || len(*mp.Datasets) != 2 {
		t.Fatalf("datasets not populated: %#v", mp.Datasets)
	}
	if comp.ModelCard.Considerations == nil || comp.ModelCard.Considerations.UseCases == nil || len(*comp.ModelCard.Considerations.UseCases) == 0 {
		t.Fatalf("expected model card considerations use cases")
	}
	if comp.ModelCard.QuantitativeAnalysis == nil || comp.ModelCard.QuantitativeAnalysis.PerformanceMetrics == nil || len(*comp.ModelCard.QuantitativeAnalysis.PerformanceMetrics) == 0 {
		t.Fatalf("expected model card quantitative analysis metrics")
	}

	for _, spec := range specs {
		if spec.Present != nil && !spec.Present(bom) {
			t.Fatalf("expected present for %s", spec.Key)
		}
	}
}

func TestDatasetPresentHandlesMissingRefs(t *testing.T) {
	var datasetSpec FieldSpec
	for _, spec := range Registry() {
		if spec.Key == ModelCardModelParametersDatasets {
			datasetSpec = spec
			break
		}
	}
	if datasetSpec.Present == nil {
		t.Fatalf("dataset spec missing present")
	}

	bom := cdx.NewBOM()
	comp := &cdx.Component{ModelCard: &cdx.MLModelCard{ModelParameters: &cdx.MLModelParameters{}}}
	bom.Metadata = &cdx.Metadata{Component: comp}

	if datasetSpec.Present(bom) {
		t.Fatalf("expected missing datasets to be false")
	}

	comp.ModelCard.ModelParameters.Datasets = &[]cdx.MLDatasetChoice{{Ref: " "}}
	if datasetSpec.Present(bom) {
		t.Fatalf("expected blank dataset ref to be false")
	}
}

func TestRegistryApplyHandlesNilTargets(t *testing.T) {
	for _, spec := range Registry() {
		ApplyFromSources(spec, Source{}, Target{})
	}
}

func TestComponentNameFallbacks(t *testing.T) {
	spec := specFor(t, ComponentName)
	comp := &cdx.Component{}
	src := Source{ModelID: "base", HF: &fetcher.ModelAPIResponse{ID: " ", ModelID: "hf/model"}}
	ApplyFromSources(spec, src, Target{Component: comp})
	if comp.Name != "hf/model" {
		t.Fatalf("expected HF model ID fallback, got %q", comp.Name)
	}
}

func TestComponentExternalReferenceBranches(t *testing.T) {
	spec := specFor(t, ComponentExternalReferences)
	t.Run("missing model id", func(t *testing.T) {
		comp := &cdx.Component{}
		ApplyFromSources(spec, Source{}, Target{Component: comp, HuggingFaceBaseURL: "https://example.com"})
		if comp.ExternalReferences != nil {
			t.Fatalf("expected no references when model id missing")
		}
	})
	t.Run("defaults base url", func(t *testing.T) {
		comp := &cdx.Component{}
		ApplyFromSources(spec, Source{ModelID: "org/model"}, Target{Component: comp})
		if comp.ExternalReferences == nil || len(*comp.ExternalReferences) != 1 {
			t.Fatalf("expected one reference")
		}
		if url := (*comp.ExternalReferences)[0].URL; url != "https://huggingface.co/org/model" {
			t.Fatalf("unexpected reference url %q", url)
		}
	})
}

func TestComponentTagsSkipEmpty(t *testing.T) {
	spec := specFor(t, ComponentTags)
	comp := &cdx.Component{}
	ApplyFromSources(spec, Source{HF: &fetcher.ModelAPIResponse{Tags: []string{" ", "\t"}}}, Target{Component: comp})
	if comp.Tags != nil {
		t.Fatalf("expected tags to remain nil when inputs empty")
	}
}

func TestComponentLicensesSkipMissing(t *testing.T) {
	spec := specFor(t, ComponentLicenses)
	comp := &cdx.Component{}
	ApplyFromSources(spec, Source{HF: &fetcher.ModelAPIResponse{}}, Target{Component: comp})
	if comp.Licenses != nil {
		t.Fatalf("expected licenses to be nil when missing")
	}
}

func TestComponentHashesSkipMissing(t *testing.T) {
	spec := specFor(t, ComponentHashes)
	comp := &cdx.Component{}
	ApplyFromSources(spec, Source{HF: &fetcher.ModelAPIResponse{SHA: " "}}, Target{Component: comp})
	if comp.Hashes != nil {
		t.Fatalf("expected hashes to be nil when sha missing")
	}
}

func TestManufacturerAndGroupSkipEmptyAuthor(t *testing.T) {
	comp := &cdx.Component{}
	src := Source{HF: &fetcher.ModelAPIResponse{Author: " "}}
	ApplyFromSources(specFor(t, ComponentManufacturer), src, Target{Component: comp})
	ApplyFromSources(specFor(t, ComponentGroup), src, Target{Component: comp})
	if comp.Manufacturer != nil || comp.Group != "" {
		t.Fatalf("expected manufacturer and group to remain empty")
	}
}

func TestModelCardSpecsSkipEmptyValues(t *testing.T) {
	t.Run("task", func(t *testing.T) {
		card := &cdx.MLModelCard{}
		ApplyFromSources(specFor(t, ModelCardModelParametersTask), Source{HF: &fetcher.ModelAPIResponse{PipelineTag: " "}}, Target{ModelCard: card})
		if card.ModelParameters != nil {
			t.Fatalf("expected model parameters to remain nil")
		}
	})
	t.Run("architecture family", func(t *testing.T) {
		card := &cdx.MLModelCard{}
		ApplyFromSources(specFor(t, ModelCardModelParametersArchitectureFamily), Source{HF: &fetcher.ModelAPIResponse{}}, Target{ModelCard: card})
		if card.ModelParameters != nil {
			t.Fatalf("expected model parameters to remain nil")
		}
	})
	t.Run("model architecture guards", func(t *testing.T) {
		card := &cdx.MLModelCard{}
		// len(architectures) == 0.
		ApplyFromSources(specFor(t, ModelCardModelParametersModelArchitecture), Source{HF: &fetcher.ModelAPIResponse{}}, Target{ModelCard: card})
		if card.ModelParameters != nil {
			t.Fatalf("expected model parameters to remain nil for empty list")
		}
		// blank first architecture.
		card2 := &cdx.MLModelCard{}
		hf := &fetcher.ModelAPIResponse{}
		hf.Config.Architectures = []string{" "}
		ApplyFromSources(specFor(t, ModelCardModelParametersModelArchitecture), Source{HF: hf}, Target{ModelCard: card2})
		if card2.ModelParameters != nil {
			t.Fatalf("expected model parameters to remain nil for blank architecture")
		}
	})
}

func TestDatasetApplySkipsEmptySources(t *testing.T) {
	card := &cdx.MLModelCard{}
	spec := specFor(t, ModelCardModelParametersDatasets)
	ApplyFromSources(spec, Source{HF: &fetcher.ModelAPIResponse{}}, Target{ModelCard: card})
	if card.ModelParameters != nil {
		t.Fatalf("expected dataset spec to skip when no data available")
	}
}

func TestHFPropsSkipWithoutHFData(t *testing.T) {
	comp := &cdx.Component{}
	tgt := Target{Component: comp}
	for _, spec := range Registry() {
		if strings.Contains(spec.Key.String(), "properties.huggingface") {
			ApplyFromSources(spec, Source{}, tgt)
		}
	}
	if comp.Properties != nil {
		t.Fatalf("expected no properties when HF data missing")
	}
	if hasProperty(comp, " ") {
		t.Fatalf("expected false for blank lookup")
	}
}

func TestHelperFunctions(t *testing.T) {
	if got := extractLicense(map[string]any{"license": " mit "}, nil); got != "mit" {
		t.Fatalf("extractLicense card data = %q", got)
	}
	if got := extractLicense(nil, []string{"license:apache-2.0"}); got != "apache-2.0" {
		t.Fatalf("extractLicense tags = %q", got)
	}

	if got := extractLanguage(map[string]any{"language": " en "}); got != "en" {
		t.Fatalf("language string = %q", got)
	}
	if got := extractLanguage(map[string]any{"language": []any{"en", "fr"}}); got != "en,fr" {
		t.Fatalf("language slice = %q", got)
	}
	if extractLanguage(map[string]any{}) != "" {
		t.Fatalf("expected empty language when key missing")
	}

	datasets := extractDatasets(map[string]any{"datasets": []any{"ds1"}}, []string{"dataset:ds2", "dataset:ds1"})
	if len(datasets) != 2 || datasets[0] != "dataset:ds1" || datasets[1] != "dataset:ds2" {
		t.Fatalf("datasets = %v", datasets)
	}
	if got := extractDatasets(map[string]any{"datasets": "solo"}, nil); len(got) != 1 || got[0] != "dataset:solo" {
		t.Fatalf("expected string dataset, got %v", got)
	}
	if got := extractDatasets(nil, []string{"dataset:tagged-dataset"}); len(got) != 1 || got[0] != "dataset:tagged-dataset" {
		t.Fatalf("expected dataset from tags, got %v", got)
	}

	if got := normalizeStrings([]string{" a ", "a", "b", "  "}); len(got) != 2 {
		t.Fatalf("normalizeStrings = %v", got)
	}

	comp := &cdx.Component{}
	setProperty(comp, " ", "value")
	if comp.Properties != nil {
		t.Fatalf("expected no properties for blank name")
	}
	if hasProperty(comp, "anything") {
		t.Fatalf("expected false when properties nil")
	}
	setProperty(comp, "prop", " value ")
	if !hasProperty(comp, "prop") {
		t.Fatalf("expected property set")
	}
	if hasProperty(comp, "missing") {
		t.Fatalf("unexpected property match")
	}
	comp.Properties = &[]cdx.Property{{Name: " padded ", Value: "trimmed"}, {Name: "empty", Value: " "}}
	if !hasProperty(comp, "padded") {
		t.Fatalf("expected trimmed property match")
	}
	if hasProperty(comp, "empty") {
		t.Fatalf("expected empty value to be ignored")
	}
	if hasProperty(comp, " ") {
		t.Fatalf("expected blank lookup to be ignored even with properties")
	}

	setProperty(nil, "prop", "value")
	if hasProperty(nil, "prop") {
		t.Fatalf("expected false for nil component")
	}

	if ensureModelParameters(&cdx.MLModelCard{}) == nil {
		t.Fatalf("expected model parameters allocated")
	}

	emptyBOM := &cdx.BOM{}
	if bomComponent(emptyBOM) != nil || bomHasComponentName(emptyBOM) {
		t.Fatalf("expected nil component helpers")
	}
	if bomModelParameters(emptyBOM) != nil {
		t.Fatalf("expected nil model parameters when metadata missing")
	}

	compWithName := &cdx.Component{Name: "name", ModelCard: &cdx.MLModelCard{}}
	namedBOM := cdx.NewBOM()
	namedBOM.Metadata = &cdx.Metadata{Component: compWithName}
	if bomComponent(namedBOM) == nil || !bomHasComponentName(namedBOM) {
		t.Fatalf("expected component present")
	}
	if bomModelParameters(namedBOM) != nil {
		t.Fatalf("expected nil model parameters before allocation")
	}
	compWithCard := &cdx.Component{}
	bomWithCard := cdx.NewBOM()
	bomWithCard.Metadata = &cdx.Metadata{Component: compWithCard}
	if bomModelParameters(bomWithCard) != nil {
		t.Fatalf("expected nil when model card missing")
	}

	if extractLicense(nil, nil) != "" {
		t.Fatalf("expected empty license when no data")
	}
	if extractLanguage(nil) != "" {
		t.Fatalf("expected empty language when absent")
	}
	if extractLanguage(map[string]any{"language": 123}) != "" {
		t.Fatalf("expected empty language for unsupported type")
	}

	if out := extractDatasets(map[string]any{"datasets": []any{"  "}}, nil); len(out) != 0 {
		t.Fatalf("expected no datasets when inputs empty")
	}
	if len(normalizeStrings(nil)) != 0 {
		t.Fatalf("expected empty slice for nil input")
	}
}

func TestRegistryPresentHandlesNilBOM(t *testing.T) {
	for _, spec := range Registry() {
		if spec.Present != nil {
			spec.Present(nil)
		}
	}
}
