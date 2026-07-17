package content

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

// wordWrap is the column width glamour wraps rendered markdown to. 80 keeps the
// output readable in a standard terminal.
const wordWrap = 80

// Render converts markdown into styled terminal output using glamour with an
// auto-detected (light/dark) style. If rendering fails for any reason it falls
// back to returning the raw markdown, so it never panics and never returns an
// empty string for non-empty input.
func Render(markdown string) string {
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(wordWrap),
	)
	if err != nil {
		return markdown
	}
	out, err := r.Render(markdown)
	if err != nil {
		return markdown
	}
	// glamour may reduce a non-empty document to whitespace only in edge cases;
	// preserve the original rather than returning effectively nothing.
	if markdown != "" && strings.TrimSpace(out) == "" {
		return markdown
	}
	return out
}

// RenderPlain returns markdown as clean, unstyled text suitable for --json or
// non-TTY output, where ANSI styling must not appear. The source is already
// human-readable markdown, so it is returned as-is (trimmed of surrounding
// whitespace); callers that need the styled TUI form should use Render.
func RenderPlain(markdown string) string {
	return strings.TrimSpace(markdown)
}
