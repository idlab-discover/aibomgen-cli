// Package scanner walks a directory tree and detects Hugging Face model and.
// dataset usage across multiple file types.
//.
// Supported file types and detection methods:.
//   - Python (.py, .ipynb): from_pretrained, hf_hub_download, snapshot_download,.
//     pipeline, InferenceClient, SentenceTransformer, ORTModel, PeftModel,.
//     LangChain loaders, evaluate.load, and more — both positional and keyword forms.
//   - YAML (.yaml, .yml): model_name_or_path, base_model, _name_or_path,.
//     pretrained_model_name_or_path.
//   - JSON (.json): adapter configs, _name_or_path, base_model.
//   - Markdown: base_model field in YAML front-matter.
//   - Shell scripts and Dockerfiles: huggingface-cli download, hf download.
//   - JavaScript / TypeScript (.js, .ts, .mjs, .cjs): pipeline and from_pretrained.
//     calls via the @huggingface/transformers library.
//.
// The primary entry point is [Scan], which returns a slice of [Discovery] values.
// describing each detected model or dataset reference.
package scanner
