package scanner

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Discovery represents a Hugging Face model or dataset reference detected in a.
// project file. ID is the Hugging Face repository identifier (e.g.
// "google-bert/bert-base-uncased"), Type is always "huggingface", and Method.
// identifies the detection rule that matched (e.g. "from_pretrained").
type Discovery struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Path     string `json:"path"`
	Evidence string `json:"evidence"`
	Method   string `json:"method"`
	// Revision is the requested model revision (branch, tag or commit), e.g. from.
	// from_pretrained(..., revision="v1.0"); empty means the default branch.
	Revision string `json:"revision,omitempty"`
	// Occurrences lists every place the reference was found, sorted by location and line.
	Occurrences []Occurrence `json:"occurrences,omitempty"`
}

// Occurrence is one place where a model reference was found.
type Occurrence struct {
	// Location is the file path; Scan makes it relative to the scan root, with "/".
	Location string `json:"location"`
	// Line is the 1-based line in the file, or in the notebook cell when Cell > 0.
	Line int `json:"line"`
	// Cell is the 1-based notebook cell, 0 for other files.
	Cell int `json:"cell,omitempty"`
	// Method is the detection rule that matched.
	Method string `json:"method"`
	// Symbol is the model ID exactly as written in the file.
	Symbol string `json:"symbol"`
}

// RuleConfidence returns how confident a detection rule is that it found a real model
// reference: explicit Hugging Face API calls score highest, then generic code keywords,
// config-file values, shell downloads, shell variables and Markdown prose.
func RuleConfidence(method string) float32 {
	switch method {
	case "from_pretrained", "from_pretrained_kwarg",
		"pipeline_positional", "pipeline_model_kwarg",
		"hf_hub_download", "hf_hub_download_kwarg",
		"snapshot_download", "snapshot_download_kwarg",
		"InferenceClient", "InferenceClient_model_kwarg", "InferenceApi",
		"SentenceTransformer", "CrossEncoder",
		"HuggingFaceHub_repo_id", "HuggingFaceEndpoint_repo_id", "HuggingFacePipeline_from_model_id",
		"js_from_pretrained", "js_pipeline_positional":
		return 0.9
	case "model_kwarg_slash", "repo_id_kwarg_slash", "model_id_kwarg_slash", "evaluate_load", "js_model_field":
		return 0.7
	case "yaml_model_field",
		"json_name_or_path", "json_model_name_or_path", "json_base_model", "json_model_field", "json_repo_id",
		"markdown_frontmatter_model":
		return 0.6
	case "hf_cli_download":
		return 0.5
	case "shell_model_env":
		return 0.4
	case "markdown_inline":
		return 0.3
	}
	return 0.5
}

// detectionRule pairs a named detection method with a compiled pattern.
// groupIdx is the capture group that contains the model ID.
type detectionRule struct {
	method   string
	pattern  *regexp.Regexp
	groupIdx int
}

// HF model ID syntax: optional "org/" prefix followed by identifier segments.
// Segment characters: letters, digits, hyphen, underscore, dot.
const (
	hfIDPat      = `[A-Za-z0-9][A-Za-z0-9_.-]*(?:/[A-Za-z0-9][A-Za-z0-9_.-]*)?`
	hfIDSlashPat = `[A-Za-z0-9][A-Za-z0-9_.-]*/[A-Za-z0-9][A-Za-z0-9_.-]*`
)

// q matches a single or double quote character.
const q = `["']`

var (
	// codeRules apply to Python source lines (.py, extracted notebook cells).
	// Patterns cover every major HF Python API across transformers, diffusers,.
	// huggingface_hub, sentence-transformers, optimum, peft, langchain, evaluate, etc.
	codeRules []detectionRule

	// yamlRules apply to YAML config files (.yaml, .yml).
	yamlRules []detectionRule

	// jsonRules apply to JSON files (.json).
	jsonRules []detectionRule

	// mdFrontmatterRules apply to Markdown YAML front-matter sections.
	mdFrontmatterRules []detectionRule

	// shellRules apply to shell scripts (.sh) and Dockerfiles.
	shellRules []detectionRule

	// jsRules apply to JavaScript / TypeScript (.js, .ts, .mjs, .cjs).
	jsRules []detectionRule
)

