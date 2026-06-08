package fetcher

import (
	"reflect"
	"testing"
)

func TestSplitFrontMatter(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		wantMeta   map[string]any
		wantBody   string
		wantNoMeta bool
	}{
		{
			name: "valid front matter",
			raw: `---
license: apache-2.0
tags:
  - ai
  - metadata
---

# Dataset Card
`,
			wantMeta: map[string]any{
				"license": "apache-2.0",
				"tags":    []any{"ai", "metadata"},
			},
			wantBody: "# Dataset Card",
		},
		{
			name:       "missing delimiter returns raw body",
			raw:        "# Plain README",
			wantNoMeta: true,
			wantBody:   "# Plain README",
		},
		{
			name:       "invalid yaml keeps raw content",
			raw:        "---\nlicense: [\n---\n# Body",
			wantNoMeta: true,
			wantBody:   "---\nlicense: [\n---\n# Body",
		},
		{
			name:       "empty input",
			raw:        "",
			wantNoMeta: true,
			wantBody:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMeta, gotBody := splitFrontMatter(tt.raw)
			if tt.wantNoMeta {
				if gotMeta != nil {
					t.Fatalf("expected nil metadata, got %#v", gotMeta)
				}
			} else if !reflect.DeepEqual(gotMeta, tt.wantMeta) {
				t.Fatalf("metadata mismatch:\n got: %#v\nwant: %#v", gotMeta, tt.wantMeta)
			}
			if gotBody != tt.wantBody {
				t.Fatalf("body = %q, want %q", gotBody, tt.wantBody)
			}
		})
	}
}

func TestStringSliceFromAny(t *testing.T) {
	got := stringSliceFromAny([]any{" alpha ", "beta", "alpha", "", 42})
	want := []string{"alpha", "beta", "42"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stringSliceFromAny() = %#v, want %#v", got, want)
	}
}
