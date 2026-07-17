package skills

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/render-oss/render-install-wizard/internal/components"
	"github.com/render-oss/render-install-wizard/internal/render"
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

func TestInstallPrefersRenderCLI(t *testing.T) {
	rec := &recorder{}
	c := &Component{
		home:     t.TempDir(),
		lookPath: lookPathFunc(render.CLIBinaryName, "npx"),
		run:      rec.run,
	}
	if err := c.Install(context.Background(), components.Options{}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !rec.called {
		t.Fatal("expected runner to be called")
	}
	if rec.name != render.CLIBinaryName {
		t.Fatalf("name = %q, want %q", rec.name, render.CLIBinaryName)
	}
	want := []string{"skills", "install"}
	if len(rec.args) != len(want) || rec.args[0] != want[0] || rec.args[1] != want[1] {
		t.Fatalf("args = %v, want %v", rec.args, want)
	}
}

func TestInstallFallsBackToNpx(t *testing.T) {
	rec := &recorder{}
	c := &Component{
		home:     t.TempDir(),
		lookPath: lookPathFunc("npx"),
		run:      rec.run,
	}
	if err := c.Install(context.Background(), components.Options{}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if rec.name != "npx" {
		t.Fatalf("name = %q, want npx", rec.name)
	}
	want := []string{"skills", "add", render.SkillsRepo}
	if len(rec.args) != len(want) {
		t.Fatalf("args = %v, want %v", rec.args, want)
	}
	for i := range want {
		if rec.args[i] != want[i] {
			t.Fatalf("args = %v, want %v", rec.args, want)
		}
	}
	if render.SkillsRepo != "render-oss/skills" {
		t.Fatalf("unexpected SkillsRepo %q", render.SkillsRepo)
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
