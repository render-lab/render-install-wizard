// Package components defines the contract for installable components — the "what"
// the wizard installs (the Render CLI, agent skills, and MCP configuration).
//
// Concrete implementations live in subpackages (cli, skills, mcp) and satisfy
// the Installer interface. This package only declares the frozen interface and
// shared value types; it contains no installation logic.
package components

import (
	"context"
	"errors"

	"github.com/render-oss/render-install-wizard/internal/ids"
)

// ErrNotImplemented is returned by stubbed methods that have no behavior yet.
var ErrNotImplemented = errors.New("not implemented")

// State describes whether a component is currently installed.
type State string

// Component installation states.
const (
	// StateInstalled indicates the component is present and usable.
	StateInstalled State = "installed"
	// StateNotInstalled indicates the component is absent.
	StateNotInstalled State = "not_installed"
	// StateUnknown indicates the component's state could not be determined.
	StateUnknown State = "unknown"
)

// Status reports the observed state of a component.
type Status struct {
	// ID identifies which component this status describes.
	ID ids.ComponentID
	// State is the observed installation state.
	State State
	// Version is the installed version, if known and applicable.
	Version string
	// Detail carries an optional human-readable note about the status.
	Detail string
}

// Options controls how a component is installed.
type Options struct {
	// DryRun, when true, plans the install without performing side effects.
	DryRun bool
	// Version pins a specific version to install; an empty string means latest.
	Version string
}

// Installer is the contract every component implementation must satisfy.
type Installer interface {
	// ID returns the canonical identifier for this component.
	ID() ids.ComponentID
	// Detect reports whether the component is already installed.
	Detect(ctx context.Context) (bool, error)
	// Install installs the component according to opts.
	Install(ctx context.Context, opts Options) error
	// Uninstall removes the component.
	Uninstall(ctx context.Context) error
	// Status returns the current status of the component.
	Status(ctx context.Context) (Status, error)
}
