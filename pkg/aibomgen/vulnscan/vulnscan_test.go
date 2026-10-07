package vulnscan

import (
	"errors"
	"reflect"
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

func refs(bom *cdx.BOM) []string {
	if bom.Vulnerabilities == nil {
		return nil
	}
	var out []string
	for _, v := range *bom.Vulnerabilities {
		out = append(out, v.BOMRef)
	}
	return out
}

func vulns(refs ...string) []cdx.Vulnerability {
	var out []cdx.Vulnerability
	for _, r := range refs {
		out = append(out, cdx.Vulnerability{BOMRef: r})
	}
	return out
}

func TestApplyToDOM(t *testing.T) {
	existing := vulns(
		"CVE-2024-1",            // merged from an SBOM: kept
		"hfsec-model-a-old.bin", // stale finding of a rescanned component: removed
		"hfsec-model-a-same.bin",
		"hfsec-model-b-x.bin", // component whose scan failed: kept
	)
	bom := cdx.NewBOM()
	bom.Vulnerabilities = &existing

	removed := ApplyToDOM(bom, []ComponentScanResult{
		{ComponentRef: "model-a", Vulnerabilities: vulns("hfsec-model-a-same.bin", "hfsec-model-a-new.bin")},
		{ComponentRef: "model-b", Err: errors.New("boom")},
		{ComponentRef: "dataset-c"}, // clean scan, nothing to add
	})

	if removed != 2 {
		t.Errorf("removed = %d, want 2", removed)
	}
	want := []string{"CVE-2024-1", "hfsec-model-b-x.bin", "hfsec-model-a-same.bin", "hfsec-model-a-new.bin"}
	if got := refs(bom); !reflect.DeepEqual(got, want) {
		t.Errorf("vulnerabilities = %v, want %v", got, want)
	}
}

func TestApplyToDOM_AllResolved(t *testing.T) {
	existing := vulns("hfsec-model-a-old.bin")
	bom := cdx.NewBOM()
	bom.Vulnerabilities = &existing

	if removed := ApplyToDOM(bom, []ComponentScanResult{{ComponentRef: "model-a"}}); removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if bom.Vulnerabilities != nil {
		t.Errorf("vulnerabilities = %v, want nil", refs(bom))
	}
}

func TestHighestSeverity(t *testing.T) {
	v := cdx.Vulnerability{Ratings: &[]cdx.VulnerabilityRating{{Severity: cdx.SeverityLow}, {Severity: cdx.SeverityUnknown}, {Severity: cdx.SeverityHigh}}}
	if got := HighestSeverity(v); got != cdx.SeverityHigh {
		t.Errorf("HighestSeverity = %q, want high", got)
	}
	if got := HighestSeverity(cdx.Vulnerability{}); got != cdx.SeverityUnknown {
		t.Errorf("HighestSeverity(unrated) = %q, want unknown", got)
	}
	if SeverityAtLeast(cdx.SeverityUnknown, cdx.SeverityInfo) {
		t.Error("unknown severity must never meet a threshold")
	}
}
