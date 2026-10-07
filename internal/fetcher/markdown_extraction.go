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
	body := rest[idx:]
	if strings.HasPrefix(body, "\n---\n") {
		body = strings.TrimPrefix(body, "\n---\n")
	} else {
		body = strings.TrimPrefix(body, "\n---")
	}
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

// Heading aliases for the model card considerations. Popular cards rarely use the exact
// Hugging Face template headings, so each field accepts several variants. They are tried
// in order and matched after normalizeHeading, so case and markup don't matter.
var (
	useCaseHeadings = []string{
		"Direct Use",
		"Uses",
		"Intended uses",
		"Intended use",
		"Intended uses & limitations",
		"Intended uses and limitations",
		"How to use", // last resort: usually instructions rather than use cases
	}
	outOfScopeHeadings = []string{
		"Out-of-Scope Use",
		"Out-of-scope uses",
		"Misuse and out-of-scope use",
		"Misuse, Malicious Use, and Out-of-Scope Use",
	}
	limitationHeadings = []string{
		"Bias, Risks, and Limitations",
		"Limitations",
		"Limitations and bias",
		"Limitations and biases",
		"Bias and limitations",
		"Known limitations",
		"Risks and limitations",
	}
	ethicalHeadings = []string{
		"Bias",
		"Ethical considerations",
		"Ethics",
		"Responsible AI",
	}
	recommendationHeadings = []string{
		"Recommendations",
	}
)

// maxSectionRunes caps free text taken from a model card section.
const maxSectionRunes = 1000

var (
	headingLineRe   = regexp.MustCompile(`^#{1,6}\s+(.+?)\s*$`)
	headingMarkupRe = regexp.MustCompile("[*_`]")
	spaceRe         = regexp.MustCompile(`\s+`)
	fencedCodeRe    = regexp.MustCompile("(?ms)^[ \t]*(```|~~~).*?^[ \t]*(```|~~~)[^\n]*$")
	htmlCommentRe   = regexp.MustCompile(`(?s)<!--.*?-->`)
	htmlTagRe       = regexp.MustCompile(`</?[a-zA-Z][^>]*>`)
	mdImageRe       = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	blankLinesRe    = regexp.MustCompile(`\n[ \t]*\n(?:[ \t]*\n)+`)
	// sentenceEndRe matches ".", "!" or "?" after a non-digit and before whitespace,
	// so list markers like "6." don't count as a sentence end.
	sentenceEndRe = regexp.MustCompile(`[^\d\s][.!?]\s`)
)

// normalizeHeading lowercases a heading and drops markup (*, _, `), closing #s and
// trailing punctuation so "## **Limitations:**" and "### limitations" compare equal.
func normalizeHeading(s string) string {
	s = headingMarkupRe.ReplaceAllString(s, "")
	s = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(s), "#"))
	s = strings.TrimRight(s, ":.!? ")
	s = spaceRe.ReplaceAllString(s, " ")
	return strings.ToLower(strings.TrimSpace(s))
}

func isFenceLine(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")
}

type mdSection struct {
	heading string // normalized
	body    string
}

// splitSections splits Markdown into sections, one per heading (any level). A section's
// body runs up to the next heading of any level. Lines inside fenced code blocks are
// never treated as headings.
func splitSections(markdown string) []mdSection {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")

	var sections []mdSection
	var cur *mdSection
	var buf []string
	flush := func() {
		if cur != nil {
			cur.body = strings.TrimSpace(strings.Join(buf, "\n"))
			sections = append(sections, *cur)
		}
		buf = buf[:0]
	}

	inFence := false
	for _, line := range lines {
		if isFenceLine(line) {
			inFence = !inFence
		} else if !inFence {
			if m := headingLineRe.FindStringSubmatch(line); m != nil {
				flush()
				cur = &mdSection{heading: normalizeHeading(m[1])}
				continue
			}
		}
		buf = append(buf, line)
	}
	flush()
	return sections
}

// extractSection returns the body of the first section whose heading matches heading.
func extractSection(markdown string, heading string) string {
	want := normalizeHeading(heading)
	for _, s := range splitSections(markdown) {
		if s.heading == want {
			return s.body
		}
	}
	return ""
}

// extractSectionAny tries the headings in order and returns the body of the first
// matching section that holds real text. Sections that are empty after cleanSectionText
// are skipped; one that only holds a template placeholder is returned only when no
// alias has real text, so README text stays verbatim.
func extractSectionAny(markdown string, headings []string) string {
	sections := splitSections(markdown)
	placeholder := ""
	for _, h := range headings {
		want := normalizeHeading(h)
		for _, s := range sections {
			if s.heading != want {
				continue
			}
			c := cleanSectionText(s.body, 0)
			switch {
			case c == "":
			case isTemplatePlaceholder(c):
				if placeholder == "" {
					placeholder = s.body
				}
			default:
				return s.body
			}
		}
	}
	return placeholder
}

// isTemplatePlaceholder reports whether s is the HF template filler "[More Information Needed]".
func isTemplatePlaceholder(s string) bool {
	s = strings.TrimSpace(s)
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(s, "["), "]"))
	return strings.EqualFold(s, "More Information Needed")
}

// cleanSectionText strips fenced code blocks, HTML (comments and tags) and images from
// section text, collapses blank lines and, when maxRunes > 0, caps the length.
func cleanSectionText(s string, maxRunes int) string {
	s = fencedCodeRe.ReplaceAllString(s, "")
	s = htmlCommentRe.ReplaceAllString(s, "")
	s = mdImageRe.ReplaceAllString(s, "")
	s = htmlTagRe.ReplaceAllString(s, "")
	s = blankLinesRe.ReplaceAllString(s, "\n\n")
	s = strings.TrimSpace(s)
	if maxRunes > 0 {
		s = truncateText(s, maxRunes)
	}
	return s
}

// truncateText cuts s to at most maxRunes runes (plus an ellipsis), preferring the end
// of a sentence, then a word boundary.
func truncateText(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	cut := string(r[:maxRunes])
	if locs := sentenceEndRe.FindAllStringIndex(cut, -1); len(locs) > 0 {
		// Keep up to and including the punctuation (\s is a single ASCII byte in RE2).
		if end := locs[len(locs)-1][1] - 1; end >= len(cut)/2 {
			return cut[:end]
		}
	}
	if i := strings.LastIndexAny(cut, " \n\t"); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimSpace(cut) + "…"
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
