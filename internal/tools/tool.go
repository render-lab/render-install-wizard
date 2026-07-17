// Package tools defines the contract for configuring coding agents — the "where"
// the wizard installs components into (Claude Code, Cursor, Codex, OpenCode).
//
// Concrete implementations live in subpackages (claudecode, cursor, codex,
// opencode) and satisfy the Target interface. This package only declares the
// frozen interface and shared value types; it contains no configuration logic.
package tools

import (
	"context"
	"errors"

	"github.com/render-oss/render-install-wizard/internal/ids"
)

// ErrNotImplemented is returned by stubbed methods that have no behavior yet.
var ErrNotImplemented = errors.New("not implemented")

// Selection describes which components to configure into a tool.
type Selection struct {
	// Components lists the component identifiers to configure into the tool.
	Components []ids.ComponentID
}

// Target is the contract every tool implementation must satisfy.
type Target interface {
	// ID returns the canonical identifier for this tool.
	ID() ids.ToolID
	// Detect reports whether the tool is installed on the system.
	Detect(ctx context.Context) (bool, error)
	// PreferredDelivery returns the delivery mechanism this tool prefers.
	PreferredDelivery() ids.Delivery
	// Configure configures the selected components into the tool.
	Configure(ctx context.Context, sel Selection) error
	// Unconfigure removes the wizard's configuration from the tool.
	Unconfigure(ctx context.Context) error
}
