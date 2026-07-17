// Package mcp implements the components.Installer contract for MCP configuration.
package mcp

import (
	"context"

	"github.com/render-oss/render-install-wizard/internal/components"
	"github.com/render-oss/render-install-wizard/internal/ids"
)

// Component installs and manages the Render MCP configuration.
type Component struct{}

// New returns a new MCP component.
func New() *Component { return &Component{} }

// ID returns the canonical identifier for the MCP component.
func (c *Component) ID() ids.ComponentID { return ids.ComponentMCP }

// Detect reports whether the MCP configuration is already installed.
func (c *Component) Detect(ctx context.Context) (bool, error) { return false, nil }

// Install installs the MCP configuration according to opts.
func (c *Component) Install(ctx context.Context, opts components.Options) error {
	return components.ErrNotImplemented
}

// Uninstall removes the MCP configuration.
func (c *Component) Uninstall(ctx context.Context) error { return components.ErrNotImplemented }

// Status returns the current status of the MCP component.
func (c *Component) Status(ctx context.Context) (components.Status, error) {
	return components.Status{ID: ids.ComponentMCP, State: components.StateUnknown}, nil
}

var _ components.Installer = (*Component)(nil)
