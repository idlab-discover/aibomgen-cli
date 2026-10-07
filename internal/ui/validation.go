package ui

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/validator"
)

// ValidationUI provides a rich UI for the validation command.
type ValidationUI struct {
	writer  io.Writer
	quiet   bool
	verbose bool // also list every missing field
}

// NewValidationUI creates a new UI handler for the validation command.
// verbose additionally lists the names of missing completeness fields.
func NewValidationUI(w io.Writer, quiet, verbose bool) *ValidationUI {
	return &ValidationUI{
		writer:  w,
		quiet:   quiet,
		verbose: verbose,
	}
}

// PrintReport renders a beautiful validation report.
func (v *ValidationUI) PrintReport(report validator.ValidationResult) {
	if v.quiet {
		return
	}

	var output strings.Builder

	// Header with validation status.
	if report.Valid {
		output.WriteString(Success.Bold(true).Render("✓ Validation Passed"))
	} else {
		output.WriteString(Error.Bold(true).Render("✗ Validation Failed"))
	}
	output.WriteString("\n\n")

	// Model Section.
	output.WriteString(v.renderModelValidation(report))

	// Errors Section.
	if len(report.Errors) > 0 {
		output.WriteString("\n\n")
		output.WriteString(v.renderErrors(report.Errors))
	}

	// Warnings Section.
	if len(report.Warnings) > 0 {
		output.WriteString("\n\n")
		output.WriteString(v.renderWarnings(report.Warnings))
	}

	// Dataset Validation Section.
	if len(report.DatasetResults) > 0 {
		output.WriteString("\n\n")
		output.WriteString(v.renderDatasetValidation(report.DatasetResults))
	}

	// Wrap in appropriate box based on validation status.
	var boxed string
	if report.Valid {
		boxed = SuccessBox.Render(output.String())
	} else {
		boxed = ErrorBox.Render(output.String())
	}
	fmt.Fprintln(v.writer, boxed)
}

// renderModelValidation creates the model validation section.
func (v *ValidationUI) renderModelValidation(report validator.ValidationResult) string {
	var sb strings.Builder

	sb.WriteString(SectionHeader.Render("Model Component"))
	sb.WriteString("\n")

	// Show model ID if available.
	if report.ModelID != "" {
		sb.WriteString(FormatKeyValue("ID", Highlight.Render(report.ModelID)))
		sb.WriteString("\n")
	}

	sb.WriteString(FormatKeyValue("Completeness", scoreBar(report.CompletenessScore)))
	sb.WriteString("\n")
	sb.WriteString(v.renderMissing(len(report.MissingRequired), joinKeys(report.MissingOptional)))

	return sb.String()
}

// renderErrors creates the errors section.
func (v *ValidationUI) renderErrors(errors []string) string {
	var sb strings.Builder

	sb.WriteString(Error.Render(fmt.Sprintf("▼ Errors (%d)", len(errors))))
	sb.WriteString("\n")
	for _, err := range errors {
		sb.WriteString("  ")
		sb.WriteString(GetCrossMark())
		sb.WriteString(" ")
		sb.WriteString(err)
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

// renderWarnings creates the warnings section.
func (v *ValidationUI) renderWarnings(warnings []string) string {
	var sb strings.Builder

	sb.WriteString(Warning.Render(fmt.Sprintf("▼ Warnings (%d)", len(warnings))))
	sb.WriteString("\n")
	for _, warn := range warnings {
		sb.WriteString("  ")
		sb.WriteString(GetWarnMark())
		sb.WriteString(" ")
		sb.WriteString(Dim.Render(warn))
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

// renderDatasetValidation creates the dataset validation section.
func (v *ValidationUI) renderDatasetValidation(datasets map[string]validator.DatasetValidationResult) string {
	var sb strings.Builder

	sb.WriteString(SectionHeader.Render("Dataset Components"))
	sb.WriteString("\n")

	names := make([]string, 0, len(datasets))
	for name := range datasets {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, dsName := range names {
		dsResult := datasets[dsName]
		// Dataset name with label.
		sb.WriteString(FormatKeyValue("ID", Highlight.Render(dsName)))
		sb.WriteString("\n")

		sb.WriteString(FormatKeyValue("Completeness", scoreBar(dsResult.CompletenessScore)))
		sb.WriteString("\n")
		sb.WriteString(v.renderMissing(len(dsResult.MissingRequired), joinKeys(dsResult.MissingOptional)))
		sb.WriteString("\n")

		// Dataset-specific errors.
		if len(dsResult.Errors) > 0 {
			sb.WriteString("\n")
			sb.WriteString(Error.Render(fmt.Sprintf("▼ Errors (%d)", len(dsResult.Errors))))
			sb.WriteString("\n")
			for _, err := range dsResult.Errors {
				sb.WriteString("  ")
				sb.WriteString(GetCrossMark())
				sb.WriteString(" ")
				sb.WriteString(err)
				sb.WriteString("\n")
			}
		}

		// Dataset-specific warnings.
		if len(dsResult.Warnings) > 0 {
			sb.WriteString("\n")
			sb.WriteString(Warning.Render(fmt.Sprintf("▼ Warnings (%d)", len(dsResult.Warnings))))
			sb.WriteString("\n")
			for _, warn := range dsResult.Warnings {
				sb.WriteString("  ")
				sb.WriteString(GetWarnMark())
				sb.WriteString(" ")
				sb.WriteString(Dim.Render(warn))
				sb.WriteString("\n")
			}
		}

		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

// renderMissing summarises missing fields; in verbose mode it also names the
// missing optional fields (missing required ones are already listed as errors in --strict).
func (v *ValidationUI) renderMissing(required int, optional []string) string {
	if required == 0 && len(optional) == 0 {
		return Dim.Render("(all fields present)")
	}
	out := Dim.Render(fmt.Sprintf("(%d required, %d optional missing)", required, len(optional)))
	if v.verbose && len(optional) > 0 {
		out += "\n" + Dim.Render("missing optional:")
		for _, key := range optional {
			out += "\n" + Dim.Render("  - "+key)
		}
	}
	return out
}

// joinKeys converts completeness keys to strings.
func joinKeys[K ~string](keys []K) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = string(k)
	}
	return out
}
