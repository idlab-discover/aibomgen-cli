package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── helpers ──────────────────────────────────────────────────────────────────.

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

func findByID(comps []Discovery, id string) (Discovery, bool) {
	for _, c := range comps {
		if c.ID == id {
			return c, true
		}
	}
	return Discovery{}, false
}

// ── core scan tests ───────────────────────────────────────────────────────────.

func TestScanDetectsModelsDedupesEvidence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "use_model.py",
		"from transformers import AutoModel\n"+
			"AutoModel.from_pretrained(\"bert-base-uncased\")\n"+
			"AutoModel.from_pretrained(\"bert-base-uncased\")\n")

	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(comps) != 1 {
		t.Fatalf("expected 1 component, got %d: %+v", len(comps), comps)
	}
	if !strings.Contains(comps[0].Evidence, "line 2") || !strings.Contains(comps[0].Evidence, "line 3") {
		t.Fatalf("expected evidence to include both occurrences, got %q", comps[0].Evidence)
	}
}

// TestMultiLinePipelineNoOrgPrefix verifies that a pipeline() call spread over.
// 3+ lines is detected even when the model ID has no "org/" prefix.
// Regression test for: pipeline(\n    "task",\n    model="single-segment-id"\n).
func TestMultiLinePipelineNoOrgPrefix(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "classify.py",
		"classifier = pipeline(\n"+
			"    \"text-classification\",\n"+
			"    model=\"distilbert-base-uncased-finetuned-sst-2-english\"\n"+
			")\n")

	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	id := "distilbert-base-uncased-finetuned-sst-2-english"
	if _, ok := findByID(comps, id); !ok {
		t.Fatalf("expected %q to be detected; discoveries: %+v", id, comps)
	}
}

func TestScanSkipsUnreadableFiles(t *testing.T) {
	dir := t.TempDir()
	pyPath := filepath.Join(dir, "blocked.py")
	if err := os.WriteFile(pyPath, []byte(`AutoModel.from_pretrained("bert")`), 0o644); err != nil {
		t.Fatalf("write python file: %v", err)
	}
	if err := os.Chmod(pyPath, 0o000); err != nil {
		t.Fatalf("chmod file: %v", err)
	}
	defer func() { _ = os.Chmod(pyPath, 0o644) }()

	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(comps) != 0 {
		t.Fatalf("expected no components for unreadable files, got %d", len(comps))
	}
}

func TestScanInvalidRootReturnsError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if _, err := Scan(missing); err == nil {
		t.Fatalf("expected error for missing root")
	}
}

func TestScanSkipsGitDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".git/hooks/pre-commit",
		`AutoModel.from_pretrained("facebook/opt-1.3b")`)
	writeFile(t, dir, "main.py",
		`AutoModel.from_pretrained("google-bert/bert-base-uncased")`)

	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	for _, c := range comps {
		if strings.Contains(c.Path, ".git") {
			t.Fatalf("should not scan inside .git, but found %s", c.Path)
		}
	}
}

// ── Python pattern tests ──────────────────────────────────────────────────────.

func TestPythonFromPretrainedDoubleQuote(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.py", `model = AutoModel.from_pretrained("facebook/opt-1.3b")`)
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "facebook/opt-1.3b"); !ok {
		t.Fatalf("expected facebook/opt-1.3b, got %+v", comps)
	}
}

func TestPythonFromPretrainedSingleQuote(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.py", `model = AutoModel.from_pretrained('google-bert/bert-base-uncased')`)
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "google-bert/bert-base-uncased"); !ok {
		t.Fatalf("expected google-bert/bert-base-uncased, got %+v", comps)
	}
}

func TestPythonFromPretrainedSingleSegment(t *testing.T) {
	// Single-segment IDs are valid for from_pretrained (e.g., "gpt2").
	dir := t.TempDir()
	writeFile(t, dir, "a.py", `model = AutoModel.from_pretrained("gpt2")`)
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "gpt2"); !ok {
		t.Fatalf("expected gpt2, got %+v", comps)
	}
}

func TestPythonFromPretrainedMultiLine(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.py",
		"model = AutoModel.from_pretrained(\n"+
			`    "facebook/opt-1.3b"`+"\n)\n")
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "facebook/opt-1.3b"); !ok {
		t.Fatalf("expected facebook/opt-1.3b from multi-line call, got %+v", comps)
	}
}

