
# AIBoMGen CLI

[![Build + Unit Tests](https://img.shields.io/github/actions/workflow/status/idlab-discover/aibomgen-cli/build.yml?label=Build+%2B+Unit+Tests)](https://github.com/idlab-discover/aibomgen-cli/actions/workflows/build.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/idlab-discover/aibomgen-cli/badge)](https://scorecard.dev/viewer/?uri=github.com/idlab-discover/aibomgen-cli)
[![Go version](https://img.shields.io/badge/go-1.27+-00ADD8?logo=go)](go.mod)
[![Go Reference](https://img.shields.io/badge/pkg.go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/idlab-discover/aibomgen-cli)
[![Go Report Card](https://img.shields.io/badge/go%20report-A%2B-brightgreen?logo=go&logoColor=white)](https://goreportcard.com/report/github.com/idlab-discover/aibomgen-cli)
[![GitHub release](https://img.shields.io/github/v/release/idlab-discover/aibomgen-cli)](https://github.com/idlab-discover/aibomgen-cli/releases)
[![License](https://img.shields.io/github/license/idlab-discover/aibomgen-cli)](LICENSE)


Go CLI tool that scans a repository for **Hugging Face model and dataset usage** and emits a **CycloneDX AI Bill of Materials (AIBOM)**.

## AIBoMGen Ecosystem

This repository is part of the broader AIBoMGen ecosystem for generating, analyzing, and validating AI/ML Bills of Materials (AIBOMs).

| Repository | Purpose |
|---|---|
| [AIBoMGen CLI](https://github.com/idlab-discover/aibomgen-cli) | Command-line tool for generating AIBOMs from source code and ML artifacts |
| [AIBoMGen CLI Action](https://github.com/CRA-tools/AIBoMGen-cli-action) | GitHub Action for automated AIBOM generation in CI/CD pipelines |
| [AIBoMGen CLI Dashboard](https://github.com/CRA-tools/aibomgen-cli-dashboard) | Demo dashboard using [AIBoMGen CLI](https://github.com/idlab-discover/aibomgen-cli) |
| [AIBoMGen](https://github.com/idlab-discover/AIBoMGen) | Proof of concept research repository |
| [AIBoMGen Experiments](https://github.com/idlab-discover/AIBoMGen-experiments) | Experimental evaluations of [AIBoMGen](https://github.com/idlab-discover/AIBoMGen)|

## Demo

![demo](docs/draft/demo.gif)

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
- `vuln-scan.hf-token` → `AIBOMGEN_VULN_SCAN_HF_TOKEN`

## Commands

### `scan`

Walks a directory for AI-related imports across Python, YAML, JSON, Markdown, shell, Dockerfile, and JavaScript/TypeScript files. Writes one AIBOM per detected model. Security scan data from the Hugging Face tree API is embedded in each BOM by default.

```bash
aibomgen-cli scan -i targets/target-2
aibomgen-cli scan -i targets/target-3 --format xml --hf-mode online
aibomgen-cli scan -i targets/target-1 --no-security-scan
```

By default this writes one JSON file per model under `dist/`, e.g.:

- `dist/google-bert_bert-base-uncased.aibom.cdx.json`
- `dist/templates_model-card-example.aibom.cdx.json`

File names follow the CycloneDX `*.cdx.json` / `*.cdx.xml` convention: `<model-ref>.aibom.cdx.<json|xml>`, where `<model-ref>` is the requested model ID (plus `_<revision>` when one was requested) with every character outside `[A-Za-z0-9._-]` replaced by `_`. Names that collide get a `_2`, `_3`, … suffix. Existing files are overwritten.

Options:

- `--input, -i <path>`: directory to scan (default: current directory; cannot be used with `--hf-mode=dummy`)
- `--output, -o <dir>`: output directory (default: `dist`)
- `--format, -f json|xml` (default: `json`)
- `--spec <version>`: CycloneDX spec version for output (e.g., `1.5`, `1.6`, `1.7`; default `1.7`)
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

- `--model-id, -m <id>`: Hugging Face model ID, optionally with a revision as `org/name@revision` (can be specified multiple times or comma-separated)
- `--interactive`: open an interactive model selector (cannot be used with `--model-id`)
- `--output, -o <dir>`: output directory (default: `dist`)
- `--format, -f json|xml` (default: `json`)
- `--spec <version>`: CycloneDX spec version for output (e.g., `1.5`, `1.6`, `1.7`; default `1.7`)
- `--hf-mode online|dummy` (default: `online`)
- `--hf-token <token>`: for gated/private models
- `--hf-timeout <seconds>`
- `--no-security-scan`: skip fetching the Hugging Face security scan tree
- `--log-level quiet|standard|debug`

### `validate`

Validates an existing AIBOM file (JSON or XML).

- **Schema:** JSON BOMs are checked against the official CycloneDX JSON schema for their spec version (1.2–1.7, embedded, no network), including formats such as `date-time` and `iri-reference`. XML BOMs and spec 1.0/1.1 skip this check with a warning.
- **Integrity:** bom-refs must be unique; references (model-card datasets, dependencies, vulnerability `affects`) that don't resolve are reported as warnings.
- **Completeness:** `--min-score` is enforced in every mode. With `--strict`, missing required model and dataset fields are errors.
- **Vulnerabilities:** reported as warnings; with `--strict`, those rated at or above `--fail-severity` are errors.

```bash
aibomgen-cli validate -i dist/google-bert_bert-base-uncased.aibom.cdx.json
aibomgen-cli validate -i dist/google-bert_bert-base-uncased.aibom.cdx.json --strict --min-score 0.5
```

Options:

- `--input, -i <path>`: path to AIBOM file (required)
- `--strict`: fail on missing required fields and on vulnerabilities rated at or above `--fail-severity`
- `--fail-severity critical|high|medium|low|info`: lowest vulnerability severity that fails `--strict` (default: `medium`). Vulnerabilities below it, or without a rated severity, are reported as warnings. Hugging Face scanner findings are rated `critical` (unsafe), `high` (suspicious) or `medium` (caution).
- `--min-score 0.0-1.0`: minimum acceptable completeness score
- `--log-level quiet|standard|debug`: `debug` also lists the missing optional fields

Exit codes: `0` valid, `1` error (unreadable file, invalid flag, …), `2` invalid BOM.

### `completeness`

Computes and prints a completeness score for an existing AIBOM using the metadata field registry. Scores both the model component and any linked dataset components.

```bash
aibomgen-cli completeness -i dist/google-bert_bert-base-uncased.aibom.cdx.json
```

Options:

- `--input, -i <path>`: path to AIBOM file (required)
- `--plain-summary`: print a single-line machine-readable summary (no styling)
- `--log-level quiet|standard|debug`

### `enrich`

Enriches an existing AIBOM by filling missing metadata fields interactively or from a YAML configuration file. Can optionally refetch the latest metadata from Hugging Face before prompting.

```bash
aibomgen-cli enrich -i dist/google-bert_bert-base-uncased.aibom.cdx.json
aibomgen-cli enrich -i dist/google-bert_bert-base-uncased.aibom.cdx.json --strategy interactive
aibomgen-cli enrich -i dist/google-bert_bert-base-uncased.aibom.cdx.json --strategy file --file config/enrichment.yaml
```

Options:

- `--input, -i <path>`: path to existing AIBOM (required)
- `--output, -o <path>`: output file path (default: overwrite input); a `.xml` path writes XML, anything else JSON
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
aibomgen-cli vuln-scan -i dist/google-bert_bert-base-uncased.aibom.cdx.json
aibomgen-cli vuln-scan -i dist/google-bert_bert-base-uncased.aibom.cdx.json --enrich
aibomgen-cli vuln-scan -i dist/google-bert_bert-base-uncased.aibom.cdx.json --enrich --no-preview
```

Options:

- `--input, -i <path>`: path to existing AIBOM (required)
- `--output, -o <path>`: output path when `--enrich` is set (default: overwrite input); a `.xml` path writes XML, anything else JSON
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
aibomgen-cli scan -i . -o dist

# 3. Merge them into a comprehensive BOM
aibomgen-cli merge --aibom dist/org_model.aibom.cdx.json --sbom sbom.json -o merged.cdx.json

# 4. Merge multiple AIBOMs with one SBOM (for projects using multiple models in separate AIBOM files)
aibomgen-cli merge --aibom dist/org_model1.aibom.cdx.json --aibom dist/org_model2.aibom.cdx.json --sbom sbom.json -o merged.cdx.json
```

Options:

- `--aibom <path>`: path to AIBOM file (can be specified multiple times, required)
- `--sbom <path>`: path to SBOM file (required)
- `--output, -o <path>`: output path for merged BOM (required); a `.xml` path writes XML, anything else JSON
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
- [`docs/mapping/`](docs/mapping/README.md) — field mapping: where every AIBOM value comes from
- `docs/draft/` — examples and the demo recording
- [`config/defaults.yaml`](config/defaults.yaml) — full reference of all config file keys

## Contact

For inquiries, feel free to reach out

Maintained by:

Wiebe Vandendriessche  
[wiebe.vandendriessche@ugent.be](mailto:wiebe.vandendriessche@ugent.be)  
[LinkedIn](https://www.linkedin.com/in/wiebe-vandendriessche/?locale=en_US)  
[DISCOVER: IDLab, Ghent University – imec](https://idlab.ugent.be/research-teams/discover).

## License

This project is licensed under the terms described in the [LICENSE](./LICENSE) file.

## Acknowledgements

This work has been partially supported by the [CRACY project](https://cra-cy.eu/), funded by the European Union’s Digital Europe Programme under grant agreement No 101190492.
