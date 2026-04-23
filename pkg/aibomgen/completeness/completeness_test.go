package completeness

import (
	"math"
	"reflect"
	"testing"

	"github.com/idlab-discover/aibomgen-cli/internal/metadata"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

// Test Strategy:.
// - Uses calculated score values (e.g., 1.0 / 12.15) instead of hardcoded floats to avoid precision issues.
// - Implements tolerance-based comparison (1e-9) for floating point scores.
// - Helper functions resultsEqual() and datasetResultsEqual() compare results with proper float handling.
// - Best practice: never hardcode floating point literals in test expectations.

// Constants from metadata registry (total weight: 12.15 for model, 9.4 for dataset).
const (
	totalModelFields   = 30
	totalDatasetFields = 17
	floatTolerance     = 1e-9 // Tolerance for floating point comparison
)

// Helper function to compare two Result structs with floating point tolerance.
func resultsEqual(got, want Result) bool {
	// Compare non-float fields.
	if got.ModelID != want.ModelID ||
		got.Passed != want.Passed ||
		got.Total != want.Total ||
		!reflect.DeepEqual(got.MissingRequired, want.MissingRequired) ||
		!reflect.DeepEqual(got.MissingOptional, want.MissingOptional) {
		return false
	}

	// Compare Score with tolerance.
	if math.Abs(got.Score-want.Score) > floatTolerance {
		return false
	}

	// Compare DatasetResults.
	if len(got.DatasetResults) != len(want.DatasetResults) {
		return false
	}

	for key, wantDS := range want.DatasetResults {
		gotDS, ok := got.DatasetResults[key]
		if !ok {
			return false
		}
		if !datasetResultsEqual(gotDS, wantDS) {
			return false
		}
	}

	return true
}

// Helper function to compare two DatasetResult structs with floating point tolerance.
func datasetResultsEqual(got, want DatasetResult) bool {
	if got.DatasetRef != want.DatasetRef ||
		got.Passed != want.Passed ||
		got.Total != want.Total ||
		!reflect.DeepEqual(got.MissingRequired, want.MissingRequired) ||
		!reflect.DeepEqual(got.MissingOptional, want.MissingOptional) {
		return false
	}

	// Compare Score with tolerance.
	return math.Abs(got.Score-want.Score) <= floatTolerance
}

func TestCheck(t *testing.T) {
	type args struct {
		bom *cdx.BOM
	}
	tests := []struct {
		name string
		args args
		want Result
	}{
		{
			name: "empty BOM",
			args: args{
				bom: &cdx.BOM{},
			},
			want: Result{
				ModelID:         "(unknown)",
				Score:           0.0,
				Passed:          0,
				Total:           totalModelFields,
				MissingRequired: []metadata.Key{metadata.ComponentName},
				MissingOptional: []metadata.Key{
					metadata.ComponentExternalReferences,
					metadata.ComponentTags,
					metadata.ComponentLicenses,
					metadata.ComponentHashes,
					metadata.ComponentManufacturer,
					metadata.ComponentGroup,
					metadata.ComponentPropertiesHuggingFaceLastModified,
					metadata.ComponentPropertiesHuggingFaceCreatedAt,
					metadata.ComponentPropertiesHuggingFaceLanguage,
					metadata.ComponentPropertiesHuggingFaceUsedStorage,
					metadata.ComponentPropertiesHuggingFacePrivate,
					metadata.ComponentPropertiesHuggingFaceLibraryName,
					metadata.ComponentPropertiesHuggingFaceDownloads,
					metadata.ComponentPropertiesHuggingFaceLikes,
					metadata.ComponentPropertiesHuggingFaceBaseModel,
					metadata.ComponentPropertiesHuggingFaceContact,
					metadata.ModelCardModelParametersTask,
					metadata.ModelCardModelParametersArchitectureFamily,
					metadata.ModelCardModelParametersModelArchitecture,
					metadata.ModelCardModelParametersDatasets,
					metadata.ModelCardConsiderationsUseCases,
					metadata.ModelCardConsiderationsTechnicalLimitations,
					metadata.ModelCardConsiderationsEthicalConsiderations,
					metadata.ModelCardQuantitativeAnalysisPerformanceMetrics,
					metadata.ModelCardConsiderationsEnvironmentalConsiderationsProperties,
					metadata.ComponentPropertiesSecurityOverallStatus,
					metadata.ComponentPropertiesSecurityScannedFiles,
					metadata.ComponentPropertiesSecurityUnsafeFiles,
					metadata.ComponentPropertiesSecurityCautionFiles,
				},
				DatasetResults: make(map[string]DatasetResult),
			},
		},
		{
			name: "BOM with component name only",
			args: args{
				bom: &cdx.BOM{
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name: "test-model",
						},
					},
				},
			},
			want: Result{
				ModelID:         "test-model",
				Score:           1.0 / 12.15, // ComponentName weight (1.0) / total weight (12.15)
				Passed:          1,
				Total:           totalModelFields,
				MissingRequired: nil, // ComponentName is satisfied
				MissingOptional: []metadata.Key{
					metadata.ComponentExternalReferences,
					metadata.ComponentTags,
					metadata.ComponentLicenses,
					metadata.ComponentHashes,
					metadata.ComponentManufacturer,
					metadata.ComponentGroup,
					metadata.ComponentPropertiesHuggingFaceLastModified,
					metadata.ComponentPropertiesHuggingFaceCreatedAt,
					metadata.ComponentPropertiesHuggingFaceLanguage,
					metadata.ComponentPropertiesHuggingFaceUsedStorage,
					metadata.ComponentPropertiesHuggingFacePrivate,
					metadata.ComponentPropertiesHuggingFaceLibraryName,
					metadata.ComponentPropertiesHuggingFaceDownloads,
					metadata.ComponentPropertiesHuggingFaceLikes,
					metadata.ComponentPropertiesHuggingFaceBaseModel,
					metadata.ComponentPropertiesHuggingFaceContact,
					metadata.ModelCardModelParametersTask,
					metadata.ModelCardModelParametersArchitectureFamily,
					metadata.ModelCardModelParametersModelArchitecture,
					metadata.ModelCardModelParametersDatasets,
					metadata.ModelCardConsiderationsUseCases,
					metadata.ModelCardConsiderationsTechnicalLimitations,
					metadata.ModelCardConsiderationsEthicalConsiderations,
					metadata.ModelCardQuantitativeAnalysisPerformanceMetrics,
					metadata.ModelCardConsiderationsEnvironmentalConsiderationsProperties,
					metadata.ComponentPropertiesSecurityOverallStatus,
					metadata.ComponentPropertiesSecurityScannedFiles,
					metadata.ComponentPropertiesSecurityUnsafeFiles,
					metadata.ComponentPropertiesSecurityCautionFiles,
				},
				DatasetResults: make(map[string]DatasetResult),
			},
		},
		{
			name: "BOM with datasets referenced but no dataset components",
			args: args{
				bom: &cdx.BOM{
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name: "test-model",
							ModelCard: &cdx.MLModelCard{
								ModelParameters: &cdx.MLModelParameters{
									Datasets: &[]cdx.MLDatasetChoice{
										{Ref: "ref:dataset-1"},
									},
								},
							},
						},
					},
				},
			},
			want: Result{
				ModelID:         "test-model",
				Score:           1.5 / 12.15, // ComponentName (1.0) + Datasets (0.5) / total (12.15)
				Passed:          2,
				Total:           totalModelFields,
				MissingRequired: nil,
				MissingOptional: []metadata.Key{
					metadata.ComponentExternalReferences,
					metadata.ComponentTags,
					metadata.ComponentLicenses,
					metadata.ComponentHashes,
					metadata.ComponentManufacturer,
					metadata.ComponentGroup,
					metadata.ComponentPropertiesHuggingFaceLastModified,
					metadata.ComponentPropertiesHuggingFaceCreatedAt,
					metadata.ComponentPropertiesHuggingFaceLanguage,
					metadata.ComponentPropertiesHuggingFaceUsedStorage,
					metadata.ComponentPropertiesHuggingFacePrivate,
					metadata.ComponentPropertiesHuggingFaceLibraryName,
					metadata.ComponentPropertiesHuggingFaceDownloads,
					metadata.ComponentPropertiesHuggingFaceLikes,
					metadata.ComponentPropertiesHuggingFaceBaseModel,
					metadata.ComponentPropertiesHuggingFaceContact,
					metadata.ModelCardModelParametersTask,
					metadata.ModelCardModelParametersArchitectureFamily,
					metadata.ModelCardModelParametersModelArchitecture,
					// Datasets is now present, so it's not in missing list.
					metadata.ModelCardConsiderationsUseCases,
					metadata.ModelCardConsiderationsTechnicalLimitations,
					metadata.ModelCardConsiderationsEthicalConsiderations,
					metadata.ModelCardQuantitativeAnalysisPerformanceMetrics,
					metadata.ModelCardConsiderationsEnvironmentalConsiderationsProperties,
					metadata.ComponentPropertiesSecurityOverallStatus,
					metadata.ComponentPropertiesSecurityScannedFiles,
					metadata.ComponentPropertiesSecurityUnsafeFiles,
					metadata.ComponentPropertiesSecurityCautionFiles,
				},
				DatasetResults: make(map[string]DatasetResult),
			},
		},
		{
			name: "BOM with datasets referenced and dataset components",
			args: args{
				bom: &cdx.BOM{
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name: "test-model",
							ModelCard: &cdx.MLModelCard{
								ModelParameters: &cdx.MLModelParameters{
									Datasets: &[]cdx.MLDatasetChoice{
										{Ref: "ref:dataset-1"},
									},
								},
							},
						},
					},
					Components: &[]cdx.Component{
						{
							Type: cdx.ComponentTypeData,
							Name: "dataset-1",
						},
					},
				},
			},
			want: Result{
				ModelID:         "test-model",
				Score:           1.5 / 12.15,
				Passed:          2,
				Total:           totalModelFields,
				MissingRequired: nil,
				MissingOptional: []metadata.Key{
					metadata.ComponentExternalReferences,
					metadata.ComponentTags,
					metadata.ComponentLicenses,
					metadata.ComponentHashes,
					metadata.ComponentManufacturer,
					metadata.ComponentGroup,
					metadata.ComponentPropertiesHuggingFaceLastModified,
					metadata.ComponentPropertiesHuggingFaceCreatedAt,
					metadata.ComponentPropertiesHuggingFaceLanguage,
					metadata.ComponentPropertiesHuggingFaceUsedStorage,
					metadata.ComponentPropertiesHuggingFacePrivate,
					metadata.ComponentPropertiesHuggingFaceLibraryName,
					metadata.ComponentPropertiesHuggingFaceDownloads,
					metadata.ComponentPropertiesHuggingFaceLikes,
					metadata.ComponentPropertiesHuggingFaceBaseModel,
					metadata.ComponentPropertiesHuggingFaceContact,
					metadata.ModelCardModelParametersTask,
					metadata.ModelCardModelParametersArchitectureFamily,
					metadata.ModelCardModelParametersModelArchitecture,
					metadata.ModelCardConsiderationsUseCases,
					metadata.ModelCardConsiderationsTechnicalLimitations,
					metadata.ModelCardConsiderationsEthicalConsiderations,
					metadata.ModelCardQuantitativeAnalysisPerformanceMetrics,
					metadata.ModelCardConsiderationsEnvironmentalConsiderationsProperties,
					metadata.ComponentPropertiesSecurityOverallStatus,
					metadata.ComponentPropertiesSecurityScannedFiles,
					metadata.ComponentPropertiesSecurityUnsafeFiles,
					metadata.ComponentPropertiesSecurityCautionFiles,
				},
				DatasetResults: map[string]DatasetResult{
					"dataset-1": {
						DatasetRef:      "dataset-1",
						Score:           1.0 / 9.4, // DatasetName weight (1.0) / total dataset weight (9.4)
						Passed:          1,
						Total:           totalDatasetFields,
						MissingRequired: nil, // DatasetName is satisfied
						MissingOptional: []metadata.DatasetKey{
							metadata.DatasetExternalReferences,
							metadata.DatasetTags,
							metadata.DatasetLicenses,
							metadata.DatasetDescription,
							metadata.DatasetManufacturer,
							metadata.DatasetAuthors,
							metadata.DatasetGroup,
							metadata.DatasetContents,
							metadata.DatasetSensitiveData,
							metadata.DatasetClassification,
							metadata.DatasetGovernance,
							metadata.DatasetHashes,
							metadata.DatasetCreatedAt,
							metadata.DatasetUsedStorage,
							metadata.DatasetLastModified,
							metadata.DatasetContact,
						},
					},
				},
			},
		},
		{
			name: "BOM with no datasets referenced",
			args: args{
				bom: &cdx.BOM{
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name: "test-model",
							ModelCard: &cdx.MLModelCard{
								ModelParameters: &cdx.MLModelParameters{
									Datasets: &[]cdx.MLDatasetChoice{},
								},
							},
						},
					},
				},
			},
			want: Result{
				ModelID:         "test-model",
				Score:           1.0 / 12.15, // Only ComponentName is present
				Passed:          1,
				Total:           totalModelFields,
				MissingRequired: nil,
				MissingOptional: []metadata.Key{
					metadata.ComponentExternalReferences,
					metadata.ComponentTags,
					metadata.ComponentLicenses,
					metadata.ComponentHashes,
					metadata.ComponentManufacturer,
					metadata.ComponentGroup,
					metadata.ComponentPropertiesHuggingFaceLastModified,
					metadata.ComponentPropertiesHuggingFaceCreatedAt,
					metadata.ComponentPropertiesHuggingFaceLanguage,
					metadata.ComponentPropertiesHuggingFaceUsedStorage,
					metadata.ComponentPropertiesHuggingFacePrivate,
					metadata.ComponentPropertiesHuggingFaceLibraryName,
					metadata.ComponentPropertiesHuggingFaceDownloads,
					metadata.ComponentPropertiesHuggingFaceLikes,
					metadata.ComponentPropertiesHuggingFaceBaseModel,
					metadata.ComponentPropertiesHuggingFaceContact,
					metadata.ModelCardModelParametersTask,
					metadata.ModelCardModelParametersArchitectureFamily,
					metadata.ModelCardModelParametersModelArchitecture,
					metadata.ModelCardModelParametersDatasets, // Counted as missing when no datasets referenced
					metadata.ModelCardConsiderationsUseCases,
					metadata.ModelCardConsiderationsTechnicalLimitations,
					metadata.ModelCardConsiderationsEthicalConsiderations,
					metadata.ModelCardQuantitativeAnalysisPerformanceMetrics,
					metadata.ModelCardConsiderationsEnvironmentalConsiderationsProperties,
					metadata.ComponentPropertiesSecurityOverallStatus,
					metadata.ComponentPropertiesSecurityScannedFiles,
					metadata.ComponentPropertiesSecurityUnsafeFiles,
					metadata.ComponentPropertiesSecurityCautionFiles,
				},
				DatasetResults: make(map[string]DatasetResult),
			},
		},
		{
			name: "BOM with empty dataset refs",
			args: args{
				bom: &cdx.BOM{
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name: "test-model",
							ModelCard: &cdx.MLModelCard{
								ModelParameters: &cdx.MLModelParameters{
									Datasets: &[]cdx.MLDatasetChoice{
										{Ref: ""},
									},
								},
							},
						},
					},
				},
			},
			want: Result{
				ModelID:         "test-model",
				Score:           1.0 / 12.15,
				Passed:          1,
				Total:           totalModelFields,
				MissingRequired: nil,
				MissingOptional: []metadata.Key{
					metadata.ComponentExternalReferences,
					metadata.ComponentTags,
					metadata.ComponentLicenses,
					metadata.ComponentHashes,
					metadata.ComponentManufacturer,
					metadata.ComponentGroup,
					metadata.ComponentPropertiesHuggingFaceLastModified,
					metadata.ComponentPropertiesHuggingFaceCreatedAt,
					metadata.ComponentPropertiesHuggingFaceLanguage,
					metadata.ComponentPropertiesHuggingFaceUsedStorage,
					metadata.ComponentPropertiesHuggingFacePrivate,
					metadata.ComponentPropertiesHuggingFaceLibraryName,
					metadata.ComponentPropertiesHuggingFaceDownloads,
					metadata.ComponentPropertiesHuggingFaceLikes,
					metadata.ComponentPropertiesHuggingFaceBaseModel,
					metadata.ComponentPropertiesHuggingFaceContact,
					metadata.ModelCardModelParametersTask,
					metadata.ModelCardModelParametersArchitectureFamily,
					metadata.ModelCardModelParametersModelArchitecture,
					metadata.ModelCardModelParametersDatasets,
					metadata.ModelCardConsiderationsUseCases,
					metadata.ModelCardConsiderationsTechnicalLimitations,
					metadata.ModelCardConsiderationsEthicalConsiderations,
					metadata.ModelCardQuantitativeAnalysisPerformanceMetrics,
					metadata.ModelCardConsiderationsEnvironmentalConsiderationsProperties,
					metadata.ComponentPropertiesSecurityOverallStatus,
					metadata.ComponentPropertiesSecurityScannedFiles,
					metadata.ComponentPropertiesSecurityUnsafeFiles,
					metadata.ComponentPropertiesSecurityCautionFiles,
				},
				DatasetResults: make(map[string]DatasetResult),
			},
		},
		{
			name: "BOM with multiple dataset components",
			args: args{
				bom: &cdx.BOM{
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name: "test-model",
							ModelCard: &cdx.MLModelCard{
								ModelParameters: &cdx.MLModelParameters{
									Datasets: &[]cdx.MLDatasetChoice{
										{Ref: "ref:dataset-1"},
										{Ref: "ref:dataset-2"},
									},
								},
							},
						},
					},
					Components: &[]cdx.Component{
						{
							Type: cdx.ComponentTypeData,
							Name: "dataset-1",
						},
						{
							Type: cdx.ComponentTypeData,
							Name: "dataset-2",
							Data: &[]cdx.ComponentData{
								{Description: "test description"},
							},
						},
						{
							Type: cdx.ComponentTypeLibrary,
							Name: "some-library",
						},
					},
				},
			},
			want: Result{
				ModelID:         "test-model",
				Score:           1.5 / 12.15,
				Passed:          2,
				Total:           totalModelFields,
				MissingRequired: nil,
				MissingOptional: []metadata.Key{
					metadata.ComponentExternalReferences,
					metadata.ComponentTags,
					metadata.ComponentLicenses,
					metadata.ComponentHashes,
					metadata.ComponentManufacturer,
					metadata.ComponentGroup,
					metadata.ComponentPropertiesHuggingFaceLastModified,
					metadata.ComponentPropertiesHuggingFaceCreatedAt,
					metadata.ComponentPropertiesHuggingFaceLanguage,
					metadata.ComponentPropertiesHuggingFaceUsedStorage,
					metadata.ComponentPropertiesHuggingFacePrivate,
					metadata.ComponentPropertiesHuggingFaceLibraryName,
					metadata.ComponentPropertiesHuggingFaceDownloads,
					metadata.ComponentPropertiesHuggingFaceLikes,
					metadata.ComponentPropertiesHuggingFaceBaseModel,
					metadata.ComponentPropertiesHuggingFaceContact,
					metadata.ModelCardModelParametersTask,
					metadata.ModelCardModelParametersArchitectureFamily,
					metadata.ModelCardModelParametersModelArchitecture,
					metadata.ModelCardConsiderationsUseCases,
					metadata.ModelCardConsiderationsTechnicalLimitations,
					metadata.ModelCardConsiderationsEthicalConsiderations,
					metadata.ModelCardQuantitativeAnalysisPerformanceMetrics,
					metadata.ModelCardConsiderationsEnvironmentalConsiderationsProperties,
					metadata.ComponentPropertiesSecurityOverallStatus,
					metadata.ComponentPropertiesSecurityScannedFiles,
					metadata.ComponentPropertiesSecurityUnsafeFiles,
					metadata.ComponentPropertiesSecurityCautionFiles,
				},
				DatasetResults: map[string]DatasetResult{
					"dataset-1": {
						DatasetRef:      "dataset-1",
						Score:           1.0 / 9.4,
						Passed:          1,
						Total:           totalDatasetFields,
						MissingRequired: nil,
						MissingOptional: []metadata.DatasetKey{
							metadata.DatasetExternalReferences,
							metadata.DatasetTags,
							metadata.DatasetLicenses,
							metadata.DatasetDescription,
							metadata.DatasetManufacturer,
							metadata.DatasetAuthors,
							metadata.DatasetGroup,
							metadata.DatasetContents,
							metadata.DatasetSensitiveData,
							metadata.DatasetClassification,
							metadata.DatasetGovernance,
							metadata.DatasetHashes,
							metadata.DatasetCreatedAt,
							metadata.DatasetUsedStorage,
							metadata.DatasetLastModified,
							metadata.DatasetContact,
						},
					},
					"dataset-2": {
						DatasetRef:      "dataset-2",
						Score:           1.7 / 9.4, // DatasetName (1.0) + DatasetDescription (0.7)
						Passed:          2,
						Total:           totalDatasetFields,
						MissingRequired: nil,
						MissingOptional: []metadata.DatasetKey{
							metadata.DatasetExternalReferences,
							metadata.DatasetTags,
							metadata.DatasetLicenses,
							// DatasetDescription is present, so excluded.
							metadata.DatasetManufacturer,
							metadata.DatasetAuthors,
							metadata.DatasetGroup,
							metadata.DatasetContents,
							metadata.DatasetSensitiveData,
							metadata.DatasetClassification,
							metadata.DatasetGovernance,
							metadata.DatasetHashes,
							metadata.DatasetCreatedAt,
							metadata.DatasetUsedStorage,
							metadata.DatasetLastModified,
							metadata.DatasetContact,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Check(tt.args.bom)
			if !resultsEqual(got, tt.want) {
				t.Errorf("Check() =\n%+v\nwant =\n%+v", got, tt.want)
			}
		})
	}
}

