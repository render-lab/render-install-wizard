// Package claudecode implements the tools.Target contract for Claude Code,
// writing Render's MCP server entry into ~/.claude.json (JSON) via the
// merge-not-clobber engine in internal/configedit.
package claudecode

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/render-oss/render-install-wizard/internal/configedit"
	"github.com/render-oss/render-install-wizard/internal/ids"
	"github.com/render-oss/render-install-wizard/internal/render"
	"github.com/render-oss/render-install-wizard/internal/tools"
)

// Tool configures the Render components into Claude Code.
//
// home and auth are injectable so tests never touch the real home directory or
// depend on the process-wide default auth mode; New applies production defaults.
type Tool struct {
	home string
	auth render.AuthMode
}

// New returns a new Claude Code tool target defaulting home to the user's home
// directory and auth to render.DefaultAuthMode. A failure to resolve the home
// directory leaves home empty; Configure/Detect surface that as an error rather
// than falling back to an unexpected location.
func New() *Tool {
	home, _ := os.UserHomeDir()
	return &Tool{home: home, auth: render.DefaultAuthMode}
}

// ID returns the canonical identifier for Claude Code.
func (t *Tool) ID() ids.ToolID { return ids.ToolClaudeCode }

// Detect reports whether Claude Code is installed on the system with a
// lightweight marker check: it looks for <home>/.claude or <home>/.claude.json.
// internal/detect is the authoritative detector; this is a best-effort probe.
func (t *Tool) Detect(ctx context.Context) (bool, error) {
	if t.home == "" {
		return false, nil
	}
	for _, marker := range []string{
		filepath.Join(t.home, ".claude"),
		filepath.Join(t.home, ".claude.json"),
	} {
		if _, err := os.Stat(marker); err == nil {
			return true, nil
		} else if !os.IsNotExist(err) {
			return false, fmt.Errorf("claudecode: stat %s: %w", marker, err)
		}
	}
	return false, nil
}

// PreferredDelivery returns the delivery mechanism Claude Code prefers.
//
// The wizard always writes Render's MCP entry through the config-file path
// (~/.claude.json), so raw delivery is preferred. This supersedes the earlier
// "plugin" framing: any recommended plugin is surfaced as next-steps elsewhere,
// not driven from here.
func (t *Tool) PreferredDelivery() ids.Delivery { return ids.DeliveryRaw }

// Configure configures the selected components into Claude Code.
//
// It only acts on the MCP component: skills are handled globally by the official
// skills installer, so a Selection without ids.ComponentMCP is a no-op that
// creates no file. When MCP is selected it merges Render's mcpServers.render
// entry into ~/.claude.json, preserving any unrelated configuration.
func (t *Tool) Configure(ctx context.Context, sel tools.Selection) error {
	if !hasComponent(sel, ids.ComponentMCP) {
		return nil
	}
	path, ok := render.MCPConfigPath(t.ID(), t.home)
	if !ok {
		return fmt.Errorf("claudecode: no MCP config path for home %q", t.home)
	}
	patch, err := t.mcpPatch()
	if err != nil {
		return err
	}
	if err := configedit.MergeJSONFile(path, patch); err != nil {
		return fmt.Errorf("claudecode: merge MCP config: %w", err)
	}
	return nil
}

// Unconfigure removes the wizard's configuration from Claude Code by deleting
// only the mcpServers.render entry. Sibling MCP servers and unrelated keys are
// preserved, and a missing config file is a no-op.
func (t *Tool) Unconfigure(ctx context.Context) error {
	path, ok := render.MCPConfigPath(t.ID(), t.home)
	if !ok {
		return fmt.Errorf("claudecode: no MCP config path for home %q", t.home)
	}
	if err := configedit.DeleteJSONPath(path, "mcpServers", render.MCPServerName); err != nil {
		return fmt.Errorf("claudecode: delete MCP config: %w", err)
	}
	return nil
}

// mcpPatch builds the JSON merge patch that adds Render's MCP server entry.
// OAuth writes a credential-free URL-only entry; API-key mode adds an
// Authorization header referencing an environment variable (never a secret).
func (t *Tool) mcpPatch() ([]byte, error) {
	entry := map[string]any{
		"type": "http",
		"url":  render.MCPServerURL,
	}
	if value, present := render.AuthorizationHeader(t.auth); present {
		entry["headers"] = map[string]any{"Authorization": value}
	}
	patch := map[string]any{
		"mcpServers": map[string]any{
			render.MCPServerName: entry,
		},
	}
	out, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("claudecode: marshal MCP patch: %w", err)
	}
	return out, nil
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
