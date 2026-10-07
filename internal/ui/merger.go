package ui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/merger"
)

// PrintMergeSummary displays the merge summary with styled output.
func PrintMergeSummary(w io.Writer, result *merger.MergeResult, outputPath string, aibomCount int, deduplicate bool, duration time.Duration) {
	var output strings.Builder

	// Header.
	output.WriteString(Success.Bold(true).Render("✓ Merge Completed Successfully"))
	output.WriteString("\n\n")

	// Summary section.
	output.WriteString(SectionHeader.Render("Summary"))
	output.WriteString("\n\n")

	// SBOM Metadata Component.
	if result.MetadataComponent != "" {
		output.WriteString(fmt.Sprintf("  %s\n",
			Muted.Render("SBOM Metadata Component:")))
		output.WriteString(fmt.Sprintf("    %s %s\n",
			GetBullet(),
			Dim.Render(truncateName(result.MetadataComponent, 50))))
		output.WriteString("\n")
	}

	// SBOM Components.
	if len(result.SBOMComponents) > 0 {
		output.WriteString(fmt.Sprintf("  %s      %s\n",
			Muted.Render("SBOM Components:"),
			Bold.Render(fmt.Sprintf("%d", len(result.SBOMComponents)))))
		for _, comp := range result.SBOMComponents {
			output.WriteString(fmt.Sprintf("    %s %s\n",
				GetBullet(),
				Dim.Render(truncateName(comp, 50))))
		}
		output.WriteString("\n")
	}

	// Model Components.
	if len(result.ModelComponents) > 0 {
		output.WriteString(fmt.Sprintf("  %s    %s\n",
			Muted.Render("Model Components:"),
			Bold.Render(fmt.Sprintf("%d", len(result.ModelComponents)))))
		for _, model := range result.ModelComponents {
			output.WriteString(fmt.Sprintf("    %s %s\n",
				GetBullet(),
				Dim.Render(truncateName(model, 50))))
		}
		output.WriteString("\n")
	}

	// Dataset Components.
	if len(result.DatasetComponents) > 0 {
		output.WriteString(fmt.Sprintf("  %s   %s\n",
			Muted.Render("Dataset Components:"),
			Bold.Render(fmt.Sprintf("%d", len(result.DatasetComponents)))))
		for _, dataset := range result.DatasetComponents {
			output.WriteString(fmt.Sprintf("    %s %s\n",
				GetBullet(),
				Dim.Render(truncateName(dataset, 50))))
		}
		output.WriteString("\n")
	}

	// Show duplicates removed if applicable.
	if deduplicate && result.DuplicatesRemoved > 0 {
		output.WriteString(fmt.Sprintf("  %s %s\n",
			Muted.Render("Duplicates Removed:"),
			Warning.Render(fmt.Sprintf("%d", result.DuplicatesRemoved))))
		output.WriteString("\n")
	}

	// Totals.
	totalComponents := result.SBOMComponentCount + result.AIBOMComponentCount
	output.WriteString(fmt.Sprintf("  %s   %s\n",
		Muted.Render("Total Components:"),
		Success.Render(fmt.Sprintf("%d", totalComponents))))

	output.WriteString(fmt.Sprintf("  %s      %s\n",
		Muted.Render("AIBOMs Merged:"),
		Bold.Render(fmt.Sprintf("%d", aibomCount))))

	// Timing.
	output.WriteString(fmt.Sprintf("  %s          %s\n",
		Muted.Render("Duration:"),
		Dim.Render(formatDuration(duration))))

	// Output file.
	output.WriteString("\n")
	output.WriteString(fmt.Sprintf("  %s %s\n",
		Muted.Render("Output:"),
		Success.Render(outputPath)))

	// Wrap in success box.
	boxed := SuccessBox.Render(output.String())
	fmt.Fprintln(w, "\n"+boxed)
}

// PrintMergeError displays a merge error message.
func PrintMergeError(w io.Writer, err error) {
	var output strings.Builder
	output.WriteString(Error.Bold(true).Render("✗ Merge Failed"))
	output.WriteString("\n\n")
	output.WriteString(err.Error())

	boxed := ErrorBox.Render(output.String())
	fmt.Fprintln(w, "\n"+boxed)
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}

// truncateName truncates a component name to a maximum length.
func truncateName(name string, maxLen int) string {
	if len(name) <= maxLen {
		return name
	}
	return name[:maxLen-3] + "..."
}
