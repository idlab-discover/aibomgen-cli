package fetcher

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestModelSearcher_Search_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("search"); got != "bert" {
			t.Fatalf("search = %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "5" {
			t.Fatalf("limit = %q", got)
		}
		if got := r.URL.Query().Get("sort"); got != "downloads" {
			t.Fatalf("sort = %q", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("Accept = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"id":           "org/model",
				"modelId":      "org/model",
				"author":       "org",
				"pipeline_tag": "fill-mask",
				"tags":         []string{"pytorch"},
				"downloads":    10,
				"likes":        2,
				"private":      false,
				"gated":        false,
			},
		})
	}))
	defer srv.Close()

	searcher := &ModelSearcher{BaseURL: srv.URL}
	results, err := searcher.Search("bert", 5)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) != 1 || results[0].ID != "org/model" {
		t.Fatalf("results = %#v", results)
	}
}

func TestModelSearcher_Search_DefaultLimitAndNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("limit"); got != "20" {
			t.Fatalf("default limit = %q", got)
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	searcher := &ModelSearcher{BaseURL: srv.URL}
	_, err := searcher.Search("", 0)
	if err == nil || !strings.Contains(err.Error(), "unexpected status code: 403") {
		t.Fatalf("expected status error, got %v", err)
	}
}

func TestModelSearcher_Search_DecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{")
	}))
	defer srv.Close()

	searcher := &ModelSearcher{BaseURL: srv.URL}
	_, err := searcher.Search("bert", 1)
	if err == nil {
		t.Fatalf("expected decode error")
	}
}
