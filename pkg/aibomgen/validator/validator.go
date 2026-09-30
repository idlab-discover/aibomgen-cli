package validator

import (
	"fmt"
	"sort"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/internal/metadata"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/completeness"
)

// ValidationResult is returned by [Validate] and summarises the outcome of.
// all checks performed on the BOM.
type ValidationResult struct {
	ModelID  string
	Valid    bool
	Errors   []string
	Warnings []string

	// AIBOM-specific metrics.
	CompletenessScore float64
	MissingRequired   []metadata.Key
	MissingOptional   []metadata.Key

	// Dataset-specific results.
	DatasetResults map[string]DatasetValidationResult // key is dataset name
}

// DatasetValidationResult holds validation results for a single dataset.
// component within the BOM.
type DatasetValidationResult struct {
	DatasetRef        string
	CompletenessScore float64
	MissingRequired   []metadata.DatasetKey
	MissingOptional   []metadata.DatasetKey
	Errors            []string
	Warnings          []string
}

// ValidationOptions configures the behaviour of [Validate].
type ValidationOptions struct {
	StrictMode           bool    // Fail if required fields missing
	MinCompletenessScore float64 // Minimum acceptable score (0.0-1.0)
	CheckModelCard       bool    // Validate model card fields
}

// Validate checks the structural and completeness properties of bom.
// It returns a [ValidationResult] with errors and warnings; Valid is false.
// when any hard error is found or when strict-mode thresholds are not met.
func Validate(bom *cdx.BOM, opts ValidationOptions) ValidationResult {

	result := ValidationResult{
		Valid:          true,
		Errors:         []string{},
		Warnings:       []string{},
		DatasetResults: make(map[string]DatasetValidationResult),
	}

	// 1. Basic structural validation.
	if bom == nil {
		result.Valid = false
		result.Errors = append(result.Errors, "BOM is nil")
		return result
	}

	// 2. Check metadata component exists.
	if bom.Metadata == nil || bom.Metadata.Component == nil {
		result.Valid = false
		result.Errors = append(result.Errors, "BOM missing metadata.component")
	}

	// 3. Validate spec version.
	validateSpecVersion(bom, &result)

	// 4. Run completeness check (leverages existing package).
	completenessResult := completeness.Check(bom)
	result.ModelID = completenessResult.ModelID
	result.CompletenessScore = completenessResult.Score
	result.MissingRequired = completenessResult.MissingRequired
	result.MissingOptional = completenessResult.MissingOptional

	// 5. Strict mode enforcement.
	if opts.StrictMode {
		if len(completenessResult.MissingRequired) > 0 {
			result.Valid = false
			for _, key := range completenessResult.MissingRequired {
				msg := fmt.Sprintf("required field missing: %s", key)
				result.Errors = append(result.Errors, msg)
			}
		}

		if completenessResult.Score < opts.MinCompletenessScore {
			result.Valid = false
			msg := fmt.Sprintf("completeness score %.2f below minimum %.2f", completenessResult.Score, opts.MinCompletenessScore)
			result.Errors = append(result.Errors, msg)
		}
	}

	// 6. Add warnings for optional fields.
	for _, key := range completenessResult.MissingOptional {
		msg := fmt.Sprintf("optional field missing: %s", key)
		result.Warnings = append(result.Warnings, msg)
	}

	// 7. Model card validation.
	if opts.CheckModelCard {
		validateModelCard(bom, &result)
	}

	// 8. Reference integrity: every ref must point at an existing bom-ref.
	result.Warnings = append(result.Warnings, DanglingRefs(bom)...)

	// 9. Validate dataset components if they exist.
	for dsName, dsCompletenessResult := range completenessResult.DatasetResults {
		dsResult := DatasetValidationResult{
			DatasetRef:        dsCompletenessResult.DatasetRef,
			CompletenessScore: dsCompletenessResult.Score,
			MissingRequired:   dsCompletenessResult.MissingRequired,
			MissingOptional:   dsCompletenessResult.MissingOptional,
			Errors:            []string{},
			Warnings:          []string{},
		}

		// Strict mode for dataset components (optional fields only).
		if opts.StrictMode && len(dsCompletenessResult.MissingRequired) > 0 {
			for _, key := range dsCompletenessResult.MissingRequired {
				msg := fmt.Sprintf("required dataset field missing: %s", key)
				dsResult.Errors = append(dsResult.Errors, msg)
				result.Warnings = append(result.Warnings, fmt.Sprintf("dataset %s: %s", dsName, msg))
			}
		}

		// Add warnings for optional dataset fields.
		for _, key := range dsCompletenessResult.MissingOptional {
			msg := fmt.Sprintf("optional dataset field missing: %s", key)
			dsResult.Warnings = append(dsResult.Warnings, msg)
		}

		result.DatasetResults[dsName] = dsResult
	}

	return result
}

