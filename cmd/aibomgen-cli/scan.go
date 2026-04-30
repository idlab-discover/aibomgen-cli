package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/idlab-discover/aibomgen-cli/internal/apperr"
	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
	"github.com/idlab-discover/aibomgen-cli/internal/ui"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/bomio"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/generator"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/scanner"
)

var (
	scanPath         string
	scanOutput       string
	scanOutputFormat string
	scanSpecVersion  string

	// hfMode controls whether metadata is fetched from Hugging Face.
	// Supported values: online|dummy.
	scanHfMode       string
	scanHfTimeoutSec int
	scanHfToken      string

	// Logging is controlled via scanLogLevel.
	scanLogLevel string

	// scanNoSecurityScan disables the HF tree security scan fetch.
	scanNoSecurityScan bool
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
	level := strings.ToLower(strings.TrimSpace(viper.GetString("scan.log-level")))
	if level == "" {
		level = "standard"
	}
	switch level {
	case "quiet", "standard", "debug":
		// ok.
	default:
		return apperr.Userf("invalid --log-level %q (expected quiet|standard|debug)", level)
	}

	quiet := level == "quiet"

	// Resolve effective HF mode (from config, env, or flag).
	mode := strings.ToLower(strings.TrimSpace(viper.GetString("scan.hf-mode")))
	if mode == "" {
		mode = "online"
	}
	switch mode {
	case "online", "dummy":
		// ok.
	default:
		return apperr.Userf("invalid --hf-mode %q (expected online|dummy)", mode)
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
		return apperr.User("--input cannot be used with --hf-mode=dummy")
	}

	// Get format from viper.
	outputFormat := viper.GetString("scan.format")
	if outputFormat == "" {
		outputFormat = "auto"
	}

	specVersion := viper.GetString("scan.spec")
	outputPath := viper.GetString("scan.output")

	// Fail fast on format/extension mismatch.
	if outputPath != "" && outputFormat != "" && outputFormat != "auto" {
		ext := filepath.Ext(outputPath)
		if outputFormat == "xml" && ext == ".json" {
			return apperr.Userf("output path extension %q does not match format %q", ext, outputFormat)
		}
		if outputFormat == "json" && ext == ".xml" {
			return apperr.Userf("output path extension %q does not match format %q", ext, outputFormat)
		}
	}

	// Get HF settings.
	hfToken := viper.GetString("scan.hf-token")
	hfTimeout := viper.GetInt("scan.hf-timeout")
	if hfTimeout <= 0 {
		hfTimeout = 10
	}
	timeout := time.Duration(hfTimeout) * time.Second

	// Run the scan.
	var discoveredBOMs []generator.DiscoveredBOM
	err := runScanDirectory(inputPath, mode, hfToken, timeout, quiet, &discoveredBOMs)
	if err != nil {
		return err
	}

	// Determine output settings.
	output := viper.GetString("scan.output")
	if output == "" {
		if outputFormat == "xml" {
			output = "dist/aibom.xml"
		} else {
			output = "dist/aibom.json"
		}
	}

	fmtChosen := outputFormat
	if fmtChosen == "auto" || fmtChosen == "" {
		ext := filepath.Ext(output)
		if ext == ".xml" {
			fmtChosen = "xml"
		} else {
			fmtChosen = "json"
		}
	}

	outputDir := filepath.Dir(output)
	if outputDir == "" {
		outputDir = "."
	}
	outputDir = filepath.Clean(outputDir)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}

	fileExt := ".json"
	if fmtChosen == "xml" {
		fileExt = ".xml"
	}

	// Write output files.
	written, err := bomio.WriteOutputFiles(discoveredBOMs, outputDir, fileExt, fmtChosen, specVersion)
	if err != nil {
		return err
	}

	// Print summary.
	if len(written) == 0 {
		genUI := ui.NewGenerateUI(cmd.OutOrStdout(), quiet)
		genUI.PrintNoModelsFound()
		return nil
	}

	genUI := ui.NewGenerateUI(cmd.OutOrStdout(), quiet)
	genUI.PrintSummary(len(written), outputDir, fmtChosen)
	return nil
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

	// Track per-model outcome for the final summary (same pattern as generate command).
	pendingModels := make(map[string]*modelTracker)
	var modelOrder []string

	// Create workflow (only if not quiet).
	var workflow *ui.Workflow
	var scanTaskIdx, processTaskIdx, writeTaskIdx int

	if !quiet {
		workflow = ui.NewWorkflow(os.Stdout, "")
		scanTaskIdx = workflow.AddTask("Scanning for possible AI imports")
		processTaskIdx = workflow.AddTask("Processing possible models")
		writeTaskIdx = workflow.AddTask("Writing output")
		workflow.Start()
	}

	// Step 1: Scan.
	if !quiet && workflow != nil {
		workflow.StartTask(scanTaskIdx, ui.Dim.Render(absTarget))
	}

	discoveries, err := scanner.Scan(absTarget)
	if err != nil {
		if !quiet && workflow != nil {
			workflow.FailTask(scanTaskIdx, err.Error())
			workflow.Stop()
		}
		return err
	}

	if !quiet && workflow != nil {
		workflow.CompleteTask(scanTaskIdx, fmt.Sprintf("found %d possible model(s)", len(discoveries)))
	}

	if len(discoveries) == 0 {
		if !quiet && workflow != nil {
			workflow.SkipTask(processTaskIdx, "no models to process")
			workflow.SkipTask(writeTaskIdx, "no files to write")
			workflow.Stop()
		}
		*results = []generator.DiscoveredBOM{}
		return nil
	}

	totalModels := len(discoveries)
	modelsCompleted := 0

	// Step 2: Process models (fetch + build combined).
	if !quiet && workflow != nil {
		workflow.StartTask(processTaskIdx, ui.Dim.Render(fmt.Sprintf("0/%d", totalModels)))
	}

	onProgress := func(evt generator.ProgressEvent) {
		if quiet || workflow == nil {
			return
		}
		// Ensure a tracker exists for this model (EventFetchStart arrives first).
		if _, ok := pendingModels[evt.ModelID]; !ok {
			pendingModels[evt.ModelID] = &modelTracker{}
			modelOrder = append(modelOrder, evt.ModelID)
		}
		switch evt.Type {
		case generator.EventFetchStart:
			workflow.UpdateMessage(processTaskIdx, ui.Dim.Render(fmt.Sprintf("%d/%d: %s (fetching)", modelsCompleted, totalModels, evt.ModelID)))
		case generator.EventFetchAPIComplete:
			pendingModels[evt.ModelID].apiOK = true
		case generator.EventBuildStart:
			workflow.UpdateMessage(processTaskIdx, ui.Dim.Render(fmt.Sprintf("%d/%d: %s (building)", modelsCompleted, totalModels, evt.ModelID)))
		case generator.EventDatasetStart:
			workflow.UpdateMessage(processTaskIdx, ui.Dim.Render(fmt.Sprintf("%d/%d: %s → %s", modelsCompleted, totalModels, evt.ModelID, evt.Message)))
		case generator.EventDatasetComplete:
			pendingModels[evt.ModelID].datasetResults = append(pendingModels[evt.ModelID].datasetResults, datasetResult{id: evt.Message})
		case generator.EventDatasetError:
			pendingModels[evt.ModelID].datasetResults = append(pendingModels[evt.ModelID].datasetResults, datasetResult{id: evt.Message, err: evt.Error})
		case generator.EventModelComplete:
			t := pendingModels[evt.ModelID]
			t.complete = true
			modelsCompleted++
			if modelsCompleted < totalModels {
				workflow.UpdateMessage(processTaskIdx, ui.Dim.Render(fmt.Sprintf("%d/%d complete", modelsCompleted, totalModels)))
			}
		case generator.EventError:
			if evt.Message != "BOM build failed" {
				t := pendingModels[evt.ModelID]
				if fetcher.IsNotFound(evt.Error) {
					t.notFound = true
				} else if fetcher.IsUnauthorized(evt.Error) && !t.apiOK {
					// 401/403 before the model API succeeded = model is private or non-existent.
					// HF Hub returns 401 for non-existent repos too, so treat this like 404.
					t.notFound = true
				} else {
					t.fetchErr = true
					if t.fetchErrVal == nil {
						t.fetchErrVal = evt.Error
					}
				}
			}
		}
	}

	opts := generator.GenerateOptions{
		HFToken:          hfToken,
		Timeout:          timeout,
		OnProgress:       onProgress,
		SkipSecurityScan: scanNoSecurityScan,
	}

	boms, err := generator.BuildPerDiscovery(discoveries, opts)
	if err != nil {
		if !quiet && workflow != nil {
			workflow.FailTask(processTaskIdx, err.Error())
			workflow.Stop()
		}
		return err
	}

	if !quiet && workflow != nil {
		workflow.CompleteTask(processTaskIdx, fmt.Sprintf("%d possible model(s)", len(discoveries)))
		workflow.StartTask(writeTaskIdx, "")
		workflow.CompleteTask(writeTaskIdx, fmt.Sprintf("%d file(s)", len(boms)))
		workflow.Stop()

		// Print individual model results after workflow completes.
		fmt.Println()
		for _, id := range modelOrder {
			printModelResult(id, pendingModels[id], hasToken)
		}
	}

	*results = boms
	return nil
}

