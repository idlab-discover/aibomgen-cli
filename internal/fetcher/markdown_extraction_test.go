package fetcher

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
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

func TestNormalizeHeading(t *testing.T) {
	tests := map[string]string{
		"Limitations":                       "limitations",
		"**Limitations:**":                  "limitations",
		"`Uses`":                            "uses",
		"LIMITATIONS AND  BIAS ##":          "limitations and bias",
		"Bias, Risks, and Limitations":      "bias, risks, and limitations",
		"_Intended uses & limitations_ ...": "intended uses & limitations",
	}
	for in, want := range tests {
		if got := normalizeHeading(in); got != want {
			t.Errorf("normalizeHeading(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExtractSectionAny_Aliases(t *testing.T) {
	lists := map[string][]string{
		"useCases":        useCaseHeadings,
		"outOfScope":      outOfScopeHeadings,
		"limitations":     limitationHeadings,
		"ethical":         ethicalHeadings,
		"recommendations": recommendationHeadings,
	}
	for name, aliases := range lists {
		for _, alias := range aliases {
			t.Run(name+"/"+alias, func(t *testing.T) {
				md := "# Model\n\nIntro.\n\n## " + alias + "\n\nSection text for " + alias + ".\n\n## Training data\n\nOther."
				got := extractSectionAny(md, aliases)
				if got != "Section text for "+alias+"." {
					t.Fatalf("extractSectionAny() = %q", got)
				}
			})
		}
	}
}

func TestExtractSectionAny_HeadingVariants(t *testing.T) {
	tests := []struct {
		name string
		md   string
	}{
		{"bold with colon", "## **Limitations:**\nText."},
		{"upper case", "### LIMITATIONS\nText."},
		{"backticks", "## `Limitations`\nText."},
		{"level one", "# Limitations\nText."},
		{"level four", "#### Limitations\nText."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractSectionAny(tt.md, limitationHeadings); got != "Text." {
				t.Fatalf("extractSectionAny() = %q, want %q", got, "Text.")
			}
		})
	}
}

func TestExtractSectionAny_Priority(t *testing.T) {
	// HF template: "## Uses" only holds a comment; the text lives in "### Direct Use".
	md := "## Uses\n\n<!-- Address questions around how the model is intended to be used -->\n\n### Direct Use\n\nClassify text.\n\n## How to use\n\nCall the pipeline."
	if got := extractSectionAny(md, useCaseHeadings); got != "Classify text." {
		t.Fatalf("got %q, want Direct Use text", got)
	}

	// A placeholder section falls through to an alias with real text.
	md = "### Direct Use\n\n[More Information Needed]\n\n## Intended uses\n\nEmbed sentences."
	if got := extractSectionAny(md, useCaseHeadings); got != "Embed sentences." {
		t.Fatalf("got %q, want Intended uses text", got)
	}

	// With no real text anywhere, the placeholder is kept verbatim.
	md = "### Direct Use\n\n[More Information Needed]\n\n## Training data\n\nX."
	if got := extractSectionAny(md, useCaseHeadings); got != "[More Information Needed]" {
		t.Fatalf("got %q, want placeholder", got)
	}

	if got := extractSectionAny("## Training data\n\nX.", useCaseHeadings); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestExtractSection_IgnoresHeadingsInFencedCode(t *testing.T) {
	md := "## Usage\n\n```python\n## Limitations\nprint('not a heading')\n```\n\n## Training data\n\nX."
	if got := extractSectionAny(md, limitationHeadings); got != "" {
		t.Fatalf("heading inside a code fence matched: %q", got)
	}

	// A "# comment" inside a fence must not end the section.
	md = "## Limitations\n\nBefore.\n\n~~~python\n# Load model\nm = load()\n~~~\n\nAfter.\n\n## Citation\n\nY."
	got := extractSection(md, "Limitations")
	if !strings.Contains(got, "Before.") || !strings.Contains(got, "After.") {
		t.Fatalf("section cut at a fenced comment: %q", got)
	}
}

func TestCleanSectionText(t *testing.T) {
	in := "Intro text.\n\n<!-- template hint -->\n\n```python\n>>> from transformers import pipeline\n```\n\n" +
		"![chart](https://example.com/c.png)\n\n<p align=\"center\">Centered</p>\n\n\n\nOutro."
	got := cleanSectionText(in, 0)
	want := "Intro text.\n\nCentered\n\nOutro."
	if got != want {
		t.Fatalf("cleanSectionText() = %q, want %q", got, want)
	}
}

func TestCleanSectionText_Cap(t *testing.T) {
	long := strings.Repeat("word ", 300) // 1500 runes, no sentence end
	got := cleanSectionText(long, maxSectionRunes)
	if n := utf8.RuneCountInString(got); n > maxSectionRunes+1 {
		t.Fatalf("len = %d, want <= %d", n, maxSectionRunes+1)
	}
	if !strings.HasSuffix(got, "word…") {
		t.Fatalf("want cut at a word boundary with ellipsis, got suffix %q", got[len(got)-10:])
	}

	sentences := strings.Repeat("This is a sentence. ", 80) // 1600 runes
	got = cleanSectionText(sentences, maxSectionRunes)
	if !strings.HasSuffix(got, "sentence.") || utf8.RuneCountInString(got) > maxSectionRunes {
		t.Fatalf("want cut at a sentence end, got %d runes ending %q", utf8.RuneCountInString(got), got[len(got)-12:])
	}

	// A numbered list marker ("6.") is not a sentence end.
	list := "Known restrictions:\n\n" + strings.Repeat("1. Item text that ends here. ", 34) + "6. Last item without an end"
	got = cleanSectionText(list, maxSectionRunes)
	if !strings.HasSuffix(got, "ends here.") {
		t.Fatalf("want cut after a sentence, not a list marker; got suffix %q", got[len(got)-12:])
	}

	if got := cleanSectionText("short", maxSectionRunes); got != "short" {
		t.Fatalf("short text changed: %q", got)
	}
}