func TestPythonPipelinePositional(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.py", `pipe = pipeline("text-generation", "facebook/opt-1.3b")`)
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "facebook/opt-1.3b"); !ok {
		t.Fatalf("expected facebook/opt-1.3b from pipeline, got %+v", comps)
	}
}

func TestPythonPipelineModelKwarg(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.py", `pipe = pipeline("ner", model="dslim/bert-base-NER")`)
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "dslim/bert-base-NER"); !ok {
		t.Fatalf("expected dslim/bert-base-NER, got %+v", comps)
	}
}

func TestPythonHfHubDownload(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.py",
		`from huggingface_hub import hf_hub_download`+"\n"+
			`hf_hub_download("facebook/opt-1.3b", "config.json")`)
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "facebook/opt-1.3b"); !ok {
		t.Fatalf("expected facebook/opt-1.3b from hf_hub_download, got %+v", comps)
	}
}

func TestPythonSnapshotDownloadKwarg(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.py", `snapshot_download(repo_id="mistralai/Mistral-7B-v0.1")`)
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "mistralai/Mistral-7B-v0.1"); !ok {
		t.Fatalf("expected mistralai/Mistral-7B-v0.1, got %+v", comps)
	}
}

func TestPythonInferenceClient(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.py", `client = InferenceClient("mistralai/Mistral-7B-v0.1")`)
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "mistralai/Mistral-7B-v0.1"); !ok {
		t.Fatalf("expected mistralai/Mistral-7B-v0.1 from InferenceClient, got %+v", comps)
	}
}

func TestPythonSentenceTransformer(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.py", `model = SentenceTransformer("sentence-transformers/all-MiniLM-L6-v2")`)
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "sentence-transformers/all-MiniLM-L6-v2"); !ok {
		t.Fatalf("expected all-MiniLM-L6-v2, got %+v", comps)
	}
}

func TestPythonModelKwargRequiresSlash(t *testing.T) {
	dir := t.TempDir()
	// Generic model= kwarg without slash should NOT be detected.
	writeFile(t, dir, "a.py", `x = some_func(model="local_model_dir")`)
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if len(comps) != 0 {
		t.Fatalf("expected no components for non-HF model kwarg, got %+v", comps)
	}
}

func TestPythonRepoIDKwarg(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.py", `upload_file("file.txt", repo_id="my-org/my-dataset", repo_type="dataset")`)
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "my-org/my-dataset"); !ok {
		t.Fatalf("expected my-org/my-dataset from repo_id kwarg, got %+v", comps)
	}
}

func TestPythonLangchainHuggingFaceHub(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.py", `llm = HuggingFaceHub(repo_id="google/flan-t5-xxl", model_kwargs={"temperature":0})`)
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "google/flan-t5-xxl"); !ok {
		t.Fatalf("expected google/flan-t5-xxl from HuggingFaceHub, got %+v", comps)
	}
}

// ── YAML tests ────────────────────────────────────────────────────────────────.

func TestYAMLModelNameOrPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "train.yaml", "model_name_or_path: meta-llama/Llama-2-7b-hf\n")
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "meta-llama/Llama-2-7b-hf"); !ok {
		t.Fatalf("expected meta-llama/Llama-2-7b-hf from YAML, got %+v", comps)
	}
}

func TestYAMLBaseModel(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.yaml", "base_model: mistralai/Mistral-7B-v0.1\n")
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "mistralai/Mistral-7B-v0.1"); !ok {
		t.Fatalf("expected mistralai/Mistral-7B-v0.1 from YAML base_model, got %+v", comps)
	}
}

func TestYAMLModelFieldWithComment(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.yaml", "model: facebook/opt-1.3b  # open weights\n")
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "facebook/opt-1.3b"); !ok {
		t.Fatalf("expected facebook/opt-1.3b from YAML model field, got %+v", comps)
	}
}

func TestYAMLSingleSegmentNotDetected(t *testing.T) {
	// YAML with a single-segment value should NOT match (too many false positives).
	dir := t.TempDir()
	writeFile(t, dir, "config.yaml", "model: gpt2\n")
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if len(comps) != 0 {
		t.Fatalf("expected no match for single-segment YAML value, got %+v", comps)
	}
}

// ── JSON tests ────────────────────────────────────────────────────────────────.

func TestJSONNameOrPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.json", `{"_name_or_path": "google-bert/bert-base-uncased", "architectures": ["BertModel"]}`)
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "google-bert/bert-base-uncased"); !ok {
		t.Fatalf("expected google-bert/bert-base-uncased from JSON, got %+v", comps)
	}
}

func TestJSONSingleSegmentNameOrPath(t *testing.T) {
	// _name_or_path accepts single-segment (common in HF config.json).
	dir := t.TempDir()
	writeFile(t, dir, "config.json", `{"_name_or_path": "gpt2"}`)
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "gpt2"); !ok {
		t.Fatalf("expected gpt2 from JSON _name_or_path, got %+v", comps)
	}
}

func TestJSONBaseModel(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "adapter_config.json", `{"base_model": "mistralai/Mistral-7B-v0.1"}`)
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "mistralai/Mistral-7B-v0.1"); !ok {
		t.Fatalf("expected mistralai/Mistral-7B-v0.1 from JSON base_model, got %+v", comps)
	}
}

// ── Markdown front-matter tests ───────────────────────────────────────────────.

func TestMarkdownFrontmatterModel(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md",
		"---\n"+
			"language: en\n"+
			"model: bert-base-uncased\n"+ // single segment – should NOT match
			"base_model: google/flan-t5-large\n"+
			"---\n"+
			"# My fine-tuned model\n")
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "google/flan-t5-large"); !ok {
		t.Fatalf("expected google/flan-t5-large from markdown front-matter, got %+v", comps)
	}
	// Single-segment "bert-base-uncased" in frontmatter should not appear.
	if _, ok := findByID(comps, "bert-base-uncased"); ok {
		t.Fatalf("should not detect single-segment model from YAML frontmatter")
	}
}

func TestMarkdownBodyInlineSlash(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md",
		"# Usage\n"+
			"Load the model with `facebook/opt-1.3b`.\n")
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "facebook/opt-1.3b"); !ok {
		t.Fatalf("expected facebook/opt-1.3b from markdown body, got %+v", comps)
	}
}

// ── Shell / Dockerfile tests ──────────────────────────────────────────────────.

func TestShellHFCliDownload(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "download.sh", "huggingface-cli download meta-llama/Llama-2-7b-hf\n")
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "meta-llama/Llama-2-7b-hf"); !ok {
		t.Fatalf("expected meta-llama/Llama-2-7b-hf from shell script, got %+v", comps)
	}
}

func TestDockerfileModelEnv(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Dockerfile", "ENV MODEL_NAME=facebook/opt-1.3b\n")
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "facebook/opt-1.3b"); !ok {
		t.Fatalf("expected facebook/opt-1.3b from Dockerfile ENV, got %+v", comps)
	}
}

// ── Jupyter Notebook tests ────────────────────────────────────────────────────.

func TestNotebookCodeCell(t *testing.T) {
	dir := t.TempDir()
	nb := map[string]any{
		"nbformat":       4,
		"nbformat_minor": 5,
		"cells": []map[string]any{
			{
				"cell_type": "code",
				"source": []string{
					"from transformers import AutoModel\n",
					`model = AutoModel.from_pretrained("facebook/opt-1.3b")`,
				},
			},
		},
	}
	data, _ := json.Marshal(nb)
	writeFile(t, dir, "notebook.ipynb", string(data))

	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "facebook/opt-1.3b"); !ok {
		t.Fatalf("expected facebook/opt-1.3b from notebook cell, got %+v", comps)
	}
}

func TestNotebookMultipleModels(t *testing.T) {
	dir := t.TempDir()
	nb := map[string]any{
		"nbformat":       4,
		"nbformat_minor": 5,
		"cells": []map[string]any{
			{
				"cell_type": "code",
				"source": []string{
					"pipe = pipeline('text-generation', 'gpt2')\n",
					`tokenizer = AutoTokenizer.from_pretrained("google-bert/bert-base-uncased")`,
				},
			},
		},
	}
	data, _ := json.Marshal(nb)
	writeFile(t, dir, "notebook.ipynb", string(data))

	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	ids := make(map[string]bool)
	for _, c := range comps {
		ids[c.ID] = true
	}
	if !ids["gpt2"] {
		t.Fatalf("expected gpt2 from notebook, got %+v", comps)
	}
	if !ids["google-bert/bert-base-uncased"] {
		t.Fatalf("expected google-bert/bert-base-uncased from notebook, got %+v", comps)
	}
}

