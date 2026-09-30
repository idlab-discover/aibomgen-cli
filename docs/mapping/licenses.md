# License mapping

The same rules apply to the model component (`BOM.metadata.component.licenses`) and to data components (`BOM.components[DATA].licenses`). The code is in [`internal/metadata/license.go`](../../internal/metadata/license.go).

## 1. Gather the license values

The values come from the first group below that still has a value after placeholders are removed:

| Order | Model | Dataset |
|---|---|---|
| 1 | `cardData.license` (a string or a list) | `cardData.license` (a string or a list) |
| 2 | `HF.license` | `license:*` tags |
| 3 | `license:*` tags | README front matter `license` (a string or a list) |
| 4 | README front matter `license` (a string or a list) | |

In addition, `license_name` and `license_link` are read from `cardData`, then from the README front matter.

## 2. Drop placeholders

Values that carry no license are removed: empty strings, `unknown`, `[unknown]`, `[More Information Needed]` and the other placeholders listed in [README.md](README.md#general-rules). Hugging Face uses `unknown` as a license value; it means that no license is known. If nothing remains, the component gets **no `licenses` field**.

## 3. Map each value

For each remaining value, deduplicated case-insensitively:

1. If the value is `other` and `license_name` holds a real value, `license_name` is used instead.
2. If the value matches an SPDX license ID case-insensitively, the result is `{"license": {"id": "<canonical SPDX ID>"}}`. The CycloneDX schema only accepts the canonical casing, so `apache-2.0` becomes `Apache-2.0`.
3. Otherwise the result is `{"license": {"name": "<value>"}}`. Custom and non-SPDX licenses (e.g. `llama3.2`, `openrail`, `gfdl`, `other`) are kept as names.

A license object never has both `id` and `name`.

## 4. Link to the license text

If exactly one license object results and `license_link` is set, the link becomes `license.url`:

- An absolute `http(s)://` link is used as is.
- A relative path (e.g. `LICENSE`) is resolved against the repository at the resolved commit: `https://huggingface.co/{id}/blob/{sha}/{path}`, or `https://huggingface.co/datasets/{id}/blob/{sha}/{path}` for datasets. If the commit is unknown, the relative link is dropped.

## SPDX list

The SPDX IDs come from `internal/metadata/spdx.schema.json`, a verbatim copy of `schema/spdx.schema.json` from cyclonedx-go v0.12.0. That file is the enum that the CycloneDX 1.6 and 1.7 schemas validate `license.id` against. Refresh the copy when upgrading cyclonedx-go.

## Examples

| Repository | Hugging Face values | Result |
|---|---|---|
| `google-bert/bert-base-uncased` | `cardData.license: apache-2.0` | `[{license: {id: Apache-2.0}}]` |
| `openai-community/gpt2` | `cardData.license: mit` | `[{license: {id: MIT}}]` |
| `meta-llama/Llama-3.2-1B-Instruct` | `cardData.license: llama3.2` | `[{license: {name: llama3.2}}]` |
| `legacy-datasets/wikipedia` | `cardData.license: [cc-by-sa-3.0, gfdl]` | `[{license: {id: CC-BY-SA-3.0}}, {license: {name: gfdl}}]` |
| `bookcorpus/bookcorpus` | `cardData.license: [unknown]` | no `licenses` field |
| a card with `license: other`, `license_name: acme-1`, `license_link: LICENSE` | | `[{license: {name: acme-1, url: https://huggingface.co/{id}/blob/{sha}/LICENSE}}]` |
