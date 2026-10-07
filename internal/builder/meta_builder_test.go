package builder

import (
	"strings"
	"testing"
	"time"

	"github.com/CycloneDX/cyclonedx-go"
)

func TestAddMetaSerialNumber(t *testing.T) {
	type args struct {
		bom *cyclonedx.BOM
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "sets serial when empty",
			args: args{bom: &cyclonedx.BOM{}},
		},
		{
			name: "preserves existing serial",
			args: args{bom: &cyclonedx.BOM{SerialNumber: "urn:uuid:existing"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddMetaSerialNumber(tt.args.bom)
			// Additional checks.
			if tt.args.bom.SerialNumber == "" {
				t.Errorf("SerialNumber should be set")
			}
		})
	}
}

func TestAddMetaSerialNumber_Unique(t *testing.T) {
	b1, b2 := &cyclonedx.BOM{}, &cyclonedx.BOM{}
	AddMetaSerialNumber(b1)
	AddMetaSerialNumber(b2)
	if b1.SerialNumber == b2.SerialNumber {
		t.Errorf("AddMetaSerialNumber should set unique serials but got duplicates: %s", b1.SerialNumber)
	}
}

func TestAddMetaTimestamp(t *testing.T) {
	type args struct {
		bom *cyclonedx.BOM
	}
	tests := []struct {
		name string
		args args
	}{
		{name: "sets timestamp when empty", args: args{bom: &cyclonedx.BOM{Metadata: &cyclonedx.Metadata{}}}},
		{name: "preserves existing timestamp", args: args{bom: &cyclonedx.BOM{Metadata: &cyclonedx.Metadata{Timestamp: "2020-01-01T00:00:00Z"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddMetaTimestamp(tt.args.bom)
			if tt.name == "sets timestamp when empty" {
				if _, err := time.Parse(time.RFC3339, tt.args.bom.Metadata.Timestamp); err != nil {
					t.Errorf("Timestamp should be set as RFC3339: %v", err)
				}
			}
			if tt.name == "preserves existing timestamp" {
				if tt.args.bom.Metadata.Timestamp != "2020-01-01T00:00:00Z" {
					t.Errorf("Timestamp should be preserved, got %s", tt.args.bom.Metadata.Timestamp)
				}
			}
		})
	}
}

func TestAddMetaTools(t *testing.T) {
	type args struct {
		bom         *cyclonedx.BOM
		toolName    string
		toolVersion string
	}
	tests := []struct {
		name string
		args args
	}{
		{name: "adds tool with provided name and version", args: args{bom: &cyclonedx.BOM{}, toolName: "mytool", toolVersion: "v1"}},
		{name: "adds tool with defaults when empty", args: args{bom: &cyclonedx.BOM{}, toolName: "", toolVersion: ""}},
		{name: "appends to existing tools", args: args{bom: func() *cyclonedx.BOM {
			b := &cyclonedx.BOM{}
			b.Metadata = &cyclonedx.Metadata{Tools: &cyclonedx.ToolsChoice{Components: &[]cyclonedx.Component{{Name: "existing"}}}}
			return b
		}(), toolName: "x", toolVersion: "v1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddMetaTools(tt.args.bom, tt.args.toolName, tt.args.toolVersion)
			if tt.args.bom.Metadata == nil || tt.args.bom.Metadata.Tools == nil || tt.args.bom.Metadata.Tools.Components == nil {
				t.Fatalf("expected tools component to be set")
			}
			comps := *tt.args.bom.Metadata.Tools.Components
			if len(comps) == 0 {
				t.Fatalf("expected at least one tool component")
			}
			last := comps[len(comps)-1]
			if tt.args.toolName != "" {
				if last.Name != tt.args.toolName {
					t.Errorf("expected tool name %s, got %s", tt.args.toolName, last.Name)
				}
				if last.Version != tt.args.toolVersion {
					t.Errorf("expected tool version %s, got %s", tt.args.toolVersion, last.Version)
				}
			} else {
				if last.Name != DefaultToolName {
					t.Errorf("expected default tool name %s, got %s", DefaultToolName, last.Name)
				}
				if last.Version != DefaultToolVersion {
					t.Errorf("expected default tool version %s, got %s", DefaultToolVersion, last.Version)
				}
			}
		})
	}
}

func TestGeneratePurl(t *testing.T) {
	type args struct {
		kind    string
		id      string
		version string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "model with version", args: args{kind: "model", id: "owner/name", version: "V1"}, want: "pkg:huggingface/owner/name@v1"},
		{name: "dataset without version", args: args{kind: "dataset", id: "owner/data", version: ""}, want: "pkg:huggingface/datasets/owner/data"},
		{name: "unknown kind", args: args{kind: "weird", id: "id", version: "1"}, want: "pkg:huggingface/unknown/id@1"},
		{name: "empty id", args: args{kind: "model", id: "", version: ""}, want: "pkg:huggingface/unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GeneratePurl(tt.args.kind, tt.args.id, tt.args.version); got != tt.want {
				t.Errorf("GeneratePurl() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeSegment(t *testing.T) {
	type args struct {
		segment string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "escape at", args: args{segment: "a@b"}, want: "a%40b"},
		{name: "escape space", args: args{segment: "a b"}, want: "a%20b"},
		{name: "no change", args: args{segment: "abc"}, want: "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeSegment(tt.args.segment); got != tt.want {
				t.Errorf("NormalizeSegment() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddComponentPurl(t *testing.T) {
	type args struct {
		c *cyclonedx.Component
	}
	tests := []struct {
		name string
		args args
	}{
		{name: "model sets purl from name and hash", args: args{c: &cyclonedx.Component{Type: cyclonedx.ComponentTypeMachineLearningModel, Name: "owner/repo", Hashes: &[]cyclonedx.Hash{{Value: "ABC"}}}}},
		{name: "dataset sets purl with datasets path", args: args{c: &cyclonedx.Component{Type: cyclonedx.ComponentTypeData, Name: "owner/ds", Hashes: &[]cyclonedx.Hash{{Value: "def"}}}}},
		{name: "noop when purl already set", args: args{c: &cyclonedx.Component{PackageURL: "pkg:already/set"}}},
		{name: "nil component", args: args{c: nil}},
		{name: "empty name becomes unknown", args: args{c: &cyclonedx.Component{Type: cyclonedx.ComponentTypeMachineLearningModel}}},
		{name: "uses property and tag lookups", args: args{c: &cyclonedx.Component{Type: cyclonedx.ComponentTypeMachineLearningModel, Name: "a b@c", Properties: &[]cyclonedx.Property{{Name: "huggingface:lastModified", Value: "2020-01-01"}}, Tags: &[]string{"lastModified:2020-02-02"}, Hashes: &[]cyclonedx.Hash{{Value: "F00"}}}}},
		{name: "unknown type produces unknown kind", args: args{c: &cyclonedx.Component{Name: "owner/repo"}}},
		{name: "hash empty omits version", args: args{c: &cyclonedx.Component{Type: cyclonedx.ComponentTypeMachineLearningModel, Name: "owner/name", Hashes: &[]cyclonedx.Hash{{Value: ""}}}}},
		{name: "uses tag lookup for lastModified", args: args{c: &cyclonedx.Component{Type: cyclonedx.ComponentTypeMachineLearningModel, Name: "owner/repo", Tags: &[]string{"lastModified:2020-02-02"}, Hashes: &[]cyclonedx.Hash{{Value: "ABC"}}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := ""
			if tt.args.c != nil {
				orig = tt.args.c.PackageURL
			}
			AddComponentPurl(tt.args.c)
			if tt.args.c == nil {
				// ensure no panic and no action.
				return
			}
			if tt.name == "noop when purl already set" {
				if tt.args.c.PackageURL != orig {
					t.Errorf("expected PackageURL to remain %s, got %s", orig, tt.args.c.PackageURL)
				}
			} else {
				if tt.args.c.PackageURL == "" {
					t.Errorf("expected PackageURL to be set for %s", tt.name)
				}
			}
		})
	}
}

func TestAddComponentBOMRef(t *testing.T) {
	type args struct {
		c *cyclonedx.Component
	}
	tests := []struct {
		name string
		args args
	}{
		{name: "uses packageURL when present", args: args{c: &cyclonedx.Component{PackageURL: "pkg:here/there"}}},
		{name: "generates uuid when no purl", args: args{c: &cyclonedx.Component{}}},
		{name: "preserves existing BOMRef", args: args{c: &cyclonedx.Component{BOMRef: "existing"}}},
		{name: "nil component", args: args{c: nil}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddComponentBOMRef(tt.args.c)
			if tt.args.c == nil {
				// nil input should be handled gracefully.
				return
			}
			switch tt.name {
			case "uses packageURL when present":
				if tt.args.c.BOMRef != tt.args.c.PackageURL {
					t.Errorf("expected BOMRef to equal PackageURL %s, got %s", tt.args.c.PackageURL, tt.args.c.BOMRef)
				}
			case "preserves existing BOMRef":
				if tt.args.c.BOMRef != "existing" {
					t.Errorf("expected BOMRef to remain existing, got %s", tt.args.c.BOMRef)
				}
			default:
				if !strings.HasPrefix(tt.args.c.BOMRef, "urn:uuid:") {
					t.Errorf("expected BOMRef to start with urn:uuid:, got %s", tt.args.c.BOMRef)
				}
			}
		})
	}
}
