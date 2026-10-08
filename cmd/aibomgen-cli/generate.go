package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
	"github.com/idlab-discover/aibomgen-cli/internal/ui"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/generator"
)

// generateCmd represents the generate command.
var generateCmd = &cobra.Command{
	Use:   "generate [model-id...]",
	Short: "Generate an AI-aware BOM (AIBOM) from Hugging Face model IDs",
	Long:  "Generate BOM from Hugging Face model ID(s), given as arguments or with --model-id, or picked with --interactive. Use 'scan' command to scan directories for AI imports.",
	Args:  cobra.ArbitraryArgs,
	RunE:  runGenerate,
}

func runGenerate(cmd *cobra.Command, args []string) error {
	quiet := verbosity < 0

	mode, err := hfMode("generate")
	if err != nil {
		return err
	}

	// Check if --interactive was explicitly provided.
	interactiveMode := viper.GetBool("generate.interactive")

	// Model IDs come from arguments and --model-id (flag, env or config).
	modelIDs := append(viper.GetStringSlice("generate.model-id"), args...)
	modelIDFlagProvided := cmd.Flags().Changed("model-id") || len(args) > 0
	// Filter out empty strings.
	var cleanModelIDs []string
	for _, id := range modelIDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			// Accept "org/name" or "org/name@revision".
			if _, err := generator.ParseModelRef(trimmed); err != nil {
				return err
			}
			cleanModelIDs = append(cleanModelIDs, trimmed)
		}
	}

	// Interactive mode validation.
	if interactiveMode {
		if modelIDFlagProvided {
			return errors.New("cannot combine --interactive with model IDs")
		}
	}

	// Disallow passing model IDs or using interactive mode when running in dummy HF mode.
	// Dummy mode uses a built-in fixture (BuildDummyBOM) — allow empty input only.
	if mode == "dummy" {
		if modelIDFlagProvided || len(cleanModelIDs) > 0 {
			return errors.New("cannot combine model IDs with --hf-mode=dummy")
		}
		if interactiveMode {
			return errors.New("cannot combine --interactive with --hf-mode=dummy")
		}
	}

	// Validate that we have either model IDs or interactive mode for non-dummy modes.
	if !interactiveMode && len(cleanModelIDs) == 0 && mode != "dummy" {
		return errors.New("no model given: pass model IDs or --interactive (use 'scan' to scan a directory)")
	}

	// Validate the output format before doing any work.
	fmtChosen, err := outputFormat(viper.GetString("generate.format"))
	if err != nil {
		return err
	}
	outputDir := viper.GetString("generate.output")

	hf := hfOptions("generate")

	// Create UI handler.
	genUI := ui.NewGenerateUI(cmd.OutOrStdout(), quiet)

	if interactiveMode {
		if !ui.CanPrompt() {
			return errors.New("interactive model selection needs a terminal; pass model IDs instead")
		}
		// Interactive mode: show model selector.
		selectedModels, err := ui.RunModelSelector(ui.ModelSelectorConfig{
			HFToken: hf.Token,
			BaseURL: hf.BaseURL,
			Timeout: hf.Timeout,
		})
		if err != nil {
			return err
		}
		if len(selectedModels) == 0 {
			return errors.New("no models selected")
		}
		cleanModelIDs = selectedModels
	}

	// Generate BOMs from model IDs.
	var discoveredBOMs []generator.DiscoveredBOM
	if err := runModelIDMode(genUI, cleanModelIDs, mode, hf, quiet, &discoveredBOMs); err != nil {
		return err
	}

	return writeDiscovered(genUI, discoveredBOMs, outputDir, fmtChosen, viper.GetString("generate.spec"))
}

func runModelIDMode(genUI *ui.GenerateUI, modelIDs []string, mode string, hf hfSettings, quiet bool, results *[]generator.DiscoveredBOM) error {
	hasToken := strings.TrimSpace(hf.Token) != ""
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

	// Create workflow with combined processing step.
	var workflow *ui.Workflow
	var processTaskIdx, writeTaskIdx int

	if !quiet {
		workflow = newWorkflow()
		processTaskIdx = workflow.AddTask("Processing possible models")
		writeTaskIdx = workflow.AddTask("Writing output")
		workflow.Start()
	}

	onProgress, finish := trackProgress(workflow, processTaskIdx, writeTaskIdx, len(modelIDs), hasToken)

	opts := generator.GenerateOptions{
		HFToken:          hf.Token,
		BaseURL:          hf.BaseURL,
		Timeout:          hf.Timeout,
		OnProgress:       onProgress,
		SkipSecurityScan: viper.GetBool("generate.no-security-scan"),
	}

	boms, err := generator.BuildFromModelIDs(modelIDs, opts)
	if err != nil {
		if workflow != nil {
			workflow.Stop()
		}
		return err
	}

	finish(len(boms))
	*results = boms
	return nil
}

func init() {
	generateCmd.Flags().StringSliceP("model-id", "m", []string{}, "Hugging Face model ID (org/name or org/name@revision); repeatable, comma-separated, or given as arguments")
	generateCmd.Flags().StringP("output", "o", "dist", "Output directory; one file per model")
	generateCmd.Flags().StringP("format", "f", "json", "Output BOM format: json|xml")
	generateCmd.Flags().String("spec", "", "CycloneDX spec version for output, e.g. 1.6 (default latest)")
	generateCmd.Flags().Bool("interactive", false, "Pick models in an interactive selector")
	generateCmd.Flags().Bool("no-security-scan", false, "Skip fetching the Hugging Face security scan tree")
	addHFFlags(generateCmd)
	addHFModeFlag(generateCmd)

	bindFlags(generateCmd, "generate")
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
