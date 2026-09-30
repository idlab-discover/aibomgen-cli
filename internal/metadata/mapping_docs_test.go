package metadata

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// mappingDocsDir holds the field mapping documentation that must list every FieldSpec key.
const mappingDocsDir = "../../docs/mapping"

// docKeyRe matches FieldSpec keys written as inline code in the mapping docs.
var docKeyRe = regexp.MustCompile("`(BOM\\.(?:metadata\\.component|components\\[DATA\\])\\.[^`]+)`")

// TestMappingDocsCoverRegistry keeps docs/mapping in sync with the registries:
// every FieldSpec key must be documented, and every documented key must exist.
func TestMappingDocsCoverRegistry(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(mappingDocsDir, "*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no mapping docs found in %s (err=%v)", mappingDocsDir, err)
	}
	var docs strings.Builder
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		docs.Write(b)
		docs.WriteByte('\n')
	}
	text := docs.String()

	known := map[string]struct{}{}
	for _, s := range Registry() {
		known[s.Key.String()] = struct{}{}
	}
	for _, s := range DatasetRegistry() {
		known[s.Key.String()] = struct{}{}
	}

	for key := range known {
		if !strings.Contains(text, "`"+key+"`") {
			t.Errorf("FieldSpec key %s is not documented in %s; add it to the mapping tables", key, mappingDocsDir)
		}
	}
	for _, m := range docKeyRe.FindAllStringSubmatch(text, -1) {
		if _, ok := known[m[1]]; !ok {
			t.Errorf("%s documents key %s, which is not a FieldSpec; update the mapping tables", mappingDocsDir, m[1])
		}
	}
}
