package cmd

import (
	"fmt"
	"strings"

	"github.com/idlab-discover/aibomgen-cli/internal/apperr"
	"github.com/idlab-discover/aibomgen-cli/internal/ui"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/bomio"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/validator"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	validateInput          string
	validateFormat         string
	validateStrict         bool
	validateMinScore       float64
	validateCheckModelCard bool
	validateLogLevel       string
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate an existing AIBOM file",
	Long:  "Validates that a CycloneDX AIBOM JSON is well-formed and optionally checks for required model card fields in strict mode.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get input from viper (respects config file and CLI flag).
		inputPath := viper.GetString("validate.input")
		if inputPath == "" {
			return apperr.User("--input is required")
		}

		// Get log level from viper (respects config file).
		level := strings.ToLower(strings.TrimSpace(viper.GetString("validate.log-level")))
		if level == "" {
			level = "standard"
		}
		switch level {
		case "quiet", "standard", "debug":
			// ok.
		default:
			return apperr.Userf("invalid --log-level %q (expected quiet|standard|debug)", level)
		}

		// Get format from viper.
		format := viper.GetString("validate.format")
		if format == "" {
			format = "auto"
		}

		// Read BOM.
		bom, err := bomio.ReadBOM(inputPath, format)
		if err != nil {
			return fmt.Errorf("failed to read BOM: %w", err)
		}

		// Get validation options from viper.
		opts := validator.ValidationOptions{
			StrictMode:           viper.GetBool("validate.strict"),
			MinCompletenessScore: viper.GetFloat64("validate.min-score"),
			CheckModelCard:       viper.GetBool("validate.check-model-card"),
		}

		result := validator.Validate(bom, opts)

		// Use the new UI for rendering if not in quiet mode.
		ui := ui.NewValidationUI(cmd.OutOrStdout(), level == "quiet")
		ui.PrintReport(result)

		if !result.Valid {
			return fmt.Errorf("validation failed")
		}

		return nil
	},
}

func init() {
	validateCmd.Flags().StringVarP(&validateInput, "input", "i", "", "Path to AIBOM file (required)")
	validateCmd.Flags().StringVarP(&validateFormat, "format", "f", "", "Input format: json|xml|auto")
	validateCmd.Flags().BoolVar(&validateStrict, "strict", false, "Strict mode: fail on missing required fields")
	validateCmd.Flags().Float64Var(&validateMinScore, "min-score", 0.0, "Minimum completeness score (0.0-1.0)")
	validateCmd.Flags().BoolVar(&validateCheckModelCard, "check-model-card", false, "Validate model card fields")
	validateCmd.Flags().StringVar(&validateLogLevel, "log-level", "", "Log level: quiet|standard|debug")

	// Bind all flags to viper for config file support.
	viper.BindPFlag("validate.input", validateCmd.Flags().Lookup("input"))
	viper.BindPFlag("validate.format", validateCmd.Flags().Lookup("format"))
	viper.BindPFlag("validate.strict", validateCmd.Flags().Lookup("strict"))
	viper.BindPFlag("validate.min-score", validateCmd.Flags().Lookup("min-score"))
	viper.BindPFlag("validate.check-model-card", validateCmd.Flags().Lookup("check-model-card"))
	viper.BindPFlag("validate.log-level", validateCmd.Flags().Lookup("log-level"))
}
