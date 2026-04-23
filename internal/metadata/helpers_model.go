package metadata

import (
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

func ensureModelParameters(card *cdx.MLModelCard) *cdx.MLModelParameters {
	if card.ModelParameters == nil {
		card.ModelParameters = &cdx.MLModelParameters{}
	}
	return card.ModelParameters
}

func ensureConsiderations(card *cdx.MLModelCard) *cdx.MLModelCardConsiderations {
	if card.Considerations == nil {
		card.Considerations = &cdx.MLModelCardConsiderations{}
	}
	return card.Considerations
}

func ensureQuantitativeAnalysis(card *cdx.MLModelCard) *cdx.MLQuantitativeAnalysis {
	if card.QuantitativeAnalysis == nil {
		card.QuantitativeAnalysis = &cdx.MLQuantitativeAnalysis{}
	}
	return card.QuantitativeAnalysis
}

func normalizeDatasetRef(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "dataset:") {
		return s
	}
	// If it already looks like a namespaced identifier (e.g., "org/ds"), still prefix with dataset:.
	return "dataset:" + s
}

func setProperty(c *cdx.Component, name, value string) {
	if c == nil {
		return
	}
	name = strings.TrimSpace(name)
	value = strings.TrimSpace(value)
	if name == "" || value == "" {
		return
	}
	if c.Properties == nil {
		c.Properties = &[]cdx.Property{}
	}
	*c.Properties = append(*c.Properties, cdx.Property{Name: name, Value: value})
}

func hasProperty(c *cdx.Component, name string) bool {
	if c == nil || c.Properties == nil {
		return false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for _, p := range *c.Properties {
		if strings.TrimSpace(p.Name) == name && strings.TrimSpace(p.Value) != "" {
			return true
		}
	}
	return false
}

func bomComponent(b *cdx.BOM) *cdx.Component {
	if b == nil || b.Metadata == nil {
		return nil
	}
	return b.Metadata.Component
}

func bomHasComponentName(b *cdx.BOM) bool {
	c := bomComponent(b)
	return c != nil && strings.TrimSpace(c.Name) != ""
}

func bomModelParameters(b *cdx.BOM) *cdx.MLModelParameters {
	c := bomComponent(b)
	if c == nil || c.ModelCard == nil {
		return nil
	}
	return c.ModelCard.ModelParameters
}

func extractLicense(cardData map[string]any, tags []string) string {
	// cardData.license.
	if cardData != nil {
		if v, ok := cardData["license"]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	// tag license:apache-2.0.
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if strings.HasPrefix(t, "license:") {
			return strings.TrimSpace(strings.TrimPrefix(t, "license:"))
		}
	}
	return ""
}

func extractLanguage(cardData map[string]any) string {
	if cardData == nil {
		return ""
	}
	v, ok := cardData["language"]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case []any:
		var out []string
		for _, it := range t {
			if s, ok := it.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					out = append(out, s)
				}
			}
		}
		return strings.Join(out, ",")
	default:
		return ""
	}
}

func extractDatasets(cardData map[string]any, tags []string) []string {
	seen := map[string]struct{}{}
	var out []string

	add := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		if !strings.Contains(raw, ":") {
			raw = "dataset:" + raw
		}
		if _, ok := seen[raw]; ok {
			return
		}
		seen[raw] = struct{}{}
		out = append(out, raw)
	}

	// cardData.datasets: string or array.
	if cardData != nil {
		if v, ok := cardData["datasets"]; ok && v != nil {
			switch t := v.(type) {
			case string:
				add(t)
			case []any:
				for _, it := range t {
					if s, ok := it.(string); ok {
						add(s)
					}
				}
			}
		}
	}

	// tags: dataset:NAME.
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if strings.HasPrefix(t, "dataset:") {
			add(t)
		}
	}

	return out
}

func normalizeStrings(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
