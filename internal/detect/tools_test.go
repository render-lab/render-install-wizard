package detect

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/render-lab/render-install-wizard/internal/ids"
)

// fakeLookPath returns a lookPathFunc that resolves only the named binaries.
func fakeLookPath(names ...string) lookPathFunc {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return func(name string) (string, error) {
		if set[name] {
			return "/usr/local/bin/" + name, nil
		}
		return "", exec.ErrNotFound
	}
}

func TestDetectorDetect(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		dirs     []string // relative to home, created as directories
		files    []string // relative to home, created as empty files
		binaries []string // resolvable via fake lookPath
		want     []ids.ToolID
	}{
		{
			name: "empty home and empty path",
			goos: "linux",
			want: nil,
		},
		{
			name: "claude via .claude dir",
			goos: "linux",
			dirs: []string{".claude"},
			want: []ids.ToolID{ids.ToolClaudeCode},
		},
		{
			name:  "claude via .claude.json file",
			goos:  "darwin",
			files: []string{".claude.json"},
			want:  []ids.ToolID{ids.ToolClaudeCode},
		},
		{
			name:     "claude via binary",
			goos:     "linux",
			binaries: []string{"claude"},
			want:     []ids.ToolID{ids.ToolClaudeCode},
		},
		{
			name: "cursor via .cursor dir",
			goos: "linux",
			dirs: []string{".cursor"},
			want: []ids.ToolID{ids.ToolCursor},
		},
		{
			name: "cursor via linux config dir",
			goos: "linux",
			dirs: []string{".config/Cursor"},
			want: []ids.ToolID{ids.ToolCursor},
		},
		{
			name: "cursor via macos config dir",
			goos: "darwin",
			dirs: []string{"Library/Application Support/Cursor"},
			want: []ids.ToolID{ids.ToolCursor},
		},
		{
			name: "codex via .codex dir",
			goos: "linux",
			dirs: []string{".codex"},
			want: []ids.ToolID{ids.ToolCodex},
		},
		{
			name: "opencode via config dir",
			goos: "linux",
			dirs: []string{".config/opencode"},
			want: []ids.ToolID{ids.ToolOpenCode},
		},
		{
			name:     "opencode via binary",
			goos:     "linux",
			binaries: []string{"opencode"},
			want:     []ids.ToolID{ids.ToolOpenCode},
		},
		{
			name:     "all tools sorted",
			goos:     "linux",
			dirs:     []string{".config/opencode", ".codex"},
			files:    []string{".claude.json"},
			binaries: []string{"cursor"},
			want: []ids.ToolID{
				ids.ToolClaudeCode,
				ids.ToolCursor,
				ids.ToolCodex,
				ids.ToolOpenCode,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			for _, dir := range tt.dirs {
				if err := os.MkdirAll(filepath.Join(home, dir), 0o755); err != nil {
					t.Fatalf("mkdir %q: %v", dir, err)
				}
			}
			for _, file := range tt.files {
				path := filepath.Join(home, file)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("mkdir for %q: %v", file, err)
				}
				if err := os.WriteFile(path, nil, 0o644); err != nil {
					t.Fatalf("write %q: %v", file, err)
				}
			}

			d := detector{
				home:     home,
				goos:     tt.goos,
				lookPath: fakeLookPath(tt.binaries...),
			}
			got := d.detect()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("detect() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDetectorHonorsConfigHomeOverrides guards F11: agents whose config lives at
// a relocated, environment-overridden location are still detected even when the
// default directories are empty and no binary is on PATH.
func TestDetectorHonorsConfigHomeOverrides(t *testing.T) {
	home := t.TempDir()     // default locations intentionally empty
	override := t.TempDir() // overridden locations live here

	// Claude: $CLAUDE_CONFIG_DIR/.claude.json
	if err := os.WriteFile(filepath.Join(override, ".claude.json"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// Codex: $CODEX_HOME directory
	codexHome := filepath.Join(override, "codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatal(err)
	}
	// OpenCode: $XDG_CONFIG_HOME/opencode directory
	xdg := filepath.Join(override, "xdg")
	if err := os.MkdirAll(filepath.Join(xdg, "opencode"), 0o755); err != nil {
		t.Fatal(err)
	}

	env := map[string]string{
		"CLAUDE_CONFIG_DIR": override,
		"CODEX_HOME":        codexHome,
		"XDG_CONFIG_HOME":   xdg,
	}
	d := detector{
		home:     home,
		goos:     "linux",
		lookPath: fakeLookPath(), // nothing on PATH: force the marker path
		getenv:   func(k string) string { return env[k] },
	}
	got := d.detect()
	want := []ids.ToolID{ids.ToolClaudeCode, ids.ToolCodex, ids.ToolOpenCode}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("detect() with overrides = %v, want %v", got, want)
	}
}
