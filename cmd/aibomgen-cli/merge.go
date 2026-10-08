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
	Use:   "merge [aibom...]",
	Short: "[BETA] Merge one or more AIBOMs with an existing SBOM",
	Long: `[BETA] Merges one or more AI Bill of Materials (AIBOMs) with a Software Bill of Materials (SBOM) from a different source.
This allows you to combine AI/ML component information with traditional software dependencies into a single comprehensive BOM.

The SBOM's application metadata is preserved as the main component, while AI/ML model and dataset components
from the AIBOM(s) are added to the components list.

Example:
  # Generate SBOM with Syft
  syft scan . -o cyclonedx-json > sbom.json

  # Generate AIBOMs with AIBoMGen (one file per model in dist/)
  aibomgen-cli scan .

  # Merge them together
  aibomgen-cli merge dist/*.aibom.cdx.json --sbom sbom.json -o merged.json`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// AIBOMs come from arguments and --aibom (flag, env or config).
		aibomPaths := append(viper.GetStringSlice("merge.aibom"), args...)
		if len(aibomPaths) == 0 {
			return errors.New("no AIBOM given: pass AIBOM files or --aibom")
		}

		sbomPath := viper.GetString("merge.sbom")
		if sbomPath == "" {
			return errors.New("no SBOM given: pass --sbom")
		}

		outputPath := viper.GetString("merge.output")
		if outputPath == "" {
			return errors.New("no output given: pass --output")
		}

		quiet := verbosity < 0

		// Initialize UI (workflow is nil when quiet).
		start := time.Now()
		var wf *ui.Workflow
		if !quiet {
			wf = newWorkflow()
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
			DeduplicateComponents: !viper.GetBool("merge.no-deduplicate"),
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
	mergeCmd.Flags().StringSlice("aibom", []string{}, "AIBOM file; repeatable, or given as arguments")
	mergeCmd.Flags().String("sbom", "", "SBOM file to merge into (required)")
	mergeCmd.Flags().StringP("output", "o", "", "Output file for the merged BOM (required); .xml writes XML, anything else JSON")
	mergeCmd.Flags().Bool("no-deduplicate", false, "Keep components with duplicate BOM-refs")

	bindFlags(mergeCmd, "merge")
}
