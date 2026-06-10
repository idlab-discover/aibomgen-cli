package fetcher

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDatasetReadmeFetcher_Fetch_Success_ParseFrontMatterAndSections(t *testing.T) {
	readme := `---
license: apache-2.0
tags:
  - tabular
language:
  - en
annotations_creators:
  - expert-generated
configs:
  - config_name: default
    data_files:
      - split: train
        path: data/train.csv
---

# Dataset Card

## Dataset Description

Metadata annotation benchmark.

## Dataset Card for Dataset Name

- **Curated by:** idlab
- **Funded by:** example grant
- **Shared by:** benchmark team
- **Repository:** https://example.com/repo
- **Paper:** https://example.com/paper
- **Demo:** https://example.com/demo

## Out-of-Scope Use

Do not use for clinical decisions.

## Personal and Sensitive Information

No personal data in the synthetic split.

## Bias, Risks, and Limitations

Schema coverage is limited.

## Dataset Card Contact

contact@example.com
`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/datasets/org/dataset/resolve/main/README.md" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Accept"); !strings.Contains(got, "text/markdown") {
			t.Fatalf("Accept = %q", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(readme))
	}))
	defer srv.Close()

	f := &DatasetReadmeFetcher{BaseURL: srv.URL}
	card, err := f.Fetch("org/dataset")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if card.License != "apache-2.0" {
		t.Fatalf("license = %q", card.License)
	}
	if len(card.Tags) != 1 || card.Tags[0] != "tabular" {
		t.Fatalf("tags = %#v", card.Tags)
	}
	if len(card.Language) != 1 || card.Language[0] != "en" {
		t.Fatalf("language = %#v", card.Language)
	}
	if len(card.Configs) != 1 || card.Configs[0].DataFiles[0].Path != "data/train.csv" {
		t.Fatalf("configs = %#v", card.Configs)
	}
	if !strings.Contains(card.DatasetDescription, "Metadata annotation") {
		t.Fatalf("DatasetDescription = %q", card.DatasetDescription)
	}
	if card.CuratedBy != "idlab" {
		t.Fatalf("CuratedBy = %q", card.CuratedBy)
	}
	if card.DatasetCardContact != "contact@example.com" {
		t.Fatalf("DatasetCardContact = %q", card.DatasetCardContact)
	}
}

func TestDatasetReadmeFetcher_Fetch_FallbackToMaster(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/datasets/org/dataset/resolve/main/README.md" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Path == "/datasets/org/dataset/resolve/master/README.md" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("# ok"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := &DatasetReadmeFetcher{BaseURL: srv.URL}
	card, err := f.Fetch("org/dataset")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if card == nil || !strings.Contains(card.Raw, "# ok") {
		t.Fatalf("expected raw readme, got %#v", card)
	}
}
