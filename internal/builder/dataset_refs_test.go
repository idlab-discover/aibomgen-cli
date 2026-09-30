package builder

import (
	"reflect"
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

const (
	bookcorpusRef = "pkg:huggingface/datasets/bookcorpus/bookcorpus@d917559bbe9cf49c638fc331c37c4bf239e3b637"
	wikipediaRef  = "pkg:huggingface/datasets/legacy-datasets/wikipedia@97a0b052c326b45fb68593a14972d9eed884cd17"
)

func modelBOM(cardRefs []string, componentRefs ...string) *cdx.BOM {
	choices := make([]cdx.MLDatasetChoice, 0, len(cardRefs))
	for _, r := range cardRefs {
		choices = append(choices, cdx.MLDatasetChoice{Ref: r})
	}
	comps := make([]cdx.Component, 0, len(componentRefs))
	for _, r := range componentRefs {
		comps = append(comps, cdx.Component{Type: cdx.ComponentTypeData, BOMRef: r})
	}
	return &cdx.BOM{
		Metadata: &cdx.Metadata{Component: &cdx.Component{
			BOMRef:    "pkg:huggingface/org/model@abc",
			ModelCard: &cdx.MLModelCard{ModelParameters: &cdx.MLModelParameters{Datasets: &choices}},
		}},
		Components: &comps,
	}
}

// entry is a compact view of a dataset choice: a ref, or an inline name (+url).
type entry struct{ ref, name, url string }

func datasetEntries(bom *cdx.BOM) []entry {
	mp := bom.Metadata.Component.ModelCard.ModelParameters
	if mp == nil || mp.Datasets == nil {
		return nil
	}
	var out []entry
	for _, d := range *mp.Datasets {
		e := entry{ref: d.Ref}
		if d.ComponentData != nil {
			if d.ComponentData.Type != cdx.ComponentDataTypeDataset {
				panic("inline dataset without type dataset")
			}
			e.name = d.ComponentData.Name
			if d.ComponentData.Contents != nil {
				e.url = d.ComponentData.Contents.URL
			}
		}
		out = append(out, e)
	}
	return out
}

func TestLinkDatasetRefs(t *testing.T) {
	tests := []struct {
		name     string
		bom      *cdx.BOM
		resolved map[string]string
		want     []entry
	}{
		{
			name:     "resolved and renamed datasets use component bom-refs",
			bom:      modelBOM([]string{"dataset:bookcorpus", "dataset:wikipedia"}, bookcorpusRef, wikipediaRef),
			resolved: map[string]string{"bookcorpus": bookcorpusRef, "wikipedia": wikipediaRef},
			want:     []entry{{ref: bookcorpusRef}, {ref: wikipediaRef}},
		},
		{
			name:     "unresolved dataset becomes inline entry",
			bom:      modelBOM([]string{"dataset:bookcorpus", "dataset:private-corpus"}, bookcorpusRef),
			resolved: map[string]string{"bookcorpus": bookcorpusRef},
			want:     []entry{{ref: bookcorpusRef}, {name: "private-corpus"}},
		},
		{
			name: "url card value keeps contents.url",
			bom:  modelBOM([]string{"https://example.com/data.csv"}),
			want: []entry{{name: "https://example.com/data.csv", url: "https://example.com/data.csv"}},
		},
		{
			name:     "two card names resolving to one component yield one ref",
			bom:      modelBOM([]string{"dataset:wikipedia", "dataset:legacy-datasets/wikipedia"}, wikipediaRef),
			resolved: map[string]string{"wikipedia": wikipediaRef, "legacy-datasets/wikipedia": wikipediaRef},
			want:     []entry{{ref: wikipediaRef}},
		},
		{
			name:     "component missing from card list is appended",
			bom:      modelBOM([]string{"dataset:bookcorpus"}, bookcorpusRef, wikipediaRef),
			resolved: map[string]string{"bookcorpus": bookcorpusRef, "wikipedia": wikipediaRef},
			want:     []entry{{ref: bookcorpusRef}, {ref: wikipediaRef}},
		},
		{
			name:     "matching is case-insensitive",
			bom:      modelBOM([]string{"dataset:BookCorpus"}, bookcorpusRef),
			resolved: map[string]string{"bookcorpus": bookcorpusRef},
			want:     []entry{{ref: bookcorpusRef}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			LinkDatasetRefs(tt.bom, tt.resolved)
			if got := datasetEntries(tt.bom); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("datasets = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestLinkDatasetRefsCreatesModelCard(t *testing.T) {
	bom := &cdx.BOM{
		Metadata:   &cdx.Metadata{Component: &cdx.Component{}},
		Components: &[]cdx.Component{{Type: cdx.ComponentTypeData, BOMRef: bookcorpusRef}},
	}
	LinkDatasetRefs(bom, map[string]string{"bookcorpus": bookcorpusRef})
	if got := datasetEntries(bom); !reflect.DeepEqual(got, []entry{{ref: bookcorpusRef}}) {
		t.Fatalf("datasets = %+v", got)
	}
}

func TestLinkDatasetRefsNoop(t *testing.T) {
	LinkDatasetRefs(nil, nil)
	LinkDatasetRefs(&cdx.BOM{}, map[string]string{"a": "b"})
	bom := &cdx.BOM{Metadata: &cdx.Metadata{Component: &cdx.Component{}}}
	LinkDatasetRefs(bom, nil)
	if bom.Metadata.Component.ModelCard != nil {
		t.Fatalf("expected no model card to be created")
	}
}

func TestDatasetRefKey(t *testing.T) {
	for in, want := range map[string]string{"dataset:BookCorpus": "bookcorpus", " Org/DS ": "org/ds", "dataset: x ": "x"} {
		if got := DatasetRefKey(in); got != want {
			t.Errorf("DatasetRefKey(%q) = %q, want %q", in, got, want)
		}
	}
}