// ── JS / TS tests ─────────────────────────────────────────────────────────────.

func TestJSPipelinePositional(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app.js",
		`const pipe = await pipeline("text-classification", "distilbert/distilbert-base-uncased-finetuned-sst-2-english");`)
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "distilbert/distilbert-base-uncased-finetuned-sst-2-english"); !ok {
		t.Fatalf("expected distilbert model from JS, got %+v", comps)
	}
}

func TestTSFromPretrained(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "model.ts",
		`const model = await AutoModel.from_pretrained("Xenova/bert-base-uncased");`)
	comps, err := Scan(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, ok := findByID(comps, "Xenova/bert-base-uncased"); !ok {
		t.Fatalf("expected Xenova/bert-base-uncased from TS, got %+v", comps)
	}
}

// ── classifyFile / shouldScanForModelID ───────────────────────────────────────.

func TestClassifyFile(t *testing.T) {
	tests := []struct {
		ext  string
		name string
		want fileClass
	}{
		{ext: ".py", want: fileClassPython},
		{ext: ".ipynb", want: fileClassNotebook},
		{ext: ".yaml", want: fileClassYAML},
		{ext: ".yml", want: fileClassYAML},
		{ext: ".json", want: fileClassJSON},
		{ext: ".md", want: fileClassMarkdown},
		{ext: ".rst", want: fileClassMarkdown},
		{ext: ".sh", want: fileClassShell},
		{ext: ".ts", want: fileClassJS},
		{ext: ".js", want: fileClassJS},
		{ext: ".txt", want: fileClassUnknown},
		{name: "dockerfile", want: fileClassShell},
		{name: "dockerfile.prod", want: fileClassShell},
	}
	for _, tt := range tests {
		got := classifyFile(tt.ext, tt.name)
		if got != tt.want {
			t.Errorf("classifyFile(%q, %q) = %v, want %v", tt.ext, tt.name, got, tt.want)
		}
	}
}

func TestShouldScanForModelID(t *testing.T) {
	tests := []struct {
		ext  string
		want bool
	}{
		{ext: ".py", want: true},
		{ext: ".ipynb", want: true},
		{ext: ".yaml", want: true},
		{ext: ".json", want: true},
		{ext: ".md", want: true},
		{ext: ".sh", want: true},
		{ext: ".ts", want: true},
		{ext: ".txt", want: false},
	}
	for _, tt := range tests {
		if got := shouldScanForModelID(tt.ext); got != tt.want {
			t.Errorf("shouldScanForModelID(%q) = %t, want %t", tt.ext, got, tt.want)
		}
	}
}

// ── dedupe ────────────────────────────────────────────────────────────────────.

func TestDedupeMergesEvidence(t *testing.T) {
	components := []Discovery{
		{ID: "bert", Type: "model", Evidence: "line 2"},
		{ID: "bert", Type: "model", Evidence: "line 3"},
		{ID: "bert", Type: "model", Evidence: "line 3"},
		{ID: "other", Type: "model", Evidence: "line 5"},
	}

	deduped := dedupe(components)
	if len(deduped) != 2 {
		t.Fatalf("expected 2 unique components, got %d", len(deduped))
	}

	var merged Discovery
	for _, c := range deduped {
		if c.ID == "bert" {
			merged = c
		}
	}
	if merged.ID == "" {
		t.Fatalf("expected bert component after dedupe")
	}
	if !strings.Contains(merged.Evidence, "line 2") {
		t.Fatalf("expected merged evidence to include line 2, got %q", merged.Evidence)
	}
	if strings.Count(merged.Evidence, "line 3") != 1 {
		t.Fatalf("expected line 3 evidence once, got %q", merged.Evidence)
	}
}

// ── target-3 integration test ──────────────────────────────────────────────.

