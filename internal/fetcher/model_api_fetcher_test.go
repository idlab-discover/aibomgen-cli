package fetcher

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func rewriteToServer(t *testing.T, srvURL string) http.RoundTripper {
	t.Helper()
	u, err := url.Parse(srvURL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	return roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		rr := r.Clone(r.Context())
		rr.URL.Scheme = u.Scheme
		rr.URL.Host = u.Host
		rr.Host = u.Host
		rr.RequestURI = ""
		return http.DefaultTransport.RoundTrip(rr)
	})
}

func TestBoolOrString_UnmarshalJSON(t *testing.T) {
	t.Run("empty bytes", func(t *testing.T) {
		var v BoolOrString
		if err := v.UnmarshalJSON([]byte("")); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if v.Bool != nil || v.String != nil {
			t.Fatalf("expected nil fields, got Bool=%v String=%v", v.Bool, v.String)
		}
	})

	t.Run("null", func(t *testing.T) {
		var v BoolOrString
		if err := v.UnmarshalJSON([]byte("null")); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if v.Bool != nil || v.String != nil {
			t.Fatalf("expected nil fields, got Bool=%v String=%v", v.Bool, v.String)
		}
	})

	t.Run("string", func(t *testing.T) {
		var v BoolOrString
		if err := v.UnmarshalJSON([]byte(`" auto "`)); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if v.String == nil || *v.String != "auto" {
			t.Fatalf("expected String=auto, got %v", v.String)
		}
		if v.Bool != nil {
			t.Fatalf("expected Bool=nil, got %v", v.Bool)
		}
	})

	t.Run("bool", func(t *testing.T) {
		var v BoolOrString
		if err := v.UnmarshalJSON([]byte(`true`)); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if v.Bool == nil || *v.Bool != true {
			t.Fatalf("expected Bool=true, got %v", v.Bool)
		}
		if v.String != nil {
			t.Fatalf("expected String=nil, got %v", v.String)
		}
	})

	t.Run("invalid string json", func(t *testing.T) {
		var v BoolOrString
		if err := v.UnmarshalJSON([]byte(`"unterminated`)); err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("invalid bool json", func(t *testing.T) {
		var v BoolOrString
		if err := v.UnmarshalJSON([]byte(`notabool`)); err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}

func TestFetch_Success_DefaultClientNil_NoToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/api/models/my/model" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("Accept = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("Authorization should be empty, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":           "my/model",
			"modelId":      "my/model",
			"library_name": "transformers",
			"pipeline_tag": "text-generation",
			"gated":        "auto",
		})
	}))
	defer srv.Close()

	f := &ModelAPIFetcher{
		Client:  nil, // cover default-client branch
		BaseURL: srv.URL,
	}
	resp, err := f.Fetch(" /my/model ")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if resp == nil {
		t.Fatalf("expected response")
	}
	if resp.Gated.String == nil || *resp.Gated.String != "auto" {
		t.Fatalf("expected gated string auto, got %#v", resp.Gated)
	}
}

func TestFetch_SetsAuthorizationHeader_And_TrimsBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer t0k" {
			t.Fatalf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"x","modelId":"x","gated":true}`)
	}))
	defer srv.Close()

	f := &ModelAPIFetcher{
		Client:  NewHFClient(0, "  t0k "),
		BaseURL: srv.URL + "/", // cover TrimRight branch
	}
	resp, err := f.Fetch("x")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if resp.Gated.Bool == nil || *resp.Gated.Bool != true {
		t.Fatalf("expected gated bool true, got %#v", resp.Gated)
	}
}

func TestFetch_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	f := &ModelAPIFetcher{BaseURL: srv.URL}
	_, err := f.Fetch("x")
	if err == nil || !strings.Contains(err.Error(), "status 403") {
		t.Fatalf("expected status error, got %v", err)
	}
}

func TestFetch_DecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{") // invalid json
	}))
	defer srv.Close()

	f := &ModelAPIFetcher{BaseURL: srv.URL}
	_, err := f.Fetch("x")
	if err == nil {
		t.Fatalf("expected decode error, got nil")
	}
}

func TestFetch_RequestError(t *testing.T) {
	want := errors.New("boom")
	f := &ModelAPIFetcher{
		BaseURL: "http://invalid.local",
		Client: &http.Client{
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				return nil, want
			}),
		},
	}
	_, err := f.Fetch("x")
	if err == nil || !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}
}

func TestFetch_DefaultBaseURLBranch_WithoutNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models/p/q" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"p/q","modelId":"p/q","gated":false}`)
	}))
	defer srv.Close()

	// BaseURL left empty to cover default-BaseURL branch, but transport rewrites to httptest server.
	f := &ModelAPIFetcher{
		BaseURL: "   ",
		Client:  &http.Client{Transport: rewriteToServer(t, srv.URL)},
	}
	resp, err := f.Fetch("/p/q")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if resp.Gated.Bool == nil || *resp.Gated.Bool != false {
		t.Fatalf("expected gated bool false, got %#v", resp.Gated)
	}
}

