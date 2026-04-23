package bomio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/generator"
)

// ReadBOM reads a BOM from a file (JSON or XML).
// The format parameter can be "json", "xml", or "auto" (default).
// If "auto", the format is determined from the file extension.
func ReadBOM(path string, format string) (*cdx.BOM, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	actual := strings.ToLower(strings.TrimSpace(format))
	switch actual {
	case "", "auto":
		switch strings.ToLower(filepath.Ext(path)) {
		case ".xml":
			actual = "xml"
		case ".json":
			actual = "json"
		default:
			// keep existing behavior: default to JSON when not .xml.
			actual = "json"
		}
	case "json", "xml":
		// ok.
	default:
		return nil, fmt.Errorf("unsupported BOM format: %q", format)
	}

	fileFmt := cdx.BOMFileFormatJSON
	if actual == "xml" {
		fileFmt = cdx.BOMFileFormatXML
	}

	bom := new(cdx.BOM)
	dec := cdx.NewBOMDecoder(f, fileFmt)
	if err := dec.Decode(bom); err != nil {
		return nil, err
	}

	return bom, nil
}

// WriteBOM writes a BOM to a file in the specified format.
// The format parameter can be "json", "xml", or "auto" (default).
// If "auto", the format is determined from the file extension.
// If spec is provided, it encodes with that specific CycloneDX version.
func WriteBOM(bom *cdx.BOM, outputPath string, format string, spec string) error {
	ext := filepath.Ext(outputPath)

	actual := strings.ToLower(strings.TrimSpace(format))
	switch actual {
	case "", "auto":
		if strings.EqualFold(ext, ".xml") {
			actual = "xml"
		} else {
			actual = "json"
		}
	case "json", "xml":
		// ok.
	default:
		return fmt.Errorf("unsupported BOM format: %q", format)
	}

	// Validate extension matches format.
	switch actual {
	case "xml":
		if ext != ".xml" {
			return fmt.Errorf("output path extension %q does not match format %q", ext, actual)
		}
	case "json":
		if ext != ".json" {
			return fmt.Errorf("output path extension %q does not match format %q", ext, actual)
		}
	}

	fileFmt := cdx.BOMFileFormatJSON
	if actual == "xml" {
		fileFmt = cdx.BOMFileFormatXML
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := cdx.NewBOMEncoder(f, fileFmt)
	encoder.SetPretty(true)

	if spec == "" {
		return encoder.Encode(bom)
	}

	sv, ok := ParseSpecVersion(spec)
	if !ok {
		return fmt.Errorf("unsupported CycloneDX spec version: %q", spec)
	}

	// WORKAROUND: Manually strip tags for spec < 1.6.
	// Tags were introduced in spec 1.6, but cyclonedx-go doesn't remove them.
	// when encoding to earlier versions (unlike manufacturer, authors, etc.).
	// See: https://github.com/CycloneDX/cyclonedx-go/issues/248.
	// TODO: Remove this workaround once issue #248 is fixed.
	if sv < cdx.SpecVersion1_6 {
		stripTagsFromBOM(bom)
	}

	return encoder.EncodeVersion(bom, sv)
}

// stripTagsFromBOM removes tags from all components in the BOM.
// WORKAROUND for cyclonedx-go issue #248: Tags are not automatically removed.
// when encoding to spec versions < 1.6, even though tags were introduced in 1.6.
// This function manually strips tags to ensure spec compliance.
// TODO: Remove this workaround once https://github.com/CycloneDX/cyclonedx-go/issues/248 is fixed.
func stripTagsFromBOM(bom *cdx.BOM) {
	if bom == nil {
		return
	}

	// Strip tags from metadata component.
	if bom.Metadata != nil && bom.Metadata.Component != nil {
		stripTagsFromComponent(bom.Metadata.Component)
	}

	// Strip tags from all components.
	if bom.Components != nil {
		for i := range *bom.Components {
			stripTagsFromComponent(&(*bom.Components)[i])
		}
	}
}

// stripTagsFromComponent recursively removes tags from a component and its children.
func stripTagsFromComponent(comp *cdx.Component) {
	if comp == nil {
		return
	}

	comp.Tags = nil

	// Recursively process child components.
	if comp.Components != nil {
		for i := range *comp.Components {
			stripTagsFromComponent(&(*comp.Components)[i])
		}
	}
}

// ParseSpecVersion parses a spec version string to a CycloneDX SpecVersion.
func ParseSpecVersion(s string) (cdx.SpecVersion, bool) {
	s = strings.TrimSpace(s)

	switch s {
	case "1.0":
		return cdx.SpecVersion1_0, true
	case "1.1":
		return cdx.SpecVersion1_1, true
	case "1.2":
		return cdx.SpecVersion1_2, true
	case "1.3":
		return cdx.SpecVersion1_3, true
	case "1.4":
		return cdx.SpecVersion1_4, true
	case "1.5":
		return cdx.SpecVersion1_5, true
	case "1.6":
		return cdx.SpecVersion1_6, true
	default:
		return cdx.SpecVersion1_6, false
	}
}

// WriteOutputFiles writes BOM files to disk and returns the list of written paths.
// Each BOM is written to a separate file named after the component.
func WriteOutputFiles(discoveredBOMs []generator.DiscoveredBOM, outputDir, fileExt, format, specVersion string) ([]string, error) {
	written := make([]string, 0, len(discoveredBOMs))
	for _, d := range discoveredBOMs {
		// Extract component name from BOM metadata.
		var name string
		if d.BOM != nil && d.BOM.Metadata != nil && d.BOM.Metadata.Component != nil {
			name = d.BOM.Metadata.Component.Name
		}
		if strings.TrimSpace(name) == "" {
			name = strings.TrimSpace(d.Discovery.Name)
			if name == "" {
				name = strings.TrimSpace(d.Discovery.ID)
			}
			if name == "" {
				name = "model"
			}
		}

		// Sanitize component name for use in filename.
		if name == "" {
			name = "model"
		}
		var b strings.Builder
		for _, r := range name {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
				b.WriteRune(r)
			case r == '-' || r == '_' || r == '.':
				b.WriteRune(r)
			default:
				b.WriteByte('_')
			}
		}
		sanitized := b.String()
		if sanitized == "" {
			sanitized = "model"
		}

		fileName := fmt.Sprintf("%s_aibom%s", sanitized, fileExt)
		dest := filepath.Join(outputDir, fileName)

		if err := WriteBOM(d.BOM, dest, format, specVersion); err != nil {
			return written, err
		}
		written = append(written, dest)
	}
	return written, nil
}
