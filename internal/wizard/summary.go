package wizard

import (
	"strings"

	"github.com/render-oss/render-install-wizard/internal/ids"
)

// Summary is the plain, end-state description of what the wizard will do: which
// components were selected and which detected tools they apply to.
type Summary struct {
	// Selection is the chosen set of components.
	Selection Selection
	// Tools are the detected tools the components will be configured into.
	Tools []ids.ToolID
}

// String renders a plain, ANSI-free one-line summary, safe for non-TTY and JSON
// contexts, e.g. "Will install: cli, skills, mcp — into: cursor, codex".
func (s Summary) String() string {
	components := make([]string, len(s.Selection.Components))
	for i, c := range s.Selection.Components {
		components[i] = string(c)
	}
	tools := make([]string, len(s.Tools))
	for i, t := range s.Tools {
		tools[i] = string(t)
	}

	install := joinOrNone(components)
	into := joinOrNone(tools)
	return "Will install: " + install + " — into: " + into
}

// joinOrNone comma-joins items, returning "(none)" when the list is empty.
func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	return strings.Join(items, ", ")
}
