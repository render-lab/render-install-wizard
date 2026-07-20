// Package cursor implements the tools.Target contract for Cursor.
//
// Cursor stores its MCP servers in a JSON file at ~/.cursor/mcp.json. The wizard
// writes (merge-not-clobber) a single "render" entry there via internal/configedit
// so that any user-defined MCP servers and unrelated keys are preserved.
package cursor

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

// Tool configures the Render components into Cursor.
//
// home and auth are unexported so tests can inject a t.TempDir() home and an
// explicit auth mode without ever touching the real user home directory.
type Tool struct {
	home string
	auth render.AuthMode
}

// New returns a new Cursor tool target defaulting to the real user home
// directory and the package-wide default auth mode. A failure to resolve the
// home directory leaves home empty; Configure/Unconfigure then surface the error
// from render.MCPConfigPath resolution paths.
func New() *Tool {
	home, _ := os.UserHomeDir()
	return &Tool{home: home, auth: render.DefaultAuthMode}
}

// ID returns the canonical identifier for Cursor.
func (t *Tool) ID() ids.ToolID { return ids.ToolCursor }

// Detect reports whether Cursor appears installed by checking for the
// ~/.cursor marker directory under home.
//
// This is a minimal self-check only; internal/detect is the authority for tool
// detection across the wizard.
func (t *Tool) Detect(ctx context.Context) (bool, error) {
	if t.home == "" {
		return false, nil
	}
	info, err := os.Stat(filepath.Join(t.home, ".cursor"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("cursor: detect: %w", err)
	}
	return info.IsDir(), nil
}

// PreferredDelivery returns the delivery mechanism Cursor prefers.
//
// The wizard always installs via the config-file path (raw), so this is
// DeliveryRaw. Cursor's Render plugin is surfaced as recommended next-step copy
// elsewhere rather than being written here.
func (t *Tool) PreferredDelivery() ids.Delivery { return ids.DeliveryRaw }

// Configure configures the selected components into Cursor.
//
// When the selection does not include the MCP component this is a no-op (skills
// are handled globally, not per-tool here). Otherwise it merges the Render MCP
// entry into ~/.cursor/mcp.json without clobbering existing servers.
func (t *Tool) Configure(ctx context.Context, sel tools.Selection) error {
	if !slices.Contains(sel.Components, ids.ComponentMCP) {
		return nil
	}
	path, ok := render.MCPConfigPath(t.ID(), t.home)
	if !ok {
		return fmt.Errorf("cursor: no MCP config path for home %q", t.home)
	}
	if err := configedit.SetJSONValue(path, t.mcpEntry(), "mcpServers", render.MCPServerName); err != nil {
		return fmt.Errorf("cursor: write MCP config: %w", err)
	}
	return nil
}

// Unconfigure removes the Render MCP entry from Cursor's config, leaving any
// other MCP servers and unrelated keys intact. Removing from a missing file is
// a no-op.
func (t *Tool) Unconfigure(ctx context.Context) error {
	path, ok := render.MCPConfigPath(t.ID(), t.home)
	if !ok {
		return fmt.Errorf("cursor: no MCP config path for home %q", t.home)
	}
	if err := configedit.DeleteJSONPath(path, "mcpServers", render.MCPServerName); err != nil {
		return fmt.Errorf("cursor: remove MCP config: %w", err)
	}
	return nil
}

// mcpEntry builds the Render MCP server entry written to mcpServers.render in
// Cursor's ~/.cursor/mcp.json:
//
//	{"type":"http","url":<MCPServerURL>}
//
// The entry is written wholesale (replacing any prior render entry), so
// switching auth modes never leaves a stale Authorization header behind. In
// API-key auth mode a "headers" object with an Authorization Bearer env-ref
// (never a stored secret) is included.
func (t *Tool) mcpEntry() map[string]any {
	entry := map[string]any{
		"type": "http",
		"url":  render.MCPServerURL,
	}
	if value, present := render.AuthorizationHeader(t.auth); present {
		entry["headers"] = map[string]any{
			"Authorization": value,
		}
	}
	return entry
}

var _ tools.Target = (*Tool)(nil)
