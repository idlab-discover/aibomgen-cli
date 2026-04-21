
# AIBoMGen CLI

[![Build + Unit Tests](https://github.com/idlab-discover/AIBoMGen-cli/actions/workflows/build.yml/badge.svg)](https://github.com/idlab-discover/AIBoMGen-cli/actions/workflows/build.yml) [![Scan Integration](https://github.com/idlab-discover/AIBoMGen-cli/actions/workflows/integration.yml/badge.svg)](https://github.com/idlab-discover/AIBoMGen-cli/actions/workflows/integration.yml)

Go CLI tool and packages that scan a repository for **Hugging Face model and dataset usage** and emit a **CycloneDX AI Bill of Materials (AIBOM)**.

## Status

What works today:

- `scan` command: walk a directory, detect AI imports across multiple file types, and emit one AIBOM per detected model
- `generate` command: generate an AIBOM directly from one or more Hugging Face model IDs, or interactively browse models
- `validate` command: validate an existing AIBOM with completeness scoring and strict mode
- `completeness` command: score an existing AIBOM against the metadata field registry
- `enrich` command: fill missing metadata fields interactively or from a YAML config file
- `merge` command: merge one or more AIBOMs with an SBOM from another tool (Syft, Trivy, etc.)
- `vuln-scan` command: fetch per-file security scan results from the Hugging Face Hub for every model and dataset component in an AIBOM; optionally inject findings as CycloneDX vulnerabilities
- Hugging Face Hub API and README/model-card fetch to populate metadata fields
- Dataset metadata fetch (API + README) with dataset components linked in the AIBOM
- Security scan data from the HF tree API embedded as BOM properties and `BOM.Vulnerabilities` during generation; disable with `--no-security-scan`
- Multi-source detection: Python (transformers, diffusers, huggingface_hub, sentence-transformers, optimum, peft, LangChain, evaluate, and more), YAML configs, JSON configs, Markdown front-matter, shell scripts, Dockerfiles, and JavaScript/TypeScript
- Rich TUI output built with Charm libraries (Lipgloss, Bubbletea, Huh)
- Interactive Hugging Face model browsing and selection

What is future work:

- AIBOM generation from local model weight files (not hosted on Hugging Face)

## Installation

### Using `go install`

Requires Go:

```bash
go install github.com/idlab-discover/AIBoMGen-cli@latest
```

Ensure `$HOME/go/bin` is in your `PATH`, then verify:

```bash
AIBoMGen-cli --help
```

### From release archive

```bash
tar -xzf AIBoMGen-cli_<version>_linux_amd64.tar.gz
chmod +x AIBoMGen-cli
sudo mv AIBoMGen-cli /usr/local/bin/aibomgen-cli
```

## Build from source

```bash
go test ./...
go build -o aibomgen-cli .
./aibomgen-cli --help
```

## Commands

### `scan`

Walks a directory for AI-related imports across Python, YAML, JSON, Markdown, shell, Dockerfile, and JavaScript/TypeScript files. Writes one AIBOM per detected model. Security scan data from the Hugging Face tree API is embedded in each BOM by default.

```bash
./aibomgen-cli scan -i targets/target-2
./aibomgen-cli scan -i targets/target-3 --format xml --hf-mode online
./aibomgen-cli scan -i targets/target-1 --no-security-scan
```

By default this writes JSON files under `dist/` with filenames derived from the model ID, e.g.:

- `dist/google-bert_bert-base-uncased_aibom.json`
- `dist/templates_model-card-example_aibom.json`

Options:

- `--input, -i <path>`: directory to scan (default: current directory; cannot be used with `--hf-mode=dummy`)
- `--output, -o <path>`: output file path (directory portion is used)
- `--format, -f json|xml|auto` (default: `auto`)
- `--spec <version>`: CycloneDX spec version for output (e.g., `1.4`, `1.5`, `1.6`)
- `--hf-mode online|dummy` (default: `online`)
- `--hf-token <token>`: for gated/private models
- `--hf-timeout <seconds>`
- `--no-security-scan`: skip fetching the Hugging Face security scan tree
- `--log-level quiet|standard|debug`

### `generate`

Generates an AIBOM from one or more Hugging Face model IDs specified directly, or through an interactive model browser. Security scan data is embedded in the BOM by default. Use `scan` instead when you want to detect models from a source directory.

```bash
./aibomgen-cli generate -m google-bert/bert-base-uncased
./aibomgen-cli generate -m gpt2 -m meta-llama/Llama-3.1-8B
./aibomgen-cli generate --interactive
```

Options:

- `--model-id, -m <id>`: Hugging Face model ID (can be specified multiple times or comma-separated)
- `--interactive`: open an interactive model selector (cannot be used with `--model-id`)
- `--output, -o <path>`: output file path (directory portion is used)
- `--format, -f json|xml|auto` (default: `auto`)
- `--spec <version>`: CycloneDX spec version for output (e.g., `1.4`, `1.5`, `1.6`)
- `--hf-mode online|dummy` (default: `online`)
- `--hf-token <token>`: for gated/private models
- `--hf-timeout <seconds>`
- `--no-security-scan`: skip fetching the Hugging Face security scan tree
- `--log-level quiet|standard|debug`

### `validate`

Validates an existing AIBOM file (JSON/XML), runs completeness checks, and can fail in strict mode.

```bash
./aibomgen-cli validate -i dist/google-bert_bert-base-uncased_aibom.json
./aibomgen-cli validate -i dist/google-bert_bert-base-uncased_aibom.json --strict --min-score 0.5
```

Options:

- `--input, -i <path>`: path to AIBOM file (required)
- `--format, -f json|xml|auto`
- `--strict`: fail on missing required fields
- `--min-score 0.0-1.0`: minimum acceptable completeness score
- `--check-model-card`: validate model card fields (default: `false`)
- `--log-level quiet|standard|debug`

### `completeness`

Computes and prints a completeness score for an existing AIBOM using the metadata field registry. Scores both the model component and any linked dataset components.

```bash
./aibomgen-cli completeness -i dist/google-bert_bert-base-uncased_aibom.json
```

Options:

- `--input, -i <path>`: path to AIBOM file (required)
- `--format, -f json|xml|auto`
- `--plain-summary`: print a single-line machine-readable summary (no styling)
- `--log-level quiet|standard|debug`

### `enrich`

Enriches an existing AIBOM by filling missing metadata fields interactively or from a YAML configuration file. Can optionally refetch the latest metadata from Hugging Face before prompting.

```bash
./aibomgen-cli enrich -i dist/google-bert_bert-base-uncased_aibom.json
./aibomgen-cli enrich -i dist/google-bert_bert-base-uncased_aibom.json --strategy interactive
./aibomgen-cli enrich -i dist/google-bert_bert-base-uncased_aibom.json --strategy file --file config/enrichment.yaml
```

Options:

- `--input, -i <path>`: path to existing AIBOM (required)
- `--output, -o <path>`: output file path (default: overwrite input)
- `--format, -f json|xml|auto`: input BOM format
- `--output-format json|xml|auto`: output BOM format (default: same as input)
- `--spec <version>`: CycloneDX spec version for output
- `--strategy interactive|file` (default: `interactive`)
- `--file <path>`: enrichment config file for file-based enrichment (default: `./config/enrichment.yaml`)
- `--required-only`: only enrich required fields
- `--min-weight <float>`: minimum weight threshold for fields to enrich
- `--refetch`: refetch model metadata from Hugging Face Hub before enrichment
- `--no-preview`: skip preview before saving
- `--hf-token <token>`: Hugging Face API token (for refetch)
- `--hf-base-url <url>`: Hugging Face base URL (for refetch)
- `--hf-timeout <seconds>`: Hugging Face API timeout (for refetch)
- `--log-level quiet|standard|debug`

### `vuln-scan`

Fetches per-file security scan results from the Hugging Face Hub for every model and dataset component referenced in an existing AIBOM and displays a vulnerability report. The scanners covered are Cisco Foundation AI (ClamAV), ProtectAI, HuggingFace Pickle Scanner, VirusTotal, and JFrog Research.

Optionally re-injects the findings back into the AIBOM as CycloneDX `BOM.Vulnerabilities` using `--enrich`.

```bash
./aibomgen-cli vuln-scan -i dist/google-bert_bert-base-uncased_aibom.json
./aibomgen-cli vuln-scan -i dist/google-bert_bert-base-uncased_aibom.json --enrich
./aibomgen-cli vuln-scan -i dist/google-bert_bert-base-uncased_aibom.json --enrich --no-preview
```

Options:

- `--input, -i <path>`: path to existing AIBOM (required)
- `--output, -o <path>`: output path when `--enrich` is set (default: overwrite input)
- `--format, -f json|xml|auto`: input BOM format
- `--output-format json|xml|auto`: output BOM format
- `--spec <version>`: CycloneDX spec version for output
- `--enrich`: inject discovered vulnerabilities back into the AIBOM
- `--interactive`: show confirmation prompt before saving (default: `true`, only relevant with `--enrich`)
- `--no-preview`: skip the confirmation prompt (only with `--enrich`)
- `--hf-token <token>`: Hugging Face API token
- `--hf-base-url <url>`: Hugging Face base URL override
- `--hf-timeout <seconds>` (default: `15`)
- `--log-level quiet|standard|debug`

### `merge`

**[BETA]** Merges one or more AIBOMs with an existing SBOM from a different source (e.g., Syft, Trivy) into a single comprehensive BOM.

The SBOM's application metadata is preserved as the main component, while AI/ML model and dataset components from the AIBOM(s) are added to the components list.

```bash
# 1. Generate SBOM for software dependencies using Syft
syft scan . -o cyclonedx-json > sbom.json

# 2. Generate AIBOM for AI/ML components using AIBoMGen
./aibomgen-cli scan -i . -o aibom.json

# 3. Merge them into a comprehensive BOM
./aibomgen-cli merge --aibom aibom.json --sbom sbom.json -o merged.json

# 4. Merge multiple AIBOMs with one SBOM (for projects using multiple models in seperate AIBOM files)
./aibomgen-cli merge --aibom model1_aibom.json --aibom model2_aibom.json --sbom sbom.json -o merged.json
```

Options:

- `--aibom <path>`: path to AIBOM file (can be specified multiple times, required)
- `--sbom <path>`: path to SBOM file (required)
- `--output, -o <path>`: output path for merged BOM (required)
- `--format, -f json|xml|auto`: output format (default: `auto`)
- `--deduplicate`: remove duplicate components based on BOM-ref (default: `true`)
- `--log-level quiet|standard|debug`

### Global flags

- `--config <path>`: config file to use (default: `$HOME/.aibomgen-cli.yaml` or `./config/defaults.yaml`)

The config file is a YAML file that sets default values for any command flag, so you don't have to repeat them on the command line. Keys are namespaced by command:

```yaml
scan:
  hf-token: "hf_..."
  hf-mode: "online"
  log-level: "debug"

validate:
  strict: true
  min-score: 0.5
```

Any flag not passed on the CLI falls back to the config file value. CLI flags always take precedence. See [`config/defaults.yaml`](config/defaults.yaml) for a full reference of all available keys.


## Package overview

Each folder below is a Go package.

### `main`

Entry point. Bootstraps the Cobra root command via [fang](https://github.com/charmbracelet/fang) for styled help output and version injection.

### `cmd/aibomgen-cli`

Cobra CLI wiring: root command, all subcommands, flag parsing, and orchestration into `internal/` and `pkg/` packages. Each command has its own file (`scan.go`, `generate.go`, `validate.go`, `completeness.go`, `enrich.go`, `merge.go`, `vulnscan.go`).

### `pkg/aibomgen/scanner`

Repository scanning used by the `scan` command. Walks a directory tree and applies a multi-rule detection engine across multiple file types:

- **Python** (`.py`, `.ipynb`): `from_pretrained`, `hf_hub_download`, `snapshot_download`, `pipeline`, `InferenceClient`, `SentenceTransformer`, `ORTModel`, `PeftModel`, LangChain loaders, `evaluate.load`, and more — both positional and keyword argument forms
- **YAML** (`.yaml`, `.yml`): `model_name_or_path`, `base_model`, `_name_or_path`, `pretrained_model_name_or_path`
- **JSON** (`.json`): adapter configs, `_name_or_path`, `base_model`
- **Markdown front-matter**: `base_model` field in YAML front-matter
- **Shell scripts and Dockerfiles**: `huggingface-cli download` and `hf download`
- **JavaScript / TypeScript** (`.js`, `.ts`, `.mjs`, `.cjs`): `pipeline` and `from_pretrained` calls via the `@huggingface/transformers` library

### `internal/fetcher`

HTTP clients for fetching model and dataset metadata from the Hugging Face Hub.

- Fetches model metadata via API (`/api/models/:id`) and README (model cards)
- Fetches dataset metadata via API (`/api/datasets/:id`) and README (dataset cards)
- Fetches per-file security scan data via the HF tree API (models and datasets) for embedding in generated BOMs and for the `vuln-scan` command
- Provides a `ModelSearcher` for the interactive model browser
- Used when `--hf-mode online` or when enriching with `--refetch`
- Supports optional bearer token via `--hf-token` for gated/private resources
- Includes dummy implementations for offline/testing scenarios
- Provides markdown extraction utilities for parsing model and dataset cards

### `internal/metadata`

Central field registry describing which CycloneDX AI-BOM fields the tool populates and scores.

- Defines field specifications for model components, dataset components, Hugging Face properties, model card fields, and security scan summaries
- Each field has a key, weight, required status, an `Apply` function, and a `Present` check
- Security scan summary fields: overall status, scanned file count, unsafe file count, caution file count — stored as `Component.Properties`
- Used by `internal/builder` to populate the BOM and by `pkg/aibomgen/completeness` to score it
- Used by `internal/enricher` to identify missing fields and apply new values

### `internal/builder`

Turns fetched metadata into a CycloneDX BOM.

- Builds the metadata component (ML model), applies the full field registry, computes a deterministic PURL and BOM-ref
- `BuildDataset` builds dataset sub-components using the dataset registry
- `InjectSecurityData` appends `BOM.Vulnerabilities` derived from the HF tree security scan entries; maps scanner statuses (`unsafe` → critical, `suspicious` → high, `caution` → medium) to CycloneDX severity ratings. Covered scanners: Cisco Foundation AI (ClamAV), ProtectAI, HuggingFace Pickle Scanner, VirusTotal, JFrog Research

### `pkg/aibomgen/generator`

Orchestrates per-discovery AIBOM generation.

- For each detected model: creates an HTTP client, fetches model API response, README/model card, linked dataset metadata, and optionally the HF security tree; builds the BOM via `internal/builder`
- `BuildDummyBOM` produces a deterministic fixture BOM for offline/testing use
- Reports progress via a `ProgressCallback` used by the UI workflow tracker

### `pkg/aibomgen/bomio`

Read/write helpers for CycloneDX BOMs.

- Supports JSON and XML
- `format=auto` infers format from file extension
- `WriteBOM` and `WriteOutputFiles` support optional CycloneDX spec version selection

### `pkg/aibomgen/completeness`

Computes a completeness score $0..1$ for a BOM using weights defined in the metadata registry. Scores the model component and each linked dataset component separately. Returns missing required and optional field lists.

### `pkg/aibomgen/validator`

Validates an existing AIBOM.

- Performs basic structural checks (nil BOM, missing `metadata.component`)
- Validates CycloneDX spec version
- Runs completeness scoring and can enforce a minimum threshold in strict mode
- Returns per-dataset validation results alongside model results

### `internal/enricher`

Interactively or automatically fills missing metadata fields in an existing AIBOM.

- Supports two strategies: `interactive` (prompts user via Huh forms) and `file` (reads from a YAML config)
- Can refetch the latest model metadata from Hugging Face Hub before prompting
- Enriches both model components and dataset components
- Shows a before/after completeness preview before saving (unless `--no-preview`)
- Respects field weights and required status when deciding what to prompt for

### `internal/vulnscan`

Standalone security scanning package used by the `vuln-scan` command.

- `ScanBOM` fetches per-file security scan results for every model and dataset component in a BOM using the HF tree API
- `ApplyToDOM` writes the discovered `cdx.Vulnerability` objects back into the BOM in-place
- Errors per component are non-fatal; the scan continues for all other components

### `internal/ui`

Comprehensive TUI system built with Charm libraries (Lipgloss, Bubbletea, Huh).

- Provides rich, styled output for all commands
- `Workflow` implements task-based progress tracking with a spinner
- Specialized renderers per command: `GenerateUI`, `ValidationUI`, `CompletenessUI`, `MergerUI`
- `styles.go`: centralized color palette and text styles (bold, muted, success, warning, error)
- `model_selector.go`: interactive fuzzy-search model browser backed by `fetcher.ModelSearcher`

### `pkg/aibomgen/merger`

**[BETA]** BOM merging functionality for combining AIBOMs with SBOMs from other tools.

- Merges one or more AIBOMs with an SBOM, preserving the SBOM's metadata component as the primary component
- AI/ML model and dataset components from AIBOMs are added to the components list
- Supports component deduplication based on BOM-ref
- Merges dependency graphs, compositions, tools, and external references
- Returns a `MergeResult` with per-category component counts and duplicate removal statistics

## Docs and examples

- `targets` are small repositories used in integration tests and examples.
- `docs/` contains design notes and field mapping documentation. These are drafts, not actual docs.


