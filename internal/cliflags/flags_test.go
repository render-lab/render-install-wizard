package cliflags

import (
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
