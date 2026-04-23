package builder

import (
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

func TestAddDependencies(t *testing.T) {
	type args struct {
		bom *cdx.BOM
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "adds expected dependencies",
			args: args{bom: func() *cdx.BOM {
				b := cdx.NewBOM()
				modelComp := &cdx.Component{
					BOMRef: "model-ref",
					Type:   cdx.ComponentTypeApplication,
					Name:   "Test Model",
				}
				datasetComp1 := &cdx.Component{
					BOMRef: "dataset-ref-1",
					Type:   cdx.ComponentTypeData,
					Name:   "Dataset 1",
				}
				datasetComp2 := &cdx.Component{
					BOMRef: "dataset-ref-2",
					Type:   cdx.ComponentTypeData,
					Name:   "Dataset 2",
				}
				b.Metadata = &cdx.Metadata{Component: modelComp}
				components := []cdx.Component{*datasetComp1, *datasetComp2}
				b.Components = &components
				return b
			}()},
		},
		{name: "nil bom", args: args{bom: nil}},
		{name: "missing modelRef", args: args{bom: func() *cdx.BOM {
			b := cdx.NewBOM()
			m := &cdx.Component{Type: cdx.ComponentTypeApplication, Name: "M"}
			b.Metadata = &cdx.Metadata{Component: m}
			return b
		}()}},
		{name: "dataset with empty BOMRef ignored", args: args{bom: func() *cdx.BOM {
			b := cdx.NewBOM()
			m := &cdx.Component{BOMRef: "model-ref", Type: cdx.ComponentTypeApplication}
			b.Metadata = &cdx.Metadata{Component: m}
			comps := []cdx.Component{{BOMRef: "", Type: cdx.ComponentTypeData}, {BOMRef: "other", Type: cdx.ComponentTypeApplication}}
			b.Components = &comps
			return b
		}()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddDependencies(tt.args.bom)

			// Handle nil input case.
			if tt.args.bom == nil {
				// function should not panic and simply return.
				return
			}

			// Special-case: missing modelRef should leave Dependencies nil.
			if tt.name == "missing modelRef" {
				if tt.args.bom.Dependencies != nil {
					t.Fatalf("expected Dependencies to remain nil when modelRef missing")
				}
				return
			}

			// dataset with empty BOMRef should result in only the model dependency.
			if tt.name == "dataset with empty BOMRef ignored" {
				if tt.args.bom.Dependencies == nil {
					t.Fatalf("bom.Dependencies is nil, want non-nil")
				}
				deps := *tt.args.bom.Dependencies
				if len(deps) != 1 {
					t.Fatalf("len(bom.Dependencies) = %d, want 1", len(deps))
				}
				if deps[0].Ref != "model-ref" {
					t.Fatalf("expected single model dependency, got %v", deps[0].Ref)
				}
				if deps[0].Dependencies != nil {
					t.Fatalf("expected model to have no dependsOn when no datasets present")
				}
				return
			}

			// Default (original) assertions for the first test case.

			deps := *tt.args.bom.Dependencies
			if len(deps) != 3 {
				t.Fatalf("len(bom.Dependencies) = %d, want 3", len(deps))
			}

			// Verify model dependency.
			var modelDep *cdx.Dependency
			for i := range deps {
				if deps[i].Ref == "model-ref" {
					modelDep = &deps[i]
					break
				}
			}
			if modelDep == nil {
				t.Fatalf("model dependency not found in bom.Dependencies")
			}
			if modelDep.Dependencies == nil || len(*modelDep.Dependencies) != 2 {
				t.Fatalf("model dependency dependsOn = %v, want 2 entries", modelDep.Dependencies)
			}
			expectedDeps := map[string]bool{
				"dataset-ref-1": true,
				"dataset-ref-2": true,
			}
			for _, depRef := range *modelDep.Dependencies {
				if !expectedDeps[depRef] {
					t.Fatalf("unexpected dependency %q in model dependsOn", depRef)
				}
			}

			// Verify dataset dependencies.
			for _, dsRef := range []string{"dataset-ref-1", "dataset-ref-2"} {
				var dsDep *cdx.Dependency
				for i := range deps {
					if deps[i].Ref == dsRef {
						dsDep = &deps[i]
						break
					}
				}
				if dsDep == nil {
					t.Fatalf("dataset dependency %q not found in bom.Dependencies", dsRef)
				}
				if dsDep.Dependencies != nil {
					t.Fatalf("dataset dependency %q has non-nil dependsOn, want nil", dsRef)
				}
			}
		})
	}
}
