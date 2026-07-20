// Package opencode implements the tools.Target contract for OpenCode.
//
// OpenCode stores its config (including MCP servers) in ~/.config/opencode/ as
// either opencode.json or opencode.jsonc (JSONC: comments + trailing commas).
// OpenCode loads both and lets .jsonc override .json, so the wizard edits the
// .jsonc form when it exists and otherwise .json. It writes a single "render"
// entry under the "mcp" key via internal/configedit, which edits the file
// surgically so user-defined MCP servers, unrelated keys, comments, and
// formatting are all preserved.
package opencode

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/render-oss/render-install-wizard/internal/configedit"
	"github.com/render-oss/render-install-wizard/internal/ids"
	"github.com/render-oss/render-install-wizard/internal/render"
	"github.com/render-oss/render-install-wizard/internal/tools"
)

// Tool configures the Render components into OpenCode.
//
// home and auth are unexported so tests can inject a t.TempDir() home and an
// explicit auth mode without ever touching the real user home directory.
type Tool struct {
	home string
	auth render.AuthMode
}

// New returns a new OpenCode tool target defaulting to the real user home
// directory and the package-wide default auth mode. A failure to resolve the
// home directory leaves home empty; Configure/Unconfigure then surface the error
// from render.MCPConfigPath resolution paths.
func New() *Tool {
	home, _ := os.UserHomeDir()
	return &Tool{home: home, auth: render.DefaultAuthMode}
}

// ID returns the canonical identifier for OpenCode.
func (t *Tool) ID() ids.ToolID { return ids.ToolOpenCode }

// Detect reports whether OpenCode appears installed by checking for the
// ~/.config/opencode marker directory under home.
//
// This is a minimal self-check only; internal/detect is the authority for tool
// detection across the wizard.
func (t *Tool) Detect(ctx context.Context) (bool, error) {
	if t.home == "" {
		return false, nil
	}
	info, err := os.Stat(filepath.Join(t.home, ".config", "opencode"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("opencode: detect: %w", err)
	}
	return info.IsDir(), nil
}

// PreferredDelivery returns the delivery mechanism OpenCode prefers.
//
// The wizard always installs via the config-file path (raw), so this is
// DeliveryRaw. OpenCode's Render plugin is surfaced as recommended next-step
// copy elsewhere rather than being written here.
func (t *Tool) PreferredDelivery() ids.Delivery { return ids.DeliveryRaw }

// configPath resolves the OpenCode config file to edit: the highest-precedence
// existing file (opencode.jsonc, then opencode.json), or the .json form to
// create when neither exists (OpenCode reads both; .json needs no JSONC
// features). The bool reports whether the resolved file already exists.
func (t *Tool) configPath() (path string, exists bool) {
	candidates := render.OpenCodeConfigFiles(t.home)
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c, true
		}
	}
	return candidates[len(candidates)-1], false
}

// Configure configures the selected components into OpenCode.
//
// When the selection does not include the MCP component this is a no-op (skills
// are handled globally, not per-tool here). Otherwise it writes the Render MCP
// entry into the active config file (opencode.jsonc if present, else
// opencode.json), replacing any prior render entry and preserving other servers,
// unrelated keys, and comments/formatting.
func (t *Tool) Configure(ctx context.Context, sel tools.Selection) error {
	if !slices.Contains(sel.Components, ids.ComponentMCP) {
		return nil
	}
	if t.home == "" {
		return fmt.Errorf("opencode: cannot resolve config path for empty home")
	}
	path, exists := t.configPath()
	// Seed OpenCode's $schema only when creating a brand-new file; never modify
	// an existing file's $schema (the user or OpenCode owns that value).
	if !exists {
		if err := configedit.SetJSONValue(path, render.OpenCodeSchemaURL, "$schema"); err != nil {
			return fmt.Errorf("opencode: initialize config %s: %w", path, err)
		}
	}
	if err := configedit.SetJSONValue(path, t.mcpEntry(), "mcp", render.MCPServerName); err != nil {
		return fmt.Errorf("opencode: write MCP config: %w", err)
	}
	return nil
}

// Unconfigure removes the Render MCP entry from OpenCode's config, leaving any
// other MCP servers, unrelated keys, and comments intact. It removes the entry
// from every existing config file (both .json and .jsonc) so no shadowed copy is
// left behind; a missing file is a no-op.
func (t *Tool) Unconfigure(ctx context.Context) error {
	if t.home == "" {
		return fmt.Errorf("opencode: cannot resolve config path for empty home")
	}
	for _, path := range render.OpenCodeConfigFiles(t.home) {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := configedit.DeleteJSONPath(path, "mcp", render.MCPServerName); err != nil {
			return fmt.Errorf("opencode: remove MCP config from %s: %w", path, err)
		}
	}
	return nil
}

// mcpEntry builds the Render MCP server entry written to mcp.render:
//
//	{"type":"remote","url":<MCPServerURL>,"enabled":true}
//
// It is written wholesale (replacing any prior render entry). In API-key auth
// mode a "headers" object with an Authorization Bearer header is added; OpenCode
// uses the "{env:VAR}" interpolation form (not the shell "$VAR" form), so the
// reference is built from render.APIKeyEnvVar directly — an env-ref, never a
// stored secret.
func (t *Tool) mcpEntry() map[string]any {
	entry := map[string]any{
		"type":    "remote",
		"url":     render.MCPServerURL,
		"enabled": true,
	}
	if t.auth == render.AuthModeAPIKey {
		entry["headers"] = map[string]any{
			"Authorization": fmt.Sprintf("Bearer {env:%s}", render.APIKeyEnvVar),
		}
	}
	return entry
}

var _ tools.Target = (*Tool)(nil)
