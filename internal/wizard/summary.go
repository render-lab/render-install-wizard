package wizard

import "strings"

// Summary reports what the wizard installed.
type Summary struct {
	// Installed lists the human-readable names of installed items.
	Installed []string
}

// String returns a human-readable, comma-joined summary of installed items.
func (s Summary) String() string {
	return strings.Join(s.Installed, ", ")
}
