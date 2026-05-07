
# AIBoMGen CLI

[![Build + Unit Tests](https://img.shields.io/github/actions/workflow/status/idlab-discover/aibomgen-cli/build.yml?label=Build+%2B+Unit+Tests)](https://github.com/idlab-discover/aibomgen-cli/actions/workflows/build.yml)
[![Scan Integration](https://img.shields.io/github/actions/workflow/status/idlab-discover/aibomgen-cli/integration.yml?label=Scan+Integration)](https://github.com/idlab-discover/aibomgen-cli/actions/workflows/integration.yml)
[![Go version](https://img.shields.io/badge/go-1.25+-00ADD8?logo=go)](go.mod)
[![Go Reference](https://img.shields.io/badge/pkg.go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/idlab-discover/aibomgen-cli)
[![Go Report Card](https://img.shields.io/badge/go%20report-A%2B-brightgreen?logo=go&logoColor=white)](https://goreportcard.com/report/github.com/idlab-discover/aibomgen-cli)
[![GitHub release](https://img.shields.io/github/v/release/idlab-discover/aibomgen-cli)](https://github.com/idlab-discover/aibomgen-cli/releases)
[![License](https://img.shields.io/github/license/idlab-discover/aibomgen-cli)](LICENSE)


Go CLI tool that scans a repository for **Hugging Face model and dataset usage** and emits a **CycloneDX AI Bill of Materials (AIBOM)**.

## Installation

### Using `go install`

Requires Go:

```bash
go install github.com/idlab-discover/aibomgen-cli@latest
```

Ensure `$HOME/go/bin` is in your `PATH`, then verify:

```bash
aibomgen-cli --help
```

**Uninstall:**

```bash
rm "$(go env GOPATH)/bin/aibomgen-cli"
hash -r  # refresh shell command cache
```

### From release archive (preferred)

```bash
VERSION=0.2.0                      # replace with the desired version (without leading 'v')
OS=$(uname -s | tr '[:upper:]' '[:lower:]')  # linux, darwin
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')  # amd64, arm64
curl -L -o aibomgen-cli_${VERSION}_${OS}_${ARCH}.tar.gz \
  https://github.com/idlab-discover/aibomgen-cli/releases/download/v${VERSION}/aibomgen-cli_${VERSION}_${OS}_${ARCH}.tar.gz
tar -xzf aibomgen-cli_${VERSION}_${OS}_${ARCH}.tar.gz
chmod +x aibomgen-cli
sudo mv aibomgen-cli /usr/local/bin/aibomgen-cli
hash -r  # refresh shell command cache without opening a new terminal
```

**Uninstall:**

```bash
sudo rm /usr/local/bin/aibomgen-cli
rm -f ~/.local/share/bash-completion/completions/aibomgen-cli
hash -r  # refresh shell command cache
```

### From source

```bash
go test ./...
go build -o aibomgen-cli .
aibomgen-cli --help
```

**Uninstall:**

```bash
rm ./aibomgen-cli
```

## Configuration Priority

Settings can come from multiple sources. The priority order (lowest to highest) is:

1. **Built-in defaults**: hardcoded in the code
2. **Config file**: values from `~/.aibomgen-cli.yaml` or `./config/defaults.yaml` (or a custom path set with `--config`)
3. **Environment variables**: `AIBOMGEN_*` prefix (e.g., `AIBOMGEN_GENERATE_HF_TOKEN`)
4. **Command-line flags**: `--flag` arguments (highest priority)

Each level overrides the ones below it. For example:

```bash
# 1. Default value from code (if set)
# 2. Config file value (if present)
generate:
  hf-token: "hf_config_value"
# 3. Environment variable (if set)
export AIBOMGEN_GENERATE_HF_TOKEN="hf_env_value"
aibomgen-cli generate -m gpt2
# Result: env_value is used

# 4. Explicit flag (always wins)
aibomgen-cli generate -m gpt2 --hf-token hf_flag_value
# Result: flag_value is used (env var and config are ignored)
```

Config keys with dashes are translated to underscores in env var names:
- `generate.hf-token` → `AIBOMGEN_GENERATE_HF_TOKEN`
- `scan.hf-mode` → `AIBOMGEN_SCAN_HF_MODE`
- `enrich.log-level` → `AIBOMGEN_ENRICH_LOG_LEVEL`

## Commands

### `scan`

Walks a directory for AI-related imports across Python, YAML, JSON, Markdown, shell, Dockerfile, and JavaScript/TypeScript files. Writes one AIBOM per detected model. Security scan data from the Hugging Face tree API is embedded in each BOM by default.

```bash
aibomgen-cli scan -i targets/target-2
aibomgen-cli scan -i targets/target-3 --format xml --hf-mode online
aibomgen-cli scan -i targets/target-1 --no-security-scan
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
aibomgen-cli generate -m google-bert/bert-base-uncased
aibomgen-cli generate -m gpt2 -m meta-llama/Llama-3.1-8B
aibomgen-cli generate --interactive
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
aibomgen-cli validate -i dist/google-bert_bert-base-uncased_aibom.json
aibomgen-cli validate -i dist/google-bert_bert-base-uncased_aibom.json --strict --min-score 0.5
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
aibomgen-cli completeness -i dist/google-bert_bert-base-uncased_aibom.json
```

Options:

- `--input, -i <path>`: path to AIBOM file (required)
- `--format, -f json|xml|auto`
- `--plain-summary`: print a single-line machine-readable summary (no styling)
- `--log-level quiet|standard|debug`

### `enrich`

Enriches an existing AIBOM by filling missing metadata fields interactively or from a YAML configuration file. Can optionally refetch the latest metadata from Hugging Face before prompting.

```bash
aibomgen-cli enrich -i dist/google-bert_bert-base-uncased_aibom.json
aibomgen-cli enrich -i dist/google-bert_bert-base-uncased_aibom.json --strategy interactive
aibomgen-cli enrich -i dist/google-bert_bert-base-uncased_aibom.json --strategy file --file config/enrichment.yaml
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
aibomgen-cli vuln-scan -i dist/google-bert_bert-base-uncased_aibom.json
aibomgen-cli vuln-scan -i dist/google-bert_bert-base-uncased_aibom.json --enrich
aibomgen-cli vuln-scan -i dist/google-bert_bert-base-uncased_aibom.json --enrich --no-preview
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
aibomgen-cli scan -i . -o aibom.json

# 3. Merge them into a comprehensive BOM
aibomgen-cli merge --aibom aibom.json --sbom sbom.json -o merged.json

# 4. Merge multiple AIBOMs with one SBOM (for projects using multiple models in separate AIBOM files)
aibomgen-cli merge --aibom model1_aibom.json --aibom model2_aibom.json --sbom sbom.json -o merged.json
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


## Docs and examples

- API reference: [pkg.go.dev/github.com/idlab-discover/aibomgen-cli](https://pkg.go.dev/github.com/idlab-discover/aibomgen-cli)
- `targets/` — small repositories used in integration tests and examples
- `docs/` — design notes and field mapping documentation (drafts)
- [`config/defaults.yaml`](config/defaults.yaml) — full reference of all config file keys


