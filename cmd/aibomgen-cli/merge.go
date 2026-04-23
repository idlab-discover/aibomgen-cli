package cmd

import (
	"fmt"
	"os"
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/internal/apperr"
	"github.com/idlab-discover/aibomgen-cli/internal/ui"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/bomio"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/merger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	mergeAIBOMs      []string
	mergeSBOM        string
	mergeOutput      string
	mergeFormat      string
	mergeDeduplicate bool
	mergeLogLevel    string
)

var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "[BETA] Merge one or more AIBOMs with an existing SBOM",
	Long: `[BETA] Merges one or more AI Bill of Materials (AIBOMs) with a Software Bill of Materials (SBOM) from a different source.
This allows you to combine AI/ML component information with traditional software dependencies into a single comprehensive BOM.

The SBOM's application metadata is preserved as the main component, while AI/ML model and dataset components
from the AIBOM(s) are added to the components list.

Example:
  # Generate SBOM with Syft
  syft scan . -o cyclonedx-json > sbom.json

  # Generate AIBOM with AIBoMGen
  ./aibomgen-cli generate -i . -o aibom.json

  # Merge them together
  ./aibomgen-cli merge --aibom aibom.json --sbom sbom.json -o merged.json

  # Merge multiple AIBOMs with one SBOM
  ./aibomgen-cli merge --aibom model1_aibom.json --aibom model2_aibom.json --sbom sbom.json -o merged.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get inputs from viper (respects config file and CLI flag).
		aibomPaths := viper.GetStringSlice("merge.aiboms")
		if len(aibomPaths) == 0 {
			return apperr.User("at least one --aibom is required")
		}

		sbomPath := viper.GetString("merge.sbom")
		if sbomPath == "" {
			return apperr.User("--sbom is required")
		}

		outputPath := viper.GetString("merge.output")
		if outputPath == "" {
			return apperr.User("--output is required")
		}

		// Get log level from viper.
		level := strings.ToLower(strings.TrimSpace(viper.GetString("merge.log-level")))
		if level == "" {
			level = "standard"
		}
		switch level {
		case "quiet", "standard", "debug":
			// ok.
		default:
			return apperr.Userf("invalid --log-level %q (expected quiet|standard|debug)", level)
		}

		// Get format from viper or detect from output path.
		format := viper.GetString("merge.format")
		if format == "" {
			format = "auto"
		}

		// Initialize UI.
		quiet := level == "quiet"
		mergerUI := ui.NewMergerUI(os.Stdout, quiet)
		mergerUI.StartWorkflow(len(aibomPaths))

		// Read SBOM (this will be the base).
		mergerUI.StartReadingSBOM(sbomPath)
		sbom, err := bomio.ReadBOM(sbomPath, "auto")
		if err != nil {
			mergerUI.PrintError(fmt.Errorf("failed to read SBOM: %w", err))
			return err
		}

		sbomComponentCount := 0
		if sbom.Components != nil {
			sbomComponentCount = len(*sbom.Components)
		}
		mergerUI.CompleteReadingSBOM(sbomComponentCount)

		// Read all AIBOMs.
		mergerUI.StartReadingAIBOMs(len(aibomPaths))
		var aiboms []*cdx.BOM
		for i, aibomPath := range aibomPaths {
			mergerUI.UpdateReadingAIBOM(i, len(aibomPaths), aibomPath)
			aibom, err := bomio.ReadBOM(aibomPath, "auto")
			if err != nil {
				mergerUI.PrintError(fmt.Errorf("failed to read AIBOM %s: %w", aibomPath, err))
				return err
			}
			aiboms = append(aiboms, aibom)
		}
		mergerUI.CompleteReadingAIBOMs(len(aiboms))

		// Prepare merge options.
		opts := merger.MergeOptions{
			DeduplicateComponents: viper.GetBool("merge.deduplicate"),
		}

		// Perform merge.
		mergerUI.StartMerging()
		result, err := merger.MergeAIBOMsWithSBOM(sbom, aiboms, opts)
		if err != nil {
			mergerUI.PrintError(fmt.Errorf("failed to merge BOMs: %w", err))
			return err
		}
		mergerUI.CompleteMerging(result.SBOMComponentCount, result.AIBOMComponentCount)

		// Write merged BOM.
		mergerUI.StartWriting(outputPath)
		if err := bomio.WriteBOM(result.MergedBOM, outputPath, format, ""); err != nil {
			mergerUI.PrintError(fmt.Errorf("failed to write merged BOM: %w", err))
			return err
		}
		mergerUI.CompleteWriting()

		// Print summary.
		mergerUI.PrintSummary(result, outputPath, len(aiboms), opts.DeduplicateComponents)

		return nil
	},
}

func init() {
	mergeCmd.Flags().StringSliceVar(&mergeAIBOMs, "aibom", []string{}, "Path to AIBOM file (can be specified multiple times, required)")
	mergeCmd.Flags().StringVar(&mergeSBOM, "sbom", "", "Path to SBOM file (required)")
	mergeCmd.Flags().StringVarP(&mergeOutput, "output", "o", "", "Output path for merged BOM (required)")
	mergeCmd.Flags().StringVarP(&mergeFormat, "format", "f", "", "Output format: json|xml|auto (default: auto)")
	mergeCmd.Flags().BoolVar(&mergeDeduplicate, "deduplicate", true, "Remove duplicate components based on BOM-ref")
	mergeCmd.Flags().StringVar(&mergeLogLevel, "log-level", "", "Log level: quiet|standard|debug")

	// Bind all flags to viper for config file support.
	viper.BindPFlag("merge.aiboms", mergeCmd.Flags().Lookup("aibom"))
	viper.BindPFlag("merge.sbom", mergeCmd.Flags().Lookup("sbom"))
	viper.BindPFlag("merge.output", mergeCmd.Flags().Lookup("output"))
	viper.BindPFlag("merge.format", mergeCmd.Flags().Lookup("format"))
	viper.BindPFlag("merge.deduplicate", mergeCmd.Flags().Lookup("deduplicate"))
	viper.BindPFlag("merge.log-level", mergeCmd.Flags().Lookup("log-level"))
}
