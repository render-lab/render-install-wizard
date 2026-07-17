package content

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchSendsAcceptAndReturnsBody(t *testing.T) {
	const md = "# Cursor\n\nSome guide."
	var gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		_, _ = w.Write([]byte(md))
	}))
	defer srv.Close()

	got, err := Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !strings.Contains(gotAccept, "text/markdown") {
		t.Errorf("Accept header = %q, want to contain %q", gotAccept, "text/markdown")
	}
	if got != md {
		t.Errorf("Fetch body = %q, want %q", got, md)
	}
}

func TestFetchNon2xxErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, err := Fetch(context.Background(), srv.URL); err == nil {
		t.Fatal("Fetch returned nil error on 500, want error")
	}
}

func TestFetchWithFallbackUsesEmbeddedOnServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer srv.Close()

	got := FetchWithFallback(context.Background(), srv.URL, "cursor")
	want, ok := Embedded("cursor")
	if !ok {
		t.Fatal("Embedded(cursor) not found")
	}
	if got != want {
		t.Errorf("FetchWithFallback = %q, want embedded snapshot", got)
	}
}

func TestFetchWithFallbackUsesEmbeddedOnRefusedConnection(t *testing.T) {
	// A server that is created then immediately closed yields a refused/closed
	// connection, exercising the transport-error fallback path.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	got := FetchWithFallback(context.Background(), url, "agents")
	want, ok := Embedded("agents")
	if !ok {
		t.Fatal("Embedded(agents) not found")
	}
	if got != want {
		t.Errorf("FetchWithFallback = %q, want embedded snapshot", got)
	}
}

func TestFetchWithFallbackUnknownKeyReturnsFallbackCopy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer srv.Close()

	got := FetchWithFallback(context.Background(), srv.URL, "does-not-exist")
	if got != FallbackCopy() {
		t.Errorf("FetchWithFallback = %q, want FallbackCopy %q", got, FallbackCopy())
	}
}
