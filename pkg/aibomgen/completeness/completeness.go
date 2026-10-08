package completeness

import (
	"github.com/idlab-discover/aibomgen-cli/internal/metadata"
	"log/slog"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

// Result holds the completeness score for the model component of a BOM and.
// all linked dataset components.
type Result struct {
	ModelID string  `json:"modelId"` // Model identifier/name
	Score   float64 `json:"score"`   // 0..1

	Passed int `json:"passed"`
	Total  int `json:"total"`

	MissingRequired []metadata.Key `json:"missingRequired"`
	MissingOptional []metadata.Key `json:"missingOptional"`

	// Dataset-specific tracking.
	DatasetResults map[string]DatasetResult `json:"datasets,omitempty"` // key is dataset name/ref
}

// DatasetResult holds the completeness score for a single dataset component.
type DatasetResult struct {
	DatasetRef string `json:"datasetRef"` // Reference to the dataset

	Score  float64 `json:"score"` // 0..1
	Passed int     `json:"passed"`
	Total  int     `json:"total"`

	MissingRequired []metadata.DatasetKey `json:"missingRequired"`
	MissingOptional []metadata.DatasetKey `json:"missingOptional"`
}

// Check checks the completeness of a BOM using the default metadata registry.
func Check(bom *cdx.BOM) Result {
	return checkWithRegistry(bom, metadata.Registry(), metadata.DatasetRegistry())
}

// checkWithRegistry allows injecting custom registries for testing.
func checkWithRegistry(bom *cdx.BOM, modelRegistry []metadata.FieldSpec, datasetRegistry []metadata.DatasetFieldSpec) Result {
	var (
		earned, max float64
		passed      int
		total       int
		missingReq  []metadata.Key
		missingOpt  []metadata.Key
	)

	// Check if datasets are referenced in model.
	datasetsReferenced := hasDatasetsReferenced(bom)

	for _, spec := range modelRegistry {
		if spec.Weight <= 0 {
			continue
		}

		total++
		max += spec.Weight

		// The datasets field counts as missing when no datasets are referenced.
		ok := false
		if spec.Present != nil && (spec.Key != metadata.ModelCardModelParametersDatasets || datasetsReferenced) {
			ok = spec.Present(bom)
		}
		slog.Debug("completeness: model field", "field", spec.Key.String(), "present", ok, "required", spec.Required)

		if ok {
			passed++
			earned += spec.Weight
			continue
		}

		if spec.Required {
			missingReq = append(missingReq, spec.Key)
		} else {
			missingOpt = append(missingOpt, spec.Key)
		}
	}

	score := 0.0
	if max > 0 {
		score = earned / max
	}

	// Extract model ID from BOM.
	modelID := "(unknown)"
	if bom != nil && bom.Metadata != nil && bom.Metadata.Component != nil && bom.Metadata.Component.Name != "" {
		modelID = bom.Metadata.Component.Name
	}

	result := Result{
		ModelID:         modelID,
		Score:           score,
		Passed:          passed,
		Total:           total,
		MissingRequired: missingReq,
		MissingOptional: missingOpt,
		DatasetResults:  make(map[string]DatasetResult),
	}

	// Check dataset components if they exist.
	if bom.Components != nil && datasetsReferenced {
		for _, comp := range *bom.Components {
			if comp.Type == cdx.ComponentTypeData {
				dsResult := checkDatasetWithRegistry(&comp, datasetRegistry)
				result.DatasetResults[comp.Name] = dsResult
			}
		}
	}

	return result
}

// hasDatasetsReferenced checks if the model references any datasets.
func hasDatasetsReferenced(bom *cdx.BOM) bool {
	if bom == nil || bom.Metadata == nil || bom.Metadata.Component == nil {
		return false
	}
	comp := bom.Metadata.Component
	if comp.ModelCard == nil || comp.ModelCard.ModelParameters == nil {
		return false
	}
	mp := comp.ModelCard.ModelParameters
	if mp.Datasets == nil || len(*mp.Datasets) == 0 {
		return false
	}
	// Check if any dataset ref or inline dataset entry is non-empty.
	for _, ds := range *mp.Datasets {
		if ds.Ref != "" || (ds.ComponentData != nil && ds.ComponentData.Name != "") {
			return true
		}
	}
	return false
}

// CheckDataset checks completeness of a single dataset component using the default registry.
func CheckDataset(comp *cdx.Component) DatasetResult {
	return checkDatasetWithRegistry(comp, metadata.DatasetRegistry())
}

// checkDatasetWithRegistry allows injecting custom registry for testing.
func checkDatasetWithRegistry(comp *cdx.Component, datasetRegistry []metadata.DatasetFieldSpec) DatasetResult {
	var (
		earned, max float64
		passed      int
		total       int
		missingReq  []metadata.DatasetKey
		missingOpt  []metadata.DatasetKey
	)

	for _, spec := range datasetRegistry {
		if spec.Weight <= 0 {
			continue
		}
		total++
		max += spec.Weight

		ok := false
		if spec.Present != nil {
			ok = spec.Present(comp)
		}
		slog.Debug("completeness: dataset field", "dataset", comp.Name, "field", spec.Key.String(), "present", ok, "required", spec.Required)

		if ok {
			passed++
			earned += spec.Weight
			continue
		}

		if spec.Required {
			missingReq = append(missingReq, spec.Key)
		} else {
			missingOpt = append(missingOpt, spec.Key)
		}
	}

	score := 0.0
	if max > 0 {
		score = earned / max
	}

	return DatasetResult{
		DatasetRef:      comp.Name,
		Score:           score,
		Passed:          passed,
		Total:           total,
		MissingRequired: missingReq,
		MissingOptional: missingOpt,
	}
}
