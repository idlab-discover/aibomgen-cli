# Model component mapping

The model is written as `metadata.component` with `type: machine-learning-model`. Sources are listed in precedence order, and the first one with a usable value wins (see [README.md](README.md#general-rules)). Notation: `HF.x` is a model API field, `cardData.x` a key of the API's `cardData`, and `README …` the model README. "Namespace" is the `org` part of the resolved model ID (`HF.id`, then `HF.modelId`, then the requested ID, whichever is first in `org/name` form), falling back to `HF.author`.

## Component fields

| CycloneDX field | FieldSpec key | Sources (in order) | Transform | Weight |
|---|---|---|---|---|
| `name` | `BOM.metadata.component.name` | `HF.id` → `HF.modelId` → discovery name → requested ID | Trimmed. The resolved ID wins, so `gpt2` becomes `openai-community/gpt2`. | 1.0, required |
| `externalReferences` | `BOM.metadata.component.externalReferences` | resolved ID (`HF.id`, else requested ID); README "Paper" and "Demo" bullets | `website` = `https://huggingface.co/{id}`; `documentation` = Paper URL; `other` = Demo URL. A bullet only counts if it holds an `http(s)` URL; the URL is taken from a Markdown link's target. | 0.5 |
| `tags` | `BOM.metadata.component.tags` | `HF.tags` → README front matter `tags` | Trimmed, deduplicated | 0.5 |
| `licenses` | `BOM.metadata.component.licenses` | `cardData.license` → `HF.license` → `license:*` tags → README front matter `license`; plus `license_name` and `license_link` | SPDX `id` or `name`, see [licenses.md](licenses.md) | 1.0 |
| `hashes` | `BOM.metadata.component.hashes` | `HF.sha` | `[{alg: SHA-1, content: sha}]` (the git commit SHA) | 1.0 |
| `manufacturer` | `BOM.metadata.component.manufacturer` | `HF.author` → README "Developed by" (placeholders skipped) | `{name}`, plus `url: [https://huggingface.co/{namespace}]` when the name is the namespace | 0.5 |
| `group` | `BOM.metadata.component.group` | namespace → README "Developed by" (placeholders skipped) | Only IDs with an `org/` part yield a namespace; `gpt2` without HF data gets no group | 0.25 |
| `supplier` | `BOM.metadata.component.supplier` | namespace | `{name: namespace, url: [https://huggingface.co/{namespace}]}` | 0.5 |
| `authors` | `BOM.metadata.component.authors` | README "Developed by" (placeholders skipped) → namespace | `[{name}]` | 0.5 |
| `version` | `BOM.metadata.component.version` | requested revision, if it isn't a 40-hex commit SHA → `HF.sha` | Named revision as is (e.g. `v1.0`, `main`); otherwise the lowercased commit SHA, equal to the purl version | 0.5 |

`purl` and `bom-ref` are computed after these fields, see [identity-and-links.md](identity-and-links.md).

## Model card (`modelCard`)

| CycloneDX field | FieldSpec key | Sources (in order) | Transform | Weight |
|---|---|---|---|---|
| `modelParameters.task` | `BOM.metadata.component.modelCard.modelParameters.task` | `HF.pipeline_tag` → README front matter `model-index[0].results[0].task.type` | Trimmed | 1.0 |
| `modelParameters.architectureFamily` | `BOM.metadata.component.modelCard.modelParameters.architectureFamily` | `HF.config.model_type` | Trimmed (e.g. `bert`) | 0.5 |
| `modelParameters.modelArchitecture` | `BOM.metadata.component.modelCard.modelParameters.modelArchitecture` | `HF.config.architectures[0]` | Trimmed (e.g. `BertForMaskedLM`) | 0.5 |
| `modelParameters.datasets` | `BOM.metadata.component.modelCard.modelParameters.datasets` | `cardData.datasets` plus `dataset:*` tags → README front matter `datasets` | Linked to the data components after they are built: `{ref: <bom-ref>}` or an inline `{type: dataset, name}`, see [identity-and-links.md](identity-and-links.md#dataset-references) | 0.5 |
| `considerations.useCases` | `BOM.metadata.component.modelCard.considerations.useCases` | README use-case section; README out-of-scope section (heading aliases below) | Section text is cleaned (see below). Out-of-scope text is prefixed with `out-of-scope: ` | 0.5 |
| `considerations.technicalLimitations` | `BOM.metadata.component.modelCard.considerations.technicalLimitations` | README limitations section (heading aliases below) | One entry with the cleaned section text | 0.5 |
| `considerations.ethicalConsiderations` | `BOM.metadata.component.modelCard.considerations.ethicalConsiderations` | README ethics section → limitations section (name); README "Recommendations" section (mitigation) | One entry with cleaned text. If only recommendations exist, the name is `bias_risks_limitations`. | 0.25 |
| `quantitativeAnalysis.performanceMetrics` | `BOM.metadata.component.modelCard.quantitativeAnalysis.performanceMetrics` | README front matter `model-index[0].results[0].metrics[]` (type and value) plus front matter `metrics` (type only) → README "Metrics" section (type) and "Results" section (value) | If only "Results" exists, the type is `testing_metrics` | 0.5 |
| `considerations.environmentalConsiderations.properties` | `BOM.metadata.component.modelCard.considerations.environmentalConsiderations.properties` | README bullets "Hardware Type", "Hours used", "Cloud Provider", "Compute Region", "Carbon Emitted" | Properties `hardwareType`, `hoursUsed`, `cloudProvider`, `computeRegion`, `carbonEmitted` | 0.25 |

### Considerations sections

Most model cards don't use the exact Hugging Face template headings, so each considerations source accepts a list of heading aliases. The aliases are tried in order, and the first section with real text wins.

| Source | Heading aliases (in order) |
|---|---|
| Use cases | Direct Use, Uses, Intended uses, Intended use, Intended uses & limitations, Intended uses and limitations, How to use |
| Out-of-scope | Out-of-Scope Use, Out-of-scope uses, Misuse and out-of-scope use, Misuse, Malicious Use, and Out-of-Scope Use |
| Limitations | Bias, Risks, and Limitations, Limitations, Limitations and bias, Limitations and biases, Bias and limitations, Known limitations, Risks and limitations |
| Ethics | Bias, Ethical considerations, Ethics, Responsible AI |
| Recommendations | Recommendations |

Matching rules:
- **Headings:** any level (`#` to `######`). Matching ignores case, `*`, `_`, `` ` ``, closing `#`s and trailing `:.!?`. Lines inside fenced code blocks are never headings.
- **Section body:** runs to the next heading of any level, so a combined section like "Intended uses & limitations" gives its intro text to the use cases. A nested "Limitations and bias" subsection gives the limitations.
- **Skipped sections:** a section that is empty after cleaning is skipped. A section holding only `[More Information Needed]` is skipped too, and kept verbatim only when no alias has real text.
- **Cleaning:** fenced code blocks, HTML comments and tags, and Markdown images are removed, and blank lines are collapsed.
- **Length cap:** the text is capped at 1000 characters. It is cut at the last sentence end, or else at a word boundary followed by `…`.

## Properties

Each property below is written to `properties[]` with the name shown (the key's part after `properties.`).

| Property | FieldSpec key | Source | Condition / transform | Weight |
|---|---|---|---|---|
| `huggingface:lastModified` | `BOM.metadata.component.properties.huggingface:lastModified` | `HF.lastModified` | Non-empty | 0.2 |
| `huggingface:createdAt` | `BOM.metadata.component.properties.huggingface:createdAt` | `HF.createdAt` | Non-empty | 0.2 |
| `huggingface:language` | `BOM.metadata.component.properties.huggingface:language` | `cardData.language` | A string, or a list joined with `,` | 0.2 |
| `huggingface:usedStorage` | `BOM.metadata.component.properties.huggingface:usedStorage` | `HF.usedStorage` | Greater than 0, in bytes | 0.2 |
| `huggingface:private` | `BOM.metadata.component.properties.huggingface:private` | `HF.private` | Always set when API data exists (`true` or `false`) | 0.2 |
| `huggingface:libraryName` | `BOM.metadata.component.properties.huggingface:libraryName` | `HF.library_name` | Non-empty | 0.2 |
| `huggingface:downloads` | `BOM.metadata.component.properties.huggingface:downloads` | `HF.downloads` | Greater than 0 | 0.2 |
| `huggingface:likes` | `BOM.metadata.component.properties.huggingface:likes` | `HF.likes` | Greater than 0 | 0.2 |
| `huggingface:baseModel` | `BOM.metadata.component.properties.huggingface:baseModel` | README front matter `base_model` | A string, or a list joined with `,` (e.g. merged models) | 0.2 |
| `huggingface:modelCardContact` | `BOM.metadata.component.properties.huggingface:modelCardContact` | README "Model Card Contact" section | Non-empty | 0.2 |
| `aibomgen.type`, `aibomgen.evidence`, `aibomgen.path` | `aibomgen.evidence` | Discovery type, evidence text and file path | Written when evidence properties are enabled (the default). The evidence records the ID as it was found or requested. | 0 (not scored) |

## Security scan

These fields come from the model tree API (skipped with `--no-security-scan`). The properties are only written when the tree has at least one entry.

| Output | FieldSpec key | Derivation | Weight |
|---|---|---|---|
| property `huggingface:security:overallStatus` | `BOM.metadata.component.properties.huggingface:security:overallStatus` | `unsafe` if any file is unsafe, else `caution` if any file needs caution, else `safe` | 0.3 |
| property `huggingface:security:scannedFileCount` | `BOM.metadata.component.properties.huggingface:security:scannedFileCount` | Number of files with a security status | 0.2 |
| property `huggingface:security:unsafeFileCount` | `BOM.metadata.component.properties.huggingface:security:unsafeFileCount` | Number of files with status `unsafe` | 0.2 |
| property `huggingface:security:cautionFileCount` | `BOM.metadata.component.properties.huggingface:security:cautionFileCount` | Number of files with status `caution` | 0.2 |

`vulnerabilities[]` isn't a FieldSpec. `InjectSecurityData` (`internal/builder/security_builder.go`) adds one entry for each file whose status is neither `safe`, `unscanned` nor empty, but only if at least one file is `unsafe` or `caution`. Each entry has:

- `bom-ref`: `hfsec-{model bom-ref}-{path}`
- `source`: "HuggingFace Security Scanner", with a URL to the file at `blob/{revision or main}`
- `affects`: the model bom-ref
- `description`: the file, its overall status, the scanner messages and any pickle imports
- `ratings`: one per scanner that reported a status other than `safe` or `unscanned`. The scanners are ClamAV, ProtectAI, the HF pickle scanner, VirusTotal and JFrog. The severity is `unsafe` → critical, `suspicious` → high, `caution` → medium, anything else → unknown.
- `advisories`: the scanner report links
