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

// maxModelIndexMetrics caps the metrics taken from model-index: MTEB-style cards list
// hundreds of results with thousands of metrics.
const maxModelIndexMetrics = 100

// parseModelIndex reads the task (from the first result) and the metrics of every
// result of the first model-index entry, each with the dataset and split it was
// measured on.
func parseModelIndex(mi any, card *ModelReadmeCard) {
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
	if res, ok := resultsAny[0].(map[string]any); ok {
		if taskAny, ok := res["task"].(map[string]any); ok {
			card.TaskType = strings.TrimSpace(stringFromAny(taskAny["type"]))
			card.TaskName = strings.TrimSpace(stringFromAny(taskAny["name"]))
		}
	}

	var out []ModelIndexMetric
	for _, r := range resultsAny {
		res, ok := r.(map[string]any)
		if !ok {
			continue
		}
		dataset, split := modelIndexDataset(res["dataset"])
		metricsAny, _ := res["metrics"].([]any)
		for _, m := range metricsAny {
			if len(out) >= maxModelIndexMetrics {
				card.ModelIndexMetrics = out
				return
			}
			mm, ok := m.(map[string]any)
			if !ok {
				continue
			}
			mt := strings.TrimSpace(stringFromAny(mm["type"]))
			mv := strings.TrimSpace(stringFromAny(mm["value"]))
			if mt == "" && mv == "" {
				continue
			}
			out = append(out, ModelIndexMetric{Type: mt, Value: mv, Dataset: dataset, Split: split})
		}
	}
	if len(out) > 0 {
		card.ModelIndexMetrics = out
	}
}

// modelIndexDataset labels a model-index result dataset: its name, else its type plus
// the config in parentheses. It also returns the split.
func modelIndexDataset(v any) (label, split string) {
	ds, ok := v.(map[string]any)
	if !ok {
		return "", ""
	}
	label = strings.TrimSpace(stringFromAny(ds["name"]))
	if label == "" {
		label = strings.TrimSpace(stringFromAny(ds["type"]))
		if cfg := strings.TrimSpace(stringFromAny(ds["config"])); cfg != "" && label != "" {
			label += " (" + cfg + ")"
		}
	}
	return label, strings.TrimSpace(stringFromAny(ds["split"]))
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
	// descriptionHeadings hold a short summary of the model, tried in order.
	descriptionHeadings = []string{
		"Model description",
		"Model Summary",
		"Description",
		"Model Overview",
		"Overview",
		"Introduction",
		"Model Details",
		"Model Information",
		"Model", // last resort: generic
	}
)

const (
	// maxSectionRunes caps free text taken from a model card section.
	maxSectionRunes = 1000
	// maxDescriptionRunes caps the one-line model description.
	maxDescriptionRunes = 300
)

var (
	headingLineRe   = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
	headingNumberRe = regexp.MustCompile(`^\d+(?:\.\d+)*[.)]?\s+`)
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

// normalizeHeading lowercases a heading and drops markup (*, _, `), closing #s, a
// leading section number and trailing punctuation, so "## **Limitations:**",
// "### limitations" and "## 2. Limitations" compare equal.
func normalizeHeading(s string) string {
	s = headingMarkupRe.ReplaceAllString(s, "")
	s = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(s), "#"))
	s = headingNumberRe.ReplaceAllString(s, "")
	s = strings.TrimRight(s, ":.!? ")
	s = spaceRe.ReplaceAllString(s, " ")
	return strings.ToLower(strings.TrimSpace(s))
}

func isFenceLine(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")
}

type mdSection struct {
	heading string // normalized; empty for the preamble
	level   int    // number of #s; 0 for the preamble
	body    string
}

