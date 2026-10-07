package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"

	"github.com/idlab-discover/aibomgen-cli/internal/apperr"
	"github.com/idlab-discover/aibomgen-cli/internal/ui"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/validator"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate an existing AIBOM file",
	Long: `Validates a CycloneDX AIBOM (JSON or XML).

JSON BOMs are checked against the official CycloneDX JSON schema for their spec
version. Every BOM is checked for unique bom-refs, dangling references, the
minimum completeness score (--min-score) and vulnerabilities. In --strict mode,
missing required fields and vulnerabilities at or above --fail-severity fail
validation.

Exit codes: 0 valid, 1 error (e.g. unreadable file, bad flag), 2 invalid BOM.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get input from viper (respects config file and CLI flag).
		inputPath := viper.GetString("validate.input")
		if inputPath == "" {
			return errors.New("--input is required")
		}

		// Get log level from viper.
		level := viper.GetString("validate.log-level")
		quiet, err := quietFrom(level)
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

		data, err := os.ReadFile(inputPath)
		if err != nil {
			return fmt.Errorf("failed to read BOM: %w", err)
		}

		// Get validation options from viper.
		opts := validator.ValidationOptions{
			StrictMode:           viper.GetBool("validate.strict"),
			MinCompletenessScore: viper.GetFloat64("validate.min-score"),
			FailSeverity:         failSeverity,
		}

		result := validator.ValidateData(data, opts)

		// Use the new UI for rendering if not in quiet mode.
		ui := ui.NewValidationUI(cmd.OutOrStdout(), quiet, strings.EqualFold(strings.TrimSpace(level), "debug"))
		ui.PrintReport(result)

		if !result.Valid {
			return apperr.ErrValidation
		}

		return nil
	},
}

func init() {
	validateCmd.Flags().StringP("input", "i", "", "Path to AIBOM file (required)")
	addDeprecatedFlag(validateCmd, "format", "f", inputFormatDeprecation)
	validateCmd.Flags().Bool("strict", false, "Strict mode: fail on missing required fields and on vulnerabilities at or above --fail-severity")
	validateCmd.Flags().String("fail-severity", "", "Lowest vulnerability severity that fails --strict: critical|high|medium|low|info (default medium)")
	validateCmd.Flags().Float64("min-score", 0.0, "Minimum completeness score (0.0-1.0), enforced with or without --strict")
	validateCmd.Flags().String("log-level", "", "Log level: quiet|standard|debug (debug also lists missing optional fields)")
	// Deprecated: completeness already scores model card fields.
	validateCmd.Flags().Bool("check-model-card", false, "")
	_ = validateCmd.Flags().MarkDeprecated("check-model-card", "model card fields are covered by the completeness score")

	// Bind all flags to viper for config file support.
	bindFlags(validateCmd, "validate")
}