func Test_hasDatasetsReferenced(t *testing.T) {
	type args struct {
		bom *cdx.BOM
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "nil BOM",
			args: args{
				bom: nil,
			},
			want: false,
		},
		{
			name: "BOM without metadata",
			args: args{
				bom: &cdx.BOM{},
			},
			want: false,
		},
		{
			name: "BOM without component",
			args: args{
				bom: &cdx.BOM{
					Metadata: &cdx.Metadata{},
				},
			},
			want: false,
		},
		{
			name: "BOM without ModelCard",
			args: args{
				bom: &cdx.BOM{
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name: "test",
						},
					},
				},
			},
			want: false,
		},
		{
			name: "BOM without ModelParameters",
			args: args{
				bom: &cdx.BOM{
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name:      "test",
							ModelCard: &cdx.MLModelCard{},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "BOM with nil Datasets",
			args: args{
				bom: &cdx.BOM{
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name: "test",
							ModelCard: &cdx.MLModelCard{
								ModelParameters: &cdx.MLModelParameters{
									Datasets: nil,
								},
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "BOM with empty Datasets",
			args: args{
				bom: &cdx.BOM{
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name: "test",
							ModelCard: &cdx.MLModelCard{
								ModelParameters: &cdx.MLModelParameters{
									Datasets: &[]cdx.MLDatasetChoice{},
								},
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "BOM with datasets but empty refs",
			args: args{
				bom: &cdx.BOM{
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name: "test",
							ModelCard: &cdx.MLModelCard{
								ModelParameters: &cdx.MLModelParameters{
									Datasets: &[]cdx.MLDatasetChoice{
										{Ref: ""},
									},
								},
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "BOM with datasets with non-empty ref",
			args: args{
				bom: &cdx.BOM{
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name: "test",
							ModelCard: &cdx.MLModelCard{
								ModelParameters: &cdx.MLModelParameters{
									Datasets: &[]cdx.MLDatasetChoice{
										{Ref: "ref:dataset-1"},
									},
								},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "BOM with multiple datasets, first is empty",
			args: args{
				bom: &cdx.BOM{
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name: "test",
							ModelCard: &cdx.MLModelCard{
								ModelParameters: &cdx.MLModelParameters{
									Datasets: &[]cdx.MLDatasetChoice{
										{Ref: ""},
										{Ref: "ref:dataset-2"},
									},
								},
							},
						},
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasDatasetsReferenced(tt.args.bom); got != tt.want {
				t.Errorf("hasDatasetsReferenced() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckDataset(t *testing.T) {
	type args struct {
		comp *cdx.Component
	}
	tests := []struct {
		name string
		args args
		want DatasetResult
	}{
		{
			name: "empty component",
			args: args{
				comp: &cdx.Component{},
			},
			want: DatasetResult{
				DatasetRef:      "",
				Score:           0.0,
				Passed:          0,
				Total:           totalDatasetFields,
				MissingRequired: []metadata.DatasetKey{metadata.DatasetName},
				MissingOptional: []metadata.DatasetKey{
					metadata.DatasetExternalReferences,
					metadata.DatasetTags,
					metadata.DatasetLicenses,
					metadata.DatasetDescription,
					metadata.DatasetManufacturer,
					metadata.DatasetAuthors,
					metadata.DatasetGroup,
					metadata.DatasetContents,
					metadata.DatasetSensitiveData,
					metadata.DatasetClassification,
					metadata.DatasetGovernance,
					metadata.DatasetHashes,
					metadata.DatasetCreatedAt,
					metadata.DatasetUsedStorage,
					metadata.DatasetLastModified,
					metadata.DatasetContact,
				},
			},
		},
		{
			name: "component with name only",
			args: args{
				comp: &cdx.Component{
					Name: "test-dataset",
				},
			},
			want: DatasetResult{
				DatasetRef:      "test-dataset",
				Score:           1.0 / 9.4, // DatasetName weight (1.0) / total weight (9.4)
				Passed:          1,
				Total:           totalDatasetFields,
				MissingRequired: nil, // DatasetName is satisfied
				MissingOptional: []metadata.DatasetKey{
					metadata.DatasetExternalReferences,
					metadata.DatasetTags,
					metadata.DatasetLicenses,
					metadata.DatasetDescription,
					metadata.DatasetManufacturer,
					metadata.DatasetAuthors,
					metadata.DatasetGroup,
					metadata.DatasetContents,
					metadata.DatasetSensitiveData,
					metadata.DatasetClassification,
					metadata.DatasetGovernance,
					metadata.DatasetHashes,
					metadata.DatasetCreatedAt,
					metadata.DatasetUsedStorage,
					metadata.DatasetLastModified,
					metadata.DatasetContact,
				},
			},
		},
		{
			name: "component with name and description",
			args: args{
				comp: &cdx.Component{
					Name: "test-dataset",
					Data: &[]cdx.ComponentData{
						{Description: "test description"},
					},
				},
			},
			want: DatasetResult{
				DatasetRef:      "test-dataset",
				Score:           1.7 / 9.4, // DatasetName (1.0) + DatasetDescription (0.7) / total (9.4)
				Passed:          2,
				Total:           totalDatasetFields,
				MissingRequired: nil,
				MissingOptional: []metadata.DatasetKey{
					metadata.DatasetExternalReferences,
					metadata.DatasetTags,
					metadata.DatasetLicenses,
					// DatasetDescription is present, so excluded.
					metadata.DatasetManufacturer,
					metadata.DatasetAuthors,
					metadata.DatasetGroup,
					metadata.DatasetContents,
					metadata.DatasetSensitiveData,
					metadata.DatasetClassification,
					metadata.DatasetGovernance,
					metadata.DatasetHashes,
					metadata.DatasetCreatedAt,
					metadata.DatasetUsedStorage,
					metadata.DatasetLastModified,
					metadata.DatasetContact,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckDataset(tt.args.comp)
			if !datasetResultsEqual(got, tt.want) {
				t.Errorf("CheckDataset() =\n%+v\nwant =\n%+v", got, tt.want)
			}
		})
	}
}

// Test edge cases using custom registries for 100% coverage.

func Test_checkWithRegistry_RequiredDatasetField(t *testing.T) {
	// Test when ModelCardModelParametersDatasets is required (covers line 56).
	customRegistry := []metadata.FieldSpec{
		{
			Key:      metadata.ComponentName,
			Weight:   1.0,
			Required: true,
			Present: func(bom *cdx.BOM) bool {
				return bom != nil && bom.Metadata != nil && bom.Metadata.Component != nil && bom.Metadata.Component.Name != ""
			},
		},
		{
			Key:      metadata.ModelCardModelParametersDatasets,
			Weight:   0.5,
			Required: true, // Make it required to cover line 56
			Present: func(bom *cdx.BOM) bool {
				return bom != nil && bom.Metadata != nil && bom.Metadata.Component != nil &&
					bom.Metadata.Component.ModelCard != nil && bom.Metadata.Component.ModelCard.ModelParameters != nil &&
					bom.Metadata.Component.ModelCard.ModelParameters.Datasets != nil &&
					len(*bom.Metadata.Component.ModelCard.ModelParameters.Datasets) > 0 &&
					(*bom.Metadata.Component.ModelCard.ModelParameters.Datasets)[0].Ref != ""
			},
		},
	}

	bom := &cdx.BOM{
		Metadata: &cdx.Metadata{
			Component: &cdx.Component{
				Name: "test-model",
				// No datasets, so the required field is missing.
			},
		},
	}

	got := checkWithRegistry(bom, customRegistry, []metadata.DatasetFieldSpec{})

	if got.ModelID != "test-model" {
		t.Errorf("ModelID = %v, want test-model", got.ModelID)
	}
	if got.Passed != 1 {
		t.Errorf("Passed = %v, want 1", got.Passed)
	}
	if got.Total != 2 {
		t.Errorf("Total = %v, want 2", got.Total)
	}
	if len(got.MissingRequired) != 1 || got.MissingRequired[0] != metadata.ModelCardModelParametersDatasets {
		t.Errorf("MissingRequired = %v, want [ModelCardModelParametersDatasets]", got.MissingRequired)
	}
}

func Test_checkDatasetWithRegistry_ZeroWeight(t *testing.T) {
	// Test field with Weight <= 0 (covers line 152).
	customRegistry := []metadata.DatasetFieldSpec{
		{
			Key:      metadata.DatasetName,
			Weight:   1.0,
			Required: true,
			Present: func(comp *cdx.Component) bool {
				return comp != nil && comp.Name != ""
			},
		},
		{
			Key:      metadata.DatasetDescription,
			Weight:   0.0, // Zero weight should be skipped
			Required: false,
			Present: func(comp *cdx.Component) bool {
				return comp != nil && comp.Data != nil && len(*comp.Data) > 0 && (*comp.Data)[0].Description != ""
			},
		},
		{
			Key:      metadata.DatasetTags,
			Weight:   -0.5, // Negative weight should also be skipped
			Required: false,
			Present: func(comp *cdx.Component) bool {
				return comp != nil && comp.Tags != nil && len(*comp.Tags) > 0
			},
		},
	}

	comp := &cdx.Component{
		Name: "test-dataset",
		Data: &[]cdx.ComponentData{
			{Description: "test description"},
		},
	}

	got := checkDatasetWithRegistry(comp, customRegistry)

	// Only DatasetName should be counted (weight > 0).
	if got.Total != 1 {
		t.Errorf("Total = %v, want 1 (fields with zero/negative weight should be skipped)", got.Total)
	}
	if got.Passed != 1 {
		t.Errorf("Passed = %v, want 1", got.Passed)
	}
	if math.Abs(got.Score-1.0) > floatTolerance {
		t.Errorf("Score = %v, want 1.0", got.Score)
	}
}

func Test_checkWithRegistry_ZeroWeight(t *testing.T) {
	// Test model field with Weight <= 0.
	customRegistry := []metadata.FieldSpec{
		{
			Key:      metadata.ComponentName,
			Weight:   1.0,
			Required: true,
			Present: func(bom *cdx.BOM) bool {
				return bom != nil && bom.Metadata != nil && bom.Metadata.Component != nil && bom.Metadata.Component.Name != ""
			},
		},
		{
			Key:      metadata.ComponentGroup,
			Weight:   0.0, // Zero weight should be skipped
			Required: false,
			Present: func(bom *cdx.BOM) bool {
				return bom != nil && bom.Metadata != nil && bom.Metadata.Component != nil && bom.Metadata.Component.Group != ""
			},
		},
	}

	bom := &cdx.BOM{
		Metadata: &cdx.Metadata{
			Component: &cdx.Component{
				Name:  "test-model",
				Group: "test-group",
			},
		},
	}

	got := checkWithRegistry(bom, customRegistry, []metadata.DatasetFieldSpec{})

	// Only ComponentName should be counted (weight > 0).
	if got.Total != 1 {
		t.Errorf("Total = %v, want 1 (fields with zero weight should be skipped)", got.Total)
	}
	if got.Passed != 1 {
		t.Errorf("Passed = %v, want 1", got.Passed)
	}
	if math.Abs(got.Score-1.0) > floatTolerance {
		t.Errorf("Score = %v, want 1.0", got.Score)
	}
}