// splitSections splits Markdown into sections, one per heading (any level). A section's
// body runs up to the next heading of any level. Text before the first heading is a
// level-0 preamble section with an empty heading. Lines inside fenced code blocks are
// never treated as headings.
func splitSections(markdown string) []mdSection {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")

	var sections []mdSection
	cur := &mdSection{}
	var buf []string
	flush := func() {
		cur.body = strings.TrimSpace(strings.Join(buf, "\n"))
		sections = append(sections, *cur)
		buf = buf[:0]
	}

	inFence := false
	for _, line := range lines {
		if isFenceLine(line) {
			inFence = !inFence
		} else if !inFence {
			if m := headingLineRe.FindStringSubmatch(line); m != nil {
				flush()
				cur = &mdSection{heading: normalizeHeading(m[2]), level: len(m[1])}
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
		if s.level > 0 && s.heading == want {
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
			if s.level == 0 || s.heading != want {
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
	return truncateAtSentence(s, maxRunes, 0.5)
}

// truncateAtSentence cuts s to at most maxRunes runes. It keeps whole sentences when the
// last sentence end within the limit lies past minFrac of it; otherwise it cuts at a
// word boundary and appends an ellipsis.
func truncateAtSentence(s string, maxRunes int, minFrac float64) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	cut := string(r[:maxRunes])
	// Look one rune past the limit so a sentence ending exactly at it is found.
	probe := string(r[:maxRunes+1])
	if locs := sentenceEndRe.FindAllStringIndex(probe, -1); len(locs) > 0 {
		// Keep up to and including the punctuation (\s is a single ASCII byte in RE2).
		if end := locs[len(locs)-1][1] - 1; float64(end) >= float64(len(cut))*minFrac {
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

var (
	mdLinkRe        = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	urlParenRe      = regexp.MustCompile(`\s*\([^()]*https?://[^()]*\)`)
	bareURLRe       = regexp.MustCompile(`<?https?://[^\s)>]+>?`)
	inlineMarkupRe  = regexp.MustCompile("\\*\\*|__|`")
	emphasisRe      = regexp.MustCompile(`(^|[\s(])[*_]([^*_\n]+)[*_]([\s.,;:!?)]|$)`)
	listItemRe      = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s`)
	boldLabelLineRe = regexp.MustCompile(`^\s*\*\*[^*]+(?::\*\*|\*\*\s*:)`)
	boldOnlyRe      = regexp.MustCompile(`^\*\*[^*]+\*\*$`)
	// pointerRe matches paragraphs that only point elsewhere ("For more details, please refer to ...").
	pointerRe = regexp.MustCompile(`(?i)^(?:for (?:more|further) (?:details|information)|please (?:refer|see|check|visit)|refer to|check out|see )`)
)

// truncateDescription caps a description, keeping whole sentences when it can.
func truncateDescription(s string) string {
	return truncateAtSentence(s, maxDescriptionRunes, 0)
}

// hfBoilerplatePrefix starts the auto-generated text of the HF model card template.
const hfBoilerplatePrefix = "this is the model card of a 🤗 transformers model"

// extractDescriptionSection returns the first prose paragraph of the first
// description-like section (see descriptionHeadings) that has one.
func extractDescriptionSection(body string) string {
	sections := splitSections(body)
	for _, h := range descriptionHeadings {
		want := normalizeHeading(h)
		for _, s := range sections {
			if s.level == 0 || s.heading != want {
				continue
			}
			if p := firstProseParagraph(s.body); p != "" {
				return truncateDescription(p)
			}
		}
	}
	return ""
}

// extractLeadParagraph returns the first prose paragraph after the title: it looks at
// the text before the first heading and under level-1 headings, and stops at the first
// heading of level 2 or deeper.
func extractLeadParagraph(body string) string {
	for _, s := range splitSections(body) {
		if s.level >= 2 {
			break
		}
		if p := firstProseParagraph(s.body); p != "" {
			return truncateDescription(p)
		}
	}
	return ""
}

// firstProseParagraph returns the first paragraph of s that reads as prose, flattened
// to one line. Code, HTML, images, badges, lists, tables, label lines and bold
// pseudo-headings are skipped, as are paragraphs that only point elsewhere. A list glued to a paragraph is cut off, and so is a
// trailing lead-in sentence ending in ":" ("It has the following features:").
func firstProseParagraph(s string) string {
	for _, para := range strings.Split(cleanSectionText(s, 0), "\n\n") {
		lines := strings.Split(strings.TrimSpace(para), "\n")
		first := lines[0]
		if first == "" || listItemRe.MatchString(first) || boldLabelLineRe.MatchString(first) ||
			boldOnlyRe.MatchString(strings.TrimSpace(first)) ||
			strings.HasPrefix(first, "|") || strings.HasPrefix(first, ">") || strings.HasPrefix(first, "#") {
			continue
		}
		// Keep the prose lines before a list or table.
		for i, l := range lines {
			if listItemRe.MatchString(l) || strings.HasPrefix(strings.TrimSpace(l), "|") {
				lines = lines[:i]
				break
			}
		}
		flat := flattenInline(strings.Join(lines, "\n"))
		if strings.HasSuffix(flat, ":") {
			flat = dropLastSentence(flat)
		}
		if len(strings.Fields(flat)) < 4 || isTemplatePlaceholder(flat) || pointerRe.MatchString(flat) ||
			strings.HasPrefix(strings.ToLower(flat), hfBoilerplatePrefix) {
			continue
		}
		return flat
	}
	return ""
}

// dropLastSentence removes the last sentence of s, returning "" if s is one sentence.
func dropLastSentence(s string) string {
	locs := sentenceEndRe.FindAllStringIndex(s, -1)
	if len(locs) == 0 {
		return ""
	}
	return s[:locs[len(locs)-1][1]-1]
}

// flattenInline turns a Markdown paragraph into one plain line: links become their
// text, bare URLs (and parentheticals holding one) and emphasis markers are dropped and
// whitespace is collapsed.
func flattenInline(s string) string {
	s = mdLinkRe.ReplaceAllString(s, "$1")
	s = urlParenRe.ReplaceAllString(s, "")
	s = bareURLRe.ReplaceAllString(s, "")
	s = inlineMarkupRe.ReplaceAllString(s, "")
	s = emphasisRe.ReplaceAllString(s, "$1$2$3")
	return strings.Join(strings.Fields(s), " ")
}
