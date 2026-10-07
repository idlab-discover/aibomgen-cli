package enricher

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"charm.land/huh/v2"
	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/internal/metadata"
)

var update = flag.Bool("update", false, "rewrite golden files")

// lineReader returns one line per Read, because huh's accessible prompts
// build a fresh bufio.Scanner for every question.
type lineReader struct{ lines []string }

func (r *lineReader) Read(p []byte) (int, error) {
	if len(r.lines) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.lines[0]+"\n")
	r.lines = r.lines[1:]
	return n, nil
}

// scriptForm swaps runForm for one that records every group's rendered view
// and then answers the form in accessible mode from lines.
func scriptForm(t *testing.T, groups int, lines []string, out *bytes.Buffer) {
	t.Helper()
	orig := runForm
	t.Cleanup(func() { runForm = orig })
	runForm = func(f *huh.Form) error {
		f.WithWidth(100).WithShowHelp(false)
		f.Init()
		for i := range groups {
			fmt.Fprintf(out, "=== view group %d ===\n%s\n", i, f.View())
			f.NextGroup()
		}
		fmt.Fprintln(out, "=== accessible transcript ===")
		return f.WithAccessible(true).WithInput(&lineReader{lines: lines}).WithOutput(out).Run()
	}
}

func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s mismatch; run go test -run %s -update and diff", name, t.Name())
	}
}

// answers maps a key to its scripted input lines; unlisted keys get one blank line.
func script[K ~string](keys []K, answers map[K][]string) []string {
	var lines []string
	for _, k := range keys {
		if a, ok := answers[k]; ok {
			lines = append(lines, a...)
		} else {
			lines = append(lines, "")
		}
	}
	return lines
}

func writeResult[K ~string](out *bytes.Buffer, changes map[K]string, err error, v any) {
	fmt.Fprintf(out, "=== err ===\n%v\n=== changes ===\n", err)
	keys := make([]string, 0, len(changes))
	for k := range changes {
		keys = append(keys, string(k))
	}
	slices.Sort(keys)
	for _, k := range keys {
		fmt.Fprintf(out, "%s = %q\n", k, changes[K(k)])
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Fprintf(out, "=== result ===\n%s\n", b)
}

// TestEnrichInteractiveGolden pins the model enrichment form (every registry
// field: titles, descriptions, placeholders, options, validation) and the
// values it writes back.
func TestEnrichInteractiveGolden(t *testing.T) {
	specs := metadata.Registry()
	keys := make([]metadata.Key, len(specs))
	for i, s := range specs {
		keys[i] = s.Key
	}
	lines := script(keys, map[metadata.Key][]string{
		metadata.ComponentName:        {"", "acme/override"}, // required: blank is rejected first
		metadata.ComponentTags:        {"alpha, beta"},
		metadata.ComponentLicenses:    {"2"},
		metadata.ComponentDescription: {"A scripted description."},
	})

	var out bytes.Buffer
	scriptForm(t, len(specs)+1, lines, &out)

	comp := &cdx.Component{Name: "orig", ModelCard: &cdx.MLModelCard{}}
	bom := &cdx.BOM{Metadata: &cdx.Metadata{Component: comp}}
	src := metadata.Source{ModelID: "acme/demo-model"}
	tgt := metadata.Target{BOM: bom, Component: comp, ModelCard: comp.ModelCard}

	changes, err := enrichInteractive(bom, specs, src, tgt)
	writeResult(&out, changes, err, bom)
	checkGolden(t, "interactive_model.golden", out.Bytes())
}

// TestEnrichDatasetInteractiveGolden is the dataset analog.
func TestEnrichDatasetInteractiveGolden(t *testing.T) {
	specs := metadata.DatasetRegistry()
	keys := make([]metadata.DatasetKey, len(specs))
	for i, s := range specs {
		keys[i] = s.Key
	}
	lines := script(keys, map[metadata.DatasetKey][]string{
		metadata.DatasetName:        {"", "acme/ds-override"},
		metadata.DatasetTags:        {"x, y"},
		metadata.DatasetLicenses:    {"3"},
		metadata.DatasetDescription: {"Scripted dataset description."},
	})

	var out bytes.Buffer
	scriptForm(t, len(specs)+1, lines, &out)

	comp := &cdx.Component{Name: "acme/demo-ds", Type: cdx.ComponentTypeData}
	src := metadata.DatasetSource{DatasetID: comp.Name}
	tgt := metadata.DatasetTarget{Component: comp}

	changes, err := enrichDatasetInteractive(comp, specs, src, tgt)
	writeResult(&out, changes, err, comp)
	checkGolden(t, "interactive_dataset.golden", out.Bytes())
}
