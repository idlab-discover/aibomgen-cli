package bomio

import (
	"path/filepath"
	"reflect"
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/generator"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/scanner"
)

func bomNamed(name string) *cdx.BOM {
	bom := cdx.NewBOM()
	bom.Metadata = &cdx.Metadata{Component: &cdx.Component{Type: cdx.ComponentTypeMachineLearningModel, Name: name}}
	return bom
}

func TestWriteOutputFiles_NamesFromRequestedRef(t *testing.T) {
	dir := t.TempDir()
	resolved := "openai-community/gpt2"
	boms := []generator.DiscoveredBOM{
		{Discovery: scanner.Discovery{ID: "gpt2"}, BOM: bomNamed(resolved)},
		{Discovery: scanner.Discovery{ID: "openai-community/gpt2"}, BOM: bomNamed(resolved)},
		{Discovery: scanner.Discovery{ID: "gpt2", Revision: "v1.0"}, BOM: bomNamed(resolved)},
		{Discovery: scanner.Discovery{ID: "org/x"}, BOM: bomNamed("org/x")},
		{Discovery: scanner.Discovery{ID: "org_x"}, BOM: bomNamed("org_x")},
		{BOM: bomNamed("from/component")},
	}
	written, err := WriteOutputFiles(boms, dir, ".json", "json", "")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range written {
		names = append(names, filepath.Base(p))
	}
	want := []string{
		"gpt2_aibom.json",
		"openai-community_gpt2_aibom.json",
		"gpt2_v1.0_aibom.json",
		"org_x_aibom.json",
		"org_x_2_aibom.json", // sanitizing collision gets a suffix instead of overwriting
		"from_component_aibom.json",
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("file names = %v, want %v", names, want)
	}
}
