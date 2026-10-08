package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/huh/v2"
	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/internal/apperr"
	"github.com/idlab-discover/aibomgen-cli/internal/metadata"
	"github.com/idlab-discover/aibomgen-cli/internal/ui"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/bomio"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/vulnscan"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// vulnScanCmd represents the vuln-scan command.
var vulnScanCmd = &cobra.Command{
	Use:   "vuln-scan [file]",
	Short: "Scan an existing AIBOM for model/dataset security vulnerabilities",
	Long: `Fetch per-file security scan results from Hugging Face for every model and
dataset referenced in an existing AIBOM and display a vulnerability report.

Optionally enrich the AIBOM in-place with the discovered vulnerabilities using
the --enrich flag. A preview and confirmation prompt are shown before writing,
unless --yes is set.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runVulnScan,
}

func runVulnScan(cmd *cobra.Command, args []string) error {
	inputPath, err := requireInput(cmd, args, "vuln-scan")
	if err != nil {
		return err
	}

	asJSON := viper.GetBool("vuln-scan.json")
	quiet := verbosity < 0 || asJSON // --json: only JSON on stdout

	enrich := viper.GetBool("vuln-scan.enrich")
	yes := viper.GetBool("vuln-scan.yes")
	// Fail before any work if the confirmation can't be shown.
	if enrich && !yes && !ui.CanPrompt() {
		return errors.New("confirming the enrichment needs a terminal; pass --yes to apply without asking")
	}

	// ── Read AIBOM ──────────────────────────────────────────────────────────.
	bom, err := bomio.ReadBOM(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input BOM: %w", err)
	}

	outPath := viper.GetString("vuln-scan.output")
	if outPath == "" {
		outPath = inputPath
	}
	specVersion := strings.TrimSpace(viper.GetString("vuln-scan.spec"))

	w := cmd.OutOrStdout()

	// ── Workflow / progress ──────────────────────────────────────────────────.
	var workflow *ui.Workflow
	if !quiet {
		workflow = newWorkflow()
		workflow.AddTask("Scanning components")
		workflow.AddTask("Building report")
		workflow.Start()
	}

	// ── Run scan ─────────────────────────────────────────────────────────────.
	if workflow != nil {
		workflow.StartTask(0, "")
	}

	hf := hfOptions("vuln-scan")
	opts := vulnscan.Options{
		HFToken: hf.Token,
		Timeout: hf.Timeout,
		BaseURL: hf.BaseURL,
	}
	results := vulnscan.ScanBOM(bom, opts)

	if workflow != nil {
		workflow.CompleteTask(0, "")
		workflow.StartTask(1, "")
		workflow.CompleteTask(1, "")
		// Stop the spinner before printing the report or showing any interactive.
		// prompt – otherwise the background render goroutine corrupts the output.
		workflow.Stop()
	}

	// ── Print report ─────────────────────────────────────────────────────────.
	if asJSON {
		if err := writeJSON(w, vulnJSON(results)); err != nil {
			return err
		}
	} else {
		printVulnReport(w, results)
	}

	// ── Optional enrichment ───────────────────────────────────────────────────.
	if !enrich {
		return nil
	}

	// Count total vulnerabilities.
	total := 0
	for _, r := range results {
		total += len(r.Vulnerabilities)
	}
	// Interactive confirmation (only when there is something to add).
	if total > 0 && !yes {
		confirmed, err := confirmVulnEnrich(results)
		if errors.Is(err, apperr.ErrCancelled) {
			return apperr.ErrCancelled
		}
		if err != nil {
			return fmt.Errorf("confirmation error: %w", err)
		}
		if !confirmed {
			return apperr.ErrCancelled
		}
	}

	removed := vulnscan.ApplyToDOM(bom, results)
	if total == 0 && removed == 0 {
		if !quiet {
			fmt.Fprintf(w, "\n%s\n", ui.SuccessBox.Render(ui.GetCheckMark()+" No vulnerabilities found – AIBOM not modified."))
		}
		return nil
	}

	if err := bomio.WriteBOM(bom, outPath, specVersion); err != nil {
		return fmt.Errorf("failed to write enriched BOM: %w", err)
	}

	if !quiet {
		msg := fmt.Sprintf("Enriched BOM with %d vulnerabilities (%d previous findings replaced) → %s", total, removed, outPath)
		fmt.Fprintf(w, "\n%s\n", ui.SuccessBox.Render(ui.GetCheckMark()+" "+msg))
	}

	return nil
}

// printVulnReport writes a human-readable vulnerability report to w.
func printVulnReport(w io.Writer, results []vulnscan.ComponentScanResult) {
	fmt.Fprintln(w)

	hasAny := false
	for _, r := range results {
		hasAny = hasAny || len(r.Entries) > 0 || r.Err != nil
	}
	if !hasAny {
		fmt.Fprintln(w, ui.Muted.Render("No components found in AIBOM to scan."))
		return
	}

	for _, r := range results {
		modelLabel := ui.Bold.Render(r.ModelID)
		if r.Err != nil {
			fmt.Fprintf(w, "%s  %s\n", ui.Error.Render("✗"), modelLabel)
			fmt.Fprintf(w, "    %s\n\n", ui.Muted.Render(r.Err.Error()))
			continue
		}

		// Summary counts.
		unsafe, suspicious, caution, safe := 0, 0, 0, 0
		for _, e := range r.Entries {
			if e.SecurityFileStatus == nil {
				continue
			}
			switch strings.ToLower(e.SecurityFileStatus.Status) {
			case "unsafe":
				unsafe++
			case "suspicious":
				suspicious++
			case "caution":
				caution++
			default:
				safe++
			}
		}

		overallIcon, overallLabel := vulnStatusDisplay(metadata.OverallSecurityStatus(r.Entries))
		fmt.Fprintf(w, "%s  %s  %s\n",
			overallIcon,
			modelLabel,
			ui.Muted.Render(fmt.Sprintf("(%d files: %d unsafe, %d suspicious, %d caution, %d safe)",
				len(r.Entries), unsafe, suspicious, caution, safe)))
		fmt.Fprintf(w, "    Overall: %s\n", overallLabel)

		if len(r.Vulnerabilities) > 0 {
			fmt.Fprintf(w, "    Vulnerabilities: %s\n", ui.Warning.Render(fmt.Sprintf("%d file(s)", len(r.Vulnerabilities))))
			for _, v := range r.Vulnerabilities {
				src := ""
				if v.Source != nil {
					src = v.Source.URL
				}
				sev := string(vulnscan.HighestSeverity(v))
				fmt.Fprintf(w, "      • %s  %s  %s\n",
					renderVulnSeverity(sev, fmt.Sprintf("[%s]", strings.ToUpper(sev))),
					ui.Dim.Render(v.Description),
					ui.Muted.Render(src))
			}
		} else {
			fmt.Fprintf(w, "    Vulnerabilities: %s\n", ui.Success.Render("none"))
		}
		fmt.Fprintln(w)
	}
}

// vulnStatusDisplay returns an icon and styled label for an overall security status.
func vulnStatusDisplay(status string) (string, string) {
	switch status {
	case "unsafe", "suspicious":
		return ui.Error.Render("✗"), ui.Error.Render(status)
	case "caution":
		return ui.Warning.Render("⚠"), ui.Warning.Render(status)
	default:
		return ui.Success.Render("✓"), ui.Success.Render(status)
	}
}

func renderVulnSeverity(sev, text string) string {
	switch strings.ToLower(sev) {
	case "critical", "high":
		return ui.Error.Render(text)
	case "medium":
		return ui.Warning.Render(text)
	default:
		return ui.Muted.Render(text)
	}
}

// confirmVulnEnrich shows a preview box and asks the user to confirm enrichment.
func confirmVulnEnrich(results []vulnscan.ComponentScanResult) (bool, error) {
	var sb strings.Builder
	sb.WriteString(ui.Primary.Render("Vulnerability Enrichment Preview"))
	sb.WriteString("\n\n")
	sb.WriteString("The following vulnerabilities will be added to the AIBOM:\n\n")

	for _, r := range results {
		if len(r.Vulnerabilities) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("  %s  →  %s\n",
			ui.Bold.Render(r.ModelID),
			ui.Warning.Render(fmt.Sprintf("%d vulnerability entries", len(r.Vulnerabilities)))))
	}

	// The preview is part of the prompt, so it goes to stderr with the form.
	fmt.Fprintln(os.Stderr, ui.Box.Render(sb.String()))

	var confirm bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Apply vulnerabilities to AIBOM?").
				Description("This will add the discovered vulnerability data to the BOM and save it.").
				Value(&confirm).
				Affirmative("Yes").
				Negative("No"),
		),
	)
	if err := ui.RunForm(form); err != nil {
		return false, err
	}
	return confirm, nil
}

func init() {
	vulnScanCmd.Flags().StringP("input", "i", "", "AIBOM file, instead of the argument")
	vulnScanCmd.Flags().StringP("output", "o", "", "Output file with --enrich (default: overwrite the input); .xml writes XML, anything else JSON")
	vulnScanCmd.Flags().String("spec", "", "CycloneDX spec version for output (default: same as input)")

	vulnScanCmd.Flags().Bool("enrich", false, "Inject discovered vulnerabilities back into the AIBOM")
	vulnScanCmd.Flags().BoolP("yes", "y", false, "Apply without preview or confirmation (only with --enrich)")

	vulnScanCmd.Flags().Bool("json", false, "Print the scan results as JSON")
	addHFFlags(vulnScanCmd)

	// Bind to viper.
	bindFlags(vulnScanCmd, "vuln-scan")
}

// vulnResultJSON is the --json view of one scanned component: the raw tree
// entries are left out and the error becomes a string.
type vulnResultJSON struct {
	Ref             string              `json:"ref"`
	Model           string              `json:"model"`
	Vulnerabilities []cdx.Vulnerability `json:"vulnerabilities"`
	Error           string              `json:"error,omitempty"`
}

func vulnJSON(results []vulnscan.ComponentScanResult) []vulnResultJSON {
	out := make([]vulnResultJSON, 0, len(results))
	for _, r := range results {
		v := vulnResultJSON{Ref: r.ComponentRef, Model: r.ModelID, Vulnerabilities: r.Vulnerabilities}
		if v.Vulnerabilities == nil {
			v.Vulnerabilities = []cdx.Vulnerability{}
		}
		if r.Err != nil {
			v.Error = r.Err.Error()
		}
		out = append(out, v)
	}
	return out
}
