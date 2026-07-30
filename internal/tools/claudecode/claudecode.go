// Package claudecode implements the tools.Target contract for Claude Code,
// writing Render's MCP server entry into ~/.claude.json (JSON) via the
// merge-not-clobber engine in internal/configedit.
package claudecode

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"time"

	"github.com/render-lab/render-install-wizard/internal/configedit"
	"github.com/render-lab/render-install-wizard/internal/execx"
	"github.com/render-lab/render-install-wizard/internal/ids"
	"github.com/render-lab/render-install-wizard/internal/render"
	"github.com/render-lab/render-install-wizard/internal/tools"
)

// cliTimeout bounds a delegated `claude mcp` invocation. The command edits a
// local file and should return promptly; the bound keeps a wedged CLI from
// stalling the run before the file-writing fallback gets its turn (F17).
const cliTimeout = 30 * time.Second

// Tool configures the Render components into Claude Code.
//
// home and auth are injectable so tests never touch the real home directory or
// depend on the process-wide default auth mode; New applies production defaults.
// lookPath/run are injectable so tests can exercise both the delegated
// `claude mcp` path and the file-writing fallback without a real Claude install.
type Tool struct {
	home     string
	auth     render.AuthMode
	lookPath func(string) (string, error)
	run      func(ctx context.Context, name string, args ...string) error
}

// New returns a new Claude Code tool target defaulting home to the user's home
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
// creates no file.
//
// When MCP is selected it prefers `claude mcp add-json --scope user`, falling back
// to merging mcpServers.render into ~/.claude.json directly. Delegation is the
// better path here for a reason specific to this file: ~/.claude.json is Claude
// Code's live state (project history, onboarding and account state), not a static
// config. A running Claude Code holds its own in-memory copy and rewrites the
// whole file on exit, so an external read-modify-write can be silently reverted --
// the atomic rename prevents a torn file but not a lost update. Letting Claude
// perform the edit keeps the wizard out of that race entirely.
func (t *Tool) Configure(ctx context.Context, sel tools.Selection) error {
	if !hasComponent(sel, ids.ComponentMCP) {
		return nil
	}
	path, ok := render.MCPConfigPath(t.ID(), t.home)
	if !ok {
		return fmt.Errorf("claudecode: no MCP config path for home %q", t.home)
	}
	// Nothing to do when the desired entry is already present. This is what keeps
	// a rerun a true no-op (F13) regardless of which writer produced the current
	// state: without it, `claude mcp add-json` would fail on the existing server,
	// fall through to the file writer, and rewrite an entry that was already
	// correct — churning the file's mtime and waking file-watchers in running
	// agents.
	if t.alreadyConfigured(path) {
		return nil
	}
	if t.addViaCLI(ctx) {
		return nil
	}
	if err := configedit.SetJSONValue(path, t.mcpEntry(), "mcpServers", render.MCPServerName); err != nil {
		return fmt.Errorf("claudecode: write MCP config: %w", err)
	}
	return nil
}

// alreadyConfigured reports whether path already holds exactly the mcpServers.render
// entry Configure would write. An unreadable, unparseable, or absent entry reports
// false so configuration proceeds.
func (t *Tool) alreadyConfigured(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var cfg struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return false
	}
	raw, ok := cfg.MCPServers[render.MCPServerName]
	if !ok {
		return false
	}
	var current any
	if err := json.Unmarshal(raw, &current); err != nil {
		return false
	}
	want, err := json.Marshal(t.mcpEntry())
	if err != nil {
		return false
	}
	var desired any
	if err := json.Unmarshal(want, &desired); err != nil {
		return false
	}
	return reflect.DeepEqual(current, desired)
}

// addViaCLI attempts to register the Render MCP server through
// `claude mcp add-json <name> <json> --scope user`, reporting whether it
// succeeded. A missing CLI or any non-zero exit (including the "already exists"
// case on a rerun) reports false so the caller falls back to editing the file,
// which is idempotent and replaces the entry wholesale.
//
// --scope user is what writes the entry to the top-level mcpServers key in
// ~/.claude.json, making it available across all projects; the default local
// scope would bury it under the current project's entry instead.
func (t *Tool) addViaCLI(ctx context.Context) bool {
	if t.lookPath == nil || t.run == nil {
		return false
	}
	if _, err := t.lookPath("claude"); err != nil {
		return false
	}
	entry, err := json.Marshal(t.mcpEntry())
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, cliTimeout)
	defer cancel()
	return t.run(ctx, "claude", "mcp", "add-json", render.MCPServerName, string(entry), "--scope", "user") == nil
}

// Unconfigure removes the wizard's configuration from Claude Code by deleting
// only the mcpServers.render entry. Sibling MCP servers and unrelated keys are
// preserved, and a missing config file is a no-op.
//
// Like Configure it prefers `claude mcp remove --scope user` so Claude owns the
// write to its own live state file, falling back to editing ~/.claude.json when
// the CLI is absent or fails.
func (t *Tool) Unconfigure(ctx context.Context) error {
	path, ok := render.MCPConfigPath(t.ID(), t.home)
	if !ok {
		return fmt.Errorf("claudecode: no MCP config path for home %q", t.home)
	}
	if t.removeViaCLI(ctx) {
		return nil
	}
	if err := configedit.DeleteJSONPath(path, "mcpServers", render.MCPServerName); err != nil {
		return fmt.Errorf("claudecode: delete MCP config: %w", err)
	}
	return nil
}

// removeViaCLI attempts to drop the Render MCP server through
// `claude mcp remove <name> --scope user`, reporting whether it succeeded. A
// missing CLI or any non-zero exit (including "no such server") reports false so
// the caller falls back to the file editor, which no-ops when the entry is absent.
func (t *Tool) removeViaCLI(ctx context.Context) bool {
	if t.lookPath == nil || t.run == nil {
		return false
	}
	if _, err := t.lookPath("claude"); err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, cliTimeout)
	defer cancel()
	return t.run(ctx, "claude", "mcp", "remove", render.MCPServerName, "--scope", "user") == nil
}

// mcpEntry builds the Render MCP server entry written to mcpServers.render.
// It is written wholesale (replacing any prior render entry). OAuth writes a
// credential-free URL-only entry; API-key mode adds an Authorization header
// referencing an environment variable (never a secret).
//
// The "type" field is load-bearing, not decorative: Claude Code reads an entry
// that has a url but no type as a stdio server, then skips it and reports a
// configuration error. It must stay in place.
func (t *Tool) mcpEntry() map[string]any {
	entry := map[string]any{
		"type": "http",
		"url":  render.MCPServerURL,
	}
	if value, present := render.AuthorizationHeader(t.auth); present {
		entry["headers"] = map[string]any{"Authorization": value}
	}
	return entry
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