func init() {
	scanCmd.Flags().StringVarP(&scanPath, "input", "i", "", "Path to scan (defaults to current directory)")
	scanCmd.Flags().StringVarP(&scanOutput, "output", "o", "", "Output file path (directory is used)")
	scanCmd.Flags().StringVarP(&scanOutputFormat, "format", "f", "", "Output BOM format: json|xml|auto")
	scanCmd.Flags().StringVar(&scanSpecVersion, "spec", "", "CycloneDX spec version for output (e.g., 1.4, 1.5, 1.6)")
	scanCmd.Flags().StringVar(&scanHfMode, "hf-mode", "", "Hugging Face metadata mode: online|dummy")
	scanCmd.Flags().IntVar(&scanHfTimeoutSec, "hf-timeout", 0, "Timeout in seconds per Hugging Face API request (default 10)")
	scanCmd.Flags().StringVar(&scanHfToken, "hf-token", "", "Hugging Face access token")
	scanCmd.Flags().StringVar(&scanLogLevel, "log-level", "", "Log level: quiet|standard|debug")
	scanCmd.Flags().BoolVar(&scanNoSecurityScan, "no-security-scan", false, "Skip fetching the HuggingFace security scan tree")

	// Bind all flags to viper for config file support.
	viper.BindPFlag("scan.input", scanCmd.Flags().Lookup("input"))
	viper.BindPFlag("scan.output", scanCmd.Flags().Lookup("output"))
	viper.BindPFlag("scan.format", scanCmd.Flags().Lookup("format"))
	viper.BindPFlag("scan.spec", scanCmd.Flags().Lookup("spec"))
	viper.BindPFlag("scan.hf-mode", scanCmd.Flags().Lookup("hf-mode"))
	viper.BindPFlag("scan.hf-timeout", scanCmd.Flags().Lookup("hf-timeout"))
	viper.BindPFlag("scan.hf-token", scanCmd.Flags().Lookup("hf-token"))
	viper.BindPFlag("scan.log-level", scanCmd.Flags().Lookup("log-level"))
}
