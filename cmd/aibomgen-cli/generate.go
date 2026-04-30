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
)

var (
	generateOutput       string
	generateOutputFormat string
	generateSpecVersion  string
	generateModelIDs     []string

	// hfMode controls whether metadata is fetched from Hugging Face.
	// Supported values: online|dummy.
	hfMode    string
	hfTimeout int
	hfToken   string

	// Logging is controlled via generateLogLevel.
	generateLogLevel string

	// interactive enables the interactive model selector.
	interactive bool

	// noSecurityScan disables the HF tree security scan fetch.
	noSecurityScan bool
)

// generateCmd represents the generate command.
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate an AI-aware BOM (AIBOM) from Hugging Face model IDs",
	Long:  "Generate BOM from Hugging Face model ID(s). Use --model-id to specify models directly or --interactive for a model selector. Use 'scan' command to scan directories for AI imports.",
	RunE:  runGenerate,
}

func runGenerate(cmd *cobra.Command, args []string) error {
	// Resolve effective log level (from config, env, or flag).
	level := strings.ToLower(strings.TrimSpace(viper.GetString("generate.log-level")))
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
	mode := strings.ToLower(strings.TrimSpace(viper.GetString("generate.hf-mode")))
	if mode == "" {
		mode = "online"
	}
	switch mode {
	case "online", "dummy":
		// ok.
	default:
		return apperr.Userf("invalid --hf-mode %q (expected online|dummy)", mode)
	}

	// Check if --interactive was explicitly provided.
	interactiveMode := viper.GetBool("generate.interactive")

	// Check if --model-id was explicitly provided on the command line.
	modelIDFlagProvided := cmd.Flags().Changed("model-id")

	// Get model IDs from viper (respects config file and CLI flag).
	modelIDs := viper.GetStringSlice("generate.model-ids")
	// Filter out empty strings.
	var cleanModelIDs []string
	for _, id := range modelIDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			cleanModelIDs = append(cleanModelIDs, trimmed)
		}
	}

	// Interactive mode validation.
	if interactiveMode {
		if modelIDFlagProvided {
			return apperr.User("--interactive cannot be used with --model-id")
		}
	}

	// Disallow passing model IDs or using interactive mode when running in dummy HF mode.
	// Dummy mode uses a built-in fixture (BuildDummyBOM) — allow empty input only.
	if mode == "dummy" {
		if modelIDFlagProvided || len(cleanModelIDs) > 0 {
			return apperr.User("--model-id cannot be used with --hf-mode=dummy")
		}
		if interactiveMode {
			return apperr.User("--interactive cannot be used with --hf-mode=dummy")
		}
	}

	// Validate that we have either model IDs or interactive mode for non-dummy modes.
	if !interactiveMode && len(cleanModelIDs) == 0 && mode != "dummy" {
		return apperr.User("either --model-id or --interactive is required. Use 'scan' command to scan directories")
	}

	// Get format from viper.
	outputFormat := viper.GetString("generate.format")
	if outputFormat == "" {
		outputFormat = "auto"
	}

	specVersion := viper.GetString("generate.spec")
	outputPath := viper.GetString("generate.output")

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
	hfToken := viper.GetString("generate.hf-token")
	hfTimeout := viper.GetInt("generate.hf-timeout")
	if hfTimeout <= 0 {
		hfTimeout = 10
	}
	timeout := time.Duration(hfTimeout) * time.Second

	// Create UI handler.
	genUI := ui.NewGenerateUI(cmd.OutOrStdout(), quiet)

	var discoveredBOMs []generator.DiscoveredBOM
	var err error

	if interactiveMode {
		// Interactive mode: show model selector.
		selectedModels, err := ui.RunModelSelector(ui.ModelSelectorConfig{
			HFToken: hfToken,
			Timeout: timeout,
		})
		if err != nil {
			return err
		}
		if len(selectedModels) == 0 {
			return apperr.User("no models selected")
		}
		cleanModelIDs = selectedModels
	}

	// Generate BOMs from model IDs.
	err = runModelIDMode(genUI, cleanModelIDs, mode, hfToken, timeout, quiet, &discoveredBOMs)
	if err != nil {
		return err
	}

	// Determine output settings.
	output := viper.GetString("generate.output")
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
		genUI.PrintNoBOMsWritten()
		return nil
	}

	genUI.PrintSummary(len(written), outputDir, fmtChosen)
	return nil
}

