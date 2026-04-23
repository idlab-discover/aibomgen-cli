package validator

import (
	"math"
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

// Test Strategy (same as completeness package):.
// - Uses calculated score values instead of hardcoded floats to avoid precision issues.
// - Implements tolerance-based comparison (1e-9) for floating point scores.
// - Best practice: never hardcode floating point literals in test expectations.

const floatTolerance = 1e-9 // Tolerance for floating point comparison

func TestValidate(t *testing.T) {
	type args struct {
		bom  *cdx.BOM
		opts ValidationOptions
	}
	tests := []struct {
		name             string
		args             args
		wantValid        bool
		wantModelID      string
		wantScore        float64
		wantErrorCount   int
		wantErrorContain string
		wantDatasetCount int
	}{
		{
			name: "nil BOM",
			args: args{
				bom:  nil,
				opts: ValidationOptions{},
			},
			wantValid:        false,
			wantModelID:      "",
			wantScore:        0.0,
			wantErrorCount:   1,
			wantErrorContain: "BOM is nil",
			wantDatasetCount: 0,
		},
		{
			name: "BOM without metadata",
			args: args{
				bom:  &cdx.BOM{},
				opts: ValidationOptions{},
			},
			wantValid:        false,
			wantModelID:      "(unknown)",
			wantScore:        0.0,
			wantErrorCount:   2, // missing metadata.component + missing spec version
			wantErrorContain: "BOM missing metadata.component",
			wantDatasetCount: 0,
		},
		{
			name: "valid BOM with spec version",
			args: args{
				bom: &cdx.BOM{
					SpecVersion: cdx.SpecVersion1_5,
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name: "test-model",
						},
					},
				},
				opts: ValidationOptions{},
			},
			wantValid:        true,
			wantModelID:      "test-model",
			wantScore:        1.0 / 12.15,
			wantErrorCount:   0,
			wantDatasetCount: 0,
		},
		{
			name: "BOM with missing spec version",
			args: args{
				bom: &cdx.BOM{
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name: "test-model",
						},
					},
				},
				opts: ValidationOptions{},
			},
			wantValid:        false,
			wantModelID:      "test-model",
			wantScore:        1.0 / 12.15,
			wantErrorCount:   1,
			wantErrorContain: "BOM missing spec version",
			wantDatasetCount: 0,
		},
		{
			name: "BOM with old spec version",
			args: args{
				bom: &cdx.BOM{
					SpecVersion: cdx.SpecVersion1_3,
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name: "test-model",
						},
					},
				},
				opts: ValidationOptions{},
			},
			wantValid:        true,
			wantModelID:      "test-model",
			wantScore:        1.0 / 12.15,
			wantErrorCount:   0,
			wantDatasetCount: 0,
		},
		{
			name: "strict mode with missing required fields",
			args: args{
				bom: &cdx.BOM{
					SpecVersion: cdx.SpecVersion1_5,
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{},
					},
				},
				opts: ValidationOptions{
					StrictMode: true,
				},
			},
			wantValid:        false,
			wantModelID:      "(unknown)",
			wantScore:        0.0,
			wantErrorCount:   1,
			wantErrorContain: "required field missing: BOM.metadata.component.name",
			wantDatasetCount: 0,
		},
		{
			name: "strict mode with low completeness score",
			args: args{
				bom: &cdx.BOM{
					SpecVersion: cdx.SpecVersion1_5,
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name: "test-model",
						},
					},
				},
				opts: ValidationOptions{
					StrictMode:           true,
					MinCompletenessScore: 0.5,
				},
			},
			wantValid:        false,
			wantModelID:      "test-model",
			wantScore:        1.0 / 12.15,
			wantErrorCount:   1,
			wantErrorContain: "completeness score 0.08 below minimum 0.50",
			wantDatasetCount: 0,
		},
		{
			name: "with model card validation - missing card",
			args: args{
				bom: &cdx.BOM{
					SpecVersion: cdx.SpecVersion1_5,
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name: "test-model",
						},
					},
				},
				opts: ValidationOptions{
					CheckModelCard: true,
				},
			},
			wantValid:        true,
			wantModelID:      "test-model",
			wantScore:        1.0 / 12.15,
			wantErrorCount:   0,
			wantDatasetCount: 0,
		},
		{
			name: "with model card present but no parameters",
			args: args{
				bom: &cdx.BOM{
					SpecVersion: cdx.SpecVersion1_5,
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name:      "test-model",
							ModelCard: &cdx.MLModelCard{},
						},
					},
				},
				opts: ValidationOptions{
					CheckModelCard: true,
				},
			},
			wantValid:        true,
			wantModelID:      "test-model",
			wantScore:        1.0 / 12.15,
			wantErrorCount:   0,
			wantDatasetCount: 0,
		},
		{
			name: "with dataset components",
			args: args{
				bom: &cdx.BOM{
					SpecVersion: cdx.SpecVersion1_5,
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name: "test-model",
							ModelCard: &cdx.MLModelCard{
								ModelParameters: &cdx.MLModelParameters{
									Datasets: &[]cdx.MLDatasetChoice{
										{Ref: "ref:dataset-1"},
									},
								},
							},
						},
					},
					Components: &[]cdx.Component{
						{
							Type: cdx.ComponentTypeData,
							Name: "dataset-1",
						},
					},
				},
				opts: ValidationOptions{},
			},
			wantValid:        true,
			wantModelID:      "test-model",
			wantScore:        1.5 / 12.15,
			wantErrorCount:   0,
			wantDatasetCount: 1,
		},
		{
			name: "with dataset strict mode and missing required",
			args: args{
				bom: &cdx.BOM{
					SpecVersion: cdx.SpecVersion1_5,
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name: "test-model",
							ModelCard: &cdx.MLModelCard{
								ModelParameters: &cdx.MLModelParameters{
									Datasets: &[]cdx.MLDatasetChoice{
										{Ref: "ref:dataset-1"},
									},
								},
							},
						},
					},
					Components: &[]cdx.Component{
						{
							Type: cdx.ComponentTypeData,
							Name: "",
						},
					},
				},
				opts: ValidationOptions{
					StrictMode: true,
				},
			},
			wantValid:        true,
			wantModelID:      "test-model",
			wantScore:        1.5 / 12.15,
			wantErrorCount:   0,
			wantDatasetCount: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Validate(tt.args.bom, tt.args.opts)

			if got.Valid != tt.wantValid {
				t.Errorf("Validate() Valid = %v, want %v", got.Valid, tt.wantValid)
			}
			if got.ModelID != tt.wantModelID {
				t.Errorf("Validate() ModelID = %v, want %v", got.ModelID, tt.wantModelID)
			}
			if math.Abs(got.CompletenessScore-tt.wantScore) > floatTolerance {
				t.Errorf("Validate() CompletenessScore = %v, want %v", got.CompletenessScore, tt.wantScore)
			}
			if len(got.Errors) != tt.wantErrorCount {
				t.Errorf("Validate() error count = %v, want %v (errors: %v)", len(got.Errors), tt.wantErrorCount, got.Errors)
			}
			if tt.wantErrorContain != "" && len(got.Errors) > 0 && got.Errors[0] != tt.wantErrorContain {
				t.Errorf("Validate() first error = %v, want to contain %v", got.Errors[0], tt.wantErrorContain)
			}
			if len(got.DatasetResults) != tt.wantDatasetCount {
				t.Errorf("Validate() dataset count = %v, want %v", len(got.DatasetResults), tt.wantDatasetCount)
			}
		})
	}
}

