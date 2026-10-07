# Field mapping

This folder documents where every value in an aibomgen-cli AIBOM comes from. It describes the implemented mapping: the field specifications ("FieldSpecs") in [`internal/metadata/`](../../internal/metadata/) and the builder steps in [`internal/builder/`](../../internal/builder/). Nothing here is aspirational. If a field isn't listed, aibomgen-cli doesn't produce it.

| Document | Covers |
|---|---|
| [model.md](model.md) | The model component (`metadata.component`): component fields, model card, properties and security findings |
| [dataset.md](dataset.md) | Data components (`components[]` with `type: data`) |
| [licenses.md](licenses.md) | How license values become `license.id` or `license.name`, for models and datasets |
| [identity-and-links.md](identity-and-links.md) | Name, purl, bom-ref, version, revisions, dataset references, dependencies and output file names |

Every BOM declares `metadata.lifecycles: [{phase: post-build}]`: it describes an already-published model, with metadata gathered after the model was built. This holds for both `generate` and `scan`.

A test (`internal/metadata/mapping_docs_test.go`) fails when a FieldSpec key is missing from these documents or when the documents list a key that no longer exists. If you change a FieldSpec, update the tables here.

## Sources

| Source | Request | Used for |
|---|---|---|
| Model API | `GET /api/models/{id}`, or `GET /api/models/{id}/revision/{revision}` when a revision is requested. If `cardData` declares a `base_model`, one more request with `?expand[]=baseModels` fetches the Hub's lineage (see [model.md](model.md#lineage)). | The model component. `cardData` is the model card's YAML front matter as parsed by the Hub. `siblings` (the file list) locates a root `LICENSE` file. |
| Model README | `GET /{id}/resolve/main/README.md` (falls back to `master`), or `/{id}/resolve/{revision}/README.md` | YAML front matter plus template sections (with heading aliases, see [model.md](model.md#considerations-sections)) and bullets in the body (e.g. "Developed by", "Direct Use") |
| Model tree | `GET /api/models/{id}/tree/{revision or main}?expand=true&recursive=true` | Security scan properties and vulnerabilities |
| Dataset API | `GET /api/datasets/{id}` (HF redirects renamed datasets; the resolved ID is used from here on) | Data components |
| Dataset README | `GET /datasets/{resolved id}/resolve/main/README.md` (falls back to `master`) | YAML front matter plus template sections and bullets in the body |
| Discovery | The `scan` result, or the `--model-id` value for `generate` | Requested ID, revision and evidence properties |

In the tables, `HF.x` is a field of the API response, `cardData.x` a key in the API's `cardData`, `README front matter x` a YAML key of the README, and `README "Section"` a Markdown section or bullet in the README body.

## General rules

1. **First usable source wins.** Each FieldSpec lists its sources in order. The first source that yields a usable value is applied, and the rest are ignored.
2. **README text is copied verbatim; derived values are not invented.** Free text from README sections and bullets (use cases, limitations, descriptions, contacts, environmental values, …) is written exactly as the card has it, including template text such as `[More Information Needed]`. The BOM reflects what the model card says. Placeholders are only filtered where a value must identify something:
   - **Licenses** (`license`, `license_name`, `license_link`): `unknown`, `[unknown]` or `[More Information Needed]` mean that no license is known, see [licenses.md](licenses.md).
   - **Names of people and organizations**: README "Developed by" (manufacturer, authors, group) and dataset governance ("Curated by", "Funded by", "Shared by").
   - **URLs**: README "Paper" and "Demo" bullets are only used when they contain an `http(s)` URL: bare, as `<…>`, as `[…]` or as the target of a Markdown link.

   The placeholders are `unknown`, `[More Information Needed]`, `needs more information`, `n/a`, `na`, `none`, `null`, `tbd`, `todo` and `-`, matched after trimming, stripping surrounding `[ ]` and ignoring case. A Markdown link whose text is a placeholder also counts, e.g. the dataset-card template's `[More Information Needed](https://github.com/huggingface/datasets/blob/master/CONTRIBUTING.md…)`.
3. **Absent beats fake.** If no source has a value, the field is omitted.
4. **`enrich` values override.** Values entered through `enrich` (interactively or from a config file) go through the same Apply logic and replace existing values. License input is normalized to SPDX like any other license value.
5. **Weights.** Every FieldSpec has a completeness weight, and some are required. `completeness` and `validate` report the weighted share of present fields. The weights are listed in the tables.

## Availability by mode

| Situation | Effect |
|---|---|
| `--hf-mode online`, API reachable | All sources are available. |
| Gated model without a token (e.g. `meta-llama/*`) | The API still returns metadata, `cardData`, tags and the commit SHA, but the README request is refused. README-only fields (e.g. use cases, "Developed by", environmental data) are missing, and authors fall back to the namespace. |
| Model API error other than 404/401 (network, 5xx, timeout) | The BOM is built from the README alone, if it could be fetched. No SHA means no purl version, `hashes` or SHA `version`, although a requested named revision is still used as `version`. Namespace fields only appear if the requested ID has an `org/` part. |
| Model API 404 or 401 | The model is skipped and no BOM is written. For `scan`, this includes local paths such as `./my_model`. |
| Dataset API failure | No data component is built. The card's dataset stays in the model card as an inline entry ([identity-and-links.md](identity-and-links.md#dataset-references)). |
| `--hf-mode dummy` | Built-in fixtures (`dummy-org/dummy-model`, no HTTP). All mappings apply to the fixture data. |

There is no offline mode.

## CycloneDX spec versions

The default output version is 1.7 (`--spec 1.7`). Selecting `--spec 1.0` to `1.6` down-converts the output with cyclonedx-go, which drops fields that the target version doesn't define:

- below 1.6: component `manufacturer`, `authors` and `tags`
- below 1.5: `modelCard`, `data` (the ML-BOM fields) and `metadata.lifecycles`
- below 1.3: `properties`
- below 1.2: `supplier`

## Sources not mapped

aibomgen-cli doesn't read repository files other than `README.md`. That includes weights, `tokenizer.json`, `LICENSE` contents and `config.json`; only the API's `config.model_type` and `config.architectures` are used. It also ignores API fields and front matter keys that no table lists.
