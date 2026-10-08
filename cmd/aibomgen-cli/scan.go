package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/idlab-discover/aibomgen-cli/internal/ui"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/generator"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/scanner"
)

// scanCmd represents the scan command.
var scanCmd = &cobra.Command{
	Use:   "scan [dir]",
	Short: "Scan a directory for AI imports and generate AIBOMs",
	Long:  "Scan a directory or repository for AI-related imports (e.g., Hugging Face models) and generate AI-aware BOMs.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runScan,
}

func runScan(cmd *cobra.Command, args []string) error {
	quiet := verbosity < 0

	mode, err := hfMode("scan")
	if err != nil {
		return err
	}

	inputPath, err := inputFrom(cmd, args, "scan")
	if err != nil {
		return err
	}
	// Dummy mode uses built-in fixture data and does not read the filesystem.
	if mode == "dummy" && inputPath != "" {
		return errors.New("cannot combine an input directory with --hf-mode=dummy")
	}
	if inputPath == "" {
		inputPath = "."
	}

	// Validate the output format before doing any work.
	fmtChosen, err := outputFormat(viper.GetString("scan.format"))
	if err != nil {
		return err
	}
	outputDir := viper.GetString("scan.output")

	// Run the scan.
	var discoveredBOMs []generator.DiscoveredBOM
	if err := runScanDirectory(inputPath, mode, hfOptions("scan"), quiet, &discoveredBOMs); err != nil {
		return err
	}

	return writeDiscovered(ui.NewGenerateUI(cmd.OutOrStdout(), quiet), discoveredBOMs, outputDir, fmtChosen, viper.GetString("scan.spec"))
}

func runScanDirectory(inputPath, mode string, hf hfSettings, quiet bool, results *[]generator.DiscoveredBOM) error {
	hasToken := strings.TrimSpace(hf.Token) != ""
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
		workflow = newWorkflow()
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
		HFToken:          hf.Token,
		BaseURL:          hf.BaseURL,
		Timeout:          hf.Timeout,
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
	scanCmd.Flags().StringP("input", "i", "", "Directory to scan, instead of the argument (default current directory)")
	scanCmd.Flags().StringP("output", "o", "dist", "Output directory; one file per model")
	scanCmd.Flags().StringP("format", "f", "json", "Output BOM format: json|xml")
	scanCmd.Flags().String("spec", "", "CycloneDX spec version for output, e.g. 1.6 (default latest)")
	scanCmd.Flags().Bool("no-security-scan", false, "Skip fetching the Hugging Face security scan tree")
	addHFFlags(scanCmd)
	addHFModeFlag(scanCmd)

	bindFlags(scanCmd, "scan")
}
