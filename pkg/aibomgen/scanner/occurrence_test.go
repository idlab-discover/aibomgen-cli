package scanner

import (
	"reflect"
	"testing"
)

func discoveryByID(t *testing.T, ds []Discovery, id string) Discovery {
	t.Helper()
	for _, d := range ds {
		if d.ID == id {
			return d
		}
	}
	t.Fatalf("no discovery for %s in %+v", id, ds)
	return Discovery{}
}

// The same model in two files gives one discovery with two occurrences.
func TestScan_OccurrencesAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a/one.py", "import transformers\n\nm = AutoModel.from_pretrained(\"org/m\")\n")
	writeFile(t, dir, "two.yaml", "model_name_or_path: org/m\n")

	ds, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(ds) != 1 {
		t.Fatalf("want one discovery, got %+v", ds)
	}
	want := []Occurrence{
		{Location: "a/one.py", Line: 3, Method: "from_pretrained", Symbol: "org/m"},
		{Location: "two.yaml", Line: 1, Method: "yaml_model_field", Symbol: "org/m"},
	}
	if got := ds[0].Occurrences; !reflect.DeepEqual(got, want) {
		t.Fatalf("occurrences:\n got %+v\nwant %+v", got, want)
	}
	// The first occurrence decides Path and Method, so they don't depend on worker order.
	if ds[0].Method != "from_pretrained" {
		t.Fatalf("method = %q", ds[0].Method)
	}
}

// A multi-line call is matched per line and again as a whole; each line is reported once,
// with the most confident rule.
func TestScan_MultiLineCallOneOccurrencePerLine(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app.py", "pipe = pipeline(\n    \"text-classification\",\n    model=\"org/m\",\n)\n")

	ds, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	d := discoveryByID(t, ds, "org/m")
	if len(d.Occurrences) != 2 {
		t.Fatalf("occurrences = %+v, want the call start (line 1) and the model= line (line 3)", d.Occurrences)
	}
	if o := d.Occurrences[0]; o.Line != 1 || o.Method != "pipeline_model_kwarg" {
		t.Fatalf("call occurrence = %+v", o)
	}
	if o := d.Occurrences[1]; o.Line != 3 || o.Method != "model_kwarg_slash" {
		t.Fatalf("line occurrence = %+v", o)
	}
}

func TestScan_NotebookOccurrenceHasCell(t *testing.T) {
	dir := t.TempDir()
	nb := `{"cells":[
		{"cell_type":"markdown","source":["# Title"]},
		{"cell_type":"code","source":["import torch\n","m = AutoModel.from_pretrained(\"org/nb\")\n"]}
	]}`
	writeFile(t, dir, "nb.ipynb", nb)

	ds, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	d := discoveryByID(t, ds, "org/nb")
	want := Occurrence{Location: "nb.ipynb", Line: 2, Cell: 2, Method: "from_pretrained", Symbol: "org/nb"}
	if len(d.Occurrences) != 1 || d.Occurrences[0] != want {
		t.Fatalf("occurrences = %+v, want %+v", d.Occurrences, want)
	}
}

func TestRuleConfidenceCoversAllRules(t *testing.T) {
	groups := map[string][]detectionRule{
		"code": codeRules, "yaml": yamlRules, "json": jsonRules,
		"markdown": mdFrontmatterRules, "shell": shellRules, "js": jsRules,
	}
	explicit := map[float32]bool{0.9: true, 0.7: true, 0.6: true, 0.5: true, 0.4: true, 0.3: true}
	for name, rules := range groups {
		for _, r := range rules {
			c := RuleConfidence(r.method)
			if !explicit[c] {
				t.Errorf("%s rule %s has confidence %v", name, r.method, c)
			}
			// 0.5 is also the fallback; only the shell download rule may use it.
			if c == 0.5 && r.method != "hf_cli_download" {
				t.Errorf("%s rule %s falls back to the default confidence; add it to RuleConfidence", name, r.method)
			}
		}
	}
	if RuleConfidence("markdown_inline") != 0.3 {
		t.Errorf("markdown_inline = %v", RuleConfidence("markdown_inline"))
	}
	// Explicit API calls > config values > shell downloads > env vars > prose.
	order := []string{"from_pretrained", "yaml_model_field", "hf_cli_download", "shell_model_env", "markdown_inline"}
	for i := 1; i < len(order); i++ {
		if RuleConfidence(order[i-1]) <= RuleConfidence(order[i]) {
			t.Errorf("%s should score above %s", order[i-1], order[i])
		}
	}
}

func TestDedupeMergesOccurrences(t *testing.T) {
	ds := dedupe([]Discovery{
		{ID: "org/m", Type: "model", Occurrences: []Occurrence{{Location: "b.py", Line: 4, Method: "model_kwarg_slash"}}},
		{ID: "org/m", Type: "model", Occurrences: []Occurrence{{Location: "a.py", Line: 9, Method: "from_pretrained"}}},
		{ID: "org/m", Type: "model", Occurrences: []Occurrence{{Location: "b.py", Line: 4, Method: "pipeline_model_kwarg"}}},
	})
	if len(ds) != 1 {
		t.Fatalf("want one discovery, got %d", len(ds))
	}
	want := []Occurrence{
		{Location: "a.py", Line: 9, Method: "from_pretrained"},
		{Location: "b.py", Line: 4, Method: "pipeline_model_kwarg"},
	}
	if !reflect.DeepEqual(ds[0].Occurrences, want) {
		t.Fatalf("occurrences = %+v, want %+v", ds[0].Occurrences, want)
	}
}
