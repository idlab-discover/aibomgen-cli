package cmd

import (
	"fmt"
	"strings"

	"github.com/idlab-discover/aibomgen-cli/internal/apperr"
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
			return apperr.Userf("invalid strategy %q (expected interactive|file)", strategy)
		}

		// Get log level from viper.
		level := strings.ToLower(strings.TrimSpace(viper.GetString("enrich.log-level")))
		if level == "" {
			level = "standard"
		}
		switch level {
		case "quiet", "standard", "debug":
			// ok.
		default:
			return apperr.Userf("invalid --log-level %q (expected quiet|standard|debug)", level)
		}

		// Read existing BOM.
		inputPath := viper.GetString("enrich.input")
		if inputPath == "" {
			return apperr.User("--input is required")
		}
		inputFormat := viper.GetString("enrich.format")
		if inputFormat == "" {
			inputFormat = "auto"
		}
		bom, err := bomio.ReadBOM(inputPath, inputFormat)
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
		outputFormat := viper.GetString("enrich.output-format")
		if outputFormat == "" {
			outputFormat = "auto"
		}

		// Build enricher configuration.
		cfg := enricher.Config{
			Strategy:     strategy,
			ConfigFile:   viper.GetString("enrich.file"),
			RequiredOnly: viper.GetBool("enrich.required-only"),
			MinWeight:    viper.GetFloat64("enrich.min-weight"),
			Refetch:      viper.GetBool("enrich.refetch"),
			NoPreview:    viper.GetBool("enrich.no-preview"),
			SpecVersion:  specVersion,
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
			Reader: cmd.InOrStdin(),
			Writer: cmd.OutOrStdout(),
			Config: cfg,
		})

		// Run enrichment.
		enriched, err := e.Enrich(bom, configViper)
		if err != nil {
			return fmt.Errorf("enrichment failed: %w", err)
		}

		// Write output.
		if err := bomio.WriteBOM(enriched, outPath, outputFormat, specVersion); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}

		if level != "quiet" {
			msg := fmt.Sprintf("Enriched BOM saved to %s", outPath)
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", ui.SuccessBox.Render(ui.GetCheckMark()+" "+msg))
		}

		return nil
	},
}

var (
	enrichInput        string
	enrichInputFormat  string
	enrichOutput       string
	enrichOutputFormat string
	enrichSpecVersion  string
	enrichStrategy     string
	enrichConfigFile   string
	enrichRequiredOnly bool
	enrichMinWeight    float64
	enrichRefetch      bool
	enrichNoPreview    bool
	enrichLogLevel     string
	enrichHFToken      string
	enrichHFBaseURL    string
	enrichHFTimeout    int
)

func init() {
	enrichCmd.Flags().StringVarP(&enrichInput, "input", "i", "", "Path to existing AIBOM (required)")
	enrichCmd.Flags().StringVarP(&enrichOutput, "output", "o", "", "Output file path (default: overwrite input)")
	enrichCmd.Flags().StringVarP(&enrichInputFormat, "format", "f", "", "Input BOM format: json|xml|auto")
	enrichCmd.Flags().StringVar(&enrichOutputFormat, "output-format", "", "Output BOM format: json|xml|auto")
	enrichCmd.Flags().StringVar(&enrichSpecVersion, "spec", "", "CycloneDX spec version for output (default: same as input)")

	enrichCmd.Flags().StringVar(&enrichStrategy, "strategy", "", "Enrichment strategy: interactive|file")
	enrichCmd.Flags().StringVar(&enrichConfigFile, "file", "", "Path to enrichment config file (YAML)")
	enrichCmd.Flags().BoolVar(&enrichRequiredOnly, "required-only", false, "Only prompt for required fields")
	enrichCmd.Flags().Float64Var(&enrichMinWeight, "min-weight", 0.0, "Only prompt for fields with weight >= this value")
	enrichCmd.Flags().BoolVar(&enrichRefetch, "refetch", false, "Refetch model metadata from Hugging Face before enrichment")
	enrichCmd.Flags().BoolVar(&enrichNoPreview, "no-preview", false, "Skip preview before saving")

	enrichCmd.Flags().StringVar(&enrichLogLevel, "log-level", "", "Log level: quiet|standard|debug")
	enrichCmd.Flags().StringVar(&enrichHFToken, "hf-token", "", "Hugging Face API token (for refetch)")
	enrichCmd.Flags().StringVar(&enrichHFBaseURL, "hf-base-url", "", "Hugging Face base URL (for refetch)")
	enrichCmd.Flags().IntVar(&enrichHFTimeout, "hf-timeout", 0, "Hugging Face API timeout in seconds (for refetch)")

	// Bind all flags to viper for config file support.
	viper.BindPFlag("enrich.input", enrichCmd.Flags().Lookup("input"))
	viper.BindPFlag("enrich.output", enrichCmd.Flags().Lookup("output"))
	viper.BindPFlag("enrich.format", enrichCmd.Flags().Lookup("format"))
	viper.BindPFlag("enrich.output-format", enrichCmd.Flags().Lookup("output-format"))
	viper.BindPFlag("enrich.spec", enrichCmd.Flags().Lookup("spec"))
	viper.BindPFlag("enrich.strategy", enrichCmd.Flags().Lookup("strategy"))
	viper.BindPFlag("enrich.file", enrichCmd.Flags().Lookup("file"))
	viper.BindPFlag("enrich.required-only", enrichCmd.Flags().Lookup("required-only"))
	viper.BindPFlag("enrich.min-weight", enrichCmd.Flags().Lookup("min-weight"))
	viper.BindPFlag("enrich.refetch", enrichCmd.Flags().Lookup("refetch"))
	viper.BindPFlag("enrich.no-preview", enrichCmd.Flags().Lookup("no-preview"))
	viper.BindPFlag("enrich.log-level", enrichCmd.Flags().Lookup("log-level"))
	viper.BindPFlag("enrich.hf-token", enrichCmd.Flags().Lookup("hf-token"))
	viper.BindPFlag("enrich.hf-base-url", enrichCmd.Flags().Lookup("hf-base-url"))
	viper.BindPFlag("enrich.hf-timeout", enrichCmd.Flags().Lookup("hf-timeout"))
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
