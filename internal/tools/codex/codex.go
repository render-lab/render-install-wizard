// Package codex implements the tools.Target contract for Codex, writing
// Render's MCP server entry into ~/.codex/config.toml (TOML) via the
// merge-not-clobber engine in internal/configedit.
package codex

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/render-lab/render-install-wizard/internal/configedit"
	"github.com/render-lab/render-install-wizard/internal/execx"
	"github.com/render-lab/render-install-wizard/internal/ids"
	"github.com/render-lab/render-install-wizard/internal/render"
	"github.com/render-lab/render-install-wizard/internal/tools"
)

// cliTimeout bounds a delegated `codex mcp` invocation. The command edits a local
// file and should return promptly; the bound keeps a wedged CLI from stalling the
// run before the file-writing fallback gets its turn (F17).
const cliTimeout = 30 * time.Second

// rmcpClientKey is the root-level Codex setting that enables its remote
// (streamable-HTTP) MCP client. Codex reads a url-based [mcp_servers.*] entry only
// when this is true, and only when it appears before those tables — so the Render
// entry is inert without it.
const rmcpClientKey = "experimental_use_rmcp_client"

// Tool configures the Render components into Codex.
//
// home and auth are injectable so tests never touch the real home directory or
// depend on the process-wide default auth mode; New applies production defaults.
// lookPath/run are injectable so tests can exercise both the delegated `codex mcp`
// path and the file-writing fallback without a real Codex install.
type Tool struct {
	home     string
	auth     render.AuthMode
	lookPath func(string) (string, error)
	run      func(ctx context.Context, name string, args ...string) error
}

// New returns a new Codex tool target defaulting home to the user's home
// directory and auth to render.DefaultAuthMode. A failure to resolve the home
// directory leaves home empty; Configure/Detect surface that as an error rather
// than falling back to an unexpected location.
func New() *Tool {
	home, _ := os.UserHomeDir()
	return &Tool{
		home:     home,
		auth:     render.DefaultAuthMode,
		lookPath: exec.LookPath,
		run:      execx.Run,
	}
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
// creates no file.
//
// When MCP is selected it does two things. First it enables Codex's remote MCP
// client at the document root, because a url-based server entry is silently inert
// without it. Then it writes the [mcp_servers.render] table, preferring `codex mcp
// add` when the Codex CLI is installed: Codex's own editor preserves the comments
// and key ordering in a hand-maintained config.toml, which the map-based TOML
// fallback cannot. Any failure of the delegated command falls through to that
// fallback so a missing or changed CLI never blocks configuration.
func (t *Tool) Configure(ctx context.Context, sel tools.Selection) error {
	if !hasComponent(sel, ids.ComponentMCP) {
		return nil
	}
	path, ok := render.MCPConfigPath(t.ID(), t.home)
	if !ok {
		return fmt.Errorf("codex: no MCP config path for home %q", t.home)
	}

	// Inserted textually and at the top, so it survives a hand-edited file intact
	// and is guaranteed to precede every [mcp_servers.*] table.
	if err := configedit.EnsureTOMLRootKey(path, rmcpClientKey, "true"); err != nil {
		return fmt.Errorf("codex: enable remote MCP client: %w", err)
	}

	if t.addViaCLI(ctx) {
		return nil
	}
	if err := configedit.SetTOMLValue(path, t.mcpServer(), "mcp_servers", render.MCPServerName); err != nil {
		return fmt.Errorf("codex: write MCP config: %w", err)
	}
	return nil
}

// addViaCLI attempts to register the Render MCP server through `codex mcp add`,
// reporting whether it succeeded. A missing CLI or any non-zero exit reports false
// so the caller falls back to editing config.toml directly.
func (t *Tool) addViaCLI(ctx context.Context) bool {
	if t.lookPath == nil || t.run == nil {
		return false
	}
	if _, err := t.lookPath("codex"); err != nil {
		return false
	}
	args := []string{"mcp", "add", render.MCPServerName, "--url", render.MCPServerURL}
	if t.auth == render.AuthModeAPIKey {
		args = append(args, "--bearer-token-env-var", render.APIKeyEnvVar)
	}
	ctx, cancel := context.WithTimeout(ctx, cliTimeout)
	defer cancel()
	return t.run(ctx, "codex", args...) == nil
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
// leaves a stale credential field behind. OAuth writes a credential-free
// URL-only table.
//
// API-key mode sets bearer_token_env_var, which names the environment variable
// holding the token. It deliberately does not use http_headers: those values are
// static strings that Codex passes through verbatim, so a shell-style
// "Bearer $RENDER_API_KEY" would be sent literally rather than expanded. Codex
// also rejects an inline bearer_token outright, so the env-var indirection is
// both the working form and the one that keeps secrets off disk.
func (t *Tool) mcpServer() map[string]any {
	server := map[string]any{
		"url": render.MCPServerURL,
	}
	if t.auth == render.AuthModeAPIKey {
		server["bearer_token_env_var"] = render.APIKeyEnvVar
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
