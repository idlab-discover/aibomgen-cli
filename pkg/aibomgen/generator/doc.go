// Package generator orchestrates AIBOM generation from Hugging Face model.
// discoveries.
//.
// For each [scanner.Discovery], the generator fetches model metadata from the.
// Hugging Face Hub (API response, model card / README, optionally the security.
// scan tree), builds linked dataset components, and produces a CycloneDX BOM.
// via the internal builder.
//.
// The primary entry point is [BuildPerDiscovery]. Progress during generation is.
// reported through the [ProgressCallback] supplied in [GenerateOptions].
// [BuildDummyBOM] produces a fully-populated fixture BOM without any network.
// calls, intended for offline testing and demos.
package generator
