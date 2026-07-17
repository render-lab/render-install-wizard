// Package cli implements the components.Installer contract for the Render CLI.
package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/render-oss/render-install-wizard/internal/components"
	"github.com/render-oss/render-install-wizard/internal/ids"
	"github.com/render-oss/render-install-wizard/internal/render"
)

// Component installs and manages the Render CLI.
//
// All external effects are routed through injectable, unexported dependencies so
// that tests can exercise behavior without touching the real filesystem home or
// shelling out to brew/curl/render.
type Component struct {
	// home is the user's home directory. The Render CLI is looked for (and
	// removed from) <home>/.render/bin/render. Injectable so tests can point at
	// a t.TempDir() instead of the real home.
	home string
	// lookPath resolves an executable name to a path, like exec.LookPath.
	lookPath func(string) (string, error)
	// run executes a command and returns only its error, discarding output.
	run func(ctx context.Context, name string, args ...string) error
	// runOutput executes a command and returns its combined output plus error.
	runOutput func(ctx context.Context, name string, args ...string) (string, error)
}

// New returns a new Render CLI component wired with production defaults: the
// real user home, exec.LookPath for resolution, and exec.CommandContext-based
// runners for command execution.
func New() *Component {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return &Component{
		home:     home,
		lookPath: exec.LookPath,
		run: func(ctx context.Context, name string, args ...string) error {
			return exec.CommandContext(ctx, name, args...).Run()
		},
		runOutput: func(ctx context.Context, name string, args ...string) (string, error) {
			var buf bytes.Buffer
			cmd := exec.CommandContext(ctx, name, args...)
			cmd.Stdout = &buf
			cmd.Stderr = &buf
			err := cmd.Run()
			return buf.String(), err
		},
	}
}

// ID returns the canonical identifier for the Render CLI component.
func (c *Component) ID() ids.ComponentID { return ids.ComponentCLI }

// binPath returns the path to the CLI binary managed under the injectable home:
// <home>/.render/bin/render.
func (c *Component) binPath() string {
	return filepath.Join(c.home, ".render", "bin", render.CLIBinaryName)
}

// Detect reports whether the Render CLI is already installed. It returns true if
// the binary resolves on PATH via lookPath, or if the managed binary exists at
// <home>/.render/bin/render.
func (c *Component) Detect(ctx context.Context) (bool, error) {
	if c.lookPath != nil {
		if _, err := c.lookPath(render.CLIBinaryName); err == nil {
			return true, nil
		}
	}
	if info, err := os.Stat(c.binPath()); err == nil && !info.IsDir() {
		return true, nil
	}
	return false, nil
}

// Status returns the current status of the Render CLI component. When detected,
// it attempts to capture the version via `render --version`; if that fails the
// state is still reported as installed with an explanatory detail.
func (c *Component) Status(ctx context.Context) (components.Status, error) {
	detected, err := c.Detect(ctx)
	if err != nil {
		return components.Status{ID: ids.ComponentCLI, State: components.StateUnknown}, err
	}
	if !detected {
		return components.Status{ID: ids.ComponentCLI, State: components.StateNotInstalled}, nil
	}

	status := components.Status{ID: ids.ComponentCLI, State: components.StateInstalled}
	if c.runOutput != nil {
		out, verErr := c.runOutput(ctx, render.CLIBinaryName, "--version")
		if verErr != nil {
			status.Detail = "installed; version could not be determined"
		} else {
			status.Version = strings.TrimSpace(out)
		}
	}
	return status, nil
}

// Install installs the Render CLI according to opts. A dry run performs no side
// effects. Otherwise it prefers Homebrew when available (`brew install render`)
// and falls back to piping the official install script through sh. Checksum
// verification is intentionally delegated to the official installer.
func (c *Component) Install(ctx context.Context, opts components.Options) error {
	if opts.DryRun {
		return nil
	}
	if c.lookPath != nil {
		if _, err := c.lookPath("brew"); err == nil {
			if err := c.run(ctx, "brew", "install", render.CLIBinaryName); err != nil {
				return fmt.Errorf("install render CLI via brew: %w", err)
			}
			return nil
		}
	}
	script := fmt.Sprintf("curl -fsSL %s | sh", render.CLIInstallScriptURL)
	if err := c.run(ctx, "sh", "-c", script); err != nil {
		return fmt.Errorf("install render CLI via official installer: %w", err)
	}
	return nil
}

// Uninstall removes the CLI best-effort. It deletes the managed binary at
// <home>/.render/bin/render when present and returns nil when there is nothing
// to do. A Homebrew-installed CLI is not removed here; uninstall it with
// `brew uninstall render`.
func (c *Component) Uninstall(ctx context.Context) error {
	path := c.binPath()
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat render CLI at %s: %w", path, err)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove render CLI at %s: %w", path, err)
	}
	return nil
}

var _ components.Installer = (*Component)(nil)
