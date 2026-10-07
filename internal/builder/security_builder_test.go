package builder

import (
	"testing"

	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

func TestInjectSecurityData(t *testing.T) {
	file := func(path, status string, protectAi string) fetcher.SecurityFileEntry {
		return fetcher.SecurityFileEntry{Path: path, SecurityFileStatus: &fetcher.SecurityFileStatus{
			Status:        status,
			ProtectAiScan: fetcher.ScannerResult{Status: protectAi},
		}}
	}
	tests := []struct {
		name    string
		entries []fetcher.SecurityFileEntry
		want    []cdx.Severity // one top rating per expected vulnerability
	}{
		{"suspicious only", []fetcher.SecurityFileEntry{file("a.bin", "suspicious", "suspicious")}, []cdx.Severity{cdx.SeverityHigh}},
		{"safe, unscanned and queued", []fetcher.SecurityFileEntry{file("a.bin", "safe", "safe"), file("b.bin", "unscanned", ""), file("q.bin", "queued", "queued"), {Path: "c"}}, nil},
		{"unsafe without scanner detail", []fetcher.SecurityFileEntry{file("a.bin", "unsafe", "")}, []cdx.Severity{cdx.SeverityCritical}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bom := cdx.NewBOM()
			InjectSecurityData(bom, &cdx.Component{BOMRef: "m"}, tt.entries, "org/m", "")
			if tt.want == nil {
				if bom.Vulnerabilities != nil {
					t.Fatalf("want no vulnerabilities, got %d", len(*bom.Vulnerabilities))
				}
				return
			}
			if bom.Vulnerabilities == nil || len(*bom.Vulnerabilities) != len(tt.want) {
				t.Fatalf("want %d vulnerabilities, got %v", len(tt.want), bom.Vulnerabilities)
			}
			for i, v := range *bom.Vulnerabilities {
				if v.Ratings == nil || (*v.Ratings)[0].Severity != tt.want[i] {
					t.Errorf("vuln %d: want severity %s, got %v", i, tt.want[i], v.Ratings)
				}
			}
		})
	}
}
