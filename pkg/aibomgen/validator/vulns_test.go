package validator

import (
	"strings"
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

func bomWithVuln(sev cdx.Severity) *cdx.BOM {
	bom := cdx.NewBOM()
	bom.Metadata = &cdx.Metadata{Component: &cdx.Component{BOMRef: "pkg:huggingface/org/m", Type: cdx.ComponentTypeMachineLearningModel, Name: "org/m"}}
	v := cdx.Vulnerability{BOMRef: "hfsec-pkg:huggingface/org/m-model.bin", Affects: &[]cdx.Affects{{Ref: "pkg:huggingface/org/m"}}}
	if sev != "" {
		v.Ratings = &[]cdx.VulnerabilityRating{{Severity: sev}}
	}
	bom.Vulnerabilities = &[]cdx.Vulnerability{v}
	return bom
}

func TestValidate_Vulnerabilities(t *testing.T) {
	tests := []struct {
		name      string
		sev       cdx.Severity
		opts      ValidationOptions
		wantValid bool
		wantError bool
	}{
		{"strict, high >= default medium", cdx.SeverityHigh, ValidationOptions{StrictMode: true}, false, true},
		{"strict, medium == default medium", cdx.SeverityMedium, ValidationOptions{StrictMode: true}, false, true},
		{"strict, low below default", cdx.SeverityLow, ValidationOptions{StrictMode: true}, true, false},
		{"strict, unknown severity never fails", "", ValidationOptions{StrictMode: true}, true, false},
		{"strict, high below critical threshold", cdx.SeverityHigh, ValidationOptions{StrictMode: true, FailSeverity: cdx.SeverityCritical}, true, false},
		{"strict, low at low threshold", cdx.SeverityLow, ValidationOptions{StrictMode: true, FailSeverity: cdx.SeverityLow}, false, true},
		{"non-strict, critical only warns", cdx.SeverityCritical, ValidationOptions{}, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Validate(bomWithVuln(tt.sev), tt.opts)
			if got.Valid != tt.wantValid {
				t.Fatalf("Valid = %v, want %v (errors: %v)", got.Valid, tt.wantValid, got.Errors)
			}
			inErrors := containsVuln(got.Errors)
			if inErrors != tt.wantError {
				t.Fatalf("vulnerability in errors = %v, want %v", inErrors, tt.wantError)
			}
			if !inErrors && !containsVuln(got.Warnings) {
				t.Fatalf("vulnerability must be reported as a warning when it is not an error: %v", got.Warnings)
			}
		})
	}
}

func containsVuln(msgs []string) bool {
	for _, m := range msgs {
		if strings.Contains(m, "vulnerability hfsec-pkg:huggingface/org/m-model.bin") && strings.Contains(m, "affects pkg:huggingface/org/m") {
			return true
		}
	}
	return false
}
