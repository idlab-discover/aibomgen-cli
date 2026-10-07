package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/completeness"
)

// CompletenessUI provides a rich UI for the completeness command.
type CompletenessUI struct {
	writer io.Writer
	quiet  bool
}

// NewCompletenessUI creates a new UI handler for the completeness command.
func NewCompletenessUI(w io.Writer, quiet bool) *CompletenessUI {
	return &CompletenessUI{
		writer: w,
		quiet:  quiet,
	}
}

// PrintReport renders a beautiful completeness report.
func (c *CompletenessUI) PrintReport(result completeness.Result) {
	if c.quiet {
		return
	}

	var output strings.Builder

	// Header.
	output.WriteString(Success.Bold(true).Render("AIBOM Completeness Report"))
	output.WriteString("\n\n")

	// Model Score Section.
	output.WriteString(c.renderModelScore(result))
	output.WriteString("\n\n")

	// Missing Fields Section.
	if len(result.MissingRequired) > 0 || len(result.MissingOptional) > 0 {
		output.WriteString(c.renderMissingFields(result))
		output.WriteString("\n\n")
	}

	// Dataset Scores Section.
	if len(result.DatasetResults) > 0 {
		output.WriteString(c.renderDatasetScores(result.DatasetResults))
		output.WriteString("\n")
	}

	// Wrap in box.
	boxed := SuccessBox.Render(output.String())
	fmt.Fprintln(c.writer, boxed)
}

// renderModelScore creates the model score visualization with progress bar.
func (c *CompletenessUI) renderModelScore(result completeness.Result) string {
	var sb strings.Builder

	sb.WriteString(SectionHeader.Render("Model Component"))
	sb.WriteString("\n")

	// Show model ID if available.
	if result.ModelID != "" {
		sb.WriteString(FormatKeyValue("ID", Highlight.Render(result.ModelID)))
		sb.WriteString("\n")
	}

	sb.WriteString(FormatKeyValue("Score", scoreBar(result.Score)))
	sb.WriteString("\n")
	sb.WriteString(Dim.Render(fmt.Sprintf("(%d/%d fields present)", result.Passed, result.Total)))

	return sb.String()
}

// renderMissingFields creates the missing fields section with expandable groups.
func (c *CompletenessUI) renderMissingFields(result completeness.Result) string {
	var sb strings.Builder

	// Required Fields.
	if len(result.MissingRequired) > 0 {
		sb.WriteString(Error.Render(fmt.Sprintf("▼ Required Fields (%d missing)", len(result.MissingRequired))))
		sb.WriteString("\n")
		for _, field := range result.MissingRequired {
			sb.WriteString("  ")
			sb.WriteString(GetCrossMark())
			sb.WriteString(" ")
			sb.WriteString(field.String())
			sb.WriteString("\n")
		}
	}

	// Optional Fields.
	if len(result.MissingOptional) > 0 {
		if len(result.MissingRequired) > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(Warning.Render(fmt.Sprintf("▼ Optional Fields (%d missing)", len(result.MissingOptional))))
		sb.WriteString("\n")
		for _, field := range result.MissingOptional {
			sb.WriteString("  ")
			sb.WriteString(GetWarnMark())
			sb.WriteString(" ")
			sb.WriteString(Dim.Render(field.String()))
			sb.WriteString("\n")
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// renderDatasetScores creates the dataset scores section.
func (c *CompletenessUI) renderDatasetScores(datasets map[string]completeness.DatasetResult) string {
	var sb strings.Builder

	sb.WriteString(SectionHeader.Render("Dataset Components"))
	sb.WriteString("\n")

	for dsName, dsResult := range datasets {
		// Dataset name with label.
		sb.WriteString(FormatKeyValue("ID", Highlight.Render(dsName)))
		sb.WriteString("\n")

		// Progress bar with label.
		sb.WriteString(FormatKeyValue("Score", scoreBar(dsResult.Score)))
		sb.WriteString("\n")
		sb.WriteString(Dim.Render(fmt.Sprintf("(%d/%d fields present)", dsResult.Passed, dsResult.Total)))
		sb.WriteString("\n")

		// Missing fields for this dataset - show underneath each other like model component.
		if len(dsResult.MissingRequired) > 0 {
			sb.WriteString("\n")
			sb.WriteString(Error.Render(fmt.Sprintf("▼ Required Fields (%d missing)", len(dsResult.MissingRequired))))
			sb.WriteString("\n")
			for _, field := range dsResult.MissingRequired {
				sb.WriteString("  ")
				sb.WriteString(GetCrossMark())
				sb.WriteString(" ")
				sb.WriteString(field.String())
				sb.WriteString("\n")
			}
		}
		if len(dsResult.MissingOptional) > 0 {
			if len(dsResult.MissingRequired) > 0 {
				sb.WriteString("\n")
			} else {
				sb.WriteString("\n")
			}
			sb.WriteString(Warning.Render(fmt.Sprintf("▼ Optional Fields (%d missing)", len(dsResult.MissingOptional))))
			sb.WriteString("\n")
			for _, field := range dsResult.MissingOptional {
				sb.WriteString("  ")
				sb.WriteString(GetWarnMark())
				sb.WriteString(" ")
				sb.WriteString(Dim.Render(field.String()))
				sb.WriteString("\n")
			}
		}

		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

// scoreBar renders a completeness score as a coloured 40-cell bar plus percentage.
func scoreBar(score float64) string {
	style := Error
	if score >= 0.8 {
		style = Success
	} else if score >= 0.5 {
		style = Warning
	}
	filled := int(score * 40)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 40-filled)
	return style.Render(bar) + " " + style.Render(fmt.Sprintf("%.1f%%", score*100))
}
