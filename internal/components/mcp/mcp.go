// Package mcp implements the components.Installer contract for MCP configuration.
//
// MCP entries are written per-tool by tools.Target.Configure (using the
// per-tool config paths in internal/render), not by this global component.
// Install and Uninstall are therefore no-ops here; this component exists to
// report aggregate detection/status across all tools.
package mcp

import (
	"context"
	"os"
	"strings"

	"github.com/render-lab/render-install-wizard/internal/components"
	"github.com/render-lab/render-install-wizard/internal/ids"
	"github.com/render-lab/render-install-wizard/internal/render"
)

// Component reports on the Render MCP configuration across tools. Writing MCP
// entries is handled per-tool during tool configuration, so this component does
// not itself write config files.
type Component struct {
	// home is the user's home directory. It is injectable so tests can point at
	// a temporary directory instead of the real home.
	home string
}

// New returns a new MCP component wired to the real OS home directory.
func New() *Component {
	home, _ := os.UserHomeDir()
	return &Component{home: home}
}

// ID returns the canonical identifier for the MCP component.
func (c *Component) ID() ids.ComponentID { return ids.ComponentMCP }

// Detect reports whether the Render MCP entry appears in any tool config.
//
// It is best-effort: it iterates the known tools, reads each tool's MCP config
// file (when one exists), and returns true if any file's contents mention the
// Render MCP server URL. A simple substring check is sufficient across the
// different config formats (JSON/TOML).
func (c *Component) Detect(ctx context.Context) (bool, error) {
	for _, tool := range ids.AllTools() {
		for _, path := range c.configPaths(tool) {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if strings.Contains(string(data), render.MCPServerURL) {
				return true, nil
			}
		}
	}
	return false, nil
}

// configPaths returns the candidate config file paths to inspect for a tool.
// OpenCode may store config in opencode.json or opencode.jsonc, so both are
// checked (matching the resolver the OpenCode writer uses).
func (c *Component) configPaths(tool ids.ToolID) []string {
	if tool == ids.ToolOpenCode {
		return render.OpenCodeConfigFiles(c.home)
	}
	if path, ok := render.MCPConfigPath(tool, c.home); ok {
		return []string{path}
	}
	return nil
}

// Install is a documented no-op: the Render MCP entry is written per-tool during
// tool configuration (tools.Target.Configure), not by this global component.
func (c *Component) Install(ctx context.Context, opts components.Options) error {
	return nil
}

// Uninstall is a documented no-op: per-tool Unconfigure removes MCP entries, so
// this global component has nothing of its own to remove.
func (c *Component) Uninstall(ctx context.Context) error {
	return nil
}

// Status returns the current status of the MCP component, derived from Detect.
func (c *Component) Status(ctx context.Context) (components.Status, error) {
	installed, err := c.Detect(ctx)
	if err != nil {
		return components.Status{ID: ids.ComponentMCP, State: components.StateUnknown}, err
	}
	state := components.StateNotInstalled
	if installed {
		state = components.StateInstalled
	}
	return components.Status{ID: ids.ComponentMCP, State: state}, nil
}

var _ components.Installer = (*Component)(nil)
