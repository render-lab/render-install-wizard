package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/render-oss/render-install-wizard/internal/components"
	"github.com/render-oss/render-install-wizard/internal/ids"
	"github.com/render-oss/render-install-wizard/internal/render"
)

// recordedCall captures a single invocation of the fake runner.
type recordedCall struct {
	name string
	args []string
}

// recorder is a fake runner that records the commands it is asked to execute.
type recorder struct {
	calls []recordedCall
	err   error
}

func (r *recorder) run(_ context.Context, name string, args ...string) error {
	r.calls = append(r.calls, recordedCall{name: name, args: args})
	return r.err
}

// lookPathFound returns a lookPath that resolves only the given names.
func lookPathFound(names ...string) func(string) (string, error) {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return func(name string) (string, error) {
		if set[name] {
			return "/fake/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
}

// lookPathNone resolves nothing.
func lookPathNone() func(string) (string, error) {
	return func(string) (string, error) { return "", errors.New("not found") }
}

// writeBin creates <home>/.render/bin/render so filesystem detection succeeds.
func writeBin(t *testing.T, home string) string {
	t.Helper()
	dir := filepath.Join(home, ".render", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, render.CLIBinaryName)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}
	return path
}

func TestDetect(t *testing.T) {
	ctx := context.Background()

	t.Run("found on PATH", func(t *testing.T) {
		c := &Component{home: t.TempDir(), lookPath: lookPathFound(render.CLIBinaryName)}
		got, err := c.Detect(ctx)
		if err != nil {
			t.Fatalf("Detect: %v", err)
		}
		if !got {
			t.Fatal("expected detected via PATH")
		}
	})

	t.Run("found in home bin", func(t *testing.T) {
		home := t.TempDir()
		writeBin(t, home)
		c := &Component{home: home, lookPath: lookPathNone()}
		got, err := c.Detect(ctx)
		if err != nil {
			t.Fatalf("Detect: %v", err)
		}
		if !got {
			t.Fatal("expected detected via home bin")
		}
	})

	t.Run("not found", func(t *testing.T) {
		c := &Component{home: t.TempDir(), lookPath: lookPathNone()}
		got, err := c.Detect(ctx)
		if err != nil {
			t.Fatalf("Detect: %v", err)
		}
		if got {
			t.Fatal("expected not detected")
		}
	})
}

func TestInstall(t *testing.T) {
	ctx := context.Background()

	t.Run("dry run does nothing", func(t *testing.T) {
		rec := &recorder{}
		c := &Component{home: t.TempDir(), lookPath: lookPathFound("brew"), run: rec.run}
		if err := c.Install(ctx, components.Options{DryRun: true}); err != nil {
			t.Fatalf("Install: %v", err)
		}
		if len(rec.calls) != 0 {
			t.Fatalf("expected no runner calls, got %v", rec.calls)
		}
	})

	t.Run("brew present", func(t *testing.T) {
		rec := &recorder{}
		c := &Component{home: t.TempDir(), lookPath: lookPathFound("brew"), run: rec.run}
		if err := c.Install(ctx, components.Options{}); err != nil {
			t.Fatalf("Install: %v", err)
		}
		if len(rec.calls) != 1 {
			t.Fatalf("expected 1 call, got %v", rec.calls)
		}
		call := rec.calls[0]
		if call.name != "brew" {
			t.Fatalf("expected brew, got %q", call.name)
		}
		want := []string{"install", render.CLIBinaryName}
		if len(call.args) != len(want) || call.args[0] != want[0] || call.args[1] != want[1] {
			t.Fatalf("expected args %v, got %v", want, call.args)
		}
	})

	t.Run("brew absent falls back to curl", func(t *testing.T) {
		rec := &recorder{}
		c := &Component{home: t.TempDir(), lookPath: lookPathNone(), run: rec.run}
		if err := c.Install(ctx, components.Options{}); err != nil {
			t.Fatalf("Install: %v", err)
		}
		if len(rec.calls) != 1 {
			t.Fatalf("expected 1 call, got %v", rec.calls)
		}
		call := rec.calls[0]
		if call.name != "sh" {
			t.Fatalf("expected sh, got %q", call.name)
		}
		if len(call.args) != 2 || call.args[0] != "-c" {
			t.Fatalf("expected sh -c <script>, got %v", call.args)
		}
		script := call.args[1]
		for _, sub := range []string{"curl", render.CLIInstallScriptURL, "| sh"} {
			if !strings.Contains(script, sub) {
				t.Fatalf("expected script to contain %q, got %q", sub, script)
			}
		}
	})

	t.Run("runner error is wrapped", func(t *testing.T) {
		rec := &recorder{err: errors.New("boom")}
		c := &Component{home: t.TempDir(), lookPath: lookPathNone(), run: rec.run}
		err := c.Install(ctx, components.Options{})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestUninstall(t *testing.T) {
	ctx := context.Background()

	t.Run("removes existing binary", func(t *testing.T) {
		home := t.TempDir()
		path := writeBin(t, home)
		c := &Component{home: home}
		if err := c.Uninstall(ctx); err != nil {
			t.Fatalf("Uninstall: %v", err)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected binary removed, stat err=%v", err)
		}
	})

	t.Run("absent is no error", func(t *testing.T) {
		c := &Component{home: t.TempDir()}
		if err := c.Uninstall(ctx); err != nil {
			t.Fatalf("Uninstall: %v", err)
		}
	})
}

func TestStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("not installed", func(t *testing.T) {
		c := &Component{home: t.TempDir(), lookPath: lookPathNone()}
		st, err := c.Status(ctx)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if st.State != components.StateNotInstalled {
			t.Fatalf("expected not_installed, got %q", st.State)
		}
	})

	t.Run("installed with version", func(t *testing.T) {
		c := &Component{
			home:     t.TempDir(),
			lookPath: lookPathFound(render.CLIBinaryName),
			runOutput: func(_ context.Context, _ string, _ ...string) (string, error) {
				return "render 2.10.0\n", nil
			},
		}
		st, err := c.Status(ctx)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if st.State != components.StateInstalled {
			t.Fatalf("expected installed, got %q", st.State)
		}
		if st.Version != "render 2.10.0" {
			t.Fatalf("expected trimmed version, got %q", st.Version)
		}
	})

	t.Run("installed but version fails", func(t *testing.T) {
		c := &Component{
			home:     t.TempDir(),
			lookPath: lookPathFound(render.CLIBinaryName),
			runOutput: func(_ context.Context, _ string, _ ...string) (string, error) {
				return "", errors.New("boom")
			},
		}
		st, err := c.Status(ctx)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if st.State != components.StateInstalled {
			t.Fatalf("expected installed, got %q", st.State)
		}
		if st.Version != "" {
			t.Fatalf("expected empty version, got %q", st.Version)
		}
		if st.Detail == "" {
			t.Fatal("expected a detail note")
		}
	})
}

func TestID(t *testing.T) {
	if got := New().ID(); got != ids.ComponentCLI {
		t.Fatalf("expected %q, got %q", ids.ComponentCLI, got)
	}
}
