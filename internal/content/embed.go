package content

import (
	"embed"
	"io/fs"
)

// embeddedFS holds the last-known-good markdown snapshots of the render.com
// agent guides. These are the offline/air-gapped/CI fallback used when the live
// fetch fails.
//
// NOTE: spec.md originally referenced assets/content/ for the embedded copies,
// but go:embed cannot reach outside the embedding package's directory, so the
// snapshots live here in internal/content/embedded/ instead. The
// refresh-content.yml workflow targets this directory when snapshotting the live
// Sanity copy.
//
//go:embed embedded/*.md
var embeddedFS embed.FS

// embeddedKeys maps a content key (as used by the wizard and manifest, e.g.
// "cursor" or "agents") to its embedded snapshot filename.
var embeddedKeys = map[string]string{
	"agents":   "agents.md",
	"claude":   "claude.md",
	"cursor":   "cursor.md",
	"codex":    "codex.md",
	"opencode": "opencode.md",
}

// Embedded returns the last-known-good markdown snapshot for key (for example
// "cursor" or "agents"). ok is false if there is no snapshot for that key.
func Embedded(key string) (md string, ok bool) {
	name, known := embeddedKeys[key]
	if !known {
		return "", false
	}
	data, err := fs.ReadFile(embeddedFS, "embedded/"+name)
	if err != nil {
		return "", false
	}
	return string(data), true
}

// fallbackCopy is the terse built-in guidance shown when no richer content is
// available (neither a live fetch nor an embedded snapshot).
const fallbackCopy = "Visit https://render.com/agents for setup guides."

// FallbackCopy returns the terse built-in fallback guidance. It is the last link
// in the resolution chain live fetch → embedded snapshot → built-in copy.
func FallbackCopy() string {
	return fallbackCopy
}
