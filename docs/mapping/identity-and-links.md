# Identity and links

These values aren't FieldSpecs. The builder and generator compute them after the FieldSpecs have run (`internal/builder/`, `pkg/aibomgen/generator/`).

## Name

Both models and datasets use the **resolved** Hugging Face ID as `name`. The Hub redirects short and renamed IDs: `gpt2` → `openai-community/gpt2`, `bert-base-uncased` → `google-bert/bert-base-uncased`, `wikipedia` → `legacy-datasets/wikipedia`. The ID as it was found in code is kept in `evidence.occurrences[].symbol`. For a requested model ID, it is in the Hub API URL in `evidence.identity` (see [Evidence](#evidence)).

## purl

```
pkg:huggingface/{namespace}/{name}@{commit sha}          model
pkg:huggingface/datasets/{namespace}/{name}@{commit sha} dataset
```

The purl is built from `name` and the first hash (the commit SHA, lowercased), following the [Hugging Face purl type](https://github.com/package-url/purl-spec/blob/main/types-doc/huggingface-definition.md). `@` and spaces in name segments are percent-encoded. Without a commit SHA (no API data), the purl has no version. Base models in `pedigree.ancestors` always get a purl without version, because their commit isn't resolved (see [model.md](model.md#lineage)).

## bom-ref

`bom-ref` equals the purl. If there is no purl, it is a random `urn:uuid:…`. This also applies to base models in `pedigree.ancestors`.

## Version and revisions

A model can be requested at a revision (a branch, tag or commit):

- `generate --model-id org/name@revision`. `@` can't occur in a repository ID, so the first `@` starts the revision. Several `--model-id` values can each have their own revision. Revisions whose names contain a comma are not supported, because `--model-id` splits on commas.
- `scan` detects a `revision="…"` keyword argument inside a Python call that references a model, such as `AutoModel.from_pretrained("org/name", revision="v1.0")` or `pipeline(..., model="org/name", revision="main")`. JavaScript, YAML and shell references carry no revision.

The model API, README and tree are then fetched at that revision (see [README.md](README.md#sources)). The result:

| Requested | `version` | purl version |
|---|---|---|
| nothing | resolved commit SHA | resolved commit SHA |
| a named revision (`v1.0`, `main`, `refs/pr/3`) | the revision name | commit SHA the revision resolves to |
| a 40-hex commit SHA | that commit SHA | that commit SHA |

`hashes` always hold the resolved commit SHA. Datasets are not versioned by revision; they are always fetched at their default branch.

## Dataset references

`modelCard.modelParameters.datasets[]` must only reference components that exist in the BOM. It is built in three steps:

1. The model-card FieldSpec lists the card's datasets (`cardData.datasets` and `dataset:*` tags, else README front matter `datasets`) as `dataset:{name}` placeholders.
2. The generator fetches every dataset in `cardData.datasets` and README front matter `datasets`, and builds a data component for each one that resolves. It records which card name produced which component bom-ref. Card names that resolve to the same repository share one component.
3. `builder.LinkDatasetRefs` rewrites the list:
   - A card entry with a component becomes `{"ref": "<component bom-ref>"}`. Matching uses the card name, ignoring case and a `dataset:` prefix, so a renamed dataset still links to its redirected component.
   - A card entry without a component (not on the Hub, or the request failed) becomes an inline entry `{"type": "dataset", "name": "<card name>"}`, instead of a reference to a bom-ref that doesn't exist. If the card value is an `http(s)://` URL, the entry also gets `contents.url`.
   - Components that the card list didn't name are appended as references.
   - Duplicate references are removed.

`aibomgen-cli validate` warns about any `datasets[].ref`, `dependencies[].ref` or `dependsOn` value that has no matching `bom-ref` in the BOM (`validator.DanglingRefs`). This also covers BOMs that were merged or edited by hand.

## Evidence

`metadata.component.evidence` records how the model was identified. It is always written.

### Source scan (`scan`)

**`occurrences[]`** has one entry per place the reference was found, sorted by file and line:
- `location`: the file path relative to the scanned directory, with `/` separators.
- `line`: the 1-based line. A call spread over several lines can be reported twice: at its first line (matched as a whole call) and at the line that holds the ID. It is left out for notebooks, whose lines count within a cell.
- `symbol`: the model ID exactly as written in the file, e.g. `gpt2`, while `name` holds the resolved `openai-community/gpt2`.
- `additionalContext`: the detection rule, e.g. `from_pretrained`. For notebooks it also gives the cell, e.g. `from_pretrained (cell 3, line 2)`.

When several rules match the same line, only the most confident one is kept.

**`identity`** holds one entry:
- `field`: `purl`.
- `concludedValue`: the component purl.
- `confidence`: the highest method confidence.
- `methods`: one per detection rule that matched, with `technique: source-code-analysis`, the rule name as `value`, and the rule's confidence. Sorted by confidence.

| Rule class | Rules | Confidence |
|---|---|---|
| Explicit Hugging Face API call | `from_pretrained*`, `pipeline_*`, `hf_hub_download*`, `snapshot_download*`, `InferenceClient*`, `InferenceApi`, `SentenceTransformer`, `CrossEncoder`, LangChain `HuggingFace*`, `js_from_pretrained`, `js_pipeline_positional` | 0.9 |
| Generic code keyword | `model_kwarg_slash`, `repo_id_kwarg_slash`, `model_id_kwarg_slash`, `evaluate_load`, `js_model_field` | 0.7 |
| Config-file value | `yaml_model_field`, `json_*`, `markdown_frontmatter_model` | 0.6 |
| Shell download | `hf_cli_download` | 0.5 |
| Shell variable | `shell_model_env` | 0.4 |
| Markdown prose | `markdown_inline` | 0.3 |

### Model ID (`generate --model-id`)

There are no occurrences. `identity` has `field: purl`, `confidence: 1` and `concludedValue`: the purl. It has one method with `technique: other`, whose `value` is the Hub API request that resolved the model, e.g. `https://huggingface.co/api/models/gpt2` or `…/api/models/org/name/revision/v1.0`.

### Older spec versions

The CycloneDX library downgrades `evidence` when writing an older `--spec`:
- **1.5:** keeps one identity without `concludedValue`, and occurrences with only `location`.
- **Below 1.5:** drops identity and occurrences.

## Dependencies

`dependencies[]` has one entry for the model that depends on every data component, plus one entry (without `dependsOn`) for each data component.

## Output file names

Each BOM is written to `{requested ID}[_{revision}]_aibom.{json|xml}`. Characters outside `A-Z a-z 0-9 . _ -` become `_`. The name uses the requested ID, not the resolved `name`. So two requested IDs that resolve to the same model, e.g. `gpt2` and `openai-community/gpt2`, each get their own file (`gpt2_aibom.json` and `openai-community_gpt2_aibom.json`). Those two BOMs differ only in their `evidence`. If two names become identical after sanitizing (`org/x` and `org_x`), the later file gets `_2`, `_3`, … instead of overwriting the earlier one.
