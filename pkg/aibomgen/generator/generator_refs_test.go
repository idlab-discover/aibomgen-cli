package generator

import (
	"net/http"
	"reflect"
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"

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
	newFetcherSet = func(*http.Client, string) fetcherSet { return fs }
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

func TestBuildFromModelIDs_InputsOutputs(t *testing.T) {
	tags := map[string]string{
		"google-bert/bert-base-uncased":          "fill-mask",
		"sentence-transformers/all-MiniLM-L6-v2": "sentence-similarity",
		"openai/whisper-large-v3":                "automatic-speech-recognition",
	}
	api := &mockModelAPIFetcher{fetchFunc: func(id string) (*fetcher.ModelAPIResponse, error) {
		return &fetcher.ModelAPIResponse{ID: id, SHA: "abc", PipelineTag: tags[id]}, nil
	}}
	withFetchers(t, hfLikeFetchers(api))

	ids := []string{"google-bert/bert-base-uncased", "sentence-transformers/all-MiniLM-L6-v2", "openai/whisper-large-v3"}
	boms, err := BuildFromModelIDs(ids, GenerateOptions{})
	if err != nil || len(boms) != len(ids) {
		t.Fatalf("BuildFromModelIDs = %d boms, err %v", len(boms), err)
	}

	want := map[string][2][]string{
		"google-bert/bert-base-uncased":          {{"text"}, {"text"}},
		"sentence-transformers/all-MiniLM-L6-v2": {{"text"}, {"embedding"}},
		"openai/whisper-large-v3":                {{"audio"}, {"text"}},
	}
	formats := func(ps *[]cdx.MLInputOutputParameters) []string {
		if ps == nil {
			return nil
		}
		out := make([]string, 0, len(*ps))
		for _, p := range *ps {
			out = append(out, p.Format)
		}
		return out
	}
	for _, b := range boms {
		id := b.Discovery.ID
		mp := b.BOM.Metadata.Component.ModelCard.ModelParameters
		if mp.Task != tags[id] {
			t.Errorf("%s: task = %q, want %q", id, mp.Task, tags[id])
		}
		if got := formats(mp.Inputs); !reflect.DeepEqual(got, want[id][0]) {
			t.Errorf("%s: inputs = %v, want %v", id, got, want[id][0])
		}
		if got := formats(mp.Outputs); !reflect.DeepEqual(got, want[id][1]) {
			t.Errorf("%s: outputs = %v, want %v", id, got, want[id][1])
		}
	}
}

func TestBuildFromModelIDs_Pedigree(t *testing.T) {
	lineage := map[string]*fetcher.ModelBaseModels{
		"Qwen/Qwen2.5-7B-Instruct": {Relation: "finetune"},
		"nvidia/Eagle2.5-8B":       {Relation: "merge"},
	}
	bases := map[string][]any{
		"Qwen/Qwen2.5-7B-Instruct": {"Qwen/Qwen2.5-7B"},
		"nvidia/Eagle2.5-8B":       {"Qwen/Qwen2.5-7B-Instruct", "google/siglip2-so400m-patch16-512"},
	}
	api := &mockModelAPIFetcher{fetchFunc: func(id string) (*fetcher.ModelAPIResponse, error) {
		return &fetcher.ModelAPIResponse{
			ID: id, SHA: "abc",
			CardData:   map[string]any{"base_model": bases[id]},
			BaseModels: lineage[id],
		}, nil
	}}
	withFetchers(t, hfLikeFetchers(api))

	boms, err := BuildFromModelIDs([]string{"Qwen/Qwen2.5-7B-Instruct", "nvidia/Eagle2.5-8B"}, GenerateOptions{})
	if err != nil || len(boms) != 2 {
		t.Fatalf("BuildFromModelIDs = %d boms, err %v", len(boms), err)
	}

	want := map[string]struct {
		purls []string
		notes string
	}{
		"Qwen/Qwen2.5-7B-Instruct": {[]string{"pkg:huggingface/Qwen/Qwen2.5-7B"}, "finetune of Qwen/Qwen2.5-7B"},
		"nvidia/Eagle2.5-8B": {
			[]string{"pkg:huggingface/Qwen/Qwen2.5-7B-Instruct", "pkg:huggingface/google/siglip2-so400m-patch16-512"},
			"merge of Qwen/Qwen2.5-7B-Instruct, google/siglip2-so400m-patch16-512",
		},
	}
	for _, b := range boms {
		id := b.Discovery.ID
		p := b.BOM.Metadata.Component.Pedigree
		if p == nil || p.Ancestors == nil {
			t.Fatalf("%s: no pedigree", id)
		}
		var purls []string
		for _, a := range *p.Ancestors {
			if a.BOMRef != a.PackageURL {
				t.Errorf("%s: ancestor bom-ref %q != purl %q", id, a.BOMRef, a.PackageURL)
			}
			purls = append(purls, a.PackageURL)
		}
		if !reflect.DeepEqual(purls, want[id].purls) || p.Notes != want[id].notes {
			t.Errorf("%s: purls = %v, notes = %q; want %v, %q", id, purls, p.Notes, want[id].purls, want[id].notes)
		}
		if dangling := validator.DanglingRefs(b.BOM); len(dangling) != 0 {
			t.Errorf("%s: dangling refs %v", id, dangling)
		}
	}
}

func TestBuild_Evidence(t *testing.T) {
	api := &mockModelAPIFetcher{fetchFunc: func(id string) (*fetcher.ModelAPIResponse, error) {
		return &fetcher.ModelAPIResponse{ID: id, SHA: "abc"}, nil
	}}
	withFetchers(t, hfLikeFetchers(api))

	// Source scan: the same model found in two files.
	d := scanner.Discovery{ID: "org/m", Name: "org/m", Type: "model", Occurrences: []scanner.Occurrence{
		{Location: "a/one.py", Line: 3, Method: "from_pretrained", Symbol: "org/m"},
		{Location: "two.yaml", Line: 1, Method: "yaml_model_field", Symbol: "org/m"},
	}}
	boms, err := BuildPerDiscovery([]scanner.Discovery{d}, GenerateOptions{})
	if err != nil || len(boms) != 1 {
		t.Fatalf("BuildPerDiscovery = %d boms, err %v", len(boms), err)
	}
	ev := boms[0].BOM.Metadata.Component.Evidence
	if ev == nil || ev.Occurrences == nil || len(*ev.Occurrences) != 2 {
		t.Fatalf("evidence = %+v, want two occurrences", ev)
	}
	if o := (*ev.Occurrences)[1]; o.Location != "two.yaml" || o.Line == nil || *o.Line != 1 {
		t.Fatalf("second occurrence = %+v", o)
	}

	// Model-ID input: identified through the Hub API.
	boms, err = BuildFromModelIDs([]string{"org/m"}, GenerateOptions{})
	if err != nil || len(boms) != 1 {
		t.Fatalf("BuildFromModelIDs = %d boms, err %v", len(boms), err)
	}
	ev = boms[0].BOM.Metadata.Component.Evidence
	if ev == nil || ev.Identity == nil || ev.Identity.Identities == nil {
		t.Fatalf("evidence = %+v", ev)
	}
	id := (*ev.Identity.Identities)[0]
	if m := (*id.Methods)[0]; m.Technique != cdx.EvidenceIdentityTechniqueOther || m.Value != "https://huggingface.co/api/models/org/m" {
		t.Fatalf("method = %+v", m)
	}
	if id.ConcludedValue != "pkg:huggingface/org/m@abc" {
		t.Fatalf("concludedValue = %q", id.ConcludedValue)
	}
}

// A dataset known only from a dataset:<id> tag gets a component, and the card's
// datasets entry (built from the same tag) references it.
func TestBuildFromModelIDs_DatasetTagFallback(t *testing.T) {
	api := &mockModelAPIFetcher{fetchFunc: func(id string) (*fetcher.ModelAPIResponse, error) {
		return &fetcher.ModelAPIResponse{ID: "org/model", SHA: "abc", Tags: []string{"pytorch", "dataset:wikipedia"}}, nil
	}}
	withFetchers(t, hfLikeFetchers(api))

	boms, err := BuildFromModelIDs([]string{"org/model"}, GenerateOptions{})
	if err != nil || len(boms) != 1 {
		t.Fatalf("BuildFromModelIDs = %d boms, err %v", len(boms), err)
	}
	bom := boms[0].BOM
	if bom.Components == nil || len(*bom.Components) != 1 {
		t.Fatalf("components = %+v, want the wikipedia data component", bom.Components)
	}
	wantRef := "pkg:huggingface/datasets/legacy-datasets/wikipedia@" + wikiSHA
	ds := *bom.Metadata.Component.ModelCard.ModelParameters.Datasets
	if len(ds) != 1 || ds[0].Ref != wantRef {
		t.Fatalf("modelCard datasets = %+v, want a ref to %s", ds, wantRef)
	}
	if dangling := validator.DanglingRefs(bom); len(dangling) != 0 {
		t.Fatalf("dangling refs: %v", dangling)
	}
}
