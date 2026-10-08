package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/idlab-discover/aibomgen-cli/internal/ui"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/bomio"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/completeness"
)

var completenessCmd = &cobra.Command{
	Use:   "completeness [file]",
	Short: "Compute completeness score for an AIBOM",
	Long:  "Reads an existing CycloneDX AIBOM (json/xml) and scores it against the configured field registry.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		quiet := verbosity < 0

		inputPath, err := requireInput(cmd, args, "completeness")
		if err != nil {
			return err
		}
		bom, err := bomio.ReadBOM(inputPath)
		if err != nil {
			return err
		}

		res := completeness.Check(bom)

		if viper.GetBool("completeness.json") {
			return writeJSON(cmd.OutOrStdout(), res)
		}
		ui.NewCompletenessUI(cmd.OutOrStdout(), quiet).PrintReport(res)
		return nil
	},
}

func init() {
	completenessCmd.Flags().StringP("input", "i", "", "AIBOM file, instead of the argument")
	completenessCmd.Flags().Bool("json", false, "Print the result as JSON")

	bindFlags(completenessCmd, "completeness")
}
