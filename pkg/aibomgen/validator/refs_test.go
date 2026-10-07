package validator

import (
	"strings"
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

func refsBOM(datasetRef, dependsOn string) *cdx.BOM {
	return &cdx.BOM{
		SpecVersion: cdx.SpecVersion1_6,
		Metadata: &cdx.Metadata{Component: &cdx.Component{
			BOMRef: "model",
			Name:   "org/model",
			ModelCard: &cdx.MLModelCard{ModelParameters: &cdx.MLModelParameters{Datasets: &[]cdx.MLDatasetChoice{
				{Ref: datasetRef},
				{ComponentData: &cdx.ComponentData{Type: cdx.ComponentDataTypeDataset, Name: "inline"}},
			}}},
		}},
		Components:   &[]cdx.Component{{Type: cdx.ComponentTypeData, BOMRef: "ds", Name: "org/ds"}},
		Dependencies: &[]cdx.Dependency{{Ref: "model", Dependencies: &[]string{dependsOn}}, {Ref: "ds"}},
	}
}

func TestDanglingRefs(t *testing.T) {
	if got := DanglingRefs(refsBOM("ds", "ds")); len(got) != 0 {
		t.Fatalf("expected no dangling refs, got %v", got)
	}
	got := DanglingRefs(refsBOM("dataset:ds", "nope"))
	if len(got) != 2 {
		t.Fatalf("expected 2 dangling refs, got %v", got)
	}
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, `"dataset:ds"`) || !strings.Contains(joined, `"nope"`) {
		t.Fatalf("unexpected messages: %v", got)
	}
	if DanglingRefs(nil) != nil {
		t.Fatalf("nil BOM must have no dangling refs")
	}
}

func TestValidateWarnsOnDanglingRefs(t *testing.T) {
	res := Validate(refsBOM("dataset:ds", "ds"), ValidationOptions{})
	for _, w := range res.Warnings {
		if strings.HasPrefix(w, "dangling reference:") {
			return
		}
	}
	t.Fatalf("expected a dangling reference warning, got %v", res.Warnings)
}

func TestDanglingRefs_VulnerabilityAffects(t *testing.T) {
	bom := cdx.NewBOM()
	bom.Metadata = &cdx.Metadata{Component: &cdx.Component{BOMRef: "model", Name: "m"}}
	bom.Vulnerabilities = &[]cdx.Vulnerability{{ID: "CVE-1", Affects: &[]cdx.Affects{{Ref: "model"}, {Ref: "gone"}}}}
	got := DanglingRefs(bom)
	if len(got) != 1 || !strings.Contains(got[0], `CVE-1 affects "gone"`) {
		t.Fatalf("DanglingRefs = %v, want one message for the affects ref %q", got, "gone")
	}
}
