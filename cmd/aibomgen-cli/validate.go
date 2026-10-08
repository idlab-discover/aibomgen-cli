package cmd

import (
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
	Use:   "validate [file]",
	Short: "Validate an existing AIBOM file",
	Long: `Validates a CycloneDX AIBOM (JSON or XML).

JSON BOMs are checked against the official CycloneDX JSON schema for their spec
version. Every BOM is checked for unique bom-refs, dangling references, the
minimum completeness score (--min-score) and vulnerabilities. In --strict mode,
missing required fields and vulnerabilities at or above --fail-severity fail
validation.

Exit codes: 0 valid, 1 error (e.g. unreadable file, bad flag), 2 invalid BOM.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputPath, err := requireInput(cmd, args, "validate")
		if err != nil {
			return err
		}

		quiet := verbosity < 0

		failSeverity := cdx.Severity(strings.ToLower(strings.TrimSpace(viper.GetString("validate.fail-severity"))))
		switch failSeverity {
		case cdx.SeverityCritical, cdx.SeverityHigh, cdx.SeverityMedium, cdx.SeverityLow, cdx.SeverityInfo:
			// ok.
		default:
			return fmt.Errorf("invalid severity %q for --fail-severity (expected critical|high|medium|low|info)", failSeverity)
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

		if viper.GetBool("validate.json") {
			if err := writeJSON(cmd.OutOrStdout(), result); err != nil {
				return err
			}
		} else {
			ui.NewValidationUI(cmd.OutOrStdout(), quiet, verbosity >= 1).PrintReport(result)
		}

		if !result.Valid {
			return apperr.ErrValidation
		}

		return nil
	},
}

func init() {
	validateCmd.Flags().StringP("input", "i", "", "AIBOM file, instead of the argument")
	validateCmd.Flags().Bool("strict", false, "Strict mode: fail on missing required fields and on vulnerabilities at or above --fail-severity")
	validateCmd.Flags().String("fail-severity", "medium", "Lowest vulnerability severity that fails --strict: critical|high|medium|low|info")
	validateCmd.Flags().Float64("min-score", 0.0, "Minimum completeness score (0.0-1.0), enforced with or without --strict")
	validateCmd.Flags().Bool("json", false, "Print the result as JSON")

	bindFlags(validateCmd, "validate")
}
