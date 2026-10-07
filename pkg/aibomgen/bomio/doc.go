// Package bomio provides read and write helpers for CycloneDX BOMs.
//
// [ReadBOM] detects JSON or XML from the file content. [WriteBOM] picks the
// encoding from the output extension (".xml" → XML, anything else → JSON) and
// accepts an optional CycloneDX spec version (e.g. "1.5") to downgrade the
// output. [WriteOutputFiles] writes one "<model-ref>.aibom.cdx.json" (or ".xml")
// file per [generator.DiscoveredBOM].
package bomio
