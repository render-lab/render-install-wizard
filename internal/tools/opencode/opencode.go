// Package opencode implements the tools.Target contract for OpenCode.
//
// OpenCode stores its config (including MCP servers) in a JSON file at
// ~/.config/opencode/opencode.json. The wizard writes (merge-not-clobber) a
// single "render" entry under the "mcp" key via internal/configedit so that any
// user-defined MCP servers and unrelated keys are preserved.
package opencode

import (
	"context"
	"encoding/json"
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

// Configure configures the selected components into OpenCode.
//
// When the selection does not include the MCP component this is a no-op (skills
// are handled globally, not per-tool here). Otherwise it merges the Render MCP
// entry into ~/.config/opencode/opencode.json without clobbering existing
// servers.
func (t *Tool) Configure(ctx context.Context, sel tools.Selection) error {
	if !slices.Contains(sel.Components, ids.ComponentMCP) {
		return nil
	}
	path, ok := render.MCPConfigPath(t.ID(), t.home)
	if !ok {
		return fmt.Errorf("opencode: no MCP config path for home %q", t.home)
	}
	patch, err := t.mcpPatch()
	if err != nil {
		return fmt.Errorf("opencode: build MCP patch: %w", err)
	}
	if err := configedit.MergeJSONFile(path, patch); err != nil {
		return fmt.Errorf("opencode: write MCP config: %w", err)
	}
	return nil
}

// Unconfigure removes the Render MCP entry from OpenCode's config, leaving any
// other MCP servers and unrelated keys intact. Removing from a missing file is
// a no-op.
func (t *Tool) Unconfigure(ctx context.Context) error {
	path, ok := render.MCPConfigPath(t.ID(), t.home)
	if !ok {
		return fmt.Errorf("opencode: no MCP config path for home %q", t.home)
	}
	if err := configedit.DeleteJSONPath(path, "mcp", render.MCPServerName); err != nil {
		return fmt.Errorf("opencode: remove MCP config: %w", err)
	}
	return nil
}

// mcpPatch builds the JSON merge patch for OpenCode's opencode.json:
//
//	{"$schema":"https://opencode.ai/config.json",
//	 "mcp":{"render":{"type":"remote","url":<MCPServerURL>,"enabled":true}}}
//
// In API-key auth mode a "headers" object with an Authorization Bearer header is
// added under the render entry. OpenCode uses the "{env:VAR}" interpolation form
// (not the shell "$VAR" form from render.AuthorizationHeader), so the reference
// is built from render.APIKeyEnvVar directly; it is an env-ref, never a stored
// secret.
func (t *Tool) mcpPatch() ([]byte, error) {
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
	patch := map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"mcp": map[string]any{
			render.MCPServerName: entry,
		},
	}
	return json.Marshal(patch)
}

var _ tools.Target = (*Tool)(nil)