func init() {
	// ── Python / code rules ─────────────────────────────────────────────────.

	// Generic positional: .from_pretrained("model-id").
	// Covers AutoModel, AutoTokenizer, AutoConfig, BertModel, GPT2Model,.
	// DiffusionPipeline, StableDiffusionPipeline, ORTModel*, PeftModel, etc.
	codeRules = append(codeRules, detectionRule{
		method:   "from_pretrained",
		pattern:  regexp.MustCompile(`\.from_pretrained\(\s*` + q + `(` + hfIDPat + `)` + q),
		groupIdx: 1,
	})

	// Keyword form: from_pretrained(pretrained_model_name_or_path="model-id").
	codeRules = append(codeRules, detectionRule{
		method:   "from_pretrained_kwarg",
		pattern:  regexp.MustCompile(`\.from_pretrained\([^)]*?pretrained_model_name_or_path\s*=\s*` + q + `(` + hfIDPat + `)` + q),
		groupIdx: 1,
	})

	// transformers pipeline – positional second argument (the model):.
	//   pipeline("text-generation", "gpt2").
	//   pipeline("text-generation", "facebook/opt-1.3b").
	codeRules = append(codeRules, detectionRule{
		method:   "pipeline_positional",
		pattern:  regexp.MustCompile(`\bpipeline\(\s*` + q + `[^"']+` + q + `\s*,\s*` + q + `(` + hfIDPat + `)` + q),
		groupIdx: 1,
	})

	// transformers pipeline – named model kwarg:.
	//   pipeline("task", model="facebook/opt-1.3b").
	codeRules = append(codeRules, detectionRule{
		method:   "pipeline_model_kwarg",
		pattern:  regexp.MustCompile(`\bpipeline\([^)]*?\bmodel\s*=\s*` + q + `(` + hfIDPat + `)` + q),
		groupIdx: 1,
	})

	// huggingface_hub.hf_hub_download – positional repo_id.
	codeRules = append(codeRules, detectionRule{
		method:   "hf_hub_download",
		pattern:  regexp.MustCompile(`\bhf_hub_download\(\s*` + q + `(` + hfIDPat + `)` + q),
		groupIdx: 1,
	})

	// huggingface_hub.hf_hub_download – keyword repo_id.
	codeRules = append(codeRules, detectionRule{
		method:   "hf_hub_download_kwarg",
		pattern:  regexp.MustCompile(`\bhf_hub_download\([^)]*?\brepo_id\s*=\s*` + q + `(` + hfIDPat + `)` + q),
		groupIdx: 1,
	})

	// huggingface_hub.snapshot_download – positional.
	codeRules = append(codeRules, detectionRule{
		method:   "snapshot_download",
		pattern:  regexp.MustCompile(`\bsnapshot_download\(\s*` + q + `(` + hfIDPat + `)` + q),
		groupIdx: 1,
	})

	// huggingface_hub.snapshot_download – keyword repo_id.
	codeRules = append(codeRules, detectionRule{
		method:   "snapshot_download_kwarg",
		pattern:  regexp.MustCompile(`\bsnapshot_download\([^)]*?\brepo_id\s*=\s*` + q + `(` + hfIDPat + `)` + q),
		groupIdx: 1,
	})

	// huggingface_hub.InferenceClient – positional model.
	codeRules = append(codeRules, detectionRule{
		method:   "InferenceClient",
		pattern:  regexp.MustCompile(`\bInferenceClient\(\s*` + q + `(` + hfIDPat + `)` + q),
		groupIdx: 1,
	})

	// huggingface_hub.InferenceClient – keyword model.
	codeRules = append(codeRules, detectionRule{
		method:   "InferenceClient_model_kwarg",
		pattern:  regexp.MustCompile(`\bInferenceClient\([^)]*?\bmodel\s*=\s*` + q + `(` + hfIDPat + `)` + q),
		groupIdx: 1,
	})

	// older huggingface_hub.InferenceApi – positional.
	codeRules = append(codeRules, detectionRule{
		method:   "InferenceApi",
		pattern:  regexp.MustCompile(`\bInferenceApi\(\s*` + q + `(` + hfIDPat + `)` + q),
		groupIdx: 1,
	})

	// sentence-transformers: SentenceTransformer("model-id").
	codeRules = append(codeRules, detectionRule{
		method:   "SentenceTransformer",
		pattern:  regexp.MustCompile(`\bSentenceTransformer\(\s*` + q + `(` + hfIDPat + `)` + q),
		groupIdx: 1,
	})

	// sentence-transformers: CrossEncoder("model-id").
	codeRules = append(codeRules, detectionRule{
		method:   "CrossEncoder",
		pattern:  regexp.MustCompile(`\bCrossEncoder\(\s*` + q + `(` + hfIDPat + `)` + q),
		groupIdx: 1,
	})

	// langchain: HuggingFaceHub(repo_id="model-id").
	codeRules = append(codeRules, detectionRule{
		method:   "HuggingFaceHub_repo_id",
		pattern:  regexp.MustCompile(`\bHuggingFaceHub\([^)]*?\brepo_id\s*=\s*` + q + `(` + hfIDPat + `)` + q),
		groupIdx: 1,
	})

	// langchain: HuggingFaceEndpoint(repo_id="model-id").
	codeRules = append(codeRules, detectionRule{
		method:   "HuggingFaceEndpoint_repo_id",
		pattern:  regexp.MustCompile(`\bHuggingFaceEndpoint\([^)]*?\brepo_id\s*=\s*` + q + `(` + hfIDPat + `)` + q),
		groupIdx: 1,
	})

	// langchain: HuggingFacePipeline.from_model_id(model_id="model-id").
	codeRules = append(codeRules, detectionRule{
		method:   "HuggingFacePipeline_from_model_id",
		pattern:  regexp.MustCompile(`\bHuggingFacePipeline\.from_model_id\([^)]*?\bmodel_id\s*=\s*` + q + `(` + hfIDPat + `)` + q),
		groupIdx: 1,
	})

	// evaluate.load("model-id") – e.g. evaluate.load("accuracy").
	// Require org/model to reduce false positives (built-in metric names look like "accuracy").
	codeRules = append(codeRules, detectionRule{
		method:   "evaluate_load",
		pattern:  regexp.MustCompile(`\bevaluate\.load\(\s*` + q + `(` + hfIDSlashPat + `)` + q),
		groupIdx: 1,
	})

	// Generic model= kwarg – require org/model to avoid false positives.
	codeRules = append(codeRules, detectionRule{
		method:   "model_kwarg_slash",
		pattern:  regexp.MustCompile(`\bmodel\s*=\s*` + q + `(` + hfIDSlashPat + `)` + q),
		groupIdx: 1,
	})

	// Generic repo_id= kwarg – require org/model.
	codeRules = append(codeRules, detectionRule{
		method:   "repo_id_kwarg_slash",
		pattern:  regexp.MustCompile(`\brepo_id\s*=\s*` + q + `(` + hfIDSlashPat + `)` + q),
		groupIdx: 1,
	})

	// Generic model_id= kwarg – require org/model.
	codeRules = append(codeRules, detectionRule{
		method:   "model_id_kwarg_slash",
		pattern:  regexp.MustCompile(`\bmodel_id\s*=\s*` + q + `(` + hfIDSlashPat + `)` + q),
		groupIdx: 1,
	})

	// ── YAML rules ──────────────────────────────────────────────────────────.
	// Common config keys in HF Trainer, Accelerate, TRL, Axolotl, LLaMA-Factory, etc.
	// Require org/model form to reduce false positives from freeform text values.
	yamlKeyAlt := `(?:` +
		`model_name_or_path` +
		`|pretrained_model_name_or_path` +
		`|model_name` +
		`|model_checkpoint` +
		`|base_model` +
		`|base_model_name_or_path` +
		`|model_id` +
		`|model` +
		`|repo_id` +
		`|hub_model_id` +
		`|teacher_model_name_or_path` +
		`|student_model_name_or_path` +
		`|foundation_model` +
		`|lm_model` +
		`)`

	yamlRules = append(yamlRules, detectionRule{
		method:   "yaml_model_field",
		pattern:  regexp.MustCompile(`^\s*` + yamlKeyAlt + `\s*:\s*["']?(` + hfIDSlashPat + `)["']?\s*(?:#.*)?$`),
		groupIdx: 1,
	})

	// ── JSON rules ───────────────────────────────────────────────────────────.
	// HF config.json: "_name_or_path" stores the original model ID (may be single-segment).
	jsonRules = append(jsonRules, detectionRule{
		method:   "json_name_or_path",
		pattern:  regexp.MustCompile(`"_name_or_path"\s*:\s*"(` + hfIDPat + `)"`),
		groupIdx: 1,
	})

	// adapter_config.json / training configs.
	jsonRules = append(jsonRules, detectionRule{
		method:   "json_model_name_or_path",
		pattern:  regexp.MustCompile(`"model_name_or_path"\s*:\s*"(` + hfIDSlashPat + `)"`),
		groupIdx: 1,
	})

	jsonRules = append(jsonRules, detectionRule{
		method:   "json_base_model",
		pattern:  regexp.MustCompile(`"base_model"\s*:\s*"(` + hfIDSlashPat + `)"`),
		groupIdx: 1,
	})

	jsonRules = append(jsonRules, detectionRule{
		method:   "json_model_field",
		pattern:  regexp.MustCompile(`"model"\s*:\s*"(` + hfIDSlashPat + `)"`),
		groupIdx: 1,
	})

	jsonRules = append(jsonRules, detectionRule{
		method:   "json_repo_id",
		pattern:  regexp.MustCompile(`"repo_id"\s*:\s*"(` + hfIDSlashPat + `)"`),
		groupIdx: 1,
	})

	// ── Markdown front-matter rules ──────────────────────────────────────────.
	// Model cards on HF Hub embed metadata in YAML front matter:.
	//   model: org/model.
	//   base_model: org/model.
	mdKeyAlt := `(?:model|base_model|model_id|model_name|model_name_or_path|widget_model)`
	mdFrontmatterRules = append(mdFrontmatterRules, detectionRule{
		method:   "markdown_frontmatter_model",
		pattern:  regexp.MustCompile(`^\s*` + mdKeyAlt + `\s*:\s*["']?(` + hfIDSlashPat + `)["']?\s*(?:#.*)?$`),
		groupIdx: 1,
	})

	// ── Shell / Dockerfile rules ──────────────────────────────────────────────.
	// huggingface-cli download org/model.
	shellRules = append(shellRules, detectionRule{
		method:   "hf_cli_download",
		pattern:  regexp.MustCompile(`huggingface-cli\s+download\s+["']?(` + hfIDSlashPat + `)["']?`),
		groupIdx: 1,
	})

	// ENV/ARG model assignments in Dockerfiles and shell scripts:.
	//   MODEL_NAME=org/model  |  export HF_MODEL="org/model".
	shellRules = append(shellRules, detectionRule{
		method:   "shell_model_env",
		pattern:  regexp.MustCompile(`(?:MODEL(?:_NAME|_ID|_PATH)?|HF_MODEL(?:_ID)?|HUGGINGFACE_MODEL)\s*=\s*["']?(` + hfIDSlashPat + `)["']?`),
		groupIdx: 1,
	})

	// ── JavaScript / TypeScript rules ─────────────────────────────────────────.
	// @xenova/transformers or @huggingface/transformers pipeline:.
	//   await pipeline("task", "org/model").
	jsRules = append(jsRules, detectionRule{
		method:   "js_pipeline_positional",
		pattern:  regexp.MustCompile(`\bpipeline\(\s*["'][^"']+["']\s*,\s*["'](` + hfIDPat + `)["']`),
		groupIdx: 1,
	})

	// @xenova/transformers or @huggingface/transformers .from_pretrained.
	jsRules = append(jsRules, detectionRule{
		method:   "js_from_pretrained",
		pattern:  regexp.MustCompile(`\.from_pretrained\(\s*["'](` + hfIDPat + `)["']`),
		groupIdx: 1,
	})

	// @huggingface/inference: hf.textGeneration({ model: "org/model" }).
	jsRules = append(jsRules, detectionRule{
		method:   "js_model_field",
		pattern:  regexp.MustCompile(`\bmodel\s*:\s*["'](` + hfIDSlashPat + `)["']`),
		groupIdx: 1,
	})
}