// TestScanRepoDifficult scans the targets/target-3 fixture and asserts:.
//   - All expected model IDs across Python, TS, Notebook, YAML, JSON,.
//     Markdown, Shell, and Dockerfile sources are found.
//   - Local paths and variable-indirected IDs are NOT false-positives.
//
// .
// Run with -v to see the full detection report.
func TestScanRepoDifficult(t *testing.T) {
	// Resolve the path from the package dir (internal/scanner → repo root → targets).
	root := filepath.Join("..", "..", "targets", "target-3")
	if _, err := os.Stat(root); os.IsNotExist(err) {
		t.Skipf("targets/target-3 not found at %s", root)
	}

	comps, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Build lookup for assertion helpers.
	byID := make(map[string]Discovery, len(comps))
	for _, c := range comps {
		byID[c.ID] = c
	}

	if testing.Verbose() {
		t.Logf("Detected %d unique model(s):", len(comps))
		ids := make([]string, 0, len(comps))
		for id := range byID {
			ids = append(ids, id)
		}
		// sort for stable output.
		for i := 0; i < len(ids); i++ {
			for j := i + 1; j < len(ids); j++ {
				if ids[i] > ids[j] {
					ids[i], ids[j] = ids[j], ids[i]
				}
			}
		}
		for _, id := range ids {
			t.Logf("  %-60s [%s]", id, byID[id].Method)
		}
	}

	mustDetect := []struct {
		id     string
		reason string
	}{
		// ── Python: from_pretrained (single-line, double-quote) ───────────────.
		{"meta-llama/Llama-3.1-8B", "from_pretrained double-quote (train.py)"},
		{"mistralai/Mistral-7B-v0.1", "from_pretrained single-quote (train.py)"},
		// ── Python: from_pretrained (multi-line open-paren split) ────────────.
		{"Qwen/Qwen2.5-7B-Instruct", "from_pretrained multi-line (train.py)"},
		// ── Python: from_pretrained with explicit kwarg name ─────────────────.
		{"google/flan-t5-xxl", "from_pretrained pretrained_model_name_or_path= (train.py)"},
		// ── Python: pipeline() positional ────────────────────────────────────.
		{"facebook/opt-1.3b", "pipeline positional (train.py)"},
		// ── Python: pipeline() model= kwarg ──────────────────────────────────.
		{"facebook/bart-large-mnli", "pipeline model= kwarg (train.py)"},
		// ── Python: PeftModel.from_pretrained second positional arg ──────────.
		{"timdettmers/guanaco-33b-merged", "PeftModel.from_pretrained second arg (train.py)"},
		// ── Python: SentenceTransformer / CrossEncoder ───────────────────────.
		{"BAAI/bge-large-en-v1.5", "SentenceTransformer (train.py)"},
		{"cross-encoder/ms-marco-MiniLM-L-6-v2", "CrossEncoder (train.py)"},
		// ── Python: hf_hub_download positional ───────────────────────────────.
		{"bartowski/Meta-Llama-3.1-8B-Instruct-GGUF", "hf_hub_download positional (train.py)"},
		// ── Python: hf_hub_download repo_id= kwarg ───────────────────────────.
		{"google/gemma-2-9b-it", "hf_hub_download repo_id= (train.py)"},
		// ── Python: snapshot_download positional ─────────────────────────────.
		{"deepseek-ai/DeepSeek-R1-Distill-Qwen-7B", "snapshot_download positional (train.py)"},
		// ── Python: snapshot_download repo_id= kwarg ─────────────────────────.
		{"microsoft/phi-4", "snapshot_download repo_id= (train.py)"},
		// ── Python: HuggingFacePipeline.from_model_id model_id= kwarg ────────.
		{"tiiuae/falcon-7b-instruct", "HuggingFacePipeline.from_model_id (train.py)"},
		// ── Python: evaluate.load (org/model required) ───────────────────────.
		{"lvwerra/stack-exchange-paired", "evaluate.load org/model (evaluate_models.py)"},
		{"huggingface-course/mse-metric", "evaluate.load org/model (evaluate_models.py)"},
		// ── Python: InferenceClient positional ───────────────────────────────.
		{"meta-llama/Llama-3.1-70B-Instruct", "InferenceClient positional (evaluate_models.py)"},
		// ── Python: InferenceClient model= kwarg ─────────────────────────────.
		{"HuggingFaceH4/zephyr-7b-beta", "InferenceClient model= (evaluate_models.py)"},
		// ── Python: HuggingFaceEndpoint repo_id= ─────────────────────────────.
		{"mistralai/Mistral-7B-Instruct-v0.3", "HuggingFaceEndpoint repo_id= (evaluate_models.py)"},
		// ── Python: generic repo_id= kwarg (upload_file) ─────────────────────.
		{"my-org/my-fine-tuned-llama", "repo_id_kwarg_slash (evaluate_models.py)"},
		// ── Python: generic model= kwarg ─────────────────────────────────────.
		{"stabilityai/stable-diffusion-xl-base-1.0", "model_kwarg_slash (evaluate_models.py)"},
		// ── TypeScript: pipeline positional ──────────────────────────────────.
		{"distilbert/distilbert-base-uncased-finetuned-sst-2-english", "js_pipeline_positional (inference.ts)"},
		// ── TypeScript: .from_pretrained ─────────────────────────────────────.
		{"Xenova/bert-base-uncased", "js_from_pretrained (inference.ts)"},
		// ── TypeScript: model: field ──────────────────────────────────────────.
		{"google/vit-base-patch16-224", "js_model_field (inference.ts)"},
		// ── Jupyter: pipeline positional (code cell) ─────────────────────────.
		{"dslim/bert-base-NER", "notebook pipeline model= (exploration.ipynb)"},
		// ── YAML: base_model / model_name_or_path / hub_model_id ─────────────.
		{"my-org/llama-3.1-8b-axolotl-ft", "yaml hub_model_id (axolotl.yaml)"},
		{"my-org/qwen2.5-7b-sft", "yaml hub_model_id (trl_config.yaml)"},
		{"Qwen/Qwen2.5-72B-Instruct", "yaml teacher_model_name_or_path (trl_config.yaml)"},
		// ── JSON: adapter_config base_model ──────────────────────────────────.
		{"meta-llama/Llama-3.1-8B-Instruct", "from_pretrained (train.py) or Dockerfile ENV"},
		// ── JSON: model_config _name_or_path (single-segment OK for JSON) ────.
		{"distilbert-base-uncased", "json _name_or_path single-segment (model_config.json)"},
		// ── Shell: huggingface-cli download ──────────────────────────────────.
		{"mistralai/Mistral-7B-Instruct-v0.3", "hf_cli_download quoted (download.sh)"},
		// ── Dockerfile: ENV MODEL_NAME= ──────────────────────────────────────.
		{"meta-llama/Llama-3.1-8B-Instruct", "Dockerfile ENV MODEL_NAME (Dockerfile)"},
		// ── Markdown: front-matter base_model ────────────────────────────────.
		{"my-org/llama-3.1-8b-arabic-sft", "markdown front-matter model_id (README.md)"},
		// ── Markdown: inline body reference ──────────────────────────────────.
		{"mistralai/Mistral-7B-v0.1", "markdown body inline (README.md)"},
	}

	failed := false
	for _, tc := range mustDetect {
		if _, ok := byID[tc.id]; !ok {
			t.Errorf("MISSING %q  (%s)", tc.id, tc.reason)
			failed = true
		}
	}

	// These IDs must NOT appear (local paths / variable-indirected).
	mustNotDetect := []struct {
		id     string
		reason string
	}{
		{"./checkpoints/my-finetuned-model", "local relative path"},
		{"/mnt/models/llama-8b", "absolute local path"},
		{"../weights/checkpoint-500", "relative parent path"},
		{"./checkpoints/step-1000", "local notebook path"},
		{"checkpoint-500", "bare checkpoint name (single segment in from_pretrained – borderline)"},
	}
	for _, tc := range mustNotDetect {
		if _, ok := byID[tc.id]; ok {
			t.Errorf("FALSE POSITIVE %q  (%s)", tc.id, tc.reason)
			failed = true
		}
	}

	// Sanity: must find a reasonable total number of unique models.
	const expected = 32
	if len(comps) != expected {
		t.Errorf("expected exactly %d unique models, got %d", expected, len(comps))
		failed = true
	}

	if !failed {
		t.Logf("OK: %d unique models detected, all assertions passed", len(comps))
	}
}

// ── isPlausibleModelID ────────────────────────────────────────────────────────.

func TestIsPlausibleModelID(t *testing.T) {
	valid := []string{
		"gpt2",
		"bert-base-uncased",
		"facebook/opt-1.3b",
		"google-bert/bert-base-uncased",
		"mistralai/Mistral-7B-v0.1",
	}
	invalid := []string{
		"",
		"x",
		"./local/path",
		"../relative",
		"/abs/path",
	}
	for _, id := range valid {
		if !isPlausibleModelID(id) {
			t.Errorf("isPlausibleModelID(%q) should be true", id)
		}
	}
	for _, id := range invalid {
		if isPlausibleModelID(id) {
			t.Errorf("isPlausibleModelID(%q) should be false", id)
		}
	}
}
