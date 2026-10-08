package validator

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/internal/metadata"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/completeness"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/vulnscan"
)

// ValidationResult is returned by [Validate] and summarises the outcome of.
// all checks performed on the BOM.
type ValidationResult struct {
	ModelID  string   `json:"modelId"`
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`

	// AIBOM-specific metrics.
	CompletenessScore float64        `json:"completenessScore"`
	MissingRequired   []metadata.Key `json:"missingRequired"`
	MissingOptional   []metadata.Key `json:"missingOptional"`

	// Dataset-specific results.
	DatasetResults map[string]DatasetValidationResult `json:"datasets,omitempty"` // key is dataset name
}

// DatasetValidationResult holds validation results for a single dataset.
// component within the BOM.
type DatasetValidationResult struct {
	DatasetRef        string                `json:"datasetRef"`
	CompletenessScore float64               `json:"completenessScore"`
	MissingRequired   []metadata.DatasetKey `json:"missingRequired"`
	MissingOptional   []metadata.DatasetKey `json:"missingOptional"`
	Errors            []string              `json:"errors"`
	Warnings          []string              `json:"warnings"`
}

// ValidationOptions configures the behaviour of [Validate].
type ValidationOptions struct {
	StrictMode           bool    // Fail if required fields missing
	MinCompletenessScore float64 // Minimum acceptable score (0.0-1.0), enforced in every mode

	// FailSeverity is the lowest vulnerability severity that is an error in
	// strict mode (default medium). Other vulnerabilities are reported as warnings.
	FailSeverity cdx.Severity
}

// ValidateData validates an encoded BOM (JSON or XML). JSON is first checked
// against the official CycloneDX schema (see [ValidateJSONSchema]); XML gets a
// warning that schema validation was skipped. The decoded BOM then goes through
// [Validate]. A BOM that cannot be decoded is reported as invalid, not as an error.
func ValidateData(data []byte, opts ValidationOptions) ValidationResult {
	// Same content sniffing as bomio.ReadBOM (bomio can't be imported here: it
	// depends on generator, whose tests import validator).
	trimmed := bytes.TrimLeft(data, " \t\r\n\ufeff")
	isXML := len(trimmed) > 0 && trimmed[0] == '<'

	var violations []string
	skipped := "XML BOM: CycloneDX schema validation is only performed for JSON"
	fileFmt := cdx.BOMFileFormatXML
	if !isXML {
		violations, skipped = ValidateJSONSchema(data)
		fileFmt = cdx.BOMFileFormatJSON
	}

	bom := new(cdx.BOM)
	err := cdx.NewBOMDecoder(bytes.NewReader(data), fileFmt).Decode(bom)
	var result ValidationResult
	if err != nil {
		result = ValidationResult{Errors: []string{fmt.Sprintf("cannot decode BOM: %v", err)}, DatasetResults: map[string]DatasetValidationResult{}}
	} else {
		result = Validate(bom, opts)
	}

	if len(violations) > 0 {
		result.Valid = false
		result.Errors = append(violations, result.Errors...)
	}
	if skipped != "" {
		result.Warnings = append([]string{skipped}, result.Warnings...)
	}
	return result
}

// Validate checks the structural and completeness properties of a decoded bom.
// It returns a [ValidationResult] with errors and warnings; Valid is false
// when any hard error is found or when strict-mode thresholds are not met.
// Missing optional fields are listed in the result, not reported as warnings.
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

	// 5. Strict mode: missing required fields are errors.
	if opts.StrictMode {
		for _, key := range completenessResult.MissingRequired {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("required field missing: %s", key))
		}
	}

	// 6. Minimum completeness score (any mode).
	if completenessResult.Score < opts.MinCompletenessScore {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("completeness score %.2f below minimum %.2f", completenessResult.Score, opts.MinCompletenessScore))
	}

	// 7. Reference integrity: bom-refs must be unique, and every ref must resolve.
	if dups := DuplicateBOMRefs(bom); len(dups) > 0 {
		result.Valid = false
		result.Errors = append(result.Errors, dups...)
	}
	result.Warnings = append(result.Warnings, DanglingRefs(bom)...)

	// 8. Vulnerabilities: errors in strict mode at or above FailSeverity, else warnings.
	validateVulnerabilities(bom, opts, &result)

	// 9. Dataset components: missing required fields are errors in strict mode.
	for dsName, dsCompletenessResult := range completenessResult.DatasetResults {
		dsResult := DatasetValidationResult{
			DatasetRef:        dsCompletenessResult.DatasetRef,
			CompletenessScore: dsCompletenessResult.Score,
			MissingRequired:   dsCompletenessResult.MissingRequired,
			MissingOptional:   dsCompletenessResult.MissingOptional,
			Errors:            []string{},
			Warnings:          []string{},
		}

		if opts.StrictMode {
			for _, key := range dsCompletenessResult.MissingRequired {
				result.Valid = false
				dsResult.Errors = append(dsResult.Errors, fmt.Sprintf("required dataset field missing: %s", key))
			}
		}

		result.DatasetResults[dsName] = dsResult
	}

	return result
}

func validateVulnerabilities(bom *cdx.BOM, opts ValidationOptions, result *ValidationResult) {
	if bom.Vulnerabilities == nil {
		return
	}
	threshold := opts.FailSeverity
	if threshold == "" {
		threshold = cdx.SeverityMedium
	}
	for _, v := range *bom.Vulnerabilities {
		sev := vulnscan.HighestSeverity(v)
		msg := fmt.Sprintf("vulnerability %s [%s] affects %s", vulnName(v), sev, affectedRefs(v))
		if opts.StrictMode && vulnscan.SeverityAtLeast(sev, threshold) {
			result.Valid = false
			result.Errors = append(result.Errors, msg)
		} else {
			result.Warnings = append(result.Warnings, msg)
		}
	}
}

// vulnName returns the vulnerability ID (e.g. a CVE) or, for findings without
// one such as Hugging Face scanner results, its bom-ref.
func vulnName(v cdx.Vulnerability) string {
	if v.ID != "" {
		return v.ID
	}
	return v.BOMRef
}

func affectedRefs(v cdx.Vulnerability) string {
	if v.Affects == nil || len(*v.Affects) == 0 {
		return "(no affected component)"
	}
	refs := make([]string, 0, len(*v.Affects))
	for _, a := range *v.Affects {
		refs = append(refs, a.Ref)
	}
	return strings.Join(refs, ", ")
}

func validateSpecVersion(bom *cdx.BOM, result *ValidationResult) {
	if bom.SpecVersion == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "BOM missing spec version")
		return
	}

	// Warn about older spec versions (< 1.5 doesn't have full ML-BOM support).
	if bom.SpecVersion < cdx.SpecVersion1_5 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("spec version 1.%d predates ML-BOM support (consider upgrading to 1.5+)",
				bom.SpecVersion-1))
	}
}

// DanglingRefs returns one message per reference in bom that does not resolve to a
// bom-ref present in the BOM. It checks model-card dataset refs
// (modelCard.modelParameters.datasets[].ref), the dependency graph
// (dependencies[].ref and dependsOn) and vulnerabilities[].affects[].ref.
// A nil or empty BOM has no dangling refs.
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
	if bom.Vulnerabilities != nil {
		for _, v := range *bom.Vulnerabilities {
			if v.Affects == nil {
				continue
			}
			for _, a := range *v.Affects {
				if _, ok := refs[a.Ref]; !ok {
					out = append(out, fmt.Sprintf("dangling reference: vulnerability %s affects %q has no matching bom-ref", vulnName(v), a.Ref))
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

// DuplicateBOMRefs returns one message per bom-ref that is used more than once
// across components, their data entries, services and vulnerabilities. CycloneDX
// requires bom-refs to be unique within a BOM.
func DuplicateBOMRefs(bom *cdx.BOM) []string {
	if bom == nil {
		return nil
	}
	counts := map[string]int{}
	forEachBOMRef(bom, func(ref string) { counts[ref]++ })
	if bom.Vulnerabilities != nil {
		for _, v := range *bom.Vulnerabilities {
			if v.BOMRef != "" {
				counts[v.BOMRef]++
			}
		}
	}
	var out []string
	for ref, n := range counts {
		if n > 1 {
			out = append(out, fmt.Sprintf("duplicate bom-ref %q used %d times", ref, n))
		}
	}
	sort.Strings(out)
	return out
}

// collectBOMRefs gathers the bom-refs of the metadata component, all (nested)
// components, their data entries and all (nested) services.
func collectBOMRefs(bom *cdx.BOM) map[string]struct{} {
	refs := map[string]struct{}{}
	forEachBOMRef(bom, func(ref string) { refs[ref] = struct{}{} })
	return refs
}

// forEachBOMRef calls fn for every non-empty bom-ref of the metadata component,
// all (nested) components, their data entries and all (nested) services.
func forEachBOMRef(bom *cdx.BOM, fn func(string)) {
	emit := func(ref string) {
		if ref != "" {
			fn(ref)
		}
	}
	add := func(c *cdx.Component) {
		emit(c.BOMRef)
		if c.Data != nil {
			for _, d := range *c.Data {
				emit(d.BOMRef)
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
		walkServices(*bom.Services, func(s *cdx.Service) { emit(s.BOMRef) })
	}
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
