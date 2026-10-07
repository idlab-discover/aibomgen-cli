package validator

import (
	"bytes"
	"strings"
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

func encodeBOM(t *testing.T, bom *cdx.BOM, format cdx.BOMFileFormat, sv cdx.SpecVersion) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := cdx.NewBOMEncoder(&buf, format).EncodeVersion(bom, sv); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func schemaTestBOM() *cdx.BOM {
	bom := cdx.NewBOM()
	bom.Metadata = &cdx.Metadata{
		Timestamp: "2026-01-02T03:04:05Z",
		Component: &cdx.Component{BOMRef: "pkg:generic/m@1", Type: cdx.ComponentTypeLibrary, Name: "m", Version: "1"},
	}
	return bom
}

func TestValidateJSONSchema_ValidPerSpecVersion(t *testing.T) {
	for _, sv := range []cdx.SpecVersion{cdx.SpecVersion1_2, cdx.SpecVersion1_3, cdx.SpecVersion1_4, cdx.SpecVersion1_5, cdx.SpecVersion1_6, cdx.SpecVersion1_7} {
		data := encodeBOM(t, schemaTestBOM(), cdx.BOMFileFormatJSON, sv)
		violations, skipped := ValidateJSONSchema(data)
		if len(violations) > 0 || skipped != "" {
			t.Errorf("spec %s: violations=%v skipped=%q", sv, violations, skipped)
		}
	}
}

func TestValidateJSONSchema_Violations(t *testing.T) {
	data := []byte(`{"bomFormat":"CycloneDX","specVersion":"1.6","version":"x",
		"metadata":{"timestamp":"yesterday"},
		"components":[{"type":"banana","name":"c"}]}`)
	violations, _ := ValidateJSONSchema(data)
	for _, want := range []string{"/version", "/metadata/timestamp", "/components/0/type"} {
		found := false
		for _, v := range violations {
			found = found || strings.HasPrefix(v, "schema: "+want+":")
		}
		if !found {
			t.Errorf("no violation for %s in %v", want, violations)
		}
	}
}

func TestValidateJSONSchema_SpecVersions(t *testing.T) {
	if _, skipped := ValidateJSONSchema([]byte(`{"bomFormat":"CycloneDX","specVersion":"1.1"}`)); skipped == "" {
		t.Error("spec 1.1 has no JSON schema and must be skipped")
	}
	if v, _ := ValidateJSONSchema([]byte(`{"bomFormat":"CycloneDX"}`)); len(v) != 1 {
		t.Errorf("missing specVersion must be one violation, got %v", v)
	}
	if v, _ := ValidateJSONSchema([]byte(`{not json`)); len(v) != 1 {
		t.Errorf("invalid JSON must be one violation, got %v", v)
	}
}

func TestValidateData(t *testing.T) {
	t.Run("valid JSON", func(t *testing.T) {
		got := ValidateData(encodeBOM(t, schemaTestBOM(), cdx.BOMFileFormatJSON, cdx.SpecVersion1_6), ValidationOptions{})
		if !got.Valid || len(got.Errors) > 0 {
			t.Fatalf("want valid, got errors %v", got.Errors)
		}
	})
	t.Run("XML skips schema with a warning", func(t *testing.T) {
		got := ValidateData(encodeBOM(t, schemaTestBOM(), cdx.BOMFileFormatXML, cdx.SpecVersion1_6), ValidationOptions{})
		if !got.Valid || len(got.Warnings) == 0 || !strings.Contains(got.Warnings[0], "schema validation") {
			t.Fatalf("want valid with a skip warning, got valid=%v warnings=%v", got.Valid, got.Warnings)
		}
	})
	t.Run("schema violation fails", func(t *testing.T) {
		got := ValidateData([]byte(`{"bomFormat":"CycloneDX","specVersion":"1.6","version":1,"metadata":{"timestamp":"yesterday","component":{"type":"library","name":"m"}}}`), ValidationOptions{})
		if got.Valid || len(got.Errors) == 0 || !strings.HasPrefix(got.Errors[0], "schema: /metadata/timestamp") {
			t.Fatalf("want schema error first, got valid=%v errors=%v", got.Valid, got.Errors)
		}
	})
	t.Run("undecodable BOM is invalid, not a crash", func(t *testing.T) {
		got := ValidateData([]byte(`{"bomFormat":"CycloneDX","specVersion":"9.9"}`), ValidationOptions{})
		if got.Valid {
			t.Fatal("want invalid")
		}
	})
}
