package merger

import (
	"fmt"
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

// MergeOptions configures how BOMs are merged.
type MergeOptions struct {
	// DeduplicateComponents removes duplicate components based on BOM-ref.
	DeduplicateComponents bool
}

// MergeResult contains the merged BOM and metadata about the merge operation.
type MergeResult struct {
	// MergedBOM is the result of merging the BOMs.
	MergedBOM *cdx.BOM
	// SBOMComponentCount is the number of components from the SBOM.
	SBOMComponentCount int
	// AIBOMComponentCount is the number of AI/ML components from the AIBOM(s).
	AIBOMComponentCount int
	// DuplicatesRemoved is the number of duplicate components removed (if deduplication enabled).
	DuplicatesRemoved int

	// Detailed component tracking.
	SBOMComponents    []string // Names of all SBOM components (libraries, files, etc.)
	ModelComponents   []string // Names of ML model components from AIBOMs
	DatasetComponents []string // Names of dataset components from AIBOMs
	MetadataComponent string   // Name of SBOM metadata component (app)
}

// Merge combines two CycloneDX BOMs into a single BOM.
// The primary BOM serves as the base, and components from the secondary BOM are added to it.
// This function handles:.
// - Merging components while avoiding duplicates (based on BOM-ref).
// - Merging dependencies.
// - Combining metadata.
// - Preserving compositions.
func Merge(primary, secondary *cdx.BOM, opts MergeOptions) (*MergeResult, error) {
	if primary == nil {
		return nil, fmt.Errorf("primary BOM is nil")
	}
	if secondary == nil {
		return nil, fmt.Errorf("secondary BOM is nil")
	}

	result := &MergeResult{
		MergedBOM: &cdx.BOM{},
	}

	// Use the primary BOM's spec version.
	result.MergedBOM.SpecVersion = primary.SpecVersion
	if result.MergedBOM.SpecVersion == cdx.SpecVersion(0) {
		result.MergedBOM.SpecVersion = cdx.SpecVersion1_6
	}

	// Merge metadata.
	result.MergedBOM.Metadata = mergeMetadata(primary.Metadata, secondary.Metadata, opts)

	// Collect all components from both BOMs.
	componentsMap := make(map[string]*cdx.Component)
	var mergedComponents []cdx.Component

	// Add primary BOM components.
	if primary.Components != nil {
		for i := range *primary.Components {
			comp := &(*primary.Components)[i]
			bomRef := getBOMRef(comp)
			if bomRef != "" {
				componentsMap[bomRef] = comp
			}
			mergedComponents = append(mergedComponents, *comp)
			result.SBOMComponentCount++
		}
	}

	// Add secondary BOM components (checking for duplicates).
	if secondary.Components != nil {
		for i := range *secondary.Components {
			comp := &(*secondary.Components)[i]
			bomRef := getBOMRef(comp)

			if opts.DeduplicateComponents && bomRef != "" {
				if _, exists := componentsMap[bomRef]; exists {
					result.DuplicatesRemoved++
					continue
				}
				componentsMap[bomRef] = comp
			}

			mergedComponents = append(mergedComponents, *comp)
			result.AIBOMComponentCount++
		}
	}

	// Update the final count after deduplication.
	result.AIBOMComponentCount -= result.DuplicatesRemoved

	if len(mergedComponents) > 0 {
		result.MergedBOM.Components = &mergedComponents
	}

	// Merge dependencies.
	result.MergedBOM.Dependencies = mergeDependencies(primary.Dependencies, secondary.Dependencies)

	// Merge compositions.
	result.MergedBOM.Compositions = mergeCompositions(primary.Compositions, secondary.Compositions)

	// Copy other fields from primary BOM.
	result.MergedBOM.SerialNumber = primary.SerialNumber
	result.MergedBOM.Version = primary.Version

	// Merge services if present.
	if primary.Services != nil || secondary.Services != nil {
		mergedServices := mergeServices(primary.Services, secondary.Services)
		if len(*mergedServices) > 0 {
			result.MergedBOM.Services = mergedServices
		}
	}

	// Merge external references if needed.
	result.MergedBOM.ExternalReferences = mergeExternalReferences(
		primary.ExternalReferences,
		secondary.ExternalReferences,
	)

	return result, nil
}

// MergeAIBOMsWithSBOM combines one or more AIBOMs with an SBOM into a single BOM.
// The SBOM serves as the base, preserving its application metadata component.
// AI/ML components from the AIBOMs are added to the components list.
// This function handles:.
// - Preserving SBOM metadata and application component.
// - Adding AI/ML model and dataset components from AIBOMs.
// - Merging dependencies.
// - Combining tools metadata.
// - Avoiding duplicates (based on BOM-ref).
func MergeAIBOMsWithSBOM(sbom *cdx.BOM, aiboms []*cdx.BOM, opts MergeOptions) (*MergeResult, error) {
	if sbom == nil {
		return nil, fmt.Errorf("SBOM is nil")
	}
	if len(aiboms) == 0 {
		return nil, fmt.Errorf("no AIBOMs provided")
	}

	result := &MergeResult{
		MergedBOM: &cdx.BOM{},
	}

	// Track unique tools by name@version for deduplication.
	toolsMap := make(map[string]bool)

	// Use the SBOM's spec version, format and schema.
	result.MergedBOM.SpecVersion = sbom.SpecVersion
	if result.MergedBOM.SpecVersion == cdx.SpecVersion(0) {
		result.MergedBOM.SpecVersion = cdx.SpecVersion1_6
	}
	if sbom.BOMFormat != "" {
		result.MergedBOM.BOMFormat = sbom.BOMFormat
	} else {
		result.MergedBOM.BOMFormat = "CycloneDX"
	}
	result.MergedBOM.JSONSchema = sbom.JSONSchema

	// Preserve SBOM metadata (including the application component).
	if sbom.Metadata != nil {
		result.MergedBOM.Metadata = &cdx.Metadata{
			Timestamp:   sbom.Metadata.Timestamp,
			Component:   sbom.Metadata.Component, // Keep SBOM's app component as metadata
			Authors:     sbom.Metadata.Authors,
			Manufacture: sbom.Metadata.Manufacture,
			Supplier:    sbom.Metadata.Supplier,
			Licenses:    sbom.Metadata.Licenses,
			Properties:  sbom.Metadata.Properties,
		}

		// Track metadata component name.
		if sbom.Metadata.Component != nil {
			result.MetadataComponent = sbom.Metadata.Component.Name
		}

		// Copy SBOM tools (handle both old tools.tools and new tools.components formats).
		if sbom.Metadata.Tools != nil {
			result.MergedBOM.Metadata.Tools = &cdx.ToolsChoice{}

			// Copy tools.components (newer format used by Syft and AIBoMGen).
			if sbom.Metadata.Tools.Components != nil && len(*sbom.Metadata.Tools.Components) > 0 {
				componentsCopy := make([]cdx.Component, len(*sbom.Metadata.Tools.Components))
				copy(componentsCopy, *sbom.Metadata.Tools.Components)
				result.MergedBOM.Metadata.Tools.Components = &componentsCopy

				// Track SBOM tools in the map for deduplication.
				for i := range *sbom.Metadata.Tools.Components {
					toolKey := getToolKey(&(*sbom.Metadata.Tools.Components)[i])
					if toolKey != "" {
						toolsMap[toolKey] = true
					}
				}
			}

			// Convert legacy tools.tools entries into tools.components.
			if sbom.Metadata.Tools.Tools != nil && len(*sbom.Metadata.Tools.Tools) > 0 {
				if result.MergedBOM.Metadata.Tools.Components == nil {
					result.MergedBOM.Metadata.Tools.Components = &[]cdx.Component{}
				}

				for i := range *sbom.Metadata.Tools.Tools {
					toolComp := legacyToolToComponent(&(*sbom.Metadata.Tools.Tools)[i])
					toolKey := getToolKey(&toolComp)
					if toolKey == "" || !toolsMap[toolKey] {
						*result.MergedBOM.Metadata.Tools.Components = append(*result.MergedBOM.Metadata.Tools.Components, toolComp)
						if toolKey != "" {
							toolsMap[toolKey] = true
						}
					}
				}
			}
		}
	}

	// Collect all components from SBOM.
	componentsMap := make(map[string]*cdx.Component)
	var mergedComponents []cdx.Component

	// Add SBOM components (software libraries, etc.).
	if sbom.Components != nil {
		for i := range *sbom.Components {
			comp := &(*sbom.Components)[i]
			bomRef := getBOMRef(comp)
			if bomRef != "" {
				componentsMap[bomRef] = comp
			}
			mergedComponents = append(mergedComponents, *comp)
			result.SBOMComponentCount++

			// Track all SBOM component names.
			result.SBOMComponents = append(result.SBOMComponents, comp.Name)
		}
	}

	// Add components from all AIBOMs (models and datasets).
	for _, aibom := range aiboms {
		// Add the AIBOM's metadata component (the ML model) to components list.
		if aibom.Metadata != nil && aibom.Metadata.Component != nil {
			comp := aibom.Metadata.Component
			bomRef := getBOMRef(comp)

			shouldAdd := true
			if opts.DeduplicateComponents && bomRef != "" {
				if _, exists := componentsMap[bomRef]; exists {
					result.DuplicatesRemoved++
					shouldAdd = false
				} else {
					componentsMap[bomRef] = comp
				}
			}

			if shouldAdd {
				mergedComponents = append(mergedComponents, *comp)
				result.AIBOMComponentCount++

				// Track ML model component name.
				if comp.Type == cdx.ComponentTypeMachineLearningModel {
					result.ModelComponents = append(result.ModelComponents, comp.Name)
				}
			}
		}

		// Add dataset components from AIBOM's components list.
		if aibom.Components != nil {
			for i := range *aibom.Components {
				comp := &(*aibom.Components)[i]
				bomRef := getBOMRef(comp)

				if opts.DeduplicateComponents && bomRef != "" {
					if _, exists := componentsMap[bomRef]; exists {
						result.DuplicatesRemoved++
						continue
					}
					componentsMap[bomRef] = comp
				}

				mergedComponents = append(mergedComponents, *comp)
				result.AIBOMComponentCount++

				// Track dataset component names.
				if comp.Type == cdx.ComponentTypeData {
					result.DatasetComponents = append(result.DatasetComponents, comp.Name)
				}
			}
		}

		// Merge tools from AIBOM metadata (with deduplication).
		if aibom.Metadata != nil && aibom.Metadata.Tools != nil && aibom.Metadata.Tools.Components != nil {
			if result.MergedBOM.Metadata == nil {
				result.MergedBOM.Metadata = &cdx.Metadata{}
			}
			if result.MergedBOM.Metadata.Tools == nil {
				result.MergedBOM.Metadata.Tools = &cdx.ToolsChoice{}
			}
			if result.MergedBOM.Metadata.Tools.Components == nil {
				result.MergedBOM.Metadata.Tools.Components = &[]cdx.Component{}
			}

			// Add each tool from AIBOM, checking for duplicates.
			for i := range *aibom.Metadata.Tools.Components {
				tool := &(*aibom.Metadata.Tools.Components)[i]
				toolKey := getToolKey(tool)

				// Only add if not already present (deduplicate by name@version).
				if toolKey == "" || !toolsMap[toolKey] {
					*result.MergedBOM.Metadata.Tools.Components = append(*result.MergedBOM.Metadata.Tools.Components, *tool)
					if toolKey != "" {
						toolsMap[toolKey] = true
					}
				}
			}
		}
	}

	if len(mergedComponents) > 0 {
		result.MergedBOM.Components = &mergedComponents
	}

	// Merge dependencies from SBOM and all AIBOMs.
	var allDependencies []*[]cdx.Dependency
	if sbom.Dependencies != nil {
		allDependencies = append(allDependencies, sbom.Dependencies)
	}
	for _, aibom := range aiboms {
		if aibom.Dependencies != nil {
			allDependencies = append(allDependencies, aibom.Dependencies)
		}
	}
	if len(allDependencies) > 0 {
		result.MergedBOM.Dependencies = mergeDependenciesMultiple(allDependencies...)
	}

	// Merge compositions from SBOM and AIBOMs.
	var allCompositions []*[]cdx.Composition
	if sbom.Compositions != nil {
		allCompositions = append(allCompositions, sbom.Compositions)
	}
	for _, aibom := range aiboms {
		if aibom.Compositions != nil {
			allCompositions = append(allCompositions, aibom.Compositions)
		}
	}
	if len(allCompositions) > 0 {
		result.MergedBOM.Compositions = mergeCompositionsMultiple(allCompositions...)
	}

	// Copy other fields from SBOM.
	result.MergedBOM.SerialNumber = sbom.SerialNumber
	result.MergedBOM.Version = sbom.Version

	// Merge services if present.
	var allServices []*[]cdx.Service
	if sbom.Services != nil {
		allServices = append(allServices, sbom.Services)
	}
	for _, aibom := range aiboms {
		if aibom.Services != nil {
			allServices = append(allServices, aibom.Services)
		}
	}
	if len(allServices) > 0 {
		mergedServices := mergeServicesMultiple(allServices...)
		if len(*mergedServices) > 0 {
			result.MergedBOM.Services = mergedServices
		}
	}

	// Merge external references.
	var allExternalRefs []*[]cdx.ExternalReference
	if sbom.ExternalReferences != nil {
		allExternalRefs = append(allExternalRefs, sbom.ExternalReferences)
	}
	for _, aibom := range aiboms {
		if aibom.ExternalReferences != nil {
			allExternalRefs = append(allExternalRefs, aibom.ExternalReferences)
		}
	}
	if len(allExternalRefs) > 0 {
		result.MergedBOM.ExternalReferences = mergeExternalReferencesMultiple(allExternalRefs...)
	}

	return result, nil
}

// mergeMetadata combines metadata from both BOMs.
func mergeMetadata(primary, secondary *cdx.Metadata, opts MergeOptions) *cdx.Metadata {
	if primary == nil && secondary == nil {
		return nil
	}

	merged := &cdx.Metadata{}

	// Prefer primary metadata as base.
	if primary != nil {
		merged.Timestamp = primary.Timestamp
		merged.Authors = primary.Authors
		merged.Component = primary.Component
		merged.Manufacture = primary.Manufacture
		merged.Supplier = primary.Supplier
		merged.Licenses = primary.Licenses
		merged.Properties = primary.Properties

		// Deep copy tools from primary.
		if primary.Tools != nil && primary.Tools.Tools != nil && len(*primary.Tools.Tools) > 0 {
			toolsCopy := make([]cdx.Tool, len(*primary.Tools.Tools)) //nolint:staticcheck // cdx.Tool is deprecated; used here intentionally to handle legacy BOM inputs
			copy(toolsCopy, *primary.Tools.Tools)
			merged.Tools = &cdx.ToolsChoice{
				Tools: &toolsCopy,
			}
		}
	}

	// Merge tools from secondary.
	if secondary != nil {
		// If primary didn't have tools, use secondary's.
		if merged.Tools == nil && secondary.Tools != nil && secondary.Tools.Tools != nil && len(*secondary.Tools.Tools) > 0 {
			toolsCopy := make([]cdx.Tool, len(*secondary.Tools.Tools)) //nolint:staticcheck // cdx.Tool is deprecated; used here intentionally to handle legacy BOM inputs
			copy(toolsCopy, *secondary.Tools.Tools)
			merged.Tools = &cdx.ToolsChoice{
				Tools: &toolsCopy,
			}
		} else if merged.Tools != nil && secondary.Tools != nil && secondary.Tools.Tools != nil && len(*secondary.Tools.Tools) > 0 {
			// Combine tools from both.
			combinedTools := append(*merged.Tools.Tools, *secondary.Tools.Tools...)
			merged.Tools = &cdx.ToolsChoice{
				Tools: &combinedTools,
			}
		}

		// If primary didn't have a timestamp but secondary does, use secondary's.
		if merged.Timestamp == "" && secondary.Timestamp != "" {
			merged.Timestamp = secondary.Timestamp
		}
	}

	return merged
}

// mergeDependencies combines dependencies from both BOMs.
func mergeDependencies(primary, secondary *[]cdx.Dependency) *[]cdx.Dependency {
	if primary == nil && secondary == nil {
		return nil
	}

	depMap := make(map[string]*cdx.Dependency)

	// Add primary dependencies.
	if primary != nil {
		for i := range *primary {
			dep := &(*primary)[i]
			depMap[dep.Ref] = dep
		}
	}

	// Merge secondary dependencies.
	if secondary != nil {
		for i := range *secondary {
			dep := &(*secondary)[i]
			if existing, exists := depMap[dep.Ref]; exists {
				// Merge dependency lists for the same ref.
				if dep.Dependencies != nil {
					if existing.Dependencies == nil {
						existing.Dependencies = dep.Dependencies
					} else {
						// Combine dependencies, removing duplicates.
						combined := mergeDependencyRefs(*existing.Dependencies, *dep.Dependencies)
						existing.Dependencies = &combined
					}
				}
			} else {
				depMap[dep.Ref] = dep
			}
		}
	}

	// Convert map back to slice.
	var merged []cdx.Dependency
	for _, dep := range depMap {
		merged = append(merged, *dep)
	}

	if len(merged) == 0 {
		return nil
	}

	return &merged
}

// mergeDependencyRefs combines two dependency ref lists, removing duplicates.
func mergeDependencyRefs(refs1, refs2 []string) []string {
	refMap := make(map[string]bool)
	var merged []string

	for _, ref := range refs1 {
		if !refMap[ref] {
			refMap[ref] = true
			merged = append(merged, ref)
		}
	}

	for _, ref := range refs2 {
		if !refMap[ref] {
			refMap[ref] = true
			merged = append(merged, ref)
		}
	}

	return merged
}

// mergeCompositions combines compositions from both BOMs.
func mergeCompositions(primary, secondary *[]cdx.Composition) *[]cdx.Composition {
	if primary == nil && secondary == nil {
		return nil
	}

	var merged []cdx.Composition

	if primary != nil {
		merged = append(merged, *primary...)
	}

	if secondary != nil {
		merged = append(merged, *secondary...)
	}

	if len(merged) == 0 {
		return nil
	}

	return &merged
}

// mergeServices combines services from both BOMs.
func mergeServices(primary, secondary *[]cdx.Service) *[]cdx.Service {
	if primary == nil && secondary == nil {
		return nil
	}

	serviceMap := make(map[string]*cdx.Service)
	var merged []cdx.Service

	// Add primary services.
	if primary != nil {
		for i := range *primary {
			svc := &(*primary)[i]
			bomRef := getServiceBOMRef(svc)
			if bomRef != "" {
				serviceMap[bomRef] = svc
			}
			merged = append(merged, *svc)
		}
	}

	// Add secondary services (checking for duplicates).
	if secondary != nil {
		for i := range *secondary {
			svc := &(*secondary)[i]
			bomRef := getServiceBOMRef(svc)
			if bomRef != "" {
				if _, exists := serviceMap[bomRef]; exists {
					continue // Skip duplicate
				}
				serviceMap[bomRef] = svc
			}
			merged = append(merged, *svc)
		}
	}

	if len(merged) == 0 {
		return nil
	}

	return &merged
}

// mergeExternalReferences combines external references from both BOMs.
func mergeExternalReferences(primary, secondary *[]cdx.ExternalReference) *[]cdx.ExternalReference {
	if primary == nil && secondary == nil {
		return nil
	}

	var merged []cdx.ExternalReference

	if primary != nil {
		merged = append(merged, *primary...)
	}

	if secondary != nil {
		merged = append(merged, *secondary...)
	}

	if len(merged) == 0 {
		return nil
	}

	return &merged
}

// getBOMRef returns the BOM-ref of a component.
func getBOMRef(comp *cdx.Component) string {
	if comp.BOMRef == "" {
		// Try to generate a ref from component identity.
		return generateBOMRef(comp)
	}
	return comp.BOMRef
}

// getToolKey generates a unique key for a tool component based on name and version.
// This is used for deduplicating tools (e.g., aibomgen-cli appears only once per version).
func getToolKey(comp *cdx.Component) string {
	if comp == nil {
		return ""
	}
	return comp.Name + "@" + comp.Version
}

// legacyToolToComponent maps deprecated metadata.tools.tools entries to.
// metadata.tools.components so merged output only uses one ToolsChoice shape.
func legacyToolToComponent(tool *cdx.Tool) cdx.Component { //nolint:staticcheck // cdx.Tool is deprecated; used here intentionally for legacy-tool normalization
	comp := cdx.Component{
		Type:    cdx.ComponentTypeApplication,
		Name:    tool.Name,
		Version: tool.Version,
	}

	if tool.Vendor != "" {
		comp.Manufacturer = &cdx.OrganizationalEntity{Name: tool.Vendor}
	}
	if tool.Hashes != nil {
		hashesCopy := make([]cdx.Hash, len(*tool.Hashes))
		copy(hashesCopy, *tool.Hashes)
		comp.Hashes = &hashesCopy
	}
	if tool.ExternalReferences != nil {
		externalRefsCopy := make([]cdx.ExternalReference, len(*tool.ExternalReferences))
		copy(externalRefsCopy, *tool.ExternalReferences)
		comp.ExternalReferences = &externalRefsCopy
	}

	return comp
}

// generateBOMRef creates a BOM-ref from component identity.
func generateBOMRef(comp *cdx.Component) string {
	parts := []string{}

	if comp.Type != "" {
		parts = append(parts, string(comp.Type))
	}

	if comp.Name != "" {
		parts = append(parts, comp.Name)
	}

	if comp.Version != "" {
		parts = append(parts, comp.Version)
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "/")
}

// getServiceBOMRef returns the BOM-ref of a service.
func getServiceBOMRef(svc *cdx.Service) string {
	if svc.BOMRef == "" {
		return ""
	}
	return svc.BOMRef
}

// mergeDependenciesMultiple combines dependencies from multiple BOMs.
func mergeDependenciesMultiple(deps ...*[]cdx.Dependency) *[]cdx.Dependency {
	if len(deps) == 0 {
		return nil
	}

	depMap := make(map[string]*cdx.Dependency)

	for _, depList := range deps {
		if depList == nil {
			continue
		}
		for i := range *depList {
			dep := &(*depList)[i]
			if existing, exists := depMap[dep.Ref]; exists {
				// Merge dependency lists for the same ref.
				if dep.Dependencies != nil {
					if existing.Dependencies == nil {
						existing.Dependencies = dep.Dependencies
					} else {
						// Combine dependencies, removing duplicates.
						combined := mergeDependencyRefs(*existing.Dependencies, *dep.Dependencies)
						existing.Dependencies = &combined
					}
				}
			} else {
				depMap[dep.Ref] = dep
			}
		}
	}

	if len(depMap) == 0 {
		return nil
	}

	var merged []cdx.Dependency
	for _, dep := range depMap {
		merged = append(merged, *dep)
	}

	return &merged
}

// mergeCompositionsMultiple combines compositions from multiple BOMs.
func mergeCompositionsMultiple(comps ...*[]cdx.Composition) *[]cdx.Composition {
	if len(comps) == 0 {
		return nil
	}

	var merged []cdx.Composition

	for _, compList := range comps {
		if compList != nil {
			merged = append(merged, *compList...)
		}
	}

	if len(merged) == 0 {
		return nil
	}

	return &merged
}

// mergeServicesMultiple combines services from multiple BOMs.
func mergeServicesMultiple(services ...*[]cdx.Service) *[]cdx.Service {
	if len(services) == 0 {
		return nil
	}

	serviceMap := make(map[string]*cdx.Service)
	var merged []cdx.Service

	for _, svcList := range services {
		if svcList == nil {
			continue
		}
		for i := range *svcList {
			svc := &(*svcList)[i]
			bomRef := getServiceBOMRef(svc)
			if bomRef != "" {
				if _, exists := serviceMap[bomRef]; exists {
					continue // Skip duplicate
				}
				serviceMap[bomRef] = svc
			}
			merged = append(merged, *svc)
		}
	}

	if len(merged) == 0 {
		return nil
	}

	return &merged
}

// mergeExternalReferencesMultiple combines external references from multiple BOMs.
func mergeExternalReferencesMultiple(refs ...*[]cdx.ExternalReference) *[]cdx.ExternalReference {
	if len(refs) == 0 {
		return nil
	}

	var merged []cdx.ExternalReference

	for _, refList := range refs {
		if refList != nil {
			merged = append(merged, *refList...)
		}
	}

	if len(merged) == 0 {
		return nil
	}

	return &merged
}
