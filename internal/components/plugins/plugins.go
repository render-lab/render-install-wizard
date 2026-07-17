// Package plugins implements the components.Installer contract for the plugin
// delivery mechanism. Plugins bundle skills and MCP config for tools that
// consume them (e.g. Claude Code, Codex); they are internal delivery machinery,
// not a user-selectable component, which is why there is no plugins ComponentID
// in the ids package.
package plugins

import (
	"context"

	"github.com/render-oss/render-install-wizard/internal/components"
	"github.com/render-oss/render-install-wizard/internal/ids"
)

// componentID is the synthesized identifier for the plugins delivery mechanism.
// Plugins are internal machinery, not a user-selectable component, so no
// ComponentID exists for them in the ids package; this is for bookkeeping only.
const componentID = ids.ComponentID("plugins")

// Component builds and manages plugin bundles used to deliver skills and MCP
// configuration to plugin-consuming tools.
type Component struct{}

// New returns a new plugins component.
func New() *Component { return &Component{} }

// ID returns the internal identifier for the plugins delivery mechanism.
func (c *Component) ID() ids.ComponentID { return componentID }

// Detect reports whether the plugin bundle is already installed.
func (c *Component) Detect(ctx context.Context) (bool, error) { return false, nil }

// Install builds and installs the plugin bundle according to opts.
func (c *Component) Install(ctx context.Context, opts components.Options) error {
	return components.ErrNotImplemented
}

// Uninstall removes the plugin bundle.
func (c *Component) Uninstall(ctx context.Context) error { return components.ErrNotImplemented }

// Status returns the current status of the plugins delivery mechanism.
func (c *Component) Status(ctx context.Context) (components.Status, error) {
	return components.Status{ID: componentID, State: components.StateUnknown}, nil
}

var _ components.Installer = (*Component)(nil)