func validateSpecVersion(bom *cdx.BOM, result *ValidationResult) {
	if bom.SpecVersion == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "BOM missing spec version")
		return
	}

	// Check if spec version is valid.
	switch bom.SpecVersion {
	case cdx.SpecVersion1_0, cdx.SpecVersion1_1, cdx.SpecVersion1_2,
		cdx.SpecVersion1_3, cdx.SpecVersion1_4, cdx.SpecVersion1_5,
		cdx.SpecVersion1_6, cdx.SpecVersion1_7:
		// Valid spec version.
	default:
		result.Valid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("invalid or unsupported spec version: %d", bom.SpecVersion))
		return
	}

	// Warn about older spec versions (< 1.5 doesn't have full ML-BOM support).
	if bom.SpecVersion < cdx.SpecVersion1_5 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("spec version 1.%d predates ML-BOM support (consider upgrading to 1.5+)",
				bom.SpecVersion-1))
	}
}

func validateModelCard(bom *cdx.BOM, result *ValidationResult) {
	comp := bom.Metadata.Component
	if comp == nil {
		return
	}

	if comp.ModelCard == nil {
		result.Warnings = append(result.Warnings, "model card not present")
		return
	}

	if comp.ModelCard.ModelParameters == nil {
		result.Warnings = append(result.Warnings, "model parameters not present")
	}
}

// DanglingRefs returns one message per reference in bom that does not resolve to a
// bom-ref present in the BOM. It checks model-card dataset refs
// (modelCard.modelParameters.datasets[].ref) and the dependency graph
// (dependencies[].ref and dependsOn). A nil or empty BOM has no dangling refs.
func DanglingRefs(bom *cdx.BOM) []string {
	if bom == nil {
		return nil
	}
	refs := collectBOMRefs(bom)
	var out []string

	checkModelCard := func(owner string, c *cdx.Component) {
		if c == nil || c.ModelCard == nil || c.ModelCard.ModelParameters == nil || c.ModelCard.ModelParameters.Datasets == nil {
			return
		}
		for _, ds := range *c.ModelCard.ModelParameters.Datasets {
			if ds.Ref == "" {
				continue
			}
			if _, ok := refs[ds.Ref]; !ok {
				out = append(out, fmt.Sprintf("dangling reference: %s modelCard dataset ref %q has no matching bom-ref", owner, ds.Ref))
			}
		}
	}
	if bom.Metadata != nil && bom.Metadata.Component != nil {
		checkModelCard("metadata.component", bom.Metadata.Component)
	}
	if bom.Components != nil {
		walkComponents(*bom.Components, func(c *cdx.Component) {
			checkModelCard(fmt.Sprintf("component %q", c.Name), c)
		})
	}

	if bom.Dependencies != nil {
		for _, dep := range *bom.Dependencies {
			if _, ok := refs[dep.Ref]; !ok {
				out = append(out, fmt.Sprintf("dangling reference: dependency ref %q has no matching bom-ref", dep.Ref))
			}
			if dep.Dependencies == nil {
				continue
			}
			for _, d := range *dep.Dependencies {
				if _, ok := refs[d]; !ok {
					out = append(out, fmt.Sprintf("dangling reference: %q dependsOn %q has no matching bom-ref", dep.Ref, d))
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

// collectBOMRefs gathers the bom-refs of the metadata component, all (nested)
// components, their data entries and all (nested) services.
func collectBOMRefs(bom *cdx.BOM) map[string]struct{} {
	refs := map[string]struct{}{}
	add := func(c *cdx.Component) {
		if c.BOMRef != "" {
			refs[c.BOMRef] = struct{}{}
		}
		if c.Data != nil {
			for _, d := range *c.Data {
				if d.BOMRef != "" {
					refs[d.BOMRef] = struct{}{}
				}
			}
		}
	}
	if bom.Metadata != nil && bom.Metadata.Component != nil {
		add(bom.Metadata.Component)
		if bom.Metadata.Component.Components != nil {
			walkComponents(*bom.Metadata.Component.Components, add)
		}
	}
	if bom.Components != nil {
		walkComponents(*bom.Components, add)
	}
	if bom.Services != nil {
		walkServices(*bom.Services, func(s *cdx.Service) {
			if s.BOMRef != "" {
				refs[s.BOMRef] = struct{}{}
			}
		})
	}
	return refs
}

func walkComponents(components []cdx.Component, fn func(*cdx.Component)) {
	for i := range components {
		c := &components[i]
		fn(c)
		if c.Components != nil {
			walkComponents(*c.Components, fn)
		}
	}
}

func walkServices(services []cdx.Service, fn func(*cdx.Service)) {
	for i := range services {
		s := &services[i]
		fn(s)
		if s.Services != nil {
			walkServices(*s.Services, fn)
		}
	}
}