func runModelIDMode(genUI *ui.GenerateUI, modelIDs []string, mode, hfToken string, timeout time.Duration, quiet bool, results *[]generator.DiscoveredBOM) error {
	hasToken := strings.TrimSpace(hfToken) != ""
	if mode == "dummy" {
		if !quiet {
			genUI.LogStep("info", "Using dummy mode (no API calls)")
		}
		boms, err := generator.BuildDummyBOM()
		if err != nil {
			return err
		}
		*results = boms
		return nil
	}

	// Track per-model outcome for the final summary.
	// fetch warnings (non-fatal) are accumulated and shown on the single success line.
	// A model that fires EventError{Message:"BOM build failed"} but never EventModelComplete.
	// produced no AIBOM and is shown as a failure.
	pendingModels := make(map[string]*modelTracker)
	var modelOrder []string // insertion-order IDs for deterministic display

	// Create workflow with combined processing step.
	var workflow *ui.Workflow
	var processTaskIdx, writeTaskIdx int

	if !quiet {
		workflow = ui.NewWorkflow(os.Stdout, "")
		processTaskIdx = workflow.AddTask("Processing possible models")
		writeTaskIdx = workflow.AddTask("Writing output")
		workflow.Start()
	}

	totalModels := len(modelIDs)
	modelsCompleted := 0

	// Start processing.
	if !quiet && workflow != nil {
		workflow.StartTask(processTaskIdx, ui.Dim.Render(fmt.Sprintf("0/%d", totalModels)))
	}

	// Progress callback to update UI.
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
			// BOM build failure is terminal for this model (no EventModelComplete follows).
			// Fetch failures are non-fatal; classify them for the summary line.
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
		SkipSecurityScan: noSecurityScan,
	}

	boms, err := generator.BuildFromModelIDs(modelIDs, opts)
	if err != nil {
		if !quiet && workflow != nil {
			workflow.Stop()
		}
		return err
	}

	if !quiet && workflow != nil {
		workflow.CompleteTask(processTaskIdx, fmt.Sprintf("%d possible model(s)", len(modelIDs)))
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
	generateCmd.Flags().StringSliceVarP(&generateModelIDs, "model-id", "m", []string{}, "Hugging Face model ID(s) (e.g., gpt2 or org/model-name) - can be used multiple times or comma-separated")
	generateCmd.Flags().StringVarP(&generateOutput, "output", "o", "", "Output file path (directory is used)")
	generateCmd.Flags().StringVarP(&generateOutputFormat, "format", "f", "", "Output BOM format: json|xml|auto")
	generateCmd.Flags().StringVar(&generateSpecVersion, "spec", "", "CycloneDX spec version for output (e.g., 1.4, 1.5, 1.6)")
	generateCmd.Flags().StringVar(&hfMode, "hf-mode", "", "Hugging Face metadata mode: online|dummy")
	generateCmd.Flags().IntVar(&hfTimeout, "hf-timeout", 0, "Timeout in seconds per Hugging Face API request (default 10)")
	generateCmd.Flags().StringVar(&hfToken, "hf-token", "", "Hugging Face access token")
	generateCmd.Flags().StringVar(&generateLogLevel, "log-level", "", "Log level: quiet|standard|debug")
	generateCmd.Flags().BoolVar(&interactive, "interactive", false, "Interactive model selector (cannot be used with --model-id)")
	generateCmd.Flags().BoolVar(&noSecurityScan, "no-security-scan", false, "Skip fetching the HuggingFace security scan tree")

	// Bind all flags to viper for config file support.
	viper.BindPFlag("generate.model-ids", generateCmd.Flags().Lookup("model-id"))
	viper.BindPFlag("generate.output", generateCmd.Flags().Lookup("output"))
	viper.BindPFlag("generate.format", generateCmd.Flags().Lookup("format"))
	viper.BindPFlag("generate.spec", generateCmd.Flags().Lookup("spec"))
	viper.BindPFlag("generate.hf-mode", generateCmd.Flags().Lookup("hf-mode"))
	viper.BindPFlag("generate.hf-timeout", generateCmd.Flags().Lookup("hf-timeout"))
	viper.BindPFlag("generate.hf-token", generateCmd.Flags().Lookup("hf-token"))
	viper.BindPFlag("generate.log-level", generateCmd.Flags().Lookup("log-level"))
	viper.BindPFlag("generate.interactive", generateCmd.Flags().Lookup("interactive"))
}

