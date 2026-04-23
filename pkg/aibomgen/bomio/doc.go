// Package bomio provides read and write helpers for CycloneDX BOMs.
//.
// Both JSON and XML serialisation are supported. When the format parameter is.
// "auto", the format is inferred from the file extension (.json → JSON,.
// .xml → XML). [WriteBOM] accepts an optional CycloneDX spec version string.
// (e.g. "1.5") to downgrade the output; omitting it encodes with the version.
// already set on the BOM. [WriteOutputFiles] writes one file per.
// [generator.DiscoveredBOM], deriving filenames from the model component name.
package bomio
