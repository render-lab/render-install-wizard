// Package content fetches and renders the per-tool setup guides shown by the
// wizard. Copy is authored in Sanity and served from render.com/agents/*.md; the
// wizard is a client of it, with a git-versioned embedded snapshot as a safety
// net so it never blocks an install on a network fetch.
package content

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// fetchTimeout bounds a single content retrieval so the wizard never blocks
// longer than this on a slow or unreachable server.
const fetchTimeout = 10 * time.Second

// Fetch retrieves the markdown document at url. It issues a context-aware GET
// with an "Accept: text/markdown" header and a sane timeout, and returns the
// response body as a string. Any non-2xx status is reported as an error
// (including the status).
func Fetch(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("content: new request: %w", err)
	}
	req.Header.Set("Accept", "text/markdown")

	client := &http.Client{Timeout: fetchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("content: fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("content: fetch %s: unexpected status %s", url, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("content: read body: %w", err)
	}
	return string(body), nil
}

// FetchWithFallback resolves content best-effort and never returns an error. It
// tries a live Fetch first; on any failure it falls back to the embedded
// last-known-good snapshot for key, and if there is no snapshot for that key it
// returns the terse built-in FallbackCopy. This mirrors the wizard's resolution
// chain: live fetch → embedded snapshot → terse built-in copy.
func FetchWithFallback(ctx context.Context, url, key string) string {
	if md, err := Fetch(ctx, url); err == nil {
		return md
	}
	if md, ok := Embedded(key); ok {
		return md
	}
	return FallbackCopy()
}