// Scan walks root and returns deduplicated discovered HF model references.
// Files in common non-source directories (.git, node_modules, __pycache__,.
// virtual-env dirs, build outputs) are skipped automatically.
// Files are processed concurrently using a goroutine worker pool.
// Scan walks the directory tree rooted at root and returns every Hugging Face.
// model and dataset reference it finds. The returned slice is deduplicated by.
// (ID, Path). Hidden directories, virtual environments, and common build.
// output directories are skipped automatically.
func Scan(root string) ([]Discovery, error) {
	// Collect file paths first (fast, serial walk).
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if classifyFile(strings.ToLower(filepath.Ext(d.Name())), strings.ToLower(d.Name())) != fileClassUnknown {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(paths) == 0 {
		return nil, nil
	}

	// Fan-out over a bounded goroutine pool.
	numWorkers := runtime.NumCPU()
	if numWorkers > len(paths) {
		numWorkers = len(paths)
	}

	pathCh := make(chan string, len(paths))
	for _, p := range paths {
		pathCh <- p
	}
	close(pathCh)

	var mu sync.Mutex
	var results []Discovery
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range pathCh {
				hits := scanFile(p)
				if len(hits) > 0 {
					mu.Lock()
					results = append(results, hits...)
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	// Report locations relative to the scan root, and fix the order so that
	// dedupe (which keeps the first Path, Method and Evidence) is deterministic.
	for i := range results {
		for j := range results[i].Occurrences {
			occ := &results[i].Occurrences[j]
			if rel, err := filepath.Rel(root, occ.Location); err == nil {
				occ.Location = filepath.ToSlash(rel)
			}
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		return occurrenceLess(firstOccurrence(results[i]), firstOccurrence(results[j]))
	})

	return dedupe(results), nil
}

func firstOccurrence(d Discovery) Occurrence {
	if len(d.Occurrences) == 0 {
		return Occurrence{}
	}
	return d.Occurrences[0]
}

// occurrenceLess orders occurrences by location, cell, line and method.
func occurrenceLess(a, b Occurrence) bool {
	if a.Location != b.Location {
		return a.Location < b.Location
	}
	if a.Cell != b.Cell {
		return a.Cell < b.Cell
	}
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Method < b.Method
}

// fileClass categorises a file so we know which rule-set to apply.
type fileClass int

const (
	fileClassUnknown  fileClass = iota
	fileClassPython             // .py
	fileClassNotebook           // .ipynb
	fileClassYAML               // .yaml / .yml
	fileClassJSON               // .json
	fileClassMarkdown           // .md / .rst
	fileClassShell              // .sh / Dockerfile* / docker-compose*
	fileClassJS                 // .js / .ts / .mjs / .cjs / .jsx / .tsx
)

func classifyFile(ext, name string) fileClass {
	switch ext {
	case ".py":
		return fileClassPython
	case ".ipynb":
		return fileClassNotebook
	case ".yaml", ".yml":
		return fileClassYAML
	case ".json":
		return fileClassJSON
	case ".md", ".rst":
		return fileClassMarkdown
	case ".sh", ".bash", ".zsh":
		return fileClassShell
	case ".js", ".ts", ".mjs", ".cjs", ".jsx", ".tsx":
		return fileClassJS
	}
	// Name-based matches (no extension).
	switch {
	case name == "dockerfile" ||
		strings.HasPrefix(name, "dockerfile.") ||
		strings.HasPrefix(name, "docker-compose") ||
		name == "containerfile":
		return fileClassShell
	}
	return fileClassUnknown
}

// shouldSkipDir reports whether a directory should be skipped entirely.
func shouldSkipDir(name string) bool {
	switch name {
	case ".git", ".hg", ".svn",
		"node_modules",
		"__pycache__",
		".venv", "venv", "env", ".env",
		".tox",
		"dist", "build", "_build",
		"site-packages",
		".mypy_cache", ".pytest_cache", ".ruff_cache":
		return true
	}
	return false
}

// scanFile dispatches a single file to the appropriate scanner.
func scanFile(path string) []Discovery {
	name := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(name))
	class := classifyFile(ext, name)

	switch class {
	case fileClassPython:
		return scanLines(path, codeRules, true)
	case fileClassNotebook:
		return scanNotebook(path)
	case fileClassYAML:
		return scanLines(path, yamlRules, false)
	case fileClassJSON:
		return scanLines(path, jsonRules, false)
	case fileClassMarkdown:
		return scanMarkdown(path)
	case fileClassShell:
		return scanLines(path, shellRules, false)
	case fileClassJS:
		return scanLines(path, jsRules, false)
	}
	return nil
}

// scanLines reads a file line by line and applies the given rules.
// When multiLine is true, lines belonging to the same open-paren call are.
// accumulated and scanned as a single concatenated string once the parens.
// balance. This correctly handles 3-or-more-line call expressions such as:.
// .
//
//	pipeline(.
//	    "text-classification",.
//	    model="org/model",.
//	).
func scanLines(path string, rules []detectionRule, multiLine bool) []Discovery {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var results []Discovery
	sc := bufio.NewScanner(f)
	lineNum := 0

	// Multi-line accumulation state (only used when multiLine=true).
	var callBuf []string
	callStartLine := 0
	depth := 0

	for sc.Scan() {
		lineNum++
		line := sc.Text()

		// Always scan each individual line.
		results = applyRules(results, rules, line, lineNum, 0, path)

		if !multiLine {
			continue
		}

		// Update paren depth for this line (naïve count; good enough for the.
		// patterns we target and avoids a full parser dependency).
		for _, ch := range line {
			switch ch {
			case '(':
				depth++
			case ')':
				depth--
			}
		}
		if depth < 0 {
			depth = 0 // guard against unmatched ')' in strings/comments
		}

		// Accumulate lines while a call is open.
		if depth > 0 || len(callBuf) > 0 {
			if len(callBuf) == 0 {
				callStartLine = lineNum
			}
			callBuf = append(callBuf, strings.TrimSpace(line))
		}

		// Flush once parens are balanced.
		if depth == 0 && len(callBuf) > 0 {
			combined := strings.Join(callBuf, " ")
			results = applyRules(results, rules, combined, callStartLine, 0, path)
			callBuf = nil
		}
	}
	return results
}

// applyRules tests a single text string against all rules and appends any hits.
// cell is the 1-based notebook cell (0 for other files); lineNum is relative to it.
func applyRules(results []Discovery, rules []detectionRule, text string, lineNum, cell int, path string) []Discovery {
	for _, rule := range rules {
		matches := rule.pattern.FindAllStringSubmatchIndex(text, -1)
		for _, m := range matches {
			if len(m) <= 2*rule.groupIdx+1 || m[2*rule.groupIdx] < 0 {
				continue
			}
			modelID := text[m[2*rule.groupIdx]:m[2*rule.groupIdx+1]]
			if !isPlausibleModelID(modelID) {
				continue
			}
			evidence := rule.method + " at line " + strconv.Itoa(lineNum) + ": " + strings.TrimSpace(text)
			results = append(results, Discovery{
				ID:       modelID,
				Name:     modelID,
				Type:     "model",
				Path:     path,
				Evidence: evidence,
				Method:   rule.method,
				Revision: callRevision(text, m[0], m[1]),
				Occurrences: []Occurrence{{
					Location: path, Line: lineNum, Cell: cell, Method: rule.method, Symbol: modelID,
				}},
			})
		}
	}
	return results
}

// revisionKwargRe matches a Python revision keyword argument: revision="v1.0".
var revisionKwargRe = regexp.MustCompile(`\brevision\s*=\s*["']([^"']+)["']`)

// callRevision returns the revision= keyword argument of the call a rule match
// belongs to: the call whose opening parenthesis lies inside text[start:end], or,
// for keyword matches such as model="org/name", the call enclosing the match.
// Every rule matching the same call therefore reports the same revision. Matches
// outside any call (e.g. top-level assignments) yield "".
func callRevision(text string, start, end int) string {
	open := strings.IndexByte(text[start:end], '(')
	if open >= 0 {
		open += start
	} else if open = enclosingParen(text, start); open < 0 {
		return ""
	}
	depth := 0
	closeIdx := len(text)
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '(':
			depth++
		case ')':
			depth--
		}
		if depth == 0 {
			closeIdx = i
			break
		}
	}
	if m := revisionKwargRe.FindStringSubmatch(text[open:closeIdx]); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// enclosingParen returns the index of the innermost unclosed '(' before pos, or -1.
func enclosingParen(text string, pos int) int {
	depth := 0
	for i := pos - 1; i >= 0; i-- {
		switch text[i] {
		case ')':
			depth++
		case '(':
			if depth == 0 {
				return i
			}
			depth--
		}
	}
	return -1
}

// notebookFormat is a minimal representation of a .ipynb file.
type notebookFormat struct {
	Cells []struct {
		CellType string          `json:"cell_type"`
		Source   json.RawMessage `json:"source"`
	} `json:"cells"`
}

// scanNotebook parses a Jupyter notebook and scans each code cell's source.
// as Python using the standard code rules.
func scanNotebook(path string) []Discovery {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var nb notebookFormat
	if err := json.Unmarshal(data, &nb); err != nil {
		// Fall back to a raw line scan with code rules if parse fails.
		return scanLines(path, codeRules, true)
	}

	var results []Discovery
	for ci, cell := range nb.Cells {
		cellNum := ci + 1
		if cell.CellType != "code" && cell.CellType != "markdown" {
			continue
		}
		rules := codeRules
		multiLine := cell.CellType == "code"
		if cell.CellType == "markdown" {
			rules = mdFrontmatterRules
		}

		// Source is either a JSON string or a JSON array of strings. Array entries
		// end in "\n", so join them before splitting to get the cell's real lines.
		lines := strings.Split(strings.Join(unmarshalSource(cell.Source), ""), "\n")
		lineNum := 0

		// Multi-line accumulation state.
		var callBuf []string
		callStartLine := 0
		depth := 0

		for _, subline := range lines {
			lineNum++
			results = applyRules(results, rules, subline, lineNum, cellNum, path)

			if !multiLine {
				continue
			}

			for _, ch := range subline {
				switch ch {
				case '(':
					depth++
				case ')':
					depth--
				}
			}
			if depth < 0 {
				depth = 0
			}

			if depth > 0 || len(callBuf) > 0 {
				if len(callBuf) == 0 {
					callStartLine = lineNum
				}
				callBuf = append(callBuf, strings.TrimSpace(subline))
			}

			if depth == 0 && len(callBuf) > 0 {
				combined := strings.Join(callBuf, " ")
				results = applyRules(results, rules, combined, callStartLine, cellNum, path)
				callBuf = nil
			}
		}
	}
	return results
}

// unmarshalSource decodes a notebook cell "source" field which is either.
// a JSON string or a JSON array of strings.
func unmarshalSource(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	// Try array first (most common).
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}
	// Fall back to bare string.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []string{s}
	}
	return nil
}

