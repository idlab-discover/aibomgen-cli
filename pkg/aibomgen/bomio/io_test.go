package bomio

import (
	"encoding/json"
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
		{"1.7", cdx.SpecVersion1_7, true},
		{"1.8", cdx.SpecVersion1_6, false},
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
	if _, err := ReadBOM(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadBOM_InvalidJSON(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(p, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadBOM(p); err == nil {
		t.Fatal("expected decode error")
	}
}

// The encoding is sniffed from the content, so the file name never matters.
func TestReadBOM_DetectsFormatFromContent(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct{ written, readAs string }{
		{"bom.json", "bom.json"},
		{"bom.xml", "bom.xml"},
		{"bom.xml", "xml-content.json"}, // XML in a .json file
		{"bom.json", "json-content"},    // no extension
		{"bom.xml", "xml-content"},      // no extension
	} {
		src := filepath.Join(dir, tc.written)
		if err := WriteBOM(minimalBOM(), src, ""); err != nil {
			t.Fatalf("WriteBOM: %v", err)
		}
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(dir, tc.readAs)
		if err := os.WriteFile(dst, append([]byte("\n  "), data...), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := ReadBOM(dst)
		if err != nil {
			t.Fatalf("ReadBOM(%s written as %s): %v", tc.readAs, tc.written, err)
		}
		if got.Metadata == nil || got.Metadata.Component == nil || got.Metadata.Component.Name != "test-model" {
			t.Fatalf("ReadBOM(%s): unexpected BOM: %#v", tc.readAs, got.Metadata)
		}
	}
}

// The output extension picks the encoding: ".xml" (any case) is XML, everything else JSON.
func TestWriteBOM_EncodingFromExtension(t *testing.T) {
	dir := t.TempDir()
	for name, wantFirst := range map[string]byte{
		"a.json":     '{',
		"a.cdx.json": '{',
		"a.xml":      '<',
		"a.XML":      '<',
		"a.cdx.xml":  '<',
		"noext":      '{',
	} {
		out := filepath.Join(dir, name)
		if err := WriteBOM(minimalBOM(), out, "1.6"); err != nil {
			t.Fatalf("WriteBOM(%s): %v", name, err)
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if data[0] != wantFirst {
			t.Errorf("%s: starts with %q, want %q", name, data[0], wantFirst)
		}
		if _, err := ReadBOM(out); err != nil {
			t.Errorf("%s: round trip: %v", name, err)
		}
	}
}

func TestWriteBOM_OutputIsDirectory(t *testing.T) {
	if err := WriteBOM(minimalBOM(), t.TempDir(), ""); err == nil {
		t.Fatal("expected error when output path is a directory")
	}
}

func TestWriteBOM_InvalidSpec_WritesNothing(t *testing.T) {
	out := filepath.Join(t.TempDir(), "bom.json")
	if err := WriteBOM(minimalBOM(), out, "9.9"); err == nil {
		t.Fatal("expected error for invalid spec")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("invalid spec must not create %s", out)
	}
}

func TestWriteBOM_Spec15_StripsToolComponentFields(t *testing.T) {
	for spec, wantPresent := range map[string]bool{"1.5": false, "1.6": true} {
		bom := minimalBOM()
		bom.Metadata.Tools = &cdx.ToolsChoice{Components: &[]cdx.Component{{
			Type:         cdx.ComponentTypeApplication,
			Name:         "aibomgen-cli",
			Manufacturer: &cdx.OrganizationalEntity{Name: "IDLab"},
			Authors:      &[]cdx.OrganizationalContact{{Name: "someone"}},
		}}}
		out := filepath.Join(t.TempDir(), "bom.json")
		if err := WriteBOM(bom, out, spec); err != nil {
			t.Fatalf("WriteBOM %s: %v", spec, err)
		}
		raw, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Metadata struct {
				Tools struct {
					Components []map[string]any `json:"components"`
				} `json:"tools"`
			} `json:"metadata"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		tool := doc.Metadata.Tools.Components[0]
		for _, key := range []string{"manufacturer", "authors"} {
			if _, ok := tool[key]; ok != wantPresent {
				t.Errorf("spec %s: %q present = %v, want %v", spec, key, ok, wantPresent)
			}
		}
	}
}
