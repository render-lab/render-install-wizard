// Package claudecode implements the tools.Target contract for Claude Code.
package claudecode

import (
	"context"

	"github.com/render-oss/render-install-wizard/internal/ids"
	"github.com/render-oss/render-install-wizard/internal/tools"
)

// Tool configures the Render components into Claude Code.
type Tool struct{}

// New returns a new Claude Code tool target.
func New() *Tool { return &Tool{} }

// ID returns the canonical identifier for Claude Code.
func (t *Tool) ID() ids.ToolID { return ids.ToolClaudeCode }

// Detect reports whether Claude Code is installed on the system.
func (t *Tool) Detect(ctx context.Context) (bool, error) { return false, nil }

// PreferredDelivery returns the delivery mechanism Claude Code prefers.
func (t *Tool) PreferredDelivery() ids.Delivery { return ids.DeliveryPlugin }

// Configure configures the selected components into Claude Code.
func (t *Tool) Configure(ctx context.Context, sel tools.Selection) error {
	return tools.ErrNotImplemented
}

// Unconfigure removes the wizard's configuration from Claude Code.
func (t *Tool) Unconfigure(ctx context.Context) error { return tools.ErrNotImplemented }

var _ tools.Target = (*Tool)(nil)
