package fetcher

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDatasetAPIFetcher_Fetch_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/api/datasets/org/dataset" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("Accept = %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":           "org/dataset",
			"author":       "org",
			"sha":          "abc123",
			"lastModified": "2026-01-02T03:04:05Z",
			"private":      false,
			"gated":        "auto",
			"tags":         []string{"tabular", "metadata"},
			"description":  "dataset description",
			"downloads":    12,
			"likes":        3,
			"usedStorage":  42,
		})
	}))
	defer srv.Close()

	f := &DatasetAPIFetcher{BaseURL: srv.URL}
	resp, err := f.Fetch(" /org/dataset ")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if resp.ID != "org/dataset" {
		t.Fatalf("ID = %q", resp.ID)
	}
	if resp.Gated.String == nil || *resp.Gated.String != "auto" {
		t.Fatalf("expected gated string auto, got %#v", resp.Gated)
	}
	if len(resp.Tags) != 2 || resp.Tags[1] != "metadata" {
		t.Fatalf("tags = %#v", resp.Tags)
	}
}

func TestDatasetAPIFetcher_Fetch_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := &DatasetAPIFetcher{BaseURL: srv.URL}
	_, err := f.Fetch("org/missing")
	if err == nil || !strings.Contains(err.Error(), "status 404") {
		t.Fatalf("expected status error, got %v", err)
	}
}

func TestDatasetAPIFetcher_Fetch_DecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{")
	}))
	defer srv.Close()

	f := &DatasetAPIFetcher{BaseURL: srv.URL}
	_, err := f.Fetch("org/dataset")
	if err == nil {
		t.Fatalf("expected decode error")
	}
}

func TestDatasetAPIFetcher_Fetch_RequestError(t *testing.T) {
	want := errors.New("boom")
	f := &DatasetAPIFetcher{
		BaseURL: "http://invalid.local",
		Client: &http.Client{
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				return nil, want
			}),
		},
	}
	_, err := f.Fetch("org/dataset")
	if err == nil || !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}
}
