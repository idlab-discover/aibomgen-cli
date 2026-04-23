package ui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/merger"
)

// MergerUI provides a rich UI for the merge command.
type MergerUI struct {
	writer    io.Writer
	quiet     bool
	workflow  *Workflow
	startTime time.Time
}

// NewMergerUI creates a new UI handler for the merge command.
func NewMergerUI(w io.Writer, quiet bool) *MergerUI {
	return &MergerUI{
		writer:    w,
		quiet:     quiet,
		startTime: time.Now(),
	}
}

// StartWorkflow initializes and displays the workflow for merging.
func (m *MergerUI) StartWorkflow(aibomCount int) {
	if m.quiet {
		return
	}

	m.startTime = time.Now()

	if aibomCount == 1 {
		m.workflow = NewWorkflow(m.writer, "Merging AIBOM with SBOM")
	} else {
		m.workflow = NewWorkflow(m.writer, fmt.Sprintf("Merging %d AIBOMs with SBOM", aibomCount))
	}

	m.workflow.AddTask("Reading SBOM")
	m.workflow.AddTask("Reading AIBOM(s)")
	m.workflow.AddTask("Merging BOMs")
	m.workflow.AddTask("Writing output")

	m.workflow.Start()
}

// StartReadingSBOM marks the SBOM reading step as running.
func (m *MergerUI) StartReadingSBOM(path string) {
	if m.quiet || m.workflow == nil {
		return
	}
	m.workflow.StartTask(0, Dim.Render(path))
}

// CompleteReadingSBOM marks SBOM reading as complete.
func (m *MergerUI) CompleteReadingSBOM(componentCount int) {
	if m.quiet || m.workflow == nil {
		return
	}
	m.workflow.CompleteTask(0, fmt.Sprintf("%d components loaded", componentCount))
}

// StartReadingAIBOMs marks the AIBOM reading step as running.
func (m *MergerUI) StartReadingAIBOMs(count int) {
	if m.quiet || m.workflow == nil {
		return
	}
	if count == 1 {
		m.workflow.StartTask(1, "")
	} else {
		m.workflow.StartTask(1, Dim.Render(fmt.Sprintf("%d files", count)))
	}
}

// UpdateReadingAIBOM updates progress for reading a specific AIBOM.
func (m *MergerUI) UpdateReadingAIBOM(index, total int, path string) {
	if m.quiet || m.workflow == nil {
		return
	}
	if total > 1 {
		m.workflow.StartTask(1, Dim.Render(fmt.Sprintf("[%d/%d] %s", index+1, total, path)))
	} else {
		m.workflow.StartTask(1, Dim.Render(path))
	}
}

// CompleteReadingAIBOMs marks AIBOM reading as complete.
func (m *MergerUI) CompleteReadingAIBOMs(count int) {
	if m.quiet || m.workflow == nil {
		return
	}
	if count == 1 {
		m.workflow.CompleteTask(1, "AIBOM loaded")
	} else {
		m.workflow.CompleteTask(1, fmt.Sprintf("%d AIBOMs loaded", count))
	}
}

// StartMerging marks the merge step as running.
func (m *MergerUI) StartMerging() {
	if m.quiet || m.workflow == nil {
		return
	}
	m.workflow.StartTask(2, "Combining components and metadata")
}

// CompleteMerging marks merging as complete.
func (m *MergerUI) CompleteMerging(sbomCount, aibomCount int) {
	if m.quiet || m.workflow == nil {
		return
	}
	total := sbomCount + aibomCount
	m.workflow.CompleteTask(2, fmt.Sprintf("%d total components", total))
}

// StartWriting marks the writing step as running.
func (m *MergerUI) StartWriting(path string) {
	if m.quiet || m.workflow == nil {
		return
	}
	m.workflow.StartTask(3, Dim.Render(path))
}

// CompleteWriting marks writing as complete.
func (m *MergerUI) CompleteWriting() {
	if m.quiet || m.workflow == nil {
		return
	}
	m.workflow.CompleteTask(3, "File written successfully")
}

// Stop stops the workflow.
func (m *MergerUI) Stop() {
	if m.workflow != nil {
		m.workflow.Stop()
	}
}

// PrintSummary displays the merge summary with styled output.
func (m *MergerUI) PrintSummary(result *merger.MergeResult, outputPath string, aibomCount int, deduplicate bool) {
	if m.quiet {
		return
	}

	// Stop workflow before printing summary.
	m.Stop()

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
	duration := time.Since(m.startTime)
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
	fmt.Fprintln(m.writer, "\n"+boxed)
}

// PrintError displays an error message.
func (m *MergerUI) PrintError(err error) {
	if m.quiet {
		return
	}

	// Stop workflow if running.
	m.Stop()

	var output strings.Builder
	output.WriteString(Error.Bold(true).Render("✗ Merge Failed"))
	output.WriteString("\n\n")
	output.WriteString(err.Error())

	boxed := ErrorBox.Render(output.String())
	fmt.Fprintln(m.writer, "\n"+boxed)
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
