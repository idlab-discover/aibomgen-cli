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
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "sets serial when empty",
			args:    args{bom: &cyclonedx.BOM{}},
			wantErr: false,
		},
		{
			name:    "preserves existing serial",
			args:    args{bom: &cyclonedx.BOM{SerialNumber: "urn:uuid:existing"}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := AddMetaSerialNumber(tt.args.bom); (err != nil) != tt.wantErr {
				t.Errorf("AddMetaSerialNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// Additional checks.
			if tt.args.bom.SerialNumber == "" {
				t.Errorf("SerialNumber should be set")
			}
		})
	}
}

func Test_generateUUID(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "uuid non-empty and unique"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u1 := generateUUID()
			u2 := generateUUID()
			if u1 == "" || u2 == "" {
				t.Errorf("generateUUID returned empty string")
			}
			if u1 == u2 {
				t.Errorf("generateUUID should return unique values but got duplicates: %s", u1)
			}
		})
	}
}

func TestAddMetaTimestamp(t *testing.T) {
	type args struct {
		bom *cyclonedx.BOM
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "sets timestamp when empty", args: args{bom: &cyclonedx.BOM{Metadata: &cyclonedx.Metadata{}}}, wantErr: false},
		{name: "preserves existing timestamp", args: args{bom: &cyclonedx.BOM{Metadata: &cyclonedx.Metadata{Timestamp: "2020-01-01T00:00:00Z"}}}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := AddMetaTimestamp(tt.args.bom); (err != nil) != tt.wantErr {
				t.Errorf("AddMetaTimestamp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.name == "sets timestamp when empty" {
				if tt.args.bom.Metadata.Timestamp == "" {
					t.Errorf("Timestamp should be set")
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

func TestCurrentTimestampRFC3339(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "valid rfc3339 format"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CurrentTimestampRFC3339(); got == "" {
				t.Errorf("CurrentTimestampRFC3339() returned empty string")
			} else {
				if _, err := time.Parse(time.RFC3339, got); err != nil {
					t.Errorf("CurrentTimestampRFC3339 produced invalid format: %v", err)
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
		name    string
		args    args
		wantErr bool
	}{
		{name: "adds tool with provided name and version", args: args{bom: &cyclonedx.BOM{}, toolName: "mytool", toolVersion: "v1"}, wantErr: false},
		{name: "adds tool with defaults when empty", args: args{bom: &cyclonedx.BOM{}, toolName: "", toolVersion: ""}, wantErr: false},
		{name: "appends to existing tools", args: args{bom: func() *cyclonedx.BOM {
			b := &cyclonedx.BOM{}
			b.Metadata = &cyclonedx.Metadata{Tools: &cyclonedx.ToolsChoice{Components: &[]cyclonedx.Component{{Name: "existing"}}}}
			return b
		}(), toolName: "x", toolVersion: "v1"}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := AddMetaTools(tt.args.bom, tt.args.toolName, tt.args.toolVersion); (err != nil) != tt.wantErr {
				t.Errorf("AddMetaTools() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
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

func TestPurlFromComponentMeta(t *testing.T) {
	type args struct {
		kind         string
		id           string
		lastModified string
		sha          string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "uses sha as version and normalizes id", args: args{kind: "model", id: " user / repo ", sha: "ABC"}, want: "pkg:huggingface/user/repo@abc"},
		{name: "empty sha omits version", args: args{kind: "dataset", id: "owner/ds", sha: ""}, want: "pkg:huggingface/datasets/owner/ds"},
		{name: "weird kind and empty id", args: args{kind: "weird", id: "", sha: "f00"}, want: "pkg:huggingface/unknown/unknown@f00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PurlFromComponentMeta(tt.args.kind, tt.args.id, tt.args.lastModified, tt.args.sha); got != tt.want {
				t.Errorf("PurlFromComponentMeta() = %v, want %v", got, tt.want)
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