func Test_validateSpecVersion(t *testing.T) {
	type args struct {
		bom    *cdx.BOM
		result *ValidationResult
	}
	tests := []struct {
		name       string
		args       args
		wantValid  bool
		wantErrors int
		wantWarns  int
	}{
		{
			name: "missing spec version",
			args: args{
				bom: &cdx.BOM{
					SpecVersion: 0,
				},
				result: &ValidationResult{
					Valid:    true,
					Errors:   []string{},
					Warnings: []string{},
				},
			},
			wantValid:  false,
			wantErrors: 1,
			wantWarns:  0,
		},
		{
			name: "spec version 1.5",
			args: args{
				bom: &cdx.BOM{
					SpecVersion: cdx.SpecVersion1_5,
				},
				result: &ValidationResult{
					Valid:    true,
					Errors:   []string{},
					Warnings: []string{},
				},
			},
			wantValid:  true,
			wantErrors: 0,
			wantWarns:  0,
		},
		{
			name: "spec version 1.6",
			args: args{
				bom: &cdx.BOM{
					SpecVersion: cdx.SpecVersion1_6,
				},
				result: &ValidationResult{
					Valid:    true,
					Errors:   []string{},
					Warnings: []string{},
				},
			},
			wantValid:  true,
			wantErrors: 0,
			wantWarns:  0,
		},
		{
			name: "old spec version 1.3",
			args: args{
				bom: &cdx.BOM{
					SpecVersion: cdx.SpecVersion1_3,
				},
				result: &ValidationResult{
					Valid:    true,
					Errors:   []string{},
					Warnings: []string{},
				},
			},
			wantValid:  true,
			wantErrors: 0,
			wantWarns:  1,
		},
		{
			name: "spec version 1.0",
			args: args{
				bom: &cdx.BOM{
					SpecVersion: cdx.SpecVersion1_0,
				},
				result: &ValidationResult{
					Valid:    true,
					Errors:   []string{},
					Warnings: []string{},
				},
			},
			wantValid:  true,
			wantErrors: 0,
			wantWarns:  1,
		},
		{
			name: "invalid spec version",
			args: args{
				bom: &cdx.BOM{
					SpecVersion: 99,
				},
				result: &ValidationResult{
					Valid:    true,
					Errors:   []string{},
					Warnings: []string{},
				},
			},
			wantValid:  false,
			wantErrors: 1,
			wantWarns:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validateSpecVersion(tt.args.bom, tt.args.result)

			if tt.args.result.Valid != tt.wantValid {
				t.Errorf("validateSpecVersion() Valid = %v, want %v", tt.args.result.Valid, tt.wantValid)
			}
			if len(tt.args.result.Errors) != tt.wantErrors {
				t.Errorf("validateSpecVersion() Errors count = %v, want %v (errors: %v)",
					len(tt.args.result.Errors), tt.wantErrors, tt.args.result.Errors)
			}
			if len(tt.args.result.Warnings) != tt.wantWarns {
				t.Errorf("validateSpecVersion() Warnings count = %v, want %v (warnings: %v)",
					len(tt.args.result.Warnings), tt.wantWarns, tt.args.result.Warnings)
			}
		})
	}
}

