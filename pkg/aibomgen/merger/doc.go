// Package merger combines CycloneDX AIBOMs with SBOMs produced by other tools.
// (e.g. Syft, Trivy) into a single comprehensive BOM.
//.
// The SBOM supplies the primary application metadata component; AI/ML model.
// and dataset components from the AIBOM(s) are appended to the component list.
// Dependency graphs, compositions, tools, and external references are merged.
// additively. Optional deduplication removes components with identical BOM-refs.
//.
// [Merge] is the primary entry point. It returns a [MergeResult] that includes.
// the merged BOM and per-category component counts.
package merger
