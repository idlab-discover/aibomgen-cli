# Dataset component mapping

Every dataset named on the model card (`cardData.datasets` and README front matter `datasets`) is looked up on the Hugging Face Hub. Each dataset that resolves becomes a component in `components[]` with `type: data`, and its dataset details go in `data[0]` (`type: dataset`). The Hub redirects renamed datasets (e.g. `wikipedia` → `legacy-datasets/wikipedia`). From then on the resolved ID (`HF.id`) is used for the name, the README request and the links. Card names that resolve to the same repository produce a single component.

Notation: `HF.x` is a dataset API field, `cardData.x` a key of the API's `cardData`, and `README …` the dataset README. "Namespace" is the `org` part of the resolved (`HF.id`) or card dataset ID, falling back to `HF.author`. Sources are listed in precedence order (see [README.md](README.md#general-rules)).

| CycloneDX field | FieldSpec key | Sources (in order) | Transform | Weight |
|---|---|---|---|---|
| `name` | `BOM.components[DATA].name` | `HF.id` → card name → dataset ID | Trimmed. The resolved ID wins (`legacy-datasets/wikipedia`). | 1.0, required |
| `externalReferences` | `BOM.components[DATA].externalReferences` | resolved ID (`HF.id`, else dataset ID); README "Paper" and "Demo" bullets | `website` = `https://huggingface.co/datasets/{id}`; `documentation` = Paper; `other` = Demo. A bullet only counts if it holds an `http(s)` URL; the URL is taken from a Markdown link's target. | 0.5 |
| `tags` | `BOM.components[DATA].tags` | `HF.tags` → README front matter `tags` | Trimmed, deduplicated | 0.5 |
| `licenses` | `BOM.components[DATA].licenses` | `cardData.license` → `license:*` tags → README front matter `license`; plus `license_name` and `license_link` | SPDX `id` or `name`, see [licenses.md](licenses.md) | 0.8 |
| `data[0].description` | `BOM.components[DATA].data.description` | README "Dataset Description" section → `HF.description` | Trimmed | 0.7 |
| `manufacturer` | `BOM.components[DATA].manufacturer` | `HF.author` → README front matter `annotations_creators[0]` | `{name}`, plus `url: [https://huggingface.co/{namespace}]` when the name is the namespace | 0.6 |
| `supplier` | `BOM.components[DATA].supplier` | namespace | `{name: namespace, url: [https://huggingface.co/{namespace}]}` | 0.4 |
| `authors` | `BOM.components[DATA].authors` | `HF.author` together with README front matter `annotations_creators` | One `{name}` per value | 0.6 |
| `group` | `BOM.components[DATA].group` | namespace | Only IDs with an `org/` part, or an `HF.author`, yield a group | 0.4 |
| `data[0].contents.attachment` | `BOM.components[DATA].data.contents.attachments` | README front matter `configs[].data_files[]` | Plain-text attachment with one line per file: `config:{config_name} split:{split} path:{path}` | 0.5 |
| `data[0].sensitiveData` | `BOM.components[DATA].data.sensitiveData` | `cardData.tags`; README "Out-of-Scope Use", "Personal and Sensitive Information", "Bias, Risks, and Limitations" sections | All values combined. The sections are prefixed with `out-of-scope: `, `personal-info: ` and `bias-risks: `. | 0.6 |
| `data[0].classification` | `BOM.components[DATA].data.classification` | `cardData.task_categories[0]` | As is | 0.6 |
| `data[0].governance` | `BOM.components[DATA].data.governance` | custodians: `HF.author` → README "Shared by" → "Curated by"; stewards: README "Curated by"; owners: README "Funded by" | One organization per role; placeholder names are skipped | 0.7 |
| `hashes` | `BOM.components[DATA].hashes` | `HF.sha` | `[{alg: SHA-1, content: sha}]` (the git commit SHA) | 0.5 |
| property `huggingface:createdAt` | `BOM.components[DATA].properties.huggingface:createdAt` | `HF.createdAt` | Non-empty | 0.3 |
| property `huggingface:usedStorage` | `BOM.components[DATA].properties.huggingface:usedStorage` | `HF.usedStorage` | Greater than 0, in bytes | 0.3 |
| tag `lastModified:{value}` | `BOM.components[DATA].tags.lastModified` | `HF.lastModified` | Appended to `tags` | 0.3 |
| property `huggingface:datasetContact` | `BOM.components[DATA].properties.huggingface:datasetContact` | README "Dataset Card Contact" section | Non-empty | 0.5 |

`purl` and `bom-ref` are computed after these fields (`pkg:huggingface/datasets/{id}@{sha}`), see [identity-and-links.md](identity-and-links.md).

Datasets are always fetched at their default branch. A model revision doesn't apply to its datasets.
