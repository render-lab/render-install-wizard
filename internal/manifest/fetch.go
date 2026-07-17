package manifest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// fetchTimeout bounds a single manifest retrieval so the wizard never blocks
// indefinitely on a slow or unreachable server.
const fetchTimeout = 10 * time.Second

// Fetch retrieves and parses the remote manifest at rawURL. The version argument
// pins the manifest to a specific release: when non-empty it is attached as a
// "version" query parameter (preserving any query already present in rawURL) so
// clients can request a stable, reproducible manifest.
//
// It issues a context-aware GET with a sane timeout, treats any non-2xx status
// as an error (including the status in the message), and delegates schema
// validation to Parse.
func Fetch(ctx context.Context, rawURL, version string) (*Manifest, error) {
	reqURL, err := manifestURL(rawURL, version)
	if err != nil {
		return nil, fmt.Errorf("manifest: build url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("manifest: new request: %w", err)
	}

	client := &http.Client{Timeout: fetchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("manifest: fetch %s: %w", reqURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("manifest: fetch %s: unexpected status %s", reqURL, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("manifest: read body: %w", err)
	}

	return Parse(body)
}

// manifestURL returns rawURL with the version pin applied as a "version" query
// parameter when version is non-empty, preserving any existing query values.
func manifestURL(rawURL, version string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if version != "" {
		q := u.Query()
		q.Set("version", version)
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}
