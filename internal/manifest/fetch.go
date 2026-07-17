package manifest

import "context"

// Fetch retrieves and parses the remote manifest at url. The version argument
// pins the manifest to a specific release: it is passed through as a "?version="
// query parameter on the request URL so that clients can request a stable,
// reproducible manifest. Fetch is a stub returning ErrNotImplemented; the HTTP
// retrieval and pinning behavior are implemented in a later phase.
func Fetch(ctx context.Context, url, version string) (*Manifest, error) {
	return nil, ErrNotImplemented
}
