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
	// HasTTY reports whether an interactive terminal is attached.
	HasTTY bool
}

// wslProcFiles are the procfs paths inspected for a WSL signature. They only
// exist on Linux hosts; missing files are treated as "no signature".
var wslProcFiles = []string{"/proc/version", "/proc/sys/kernel/osrelease"}

// DetectPlatform returns the current host platform, including WSL and TTY
// detection.
func DetectPlatform() Platform {
	return Platform{
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		IsWSL:  detectWSL(runtime.GOOS, readWSLProcFiles),
		HasTTY: term.IsTerminal(int(os.Stdin.Fd())),
	}
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
