package metadata

import (
	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/scanner"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

// Key identifies a CycloneDX field (or pseudo-field) we want to populate/check.
type Key string

func (k Key) String() string { return string(k) }

const (
	// BOM.metadata.component.* (MODEL).
	ComponentName               Key = "BOM.metadata.component.name"
	ComponentExternalReferences Key = "BOM.metadata.component.externalReferences"
	ComponentTags               Key = "BOM.metadata.component.tags"
	ComponentLicenses           Key = "BOM.metadata.component.licenses"
	ComponentHashes             Key = "BOM.metadata.component.hashes"
	ComponentManufacturer       Key = "BOM.metadata.component.manufacturer"
	ComponentGroup              Key = "BOM.metadata.component.group"

	// Component-level extra properties (stored later as CycloneDX Component.Properties).
	ComponentPropertiesHuggingFaceLastModified Key = "BOM.metadata.component.properties.huggingface:lastModified"
	ComponentPropertiesHuggingFaceCreatedAt    Key = "BOM.metadata.component.properties.huggingface:createdAt"
	ComponentPropertiesHuggingFaceLanguage     Key = "BOM.metadata.component.properties.huggingface:language"
	ComponentPropertiesHuggingFaceUsedStorage  Key = "BOM.metadata.component.properties.huggingface:usedStorage"
	ComponentPropertiesHuggingFacePrivate      Key = "BOM.metadata.component.properties.huggingface:private"
	ComponentPropertiesHuggingFaceLibraryName  Key = "BOM.metadata.component.properties.huggingface:libraryName"
	ComponentPropertiesHuggingFaceDownloads    Key = "BOM.metadata.component.properties.huggingface:downloads"
	ComponentPropertiesHuggingFaceLikes        Key = "BOM.metadata.component.properties.huggingface:likes"
	ComponentPropertiesHuggingFaceBaseModel    Key = "BOM.metadata.component.properties.huggingface:baseModel"
	ComponentPropertiesHuggingFaceContact      Key = "BOM.metadata.component.properties.huggingface:modelCardContact"

	// BOM.metadata.component.modelCard.* (MODEL CARD).
	ModelCardModelParametersTask                                 Key = "BOM.metadata.component.modelCard.modelParameters.task"
	ModelCardModelParametersArchitectureFamily                   Key = "BOM.metadata.component.modelCard.modelParameters.architectureFamily"
	ModelCardModelParametersModelArchitecture                    Key = "BOM.metadata.component.modelCard.modelParameters.modelArchitecture"
	ModelCardModelParametersDatasets                             Key = "BOM.metadata.component.modelCard.modelParameters.datasets"
	ModelCardConsiderationsUseCases                              Key = "BOM.metadata.component.modelCard.considerations.useCases"
	ModelCardConsiderationsTechnicalLimitations                  Key = "BOM.metadata.component.modelCard.considerations.technicalLimitations"
	ModelCardConsiderationsEthicalConsiderations                 Key = "BOM.metadata.component.modelCard.considerations.ethicalConsiderations"
	ModelCardQuantitativeAnalysisPerformanceMetrics              Key = "BOM.metadata.component.modelCard.quantitativeAnalysis.performanceMetrics"
	ModelCardConsiderationsEnvironmentalConsiderationsProperties Key = "BOM.metadata.component.modelCard.considerations.environmentalConsiderations.properties"

	// Security scan summary stored as Component.Properties.
	ComponentPropertiesSecurityOverallStatus Key = "BOM.metadata.component.properties.huggingface:security:overallStatus"
	ComponentPropertiesSecurityScannedFiles  Key = "BOM.metadata.component.properties.huggingface:security:scannedFileCount"
	ComponentPropertiesSecurityUnsafeFiles   Key = "BOM.metadata.component.properties.huggingface:security:unsafeFileCount"
	ComponentPropertiesSecurityCautionFiles  Key = "BOM.metadata.component.properties.huggingface:security:cautionFileCount"
)

// DatasetKey identifies dataset-specific CycloneDX fields.
type DatasetKey string

func (k DatasetKey) String() string { return string(k) }

const (
	// BOM.components[DATA].* (DATASET).
	DatasetName               DatasetKey = "BOM.components[DATA].name"
	DatasetExternalReferences DatasetKey = "BOM.components[DATA].externalReferences"
	DatasetTags               DatasetKey = "BOM.components[DATA].tags"
	DatasetLicenses           DatasetKey = "BOM.components[DATA].licenses"
	DatasetDescription        DatasetKey = "BOM.components[DATA].data.description"
	DatasetManufacturer       DatasetKey = "BOM.components[DATA].manufacturer"
	DatasetAuthors            DatasetKey = "BOM.components[DATA].authors"
	DatasetGroup              DatasetKey = "BOM.components[DATA].group"
	DatasetContents           DatasetKey = "BOM.components[DATA].data.contents.attachments"
	DatasetSensitiveData      DatasetKey = "BOM.components[DATA].data.sensitiveData"
	DatasetClassification     DatasetKey = "BOM.components[DATA].data.classification"
	DatasetGovernance         DatasetKey = "BOM.components[DATA].data.governance"
	DatasetHashes             DatasetKey = "BOM.components[DATA].hashes"
	DatasetContact            DatasetKey = "BOM.components[DATA].properties.huggingface:datasetContact"
	DatasetCreatedAt          DatasetKey = "BOM.components[DATA].properties.huggingface:createdAt"
	DatasetUsedStorage        DatasetKey = "BOM.components[DATA].properties.huggingface:usedStorage"
	DatasetLastModified       DatasetKey = "BOM.components[DATA].tags.lastModified"
)

// Source is everything FieldSpecs can read from.
type Source struct {
	ModelID      string
	Scan         scanner.Discovery
	HF           *fetcher.ModelAPIResponse
	Readme       *fetcher.ModelReadmeCard
	SecurityTree []fetcher.SecurityFileEntry
}

// Target is everything FieldSpecs are allowed to mutate.
type Target struct {
	BOM       *cdx.BOM
	Component *cdx.Component
	ModelCard *cdx.MLModelCard

	// Options (builder can set these when calling Apply).
	IncludeEvidenceProperties bool
	HuggingFaceBaseURL        string
}

// DatasetSource mirrors Source but for datasets.
type DatasetSource struct {
	DatasetID string
	Scan      scanner.Discovery
	HF        *fetcher.DatasetAPIResponse
	Readme    *fetcher.DatasetReadmeCard
}

// DatasetTarget is the dataset component being built.
type DatasetTarget struct {
	Component *cdx.Component

	// Options.
	IncludeEvidenceProperties bool
	HuggingFaceBaseURL        string
}

// InputType defines the type of input field for interactive enrichment.
type InputType string

const (
	InputTypeText      InputType = "text"      // Single-line text input
	InputTypeTextArea  InputType = "textarea"  // Multi-line text input
	InputTypeSelect    InputType = "select"    // Dropdown selection
	InputTypeMultiText InputType = "multitext" // Comma-separated values
)

// FieldSpec is a first-class definition of a field:.
// - how it contributes to completeness.
// - how it is populated into the BOM.
// - how its presence is detected.
// - how user-provided values are set.
// - how it should be presented in interactive forms.
type FieldSpec struct {
	Key      Key
	Weight   float64
	Required bool

	Sources []func(Source) (any, bool)
	Parse   func(string) (any, error)
	Apply   func(Target, any) error
	Present func(*cdx.BOM) bool

	// UI metadata for interactive enrichment.
	InputType   InputType
	Placeholder string
	Suggestions []string
}

// DatasetFieldSpec is the dataset analog of FieldSpec.
type DatasetFieldSpec struct {
	Key      DatasetKey
	Weight   float64
	Required bool

	Sources []func(DatasetSource) (any, bool)
	Parse   func(string) (any, error)
	Apply   func(DatasetTarget, any) error
	Present func(comp *cdx.Component) bool

	// UI metadata for interactive enrichment.
	InputType   InputType
	Placeholder string
	Suggestions []string
}

// Registry is the central registry of all known FieldSpecs.
// Each spec defines how to apply itself and how to check presence.
// The registry is used by the BOM builder and completeness checker.
// It is the single source of truth for what fields we care about.
func Registry() []FieldSpec {
	specs := make([]FieldSpec, 0, 32)
	specs = append(specs, componentFields()...)
	specs = append(specs, evidenceFields()...)
	specs = append(specs, hfPropFields()...)
	specs = append(specs, modelCardFields()...)
	specs = append(specs, securityFields()...)
	return specs
}
