// Package skills implements the components.Installer contract for agent skills.
package skills

import (
	"context"

	"github.com/render-oss/render-install-wizard/internal/components"
	"github.com/render-oss/render-install-wizard/internal/ids"
)

// Component installs and manages Render agent skills.
type Component struct{}

// New returns a new agent skills component.
func New() *Component { return &Component{} }

// ID returns the canonical identifier for the agent skills component.
func (c *Component) ID() ids.ComponentID { return ids.ComponentSkills }

// Detect reports whether agent skills are already installed.
func (c *Component) Detect(ctx context.Context) (bool, error) { return false, nil }

// Install installs agent skills according to opts.
func (c *Component) Install(ctx context.Context, opts components.Options) error {
	return components.ErrNotImplemented
}

// Uninstall removes agent skills.
func (c *Component) Uninstall(ctx context.Context) error { return components.ErrNotImplemented }

// Status returns the current status of the agent skills component.
func (c *Component) Status(ctx context.Context) (components.Status, error) {
	return components.Status{ID: ids.ComponentSkills, State: components.StateUnknown}, nil
}

var _ components.Installer = (*Component)(nil)
