package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/idlab-discover/aibomgen-cli/internal/ui"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/generator"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/scanner"
)

// scanCmd represents the scan command.
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan a directory for AI imports and generate AIBOMs",
	Long:  "Scan a directory or repository for AI-related imports (e.g., Hugging Face models) and generate AI-aware BOMs.",
	RunE:  runScan,
}

func runScan(cmd *cobra.Command, args []string) error {
	// Resolve effective log level (from config, env, or flag).
	quiet, err := quietFrom(viper.GetString("scan.log-level"))
	if err != nil {
		return err
	}

	// Resolve effective HF mode (from config, env, or flag).
	mode := strings.ToLower(strings.TrimSpace(viper.GetString("scan.hf-mode")))
	if mode == "" {
		mode = "online"
	}
	switch mode {
	case "online", "dummy":
		// ok.
	default:
		return fmt.Errorf("invalid --hf-mode %q (expected online|dummy)", mode)
	}

	inputPath := viper.GetString("scan.input")
	// Detect whether the user explicitly provided --input on the CLI (vs. using default).
	inputPathProvided := cmd.Flags().Changed("input")
	if inputPath == "" {
		inputPath = "."
	}

	// Disallow providing an input path when running in dummy HF mode — dummy mode.
	// uses built-in fixture data and does not consult the filesystem.
	if mode == "dummy" && inputPathProvided {
		return errors.New("--input cannot be used with --hf-mode=dummy")
	}

	// Validate the output format before doing any work.
	fmtChosen, err := outputFormat(viper.GetString("scan.format"))
	if err != nil {
		return err
	}
	outputDir := viper.GetString("scan.output")

	// Get HF settings.
	hfToken := viper.GetString("scan.hf-token")
	hfTimeout := viper.GetInt("scan.hf-timeout")
	if hfTimeout <= 0 {
		hfTimeout = 10
	}
	timeout := time.Duration(hfTimeout) * time.Second

	// Run the scan.
	var discoveredBOMs []generator.DiscoveredBOM
	if err := runScanDirectory(inputPath, mode, hfToken, timeout, quiet, &discoveredBOMs); err != nil {
		return err
	}

	return writeDiscovered(ui.NewGenerateUI(cmd.OutOrStdout(), quiet), discoveredBOMs, outputDir, fmtChosen, viper.GetString("scan.spec"))
}

func runScanDirectory(inputPath, mode, hfToken string, timeout time.Duration, quiet bool, results *[]generator.DiscoveredBOM) error {
	hasToken := strings.TrimSpace(hfToken) != ""
	absTarget, err := filepath.Abs(inputPath)
	if err != nil {
		return err
	}

	if mode == "dummy" {
		if !quiet {
			genUI := ui.NewGenerateUI(os.Stdout, quiet)
			genUI.LogStep("info", "Using dummy mode (no API calls)")
		}
		boms, err := generator.BuildDummyBOM()
		if err != nil {
			return err
		}
		*results = boms
		return nil
	}

	// Create workflow (only if not quiet).
	var workflow *ui.Workflow
	var scanTaskIdx, processTaskIdx, writeTaskIdx int

	if !quiet {
		workflow = ui.NewWorkflow(os.Stdout)
		scanTaskIdx = workflow.AddTask("Scanning for possible AI imports")
		processTaskIdx = workflow.AddTask("Processing possible models")
		writeTaskIdx = workflow.AddTask("Writing output")
		workflow.Start()
	}

	// Step 1: Scan.
	if workflow != nil {
		workflow.StartTask(scanTaskIdx, ui.Dim.Render(absTarget))
	}

	discoveries, err := scanner.Scan(absTarget)
	if err != nil {
		if workflow != nil {
			workflow.FailTask(scanTaskIdx, err.Error())
			workflow.Stop()
		}
		return err
	}

	if workflow != nil {
		workflow.CompleteTask(scanTaskIdx, fmt.Sprintf("found %d possible model(s)", len(discoveries)))
	}

	if len(discoveries) == 0 {
		if workflow != nil {
			workflow.SkipTask(processTaskIdx, "no models to process")
			workflow.SkipTask(writeTaskIdx, "no files to write")
			workflow.Stop()
		}
		*results = []generator.DiscoveredBOM{}
		return nil
	}

	// Step 2: Process models (fetch + build combined).
	onProgress, finish := trackProgress(workflow, processTaskIdx, writeTaskIdx, len(discoveries), hasToken)

	opts := generator.GenerateOptions{
		HFToken:          hfToken,
		Timeout:          timeout,
		OnProgress:       onProgress,
		SkipSecurityScan: viper.GetBool("scan.no-security-scan"),
	}

	boms, err := generator.BuildPerDiscovery(discoveries, opts)
	if err != nil {
		if workflow != nil {
			workflow.FailTask(processTaskIdx, err.Error())
			workflow.Stop()
		}
		return err
	}

	finish(len(boms))
	*results = boms
	return nil
}

func init() {
	scanCmd.Flags().StringP("input", "i", "", "Path to scan (defaults to current directory)")
	scanCmd.Flags().StringP("output", "o", "", "Output directory (default dist)")
	scanCmd.Flags().StringP("format", "f", "", "Output BOM format: json|xml (default json)")
	scanCmd.Flags().String("spec", "", "CycloneDX spec version for output (e.g., 1.5, 1.6, 1.7; default 1.7)")
	scanCmd.Flags().String("hf-mode", "", "Hugging Face metadata mode: online|dummy")
	scanCmd.Flags().Int("hf-timeout", 0, "Timeout in seconds per Hugging Face API request (default 10)")
	scanCmd.Flags().String("hf-token", "", "Hugging Face access token")
	scanCmd.Flags().String("log-level", "", "Log level: quiet|standard|debug")
	scanCmd.Flags().Bool("no-security-scan", false, "Skip fetching the HuggingFace security scan tree")

	// Bind all flags to viper for config file support.
	bindFlags(scanCmd, "scan")
}
