// Package skills implements the components.Installer contract for agent skills.
//
// Skills are not written directly by this component. Instead the work is
// delegated to the official skills installer (the Render CLI's `render skills
// install`, or `npx skills add <repo>` as a fallback). The official installer
// auto-detects the coding agents on the machine and writes both the per-tool
// skills directories and the universal (~/.agents/skills) directory. This
// component's job is to locate an appropriate installer, invoke it, and provide
// best-effort detection/cleanup around the universal marker directory.
package skills

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/render-oss/render-install-wizard/internal/components"
	"github.com/render-oss/render-install-wizard/internal/ids"
	"github.com/render-oss/render-install-wizard/internal/render"
)

// Component installs and manages Render agent skills by delegating to the
// official skills installer.
type Component struct {
	// home is the user's home directory. It is injectable so tests can point at
	// a temporary directory instead of the real home.
	home string
	// lookPath resolves an executable in PATH. Injectable for tests.
	lookPath func(string) (string, error)
	// run executes an external command. Injectable so tests never shell out.
	run func(ctx context.Context, name string, args ...string) error
}

// New returns a new agent skills component wired to the real environment:
// the OS home directory, exec.LookPath, and exec.CommandContext.
func New() *Component {
	home, _ := os.UserHomeDir()
	return &Component{
		home:     home,
		lookPath: exec.LookPath,
		run: func(ctx context.Context, name string, args ...string) error {
			return exec.CommandContext(ctx, name, args...).Run()
		},
	}
}

// ID returns the canonical identifier for the agent skills component.
func (c *Component) ID() ids.ComponentID { return ids.ComponentSkills }

// Detect reports whether agent skills appear to be installed.
//
// This is a heuristic: the official installer writes many per-tool directories
// that aren't fully tracked here, so we look only for well-known markers — the
// universal skills directory (~/.agents/skills) or Claude's per-tool skills
// directory (~/.claude/skills). Presence of either is treated as installed.
func (c *Component) Detect(ctx context.Context) (bool, error) {
	if dirExists(render.UniversalSkillsDir(c.home)) {
		return true, nil
	}
	if dirExists(claudeSkillsDir(c.home)) {
		return true, nil
	}
	return false, nil
}

// Install installs agent skills by delegating to the official installer.
//
// On a dry run it performs no side effects. Otherwise it prefers the `skills`
// CLI via npx (the documented primary path) with fully non-interactive flags —
// `--all` installs every skill to all detected agents without prompts, and `-g`
// installs to the user (global) skills directory. It falls back to the Render
// CLI's `render skills install` (>= 2.10) when npx isn't available; the runner
// leaves child stdin unset (/dev/null), so any prompt there gets EOF rather than
// hanging. It returns an actionable error when neither installer is present.
func (c *Component) Install(ctx context.Context, opts components.Options) error {
	if opts.DryRun {
		return nil
	}
	if _, err := c.lookPath("npx"); err == nil {
		return c.run(ctx, "npx", "skills", "add", render.SkillsRepo, "--all", "-g")
	}
	if _, err := c.lookPath(render.CLIBinaryName); err == nil {
		return c.run(ctx, render.CLIBinaryName, "skills", "install")
	}
	return errors.New("cannot install skills: install Node/npx or the Render CLI to add skills")
}

// Uninstall best-effort removes the universal skills directory.
//
// It does not fully undo an install: the official installer also writes per-tool
// skills directories that aren't tracked here, so those are left in place. A
// missing directory is not an error.
func (c *Component) Uninstall(ctx context.Context) error {
	if err := os.RemoveAll(render.UniversalSkillsDir(c.home)); err != nil {
		return fmt.Errorf("remove universal skills dir: %w", err)
	}
	return nil
}

// Status returns the current status of the agent skills component, derived from
// the Detect heuristic.
func (c *Component) Status(ctx context.Context) (components.Status, error) {
	installed, err := c.Detect(ctx)
	if err != nil {
		return components.Status{ID: ids.ComponentSkills, State: components.StateUnknown}, err
	}
	state := components.StateNotInstalled
	if installed {
		state = components.StateInstalled
	}
	return components.Status{ID: ids.ComponentSkills, State: state}, nil
}

// dirExists reports whether path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// claudeSkillsDir returns Claude Code's per-tool skills directory under home.
func claudeSkillsDir(home string) string {
	return filepath.Join(home, ".claude", "skills")
}

var _ components.Installer = (*Component)(nil)
