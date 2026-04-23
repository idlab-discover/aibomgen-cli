package cmd

import (
	"fmt"
	"io"
	"strings"
	"time"

	"charm.land/huh/v2"
	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/internal/apperr"
	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
	"github.com/idlab-discover/aibomgen-cli/internal/ui"
	"github.com/idlab-discover/aibomgen-cli/internal/vulnscan"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/bomio"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// vulnScanCmd represents the vuln-scan command.
var vulnScanCmd = &cobra.Command{
	Use:   "vuln-scan",
	Short: "Scan an existing AIBOM for model/dataset security vulnerabilities",
	Long: `Fetch per-file security scan results from Hugging Face for every model and
dataset referenced in an existing AIBOM and display a vulnerability report.

Optionally enrich the AIBOM in-place with the discovered vulnerabilities using
the --enrich flag. When --interactive is also set (default when --enrich is
active) a preview and confirmation prompt are shown before writing.`,
	RunE: runVulnScan,
}

func runVulnScan(cmd *cobra.Command, _ []string) error {
	inputPath := viper.GetString("vuln-scan.input")
	if inputPath == "" {
		return apperr.User("--input is required")
	}

	inputFormat := viper.GetString("vuln-scan.format")
	if inputFormat == "" {
		inputFormat = "auto"
	}

	logLevel := strings.ToLower(strings.TrimSpace(viper.GetString("vuln-scan.log-level")))
	if logLevel == "" {
		logLevel = "standard"
	}
	switch logLevel {
	case "quiet", "standard", "debug":
	default:
		return apperr.Userf("invalid --log-level %q (expected quiet|standard|debug)", logLevel)
	}

	enrich := viper.GetBool("vuln-scan.enrich")
	interactive := viper.GetBool("vuln-scan.interactive")
	noPreview := viper.GetBool("vuln-scan.no-preview")
	timeout := viper.GetInt("vuln-scan.hf-timeout")
	if timeout <= 0 {
		timeout = 15
	}

	// ── Read AIBOM ──────────────────────────────────────────────────────────.
	bom, err := bomio.ReadBOM(inputPath, inputFormat)
	if err != nil {
		return fmt.Errorf("failed to read input BOM: %w", err)
	}

	outPath := viper.GetString("vuln-scan.output")
	if outPath == "" {
		outPath = inputPath
	}
	outputFormat := viper.GetString("vuln-scan.output-format")
	if outputFormat == "" {
		outputFormat = "auto"
	}
	specVersion := strings.TrimSpace(viper.GetString("vuln-scan.spec"))

	w := cmd.OutOrStdout()

	// ── Workflow / progress ──────────────────────────────────────────────────.
	var workflow *ui.Workflow
	if logLevel != "quiet" {
		workflow = ui.NewWorkflow(w, "Vulnerability Scan")
		workflow.AddTask("Scanning components")
		workflow.AddTask("Building report")
		workflow.Start()
	}

	// ── Run scan ─────────────────────────────────────────────────────────────.
	if workflow != nil {
		workflow.StartTask(0, "")
	}

	opts := vulnscan.Options{
		HFToken: viper.GetString("vuln-scan.hf-token"),
		Timeout: time.Duration(timeout) * time.Second,
		BaseURL: viper.GetString("vuln-scan.hf-base-url"),
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
	printVulnReport(w, results)

	// ── Optional enrichment ───────────────────────────────────────────────────.
	if !enrich {
		return nil
	}

	// Count total vulnerabilities.
	total := 0
	for _, r := range results {
		total += len(r.Vulnerabilities)
	}
	if total == 0 {
		if logLevel != "quiet" {
			fmt.Fprintf(w, "\n%s\n", ui.SuccessBox.Render(ui.GetCheckMark()+" No vulnerabilities found – AIBOM not modified."))
		}
		return nil
	}

	// Interactive confirmation.
	if interactive && !noPreview {
		confirmed, err := confirmVulnEnrich(results)
		if err != nil {
			return fmt.Errorf("confirmation error: %w", err)
		}
		if !confirmed {
			return apperr.ErrCancelled
		}
	}

	vulnscan.ApplyToDOM(bom, results)

	if err := bomio.WriteBOM(bom, outPath, outputFormat, specVersion); err != nil {
		return fmt.Errorf("failed to write enriched BOM: %w", err)
	}

	if logLevel != "quiet" {
		msg := fmt.Sprintf("Enriched BOM with %d vulnerabilities → %s", total, outPath)
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
		unsafe, caution, safe := 0, 0, 0
		for _, e := range r.Entries {
			if e.SecurityFileStatus == nil {
				continue
			}
			switch e.SecurityFileStatus.Status {
			case "unsafe":
				unsafe++
			case "caution":
				caution++
			default:
				safe++
			}
		}

		overallIcon, overallLabel := vulnStatusDisplay(r.Entries)
		fmt.Fprintf(w, "%s  %s  %s\n",
			overallIcon,
			modelLabel,
			ui.Muted.Render(fmt.Sprintf("(%d files: %d unsafe, %d caution, %d safe)",
				len(r.Entries), unsafe, caution, safe)))
		fmt.Fprintf(w, "    Overall: %s\n", overallLabel)

		if len(r.Vulnerabilities) > 0 {
			fmt.Fprintf(w, "    Vulnerabilities: %s\n", ui.Warning.Render(fmt.Sprintf("%d file(s)", len(r.Vulnerabilities))))
			for _, v := range r.Vulnerabilities {
				src := ""
				if v.Source != nil {
					src = v.Source.URL
				}
				sev := highestSeverity(v)
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

// vulnStatusDisplay returns an icon and styled label for a set of entries.
func vulnStatusDisplay(entries []fetcher.SecurityFileEntry) (string, string) {
	unsafe := false
	caution := false
	for _, e := range entries {
		if e.SecurityFileStatus == nil {
			continue
		}
		switch e.SecurityFileStatus.Status {
		case "unsafe":
			unsafe = true
		case "caution":
			caution = true
		}
	}
	switch {
	case unsafe:
		return ui.Error.Render("✗"), ui.Error.Render("unsafe")
	case caution:
		return ui.Warning.Render("⚠"), ui.Warning.Render("caution")
	default:
		return ui.Success.Render("✓"), ui.Success.Render("safe")
	}
}

// highestSeverity returns the highest severity string among a vulnerability's ratings.
func highestSeverity(v cdx.Vulnerability) string {
	order := map[cdx.Severity]int{
		cdx.SeverityCritical: 5,
		cdx.SeverityHigh:     4,
		cdx.SeverityMedium:   3,
		cdx.SeverityLow:      2,
		cdx.SeverityInfo:     1,
		cdx.SeverityNone:     0,
	}
	best := ""
	bestRank := -1
	if v.Ratings != nil {
		for _, r := range *v.Ratings {
			if rank, ok := order[r.Severity]; ok && rank > bestRank {
				bestRank = rank
				best = string(r.Severity)
			}
		}
	}
	if best == "" {
		return "unknown"
	}
	return best
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

	fmt.Println(ui.Box.Render(sb.String()))

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
	if err := form.Run(); err != nil {
		return false, err
	}
	return confirm, nil
}

// ── Flag vars ─────────────────────────────────────────────────────────────────.

var (
	vulnScanInput        string
	vulnScanInputFormat  string
	vulnScanOutput       string
	vulnScanOutputFormat string
	vulnScanSpecVersion  string
	vulnScanEnrich       bool
	vulnScanInteractive  bool
	vulnScanNoPreview    bool
	vulnScanLogLevel     string
	vulnScanHFToken      string
	vulnScanHFBaseURL    string
	vulnScanHFTimeout    int
)

func init() {
	vulnScanCmd.Flags().StringVarP(&vulnScanInput, "input", "i", "", "Path to existing AIBOM (required)")
	vulnScanCmd.Flags().StringVarP(&vulnScanOutput, "output", "o", "", "Output path when --enrich is set (default: overwrite input)")
	vulnScanCmd.Flags().StringVarP(&vulnScanInputFormat, "format", "f", "", "Input BOM format: json|xml|auto")
	vulnScanCmd.Flags().StringVar(&vulnScanOutputFormat, "output-format", "", "Output BOM format: json|xml|auto")
	vulnScanCmd.Flags().StringVar(&vulnScanSpecVersion, "spec", "", "CycloneDX spec version for output")

	vulnScanCmd.Flags().BoolVar(&vulnScanEnrich, "enrich", false, "Inject discovered vulnerabilities back into the AIBOM")
	vulnScanCmd.Flags().BoolVar(&vulnScanInteractive, "interactive", true, "Show confirmation prompt before saving (only with --enrich)")
	vulnScanCmd.Flags().BoolVar(&vulnScanNoPreview, "no-preview", false, "Skip preview prompt (only with --enrich)")

	vulnScanCmd.Flags().StringVar(&vulnScanLogLevel, "log-level", "", "Log level: quiet|standard|debug")
	vulnScanCmd.Flags().StringVar(&vulnScanHFToken, "hf-token", "", "Hugging Face API token")
	vulnScanCmd.Flags().StringVar(&vulnScanHFBaseURL, "hf-base-url", "", "Hugging Face base URL override")
	vulnScanCmd.Flags().IntVar(&vulnScanHFTimeout, "hf-timeout", 15, "Hugging Face API timeout in seconds")

	// Bind to viper.
	viper.BindPFlag("vuln-scan.input", vulnScanCmd.Flags().Lookup("input"))
	viper.BindPFlag("vuln-scan.output", vulnScanCmd.Flags().Lookup("output"))
	viper.BindPFlag("vuln-scan.format", vulnScanCmd.Flags().Lookup("format"))
	viper.BindPFlag("vuln-scan.output-format", vulnScanCmd.Flags().Lookup("output-format"))
	viper.BindPFlag("vuln-scan.spec", vulnScanCmd.Flags().Lookup("spec"))
	viper.BindPFlag("vuln-scan.enrich", vulnScanCmd.Flags().Lookup("enrich"))
	viper.BindPFlag("vuln-scan.interactive", vulnScanCmd.Flags().Lookup("interactive"))
	viper.BindPFlag("vuln-scan.no-preview", vulnScanCmd.Flags().Lookup("no-preview"))
	viper.BindPFlag("vuln-scan.log-level", vulnScanCmd.Flags().Lookup("log-level"))
	viper.BindPFlag("vuln-scan.hf-token", vulnScanCmd.Flags().Lookup("hf-token"))
	viper.BindPFlag("vuln-scan.hf-base-url", vulnScanCmd.Flags().Lookup("hf-base-url"))
	viper.BindPFlag("vuln-scan.hf-timeout", vulnScanCmd.Flags().Lookup("hf-timeout"))
}
