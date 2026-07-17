package paths

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsurePATHEntryIdempotent(t *testing.T) {
	tests := []struct {
		name     string
		shell    string
		wantLine string
	}{
		{
			name:     "zsh",
			shell:    "zsh",
			wantLine: `export PATH="/home/u/.render/bin:$PATH"`,
		},
		{
			name:     "bash",
			shell:    "bash",
			wantLine: `export PATH="/home/u/.render/bin:$PATH"`,
		},
		{
			name:     "fish",
			shell:    "fish",
			wantLine: "fish_add_path /home/u/.render/bin",
		},
	}

	const binDir = "/home/u/.render/bin"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rcFile := filepath.Join(t.TempDir(), "rcfile")

			changed, err := EnsurePATHEntry(rcFile, tt.shell, binDir)
			if err != nil {
				t.Fatalf("first EnsurePATHEntry: %v", err)
			}
			if !changed {
				t.Fatalf("first call changed = false, want true")
			}

			first, err := os.ReadFile(rcFile)
			if err != nil {
				t.Fatalf("read rc after first call: %v", err)
			}
			content := string(first)
			if !strings.Contains(content, pathBlockStart) || !strings.Contains(content, pathBlockEnd) {
				t.Errorf("block markers missing:\n%s", content)
			}
			if !strings.Contains(content, tt.wantLine) {
				t.Errorf("expected line %q not found in:\n%s", tt.wantLine, content)
			}

			changed, err = EnsurePATHEntry(rcFile, tt.shell, binDir)
			if err != nil {
				t.Fatalf("second EnsurePATHEntry: %v", err)
			}
			if changed {
				t.Fatalf("second call changed = true, want false")
			}

			second, err := os.ReadFile(rcFile)
			if err != nil {
				t.Fatalf("read rc after second call: %v", err)
			}
			if string(second) != content {
				t.Errorf("file changed on second call:\nbefore:\n%s\nafter:\n%s", content, second)
			}
			if n := strings.Count(string(second), pathBlockStart); n != 1 {
				t.Errorf("found %d start markers, want 1", n)
			}
		})
	}
}

func TestEnsurePATHEntryCreatesParentDirs(t *testing.T) {
	rcFile := filepath.Join(t.TempDir(), ".config", "fish", "config.fish")
	changed, err := EnsurePATHEntry(rcFile, "fish", "/home/u/.render/bin")
	if err != nil {
		t.Fatalf("EnsurePATHEntry: %v", err)
	}
	if !changed {
		t.Fatalf("changed = false, want true")
	}
	if _, err := os.Stat(rcFile); err != nil {
		t.Fatalf("rc file not created: %v", err)
	}
}

func TestEnsurePATHEntryPreservesExisting(t *testing.T) {
	rcFile := filepath.Join(t.TempDir(), ".bashrc")
	original := "# user config\nalias ll='ls -la'"
	if err := os.WriteFile(rcFile, []byte(original), 0o644); err != nil {
		t.Fatalf("seed rc: %v", err)
	}

	changed, err := EnsurePATHEntry(rcFile, "bash", "/home/u/.render/bin")
	if err != nil {
		t.Fatalf("EnsurePATHEntry: %v", err)
	}
	if !changed {
		t.Fatalf("changed = false, want true")
	}

	got, err := os.ReadFile(rcFile)
	if err != nil {
		t.Fatalf("read rc: %v", err)
	}
	if !strings.HasPrefix(string(got), original) {
		t.Errorf("original content not preserved:\n%s", got)
	}
}

func TestShellRCFile(t *testing.T) {
	const home = "/home/u"
	tests := []struct {
		shell   string
		want    string
		wantErr bool
	}{
		{shell: "zsh", want: filepath.Join(home, ".zshrc")},
		{shell: "bash", want: filepath.Join(home, ".bashrc")},
		{shell: "fish", want: filepath.Join(home, ".config", "fish", "config.fish")},
		{shell: "tcsh", wantErr: true},
		{shell: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			got, err := ShellRCFile(tt.shell, home)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ShellRCFile(%q) err = nil, want error", tt.shell)
				}
				return
			}
			if err != nil {
				t.Fatalf("ShellRCFile(%q): %v", tt.shell, err)
			}
			if got != tt.want {
				t.Errorf("ShellRCFile(%q) = %q, want %q", tt.shell, got, tt.want)
			}
		})
	}
}
