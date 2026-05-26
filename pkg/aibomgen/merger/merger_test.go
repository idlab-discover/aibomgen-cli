package merger

import (
	"encoding/json"
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

func TestMergeAIBOMsWithSBOM_NormalizesLegacyToolsForMarshal(t *testing.T) {
	sbom := &cdx.BOM{
		Metadata: &cdx.Metadata{
			Tools: &cdx.ToolsChoice{
				Tools: &[]cdx.Tool{{ //nolint:staticcheck // cdx.Tool deprecated; used to test legacy-tool normalisation
					Vendor:  "syft",
					Name:    "syft",
					Version: "1.0.0",
				}},
			},
		},
	}

	aibom := &cdx.BOM{
		Metadata: &cdx.Metadata{
			Tools: &cdx.ToolsChoice{
				Components: &[]cdx.Component{{
					Type:    cdx.ComponentTypeApplication,
					Name:    "aibomgen-cli",
					Version: "0.1.0",
				}},
			},
		},
	}

	result, err := MergeAIBOMsWithSBOM(sbom, []*cdx.BOM{aibom}, MergeOptions{DeduplicateComponents: true})
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	if result.MergedBOM.Metadata == nil || result.MergedBOM.Metadata.Tools == nil {
		t.Fatal("expected merged BOM metadata tools to be present")
	}
	if result.MergedBOM.Metadata.Tools.Tools != nil {
		t.Fatal("expected merged BOM to not use legacy tools.tools representation")
	}
	if result.MergedBOM.Metadata.Tools.Components == nil || len(*result.MergedBOM.Metadata.Tools.Components) != 2 {
		t.Fatalf("expected two tool components, got %v", result.MergedBOM.Metadata.Tools.Components)
	}

	if _, err := json.Marshal(result.MergedBOM); err != nil {
		t.Fatalf("expected merged BOM to marshal cleanly, got error: %v", err)
	}
}

func TestMergeAIBOMsWithSBOM_DeduplicatesLegacyAndComponentTools(t *testing.T) {
	sbom := &cdx.BOM{
		Metadata: &cdx.Metadata{
			Tools: &cdx.ToolsChoice{
				Tools: &[]cdx.Tool{{ //nolint:staticcheck // cdx.Tool deprecated; used to test legacy-tool normalisation
					Vendor:  "idlab-discover",
					Name:    "aibomgen-cli",
					Version: "0.1.0",
				}},
			},
		},
	}

	aibom := &cdx.BOM{
		Metadata: &cdx.Metadata{
			Tools: &cdx.ToolsChoice{
				Components: &[]cdx.Component{{
					Type:    cdx.ComponentTypeApplication,
					Name:    "aibomgen-cli",
					Version: "0.1.0",
				}},
			},
		},
	}

	result, err := MergeAIBOMsWithSBOM(sbom, []*cdx.BOM{aibom}, MergeOptions{DeduplicateComponents: true})
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	if result.MergedBOM.Metadata == nil || result.MergedBOM.Metadata.Tools == nil || result.MergedBOM.Metadata.Tools.Components == nil {
		t.Fatal("expected merged BOM tool components to be present")
	}

	if got := len(*result.MergedBOM.Metadata.Tools.Components); got != 1 {
		t.Fatalf("expected one deduplicated tool component, got %d", got)
	}

	if _, err := json.Marshal(result.MergedBOM); err != nil {
		t.Fatalf("expected merged BOM to marshal cleanly, got error: %v", err)
	}
}

func TestMergeAIBOMsWithSBOM_PreservesVulnerabilities(t *testing.T) {
	sbom := &cdx.BOM{
		Vulnerabilities: &[]cdx.Vulnerability{
			{ID: "CVE-2024-0001", Description: "SBOM vuln"},
		},
	}

	aibom := &cdx.BOM{
		Metadata: &cdx.Metadata{},
		Vulnerabilities: &[]cdx.Vulnerability{
			{ID: "CVE-2024-0002", Description: "AIBOM vuln"},
			{ID: "CVE-2024-0001", Description: "duplicate — should be deduplicated"},
		},
	}

	result, err := MergeAIBOMsWithSBOM(sbom, []*cdx.BOM{aibom}, MergeOptions{})
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	if result.MergedBOM.Vulnerabilities == nil {
		t.Fatal("expected merged BOM to contain vulnerabilities, got nil")
	}

	if got := len(*result.MergedBOM.Vulnerabilities); got != 2 {
		t.Fatalf("expected 2 vulnerabilities after deduplication, got %d", got)
	}
}

func TestMerge_PreservesVulnerabilities(t *testing.T) {
	primary := &cdx.BOM{
		Vulnerabilities: &[]cdx.Vulnerability{
			{ID: "CVE-2024-0001", Description: "primary vuln"},
		},
	}
	secondary := &cdx.BOM{
		Vulnerabilities: &[]cdx.Vulnerability{
			{ID: "CVE-2024-0002", Description: "secondary vuln"},
		},
	}

	result, err := Merge(primary, secondary, MergeOptions{})
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	if result.MergedBOM.Vulnerabilities == nil {
		t.Fatal("expected merged BOM to contain vulnerabilities, got nil")
	}

	if got := len(*result.MergedBOM.Vulnerabilities); got != 2 {
		t.Fatalf("expected 2 vulnerabilities, got %d", got)
	}
}
