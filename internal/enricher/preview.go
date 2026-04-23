package enricher

import (
	"fmt"
	"strings"

	"charm.land/huh/v2"
	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/internal/metadata"
	"github.com/idlab-discover/aibomgen-cli/internal/ui"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/completeness"
)

// ShowPreviewWithConfirm shows a preview of changes and asks for confirmation using huh.
func ShowPreviewWithConfirm(
	initial completeness.Result,
	postRefetch completeness.Result,
	enriched *cdx.BOM,
	modelChanges map[metadata.Key]string,
	datasetChanges map[string]map[metadata.DatasetKey]string,
) (bool, error) {

	// Build preview content.
	var sb strings.Builder

	// Header.
	sb.WriteString(ui.Bold.Render("Preview Changes"))
	sb.WriteString("\n\n")

	// Model changes.
	if len(modelChanges) > 0 {
		sb.WriteString(ui.Primary.Render("Model Fields:"))
		sb.WriteString("\n")
		for key, value := range modelChanges {
			sb.WriteString(fmt.Sprintf("  %s %s: %s\n",
				ui.Success.Render("✓"),
				ui.Secondary.Render(string(key)),
				ui.Dim.Render(truncateValue(value, 60))))
		}
		sb.WriteString("\n")
	}

	// Dataset changes.
	if len(datasetChanges) > 0 {
		sb.WriteString(ui.Primary.Render("Dataset Fields:"))
		sb.WriteString("\n")
		for dsName, changes := range datasetChanges {
			sb.WriteString(fmt.Sprintf("  %s:\n", ui.Bold.Render(dsName)))
			for key, value := range changes {
				sb.WriteString(fmt.Sprintf("    %s %s: %s\n",
					ui.Success.Render("✓"),
					ui.Secondary.Render(string(key)),
					ui.Dim.Render(truncateValue(value, 60))))
			}
		}
		sb.WriteString("\n")
	}

	// Completeness progression.
	finalResult := completeness.Check(enriched)
	sb.WriteString(ui.Primary.Render("Completeness Progress:"))
	sb.WriteString("\n")

	scoreStyle := func(score float64) string {
		percentage := fmt.Sprintf("%.1f%%", score*100)
		if score >= 0.8 {
			return ui.Success.Render(percentage)
		} else if score >= 0.5 {
			return ui.Warning.Render(percentage)
		}
		return ui.Error.Render(percentage)
	}

	sb.WriteString(fmt.Sprintf("  Initial:          %s (%d/%d fields)\n",
		scoreStyle(initial.Score),
		initial.Passed, initial.Total))

	if postRefetch.Score != initial.Score {
		sb.WriteString(fmt.Sprintf("  After refetch:    %s (%d/%d fields)\n",
			scoreStyle(postRefetch.Score),
			postRefetch.Passed, postRefetch.Total))
	}

	sb.WriteString(fmt.Sprintf("  After enrichment: %s (%d/%d fields)\n",
		scoreStyle(finalResult.Score),
		finalResult.Passed, finalResult.Total))

	// Show dataset completeness.
	if len(finalResult.DatasetResults) > 0 {
		sb.WriteString("\n")
		sb.WriteString(ui.Primary.Render("Datasets:"))
		sb.WriteString("\n")
		for dsName, dsResult := range finalResult.DatasetResults {
			sb.WriteString(fmt.Sprintf("  %s: %s (%d/%d fields)\n",
				dsName,
				scoreStyle(dsResult.Score),
				dsResult.Passed, dsResult.Total))
		}
	}

	// Render preview in a box using UI styles.
	previewBox := ui.Box.Render(sb.String())

	fmt.Println(previewBox)

	// Confirmation prompt.
	var confirm bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Save changes?").
				Description("Do you want to save these changes to the BOM?").
				Value(&confirm).
				Affirmative("Yes").
				Negative("No"),
		),
	)

	err := form.Run()
	if err != nil {
		return false, err
	}

	return confirm, nil
}

func truncateValue(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
