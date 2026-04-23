// Package completeness computes metadata completeness scores for CycloneDX.
// AIBOMs.
//.
// Each field in the metadata registry carries a weight and a required flag.
// [Check] scores the model component of a BOM and all linked dataset.
// components, returning a [Result] that includes the weighted score (0–1),.
// counts of present/total fields, and lists of missing required and optional.
// fields. [CheckDataset] scores a single dataset component in isolation.
package completeness