func Test_validateModelCard(t *testing.T) {
	type args struct {
		bom    *cdx.BOM
		result *ValidationResult
	}
	tests := []struct {
		name      string
		args      args
		wantWarns int
	}{
		{
			name: "nil component",
			args: args{
				bom: &cdx.BOM{
					Metadata: &cdx.Metadata{
						Component: nil,
					},
				},
				result: &ValidationResult{
					Warnings: []string{},
				},
			},
			wantWarns: 0,
		},
		{
			name: "no model card",
			args: args{
				bom: &cdx.BOM{
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name: "test",
						},
					},
				},
				result: &ValidationResult{
					Warnings: []string{},
				},
			},
			wantWarns: 1,
		},
		{
			name: "model card without parameters",
			args: args{
				bom: &cdx.BOM{
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name:      "test",
							ModelCard: &cdx.MLModelCard{},
						},
					},
				},
				result: &ValidationResult{
					Warnings: []string{},
				},
			},
			wantWarns: 1,
		},
		{
			name: "model card with parameters",
			args: args{
				bom: &cdx.BOM{
					Metadata: &cdx.Metadata{
						Component: &cdx.Component{
							Name: "test",
							ModelCard: &cdx.MLModelCard{
								ModelParameters: &cdx.MLModelParameters{},
							},
						},
					},
				},
				result: &ValidationResult{
					Warnings: []string{},
				},
			},
			wantWarns: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validateModelCard(tt.args.bom, tt.args.result)

			if len(tt.args.result.Warnings) != tt.wantWarns {
				t.Errorf("validateModelCard() Warnings count = %v, want %v (warnings: %v)",
					len(tt.args.result.Warnings), tt.wantWarns, tt.args.result.Warnings)
			}
		})
	}
}
