package generator

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/scanner"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/validator"
)

const wikiSHA = "97a0b052c326b45fb68593a14972d9eed884cd17"

// hfLikeFetchers simulates the HF Hub: "wikipedia" redirects to legacy-datasets/wikipedia,
// "missing" does not exist.
func hfLikeFetchers(api *mockModelAPIFetcher) fetcherSet {
	return fetcherSet{
		modelAPI:    api,
		modelReadme: &mockModelReadmeFetcher{},
		datasetAPI: &mockDatasetAPIFetcher{fetchFunc: func(id string) (*fetcher.DatasetAPIResponse, error) {
			switch id {
			case "wikipedia", "legacy-datasets/wikipedia":
				return &fetcher.DatasetAPIResponse{ID: "legacy-datasets/wikipedia", Author: "legacy-datasets", SHA: wikiSHA}, nil
			default:
				return nil, &fetcher.HFError{StatusCode: http.StatusNotFound}
			}
		}},
		datasetReadme: &mockDatasetReadmeFetcher{},
		modelTree:     &fetcher.DummyModelTreeFetcher{},
	}
}

func withFetchers(t *testing.T, fs fetcherSet) {
	t.Helper()
	orig := newFetcherSet
	newFetcherSet = func(*http.Client) fetcherSet { return fs }
	t.Cleanup(func() { newFetcherSet = orig })
}

func TestBuildFromModelIDs_DatasetRefsResolve(t *testing.T) {
	api := &mockModelAPIFetcher{fetchFunc: func(id string) (*fetcher.ModelAPIResponse, error) {
		return &fetcher.ModelAPIResponse{
			ID:       "org/model",
			SHA:      "abc",
			CardData: map[string]any{"datasets": []any{"wikipedia", "legacy-datasets/wikipedia", "missing"}},
		}, nil
	}}
	withFetchers(t, hfLikeFetchers(api))

	boms, err := BuildFromModelIDs([]string{"org/model"}, GenerateOptions{})
	if err != nil || len(boms) != 1 {
		t.Fatalf("BuildFromModelIDs = %d boms, err %v", len(boms), err)
	}
	bom := boms[0].BOM

	if dangling := validator.DanglingRefs(bom); len(dangling) != 0 {
		t.Fatalf("dangling refs: %v", dangling)
	}
	if bom.Components == nil || len(*bom.Components) != 1 {
		t.Fatalf("expected one deduplicated data component, got %+v", bom.Components)
	}
	wantRef := "pkg:huggingface/datasets/legacy-datasets/wikipedia@" + wikiSHA
	if got := (*bom.Components)[0].BOMRef; got != wantRef {
		t.Fatalf("data component bom-ref = %q, want %q", got, wantRef)
	}

	ds := *bom.Metadata.Component.ModelCard.ModelParameters.Datasets
	if len(ds) != 2 || ds[0].Ref != wantRef || ds[1].ComponentData == nil || ds[1].ComponentData.Name != "missing" {
		t.Fatalf("modelCard datasets = %+v", ds)
	}
}

func TestBuildDummyBOM_RefsResolve(t *testing.T) {
	boms, err := BuildDummyBOM()
	if err != nil || len(boms) != 1 {
		t.Fatalf("BuildDummyBOM = %d boms, err %v", len(boms), err)
	}
	if dangling := validator.DanglingRefs(boms[0].BOM); len(dangling) != 0 {
		t.Fatalf("dangling refs: %v", dangling)
	}
}

func TestBuildPerDiscovery_RefsResolveAndRevision(t *testing.T) {
	api := &mockModelAPIFetcher{fetchFunc: func(id string) (*fetcher.ModelAPIResponse, error) {
		return &fetcher.ModelAPIResponse{ID: "org/model", SHA: "abc", CardData: map[string]any{"datasets": "wikipedia"}}, nil
	}}
	withFetchers(t, hfLikeFetchers(api))

	boms, err := BuildPerDiscovery([]scanner.Discovery{{ID: "org/model", Name: "org/model", Type: "model", Revision: "v2"}}, GenerateOptions{})
	if err != nil || len(boms) != 1 {
		t.Fatalf("BuildPerDiscovery = %d boms, err %v", len(boms), err)
	}
	if dangling := validator.DanglingRefs(boms[0].BOM); len(dangling) != 0 {
		t.Fatalf("dangling refs: %v", dangling)
	}
	if !reflect.DeepEqual(api.revisions, []string{"v2"}) {
		t.Fatalf("revisions passed to API = %v", api.revisions)
	}
	if v := boms[0].BOM.Metadata.Component.Version; v != "v2" {
		t.Fatalf("version = %q, want v2", v)
	}
}

func TestBuildFromModelIDs_Revisions(t *testing.T) {
	api := &mockModelAPIFetcher{fetchFunc: func(id string) (*fetcher.ModelAPIResponse, error) {
		return &fetcher.ModelAPIResponse{ID: id, SHA: "ABC"}, nil
	}}
	withFetchers(t, hfLikeFetchers(api))

	boms, err := BuildFromModelIDs([]string{"org/model@v1.0", "org/model", "org/model@"}, GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(boms) != 2 {
		t.Fatalf("expected 2 BOMs (invalid ref skipped), got %d", len(boms))
	}
	if !reflect.DeepEqual(api.revisions, []string{"v1.0", ""}) {
		t.Fatalf("revisions passed to API = %v", api.revisions)
	}
	if got := boms[0].Discovery.Revision; got != "v1.0" {
		t.Fatalf("discovery revision = %q", got)
	}
	if got := boms[0].BOM.Metadata.Component.Version; got != "v1.0" {
		t.Fatalf("named revision version = %q", got)
	}
	if got := boms[1].BOM.Metadata.Component.Version; got != "abc" {
		t.Fatalf("default version = %q, want resolved sha", got)
	}
	// purl always pins the commit.
	for _, b := range boms {
		if got := b.BOM.Metadata.Component.PackageURL; got != "pkg:huggingface/org/model@abc" {
			t.Fatalf("purl = %q", got)
		}
	}
}

func TestParseModelRef(t *testing.T) {
	tests := []struct {
		in      string
		want    ModelRef
		wantErr bool
	}{
		{in: "org/name", want: ModelRef{ID: "org/name"}},
		{in: " gpt2 ", want: ModelRef{ID: "gpt2"}},
		{in: "org/name@v1", want: ModelRef{ID: "org/name", Revision: "v1"}},
		{in: "org/name@refs/pr/1", want: ModelRef{ID: "org/name", Revision: "refs/pr/1"}},
		{in: "org/name@", wantErr: true},
		{in: "@v1", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tt := range tests {
		got, err := ParseModelRef(tt.in)
		if (err != nil) != tt.wantErr {
			t.Fatalf("ParseModelRef(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
		}
		if !tt.wantErr && got != tt.want {
			t.Fatalf("ParseModelRef(%q) = %+v, want %+v", tt.in, got, tt.want)
		}
	}
	if s := (ModelRef{ID: "a/b", Revision: "v1"}).String(); s != "a/b@v1" {
		t.Fatalf("String() = %q", s)
	}
}
