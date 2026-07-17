// Package detect discovers the host platform and installed coding agents.
package detect

import "runtime"

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

// DetectPlatform returns the current host platform. WSL and TTY detection are
// stubbed for now.
func DetectPlatform() Platform {
	return Platform{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
		// TODO(phase 1B): implement real WSL and TTY detection.
		IsWSL:  false,
		HasTTY: false,
	}
}
