package metadata

import (
	"reflect"
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
)

// lic is a compact expectation for one license entry: exactly one of id/name, optional url.
type lic struct{ id, name, url string }

func licensesOf(ls *cdx.Licenses) []lic {
	if ls == nil {
		return nil
	}
	var out []lic
	for _, c := range *ls {
		out = append(out, lic{id: c.License.ID, name: c.License.Name, url: c.License.URL})
	}
	return out
}

func TestSPDXListEmbedded(t *testing.T) {
	spdxOnce.Do(loadSPDX)
	if len(spdxIDs) < 500 {
		t.Fatalf("expected >500 SPDX IDs, got %d", len(spdxIDs))
	}
	for in, want := range map[string]string{"apache-2.0": "Apache-2.0", "MIT": "MIT", "cc-by-nc-4.0": "CC-BY-NC-4.0", "Cc0-1.0": "CC0-1.0"} {
		if got, ok := spdxID(in); !ok || got != want {
			t.Errorf("spdxID(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"llama3.1", "other", "openrail", "gfdl"} {
		if _, ok := spdxID(in); ok {
			t.Errorf("spdxID(%q) matched, want no match", in)
		}
	}
}

func TestBuildLicenses(t *testing.T) {
	const base = "https://huggingface.co/"
	tests := []struct {
		name string
		in   licenseInput
		want []lic
	}{
		{"spdx canonical casing", licenseInput{Values: []string{"apache-2.0"}}, []lic{{id: "Apache-2.0"}}},
		{"spdx mit", licenseInput{Values: []string{"mit"}}, []lic{{id: "MIT"}}},
		{"spdx cc-by-nc", licenseInput{Values: []string{"cc-by-nc-4.0"}}, []lic{{id: "CC-BY-NC-4.0"}}},
		{"custom llama stays name", licenseInput{Values: []string{"llama3.1"}}, []lic{{name: "llama3.1"}}},
		{"custom name stays name", licenseInput{Values: []string{"my-license"}}, []lic{{name: "my-license"}}},
		{"other uses license_name", licenseInput{Values: []string{"other"}, Name: "deepseek-license"}, []lic{{name: "deepseek-license"}}},
		{"other without license_name", licenseInput{Values: []string{"other"}}, []lic{{name: "other"}}},
		{"absolute license_link", licenseInput{Values: []string{"other"}, Name: "x", Link: "https://example.com/LICENSE"}, []lic{{name: "x", url: "https://example.com/LICENSE"}}},
		{"relative license_link at commit", licenseInput{Values: []string{"llama3.2"}, Link: "LICENSE.txt", RepoPath: "meta-llama/M", SHA: "ABC"},
			[]lic{{name: "llama3.2", url: "https://huggingface.co/meta-llama/M/blob/abc/LICENSE.txt"}}},
		{"relative license_link without commit", licenseInput{Values: []string{"llama3.2"}, Link: "LICENSE"}, []lic{{name: "llama3.2"}}},
		{"multi license", licenseInput{Values: []string{"cc-by-sa-3.0", "gfdl"}, Link: "https://x/y"}, []lic{{id: "CC-BY-SA-3.0"}, {name: "gfdl"}}},
		{"dedupe case-insensitive", licenseInput{Values: []string{"MIT", "mit"}}, []lic{{id: "MIT"}}},
		{"no values", licenseInput{}, nil},
		{"[unknown]", licenseInput{Values: []string{"[unknown]"}}, nil},
		{"unknown", licenseInput{Values: []string{"unknown"}}, nil},
		{"more information needed", licenseInput{Values: []string{"[More Information Needed]"}}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := licensesOf(buildLicenses(tt.in, base))
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("buildLicenses = %+v, want %+v", got, tt.want)
			}
			for _, l := range got {
				if (l.id == "") == (l.name == "") {
					t.Fatalf("license must carry exactly one of id/name: %+v", l)
				}
			}
		})
	}
}

