package metadata

import (
	"fmt"
	"regexp"
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

// pedigreeValue is the lineage of a model: its base model IDs and the relation to them.
type pedigreeValue struct {
	IDs      []string
	Relation string
}

// hfRepoIDRe matches a Hugging Face repository ID: "org/name", or a legacy
// single-segment ID such as "gpt2". URLs and local paths don't match.
var hfRepoIDRe = regexp.MustCompile(`^[A-Za-z0-9][\w.-]*(?:/[\w.-]+)?$`)

func pedigreeFields() []FieldSpec {
	return []FieldSpec{
		{
			Key: ComponentPedigreeAncestors,
			// Not scored: foundation models legitimately have no ancestors.
			Weight: 0,
			Sources: []func(Source) (any, bool){
				func(src Source) (any, bool) {
					ids := baseModelIDs(src)
					if len(ids) == 0 {
						return nil, false
					}
					return pedigreeValue{IDs: ids, Relation: baseModelRelation(src)}, true
				},
			},
			Parse: func(value string) (any, error) {
				ids, err := parseCommaList(value, "base models")
				if err != nil {
					return nil, err
				}
				return pedigreeValue{IDs: ids}, nil
			},
			Apply: func(tgt Target, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", ComponentPedigreeAncestors)
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				v, _ := input.Value.(pedigreeValue)
				if len(v.IDs) == 0 {
					return fmt.Errorf("base models value is empty")
				}
				if p := tgt.Component.Pedigree; !input.Force && p != nil && p.Ancestors != nil && len(*p.Ancestors) > 0 {
					return nil
				}

				base := hfBaseURL(tgt.HuggingFaceBaseURL)
				ancestors := make([]cdx.Component, 0, len(v.IDs))
				for _, id := range v.IDs {
					ns, _ := hfNamespace(id)
					ancestors = append(ancestors, cdx.Component{
						Type:  cdx.ComponentTypeMachineLearningModel,
						Group: ns,
						Name:  id,
						ExternalReferences: &[]cdx.ExternalReference{{
							Type: cdx.ExternalReferenceType("website"),
							URL:  base + id,
						}},
					})
				}
				if tgt.Component.Pedigree == nil {
					tgt.Component.Pedigree = &cdx.Pedigree{}
				}
				tgt.Component.Pedigree.Ancestors = &ancestors
				tgt.Component.Pedigree.Notes = pedigreeNotes(v)
				return nil
			},
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				return c != nil && c.Pedigree != nil && c.Pedigree.Ancestors != nil && len(*c.Pedigree.Ancestors) > 0
			},
			InputType:   InputTypeMultiText,
			Placeholder: "org/base-model, org/other-base-model",
		},
	}
}

// baseModelIDs returns the model's base model IDs: README front matter base_model,
// else cardData.base_model, else the Hub's baseModels. Values that aren't Hub repo IDs
// (URLs, local paths, placeholders), duplicates and the model's own ID are dropped.
func baseModelIDs(src Source) []string {
	var candidates [][]string
	if src.Readme != nil {
		ids := src.Readme.BaseModels
		if len(ids) == 0 && src.Readme.BaseModel != "" {
			// Cards built without BaseModels only carry the comma-joined value.
			ids = strings.Split(src.Readme.BaseModel, ",")
		}
		candidates = append(candidates, ids)
	}
	if src.HF != nil {
		candidates = append(candidates, stringsFromAny(src.HF.CardData["base_model"]))
		if bm := src.HF.BaseModels; bm != nil {
			var ids []string
			for _, m := range bm.Models {
				ids = append(ids, m.ID)
			}
			candidates = append(candidates, ids)
		}
	}

	self := map[string]bool{strings.ToLower(strings.TrimSpace(src.ModelID)): true}
	if src.HF != nil {
		self[strings.ToLower(strings.TrimSpace(src.HF.ID))] = true
	}

	for _, list := range candidates {
		seen := map[string]bool{}
		var out []string
		for _, id := range list {
			id = realString(id)
			key := strings.ToLower(id)
			if id == "" || !hfRepoIDRe.MatchString(id) || self[key] || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, id)
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

// baseModelRelation returns the relation to the base models: README front matter
// base_model_relation, else cardData.base_model_relation, else the Hub's inferred relation.
func baseModelRelation(src Source) string {
	var rels []string
	if src.Readme != nil {
		rels = append(rels, src.Readme.BaseModelRelation)
	}
	if src.HF != nil {
		r, _ := src.HF.CardData["base_model_relation"].(string)
		rels = append(rels, r)
		if src.HF.BaseModels != nil {
			rels = append(rels, src.HF.BaseModels.Relation)
		}
	}
	for _, r := range rels {
		if r = strings.ToLower(realString(r)); r != "" {
			return r
		}
	}
	return ""
}

// pedigreeNotes describes the lineage, e.g. "finetune of Qwen/Qwen2.5-7B".
func pedigreeNotes(v pedigreeValue) string {
	ids := strings.Join(v.IDs, ", ")
	if v.Relation == "" {
		return "derived from " + ids + " (relation unknown)"
	}
	return v.Relation + " of " + ids
}
