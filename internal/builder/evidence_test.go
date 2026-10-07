package builder

import (
	"strings"
	"testing"

	"github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/scanner"
)

func identity(t *testing.T, ev *cyclonedx.Evidence) cyclonedx.EvidenceIdentity {
	t.Helper()
	if ev == nil || ev.Identity == nil || ev.Identity.Identities == nil || len(*ev.Identity.Identities) != 1 {
		t.Fatalf("want one identity, got %+v", ev)
	}
	return (*ev.Identity.Identities)[0]
}

func TestAddComponentEvidence_Scan(t *testing.T) {
	c := &cyclonedx.Component{PackageURL: "pkg:huggingface/org/m@abc"}
	AddComponentEvidence(c, BuildContext{
		ModelID: "org/m",
		Scan: scanner.Discovery{Occurrences: []scanner.Occurrence{
			{Location: "a/one.py", Line: 3, Method: "from_pretrained", Symbol: "org/m"},
			{Location: "nb.ipynb", Line: 2, Cell: 4, Method: "model_kwarg_slash", Symbol: "org/m"},
			{Location: "two.yaml", Line: 1, Method: "yaml_model_field", Symbol: "ORG/m"},
		}},
	}, "")

	occs := *c.Evidence.Occurrences
	if len(occs) != 3 {
		t.Fatalf("occurrences = %+v", occs)
	}
	if o := occs[0]; o.Location != "a/one.py" || o.Line == nil || *o.Line != 3 || o.AdditionalContext != "from_pretrained" || o.Symbol != "org/m" {
		t.Fatalf("file occurrence = %+v", o)
	}
	if o := occs[1]; o.Line != nil || o.AdditionalContext != "model_kwarg_slash (cell 4, line 2)" {
		t.Fatalf("notebook occurrence = %+v, want no line and the cell in the context", o)
	}
	if occs[2].Symbol != "ORG/m" {
		t.Fatalf("symbol must keep the ID as written, got %q", occs[2].Symbol)
	}

	id := identity(t, c.Evidence)
	if id.Field != cyclonedx.EvidenceIdentityFieldTypePURL || id.ConcludedValue != c.PackageURL || id.Confidence == nil || *id.Confidence != 0.9 {
		t.Fatalf("identity = %+v", id)
	}
	var got []string
	for _, m := range *id.Methods {
		if m.Technique != cyclonedx.EvidenceIdentityTechniqueSourceCodeAnalysis || m.Confidence == nil {
			t.Fatalf("method = %+v", m)
		}
		got = append(got, m.Value)
	}
	if strings.Join(got, ",") != "from_pretrained,model_kwarg_slash,yaml_model_field" {
		t.Fatalf("methods = %v, want sorted by confidence", got)
	}
}

func TestAddComponentEvidence_ModelID(t *testing.T) {
	c := &cyclonedx.Component{PackageURL: "pkg:huggingface/org/m@abc"}
	AddComponentEvidence(c, BuildContext{ModelID: "org/m", Revision: "v1"}, "https://huggingface.co")

	if c.Evidence.Occurrences != nil {
		t.Fatalf("model-ID input must not have occurrences: %+v", c.Evidence.Occurrences)
	}
	id := identity(t, c.Evidence)
	m := (*id.Methods)[0]
	if m.Technique != cyclonedx.EvidenceIdentityTechniqueOther || m.Value != "https://huggingface.co/api/models/org/m/revision/v1" {
		t.Fatalf("method = %+v", m)
	}
	if id.ConcludedValue != c.PackageURL || *id.Confidence != 1 {
		t.Fatalf("identity = %+v", id)
	}
}

func TestBuild_EvidenceReplacesAibomgenProperties(t *testing.T) {
	bom, err := NewBOMBuilder(DefaultOptions()).Build(BuildContext{
		ModelID: "org/m",
		HF:      &fetcher.ModelAPIResponse{ID: "org/m", SHA: "abc"},
		Scan: scanner.Discovery{
			ID: "org/m", Type: "model", Path: "/src/app.py", Evidence: "from_pretrained at line 3",
			Occurrences: []scanner.Occurrence{{Location: "app.py", Line: 3, Method: "from_pretrained", Symbol: "org/m"}},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	c := bom.Metadata.Component
	if c.Properties != nil {
		for _, p := range *c.Properties {
			if strings.HasPrefix(p.Name, "aibomgen.") {
				t.Fatalf("unexpected legacy property %s", p.Name)
			}
		}
	}
	if id := identity(t, c.Evidence); id.ConcludedValue != "pkg:huggingface/org/m@abc" {
		t.Fatalf("concludedValue = %q, want the computed purl", id.ConcludedValue)
	}
}

func TestBuild_LifecyclePostBuild(t *testing.T) {
	bom, err := NewBOMBuilder(DefaultOptions()).Build(BuildContext{ModelID: "org/m"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	lc := bom.Metadata.Lifecycles
	if lc == nil || len(*lc) != 1 || (*lc)[0].Phase != cyclonedx.LifecyclePhasePostBuild {
		t.Fatalf("lifecycles = %+v, want one post-build phase", lc)
	}
}
