package bomio

import (
	"os"
	"path/filepath"
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

func minimalBOM() *cdx.BOM {
	bom := cdx.NewBOM()
	bom.SpecVersion = cdx.SpecVersion1_6
	bom.Metadata = &cdx.Metadata{
		Component: &cdx.Component{
			Name: "test-model",
		},
	}
	return bom
}

func TestParseSpecVersion_AllCases(t *testing.T) {
	tcs := []struct {
		in   string
		want cdx.SpecVersion
		ok   bool
	}{
		{"1.0", cdx.SpecVersion1_0, true},
		{"1.1", cdx.SpecVersion1_1, true},
		{"1.2", cdx.SpecVersion1_2, true},
		{"1.3", cdx.SpecVersion1_3, true},
		{"1.4", cdx.SpecVersion1_4, true},
		{"1.5", cdx.SpecVersion1_5, true},
		{"1.6", cdx.SpecVersion1_6, true},
		{"2.0", cdx.SpecVersion1_6, false},  // default branch
		{" 1.6 ", cdx.SpecVersion1_6, true}, // now trimmed in ParseSpecVersion
		{"", cdx.SpecVersion1_6, false},
		{"1", cdx.SpecVersion1_6, false},
		{"1.7", cdx.SpecVersion1_6, false},
		{"nope", cdx.SpecVersion1_6, false},
	}

	for _, tc := range tcs {
		got, ok := ParseSpecVersion(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("ParseSpecVersion(%q) = (%v,%v), want (%v,%v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestReadBOM_OpenError(t *testing.T) {
	_, err := ReadBOM(filepath.Join(t.TempDir(), "missing.json"), "auto")
	if err == nil {
		t.Fatalf("expected error for missing file")
	}
}

func TestReadBOM_Auto_SelectsJSONByExtension(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bom.json")
	if err := os.WriteFile(p, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := ReadBOM(p, "auto")
	if err != nil {
		t.Fatalf("ReadBOM(auto): %v", err)
	}
	if got == nil {
		t.Fatalf("expected BOM")
	}
}

func TestReadBOM_DecodeError_WhenFormatDoesNotMatchContent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bom.json")
	if err := os.WriteFile(p, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := ReadBOM(p, "xml")
	if err == nil {
		t.Fatalf("expected decode error when reading JSON as XML")
	}
}

func TestReadBOM_DecodeError_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bom.json")
	if err := os.WriteFile(p, []byte(`{`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := ReadBOM(p, "json")
	if err == nil {
		t.Fatalf("expected decode error for invalid JSON")
	}
}

func TestWriteBOM_JSON_Auto_SpecEmpty_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "bom.json")

	if err := WriteBOM(minimalBOM(), out, "auto", ""); err != nil {
		t.Fatalf("WriteBOM: %v", err)
	}

	got, err := ReadBOM(out, " JSON ")
	if err != nil {
		t.Fatalf("ReadBOM: %v", err)
	}
	if got == nil || got.Metadata == nil || got.Metadata.Component == nil || got.Metadata.Component.Name != "test-model" {
		t.Fatalf("roundtrip BOM missing expected metadata.component.name")
	}
}

func TestWriteBOM_JSON_Explicit_SpecVersion_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "bom.json")

	if err := WriteBOM(minimalBOM(), out, " json ", "1.6"); err != nil {
		t.Fatalf("WriteBOM: %v", err)
	}

	got, err := ReadBOM(out, "json")
	if err != nil {
		t.Fatalf("ReadBOM: %v", err)
	}
	if got.SpecVersion == 0 {
		t.Fatalf("expected specVersion to be set after decode")
	}
}

func TestWriteBOM_XML_Explicit_RoundTrip_AndReadAuto(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "bom.xml")

	if err := WriteBOM(minimalBOM(), out, "xml", ""); err != nil {
		t.Fatalf("WriteBOM: %v", err)
	}

	got, err := ReadBOM(out, "")
	if err != nil {
		t.Fatalf("ReadBOM: %v", err)
	}
	if got == nil {
		t.Fatalf("expected BOM")
	}
}

func TestWriteBOM_XML_Auto_SelectsByExtension_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "bom.xml")

	if err := WriteBOM(minimalBOM(), out, "auto", ""); err != nil {
		t.Fatalf("WriteBOM: %v", err)
	}

	got, err := ReadBOM(out, "xml")
	if err != nil {
		t.Fatalf("ReadBOM: %v", err)
	}
	if got == nil || got.Metadata == nil || got.Metadata.Component == nil || got.Metadata.Component.Name != "test-model" {
		t.Fatalf("roundtrip BOM missing expected metadata.component.name")
	}
}

func TestReadBOM_Auto_NoExtension_DefaultsToJSONByContent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bom") // no extension

	// Looks like JSON => current impl appears to treat this as JSON in "auto"/"" mode.
	if err := os.WriteFile(p, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := ReadBOM(p, "") // "" behaves like auto in this package
	if err != nil {
		t.Fatalf("expected no error when auto-reading JSON content without extension, got: %v", err)
	}
	if got == nil {
		t.Fatalf("expected BOM")
	}
}

func TestWriteBOM_UnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bom.json")

	if err := WriteBOM(minimalBOM(), p, "json", ""); err != nil {
		t.Fatalf("WriteBOM: %v", err)
	}

	_, err := ReadBOM(p, "yaml")
	if err == nil {
		t.Fatalf("expected error for unsupported format")
	}
}

func TestWriteBOM_OpenError_WhenOutputIsDirectory(t *testing.T) {
	dir := t.TempDir()

	// Make a *directory* that still has a valid ".json" extension so we get past.
	// extension validation and hit the os.Create(...) error path.
	outDir := filepath.Join(dir, "bom.json")
	if err := os.Mkdir(outDir, 0o700); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	if err := WriteBOM(minimalBOM(), outDir, "json", ""); err == nil {
		t.Fatalf("expected error when output path is a directory")
	}
}

func TestWriteBOM_UnsupportedFormat_Errors(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "bom.json")

	if err := WriteBOM(minimalBOM(), out, "yaml", ""); err == nil {
		t.Fatalf("expected error for unsupported write format")
	}
}

func TestWriteBOM_ExtensionMismatch_XMLFormatButJSONPath(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "bom.json")

	if err := WriteBOM(minimalBOM(), out, "xml", ""); err == nil {
		t.Fatalf("expected error for extension/format mismatch")
	}
}

func TestWriteBOM_ExtensionMismatch_JSONFormatButXMLPath(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "bom.xml")

	if err := WriteBOM(minimalBOM(), out, "json", ""); err == nil {
		t.Fatalf("expected error for extension/format mismatch")
	}
}

func TestWriteBOM_SpecProvidedButInvalid_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "bom.json")

	if err := WriteBOM(minimalBOM(), out, "json", "9.9"); err == nil {
		t.Fatalf("expected error for unsupported CycloneDX spec version")
	}
}

func TestWriteBOM_Auto_UppercaseXMLExtension_HitsEqualFoldThenValidationMismatch(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "bom.XML") // ext is ".XML"

	// auto picks "xml" due to EqualFold(ext, ".xml"), then validation compares ext != ".xml" and errors.
	if err := WriteBOM(minimalBOM(), out, "auto", ""); err == nil {
		t.Fatalf("expected error for uppercase .XML extension validation mismatch")
	}
}
