// Package codex implements the tools.Target contract for Codex.
package codex

import (
	"context"

	"github.com/render-oss/render-install-wizard/internal/ids"
	"github.com/render-oss/render-install-wizard/internal/tools"
)

// Tool configures the Render components into Codex.
type Tool struct{}

// New returns a new Codex tool target.
func New() *Tool { return &Tool{} }

// ID returns the canonical identifier for Codex.
func (t *Tool) ID() ids.ToolID { return ids.ToolCodex }

// Detect reports whether Codex is installed on the system.
func (t *Tool) Detect(ctx context.Context) (bool, error) { return false, nil }

// PreferredDelivery returns the delivery mechanism Codex prefers.
func (t *Tool) PreferredDelivery() ids.Delivery { return ids.DeliveryPlugin }

// Configure configures the selected components into Codex.
func (t *Tool) Configure(ctx context.Context, sel tools.Selection) error {
	return tools.ErrNotImplemented
}

// Unconfigure removes the wizard's configuration from Codex.
func (t *Tool) Unconfigure(ctx context.Context) error { return tools.ErrNotImplemented }

var _ tools.Target = (*Tool)(nil)
