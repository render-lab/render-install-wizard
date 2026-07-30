package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// bashRCForHost is the rc file bash uses on the host OS. macOS Terminal starts
// login shells, which read ~/.bash_profile rather than ~/.bashrc.
func bashRCForHost(home string) string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, ".bash_profile")
	}
	return filepath.Join(home, ".bashrc")
}

// TestShellRCFileBashPerOS pins both sides of the bash split explicitly, so the
// macOS behavior is covered even when the suite runs on Linux and vice versa.
func TestShellRCFileBashPerOS(t *testing.T) {
	home := "/home/u"
	got, err := shellRCFile("bash", home, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".bash_profile"); got != want {
		t.Errorf("darwin bash rc = %q, want %q (Terminal starts login shells, which skip .bashrc)", got, want)
	}

	got, err = shellRCFile("bash", home, "linux")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".bashrc"); got != want {
		t.Errorf("linux bash rc = %q, want %q", got, want)
	}
}

// TestEnsurePATHEntryRepairsStaleBlock is the regression guard for a block that
// could never be corrected: EnsurePATHEntry used to return early on merely seeing
// the start marker, so a block naming an outdated directory survived every rerun.
func TestEnsurePATHEntryRepairsStaleBlock(t *testing.T) {
	rcFile := filepath.Join(t.TempDir(), ".zshrc")

	if _, err := EnsurePATHEntry(rcFile, "zsh", "/old/stale/bin"); err != nil {
		t.Fatal(err)
	}
	changed, err := EnsurePATHEntry(rcFile, "zsh", "/new/correct/bin")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("changed = false; a block naming a different directory must be repaired")
	}

	got, err := os.ReadFile(rcFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "/new/correct/bin") {
		t.Errorf("new directory missing:\n%s", got)
	}
	if strings.Contains(string(got), "/old/stale/bin") {
		t.Errorf("stale directory still present:\n%s", got)
	}
	if n := strings.Count(string(got), pathBlockStart); n != 1 {
		t.Errorf("found %d blocks, want 1:\n%s", n, got)
	}
}

// TestEnsurePATHEntryPreservesSurroundingContent guards the core promise: only the
// marked region is ours.
func TestEnsurePATHEntryPreservesSurroundingContent(t *testing.T) {
	rcFile := filepath.Join(t.TempDir(), ".zshrc")
	before := "# my own setup\nexport EDITOR=vim\n"
	after := "alias ll='ls -la'\n"
	seed := before + pathBlockStart + "\nexport PATH=\"/old/bin:$PATH\"\n" + pathBlockEnd + "\n" + after
	if err := os.WriteFile(rcFile, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := EnsurePATHEntry(rcFile, "zsh", "/new/bin"); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(rcFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(got), before) {
		t.Errorf("content before the block was altered:\n%s", got)
	}
	if !strings.HasSuffix(string(got), after) {
		t.Errorf("content after the block was altered:\n%s", got)
	}
	if strings.Contains(string(got), "/old/bin") {
		t.Errorf("stale entry survived:\n%s", got)
	}
}

// TestEnsurePATHEntryQuotesAwkwardPaths covers the fish bug directly: an unquoted
// path splits on whitespace, so a home directory with a space produced two wrong
// entries instead of one correct one.
func TestEnsurePATHEntryQuotesAwkwardPaths(t *testing.T) {
	const spaced = "/Users/John Smith/.render/bin"

	t.Run("fish", func(t *testing.T) {
		rcFile := filepath.Join(t.TempDir(), "config.fish")
		if _, err := EnsurePATHEntry(rcFile, "fish", spaced); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(rcFile)
		if err != nil {
			t.Fatal(err)
		}
		want := "fish_add_path '" + spaced + "'"
		if !strings.Contains(string(got), want) {
			t.Errorf("want %q in:\n%s", want, got)
		}
	})

	t.Run("zsh", func(t *testing.T) {
		rcFile := filepath.Join(t.TempDir(), ".zshrc")
		if _, err := EnsurePATHEntry(rcFile, "zsh", spaced); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(rcFile)
		if err != nil {
			t.Fatal(err)
		}
		want := `export PATH="` + spaced + `:$PATH"`
		if !strings.Contains(string(got), want) {
			t.Errorf("want %q in:\n%s", want, got)
		}
	})
}

// TestPathBlockEscapesShellMetacharacters ensures a path containing shell-special
// characters is emitted literally rather than being reinterpreted, while $PATH
// itself stays expandable in the posix case.
func TestPathBlockEscapesShellMetacharacters(t *testing.T) {
	tricky := `/tmp/we"ird/$HOME/back\slash/bin`

	posix, err := pathBlock("zsh", tricky)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`\"`, `\$HOME`, `\\slash`} {
		if !strings.Contains(posix, want) {
			t.Errorf("zsh block missing escape %q:\n%s", want, posix)
		}
	}
	if !strings.HasSuffix(strings.TrimSpace(strings.Split(posix, "\n")[1]), `:$PATH"`) {
		t.Errorf("zsh block must leave $PATH expandable:\n%s", posix)
	}

	fishBlock, err := pathBlock("fish", `/tmp/it's/bin`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fishBlock, `\'`) {
		t.Errorf("fish block must escape a single quote:\n%s", fishBlock)
	}
}

// TestEnsurePATHEntryRepairsTruncatedBlock covers a half-written block (start
// marker, no end marker), which must be repaired rather than duplicated.
func TestEnsurePATHEntryRepairsTruncatedBlock(t *testing.T) {
	rcFile := filepath.Join(t.TempDir(), ".zshrc")
	if err := os.WriteFile(rcFile, []byte("# mine\n"+pathBlockStart+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsurePATHEntry(rcFile, "zsh", "/new/bin"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(rcFile)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(got), pathBlockStart); n != 1 {
		t.Errorf("found %d start markers, want 1:\n%s", n, got)
	}
	if !strings.Contains(string(got), pathBlockEnd) {
		t.Errorf("end marker not restored:\n%s", got)
	}
	if !strings.Contains(string(got), "/new/bin") {
		t.Errorf("new entry missing:\n%s", got)
	}
}
