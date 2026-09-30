package builder

import (
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

// DatasetRefKey normalizes a dataset reference as written on a model card
// ("bookcorpus", "dataset:bookcorpus", "Org/DS") into the key used to match it
// against the dataset components built for it.
func DatasetRefKey(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "dataset:")
	return strings.ToLower(strings.TrimSpace(s))
}

// LinkDatasetRefs rewrites modelCard.modelParameters.datasets so that every entry
// either references the bom-ref of an existing data component or is an inline dataset.
//
// resolved maps DatasetRefKey(card dataset) to the bom-ref of the component built
// for it (the component is resolved through Hugging Face, so renamed datasets map to
// the redirected repo). Card entries without a component become inline entries
// ({type: dataset, name}) instead of dangling refs. Components that are not listed on
// the card yet are appended, and duplicate references are removed.
func LinkDatasetRefs(bom *cdx.BOM, resolved map[string]string) {
	if bom == nil || bom.Metadata == nil || bom.Metadata.Component == nil {
		return
	}
	comp := bom.Metadata.Component

	var existing []cdx.MLDatasetChoice
	if mc := comp.ModelCard; mc != nil && mc.ModelParameters != nil && mc.ModelParameters.Datasets != nil {
		existing = *mc.ModelParameters.Datasets
	}
	if len(existing) == 0 && len(resolved) == 0 {
		return
	}

	out := make([]cdx.MLDatasetChoice, 0, len(existing)+len(resolved))
	seenRefs := map[string]struct{}{}
	seenInline := map[string]struct{}{}

	addRef := func(ref string) {
		if _, ok := seenRefs[ref]; ok {
			return
		}
		seenRefs[ref] = struct{}{}
		out = append(out, cdx.MLDatasetChoice{Ref: ref})
	}
	addInline := func(name string) {
		key := strings.ToLower(name)
		if _, ok := seenInline[key]; ok {
			return
		}
		seenInline[key] = struct{}{}
		data := &cdx.ComponentData{Type: cdx.ComponentDataTypeDataset, Name: name}
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
			data.Contents = &cdx.ComponentDataContents{URL: name}
		}
		out = append(out, cdx.MLDatasetChoice{ComponentData: data})
	}

	for _, ds := range existing {
		if ds.Ref == "" {
			if ds.ComponentData != nil {
				out = append(out, ds)
			}
			continue
		}
		if ref, ok := resolved[DatasetRefKey(ds.Ref)]; ok && ref != "" {
			addRef(ref)
			continue
		}
		if name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ds.Ref), "dataset:")); name != "" {
			addInline(name)
		}
	}

	// Reference every built data component, even if the card list did not name it.
	if bom.Components != nil {
		for _, c := range *bom.Components {
			if c.Type != cdx.ComponentTypeData || c.BOMRef == "" {
				continue
			}
			for _, ref := range resolved {
				if ref == c.BOMRef {
					addRef(ref)
					break
				}
			}
		}
	}

	if len(out) == 0 {
		return
	}
	if comp.ModelCard == nil {
		comp.ModelCard = &cdx.MLModelCard{}
	}
	if comp.ModelCard.ModelParameters == nil {
		comp.ModelCard.ModelParameters = &cdx.MLModelParameters{}
	}
	comp.ModelCard.ModelParameters.Datasets = &out
}
