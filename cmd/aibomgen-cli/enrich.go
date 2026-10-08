package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/idlab-discover/aibomgen-cli/internal/apperr"
	"github.com/idlab-discover/aibomgen-cli/internal/enricher"
	"github.com/idlab-discover/aibomgen-cli/internal/ui"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/bomio"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// enrichCmd represents the enrich command.
var enrichCmd = &cobra.Command{
	Use:   "enrich [file]",
	Short: "Enrich an existing AIBOM with additional metadata",
	Long: `Enrich an existing AIBOM with additional metadata, prompting for missing fields
or, with --file, loading the values from a YAML file (see config/enrichment.yaml).
By default the model metadata is first refetched from the Hugging Face API and README.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		quiet := verbosity < 0
		yes := viper.GetBool("enrich.yes")
		configFile := strings.TrimSpace(viper.GetString("enrich.file"))
		strategy := "interactive"
		if configFile != "" {
			strategy = "file"
		}

		inputPath, err := requireInput(cmd, args, "enrich")
		if err != nil {
			return err
		}

		// Fail before any work if a prompt would be needed but can't be shown.
		if !ui.CanPrompt() {
			if strategy == "interactive" {
				return errors.New("interactive enrichment needs a terminal; pass --file <enrichment.yaml>")
			}
			if !yes {
				return errors.New("confirming the changes needs a terminal; pass --yes to save without asking")
			}
		}

		bom, err := bomio.ReadBOM(inputPath)
		if err != nil {
			return fmt.Errorf("failed to read input BOM: %w", err)
		}

		// Determine output path.
		outPath := viper.GetString("enrich.output")
		if outPath == "" {
			outPath = inputPath // overwrite by default
		}

		// Get settings from viper (respects config file).
		specVersion := strings.TrimSpace(viper.GetString("enrich.spec"))

		hf := hfOptions("enrich")
		cfg := enricher.Config{
			Strategy:     strategy,
			ConfigFile:   configFile,
			RequiredOnly: viper.GetBool("enrich.required-only"),
			MinWeight:    viper.GetFloat64("enrich.min-weight"),
			Refetch:      viper.GetBool("enrich.refetch"),
			Yes:          yes,
			HFToken:      hf.Token,
			HFBaseURL:    hf.BaseURL,
			HFTimeout:    int(hf.Timeout / time.Second),
		}

		// Load the enrichment values when a file is given.
		var configViper *viper.Viper
		if strategy == "file" {
			configViper, err = loadEnrichmentConfig(configFile)
			if err != nil {
				return fmt.Errorf("failed to load config file: %w", err)
			}
		}

		// Create enricher.
		e := enricher.New(enricher.Options{Config: cfg})

		// Run enrichment.
		enriched, err := e.Enrich(bom, configViper)
		if errors.Is(err, apperr.ErrCancelled) {
			return apperr.ErrCancelled
		}
		if err != nil {
			return fmt.Errorf("enrichment failed: %w", err)
		}

		// Write output.
		if err := bomio.WriteBOM(enriched, outPath, specVersion); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}

		if !quiet {
			msg := fmt.Sprintf("Enriched BOM saved to %s", outPath)
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", ui.SuccessBox.Render(ui.GetCheckMark()+" "+msg))
		}

		return nil
	},
}

func init() {
	enrichCmd.Flags().StringP("input", "i", "", "AIBOM file, instead of the argument")
	enrichCmd.Flags().StringP("output", "o", "", "Output file (default: overwrite the input); .xml writes XML, anything else JSON")
	enrichCmd.Flags().String("spec", "", "CycloneDX spec version for output (default: same as input)")
	enrichCmd.Flags().String("file", "", "YAML file with the values to fill in, instead of prompting")
	enrichCmd.Flags().Bool("required-only", false, "Only fill required fields")
	enrichCmd.Flags().Float64("min-weight", 0.0, "Only fill fields with weight >= this value")
	enrichCmd.Flags().Bool("refetch", true, "Refetch model metadata from Hugging Face first (uses the --hf-* flags)")
	enrichCmd.Flags().BoolP("yes", "y", false, "Save without preview or confirmation")
	addHFFlags(enrichCmd)

	bindFlags(enrichCmd, "enrich")
}

// loadEnrichmentConfig loads enrichment values from a YAML config file.
func loadEnrichmentConfig(path string) (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	return v, nil
}
