package detect

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/render-oss/render-install-wizard/internal/ids"
)

// lookPathFunc resolves an executable name to a path, mirroring exec.LookPath.
type lookPathFunc func(string) (string, error)

// detector probes the host for installed coding agents. Its dependencies are
// injected so detection can be exercised against a fixture home directory and a
// fake PATH lookup in tests.
type detector struct {
	// home is the user's home directory.
	home string
	// goos is the operating system, used to resolve OS-specific config dirs.
	goos string
	// lookPath resolves a binary name on PATH.
	lookPath lookPathFunc
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

// hasClaudeCode reports whether Claude Code is installed: a ~/.claude directory
// or ~/.claude.json marker, or a claude binary on PATH.
func (d detector) hasClaudeCode() bool {
	return d.pathExists(filepath.Join(d.home, ".claude")) ||
		d.pathExists(filepath.Join(d.home, ".claude.json")) ||
		d.onPath("claude")
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

// hasCodex reports whether Codex is installed: a ~/.codex directory marker or a
// codex binary on PATH.
func (d detector) hasCodex() bool {
	return d.pathExists(filepath.Join(d.home, ".codex")) ||
		d.onPath("codex")
}

// hasOpenCode reports whether OpenCode is installed: a ~/.config/opencode
// directory marker or an opencode binary on PATH.
func (d detector) hasOpenCode() bool {
	return d.pathExists(filepath.Join(d.home, ".config", "opencode")) ||
		d.onPath("opencode")
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
