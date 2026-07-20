package cliflags

import (
	"errors"
	"flag"
	"reflect"
	"testing"
)

func TestParseComponentsSplitAndTrim(t *testing.T) {
	f, err := Parse([]string{"--components", " cli , skills ,, mcp "})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []string{"cli", "skills", "mcp"}
	if !reflect.DeepEqual(f.Components, want) {
		t.Errorf("Components = %#v, want %#v", f.Components, want)
	}
}

func TestParseRepeatableAgent(t *testing.T) {
	f, err := Parse([]string{"--agent", "cursor", "--agent", "codex"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []string{"cursor", "codex"}
	if !reflect.DeepEqual(f.Agents, want) {
		t.Errorf("Agents = %#v, want %#v", f.Agents, want)
	}
}

func TestParseShortAndLongAliases(t *testing.T) {
	for _, args := range [][]string{{"-y"}, {"--yes"}} {
		f, err := Parse(args)
		if err != nil {
			t.Fatalf("Parse(%v): %v", args, err)
		}
		if !f.Yes {
			t.Errorf("Parse(%v): Yes = false, want true", args)
		}
	}
	for _, args := range [][]string{{"-r"}, {"--uninstall"}} {
		f, err := Parse(args)
		if err != nil {
			t.Fatalf("Parse(%v): %v", args, err)
		}
		if !f.Uninstall {
			t.Errorf("Parse(%v): Uninstall = false, want true", args)
		}
	}
}

func TestParseNoComponentsYieldsNil(t *testing.T) {
	f, err := Parse(nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(f.Components) != 0 {
		t.Errorf("Components = %#v, want empty", f.Components)
	}
	if len(f.Agents) != 0 {
		t.Errorf("Agents = %#v, want empty", f.Agents)
	}
}

func TestParseInvalidFlagErrors(t *testing.T) {
	if _, err := Parse([]string{"--nope"}); err == nil {
		t.Fatal("Parse(--nope) = nil error, want error")
	}
}

// TestParseRejectsPositionalArguments guards F01: Go's flag parser stops at the
// first operand, so a stray token must be rejected instead of silently
// disabling the flags that follow it (e.g. `install --dry-run` running a real
// install).
func TestParseRejectsPositionalArguments(t *testing.T) {
	cases := [][]string{
		{"install"},                        // bare subcommand-like operand
		{"install", "--dry-run"},           // operand before a safety flag
		{"--dry-run", "install"},           // operand after a safety flag
		{"--components", "cli", "install"}, // operand after a value flag
		{"install", "extra"},               // multiple operands
	}
	for _, args := range cases {
		if _, err := Parse(args); err == nil {
			t.Errorf("Parse(%v) = nil error, want rejection of positional operand", args)
		}
	}
}

// TestParseDryRunAloneSucceeds is the positive counterpart: without any
// operand, --dry-run is honored.
func TestParseDryRunAloneSucceeds(t *testing.T) {
	f, err := Parse([]string{"--dry-run"})
	if err != nil {
		t.Fatalf("Parse(--dry-run): %v", err)
	}
	if !f.DryRun {
		t.Error("DryRun = false, want true")
	}
}

// TestParseHelpReturnsErrHelp ensures -h/--help stays a distinguishable,
// non-error path so the entrypoint can exit 0 without mutating anything.
func TestParseHelpReturnsErrHelp(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}} {
		if _, err := Parse(args); !errors.Is(err, flag.ErrHelp) {
			t.Errorf("Parse(%v) err = %v, want flag.ErrHelp", args, err)
		}
	}
}
