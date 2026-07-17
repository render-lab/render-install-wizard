// Package wizard drives the interactive install experience. Phase 0 provides
// plain stubs with no external dependencies.
package wizard

import (
	"context"
	"errors"
)

// ErrNotImplemented is returned by operations that are not yet implemented.
var ErrNotImplemented = errors.New("wizard: not implemented")

// Model holds the interactive wizard state.
type Model struct{}

// New returns a new wizard Model.
func New() *Model {
	return &Model{}
}

// Run executes the interactive wizard. It is currently a stub.
func (m *Model) Run(ctx context.Context) error {
	_ = ctx
	// TODO(phase 1E): replace with a bubbletea model and event loop.
	return ErrNotImplemented
}
