package cmd

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

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

// verbosity is the resolved output level for the running command:
// -1 quiet, 0 standard, 1 verbose (-v), 2 debug (-vv). Set in PersistentPreRunE.
var verbosity int

// resolveVerbosity reads --quiet/--verbose (or their config/env keys).
func resolveVerbosity() (int, error) {
	quiet, verbose := viper.GetBool("quiet"), viper.GetInt("verbose")
	switch {
	case quiet && verbose > 0:
		return 0, errors.New("cannot combine --quiet with --verbose")
	case quiet:
		return -1, nil
	}
	return min(verbose, 2), nil
}

// slogLevel maps a verbosity to the minimum level logged to stderr.
func slogLevel(v int) slog.Level {
	switch {
	case v < 0:
		return slog.LevelError
	case v == 1:
		return slog.LevelInfo
	case v >= 2:
		return slog.LevelDebug
	}
	return slog.LevelWarn
}

// setLogger installs the default slog logger at the level for v. It writes to
// stderr through ui.LogWriter, which holds lines back while a prompt is open.
func setLogger(v int) {
	slog.SetDefault(slog.New(slog.NewTextHandler(ui.LogWriter(), &slog.HandlerOptions{Level: slogLevel(v), ReplaceAttr: dropTime})))
}

// dropTime removes the timestamp from log lines; it is noise for a CLI.
func dropTime(groups []string, a slog.Attr) slog.Attr {
	if len(groups) == 0 && a.Key == slog.TimeKey {
		return slog.Attr{}
	}
	return a
}

// newWorkflow returns a progress workflow on stdout. With logs enabled (-v) or
// stdout not a terminal it renders only the final state: no spinner redraws to
// erase log lines or leave escape codes in redirected output.
func newWorkflow() *ui.Workflow {
	wf := ui.NewWorkflow(os.Stdout)
	wf.Static = verbosity >= 1 || !ui.IsTTY(os.Stdout)
	return wf
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

// outputFormat validates a --format value for directory outputs: json or xml.
func outputFormat(format string) (string, error) {
	switch format = strings.ToLower(strings.TrimSpace(format)); format {
	case "json", "xml":
		return format, nil
	}
	return "", fmt.Errorf("invalid format %q for --format (expected json|xml)", format)
}

// writeDiscovered writes one file per BOM into dir and prints the summary.
func writeDiscovered(genUI *ui.GenerateUI, boms []generator.DiscoveredBOM, dir, format, specVersion string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	written, err := bomio.WriteOutputFiles(boms, dir, format, specVersion)
	if err != nil {
		return err
	}
	if len(written) == 0 {
		genUI.PrintNoBOMsWritten()
		return nil
	}
	genUI.PrintSummary(written, format)
	return nil
}

// inputFrom returns the command's single input: the positional argument if
// given, otherwise the <prefix>.input value (flag, env or config).
func inputFrom(cmd *cobra.Command, args []string, prefix string) (string, error) {
	if len(args) == 0 {
		return strings.TrimSpace(viper.GetString(prefix + ".input")), nil
	}
	if cmd.Flags().Changed("input") {
		return "", errors.New("pass the input as an argument or with --input, not both")
	}
	return args[0], nil
}

// requireInput is inputFrom for commands that cannot default the input.
func requireInput(cmd *cobra.Command, args []string, prefix string) (string, error) {
	in, err := inputFrom(cmd, args, prefix)
	if err == nil && in == "" {
		err = errors.New("no input given: pass a file path or --input")
	}
	return in, err
}

// hfSettings are the Hugging Face options shared by generate, scan, enrich and vuln-scan.
type hfSettings struct {
	Token   string
	BaseURL string
	Timeout time.Duration
}

// hfOptions reads the HF flags under prefix, falling back to the standard
// huggingface_hub env vars HF_TOKEN and HF_ENDPOINT.
func hfOptions(prefix string) hfSettings {
	return hfSettings{
		Token:   cmp.Or(viper.GetString(prefix+".hf-token"), os.Getenv("HF_TOKEN")),
		BaseURL: cmp.Or(viper.GetString(prefix+".hf-base-url"), os.Getenv("HF_ENDPOINT")),
		Timeout: time.Duration(viper.GetInt(prefix+".hf-timeout")) * time.Second,
	}
}

// addHFFlags registers the shared Hugging Face flags on c.
func addHFFlags(c *cobra.Command) {
	c.Flags().String("hf-token", "", "Hugging Face access token (default $HF_TOKEN)")
	c.Flags().String("hf-base-url", "", "Hugging Face endpoint (default $HF_ENDPOINT or https://huggingface.co)")
	c.Flags().Int("hf-timeout", 10, "Timeout in seconds per Hugging Face API request")
}

// addHFModeFlag registers the hidden --hf-mode flag: "dummy" builds a fixture
// BOM without network access (CI smoke tests), "online" is normal operation.
func addHFModeFlag(c *cobra.Command) {
	c.Flags().String("hf-mode", "online", "Hugging Face metadata mode: online|dummy")
	_ = c.Flags().MarkHidden("hf-mode")
}

// hfMode validates <prefix>.hf-mode.
func hfMode(prefix string) (string, error) {
	switch mode := strings.ToLower(strings.TrimSpace(viper.GetString(prefix + ".hf-mode"))); mode {
	case "online", "dummy":
		return mode, nil
	default:
		return "", fmt.Errorf("invalid mode %q for --hf-mode (expected online|dummy)", mode)
	}
}

// writeJSON writes v to w as indented JSON (the --json output of a command).
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
