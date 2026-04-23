package metadata

import cdx "github.com/CycloneDX/cyclonedx-go"

// Helper functions for working with Component.Data slice.
func ensureComponentData(comp *cdx.Component) *cdx.ComponentData {
	if comp.Data == nil {
		comp.Data = &[]cdx.ComponentData{{
			Type: cdx.ComponentDataTypeDataset,
		}}
	} else if len(*comp.Data) == 0 {
		*comp.Data = []cdx.ComponentData{{
			Type: cdx.ComponentDataTypeDataset,
		}}
	}
	// Ensure Type is always set.
	data := &(*comp.Data)[0]
	if data.Type == "" {
		data.Type = cdx.ComponentDataTypeDataset
	}
	return data
}

func getComponentData(comp *cdx.Component) *cdx.ComponentData {
	if comp.Data == nil || len(*comp.Data) == 0 {
		return nil
	}
	return &(*comp.Data)[0]
}
