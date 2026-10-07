package bomio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

func evidenceBOM() *cdx.BOM {
	bom := minimalBOM()
	line := 3
	conf := float32(0.9)
	methods := []cdx.EvidenceIdentityMethod{{Technique: cdx.EvidenceIdentityTechniqueSourceCodeAnalysis, Confidence: &conf, Value: "from_pretrained"}}
	bom.Metadata.Component.Evidence = &cdx.Evidence{
		Identity: &cdx.EvidenceIdentityChoice{Identities: &[]cdx.EvidenceIdentity{{
			Field: cdx.EvidenceIdentityFieldTypePURL, Confidence: &conf, ConcludedValue: "pkg:huggingface/org/m@abc", Methods: &methods,
		}}},
		Occurrences: &[]cdx.EvidenceOccurrence{{Location: "app.py", Line: &line, Symbol: "org/m", AdditionalContext: "from_pretrained"}},
	}
	return bom
}

// The evidence block is written as-is for 1.6/1.7 and downgraded by the CycloneDX
// library for 1.5: a single identity without concludedValue, occurrences without line.
func TestWriteBOM_EvidencePerSpecVersion(t *testing.T) {
	for _, spec := range []string{"1.5", "1.6", "1.7"} {
		t.Run(spec, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "bom.json")
			if err := WriteBOM(evidenceBOM(), out, spec); err != nil {
				t.Fatalf("WriteBOM: %v", err)
			}
			raw, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			var doc struct {
				Metadata struct {
					Component struct {
						Evidence struct {
							Identity    json.RawMessage  `json:"identity"`
							Occurrences []map[string]any `json:"occurrences"`
						} `json:"evidence"`
					} `json:"component"`
				} `json:"metadata"`
			}
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatal(err)
			}
			ev := doc.Metadata.Component.Evidence
			if len(ev.Occurrences) != 1 || ev.Occurrences[0]["location"] != "app.py" {
				t.Fatalf("occurrences = %v", ev.Occurrences)
			}
			_, hasLine := ev.Occurrences[0]["line"]
			isArray := len(ev.Identity) > 0 && ev.Identity[0] == '['
			if spec == "1.5" {
				var id map[string]any
				if isArray || hasLine || json.Unmarshal(ev.Identity, &id) != nil || id["concludedValue"] != nil {
					t.Fatalf("1.5: identity %s, occurrence %v; want one object without concludedValue and no line", ev.Identity, ev.Occurrences[0])
				}
				return
			}
			if !isArray || !hasLine {
				t.Fatalf("%s: identity %s, occurrence %v; want an identity array and a line", spec, ev.Identity, ev.Occurrences[0])
			}
		})
	}
}
