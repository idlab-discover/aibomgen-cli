package ui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// GenerateUI provides a rich UI for the generate command.
type GenerateUI struct {
	writer       io.Writer
	quiet        bool
	workflow     *Workflow
	startTime    time.Time
	currentModel string
}

// NewGenerateUI creates a new UI handler for the generate command.
func NewGenerateUI(w io.Writer, quiet bool) *GenerateUI {
	return &GenerateUI{
		writer:    w,
		quiet:     quiet,
		startTime: time.Now(), // Initialize start time immediately
	}
}

// StartWorkflow initializes and displays the workflow for generation.
func (g *GenerateUI) StartWorkflow(modelIDs []string, scanMode bool) {
	if g.quiet {
		return
	}

	g.startTime = time.Now()

	if scanMode {
		g.workflow = NewWorkflow(g.writer, "Generating AIBOM")
		g.workflow.AddTask("Scanning directory for AI imports")
		g.workflow.AddTask("Fetching metadata from Hugging Face")
		g.workflow.AddTask("Building AIBOM components")
		g.workflow.AddTask("Writing output files")
	} else {
		g.workflow = NewWorkflow(g.writer, "Generating AIBOM")
		for _, id := range modelIDs {
			g.workflow.AddTask(fmt.Sprintf("Processing %s", id))
		}
		g.workflow.AddTask("Writing output files")
	}

	g.workflow.Start()
}

// StartScanning marks the scanning step as running.
func (g *GenerateUI) StartScanning(path string) {
	if g.quiet || g.workflow == nil {
		return
	}
	g.workflow.StartTask(0, Dim.Render(path))
}

// CompleteScanningWithResults marks scanning as complete with results.
func (g *GenerateUI) CompleteScanningWithResults(count int) {
	if g.quiet || g.workflow == nil {
		return
	}
	g.workflow.CompleteTask(0, fmt.Sprintf("found %d model(s)", count))
}

// StartFetching marks the fetching step as running.
func (g *GenerateUI) StartFetching(modelID string) {
	if g.quiet || g.workflow == nil {
		return
	}
	g.currentModel = modelID
	g.workflow.StartTask(1, Dim.Render(modelID))
}

// UpdateFetchingStatus updates the message during fetching.
func (g *GenerateUI) UpdateFetchingStatus(message string) {
	if g.quiet || g.workflow == nil {
		return
	}
	g.workflow.UpdateMessage(1, Dim.Render(message))
}

// CompleteFetching marks fetching as complete.
func (g *GenerateUI) CompleteFetching() {
	if g.quiet || g.workflow == nil {
		return
	}
	g.workflow.CompleteTask(1, "metadata retrieved")
}

// StartBuilding marks the building step as running.
func (g *GenerateUI) StartBuilding() {
	if g.quiet || g.workflow == nil {
		return
	}
	g.workflow.StartTask(2, "")
}

// CompleteBuilding marks building as complete.
func (g *GenerateUI) CompleteBuilding(componentCount int) {
	if g.quiet || g.workflow == nil {
		return
	}
	g.workflow.CompleteTask(2, fmt.Sprintf("%d component(s)", componentCount))
}

// StartWriting marks the writing step as running.
func (g *GenerateUI) StartWriting() {
	if g.quiet || g.workflow == nil {
		return
	}
	taskIdx := 3
	if g.workflow != nil && len(g.workflow.tasks) > 0 {
		taskIdx = len(g.workflow.tasks) - 1
	}
	g.workflow.StartTask(taskIdx, "")
}

// CompleteWriting marks writing as complete.
func (g *GenerateUI) CompleteWriting(outputDir string, count int) {
	if g.quiet || g.workflow == nil {
		return
	}
	taskIdx := 3
	if g.workflow != nil && len(g.workflow.tasks) > 0 {
		taskIdx = len(g.workflow.tasks) - 1
	}
	g.workflow.CompleteTask(taskIdx, fmt.Sprintf("%d file(s) → %s", count, outputDir))
}

// For model-id mode: process individual models.

// StartModelProcessing marks a model as being processed (for model-id mode).
func (g *GenerateUI) StartModelProcessing(idx int, modelID string) {
	if g.quiet || g.workflow == nil {
		return
	}
	g.currentModel = modelID
	g.workflow.StartTask(idx, "fetching metadata...")
}

// UpdateModelProcessing updates the status of a model being processed.
func (g *GenerateUI) UpdateModelProcessing(idx int, status string) {
	if g.quiet || g.workflow == nil {
		return
	}
	g.workflow.UpdateMessage(idx, Dim.Render(status))
}

