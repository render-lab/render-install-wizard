// Package opencode implements the tools.Target contract for OpenCode.
package opencode

import (
	"context"

	"github.com/render-oss/render-install-wizard/internal/ids"
	"github.com/render-oss/render-install-wizard/internal/tools"
)

// Tool configures the Render components into OpenCode.
type Tool struct{}

// New returns a new OpenCode tool target.
func New() *Tool { return &Tool{} }

// ID returns the canonical identifier for OpenCode.
func (t *Tool) ID() ids.ToolID { return ids.ToolOpenCode }

// Detect reports whether OpenCode is installed on the system.
func (t *Tool) Detect(ctx context.Context) (bool, error) { return false, nil }

// PreferredDelivery returns the delivery mechanism OpenCode prefers.
func (t *Tool) PreferredDelivery() ids.Delivery { return ids.DeliveryRaw }

// Configure configures the selected components into OpenCode.
func (t *Tool) Configure(ctx context.Context, sel tools.Selection) error {
	return tools.ErrNotImplemented
}

// Unconfigure removes the wizard's configuration from OpenCode.
func (t *Tool) Unconfigure(ctx context.Context) error { return tools.ErrNotImplemented }

var _ tools.Target = (*Tool)(nil)
