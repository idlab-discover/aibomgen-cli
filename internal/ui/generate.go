package ui

import (
	"fmt"
	"io"
	"strings"
	"time"
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

// For model-id mode: process individual models.

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

// PrintNoBOMsWritten prints a message when no BOMs were written.
func (g *GenerateUI) PrintNoBOMsWritten() {
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
