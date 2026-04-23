package builder

import cdx "github.com/CycloneDX/cyclonedx-go"

// AddDependencies builds a minimal dependency graph for the BOM where the.
// model (metadata component) depends on all dataset components. The function.
// creates one dependency entry for the model (with a dependsOn list) and a.
// dependency entry for each dataset (with no dependsOn entries) — matching.
// the example structure used elsewhere in the codebase.
func AddDependencies(bom *cdx.BOM) {
	if bom == nil {
		return
	}

	// Determine model BOMRef.
	var modelRef string
	if bom.Metadata != nil && bom.Metadata.Component != nil {
		modelRef = bom.Metadata.Component.BOMRef
	}
	if modelRef == "" {
		return
	}

	// Collect dataset BOMRefs.
	var datasetRefs []string
	if bom.Components != nil {
		for _, comp := range *bom.Components {
			if comp.Type == cdx.ComponentTypeData && comp.BOMRef != "" {
				datasetRefs = append(datasetRefs, comp.BOMRef)
			}
		}
	}

	// Build dependencies slice: model entry (with dependsOn) + dataset entries.
	deps := make([]cdx.Dependency, 0, 1+len(datasetRefs))

	// Model dependency (depends on datasets if present).
	modelDep := cdx.Dependency{Ref: modelRef}
	if len(datasetRefs) > 0 {
		// copy to avoid referencing underlying slice later.
		cp := make([]string, len(datasetRefs))
		copy(cp, datasetRefs)
		modelDep.Dependencies = &cp
	}
	deps = append(deps, modelDep)

	// Add dataset nodes (no further dependencies).
	for _, ds := range datasetRefs {
		deps = append(deps, cdx.Dependency{Ref: ds})
	}

	bom.Dependencies = &deps
}
