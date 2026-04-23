// Package validator validates CycloneDX AIBOMs produced by AIBoMGen.
//.
// [Validate] performs structural checks (nil BOM, missing metadata component,.
// spec version), delegates completeness scoring to the [completeness] package,.
// and enforces optional thresholds via [ValidationOptions]. Results are.
// returned as a [ValidationResult] that includes per-dataset breakdowns.
package validator