func TestModelLicenseSourcePrecedence(t *testing.T) {
	spec := specFor(t, ComponentLicenses)
	apply := func(src Source) []lic {
		comp := &cdx.Component{}
		ApplyFromSources(spec, src, Target{Component: comp, HuggingFaceBaseURL: "https://huggingface.co/"})
		return licensesOf(comp.Licenses)
	}

	// cardData wins over tags and README; placeholder groups fall through.
	src := Source{
		HF: &fetcher.ModelAPIResponse{
			ID:       "org/m",
			CardData: map[string]any{"license": []any{"unknown"}},
			Tags:     []string{"license:mit"},
		},
	}
	if got := apply(src); !reflect.DeepEqual(got, []lic{{id: "MIT"}}) {
		t.Fatalf("placeholder cardData should fall through to tags, got %+v", got)
	}

	// README front matter list with license_name/link.
	src = Source{
		HF: &fetcher.ModelAPIResponse{ID: "org/m", SHA: "abc"},
		Readme: &fetcher.ModelReadmeCard{FrontMatter: map[string]any{
			"license": "other", "license_name": "acme-1", "license_link": "LICENSE",
		}},
	}
	if got := apply(src); !reflect.DeepEqual(got, []lic{{name: "acme-1", url: "https://huggingface.co/org/m/blob/abc/LICENSE"}}) {
		t.Fatalf("README front matter license = %+v", got)
	}

	// No license anywhere -> no licenses field.
	if got := apply(Source{HF: &fetcher.ModelAPIResponse{ID: "org/m"}, Readme: &fetcher.ModelReadmeCard{License: "[More Information Needed]"}}); got != nil {
		t.Fatalf("expected no licenses, got %+v", got)
	}

	// User value (enrich) is normalized too.
	comp := &cdx.Component{}
	if err := ApplyUserValue(spec, "apache-2.0", Target{Component: comp}); err != nil {
		t.Fatal(err)
	}
	if got := licensesOf(comp.Licenses); !reflect.DeepEqual(got, []lic{{id: "Apache-2.0"}}) {
		t.Fatalf("user license = %+v", got)
	}
}

func TestDatasetLicenseFromList(t *testing.T) {
	spec := datasetSpecFor(t, DatasetLicenses)
	apply := func(src DatasetSource) []lic {
		comp := &cdx.Component{}
		ApplyDatasetFromSources(spec, src, DatasetTarget{Component: comp})
		return licensesOf(comp.Licenses)
	}
	// ["unknown"] used to be written as license.name "[unknown]".
	if got := apply(DatasetSource{HF: &fetcher.DatasetAPIResponse{ID: "bookcorpus/bookcorpus", CardData: map[string]any{"license": []any{"unknown"}}}}); got != nil {
		t.Fatalf("expected no licenses for [unknown], got %+v", got)
	}
	got := apply(DatasetSource{
		HF:     &fetcher.DatasetAPIResponse{ID: "legacy-datasets/wikipedia", CardData: map[string]any{"license": []any{"cc-by-sa-3.0", "gfdl"}}},
		Readme: &fetcher.DatasetReadmeCard{FrontMatter: map[string]any{"license": "mit"}},
	})
	if !reflect.DeepEqual(got, []lic{{id: "CC-BY-SA-3.0"}, {name: "gfdl"}}) {
		t.Fatalf("wikipedia licenses = %+v", got)
	}
}

func TestIsPlaceholder(t *testing.T) {
	for _, s := range []string{"", " ", "[unknown]", "Unknown", "[More Information Needed]", "more information needed", "N/A", "none", "TBD"} {
		if !isPlaceholder(s) {
			t.Errorf("isPlaceholder(%q) = false", s)
		}
	}
	for _, s := range []string{"mit", "Google", "other", "[mit]x"} {
		if isPlaceholder(s) {
			t.Errorf("isPlaceholder(%q) = true", s)
		}
	}
}