// scanMarkdown scans a Markdown file, applying mdFrontmatterRules to the YAML.
// front-matter block (between leading "---" delimiters) if present, and.
// falling back to a generic org/model inline search in the body.
func scanMarkdown(path string) []Discovery {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	// Inline pattern for bare org/model references in prose.
	inlinePattern := regexp.MustCompile(`\b(` + hfIDSlashPat + `)\b`)

	var results []Discovery
	sc := bufio.NewScanner(f)
	lineNum := 0

	inFrontmatter := false
	frontmatterClosed := false
	firstLine := true

	for sc.Scan() {
		lineNum++
		line := sc.Text()
		trimmed := strings.TrimSpace(line)

		// Detect YAML front-matter delimiters.
		if firstLine {
			firstLine = false
			if trimmed == "---" {
				inFrontmatter = true
				continue
			}
		}

		if inFrontmatter {
			if trimmed == "---" || trimmed == "..." {
				inFrontmatter = false
				frontmatterClosed = true
				continue
			}
			results = applyRules(results, mdFrontmatterRules, line, lineNum, 0, path)
			continue
		}

		// Body of the document – scan for inline org/model references.
		if frontmatterClosed || !inFrontmatter {
			matches := inlinePattern.FindAllStringSubmatch(line, -1)
			for _, m := range matches {
				if len(m) < 2 {
					continue
				}
				modelID := m[1]
				if !isPlausibleModelID(modelID) {
					continue
				}
				evidence := "markdown_inline at line " + strconv.Itoa(lineNum) + ": " + strings.TrimSpace(line)
				results = append(results, Discovery{
					ID:       modelID,
					Name:     modelID,
					Type:     "model",
					Path:     path,
					Evidence: evidence,
					Method:   "markdown_inline",
					Occurrences: []Occurrence{{
						Location: path, Line: lineNum, Method: "markdown_inline", Symbol: modelID,
					}},
				})
			}
		}
	}
	return results
}

