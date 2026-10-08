
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
aibomgen-cli generate gpt2
# Result: env_value is used

# 4. Explicit flag (always wins)
aibomgen-cli generate gpt2 --hf-token hf_flag_value
# Result: flag_value is used (env var and config are ignored)
```

Config keys match the flag names. Dashes and dots become underscores in env var names:
- `generate.hf-token` → `AIBOMGEN_GENERATE_HF_TOKEN`
- `vuln-scan.hf-timeout` → `AIBOMGEN_VULN_SCAN_HF_TIMEOUT`
- `verbose` → `AIBOMGEN_VERBOSE` (top-level keys have no command prefix)

**Hugging Face settings.** `generate`, `scan`, `enrich` and `vuln-scan` share `--hf-token`, `--hf-base-url` and `--hf-timeout` (default `10` seconds). When no token or base URL is configured, the standard Hugging Face env vars `HF_TOKEN` and `HF_ENDPOINT` are used. Prefer `HF_TOKEN` over `--hf-token`: a token on the command line ends up in your shell history and the process list.

## Commands

Commands that read a file or directory take it as an argument; `--input, -i` does the same and is useful in a config file. Passing both is an error.

### `scan`

Walks a directory for AI-related imports across Python, YAML, JSON, Markdown, shell, Dockerfile, and JavaScript/TypeScript files. Writes one AIBOM per detected model. Security scan data from the Hugging Face tree API is embedded in each BOM by default.

```bash
aibomgen-cli scan                       # current directory
aibomgen-cli scan targets/target-3 --format xml
aibomgen-cli scan targets/target-1 --no-security-scan
```

By default this writes one JSON file per model under `dist/`, e.g.:

- `dist/google-bert_bert-base-uncased.aibom.cdx.json`
- `dist/templates_model-card-example.aibom.cdx.json`

File names follow the CycloneDX `*.cdx.json` / `*.cdx.xml` convention: `<model-ref>.aibom.cdx.<json|xml>`, where `<model-ref>` is the requested model ID (plus `_<revision>` when one was requested) with every character outside `[A-Za-z0-9._-]` replaced by `_`. Names that collide get a `_2`, `_3`, … suffix. Existing files are overwritten.

Usage: `aibomgen-cli scan [dir]`

- `--input, -i <dir>`: directory to scan, instead of the argument (default: current directory)
- `--output, -o <dir>`: output directory (default: `dist`)
- `--format, -f json|xml` (default: `json`)
- `--spec <version>`: CycloneDX spec version for output, e.g. `1.6` (default: latest, `1.7`)
- `--no-security-scan`: skip fetching the Hugging Face security scan tree
- `--hf-token`, `--hf-base-url`, `--hf-timeout`: see Hugging Face settings above

### `generate`

Generates an AIBOM from one or more Hugging Face model IDs, or from models picked in an interactive browser. Security scan data is embedded in the BOM by default. Use `scan` instead when you want to detect models from a source directory.

```bash
aibomgen-cli generate google-bert/bert-base-uncased
aibomgen-cli generate gpt2 meta-llama/Llama-3.1-8B@main
aibomgen-cli generate --interactive
```

Usage: `aibomgen-cli generate [model-id...]`

- `--model-id, -m <id>`: model ID as `org/name` or `org/name@revision`; repeatable or comma-separated, combined with the arguments
- `--interactive`: pick models in an interactive selector (needs a terminal; cannot be combined with model IDs)
- `--output, -o <dir>`: output directory (default: `dist`)
- `--format, -f json|xml` (default: `json`)
- `--spec <version>`: CycloneDX spec version for output, e.g. `1.6` (default: latest, `1.7`)
- `--no-security-scan`: skip fetching the Hugging Face security scan tree
- `--hf-token`, `--hf-base-url`, `--hf-timeout`: see Hugging Face settings above

### `validate`

Validates an existing AIBOM file (JSON or XML).

- **Schema:** JSON BOMs are checked against the official CycloneDX JSON schema for their spec version (1.2–1.7, embedded, no network), including formats such as `date-time` and `iri-reference`. XML BOMs and spec 1.0/1.1 skip this check with a warning.
- **Integrity:** bom-refs must be unique; references (model-card datasets, dependencies, vulnerability `affects`) that don't resolve are reported as warnings.
- **Completeness:** `--min-score` is enforced in every mode. With `--strict`, missing required model and dataset fields are errors.
- **Vulnerabilities:** reported as warnings; with `--strict`, those rated at or above `--fail-severity` are errors.

```bash
aibomgen-cli validate dist/google-bert_bert-base-uncased.aibom.cdx.json
aibomgen-cli validate dist/google-bert_bert-base-uncased.aibom.cdx.json --strict --min-score 0.5
```

Usage: `aibomgen-cli validate [file]`

- `--input, -i <file>`: AIBOM file, instead of the argument
- `--strict`: fail on missing required fields and on vulnerabilities rated at or above `--fail-severity`
- `--fail-severity critical|high|medium|low|info`: lowest vulnerability severity that fails `--strict` (default: `medium`). Vulnerabilities below it, or without a rated severity, are reported as warnings. Hugging Face scanner findings are rated `critical` (unsafe), `high` (suspicious) or `medium` (caution).
- `--min-score 0.0-1.0`: minimum acceptable completeness score
- `--json`: print the result as JSON

Exit codes: `0` valid, `1` error (unreadable file, invalid flag, …), `2` invalid BOM.

### `completeness`

Computes and prints a completeness score for an existing AIBOM using the metadata field registry. Scores both the model component and any linked dataset components.

```bash
aibomgen-cli completeness dist/google-bert_bert-base-uncased.aibom.cdx.json
aibomgen-cli completeness dist/google-bert_bert-base-uncased.aibom.cdx.json --json | jq .score
```

Usage: `aibomgen-cli completeness [file]`

- `--input, -i <file>`: AIBOM file, instead of the argument
- `--json`: print the result as JSON

### `enrich`

Enriches an existing AIBOM by filling missing metadata fields: interactively, or from a YAML file with `--file` (see [`config/enrichment.yaml`](config/enrichment.yaml) for an example). By default the latest metadata is refetched from Hugging Face first.

```bash
aibomgen-cli enrich dist/google-bert_bert-base-uncased.aibom.cdx.json
aibomgen-cli enrich dist/google-bert_bert-base-uncased.aibom.cdx.json --file config/enrichment.yaml --yes
```

Usage: `aibomgen-cli enrich [file]`

- `--input, -i <file>`: AIBOM file, instead of the argument
- `--output, -o <file>`: output file (default: overwrite the input); a `.xml` path writes XML, anything else JSON
- `--spec <version>`: CycloneDX spec version for output (default: same as input)
- `--file <path>`: YAML file with the values to fill in, instead of prompting (required when not running in a terminal)
- `--required-only`: only fill required fields
- `--min-weight <float>`: only fill fields with at least this weight
- `--refetch`: refetch model metadata from Hugging Face first (default: `true`; disable with `--refetch=false`)
- `-y, --yes`: save without preview or confirmation (required when not running in a terminal)
- `--hf-token`, `--hf-base-url`, `--hf-timeout`: see Hugging Face settings above

### `vuln-scan`

Fetches per-file security scan results from the Hugging Face Hub for every model and dataset component referenced in an existing AIBOM and displays a vulnerability report. The scanners covered are Cisco Foundation AI (ClamAV), ProtectAI, HuggingFace Pickle Scanner, VirusTotal, and JFrog Research.

Optionally re-injects the findings back into the AIBOM as CycloneDX `BOM.Vulnerabilities` using `--enrich`.

```bash
aibomgen-cli vuln-scan dist/google-bert_bert-base-uncased.aibom.cdx.json
aibomgen-cli vuln-scan dist/google-bert_bert-base-uncased.aibom.cdx.json --enrich
aibomgen-cli vuln-scan dist/google-bert_bert-base-uncased.aibom.cdx.json --enrich --yes
```

Usage: `aibomgen-cli vuln-scan [file]`

- `--input, -i <file>`: AIBOM file, instead of the argument
- `--output, -o <file>`: output file with `--enrich` (default: overwrite the input); a `.xml` path writes XML, anything else JSON
- `--spec <version>`: CycloneDX spec version for output (default: same as input)
- `--enrich`: inject discovered vulnerabilities back into the AIBOM
- `-y, --yes`: apply without preview or confirmation (only with `--enrich`; required when not running in a terminal)
- `--json`: print the scan results as JSON
- `--hf-token`, `--hf-base-url`, `--hf-timeout`: see Hugging Face settings above

### `merge`

**[BETA]** Merges one or more AIBOMs with an existing SBOM from a different source (e.g., Syft, Trivy) into a single comprehensive BOM.

The SBOM's application metadata is preserved as the main component, while AI/ML model and dataset components from the AIBOM(s) are added to the components list.

```bash
# 1. Generate SBOM for software dependencies using Syft
syft scan . -o cyclonedx-json > sbom.json

