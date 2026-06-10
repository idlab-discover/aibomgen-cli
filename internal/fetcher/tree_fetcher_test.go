package fetcher

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDatasetTreeFetcher_Fetch_PaginatesWithCursor(t *testing.T) {
	var seenCursors []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/datasets/org/dataset/tree/main" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("expand") != "true" {
			t.Fatalf("expand = %q", r.URL.Query().Get("expand"))
		}
		if r.URL.Query().Get("recursive") != "true" {
			t.Fatalf("recursive = %q", r.URL.Query().Get("recursive"))
		}
		cursor := r.URL.Query().Get("cursor")
		seenCursors = append(seenCursors, cursor)

		w.Header().Set("Content-Type", "application/json")
		switch cursor {
		case "":
			w.Header().Set("Link", `<`+srvURL(t, r)+`?cursor=next-page>; rel="next"`)
			_ = json.NewEncoder(w).Encode([]SecurityFileEntry{{Path: "data/train.csv", Size: 12}})
		case "next-page":
			_ = json.NewEncoder(w).Encode([]SecurityFileEntry{{Path: "README.md", Size: 34}})
		default:
			t.Fatalf("unexpected cursor %q", cursor)
		}
	}))
	defer srv.Close()

	f := &DatasetTreeFetcher{BaseURL: srv.URL}
	entries, err := f.Fetch("org/dataset")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %#v", entries)
	}
	if entries[0].Path != "data/train.csv" || entries[1].Path != "README.md" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
	if strings.Join(seenCursors, ",") != ",next-page" {
		t.Fatalf("seen cursors = %#v", seenCursors)
	}
}

func TestModelTreeFetcher_Fetch_PaginatesWithCursor(t *testing.T) {
	var seenCursors []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models/org/model/tree/main" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		cursor := r.URL.Query().Get("cursor")
		seenCursors = append(seenCursors, cursor)

		w.Header().Set("Content-Type", "application/json")
		switch cursor {
		case "":
			w.Header().Set("Link", `<`+srvURL(t, r)+`?cursor=next-page>; rel="next"`)
			_ = json.NewEncoder(w).Encode([]SecurityFileEntry{{Path: "model.safetensors", Size: 12}})
		case "next-page":
			_ = json.NewEncoder(w).Encode([]SecurityFileEntry{{Path: "README.md", Size: 34}})
		default:
			t.Fatalf("unexpected cursor %q", cursor)
		}
	}))
	defer srv.Close()

	f := &ModelTreeFetcher{BaseURL: srv.URL}
	entries, err := f.Fetch("org/model")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %#v", entries)
	}
	if entries[0].Path != "model.safetensors" || entries[1].Path != "README.md" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
	if strings.Join(seenCursors, ",") != ",next-page" {
		t.Fatalf("seen cursors = %#v", seenCursors)
	}
}

func TestDatasetTreeFetcher_Fetch_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := &DatasetTreeFetcher{BaseURL: srv.URL}
	_, err := f.Fetch("org/missing")
	if err == nil || !strings.Contains(err.Error(), "status 404") {
		t.Fatalf("expected status error, got %v", err)
	}
}

func srvURL(t *testing.T, r *http.Request) string {
	t.Helper()
	return "http://" + r.Host + r.URL.Path
}
