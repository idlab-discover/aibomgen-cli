package builder

import (
	"reflect"
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/scanner"
)

func TestNewBOMBuilder(t *testing.T) {
	type args struct {
		opts Options
	}
	tests := []struct {
		name string
		args args
		want *BOMBuilder
	}{
		{name: "returns builder with opts", args: args{opts: Options{IncludeEvidenceProperties: false, HuggingFaceBaseURL: "https://example/"}}, want: &BOMBuilder{Opts: Options{IncludeEvidenceProperties: false, HuggingFaceBaseURL: "https://example/"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewBOMBuilder(tt.args.opts); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewBOMBuilder() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBOMBuilder_Build(t *testing.T) {
	type fields struct {
		Opts Options
	}
	type args struct {
		ctx BuildContext
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{name: "builds bom with metadata component", fields: fields{Opts: DefaultOptions()}, args: args{ctx: BuildContext{ModelID: "mymodel"}}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := BOMBuilder{
				Opts: tt.fields.Opts,
			}
			got, err := b.Build(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("BOMBuilder.Build() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got == nil {
				t.Fatalf("expected non-nil BOM")
			}
			if got.Metadata == nil || got.Metadata.Component == nil {
				t.Fatalf("expected metadata component to be present")
			}
			if got.Metadata.Component.Type != cdx.ComponentTypeMachineLearningModel {
				t.Errorf("expected component type MachineLearningModel, got %v", got.Metadata.Component.Type)
			}
			if got.Metadata.Component.Name != "mymodel" {
				t.Errorf("expected component name mymodel, got %s", got.Metadata.Component.Name)
			}
			// Serial and timestamp should be set.
			if got.SerialNumber == "" {
				t.Errorf("expected SerialNumber to be set")
			}
			if got.Metadata.Timestamp == "" {
				t.Errorf("expected Metadata.Timestamp to be set")
			}
			// BOMRef or PackageURL should be set on component.
			if got.Metadata.Component.PackageURL == "" && got.Metadata.Component.BOMRef == "" {
				t.Errorf("expected PackageURL or BOMRef to be set on component")
			}
		})
	}
}

func TestBOMBuilder_BuildDataset(t *testing.T) {
	type fields struct {
		Opts Options
	}
	type args struct {
		ctx DatasetBuildContext
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{name: "builds dataset component", fields: fields{Opts: DefaultOptions()}, args: args{ctx: DatasetBuildContext{DatasetID: "mydataset"}}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := BOMBuilder{
				Opts: tt.fields.Opts,
			}
			got, err := b.BuildDataset(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("BOMBuilder.BuildDataset() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got == nil {
				t.Fatalf("expected non-nil component")
			}
			if got.Type != cdx.ComponentTypeData {
				t.Errorf("expected component type Data, got %v", got.Type)
			}
			if got.Name != "mydataset" {
				t.Errorf("expected component name mydataset, got %s", got.Name)
			}
			if got.PackageURL == "" && got.BOMRef == "" {
				t.Errorf("expected PackageURL or BOMRef to be set on dataset component")
			}
		})
	}
}

func Test_buildMetadataComponent(t *testing.T) {
	type args struct {
		ctx BuildContext
	}
	tests := []struct {
		name string
		args args
		want *cdx.Component
	}{
		{name: "uses modelID when present", args: args{ctx: BuildContext{ModelID: "mid"}}, want: &cdx.Component{Type: cdx.ComponentTypeMachineLearningModel, Name: "mid", ModelCard: &cdx.MLModelCard{}}},
		{name: "uses scan name when modelID empty", args: args{ctx: BuildContext{Scan: scanner.Discovery{Name: "scanname"}}}, want: &cdx.Component{Type: cdx.ComponentTypeMachineLearningModel, Name: "scanname", ModelCard: &cdx.MLModelCard{}}},
		{name: "defaults to model when nothing set", args: args{ctx: BuildContext{}}, want: &cdx.Component{Type: cdx.ComponentTypeMachineLearningModel, Name: "model", ModelCard: &cdx.MLModelCard{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildMetadataComponent(tt.args.ctx); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildMetadataComponent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_buildDatasetComponent(t *testing.T) {
	type args struct {
		ctx DatasetBuildContext
	}
	tests := []struct {
		name string
		args args
		want *cdx.Component
	}{
		{name: "uses datasetID when present", args: args{ctx: DatasetBuildContext{DatasetID: "did"}}, want: &cdx.Component{Type: cdx.ComponentTypeData, Name: "did"}},
		{name: "uses scan name when datasetID empty", args: args{ctx: DatasetBuildContext{Scan: scanner.Discovery{Name: "scanname"}}}, want: &cdx.Component{Type: cdx.ComponentTypeData, Name: "scanname"}},
		{name: "defaults to dataset when nothing set", args: args{ctx: DatasetBuildContext{}}, want: &cdx.Component{Type: cdx.ComponentTypeData, Name: "dataset"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildDatasetComponent(tt.args.ctx); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildDatasetComponent() = %v, want %v", got, tt.want)
			}
		})
	}
}
