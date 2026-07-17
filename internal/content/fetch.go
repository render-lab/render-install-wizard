// Package content fetches and renders the per-tool setup guides shown by the
// wizard.
package content

import (
	"context"
	"errors"
)

// ErrNotImplemented is returned by operations that are not yet implemented.
var ErrNotImplemented = errors.New("content: not implemented")

// Fetch retrieves the content at the given URL. The intended resolution chain is
// live fetch → embedded snapshot → terse built-in fallback copy. It is currently
// a stub.
func Fetch(ctx context.Context, url string) (string, error) {
	_ = ctx
	_ = url
	// TODO(phase 1D): implement live fetch with embedded/built-in fallback.
	return "", ErrNotImplemented
}