// datasetResult holds the outcome of fetching a single dataset referenced by a model.
type datasetResult struct {
	id  string
	err error // nil = fetched and built successfully
}

// modelTracker accumulates per-model progress events so the final summary.
// line can reflect the true outcome (success, 404, auth failure, etc.).
// Shared by the generate and scan commands (same package).
type modelTracker struct {
	apiOK          bool            // API fetch succeeded → model exists on HF Hub
	notFound       bool            // at least one fetch came back 404 (or 401 before apiOK)
	fetchErr       bool            // at least one non-404, post-apiOK fetch failure
	fetchErrVal    error           // the first such error, kept for classification
	complete       bool            // true when EventModelComplete was received
	datasetResults []datasetResult // one entry per dataset referenced by the model
}

// modelOutcome derives the terminal mark and detail string for the model line.
// detail is empty for a clean success (datasets are shown on sub-lines instead).
func modelOutcome(t *modelTracker, hasToken bool) (mark, detail string) {
	switch {
	case t == nil:
		return ui.GetCrossMark(), ui.Error.Render("→ BOM build failed")

	case t.notFound && !t.apiOK:
		if hasToken {
			return ui.GetCrossMark(), ui.Error.Render("→ not found on HF Hub; no BOM written")
		}
		return ui.GetCrossMark(), ui.Error.Render("→ not found (or private – set --hf-token); no BOM written")

	case !t.complete:
		return ui.GetCrossMark(), ui.Error.Render("→ BOM build failed")

	case t.fetchErr:
		if fetcher.IsUnauthorized(t.fetchErrVal) {
			if hasToken {
				return ui.GetWarnMark(), ui.Warning.Render("→ private repo (token lacks access)")
			}
			return ui.GetWarnMark(), ui.Warning.Render("→ private or non-existent repo (set --hf-token)")
		}
		return ui.GetWarnMark(), ui.Warning.Render("→ metadata fetch failed")

	case t.apiOK && t.notFound:
		// fetchErr is false here; model exists but has no README.
		return ui.GetWarnMark(), ui.Warning.Render("→ no README")

	default:
		return ui.GetCheckMark(), ""
	}
}

// datasetOutcome derives the mark and detail string for one dataset sub-line.
func datasetOutcome(r datasetResult, hasToken bool) (mark, detail string) {
	if r.err == nil {
		return ui.GetCheckMark(), ""
	}
	if fetcher.IsNotFound(r.err) || (fetcher.IsUnauthorized(r.err)) {
		if hasToken {
			return ui.GetWarnMark(), ui.Warning.Render("→ not found on HF Hub")
		}
		return ui.GetWarnMark(), ui.Warning.Render("→ not found (or private – set --hf-token)")
	}
	return ui.GetWarnMark(), ui.Warning.Render("→ fetch failed")
}

// printModelResult prints the model summary line followed by one sub-line per dataset.
func printModelResult(id string, t *modelTracker, hasToken bool) {
	mark, detail := modelOutcome(t, hasToken)
	if detail != "" {
		fmt.Printf("  %s %s %s\n", mark, ui.Highlight.Render(id), detail)
	} else {
		fmt.Printf("  %s %s\n", mark, ui.Highlight.Render(id))
	}
	for _, ds := range t.datasetResults {
		dsmark, dsdetail := datasetOutcome(ds, hasToken)
		if dsdetail != "" {
			fmt.Printf("      %s %s %s\n", dsmark, ui.Dim.Render(ds.id), dsdetail)
		} else {
			fmt.Printf("      %s %s\n", dsmark, ui.Dim.Render(ds.id))
		}
	}
}
