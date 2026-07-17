// Package cli implements the components.Installer contract for the Render CLI.
package cli

import (
	"context"

	"github.com/render-oss/render-install-wizard/internal/components"
	"github.com/render-oss/render-install-wizard/internal/ids"
)

// Component installs and manages the Render CLI.
type Component struct{}

// New returns a new Render CLI component.
func New() *Component { return &Component{} }

// ID returns the canonical identifier for the Render CLI component.
func (c *Component) ID() ids.ComponentID { return ids.ComponentCLI }

// Detect reports whether the Render CLI is already installed.
func (c *Component) Detect(ctx context.Context) (bool, error) { return false, nil }

// Install installs the Render CLI according to opts.
func (c *Component) Install(ctx context.Context, opts components.Options) error {
	return components.ErrNotImplemented
}

// Uninstall removes the Render CLI.
func (c *Component) Uninstall(ctx context.Context) error { return components.ErrNotImplemented }

// Status returns the current status of the Render CLI component.
func (c *Component) Status(ctx context.Context) (components.Status, error) {
	return components.Status{ID: ids.ComponentCLI, State: components.StateUnknown}, nil
}

var _ components.Installer = (*Component)(nil)
