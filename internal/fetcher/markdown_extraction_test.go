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
		"1. Introduction":                   "introduction",
		"2.1 Model Summary":                 "model summary",
		"3) Limitations":                    "limitations",
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

func TestExtractDescriptionSection(t *testing.T) {
	for _, alias := range descriptionHeadings {
		t.Run(alias, func(t *testing.T) {
			md := "# Title\n\n## " + alias + "\n\nThis model does a useful thing well.\n\n## Training\n\nX y z w."
			if got := extractDescriptionSection(md); got != "This model does a useful thing well." {
				t.Fatalf("extractDescriptionSection() = %q", got)
			}
		})
	}

	tests := []struct {
		name string
		md   string
		want string
	}{
		{
			name: "numbered heading",
			md:   "# DeepSeek-R1\n\n## 1. Introduction\n\nWe introduce our first-generation reasoning models.",
			want: "We introduce our first-generation reasoning models.",
		},
		{
			name: "falls through a section without prose",
			md:   "## Model Details\n\n- **Developed by:** org\n- **License:** mit\n\n## Introduction\n\nQwen2 is a new series of large language models.",
			want: "Qwen2 is a new series of large language models.",
		},
		{
			name: "HF template boilerplate is skipped",
			md: "## Model Details\n\n### Model Description\n\n<!-- Provide a longer summary of what this model is. -->\n\n" +
				"This is the model card of a 🤗 transformers model that has been pushed on the Hub. This model card has been automatically generated.\n\n" +
				"- **Developed by:** [More Information Needed]",
			want: "",
		},
		{
			name: "heading inside a code fence is ignored",
			md:   "## Usage\n\n```python\n## Introduction\nprint('this is not a heading at all')\n```\n",
			want: "",
		},
		{
			name: "bold pseudo-heading is skipped",
			md:   "## 2. Model Summary\n\n---\n\n**Post-Training: Large-Scale Reinforcement Learning on the Base Model**\n\n- We directly apply RL.\n\n## 1. Introduction\n\nWe introduce our reasoning models.",
			want: "We introduce our reasoning models.",
		},
		{
			name: "glued list and lead-in sentence are cut",
			md:   "## Introduction\n\nQwen2.5 is the latest series of Qwen models. It brings the following improvements:\n- More knowledge\n- Better coding",
			want: "Qwen2.5 is the latest series of Qwen models.",
		},
		{
			name: "pointer paragraph is skipped",
			md:   "## Model Overview\n\nFor more details, including benchmark evaluation, please refer to our [blog](https://x.example).",
			want: "",
		},
		{
			name: "single-sentence lead-in is not prose",
			md:   "## Model Overview\n\nQwen3-4B-Instruct-2507 has the following features:\n- Type: Causal Language Models",
			want: "",
		},
		{
			name: "label lines and lead-ins are skipped",
			md:   "### Description\n\n**Model developer**: Meta\n\nThe model has the following features:\n\nGemma is a family of lightweight open models.",
			want: "Gemma is a family of lightweight open models.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractDescriptionSection(tt.md); got != tt.want {
				t.Fatalf("extractDescriptionSection() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractLeadParagraph(t *testing.T) {
	tests := []struct {
		name string
		md   string
		want string
	}{
		{
			name: "badge-only README",
			md: "# My Model\n\n[![Build](https://img.shields.io/badge/build-passing-green)](https://ci.example.com)\n" +
				"[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)\n\n" +
				"<a href=\"https://chat.example.com\"><img alt=\"Chat\" src=\"https://img.shields.io/badge/chat-blue\"/></a>\n\n## Usage\n\nRun the model with the pipeline.",
			want: "",
		},
		{
			name: "HTML-only README",
			md:   "<div align=\"center\">\n  <img src=\"logo.svg\" width=\"60%\" />\n</div>\n<hr>\n<p align=\"center\"><a href=\"https://x.example\">Homepage</a></p>\n",
			want: "",
		},
		{
			name: "prose inside HTML counts once tags are stripped",
			md:   "<p align=\"center\">A compact vision model for image tagging.</p>\n",
			want: "A compact vision model for image tagging.",
		},
		{
			name: "lead-in ending in a URL is skipped",
			md: "# GPT-2\n\nTest the whole generation capabilities here: https://transformer.huggingface.co/doc/gpt2-large\n\n" +
				"Pretrained model on English language using a causal language modeling (CLM) objective.",
			want: "Pretrained model on English language using a causal language modeling (CLM) objective.",
		},
		{
			name: "list and table before prose",
			md:   "# Model\n\n- item one here\n- item two here\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\nA ResNet-B image classification model.",
			want: "A ResNet-B image classification model.",
		},
		{
			name: "stops at the first level-2 heading",
			md:   "# Model Card for Mistral\n\n## Encode and Decode\n\n```py\nfrom mistral_common import x\n```\n\nThis paragraph is not after the title.",
			want: "",
		},
		{
			name: "links flattened and lines joined",
			md:   "# all-MiniLM-L6-v2\n\nThis is a [sentence-transformers](https://www.SBERT.net) model: It maps sentences\nto a **384 dimensional** dense vector space.",
			want: "This is a sentence-transformers model: It maps sentences to a 384 dimensional dense vector space.",
		},
		{
			name: "parenthetical URL and emphasis removed",
			md:   "# SDXL\n\nThe base model feeds a refinement model (available here: https://huggingface.co/x/y) and is a _sequence-to-sequence_ model, see *paper*.",
			want: "The base model feeds a refinement model and is a sequence-to-sequence model, see paper.",
		},
		{
			name: "snake_case is kept",
			md:   "# M\n\nSet the model_type field to bert in the config file.",
			want: "Set the model_type field to bert in the config file.",
		},
		{
			name: "text under a later level-1 heading",
			md:   "For more details please refer to our github repo: https://github.com/x/y\n\n# BGE-M3 ([paper](https://arxiv.org/x))\n\nIn this project, we introduce BGE-M3.",
			want: "In this project, we introduce BGE-M3.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractLeadParagraph(tt.md); got != tt.want {
				t.Fatalf("extractLeadParagraph() = %q, want %q", got, tt.want)
			}
		})
	}

	long := "# Model\n\n" + strings.Repeat("This model is good at many things. ", 20)
	got := extractLeadParagraph(long)
	if n := utf8.RuneCountInString(got); n > maxDescriptionRunes || !strings.HasSuffix(got, "things.") {
		t.Fatalf("want <= %d runes ending at a sentence, got %d: %q", maxDescriptionRunes, n, got)
	}

	// A short first sentence is kept whole instead of being cut mid-sentence later on.
	first := "GPT-2 is a transformers model pretrained on English data."
	got = extractLeadParagraph("# GPT-2\n\n" + first + " " + strings.Repeat("word ", 80) + "end.")
	if got != first {
		t.Fatalf("want the first sentence only, got %q", got)
	}
}
