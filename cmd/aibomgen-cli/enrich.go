package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/idlab-discover/aibomgen-cli/internal/enricher"
	"github.com/idlab-discover/aibomgen-cli/internal/ui"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/bomio"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// enrichCmd represents the enrich command.
var enrichCmd = &cobra.Command{
	Use:   "enrich",
	Short: "Enrich an existing AIBOM with additional metadata",
	Long: `Enrich an existing AIBOM with additional metadata through interactive prompts
or by loading values from a configuration file. Optionally refetch model metadata
from Hugging Face API and README before enrichment.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get strategy from viper (respects config file).
		strategy := strings.ToLower(strings.TrimSpace(viper.GetString("enrich.strategy")))
		if strategy == "" {
			strategy = "interactive"
		}
		switch strategy {
		case "interactive", "file":
			// ok.
		default:
			return fmt.Errorf("invalid strategy %q (expected interactive|file)", strategy)
		}

		// Get log level from viper.
		quiet, err := quietFrom(viper.GetString("enrich.log-level"))
		if err != nil {
			return err
		}

		// Read existing BOM.
		inputPath := viper.GetString("enrich.input")
		if inputPath == "" {
			return errors.New("--input is required")
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

		// Build enricher configuration.
		cfg := enricher.Config{
			Strategy:     strategy,
			ConfigFile:   viper.GetString("enrich.file"),
			RequiredOnly: viper.GetBool("enrich.required-only"),
			MinWeight:    viper.GetFloat64("enrich.min-weight"),
			Refetch:      viper.GetBool("enrich.refetch"),
			NoPreview:    viper.GetBool("enrich.no-preview"),
			HFToken:      viper.GetString("enrich.hf-token"),
			HFBaseURL:    viper.GetString("enrich.hf-base-url"),
			HFTimeout:    viper.GetInt("enrich.hf-timeout"),
		}

		// Load config file values if using file strategy.
		var configViper *viper.Viper
		if strategy == "file" {
			configFile := cfg.ConfigFile
			if configFile == "" {
				configFile = "./config/enrichment.yaml"
			}
			configViper, err = loadEnrichmentConfig(configFile)
			if err != nil {
				return fmt.Errorf("failed to load config file: %w", err)
			}
		}

		// Create enricher.
		e := enricher.New(enricher.Options{
			Writer: cmd.OutOrStdout(),
			Config: cfg,
		})

		// Run enrichment.
		enriched, err := e.Enrich(bom, configViper)
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
	enrichCmd.Flags().StringP("input", "i", "", "Path to existing AIBOM (required)")
	enrichCmd.Flags().StringP("output", "o", "", "Output file path (default: overwrite input)")
	addDeprecatedFlag(enrichCmd, "format", "f", inputFormatDeprecation)
	addDeprecatedFlag(enrichCmd, "output-format", "", outputFormatDeprecation)
	enrichCmd.Flags().String("spec", "", "CycloneDX spec version for output (default: same as input)")

	enrichCmd.Flags().String("strategy", "", "Enrichment strategy: interactive|file")
	enrichCmd.Flags().String("file", "", "Path to enrichment config file (YAML)")
	enrichCmd.Flags().Bool("required-only", false, "Only prompt for required fields")
	enrichCmd.Flags().Float64("min-weight", 0.0, "Only prompt for fields with weight >= this value")
	enrichCmd.Flags().Bool("refetch", false, "Refetch model metadata from Hugging Face before enrichment")
	enrichCmd.Flags().Bool("no-preview", false, "Skip preview before saving")

	enrichCmd.Flags().String("log-level", "", "Log level: quiet|standard|debug")
	enrichCmd.Flags().String("hf-token", "", "Hugging Face API token (for refetch)")
	enrichCmd.Flags().String("hf-base-url", "", "Hugging Face base URL (for refetch)")
	enrichCmd.Flags().Int("hf-timeout", 0, "Hugging Face API timeout in seconds (for refetch)")

	// Bind all flags to viper for config file support.
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
