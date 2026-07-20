// Package codex implements the tools.Target contract for Codex, writing
// Render's MCP server entry into ~/.codex/config.toml (TOML) via the
// merge-not-clobber engine in internal/configedit.
package codex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/render-oss/render-install-wizard/internal/configedit"
	"github.com/render-oss/render-install-wizard/internal/ids"
	"github.com/render-oss/render-install-wizard/internal/render"
	"github.com/render-oss/render-install-wizard/internal/tools"
)

// Tool configures the Render components into Codex.
//
// home and auth are injectable so tests never touch the real home directory or
// depend on the process-wide default auth mode; New applies production defaults.
type Tool struct {
	home string
	auth render.AuthMode
}

// New returns a new Codex tool target defaulting home to the user's home
// directory and auth to render.DefaultAuthMode. A failure to resolve the home
// directory leaves home empty; Configure/Detect surface that as an error rather
// than falling back to an unexpected location.
func New() *Tool {
	home, _ := os.UserHomeDir()
	return &Tool{home: home, auth: render.DefaultAuthMode}
}

// ID returns the canonical identifier for Codex.
func (t *Tool) ID() ids.ToolID { return ids.ToolCodex }

// Detect reports whether Codex is installed on the system with a lightweight
// marker check: it looks for the <home>/.codex directory. internal/detect is
// the authoritative detector; this is a best-effort probe.
func (t *Tool) Detect(ctx context.Context) (bool, error) {
	if t.home == "" {
		return false, nil
	}
	marker := filepath.Join(t.home, ".codex")
	if _, err := os.Stat(marker); err == nil {
		return true, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("codex: stat %s: %w", marker, err)
	}
	return false, nil
}

// PreferredDelivery returns the delivery mechanism Codex prefers.
//
// The wizard always writes Render's MCP entry through the config-file path
// (~/.codex/config.toml), so raw delivery is preferred. This supersedes the
// earlier "plugin" framing: the Codex Render plugin is installed in-app and is
// surfaced as next-steps elsewhere, not driven from here.
func (t *Tool) PreferredDelivery() ids.Delivery { return ids.DeliveryRaw }

// Configure configures the selected components into Codex.
//
// It only acts on the MCP component: skills are handled globally by the official
// skills installer, so a Selection without ids.ComponentMCP is a no-op that
// creates no file. When MCP is selected it writes Render's [mcp_servers.render]
// table into ~/.codex/config.toml, preserving any unrelated configuration.
func (t *Tool) Configure(ctx context.Context, sel tools.Selection) error {
	if !hasComponent(sel, ids.ComponentMCP) {
		return nil
	}
	path, ok := render.MCPConfigPath(t.ID(), t.home)
	if !ok {
		return fmt.Errorf("codex: no MCP config path for home %q", t.home)
	}
	if err := configedit.SetTOMLValue(path, t.mcpServer(), "mcp_servers", render.MCPServerName); err != nil {
		return fmt.Errorf("codex: write MCP config: %w", err)
	}
	return nil
}

// Unconfigure removes the wizard's configuration from Codex by deleting only the
// mcp_servers.render table. Sibling MCP servers and unrelated keys are
// preserved, and a missing config file is a no-op.
func (t *Tool) Unconfigure(ctx context.Context) error {
	path, ok := render.MCPConfigPath(t.ID(), t.home)
	if !ok {
		return fmt.Errorf("codex: no MCP config path for home %q", t.home)
	}
	if err := configedit.DeleteTOMLPath(path, "mcp_servers", render.MCPServerName); err != nil {
		return fmt.Errorf("codex: delete MCP config: %w", err)
	}
	return nil
}

// mcpServer builds the Render [mcp_servers.render] table value. It is written
// wholesale (replacing any prior render table), so switching auth modes never
// leaves a stale http_headers Authorization behind. OAuth writes a
// credential-free URL-only table; API-key mode adds an http_headers
// Authorization referencing an environment variable (never a secret).
func (t *Tool) mcpServer() map[string]any {
	server := map[string]any{
		"url": render.MCPServerURL,
	}
	if value, present := render.AuthorizationHeader(t.auth); present {
		server["http_headers"] = map[string]any{"Authorization": value}
	}
	return server
}

// hasComponent reports whether the selection includes the given component.
func hasComponent(sel tools.Selection, want ids.ComponentID) bool {
	for _, c := range sel.Components {
		if c == want {
			return true
		}
	}
	return false
}

var _ tools.Target = (*Tool)(nil)
