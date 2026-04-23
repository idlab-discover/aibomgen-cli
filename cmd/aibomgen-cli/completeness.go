package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/idlab-discover/aibomgen-cli/internal/apperr"
	"github.com/idlab-discover/aibomgen-cli/internal/ui"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/bomio"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/completeness"
)

var completenessCmd = &cobra.Command{
	Use:   "completeness",
	Short: "Compute completeness score for an AIBOM",
	Long:  "Reads an existing CycloneDX AIBOM (json/xml) and scores it against the configured field registry.",
	RunE: func(cmd *cobra.Command, args []string) error {

		// Get log level from viper (respects config file and CLI flag).
		level := strings.ToLower(strings.TrimSpace(viper.GetString("completeness.log-level")))
		if level == "" {
			level = "standard"
		}
		switch level {
		case "quiet", "standard", "debug":
			// ok.
		default:
			return apperr.Userf("invalid --log-level %q (expected quiet|standard|debug)", level)
		}

		// Get input path and format from viper.
		inputPath := viper.GetString("completeness.input")
		if inputPath == "" {
			return apperr.User("--input is required")
		}
		inputFormat := viper.GetString("completeness.format")
		if inputFormat == "" {
			inputFormat = "auto"
		}

		bom, err := bomio.ReadBOM(inputPath, inputFormat)
		if err != nil {
			return err
		}

		res := completeness.Check(bom)

		// If plain-summary requested, print a machine-readable plain summary (no styling).
		if completenessPlainSummary {
			// Model summary line.
			fmt.Printf("Model: %s | Score: %.1f%% | Fields: %d/%d\n", res.ModelID, res.Score*100, res.Passed, res.Total)
			// Dataset summary lines (if any).
			for dsName, ds := range res.DatasetResults {
				fmt.Printf("Dataset: %s | Score: %.1f%% | Fields: %d/%d\n", dsName, ds.Score*100, ds.Passed, ds.Total)
			}
			return nil
		}

		// Use the new UI for rendering if not in quiet mode.
		ui := ui.NewCompletenessUI(cmd.OutOrStdout(), level == "quiet")
		ui.PrintReport(res)

		return nil
	},
}

var (
	inPath                   string
	inFormat                 string
	completenessLogLevel     string
	completenessPlainSummary bool
)

func init() {
	completenessCmd.Flags().StringVarP(&inPath, "input", "i", "", "Path to existing AIBOM file (required)")
	completenessCmd.Flags().StringVarP(&inFormat, "format", "f", "", "Input BOM format: json|xml|auto")
	completenessCmd.Flags().StringVar(&completenessLogLevel, "log-level", "", "Log level: quiet|standard|debug")
	completenessCmd.Flags().BoolVar(&completenessPlainSummary, "plain-summary", false, "Print a single-line plain summary (no styling)")

	// Bind all flags to viper for config file support.
	viper.BindPFlag("completeness.input", completenessCmd.Flags().Lookup("input"))
	viper.BindPFlag("completeness.format", completenessCmd.Flags().Lookup("format"))
	viper.BindPFlag("completeness.log-level", completenessCmd.Flags().Lookup("log-level"))
	viper.BindPFlag("completeness.plain-summary", completenessCmd.Flags().Lookup("plain-summary"))
}
