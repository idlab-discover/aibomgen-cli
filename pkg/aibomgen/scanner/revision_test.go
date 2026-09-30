package scanner

import (
	"sort"
	"testing"
)

func TestScanCapturesRevisionKwarg(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app.py", `from transformers import AutoModel, pipeline
a = AutoModel.from_pretrained("org/model-a", revision="v1.0")
b = AutoModel.from_pretrained(
    "org/model-b",
    trust_remote_code=True,
    revision='refs/pr/3',
)
c = AutoModel.from_pretrained("org/model-c")
p = pipeline("text-generation", model="org/model-d", revision="main")
x = AutoModel.from_pretrained("org/model-e"); y = f(revision="not-for-e")
`)
	comps, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"org/model-a": "v1.0",
		"org/model-b": "refs/pr/3",
		"org/model-c": "",
		"org/model-d": "main",
		"org/model-e": "",
	}
	// Several rules can match one call (e.g. pipeline_model_kwarg and the generic
	// model= keyword rule); they must agree on the revision, giving one discovery.
	count := map[string]int{}
	for _, c := range comps {
		count[c.ID]++
	}
	for id := range want {
		if count[id] != 1 {
			t.Errorf("%s: %d discoveries, want 1: %+v", id, count[id], comps)
		}
	}
	for id, rev := range want {
		d, ok := findByID(comps, id)
		if !ok {
			t.Fatalf("missing discovery %s in %+v", id, comps)
		}
		if d.Revision != rev {
			t.Errorf("%s revision = %q, want %q", id, d.Revision, rev)
		}
	}
}

func TestScanKeepsRevisionsSeparate(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.py", `m = AutoModel.from_pretrained("org/model", revision="v1")`+"\n")
	writeFile(t, dir, "b.py", `m = AutoModel.from_pretrained("org/model")`+"\n")
	writeFile(t, dir, "c.py", `m = AutoModel.from_pretrained("org/model", revision="v1")`+"\n")
	comps, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	var revs []string
	for _, c := range comps {
		if c.ID == "org/model" {
			revs = append(revs, c.Revision)
		}
	}
	sort.Strings(revs)
	if len(revs) != 2 || revs[0] != "" || revs[1] != "v1" {
		t.Fatalf("expected one discovery per revision, got %q", revs)
	}
}
