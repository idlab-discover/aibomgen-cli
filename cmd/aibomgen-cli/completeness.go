package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/idlab-discover/aibomgen-cli/internal/ui"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/bomio"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/completeness"
)

var completenessCmd = &cobra.Command{
	Use:   "completeness",
	Short: "Compute completeness score for an AIBOM",
	Long:  "Reads an existing CycloneDX AIBOM (json/xml) and scores it against the configured field registry.",
	RunE: func(cmd *cobra.Command, args []string) error {

		// Get log level from viper.
		quiet, err := quietFrom(viper.GetString("completeness.log-level"))
		if err != nil {
			return err
		}

		// Get input path from viper.
		inputPath := viper.GetString("completeness.input")
		if inputPath == "" {
			return errors.New("--input is required")
		}
		bom, err := bomio.ReadBOM(inputPath)
		if err != nil {
			return err
		}

		res := completeness.Check(bom)

		// If plain-summary requested, print a machine-readable plain summary (no styling).
		if viper.GetBool("completeness.plain-summary") {
			// Model summary line.
			fmt.Printf("Model: %s | Score: %.1f%% | Fields: %d/%d\n", res.ModelID, res.Score*100, res.Passed, res.Total)
			// Dataset summary lines (if any).
			for dsName, ds := range res.DatasetResults {
				fmt.Printf("Dataset: %s | Score: %.1f%% | Fields: %d/%d\n", dsName, ds.Score*100, ds.Passed, ds.Total)
			}
			return nil
		}

		// Use the new UI for rendering if not in quiet mode.
		ui := ui.NewCompletenessUI(cmd.OutOrStdout(), quiet)
		ui.PrintReport(res)

		return nil
	},
}

func init() {
	completenessCmd.Flags().StringP("input", "i", "", "Path to existing AIBOM file (required)")
	addDeprecatedFlag(completenessCmd, "format", "f", inputFormatDeprecation)
	completenessCmd.Flags().String("log-level", "", "Log level: quiet|standard|debug")
	completenessCmd.Flags().Bool("plain-summary", false, "Print a single-line plain summary (no styling)")

	// Bind all flags to viper for config file support.
	bindFlags(completenessCmd, "completeness")
}
