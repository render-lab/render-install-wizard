package detect

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/render-lab/render-install-wizard/internal/ids"
)

// lookPathFunc resolves an executable name to a path, mirroring exec.LookPath.
type lookPathFunc func(string) (string, error)

// detector probes the host for installed coding agents. Its dependencies are
// injected so detection can be exercised against a fixture home directory, a
// fake PATH lookup, and a fake environment in tests.
type detector struct {
	// home is the user's home directory.
	home string
	// goos is the operating system, used to resolve OS-specific config dirs.
	goos string
	// lookPath resolves a binary name on PATH.
	lookPath lookPathFunc
	// getenv resolves environment variables (tool config-home overrides). It may
	// be nil, in which case no override is honored.
	getenv func(string) string
}

// DetectTools discovers which supported coding agents are installed on the host.
// Each tool is detected via a home-directory marker and/or a binary on PATH.
// The result is sorted in ids.AllTools() order.
func DetectTools(ctx context.Context) ([]ids.ToolID, error) {
	_ = ctx
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	d := detector{
		home:     home,
		goos:     runtime.GOOS,
		lookPath: exec.LookPath,
		getenv:   os.Getenv,
	}
	return d.detect(), nil
}

// detect returns the tools installed according to the detector's home directory
// and PATH lookup, sorted in ids.AllTools() order.
func (d detector) detect() []ids.ToolID {
	checks := map[ids.ToolID]func() bool{
		ids.ToolClaudeCode: d.hasClaudeCode,
		ids.ToolCursor:     d.hasCursor,
		ids.ToolCodex:      d.hasCodex,
		ids.ToolOpenCode:   d.hasOpenCode,
	}
	var found []ids.ToolID
	for _, tool := range ids.AllTools() {
		if check, ok := checks[tool]; ok && check() {
			found = append(found, tool)
		}
	}
	return found
}

// hasClaudeCode reports whether Claude Code is installed: a claude binary on
// PATH, or a .claude/.claude.json marker in the default home or in a
// CLAUDE_CONFIG_DIR-overridden config directory.
func (d detector) hasClaudeCode() bool {
	if d.onPath("claude") {
		return true
	}
	dirs := []string{d.home}
	if override := d.env("CLAUDE_CONFIG_DIR"); override != "" {
		dirs = append(dirs, override)
	}
	for _, dir := range dirs {
		if d.pathExists(filepath.Join(dir, ".claude")) || d.pathExists(filepath.Join(dir, ".claude.json")) {
			return true
		}
	}
	return false
}

// hasCursor reports whether Cursor is installed: a ~/.cursor directory, the
// OS-specific application config directory, or a cursor binary on PATH.
func (d detector) hasCursor() bool {
	return d.pathExists(filepath.Join(d.home, ".cursor")) ||
		d.pathExists(d.cursorConfigDir()) ||
		d.onPath("cursor")
}

// cursorConfigDir returns the OS-specific Cursor application config directory.
func (d detector) cursorConfigDir() string {
	switch d.goos {
	case "darwin":
		return filepath.Join(d.home, "Library", "Application Support", "Cursor")
	case "linux":
		return filepath.Join(d.home, ".config", "Cursor")
	default:
		return ""
	}
}

// hasCodex reports whether Codex is installed: a codex binary on PATH or a
// config directory marker (~/.codex, or $CODEX_HOME when set).
func (d detector) hasCodex() bool {
	if d.onPath("codex") {
		return true
	}
	dir := filepath.Join(d.home, ".codex")
	if override := d.env("CODEX_HOME"); override != "" {
		dir = override
	}
	return d.pathExists(dir)
}

// hasOpenCode reports whether OpenCode is installed: an opencode binary on PATH,
// an explicit OPENCODE_CONFIG file, or the global config directory
// (~/.config/opencode, honoring XDG_CONFIG_HOME).
func (d detector) hasOpenCode() bool {
	if d.onPath("opencode") {
		return true
	}
	if f := d.env("OPENCODE_CONFIG"); f != "" && d.pathExists(f) {
		return true
	}
	base := filepath.Join(d.home, ".config")
	if override := d.env("XDG_CONFIG_HOME"); override != "" {
		base = override
	}
	return d.pathExists(filepath.Join(base, "opencode"))
}

// pathExists reports whether the given non-empty path exists on disk.
func (d detector) pathExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// onPath reports whether the named binary resolves via the detector's lookPath.
func (d detector) onPath(name string) bool {
	if d.lookPath == nil {
		return false
	}
	_, err := d.lookPath(name)
	return err == nil
}

// env resolves an environment variable via the detector's getenv, returning ""
// when no environment lookup is configured.
func (d detector) env(key string) string {
	if d.getenv == nil {
		return ""
	}
	return d.getenv(key)
}
