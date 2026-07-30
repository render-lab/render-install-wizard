package detect

import (
	"os"
	"testing"
)

// TestHasTTYRequiresBothStreams covers the condition that gates the interactive
// picker. A TUI needs to read keystrokes *and* draw, so an interactive stdin with
// a redirected stdout must not count: the escape sequences would go into the file
// or pipe instead of the screen, corrupting the captured output while the user
// sees nothing to respond to.
func TestHasTTYRequiresBothStreams(t *testing.T) {
	in, out := os.Stdin.Fd(), os.Stdout.Fd()
	for _, tc := range []struct {
		name     string
		terminal map[uintptr]bool
		want     bool
	}{
		{"both are terminals", map[uintptr]bool{in: true, out: true}, true},
		{"stdout redirected", map[uintptr]bool{in: true, out: false}, false},
		{"stdin piped", map[uintptr]bool{in: false, out: true}, false},
		{"neither, as in CI", map[uintptr]bool{in: false, out: false}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := hasTTY(func(fd uintptr) bool { return tc.terminal[fd] })
			if got != tc.want {
				t.Fatalf("hasTTY() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestDetectPlatformReportsNoTTYWhenStdoutIsNot checks DetectPlatform is actually
// wired to the combined predicate. It is guarded rather than unconditional because
// whether the test binary inherits a terminal depends on how it was launched.
func TestDetectPlatformReportsNoTTYWhenStdoutIsNot(t *testing.T) {
	if isTerminal(os.Stdout.Fd()) {
		t.Skip("stdout is a terminal in this run, so there is nothing to assert")
	}
	if DetectPlatform().HasTTY {
		t.Fatal("HasTTY must be false when stdout is not a terminal")
	}
}
