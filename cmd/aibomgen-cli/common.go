package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
	"github.com/idlab-discover/aibomgen-cli/internal/ui"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/bomio"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/generator"
)

// bindFlags binds every local flag of cmd to viper under prefix.<flag-name>.
func bindFlags(cmd *cobra.Command, prefix string) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = viper.BindPFlag(prefix+"."+f.Name, f)
	})
}

// quietFrom validates a log level (quiet|standard|debug, empty = standard)
// and reports whether it is quiet.
func quietFrom(level string) (bool, error) {
	level = strings.ToLower(strings.TrimSpace(level))
	switch level {
	case "", "quiet", "standard", "debug":
		return level == "quiet", nil
	}
	return false, fmt.Errorf("invalid --log-level %q (expected quiet|standard|debug)", level)
}

// trackProgress starts the processing task on wf (nil when quiet) and returns
// the generator progress callback plus a finish func that completes the
// workflow and prints one result line per model.
func trackProgress(wf *ui.Workflow, processIdx, writeIdx, total int, hasToken bool) (func(generator.ProgressEvent), func(written int)) {
	// A model that fires EventError{Message:"BOM build failed"} but never
	// EventModelComplete produced no AIBOM and is shown as a failure.
	pendingModels := make(map[string]*modelTracker)
	var modelOrder []string // insertion-order IDs for deterministic display
	completed := 0

	if wf != nil {
		wf.StartTask(processIdx, ui.Dim.Render(fmt.Sprintf("0/%d", total)))
	}

	onProgress := func(evt generator.ProgressEvent) {
		if wf == nil {
			return
		}
		// Ensure a tracker exists for this model (EventFetchStart arrives first).
		if _, ok := pendingModels[evt.ModelID]; !ok {
			pendingModels[evt.ModelID] = &modelTracker{}
			modelOrder = append(modelOrder, evt.ModelID)
		}
		t := pendingModels[evt.ModelID]
		switch evt.Type {
		case generator.EventFetchStart:
			wf.UpdateMessage(processIdx, ui.Dim.Render(fmt.Sprintf("%d/%d: %s (fetching)", completed, total, evt.ModelID)))
		case generator.EventFetchAPIComplete:
			t.apiOK = true
		case generator.EventBuildStart:
			wf.UpdateMessage(processIdx, ui.Dim.Render(fmt.Sprintf("%d/%d: %s (building)", completed, total, evt.ModelID)))
		case generator.EventDatasetStart:
			wf.UpdateMessage(processIdx, ui.Dim.Render(fmt.Sprintf("%d/%d: %s → %s", completed, total, evt.ModelID, evt.Message)))
		case generator.EventDatasetComplete:
			t.datasetResults = append(t.datasetResults, datasetResult{id: evt.Message})
		case generator.EventDatasetError:
			t.datasetResults = append(t.datasetResults, datasetResult{id: evt.Message, err: evt.Error})
		case generator.EventModelComplete:
			t.complete = true
			completed++
			if completed < total {
				wf.UpdateMessage(processIdx, ui.Dim.Render(fmt.Sprintf("%d/%d complete", completed, total)))
			}
		case generator.EventError:
			// BOM build failure is terminal for this model (no EventModelComplete follows).
			// Fetch failures are non-fatal; classify them for the summary line.
			if evt.Message != "BOM build failed" {
				if fetcher.IsNotFound(evt.Error) {
					t.notFound = true
				} else if fetcher.IsUnauthorized(evt.Error) && !t.apiOK {
					// 401/403 before the model API succeeded = model is private or non-existent.
					// HF Hub returns 401 for non-existent repos too, so treat this like 404.
					t.notFound = true
				} else {
					t.fetchErr = true
					if t.fetchErrVal == nil {
						t.fetchErrVal = evt.Error
					}
				}
			}
		}
	}

	finish := func(written int) {
		if wf == nil {
			return
		}
		wf.CompleteTask(processIdx, fmt.Sprintf("%d possible model(s)", total))
		wf.StartTask(writeIdx, "")
		wf.CompleteTask(writeIdx, fmt.Sprintf("%d file(s)", written))
		wf.Stop()

		// Print individual model results after workflow completes.
		fmt.Println()
		for _, id := range modelOrder {
			printModelResult(id, pendingModels[id], hasToken)
		}
	}
	return onProgress, finish
}

// resolveOutput fails fast on a format/extension mismatch and returns the
// output directory and the concrete format (json|xml) to write.
func resolveOutput(output, format string) (dir, fmtChosen string, err error) {
	ext := filepath.Ext(output)
	if (format == "xml" && ext == ".json") || (format == "json" && ext == ".xml") {
		return "", "", fmt.Errorf("output path extension %q does not match format %q", ext, format)
	}
	if output == "" {
		output = "dist/aibom.json"
		if format == "xml" {
			output = "dist/aibom.xml"
		}
	}
	fmtChosen = format
	if fmtChosen == "auto" || fmtChosen == "" {
		fmtChosen = "json"
		if filepath.Ext(output) == ".xml" {
			fmtChosen = "xml"
		}
	}
	return filepath.Dir(output), fmtChosen, nil
}

// writeDiscovered writes one file per BOM into dir and prints the summary.
func writeDiscovered(genUI *ui.GenerateUI, boms []generator.DiscoveredBOM, dir, fmtChosen, specVersion string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	fileExt := ".json"
	if fmtChosen == "xml" {
		fileExt = ".xml"
	}
	written, err := bomio.WriteOutputFiles(boms, dir, fileExt, fmtChosen, specVersion)
	if err != nil {
		return err
	}
	if len(written) == 0 {
		genUI.PrintNoBOMsWritten()
		return nil
	}
	genUI.PrintSummary(len(written), dir, fmtChosen)
	return nil
}
