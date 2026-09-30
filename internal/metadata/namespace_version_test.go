package metadata

import (
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/scanner"
)

func datasetSpecFor(t *testing.T, key DatasetKey) DatasetFieldSpec {
	t.Helper()
	for _, spec := range DatasetRegistry() {
		if spec.Key == key {
			return spec
		}
	}
	t.Fatalf("missing dataset spec %s", key)
	return DatasetFieldSpec{}
}

func applyModel(src Source) *cdx.Component {
	comp := &cdx.Component{ModelCard: &cdx.MLModelCard{}}
	tgt := Target{Component: comp, ModelCard: comp.ModelCard, HuggingFaceBaseURL: "https://huggingface.co/"}
	for _, spec := range Registry() {
		ApplyFromSources(spec, src, tgt)
	}
	return comp
}

func entityURL(e *cdx.OrganizationalEntity) string {
	if e == nil || e.URL == nil || len(*e.URL) == 0 {
		return ""
	}
	return (*e.URL)[0]
}

func authorNames(c *cdx.Component) []string {
	if c.Authors == nil {
		return nil
	}
	var out []string
	for _, a := range *c.Authors {
		out = append(out, a.Name)
	}
	return out
}

func TestModelNamespaceFields(t *testing.T) {
	comp := applyModel(Source{
		ModelID: "google-bert/bert-base-uncased",
		HF:      &fetcher.ModelAPIResponse{ID: "google-bert/bert-base-uncased", Author: "google-bert"},
	})
	if comp.Supplier == nil || comp.Supplier.Name != "google-bert" || entityURL(comp.Supplier) != "https://huggingface.co/google-bert" {
		t.Fatalf("supplier = %+v", comp.Supplier)
	}
	if comp.Manufacturer == nil || comp.Manufacturer.Name != "google-bert" || entityURL(comp.Manufacturer) != "https://huggingface.co/google-bert" {
		t.Fatalf("manufacturer = %+v", comp.Manufacturer)
	}
	if got := authorNames(comp); len(got) != 1 || got[0] != "google-bert" {
		t.Fatalf("authors = %v", got)
	}
	if comp.Group != "google-bert" {
		t.Fatalf("group = %q", comp.Group)
	}
}

func TestModelNamespaceWithoutOrg(t *testing.T) {
	// No HF data and no org: nothing may be derived.
	comp := applyModel(Source{ModelID: "gpt2"})
	if comp.Supplier != nil || comp.Manufacturer != nil || comp.Authors != nil || comp.Group != "" {
		t.Fatalf("expected no namespace fields, got supplier=%+v manufacturer=%+v authors=%v group=%q",
			comp.Supplier, comp.Manufacturer, authorNames(comp), comp.Group)
	}

	// HF resolves the short ID: namespace comes from the resolved ID.
	comp = applyModel(Source{ModelID: "gpt2", HF: &fetcher.ModelAPIResponse{ID: "openai-community/gpt2"}})
	if comp.Supplier == nil || comp.Supplier.Name != "openai-community" {
		t.Fatalf("supplier = %+v", comp.Supplier)
	}
	if comp.Name != "openai-community/gpt2" {
		t.Fatalf("name = %q, want resolved ID", comp.Name)
	}
	if comp.Group != "openai-community" {
		t.Fatalf("group = %q", comp.Group)
	}
}

func TestModelAuthorsAndManufacturerFromCard(t *testing.T) {
	// Real "Developed by" names the authors; a non-namespace manufacturer gets no HF URL.
	comp := applyModel(Source{
		ModelID: "org/m",
		Readme:  &fetcher.ModelReadmeCard{DevelopedBy: "Acme Research"},
	})
	if got := authorNames(comp); len(got) != 1 || got[0] != "Acme Research" {
		t.Fatalf("authors = %v", got)
	}
	if comp.Manufacturer == nil || comp.Manufacturer.Name != "Acme Research" || comp.Manufacturer.URL != nil {
		t.Fatalf("manufacturer = %+v", comp.Manufacturer)
	}

	// Placeholder "Developed by" is ignored everywhere.
	comp = applyModel(Source{
		ModelID: "org/m",
		Readme:  &fetcher.ModelReadmeCard{DevelopedBy: "[More Information Needed]"},
	})
	if got := authorNames(comp); len(got) != 1 || got[0] != "org" {
		t.Fatalf("authors = %v, want namespace fallback", got)
	}
	if comp.Manufacturer != nil {
		t.Fatalf("manufacturer = %+v, want none", comp.Manufacturer)
	}
}

func TestModelVersion(t *testing.T) {
	const sha = "86B5E0934494BD15C9632B12F734A8A67F723594"
	tests := []struct {
		name string
		src  Source
		want string
	}{
		{"commit SHA lowercased", Source{HF: &fetcher.ModelAPIResponse{SHA: sha}}, "86b5e0934494bd15c9632b12f734a8a67f723594"},
		{"named revision", Source{Revision: "v1.0", HF: &fetcher.ModelAPIResponse{SHA: sha}}, "v1.0"},
		{"SHA revision uses resolved commit", Source{Revision: "86b5e0934494bd15c9632b12f734a8a67f723594", HF: &fetcher.ModelAPIResponse{SHA: sha}}, "86b5e0934494bd15c9632b12f734a8a67f723594"},
		{"no HF data", Source{ModelID: "org/m"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := applyModel(tt.src).Version; got != tt.want {
				t.Fatalf("version = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDatasetNamespaceFields(t *testing.T) {
	comp := &cdx.Component{}
	src := DatasetSource{
		DatasetID: "wikipedia",
		Scan:      scanner.Discovery{ID: "wikipedia", Name: "wikipedia", Type: "dataset"},
		HF:        &fetcher.DatasetAPIResponse{ID: "legacy-datasets/wikipedia", Author: "legacy-datasets"},
	}
	for _, spec := range DatasetRegistry() {
		ApplyDatasetFromSources(spec, src, DatasetTarget{Component: comp})
	}
	if comp.Name != "legacy-datasets/wikipedia" {
		t.Fatalf("name = %q, want resolved ID", comp.Name)
	}
	if comp.Supplier == nil || comp.Supplier.Name != "legacy-datasets" || entityURL(comp.Supplier) != "https://huggingface.co/legacy-datasets" {
		t.Fatalf("supplier = %+v", comp.Supplier)
	}
	if entityURL(comp.Manufacturer) != "https://huggingface.co/legacy-datasets" {
		t.Fatalf("manufacturer = %+v", comp.Manufacturer)
	}

	// Without HF data and without org: no supplier and no group.
	comp = &cdx.Component{}
	for _, spec := range DatasetRegistry() {
		ApplyDatasetFromSources(spec, DatasetSource{DatasetID: "wikipedia"}, DatasetTarget{Component: comp})
	}
	if comp.Supplier != nil || comp.Group != "" {
		t.Fatalf("expected no supplier/group, got %+v %q", comp.Supplier, comp.Group)
	}
}
