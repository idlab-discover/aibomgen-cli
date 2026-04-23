package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/validator"
)

// ValidationUI provides a rich UI for the validation command.
type ValidationUI struct {
	writer io.Writer
	quiet  bool
}

// NewValidationUI creates a new UI handler for the validation command.
func NewValidationUI(w io.Writer, quiet bool) *ValidationUI {
	return &ValidationUI{
		writer: w,
		quiet:  quiet,
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

	// Completeness score.
	scoreBar := v.renderProgressBar(report.CompletenessScore, 40)
	scorePercent := v.renderScorePercentage(report.CompletenessScore)
	sb.WriteString(FormatKeyValue("Completeness", scoreBar+" "+scorePercent))
	sb.WriteString("\n")

	// Missing fields summary.
	totalMissing := len(report.MissingRequired) + len(report.MissingOptional)
	if totalMissing > 0 {
		sb.WriteString(Dim.Render(fmt.Sprintf("(%d required, %d optional missing)", len(report.MissingRequired), len(report.MissingOptional))))
	} else {
		sb.WriteString(Dim.Render("(all fields present)"))
	}

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

	for dsName, dsResult := range datasets {
		// Dataset name with label.
		sb.WriteString(FormatKeyValue("ID", Highlight.Render(dsName)))
		sb.WriteString("\n")

		// Completeness score.
		scoreBar := v.renderProgressBar(dsResult.CompletenessScore, 40)
		scorePercent := v.renderScorePercentage(dsResult.CompletenessScore)
		sb.WriteString(FormatKeyValue("Completeness", scoreBar+" "+scorePercent))
		sb.WriteString("\n")

		// Missing fields summary.
		totalMissing := len(dsResult.MissingRequired) + len(dsResult.MissingOptional)
		if totalMissing > 0 {
			sb.WriteString(Dim.Render(fmt.Sprintf("(%d required, %d optional missing)", len(dsResult.MissingRequired), len(dsResult.MissingOptional))))
		} else {
			sb.WriteString(Dim.Render("(all fields present)"))
		}
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
			if len(dsResult.Errors) > 0 {
				sb.WriteString("\n")
			} else {
				sb.WriteString("\n")
			}
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

// renderProgressBar creates a visual progress bar (same as completeness).
func (v *ValidationUI) renderProgressBar(score float64, width int) string {
	filled := int(score * float64(width))
	empty := width - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)

	// Color the bar based on score.
	if score >= 0.8 {
		return Success.Render(bar)
	} else if score >= 0.5 {
		return Warning.Render(bar)
	}
	return Error.Render(bar)
}

// renderScorePercentage formats the score as a percentage (same as completeness).
func (v *ValidationUI) renderScorePercentage(score float64) string {
	percentage := score * 100
	formatted := fmt.Sprintf("%.1f%%", percentage)

	if score >= 0.8 {
		return Success.Render(formatted)
	} else if score >= 0.5 {
		return Warning.Render(formatted)
	}
	return Error.Render(formatted)
}

// PrintSimpleReport prints a minimal text report.
func (v *ValidationUI) PrintSimpleReport(report validator.ValidationResult) {
	if report.Valid {
		fmt.Fprintf(v.writer, "%s Validation passed\n", GetCheckMark())
	} else {
		fmt.Fprintf(v.writer, "%s Validation failed\n", GetCrossMark())
	}

	fmt.Fprintf(v.writer, "Completeness: %.1f%%\n", report.CompletenessScore*100)
	fmt.Fprintf(v.writer, "Errors: %d, Warnings: %d\n", len(report.Errors), len(report.Warnings))

	if len(report.DatasetResults) > 0 {
		fmt.Fprintf(v.writer, "Datasets: %d\n", len(report.DatasetResults))
	}
}
