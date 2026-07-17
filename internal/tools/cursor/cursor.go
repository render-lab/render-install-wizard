// Package cursor implements the tools.Target contract for Cursor.
package cursor

import (
	"context"

	"github.com/render-oss/render-install-wizard/internal/ids"
	"github.com/render-oss/render-install-wizard/internal/tools"
)

// Tool configures the Render components into Cursor.
type Tool struct{}

// New returns a new Cursor tool target.
func New() *Tool { return &Tool{} }

// ID returns the canonical identifier for Cursor.
func (t *Tool) ID() ids.ToolID { return ids.ToolCursor }

// Detect reports whether Cursor is installed on the system.
func (t *Tool) Detect(ctx context.Context) (bool, error) { return false, nil }

// PreferredDelivery returns the delivery mechanism Cursor prefers.
func (t *Tool) PreferredDelivery() ids.Delivery { return ids.DeliveryRaw }

// Configure configures the selected components into Cursor.
func (t *Tool) Configure(ctx context.Context, sel tools.Selection) error {
	return tools.ErrNotImplemented
}

// Unconfigure removes the wizard's configuration from Cursor.
func (t *Tool) Unconfigure(ctx context.Context) error { return tools.ErrNotImplemented }

var _ tools.Target = (*Tool)(nil)
