package metadata

import (
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
)

func hubBaseModels(relation string, ids ...string) *fetcher.ModelBaseModels {
	bm := &fetcher.ModelBaseModels{Relation: relation}
	for _, id := range ids {
		bm.Models = append(bm.Models, struct {
			ID string `json:"id"`
		}{ID: id})
	}
	return bm
}

func ancestorNames(c *cdx.Component) []string {
	if c.Pedigree == nil || c.Pedigree.Ancestors == nil {
		return nil
	}
	var out []string
	for _, a := range *c.Pedigree.Ancestors {
		out = append(out, a.Name)
	}
	return out
}

func TestPedigree_FineTune(t *testing.T) {
	comp := applyModel(Source{
		ModelID: "Qwen/Qwen2.5-7B-Instruct",
		HF:      &fetcher.ModelAPIResponse{ID: "Qwen/Qwen2.5-7B-Instruct", BaseModels: hubBaseModels("finetune", "Qwen/Qwen2.5-7B")},
		Readme:  &fetcher.ModelReadmeCard{BaseModel: "Qwen/Qwen2.5-7B", BaseModels: []string{"Qwen/Qwen2.5-7B"}},
	})
	p := comp.Pedigree
	if p == nil || p.Ancestors == nil || len(*p.Ancestors) != 1 {
		t.Fatalf("pedigree = %+v, want one ancestor", p)
	}
	a := (*p.Ancestors)[0]
	if a.Type != cdx.ComponentTypeMachineLearningModel || a.Name != "Qwen/Qwen2.5-7B" || a.Group != "Qwen" {
		t.Fatalf("ancestor = %+v", a)
	}
	if a.ExternalReferences == nil || (*a.ExternalReferences)[0].URL != "https://huggingface.co/Qwen/Qwen2.5-7B" {
		t.Fatalf("ancestor externalReferences = %+v", a.ExternalReferences)
	}
	if p.Notes != "finetune of Qwen/Qwen2.5-7B" {
		t.Fatalf("notes = %q", p.Notes)
	}
	// The vendor property stays for backward compatibility.
	if !hasProperty(comp, "huggingface:baseModel") {
		t.Fatalf("huggingface:baseModel property missing")
	}
}

func TestPedigree_Merge(t *testing.T) {
	comp := applyModel(Source{
		ModelID: "org/merged",
		HF:      &fetcher.ModelAPIResponse{BaseModels: hubBaseModels("finetune", "org/a")},
		Readme:  &fetcher.ModelReadmeCard{BaseModels: []string{"org/a", "org/b"}, BaseModelRelation: "merge"},
	})
	if got := ancestorNames(comp); len(got) != 2 || got[0] != "org/a" || got[1] != "org/b" {
		t.Fatalf("ancestors = %v", got)
	}
	// Front matter relation beats the Hub's inferred relation.
	if comp.Pedigree.Notes != "merge of org/a, org/b" {
		t.Fatalf("notes = %q", comp.Pedigree.Notes)
	}
}

func TestPedigree_SourcesAndFiltering(t *testing.T) {
	comp := applyModel(Source{
		ModelID: "org/m",
		Readme: &fetcher.ModelReadmeCard{BaseModels: []string{
			"https://huggingface.co/org/x", "./local/checkpoint", "[More Information Needed]",
			"gpt2", "GPT2", "org/m", "org/base",
		}},
	})
	if got := ancestorNames(comp); len(got) != 2 || got[0] != "gpt2" || got[1] != "org/base" {
		t.Fatalf("ancestors = %v, want gpt2 and org/base", got)
	}
	if g := (*comp.Pedigree.Ancestors)[0].Group; g != "" {
		t.Fatalf("legacy ID group = %q, want empty", g)
	}
	if comp.Pedigree.Notes != "derived from gpt2, org/base (relation unknown)" {
		t.Fatalf("notes = %q", comp.Pedigree.Notes)
	}

	// Without a README, cardData and then the Hub's baseModels are used.
	comp = applyModel(Source{ModelID: "org/m", HF: &fetcher.ModelAPIResponse{
		CardData: map[string]any{"base_model": []any{"org/c"}, "base_model_relation": "adapter"},
	}})
	if got := ancestorNames(comp); len(got) != 1 || got[0] != "org/c" || comp.Pedigree.Notes != "adapter of org/c" {
		t.Fatalf("cardData: ancestors = %v, notes = %q", got, comp.Pedigree.Notes)
	}
	comp = applyModel(Source{ModelID: "org/m", HF: &fetcher.ModelAPIResponse{BaseModels: hubBaseModels("quantized", "org/d")}})
	if got := ancestorNames(comp); len(got) != 1 || got[0] != "org/d" || comp.Pedigree.Notes != "quantized of org/d" {
		t.Fatalf("hub: ancestors = %v, notes = %q", got, comp.Pedigree.Notes)
	}

	// The legacy comma-joined BaseModel still works.
	comp = applyModel(Source{ModelID: "org/m", Readme: &fetcher.ModelReadmeCard{BaseModel: "org/a, org/b"}})
	if got := ancestorNames(comp); len(got) != 2 {
		t.Fatalf("joined BaseModel: ancestors = %v", got)
	}
}

func TestPedigree_NoBaseModel(t *testing.T) {
	comp := applyModel(Source{ModelID: "google-bert/bert-base-uncased", HF: &fetcher.ModelAPIResponse{}, Readme: &fetcher.ModelReadmeCard{}})
	if comp.Pedigree != nil {
		t.Fatalf("pedigree = %+v, want none", comp.Pedigree)
	}
}
