package main

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/render-oss/render-install-wizard/internal/cliflags"
	"github.com/render-oss/render-install-wizard/internal/ids"
	"github.com/render-oss/render-install-wizard/internal/wizard"
)

// captureOutput redirects os.Stdout and os.Stderr for the duration of fn and
// returns whatever was written to stdout. It lets the entrypoint tests assert
// exit codes without leaking the flag package's usage text into test output.
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	origOut, origErr := os.Stdout, os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout, os.Stderr = wOut, wErr
	defer func() { os.Stdout, os.Stderr = origOut, origErr }()

	fn()

	wOut.Close()
	wErr.Close()
	out, _ := io.ReadAll(rOut)
	_, _ = io.ReadAll(rErr)
	return string(out)
}

// TestRunRejectsPositionalArguments guards F01 at the entrypoint: operand forms
// exit nonzero. The parse error is returned before the orchestrator registry is
// built, so no installer runs and no state changes.
func TestRunRejectsPositionalArguments(t *testing.T) {
	cases := [][]string{
		{"install", "--dry-run"}, // the canonical dangerous form
		{"--dry-run", "install"}, // operand after the safety flag
		{"install"},              // bare operand
	}
	for _, args := range cases {
		var code int
		captureOutput(t, func() { code = run(args) })
		if code != 2 {
			t.Errorf("run(%v) = %d, want 2", args, code)
		}
	}
}

// TestRunHelpExitsZero ensures -h/--help is a successful, non-mutating path.
func TestRunHelpExitsZero(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}} {
		var code int
		captureOutput(t, func() { code = run(args) })
		if code != 0 {
			t.Errorf("run(%v) = %d, want 0", args, code)
		}
	}
}

// TestRunVersionExitsZero ensures --version prints and exits 0 without touching
// installers.
func TestRunVersionExitsZero(t *testing.T) {
	var code int
	out := captureOutput(t, func() { code = run([]string{"--version"}) })
	if code != 0 {
		t.Fatalf("run(--version) = %d, want 0", code)
	}
	if strings.TrimSpace(out) == "" {
		t.Error("run(--version) printed nothing, want a version string")
	}
}

// stubWizard installs interactive seams that simulate a TTY and a wizard.Run
// with the given behavior, restoring the originals when the test ends.
func stubWizard(t *testing.T, fn func() (wizard.Selection, bool, error)) {
	t.Helper()
	origTTY, origRun := detectHasTTY, runWizard
	t.Cleanup(func() { detectHasTTY, runWizard = origTTY, origRun })
	detectHasTTY = func() bool { return true }
	runWizard = func(context.Context, []ids.ToolID) (wizard.Selection, bool, error) { return fn() }
}

// TestResolveSelectionFailsClosedOnWizardError guards F06: an interactive picker
// that errors must surface the error (fail closed), not silently return the
// default selection.
func TestResolveSelectionFailsClosedOnWizardError(t *testing.T) {
	stubWizard(t, func() (wizard.Selection, bool, error) {
		return wizard.Selection{}, false, errors.New("tui init failed")
	})
	var warnings []string
	sel, cancelled, err := resolveSelection(context.Background(), &cliflags.Flags{}, nil, &warnings)
	if err == nil {
		t.Fatal("expected a fail-closed error from a failed interactive picker")
	}
	if cancelled {
		t.Error("cancelled should be false when the picker errors")
	}
	if len(sel.Components) != 0 {
		t.Errorf("expected empty selection on error, got %v", sel.Components)
	}
}

// TestResolveSelectionCancel confirms a clean user cancel is distinct from an
// error: no error, cancelled=true.
func TestResolveSelectionCancel(t *testing.T) {
	stubWizard(t, func() (wizard.Selection, bool, error) {
		return wizard.Selection{}, false, nil
	})
	var warnings []string
	_, cancelled, err := resolveSelection(context.Background(), &cliflags.Flags{}, nil, &warnings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cancelled {
		t.Error("expected cancelled=true")
	}
}

// TestRunFailsClosedOnWizardError guards F06 end-to-end: a failed interactive
// picker makes the process exit nonzero before any plan executes.
func TestRunFailsClosedOnWizardError(t *testing.T) {
	called := false
	stubWizard(t, func() (wizard.Selection, bool, error) {
		called = true
		return wizard.Selection{}, false, errors.New("render loop failed")
	})
	var code int
	captureOutput(t, func() { code = run(nil) })
	if code != 1 {
		t.Fatalf("run() = %d, want 1 (fail closed)", code)
	}
	if !called {
		t.Fatal("expected the interactive picker seam to be exercised")
	}
}
