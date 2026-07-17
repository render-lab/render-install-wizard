// Package configedit merges Render entries into coding-agent configuration files
// without clobbering other servers or keys the user already has configured.
package configedit

import "errors"

// ErrNotImplemented is returned by operations that are not yet implemented.
var ErrNotImplemented = errors.New("configedit: not implemented")

// MergeJSON merges the given JSON patch into the config file at path, preserving
// unrelated keys. It is currently a stub.
func MergeJSON(path string, patch []byte) error {
	_ = path
	_ = patch
	// TODO(phase 2): parse, deep-merge, and atomically rewrite the target file.
	return ErrNotImplemented
}