// CompleteModelProcessing marks a model as processed.
func (g *GenerateUI) CompleteModelProcessing(idx int, details string) {
	if g.quiet || g.workflow == nil {
		return
	}
	g.workflow.CompleteTask(idx, details)
}

// FailModelProcessing marks a model as failed.
func (g *GenerateUI) FailModelProcessing(idx int, err string) {
	if g.quiet || g.workflow == nil {
		return
	}
	g.workflow.FailTask(idx, err)
}

// FinishWorkflow completes the workflow display.
func (g *GenerateUI) FinishWorkflow() {
	if g.quiet || g.workflow == nil {
		return
	}
	g.workflow.Stop()
}

// PrintSummary prints a final summary.
func (g *GenerateUI) PrintSummary(filesWritten int, outputDir, format string) {
	if g.quiet {
		return
	}

	elapsed := time.Since(g.startTime)

	fmt.Fprintln(g.writer)

	// Summary box.
	var summary strings.Builder
	summary.WriteString(Success.Bold(true).Render("Generation Complete"))
	summary.WriteString("\n\n")
	summary.WriteString(FormatKeyValue("Files written", fmt.Sprintf("%d", filesWritten)))
	summary.WriteString("\n")
	summary.WriteString(FormatKeyValue("Output directory", outputDir))
	summary.WriteString("\n")
	summary.WriteString(FormatKeyValue("Format", format))
	summary.WriteString("\n")
	summary.WriteString(FormatKeyValue("Duration", elapsed.Round(time.Millisecond).String()))

	fmt.Fprintln(g.writer, SuccessBox.Render(summary.String()))
}

// PrintNoModelsFound prints a message when no models are found.
func (g *GenerateUI) PrintNoModelsFound() {
	if g.quiet {
		return
	}

	msg := "No BOMs written."
	fmt.Fprintln(g.writer, Error.Render(GetCrossMark()+" "+msg))
}

// LogStep prints a simple log message (non-workflow mode).
func (g *GenerateUI) LogStep(icon, message string) {
	if g.quiet {
		return
	}

	var iconStyled string
	switch icon {
	case "success":
		iconStyled = GetCheckMark()
	case "error":
		iconStyled = GetCrossMark()
	case "warning":
		iconStyled = GetWarnMark()
	case "info":
		iconStyled = GetInfoMark()
	default:
		iconStyled = Secondary.Render("→")
	}

	fmt.Fprintf(g.writer, "%s %s\n", iconStyled, message)
}

// LogModelStep logs a step for a specific model.
func (g *GenerateUI) LogModelStep(modelID, action, detail string) {
	if g.quiet {
		return
	}

	modelStyled := Highlight.Render(modelID)
	actionStyled := action
	if detail != "" {
		actionStyled += " " + Dim.Render(detail)
	}

	fmt.Fprintf(g.writer, "%s %s %s\n", Secondary.Render("→"), modelStyled, actionStyled)
}

// PrintBanner prints the application banner.
func PrintBanner(w io.Writer) {
	banner := `
  /$$$$$$  /$$$$$$ /$$$$$$$            /$$      /$$  /$$$$$$                                        /$$ /$$
 /$$__  $$|_  $$_/| $$__  $$          | $$$    /$$$ /$$__  $$                                      | $$|__/
| $$  \ $$  | $$  | $$  \ $$  /$$$$$$ | $$$$  /$$$$| $$  \__/  /$$$$$$  /$$$$$$$           /$$$$$$$| $$ /$$
| $$$$$$$$  | $$  | $$$$$$$  /$$__  $$| $$ $$/$$ $$| $$ /$$$$ /$$__  $$| $$__  $$ /$$$$$$ /$$_____/| $$| $$
| $$__  $$  | $$  | $$__  $$| $$  \ $$| $$  $$$| $$| $$|_  $$| $$$$$$$$| $$  \ $$|______/| $$      | $$| $$
| $$  | $$  | $$  | $$  \ $$| $$  | $$| $$\  $ | $$| $$  \ $$| $$_____/| $$  | $$        | $$      | $$| $$
| $$  | $$ /$$$$$$| $$$$$$$/|  $$$$$$/| $$ \/  | $$|  $$$$$$/|  $$$$$$$| $$  | $$        |  $$$$$$$| $$| $$
|__/  |__/|______/|_______/  \______/ |__/     |__/ \______/  \_______/|__/  |__/         \_______/|__/|__/
`
	styled := lipgloss.NewStyle().Foreground(ColorSuccess).Render(banner)
	fmt.Fprintln(w, styled)
}
