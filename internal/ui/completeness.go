package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/completeness"

	lipgloss "charm.land/lipgloss/v2"
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

	sb.WriteString(FormatKeyValue("Score", c.renderProgressBar(result.Score, 40)+" "+c.renderScorePercentage(result.Score)))
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
		sb.WriteString(FormatKeyValue("Score", c.renderProgressBar(dsResult.Score, 40)+" "+c.renderScorePercentage(dsResult.Score)))
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

// renderProgressBar creates a visual progress bar.
func (c *CompletenessUI) renderProgressBar(score float64, width int) string {
	filled := int(score * float64(width))
	empty := width - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)

	// Color the bar based on score.
	var style lipgloss.Style
	if score >= 0.8 {
		style = lipgloss.NewStyle().Foreground(ColorSuccess)
	} else if score >= 0.5 {
		style = lipgloss.NewStyle().Foreground(ColorWarning)
	} else {
		style = lipgloss.NewStyle().Foreground(ColorError)
	}

	return style.Render(bar)
}

// renderScorePercentage formats the score as a percentage.
func (c *CompletenessUI) renderScorePercentage(score float64) string {
	percentage := score * 100
	formatted := fmt.Sprintf("%.1f%%", percentage)

	if score >= 0.8 {
		return Success.Render(formatted)
	} else if score >= 0.5 {
		return Warning.Render(formatted)
	}
	return Error.Render(formatted)
}
