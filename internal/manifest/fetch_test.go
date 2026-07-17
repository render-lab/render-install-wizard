package manifest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func sampleManifest(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../manifest/manifest.json")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	return data
}

func TestFetchParsesSample(t *testing.T) {
	sample := sampleManifest(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(sample)
	}))
	defer srv.Close()

	m, err := Fetch(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if m.Version != "1" {
		t.Errorf("Version = %q, want %q", m.Version, "1")
	}
	if len(m.Components) != 3 {
		t.Errorf("len(Components) = %d, want 3", len(m.Components))
	}
	if len(m.Tools) != 4 {
		t.Errorf("len(Tools) = %d, want 4", len(m.Tools))
	}
}

func TestFetchAppendsVersion(t *testing.T) {
	sample := sampleManifest(t)
	var gotVersion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion = r.URL.Query().Get("version")
		_, _ = w.Write(sample)
	}))
	defer srv.Close()

	if _, err := Fetch(context.Background(), srv.URL, "v9"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if gotVersion != "v9" {
		t.Errorf("version query = %q, want %q", gotVersion, "v9")
	}
}

func TestFetchPreservesExistingQuery(t *testing.T) {
	sample := sampleManifest(t)
	var gotFoo, gotVersion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotFoo = r.URL.Query().Get("foo")
		gotVersion = r.URL.Query().Get("version")
		_, _ = w.Write(sample)
	}))
	defer srv.Close()

	if _, err := Fetch(context.Background(), srv.URL+"?foo=bar", "v9"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if gotFoo != "bar" {
		t.Errorf("foo query = %q, want %q", gotFoo, "bar")
	}
	if gotVersion != "v9" {
		t.Errorf("version query = %q, want %q", gotVersion, "v9")
	}
}

func TestFetchNon2xxErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, err := Fetch(context.Background(), srv.URL, ""); err == nil {
		t.Fatal("Fetch returned nil error on 500, want error")
	}
}
