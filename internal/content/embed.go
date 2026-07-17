package content

// fallbackCopy is the terse built-in guidance shown when no richer content is
// available.
const fallbackCopy = "Visit https://render.com/agents for setup guides."

// FallbackCopy returns the terse built-in fallback guidance.
// TODO(phase 1D): replace with a go:embed'd content snapshot.
func FallbackCopy() string {
	return fallbackCopy
}
