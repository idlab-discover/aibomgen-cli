package cmd

import (
	"errors"
	"fmt"
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"

	"github.com/idlab-discover/aibomgen-cli/internal/ui"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/bomio"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/validator"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate an existing AIBOM file",
	Long:  "Validates that a CycloneDX AIBOM JSON is well-formed and optionally checks for required model card fields in strict mode.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get input from viper (respects config file and CLI flag).
		inputPath := viper.GetString("validate.input")
		if inputPath == "" {
			return errors.New("--input is required")
		}

		// Get log level from viper.
		quiet, err := quietFrom(viper.GetString("validate.log-level"))
		if err != nil {
			return err
		}

		failSeverity := cdx.Severity(strings.ToLower(strings.TrimSpace(viper.GetString("validate.fail-severity"))))
		switch failSeverity {
		case "":
			failSeverity = cdx.SeverityMedium
		case cdx.SeverityCritical, cdx.SeverityHigh, cdx.SeverityMedium, cdx.SeverityLow, cdx.SeverityInfo:
			// ok.
		default:
			return fmt.Errorf("invalid --fail-severity %q (expected critical|high|medium|low|info)", failSeverity)
		}

		// Read BOM.
		bom, err := bomio.ReadBOM(inputPath)
		if err != nil {
			return fmt.Errorf("failed to read BOM: %w", err)
		}

		// Get validation options from viper.
		opts := validator.ValidationOptions{
			StrictMode:           viper.GetBool("validate.strict"),
			MinCompletenessScore: viper.GetFloat64("validate.min-score"),
			CheckModelCard:       viper.GetBool("validate.check-model-card"),
			FailSeverity:         failSeverity,
		}

		result := validator.Validate(bom, opts)

		// Use the new UI for rendering if not in quiet mode.
		ui := ui.NewValidationUI(cmd.OutOrStdout(), quiet)
		ui.PrintReport(result)

		if !result.Valid {
			return fmt.Errorf("validation failed")
		}

		return nil
	},
}

func init() {
	validateCmd.Flags().StringP("input", "i", "", "Path to AIBOM file (required)")
	addDeprecatedFlag(validateCmd, "format", "f", inputFormatDeprecation)
	validateCmd.Flags().Bool("strict", false, "Strict mode: fail on missing required fields and on vulnerabilities at or above --fail-severity")
	validateCmd.Flags().String("fail-severity", "", "Lowest vulnerability severity that fails --strict: critical|high|medium|low|info (default medium)")
	validateCmd.Flags().Float64("min-score", 0.0, "Minimum completeness score (0.0-1.0)")
	validateCmd.Flags().Bool("check-model-card", false, "Validate model card fields")
	validateCmd.Flags().String("log-level", "", "Log level: quiet|standard|debug")

	// Bind all flags to viper for config file support.
	bindFlags(validateCmd, "validate")
}
