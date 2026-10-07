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

	// WORKAROUND: cyclonedx-go doesn't remove 1.6-only fields everywhere when encoding
	// to earlier versions: tags are kept on all components (cyclonedx-go#248), and
	// manufacturer/authors are kept on metadata.tools.components at 1.5 (#57).
	// TODO: Remove this workaround once cyclonedx-go handles these.
	if sv < cdx.SpecVersion1_6 {
		stripPre16Fields(bom)
	}

	return encoder.EncodeVersion(bom, sv)
}

// stripPre16Fields removes fields introduced in spec 1.6 that cyclonedx-go leaves in place.
func stripPre16Fields(bom *cdx.BOM) {
	if bom == nil {
		return
	}
	if bom.Metadata != nil {
		stripTagsFromComponent(bom.Metadata.Component)
		if bom.Metadata.Tools != nil && bom.Metadata.Tools.Components != nil {
			for i := range *bom.Metadata.Tools.Components {
				tool := &(*bom.Metadata.Tools.Components)[i]
				tool.Tags, tool.Manufacturer, tool.Authors = nil, nil, nil
			}
		}
	}
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
	case "1.7":
		return cdx.SpecVersion1_7, true
	default:
		return cdx.SpecVersion1_6, false
	}
}

// WriteOutputFiles writes BOM files to disk and returns the list of written paths.
// Each BOM is written to a separate file named after the requested model reference
// (Discovery.ID, plus "@revision" when set), so two requested IDs that resolve to the
// same Hugging Face model still get their own file. Names that collide after
// sanitizing get a numeric suffix (_2, _3, ...) instead of overwriting each other.
func WriteOutputFiles(discoveredBOMs []generator.DiscoveredBOM, outputDir, fileExt, format, specVersion string) ([]string, error) {
	written := make([]string, 0, len(discoveredBOMs))
	used := make(map[string]struct{}, len(discoveredBOMs))
	for _, d := range discoveredBOMs {
		base := sanitizeFileName(outputBaseName(d))
		fileName := fmt.Sprintf("%s_aibom%s", base, fileExt)
		for n := 2; ; n++ {
			if _, taken := used[fileName]; !taken {
				break
			}
			fileName = fmt.Sprintf("%s_%d_aibom%s", base, n, fileExt)
		}
		used[fileName] = struct{}{}
		dest := filepath.Join(outputDir, fileName)

		if err := WriteBOM(d.BOM, dest, format, specVersion); err != nil {
			return written, err
		}
		written = append(written, dest)
	}
	return written, nil
}

// outputBaseName returns the unsanitized base name for a BOM file: the requested
// ID (with "@revision"), else the discovery name, else the component name.
func outputBaseName(d generator.DiscoveredBOM) string {
	name := strings.TrimSpace(d.Discovery.ID)
	if name == "" {
		name = strings.TrimSpace(d.Discovery.Name)
	}
	if name == "" && d.BOM != nil && d.BOM.Metadata != nil && d.BOM.Metadata.Component != nil {
		name = strings.TrimSpace(d.BOM.Metadata.Component.Name)
	}
	if name == "" {
		return "model"
	}
	if rev := strings.TrimSpace(d.Discovery.Revision); rev != "" {
		name += "@" + rev
	}
	return name
}

// sanitizeFileName keeps [A-Za-z0-9._-] and replaces every other rune with '_'.
func sanitizeFileName(name string) string {
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
	if b.Len() == 0 {
		return "model"
	}
	return b.String()
}
