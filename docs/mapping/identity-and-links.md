# Identity and links

These values aren't FieldSpecs. The builder and generator compute them after the FieldSpecs have run (`internal/builder/`, `pkg/aibomgen/generator/`).

## Name

Both models and datasets use the **resolved** Hugging Face ID as `name`. The Hub redirects short and renamed IDs: `gpt2` → `openai-community/gpt2`, `bert-base-uncased` → `google-bert/bert-base-uncased`, `wikipedia` → `legacy-datasets/wikipedia`. The ID as it was requested or found in code is kept in the `aibomgen.evidence` property.

## purl

```
pkg:huggingface/{namespace}/{name}@{commit sha}          model
pkg:huggingface/datasets/{namespace}/{name}@{commit sha} dataset
```

The purl is built from `name` and the first hash (the commit SHA, lowercased), following the [Hugging Face purl type](https://github.com/package-url/purl-spec/blob/main/types-doc/huggingface-definition.md). `@` and spaces in name segments are percent-encoded. Without a commit SHA (no API data), the purl has no version.

## bom-ref

`bom-ref` equals the purl. If there is no purl, it is a random `urn:uuid:…`.

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

## Dependencies

`dependencies[]` has one entry for the model that depends on every data component, plus one entry (without `dependsOn`) for each data component.

## Output file names

Each BOM is written to `{requested ID}[_{revision}]_aibom.{json|xml}`. Characters outside `A-Z a-z 0-9 . _ -` become `_`. The name uses the requested ID, not the resolved `name`. So two requested IDs that resolve to the same model, e.g. `gpt2` and `openai-community/gpt2`, each get their own file (`gpt2_aibom.json` and `openai-community_gpt2_aibom.json`). Those two BOMs differ only in their evidence properties. If two names become identical after sanitizing (`org/x` and `org_x`), the later file gets `_2`, `_3`, … instead of overwriting the earlier one.
