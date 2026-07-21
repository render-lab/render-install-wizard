package skills

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/render-lab/render-install-wizard/internal/components"
	"github.com/render-lab/render-install-wizard/internal/ids"
	"github.com/render-lab/render-install-wizard/internal/render"
)

// recorder captures the command a Component would have run without shelling out.
type recorder struct {
	called bool
	name   string
	args   []string
}

func (r *recorder) run(ctx context.Context, name string, args ...string) error {
	r.called = true
	r.name = name
	r.args = args
	return nil
}

// lookPathFunc returns a lookPath that resolves only the named executables.
func lookPathFunc(available ...string) func(string) (string, error) {
	set := make(map[string]bool, len(available))
	for _, a := range available {
		set[a] = true
	}
	return func(name string) (string, error) {
		if set[name] {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
}

func argsEqual(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestInstallPrefersNpxNonInteractive(t *testing.T) {
	rec := &recorder{}
	c := &Component{
		home:     t.TempDir(),
		lookPath: lookPathFunc(render.CLIBinaryName, "npx"), // both present -> npx wins
		run:      rec.run,
	}
	// Unscoped run (no Agents): all skills, all detected agents.
	if err := c.Install(context.Background(), components.Options{}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !rec.called || rec.name != "npx" {
		t.Fatalf("name = %q, want npx", rec.name)
	}
	// Pinned package (-y skills@<ver>) + --all + -g: non-interactive, global.
	want := []string{"-y", render.SkillsCLISpec, "add", render.SkillsRepo, "--all", "-g"}
	if !argsEqual(rec.args, want) {
		t.Fatalf("args = %v, want %v", rec.args, want)
	}
	// F05: the installer package must be pinned to an exact version, not "latest".
	if render.SkillsCLISpec == "skills" || !strings.Contains(render.SkillsCLISpec, "@") {
		t.Fatalf("SkillsCLISpec %q is not version-pinned", render.SkillsCLISpec)
	}
}

// TestInstallScopedTargetsOnlyNamedAgents guards F02: an explicit --agent scope
// must produce an installer invocation limited to those agents (never --all).
func TestInstallScopedTargetsOnlyNamedAgents(t *testing.T) {
	t.Run("single agent", func(t *testing.T) {
		rec := &recorder{}
		c := &Component{
			home:     t.TempDir(),
			lookPath: lookPathFunc("npx"),
			run:      rec.run,
		}
		err := c.Install(context.Background(), components.Options{Agents: []ids.ToolID{ids.ToolCursor}})
		if err != nil {
			t.Fatalf("Install: %v", err)
		}
		want := []string{"-y", render.SkillsCLISpec, "add", render.SkillsRepo, "--skill", "*", "-a", "cursor", "-g", "-y"}
		if rec.name != "npx" || !argsEqual(rec.args, want) {
			t.Fatalf("got %s %v, want npx %v", rec.name, rec.args, want)
		}
		for _, a := range rec.args {
			if a == "--all" {
				t.Fatal("scoped install must not pass --all")
			}
		}
	})

	t.Run("multiple agents", func(t *testing.T) {
		rec := &recorder{}
		c := &Component{
			home:     t.TempDir(),
			lookPath: lookPathFunc("npx"),
			run:      rec.run,
		}
		err := c.Install(context.Background(), components.Options{Agents: []ids.ToolID{ids.ToolCursor, ids.ToolCodex}})
		if err != nil {
			t.Fatalf("Install: %v", err)
		}
		want := []string{"-y", render.SkillsCLISpec, "add", render.SkillsRepo, "--skill", "*", "-a", "cursor", "-a", "codex", "-g", "-y"}
		if !argsEqual(rec.args, want) {
			t.Fatalf("args = %v, want %v", rec.args, want)
		}
	})
}

func TestInstallFallsBackToRenderCLI(t *testing.T) {
	rec := &recorder{}
	c := &Component{
		home:     t.TempDir(),
		lookPath: lookPathFunc(render.CLIBinaryName), // no npx -> render CLI
		run:      rec.run,
	}
	// Unscoped fallback is allowed (the CLI installs for all detected agents).
	if err := c.Install(context.Background(), components.Options{}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	// F04: fully non-interactive invocation, via the resolved absolute path.
	if rec.name != "/usr/bin/"+render.CLIBinaryName {
		t.Fatalf("name = %q, want resolved absolute render path", rec.name)
	}
	want := []string{"skills", "install", "--confirm", "--scope", "user", "-o", "text"}
	if !argsEqual(rec.args, want) {
		t.Fatalf("args = %v, want %v", rec.args, want)
	}
}

// TestInstallFallbackPrefersOwnedAbsolutePath guards F04: the fallback executes
// the wizard-owned CLI (~/.render/bin/render) by absolute path in preference to
// whatever a possibly-stale PATH resolves.
func TestInstallFallbackPrefersOwnedAbsolutePath(t *testing.T) {
	home := t.TempDir()
	owned := filepath.Join(home, ".render", "bin", render.CLIBinaryName)
	if err := os.MkdirAll(filepath.Dir(owned), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(owned, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{}
	c := &Component{
		home:     home,
		lookPath: lookPathFunc(render.CLIBinaryName), // PATH also resolves render, elsewhere
		run:      rec.run,
	}
	if err := c.Install(context.Background(), components.Options{}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if rec.name != owned {
		t.Fatalf("name = %q, want wizard-owned absolute path %q", rec.name, owned)
	}
}

// TestInstallScopedFailsClosedWithoutNpx guards F02: when a scope is requested
// but only the non-scopable Render CLI fallback is available, installation must
// fail closed rather than modify unselected agents.
func TestInstallScopedFailsClosedWithoutNpx(t *testing.T) {
	rec := &recorder{}
	c := &Component{
		home:     t.TempDir(),
		lookPath: lookPathFunc(render.CLIBinaryName), // no npx, render present
		run:      rec.run,
	}
	err := c.Install(context.Background(), components.Options{Agents: []ids.ToolID{ids.ToolCursor}})
	if err == nil {
		t.Fatal("expected fail-closed error for scoped install without npx")
	}
	if rec.called {
		t.Fatal("runner must not be called when failing closed")
	}
}

// condRecorder records the command names it runs and fails for a chosen name,
// so fallback ordering can be asserted.
type condRecorder struct {
	calls   []string
	failFor string
}

func (m *condRecorder) run(_ context.Context, name string, _ ...string) error {
	m.calls = append(m.calls, name)
	if name == m.failFor {
		return errors.New(name + " failed")
	}
	return nil
}

// TestInstallNpxFailureFallsBackToRenderCLI guards F36: when npx is present but
// fails on an unscoped run, the Render CLI fallback is used.
func TestInstallNpxFailureFallsBackToRenderCLI(t *testing.T) {
	rec := &condRecorder{failFor: "npx"}
	c := &Component{
		home:     t.TempDir(),
		lookPath: lookPathFunc("npx", render.CLIBinaryName),
		run:      rec.run,
	}
	if err := c.Install(context.Background(), components.Options{}); err != nil {
		t.Fatalf("expected fallback to Render CLI to succeed, got %v", err)
	}
	if len(rec.calls) != 2 || rec.calls[0] != "npx" {
		t.Fatalf("expected npx then render CLI, got %v", rec.calls)
	}
	if rec.calls[1] != "/usr/bin/"+render.CLIBinaryName {
		t.Errorf("fallback should invoke the resolved render CLI, got %q", rec.calls[1])
	}
}

// TestInstallNpxFailureScopedDoesNotFallBack guards F36 + F02: a scoped run must
// NOT fall back to the non-scopable Render CLI; it surfaces the npx error.
func TestInstallNpxFailureScopedDoesNotFallBack(t *testing.T) {
	rec := &condRecorder{failFor: "npx"}
	c := &Component{
		home:     t.TempDir(),
		lookPath: lookPathFunc("npx", render.CLIBinaryName),
		run:      rec.run,
	}
	err := c.Install(context.Background(), components.Options{Agents: []ids.ToolID{ids.ToolCursor}})
	if err == nil {
		t.Fatal("expected the npx error to surface for a scoped run")
	}
	if len(rec.calls) != 1 || rec.calls[0] != "npx" {
		t.Errorf("scoped run must not fall back to the Render CLI, calls=%v", rec.calls)
	}
}

func TestInstallErrorsWhenNoInstaller(t *testing.T) {
	rec := &recorder{}
	c := &Component{
		home:     t.TempDir(),
		lookPath: lookPathFunc(),
		run:      rec.run,
	}
	if err := c.Install(context.Background(), components.Options{}); err == nil {
		t.Fatal("expected error when neither render nor npx present")
	}
	if rec.called {
		t.Fatal("runner should not be called")
	}
}

func TestInstallDryRunSkipsRunner(t *testing.T) {
	rec := &recorder{}
	c := &Component{
		home:     t.TempDir(),
		lookPath: lookPathFunc(render.CLIBinaryName),
		run:      rec.run,
	}
	if err := c.Install(context.Background(), components.Options{DryRun: true}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if rec.called {
		t.Fatal("runner should not be called on dry run")
	}
}

func TestDetectUniversalSkillsDir(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(render.UniversalSkillsDir(home), 0o755); err != nil {
		t.Fatal(err)
	}
	c := &Component{home: home, lookPath: lookPathFunc()}
	got, err := c.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !got {
		t.Fatal("expected Detect true with universal skills dir present")
	}
}

func TestDetectClaudeSkillsDir(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	c := &Component{home: home, lookPath: lookPathFunc()}
	got, err := c.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !got {
		t.Fatal("expected Detect true with claude skills dir present")
	}
}

func TestDetectEmptyHome(t *testing.T) {
	c := &Component{home: t.TempDir(), lookPath: lookPathFunc()}
	got, err := c.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if got {
		t.Fatal("expected Detect false in empty home")
	}
}

func TestUninstallRemovesUniversalDir(t *testing.T) {
	home := t.TempDir()
	dir := render.UniversalSkillsDir(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	c := &Component{home: home}
	if err := c.Uninstall(context.Background()); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected universal skills dir removed, stat err = %v", err)
	}
	// Missing dir is not an error.
	if err := c.Uninstall(context.Background()); err != nil {
		t.Fatalf("Uninstall (missing): %v", err)
	}
}