# 2. Generate AIBOMs for AI/ML components using AIBoMGen
aibomgen-cli scan .

# 3. Merge them into a comprehensive BOM
aibomgen-cli merge dist/*.aibom.cdx.json --sbom sbom.json -o merged.cdx.json
```

Usage: `aibomgen-cli merge [aibom...] --sbom <file> -o <file>`

- `--aibom <file>`: AIBOM file; repeatable, combined with the arguments
- `--sbom <file>`: SBOM file to merge into (required)
- `--output, -o <file>`: output file for the merged BOM (required); a `.xml` path writes XML, anything else JSON
- `--no-deduplicate`: keep components with duplicate BOM-refs (by default duplicates are removed)

### `version`

Prints the version: `aibomgen-cli version`.

### Global flags

- `--config <path>`: config file to use (default: `$HOME/.aibomgen-cli.yaml` or `./config/defaults.yaml`)
- `-q, --quiet`: only print errors and results (no progress output)
- `-v, --verbose`: log to stderr; `-v` shows info (config file used, scan and vuln-scan summaries, missing optional fields in `validate`), `-vv` adds debug detail (HF requests, scanner hits, metadata fields applied, completeness checks, merge dedup)
- `--no-input`: never prompt; commands that would need input fail with a hint instead

`-q` and `-v` are mutually exclusive. `-q` silences output, not prompts.

**Output streams and interactive mode.** Results go to stdout. Logs, prompts and TUIs (the `generate --interactive` model selector, `enrich` forms, and the confirmations of `enrich` and `vuln-scan --enrich`) go to stderr. While a prompt is open, log lines are held back and printed when it closes, so `-v`/`-vv` can be combined with interactive mode. Prompts only run when stdin and stderr are terminals; otherwise (CI, pipes, `--no-input`) the command fails up front with a hint, such as `--file` or `--yes`. Progress spinners are only animated on a terminal without `-v`.

**Machine-readable output.** `validate`, `completeness` and `vuln-scan` accept `--json` to print their result as JSON on stdout, with no styling or progress output. Exit codes are unchanged.

The config file is a YAML file that sets default values for any command flag, so you don't have to repeat them on the command line. Keys are namespaced by command:

```yaml
verbose: 2  # same as -vv, for every command

scan:
  output: "out"
  no-security-scan: true

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
