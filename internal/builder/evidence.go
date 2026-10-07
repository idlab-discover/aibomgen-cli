package builder

import (
	"fmt"
	neturl "net/url"
	"sort"
	"strings"

	"github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/scanner"
)

// AddComponentEvidence records how the model was identified in component.evidence.
// It must run after AddComponentPurl, because the identity concludes the purl.
//   - Source scan: one occurrence per place the reference was found, and an identity
//     whose methods are the matching rules (source-code-analysis) with their confidence.
//   - Model ID: an identity whose method is the Hub API request that resolved the model.
func AddComponentEvidence(c *cyclonedx.Component, ctx BuildContext, hfBaseURL string) {
	if c == nil {
		return
	}
	if occs := ctx.Scan.Occurrences; len(occs) > 0 {
		c.Evidence = scanEvidence(occs, c.PackageURL)
		return
	}
	if id := strings.TrimSpace(ctx.ModelID); id != "" {
		c.Evidence = modelIDEvidence(id, ctx.Revision, hfBaseURL, c.PackageURL)
	}
}

func scanEvidence(occs []scanner.Occurrence, purl string) *cyclonedx.Evidence {
	occurrences := make([]cyclonedx.EvidenceOccurrence, 0, len(occs))
	methodSet := map[string]bool{}
	for _, o := range occs {
		occ := cyclonedx.EvidenceOccurrence{
			Location:          o.Location,
			Symbol:            o.Symbol,
			AdditionalContext: o.Method,
		}
		if o.Cell > 0 {
			// A notebook line counts within its cell, not the file: keep it in the context.
			occ.AdditionalContext = fmt.Sprintf("%s (cell %d, line %d)", o.Method, o.Cell, o.Line)
		} else if o.Line > 0 {
			line := o.Line
			occ.Line = &line
		}
		occurrences = append(occurrences, occ)
		methodSet[o.Method] = true
	}

	names := make([]string, 0, len(methodSet))
	for m := range methodSet {
		names = append(names, m)
	}
	sort.Slice(names, func(i, j int) bool {
		ci, cj := scanner.RuleConfidence(names[i]), scanner.RuleConfidence(names[j])
		if ci != cj {
			return ci > cj
		}
		return names[i] < names[j]
	})

	methods := make([]cyclonedx.EvidenceIdentityMethod, 0, len(names))
	var best float32
	for _, m := range names {
		conf := scanner.RuleConfidence(m)
		if conf > best {
			best = conf
		}
		methods = append(methods, cyclonedx.EvidenceIdentityMethod{
			Technique:  cyclonedx.EvidenceIdentityTechniqueSourceCodeAnalysis,
			Confidence: &conf,
			Value:      m,
		})
	}

	return &cyclonedx.Evidence{
		Identity:    identityChoice(purl, best, methods),
		Occurrences: &occurrences,
	}
}

func modelIDEvidence(id, revision, hfBaseURL, purl string) *cyclonedx.Evidence {
	base := strings.TrimSpace(hfBaseURL)
	if base == "" {
		base = "https://huggingface.co/"
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	url := base + "api/models/" + strings.TrimPrefix(id, "/")
	if rev := strings.TrimSpace(revision); rev != "" {
		url += "/revision/" + neturl.PathEscape(rev)
	}

	conf := float32(1)
	methods := []cyclonedx.EvidenceIdentityMethod{{
		Technique:  cyclonedx.EvidenceIdentityTechniqueOther,
		Confidence: &conf,
		Value:      url,
	}}
	return &cyclonedx.Evidence{Identity: identityChoice(purl, conf, methods)}
}

func identityChoice(purl string, confidence float32, methods []cyclonedx.EvidenceIdentityMethod) *cyclonedx.EvidenceIdentityChoice {
	return &cyclonedx.EvidenceIdentityChoice{
		Identities: &[]cyclonedx.EvidenceIdentity{{
			Field:          cyclonedx.EvidenceIdentityFieldTypePURL,
			Confidence:     &confidence,
			ConcludedValue: purl,
			Methods:        &methods,
		}},
	}
}
