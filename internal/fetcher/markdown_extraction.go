package fetcher

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// ---- Markdown extraction helpers (shared with model_readme_fetcher and dataset_readme_fetcher) ----.

func splitFrontMatter(raw string) (map[string]any, string) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "---\n") {
		return nil, raw
	}
	// Find the second '---' marker at start of a line.
	// We only treat it as front matter if it starts at the very beginning.
	rest := strings.TrimPrefix(raw, "---\n")
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		// allow file ending marker.
		idx = strings.Index(rest, "\n---")
		if idx < 0 {
			return nil, raw
		}
	}

	y := rest[:idx]
	body := strings.TrimSpace(rest[idx:])
	body = strings.TrimPrefix(body, "\n---\n")
	body = strings.TrimPrefix(body, "\n---")
	body = strings.TrimSpace(body)

	m := map[string]any{}
	dec := yaml.NewDecoder(bytes.NewReader([]byte(y)))
	dec.KnownFields(false)
	if err := dec.Decode(&m); err != nil {
		// If YAML parsing fails, still return body; callers can still regex parse.
		return nil, strings.TrimSpace(raw)
	}
	return m, body
}

func stringFromAny(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func stringSliceFromAny(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			s := strings.TrimSpace(stringFromAny(x))
			if s != "" {
				out = append(out, s)
			}
		}
		return normalizeStrings(out)
	case []string:
		return normalizeStrings(t)
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return nil
		}
		return []string{s}
	default:
		s := strings.TrimSpace(fmt.Sprint(t))
		if s == "" {
			return nil
		}
		return []string{s}
	}
}

func normalizeStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func parseModelIndex(mi any, card *ModelReadmeCard) {
	// The model-index is typically a list of entries.
	// We only take the first result for now.
	list, ok := mi.([]any)
	if !ok || len(list) == 0 {
		return
	}
	first, ok := list[0].(map[string]any)
	if !ok {
		return
	}
	resultsAny, ok := first["results"].([]any)
	if !ok || len(resultsAny) == 0 {
		return
	}
	res, ok := resultsAny[0].(map[string]any)
	if !ok {
		return
	}
	// task.
	if taskAny, ok := res["task"].(map[string]any); ok {
		card.TaskType = strings.TrimSpace(stringFromAny(taskAny["type"]))
		card.TaskName = strings.TrimSpace(stringFromAny(taskAny["name"]))
	}
	// metrics.
	if metricsAny, ok := res["metrics"].([]any); ok {
		out := make([]ModelIndexMetric, 0, len(metricsAny))
		for _, m := range metricsAny {
			mm, ok := m.(map[string]any)
			if !ok {
				continue
			}
			mt := strings.TrimSpace(stringFromAny(mm["type"]))
			mv := strings.TrimSpace(stringFromAny(mm["value"]))
			if mt == "" && mv == "" {
				continue
			}
			out = append(out, ModelIndexMetric{Type: mt, Value: mv})
		}
		if len(out) > 0 {
			card.ModelIndexMetrics = out
		}
	}
}

func extractSection(markdown string, heading string) string {
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	lines := strings.Split(markdown, "\n")

	// Find a level-2 or level-3 heading with the requested text.
	// Example matches:.
	//   "## Bias, Risks, and Limitations".
	//   "### Direct Use".
	headingRe := regexp.MustCompile(fmt.Sprintf(`^#{2,3}\s+%s\s*$`, regexp.QuoteMeta(heading)))
	nextHeadingRe := regexp.MustCompile(`^#+\s+.+$`)

	found := false
	buf := make([]string, 0)
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !found {
			if headingRe.MatchString(line) {
				found = true
			}
			continue
		}
		// Stop at the next heading (any level).
		if nextHeadingRe.MatchString(line) {
			break
		}
		buf = append(buf, line)
	}
	return strings.TrimSpace(strings.Join(buf, "\n"))
}

func extractBulletValue(markdown string, label string) string {
	// Extract values like:.
	// - **Paper [optional]:** https://...
	// - **Developed by:** org.
	// - **Carbon Emitted** *(additional text)*: 149.2 kg eq. CO2.
	// Supports optional bracketed qualifiers in the label part and text between the label and colon.
	// Pattern handles both: **Label:** (colon inside) and **Label** text: (colon outside).
	pat := fmt.Sprintf(`(?m)^-\s+\*\*%s(?:\s*\[[^\]]+\])?(?::\*\*|\*\*[^:\n]*:)\s*(.+?)\s*$`, regexp.QuoteMeta(label))
	re := regexp.MustCompile(pat)
	m := re.FindStringSubmatch(markdown)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}
