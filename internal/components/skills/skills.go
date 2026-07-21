// Package skills implements the components.Installer contract for agent skills.
//
// Skills are not written directly by this component. Instead the work is
// delegated to the official skills installer: the `skills` CLI via `npx skills
// add <repo>` (the primary path), or the Render CLI's `render skills install` as
// a fallback when npx is unavailable. Unscoped, the installer auto-detects the
// coding agents on the machine and writes both the per-tool skills directories
// and the universal (~/.agents/skills) directory; a --agent scope narrows it to
// the named agents. This component's job is to locate an appropriate installer,
// build the correctly scoped invocation, and provide best-effort
// detection/cleanup around the universal marker directory.
package skills

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/render-lab/render-install-wizard/internal/components"
	"github.com/render-lab/render-install-wizard/internal/execx"
	"github.com/render-lab/render-install-wizard/internal/ids"
	"github.com/render-lab/render-install-wizard/internal/render"
)

// installTimeout bounds each skills-install attempt (F17) so a stalled npx or
// Render CLI subprocess fails with a clear error rather than hanging.
const installTimeout = 5 * time.Minute

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
		run:      execx.Run,
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

// Install installs agent skills by delegating to the official installer,
// honoring the agent scope in opts.
//
// On a dry run it performs no side effects. Otherwise it prefers the `skills`
// CLI via npx (the documented primary path), pinned to render.SkillsCLISpec and
// built fully non-interactive from the scope:
//
//   - Unscoped (opts.Agents empty): `-y skills@<ver> add <repo> --all -g`
//     installs every skill to all detected agents (and the universal
//     ~/.agents/skills dir) without prompts, delegating agent detection to the
//     official installer.
//   - Scoped (opts.Agents non-empty): `-y skills@<ver> add <repo> --skill *
//     -a <agent>… -g -y` installs every skill to only the named agents, so an
//     explicit --agent scope never touches unrelated agent environments.
//
// When npx is unavailable it falls back to the Render CLI, run by absolute path
// as `render skills install --confirm --scope user -o text` — non-interactive
// (no prompt can block on EOF) and installing all detected tools/skills. That
// fallback cannot scope by agent, so a scoped request fails closed with an
// actionable error rather than modifying unselected agents. It returns an
// actionable error when neither installer is present.
func (c *Component) Install(ctx context.Context, opts components.Options) error {
	if opts.DryRun {
		return nil
	}
	_, npxLookErr := c.lookPath("npx")
	cliPath, cliAvail := c.renderCLIPath()

	if npxLookErr == nil {
		if npxErr := c.runBounded(ctx, "npx", skillsAddArgs(opts.Agents)...); npxErr == nil {
			return nil
		} else if len(opts.Agents) == 0 && cliAvail {
			// npx is present but failed. The Render CLI fallback can't scope by
			// agent, so only fall back for an unscoped run (F36); a scoped run
			// surfaces the npx error rather than silently touching all agents.
			if cliErr := c.runBounded(ctx, cliPath, renderSkillsArgs()...); cliErr != nil {
				return fmt.Errorf("install skills: npx failed (%v); Render CLI fallback also failed: %w", npxErr, cliErr)
			}
			return nil
		} else {
			return fmt.Errorf("install skills via npx: %w", npxErr)
		}
	}

	if cliAvail {
		if len(opts.Agents) > 0 {
			return fmt.Errorf("cannot scope skills to %s: the Render CLI fallback (render skills install) installs skills for all detected agents. Install Node/npx to scope skill installation by agent", agentNames(opts.Agents))
		}
		return c.runBounded(ctx, cliPath, renderSkillsArgs()...)
	}
	return errors.New("cannot install skills: install Node/npx or the Render CLI to add skills")
}

// runBounded runs a skills-install command under installTimeout so a stalled
// child fails with a clear deadline error instead of hanging (F17).
func (c *Component) runBounded(ctx context.Context, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, installTimeout)
	defer cancel()
	return c.run(ctx, name, args...)
}

// renderSkillsArgs is the fully non-interactive Render CLI skills-install
// invocation (F04): on a non-TTY the CLI emits text output, and --confirm makes
// it use defaults (all detected tools/skills, user scope) and skip every prompt
// rather than blocking on EOF from a nil stdin.
func renderSkillsArgs() []string {
	return []string{"skills", "install", "--confirm", "--scope", "user", "-o", "text"}
}

// renderCLIPath resolves the Render CLI to an absolute path, preferring the
// wizard-owned install (~/.render/bin/render) over a PATH lookup so the skills
// fallback executes the binary the wizard just installed rather than whatever a
// possibly-stale PATH resolves.
func (c *Component) renderCLIPath() (string, bool) {
	if c.home != "" {
		owned := filepath.Join(c.home, ".render", "bin", render.CLIBinaryName)
		if fi, err := os.Stat(owned); err == nil && !fi.IsDir() {
			return owned, true
		}
	}
	if c.lookPath != nil {
		if p, err := c.lookPath(render.CLIBinaryName); err == nil {
			return p, true
		}
	}
	return "", false
}

// skillsAddArgs builds the argument vector passed to `npx` for the skills
// installer. The `-y` before the package lets npx install it without prompting,
// and render.SkillsCLISpec pins the installer to an exact version so npx never
// executes an unverified "latest" release.
//
// An empty agents slice yields the unscoped `--all -g` form; a non-empty slice
// yields an explicit, non-interactive per-agent scope. Agent IDs match the
// skills CLI's --agent values (claude-code, cursor, codex, opencode) exactly, so
// no name translation is needed.
func skillsAddArgs(agents []ids.ToolID) []string {
	args := []string{"-y", render.SkillsCLISpec, "add", render.SkillsRepo}
	if len(agents) == 0 {
		return append(args, "--all", "-g")
	}
	args = append(args, "--skill", "*")
	for _, a := range agents {
		args = append(args, "-a", string(a))
	}
	// -g installs to the user (global) skills dir; -y skips the skills CLI's own
	// prompts (implied by --all in the unscoped path, but required once we drop
	// --all here).
	return append(args, "-g", "-y")
}

// agentNames renders a set of agent IDs as a comma-separated string for errors.
func agentNames(agents []ids.ToolID) string {
	names := make([]string, len(agents))
	for i, a := range agents {
		names[i] = string(a)
	}
	return strings.Join(names, ", ")
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
