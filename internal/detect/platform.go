// Package detect discovers the host platform and installed coding agents.
package detect

import (
	"os"
	"runtime"
	"strings"

	"golang.org/x/term"
)

// Platform describes the host environment relevant to installation decisions.
type Platform struct {
	// OS is the operating system (runtime.GOOS).
	OS string
	// Arch is the CPU architecture (runtime.GOARCH).
	Arch string
	// IsWSL reports whether the host is running under Windows Subsystem for Linux.
	IsWSL bool
	// HasTTY reports whether an interactive terminal is attached, meaning the
	// wizard may take over the screen with a TUI. See DetectPlatform for why both
	// stdin and stdout must be terminals for this to be true.
	HasTTY bool
}

// wslProcFiles are the procfs paths inspected for a WSL signature. They only
// exist on Linux hosts; missing files are treated as "no signature".
var wslProcFiles = []string{"/proc/version", "/proc/sys/kernel/osrelease"}

// DetectPlatform returns the current host platform, including WSL and TTY
// detection.
//
// HasTTY requires stdin *and* stdout to be terminals. Stdin alone is not enough:
// it only establishes that the wizard can read keystrokes, while a TUI also has to
// draw. With stdout redirected — `render-setup > install.log`, a pipe into another
// program, or a CI job capturing output — cursor moves and redraws would be
// written into the file or pipe as escape sequences, leaving the user watching an
// apparently hung program whose prompts they cannot see, while the captured output
// is corrupted. Requiring both means those cases fall through to the
// non-interactive default instead.
func DetectPlatform() Platform {
	return Platform{
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		IsWSL:  detectWSL(runtime.GOOS, readWSLProcFiles),
		HasTTY: hasTTY(isTerminal),
	}
}

// isTerminal reports whether fd refers to a terminal.
func isTerminal(fd uintptr) bool { return term.IsTerminal(int(fd)) }

// hasTTY reports whether both of this process's interactive streams are
// terminals, using the injected predicate so the stdin/stdout combinations are
// testable without a real terminal on either descriptor.
func hasTTY(isTerm func(uintptr) bool) bool {
	return isTerm(os.Stdin.Fd()) && isTerm(os.Stdout.Fd())
}

// readWSLProcFiles reads the procfs files used for WSL detection, concatenating
// the contents of any that exist. Unreadable or missing files are skipped.
func readWSLProcFiles() string {
	var sb strings.Builder
	for _, path := range wslProcFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		sb.Write(data)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// detectWSL reports whether the given OS and procfs content indicate a WSL
// environment. Detection only applies on Linux; the content source is injected
// so the logic is testable without a real WSL host. The match is
// case-insensitive against "microsoft" and "wsl".
func detectWSL(goos string, readProc func() string) bool {
	if goos != "linux" {
		return false
	}
	return containsWSLSignature(readProc())
}

// containsWSLSignature reports whether the given text carries a WSL marker
// ("microsoft" or "wsl"), case-insensitively.
func containsWSLSignature(content string) bool {
	lower := strings.ToLower(content)
	return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
}