// isPlausibleModelID applies basic sanity checks to reject obvious noise.
func isPlausibleModelID(id string) bool {
	if id == "" || len(id) < 2 || len(id) > 200 {
		return false
	}
	// Reject pure version strings like "1.0", "3.14".
	if versRe.MatchString(id) && !strings.ContainsAny(id, "_-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return false
	}
	// Reject common local paths.
	if strings.HasPrefix(id, "./") || strings.HasPrefix(id, "../") || strings.HasPrefix(id, "/") {
		return false
	}
	return true
}

var versRe = regexp.MustCompile(`^\d+\.\d+`)

// dedupe merges discoveries with identical Type+ID+Revision, concatenating distinct
// evidence strings and merging occurrences.
func dedupe(components []Discovery) []Discovery {
	index := make(map[string]Discovery)
	var order []string
	for _, c := range components {
		key := c.Type + "::" + c.ID + "@" + c.Revision
		if existing, ok := index[key]; ok {
			if !strings.Contains(existing.Evidence, c.Evidence) {
				existing.Evidence += ". " + c.Evidence
			}
			// Keep the first seen Method; additional methods are visible via Occurrences.
			existing.Occurrences = append(existing.Occurrences, c.Occurrences...)
			index[key] = existing
		} else {
			index[key] = c
			order = append(order, key)
		}
	}
	out := make([]Discovery, 0, len(index))
	for _, key := range order {
		d := index[key]
		d.Occurrences = mergeOccurrences(d.Occurrences)
		out = append(out, d)
	}
	return out
}

// mergeOccurrences keeps one occurrence per (location, cell, line): the per-line pass,
// the multi-line call pass and overlapping rules can all hit the same line. The rule
// with the highest confidence wins. The result is sorted.
func mergeOccurrences(occs []Occurrence) []Occurrence {
	if len(occs) == 0 {
		return nil
	}
	type pos struct {
		loc        string
		cell, line int
	}
	best := make(map[pos]Occurrence, len(occs))
	for _, o := range occs {
		k := pos{o.Location, o.Cell, o.Line}
		cur, ok := best[k]
		if !ok || RuleConfidence(o.Method) > RuleConfidence(cur.Method) ||
			(RuleConfidence(o.Method) == RuleConfidence(cur.Method) && o.Method < cur.Method) {
			best[k] = o
		}
	}
	out := make([]Occurrence, 0, len(best))
	for _, o := range best {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return occurrenceLess(out[i], out[j]) })
	return out
}
