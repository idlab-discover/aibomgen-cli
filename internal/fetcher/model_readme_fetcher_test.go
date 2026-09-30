package fetcher

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestModelReadmeFetcher_Fetch_Success_ParseFrontMatterAndSections(t *testing.T) {
	readme := `---
license: apache-2.0
tags:
  - tag-a
  - tag-b
datasets:
  - glue
metrics:
  - accuracy
base_model: bert-base-uncased
model-index:
  - name: org/model
    results:
      - task:
          type: text-classification
          name: Text Classification
        metrics:
          - type: accuracy
            value: 0.91
---

# Model Card

## Model Details

### Model Description

- **Developed by:** hf-team
- **Paper [optional]:** https://example.com/paper
- **Demo [optional]:** https://example.com/demo

## Uses

### Direct Use

Use it for classification.

### Out-of-Scope Use

Do not use for medical.

## Bias, Risks, and Limitations

This model may be biased.

### Recommendations

Use with care.

## Environmental Impact

- **Hardware Type:** NVIDIA A100
- **Hours used:** 10
- **Cloud Provider:** AWS
- **Compute Region:** us-east-1
- **Carbon Emitted:** 123g

## Model Card Contact

contact@example.com
`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method=%s", r.Method)
		}
		if r.URL.Path != "/org/model/resolve/main/README.md" {
			t.Fatalf("path=%q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Fatalf("Authorization=%q", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(readme))
	}))
	defer srv.Close()

	f := &ModelReadmeFetcher{Client: NewHFClient(0, "tok"), BaseURL: srv.URL}
	card, err := f.Fetch("org/model")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if card == nil {
		t.Fatalf("expected card")
	}
	if strings.TrimSpace(card.License) != "apache-2.0" {
		t.Fatalf("license=%q", card.License)
	}
	if len(card.Tags) != 2 {
		t.Fatalf("tags=%v", card.Tags)
	}
	if len(card.Datasets) != 1 || card.Datasets[0] != "glue" {
		t.Fatalf("datasets=%v", card.Datasets)
	}
	if card.BaseModel != "bert-base-uncased" {
		t.Fatalf("base_model=%q", card.BaseModel)
	}
	if card.DevelopedBy != "hf-team" {
		t.Fatalf("developedBy=%q", card.DevelopedBy)
	}
	if card.TaskType != "text-classification" {
		t.Fatalf("taskType=%q", card.TaskType)
	}
	if len(card.ModelIndexMetrics) != 1 || card.ModelIndexMetrics[0].Type != "accuracy" {
		t.Fatalf("modelIndexMetrics=%v", card.ModelIndexMetrics)
	}
	if !strings.Contains(card.DirectUse, "classification") {
		t.Fatalf("directUse=%q", card.DirectUse)
	}
	if card.ModelCardContact != "contact@example.com" {
		t.Fatalf("modelCardContact=%q", card.ModelCardContact)
	}
	if card.EnvironmentalHardwareType != "NVIDIA A100" {
		t.Fatalf("hardwareType=%q", card.EnvironmentalHardwareType)
	}
	if card.EnvironmentalCloudProvider != "AWS" {
		t.Fatalf("cloudProvider=%q", card.EnvironmentalCloudProvider)
	}
	if card.EnvironmentalCarbonEmitted != "123g" {
		t.Fatalf("carbonEmitted=%q", card.EnvironmentalCarbonEmitted)
	}
}

func TestModelReadmeFetcher_Fetch_FallbackToMaster(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/org/model/resolve/main/README.md" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Path == "/org/model/resolve/master/README.md" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("# ok"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := &ModelReadmeFetcher{BaseURL: srv.URL}
	card, err := f.Fetch("org/model")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if card == nil || !strings.Contains(card.Raw, "# ok") {
		t.Fatalf("expected raw readme")
	}
}

func TestParseReadmeCard_BaseModel(t *testing.T) {
	tests := map[string]string{
		"---\nbase_model: meta-llama/Llama-3.2-1B\n---\n": "meta-llama/Llama-3.2-1B",
		"---\nbase_model:\n- org/a\n- org/b\n---\n":       "org/a,org/b",
		"---\nbase_model: []\n---\n":                      "",
		"---\nlicense: mit\n---\n":                        "",
	}
	for raw, want := range tests {
		if got := parseReadmeCard(raw).BaseModel; got != want {
			t.Errorf("BaseModel for %q = %q, want %q", raw, got, want)
		}
	}
}