func TestFetch_NewRequestError_InvalidBaseURL(t *testing.T) {
	f := &ModelAPIFetcher{
		// Invalid host (missing closing bracket) => NewRequestWithContext should error.
		BaseURL: "http://[::1",
	}
	got, err := f.Fetch("x")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if got != nil {
		t.Fatalf("expected nil response, got %#v", got)
	}
}

func TestModelFetchersUseRevisionURLs(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.EscapedPath())
		switch {
		case strings.HasSuffix(r.URL.Path, "README.md"):
			_, _ = w.Write([]byte("---\nlicense: mit\n---\n"))
		case strings.Contains(r.URL.Path, "/tree/"):
			_, _ = w.Write([]byte("[]"))
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "org/m", "sha": "abc"})
		}
	}))
	defer srv.Close()

	if _, err := (&ModelAPIFetcher{BaseURL: srv.URL}).FetchRevision("org/m", "refs/pr/1"); err != nil {
		t.Fatal(err)
	}
	if _, err := (&ModelReadmeFetcher{BaseURL: srv.URL}).FetchRevision("org/m", "v1.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := (&ModelTreeFetcher{BaseURL: srv.URL}).FetchRevision("org/m", "v1.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := (&ModelAPIFetcher{BaseURL: srv.URL}).Fetch("org/m"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"/api/models/org/m/revision/refs%2Fpr%2F1",
		"/org/m/resolve/v1.0/README.md",
		"/api/models/org/m/tree/v1.0",
		"/api/models/org/m",
	}
	if strings.Join(paths, " ") != strings.Join(want, " ") {
		t.Fatalf("requested paths = %v, want %v", paths, want)
	}
}

// lineageServer serves a model API response and, for ?expand[]=baseModels, the lineage.
// It records the request URIs.
func lineageServer(t *testing.T, cardData string, lineageStatus int) (*httptest.Server, *[]string) {
	t.Helper()
	var requests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("expand[]") == "baseModels" {
			w.WriteHeader(lineageStatus)
			_, _ = io.WriteString(w, `{"_id":"x","id":"org/m","baseModels":{"relation":"finetune","models":[{"_id":"y","id":"org/base"}]}}`)
			return
		}
		_, _ = io.WriteString(w, `{"id":"org/m","sha":"abc","cardData":`+cardData+`}`)
	}))
	t.Cleanup(srv.Close)
	return srv, &requests
}

func TestFetchRevision_BaseModels(t *testing.T) {
	srv, reqs := lineageServer(t, `{"base_model":"org/base"}`, http.StatusOK)
	resp, err := (&ModelAPIFetcher{BaseURL: srv.URL}).Fetch("org/m")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(*reqs) != 2 || !strings.Contains((*reqs)[1], "expand") {
		t.Fatalf("requests = %v, want the model then the lineage request", *reqs)
	}
	bm := resp.BaseModels
	if bm == nil || bm.Relation != "finetune" || len(bm.Models) != 1 || bm.Models[0].ID != "org/base" {
		t.Fatalf("BaseModels = %+v", bm)
	}
	if resp.SHA != "abc" {
		t.Fatalf("main response fields lost: sha = %q", resp.SHA)
	}
}

func TestFetchRevision_NoBaseModel_NoLineageRequest(t *testing.T) {
	srv, reqs := lineageServer(t, `{"license":"mit"}`, http.StatusOK)
	resp, err := (&ModelAPIFetcher{BaseURL: srv.URL}).Fetch("org/m")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(*reqs) != 1 || resp.BaseModels != nil {
		t.Fatalf("requests = %v, baseModels = %+v; want one request and no lineage", *reqs, resp.BaseModels)
	}
}

func TestFetchRevision_LineageFailureIsIgnored(t *testing.T) {
	srv, _ := lineageServer(t, `{"base_model":["org/a","org/b"]}`, http.StatusInternalServerError)
	resp, err := (&ModelAPIFetcher{BaseURL: srv.URL}).Fetch("org/m")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if resp.BaseModels != nil || resp.SHA != "abc" {
		t.Fatalf("want the main response without lineage, got %+v", resp)
	}
}

func TestFetchRevision_LineageUsesRevisionPath(t *testing.T) {
	srv, reqs := lineageServer(t, `{"base_model":"org/base"}`, http.StatusOK)
	if _, err := (&ModelAPIFetcher{BaseURL: srv.URL}).FetchRevision("org/m", "v1.0"); err != nil {
		t.Fatalf("FetchRevision error: %v", err)
	}
	if len(*reqs) != 2 || !strings.HasPrefix((*reqs)[1], "/api/models/org/m/revision/v1.0?expand") {
		t.Fatalf("requests = %v", *reqs)
	}
}
