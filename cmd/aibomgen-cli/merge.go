package cmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/internal/ui"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/bomio"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/merger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
			return errors.New("at least one --aibom is required")
		}

		sbomPath := viper.GetString("merge.sbom")
		if sbomPath == "" {
			return errors.New("--sbom is required")
		}

		outputPath := viper.GetString("merge.output")
		if outputPath == "" {
			return errors.New("--output is required")
		}

		// Get log level from viper.
		quiet, err := quietFrom(viper.GetString("merge.log-level"))
		if err != nil {
			return err
		}

		// Initialize UI (workflow is nil when quiet).
		start := time.Now()
		var wf *ui.Workflow
		if !quiet {
			wf = ui.NewWorkflow(os.Stdout)
			wf.AddTask("Reading SBOM")
			wf.AddTask("Reading AIBOM(s)")
			wf.AddTask("Merging BOMs")
			wf.AddTask("Writing output")
			wf.Start()
		}
		fail := func(err error) {
			if wf != nil {
				wf.Stop()
				ui.PrintMergeError(os.Stdout, err)
			}
		}

		// Read SBOM (this will be the base).
		if wf != nil {
			wf.StartTask(0, ui.Dim.Render(sbomPath))
		}
		sbom, err := bomio.ReadBOM(sbomPath)
		if err != nil {
			fail(fmt.Errorf("failed to read SBOM: %w", err))
			return err
		}

		if wf != nil {
			sbomComponentCount := 0
			if sbom.Components != nil {
				sbomComponentCount = len(*sbom.Components)
			}
			wf.CompleteTask(0, fmt.Sprintf("%d components loaded", sbomComponentCount))
		}

		// Read all AIBOMs.
		var aiboms []*cdx.BOM
		for i, aibomPath := range aibomPaths {
			if wf != nil {
				msg := aibomPath
				if len(aibomPaths) > 1 {
					msg = fmt.Sprintf("[%d/%d] %s", i+1, len(aibomPaths), aibomPath)
				}
				wf.StartTask(1, ui.Dim.Render(msg))
			}
			aibom, err := bomio.ReadBOM(aibomPath)
			if err != nil {
				fail(fmt.Errorf("failed to read AIBOM %s: %w", aibomPath, err))
				return err
			}
			aiboms = append(aiboms, aibom)
		}
		if wf != nil {
			if len(aiboms) == 1 {
				wf.CompleteTask(1, "AIBOM loaded")
			} else {
				wf.CompleteTask(1, fmt.Sprintf("%d AIBOMs loaded", len(aiboms)))
			}
		}

		// Prepare merge options.
		opts := merger.MergeOptions{
			DeduplicateComponents: viper.GetBool("merge.deduplicate"),
		}

		// Perform merge.
		if wf != nil {
			wf.StartTask(2, "Combining components and metadata")
		}
		result, err := merger.MergeAIBOMsWithSBOM(sbom, aiboms, opts)
		if err != nil {
			fail(fmt.Errorf("failed to merge BOMs: %w", err))
			return err
		}

		// Write merged BOM.
		if wf != nil {
			wf.CompleteTask(2, fmt.Sprintf("%d total components", result.SBOMComponentCount+result.AIBOMComponentCount))
			wf.StartTask(3, ui.Dim.Render(outputPath))
		}
		if err := bomio.WriteBOM(result.MergedBOM, outputPath, ""); err != nil {
			fail(fmt.Errorf("failed to write merged BOM: %w", err))
			return err
		}

		// Print summary.
		if wf != nil {
			wf.CompleteTask(3, "File written successfully")
			wf.Stop()
			ui.PrintMergeSummary(os.Stdout, result, outputPath, len(aiboms), opts.DeduplicateComponents, time.Since(start))
		}

		return nil
	},
}

func init() {
	mergeCmd.Flags().StringSlice("aibom", []string{}, "Path to AIBOM file (can be specified multiple times, required)")
	mergeCmd.Flags().String("sbom", "", "Path to SBOM file (required)")
	mergeCmd.Flags().StringP("output", "o", "", "Output path for merged BOM (required)")
	addDeprecatedFlag(mergeCmd, "format", "f", outputFormatDeprecation)
	mergeCmd.Flags().Bool("deduplicate", true, "Remove duplicate components based on BOM-ref")
	mergeCmd.Flags().String("log-level", "", "Log level: quiet|standard|debug")

	// Bind all flags to viper for config file support.
	bindFlags(mergeCmd, "merge")
	_ = viper.BindPFlag("merge.aiboms", mergeCmd.Flags().Lookup("aibom"))
}
