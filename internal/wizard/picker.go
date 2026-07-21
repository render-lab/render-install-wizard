package wizard

import "github.com/render-lab/render-install-wizard/internal/ids"

// Selection captures the components the user has chosen to install.
type Selection struct {
	// Components are the chosen components, in ids.AllComponents() order.
	Components []ids.ComponentID
}

// DefaultSelection returns the selection containing all components.
func DefaultSelection() Selection {
	return Selection{Components: ids.AllComponents()}
}

// component is a single togglable row in the single-axis "what to install"
// picker: a component ID plus whether it is currently checked.
type component struct {
	id      ids.ComponentID
	checked bool
}

// defaultComponents returns every component pre-checked, in display order. This
// encodes the detect-then-default policy: one Enter installs everything.
func defaultComponents() []component {
	all := ids.AllComponents()
	items := make([]component, len(all))
	for i, id := range all {
		items[i] = component{id: id, checked: true}
	}
	return items
}

// toggle flips the checked state of the component at index i. Out-of-range
// indices are ignored so callers need not bounds-check.
func toggle(items []component, i int) {
	if i >= 0 && i < len(items) {
		items[i].checked = !items[i].checked
	}
}

// selection builds a Selection from the checked components, preserving order.
func selection(items []component) Selection {
	var chosen []ids.ComponentID
	for _, it := range items {
		if it.checked {
			chosen = append(chosen, it.id)
		}
	}
	return Selection{Components: chosen}
}

// componentLabel returns a short human-readable label for a component.
func componentLabel(id ids.ComponentID) string {
	switch id {
	case ids.ComponentCLI:
		return "Render CLI"
	case ids.ComponentSkills:
		return "Agent skills"
	case ids.ComponentMCP:
		return "Render MCP"
	default:
		return string(id)
	}
}

// toolNames maps a tool ID to its human-readable display name.
var toolNames = map[ids.ToolID]string{
	ids.ToolClaudeCode: "Claude Code",
	ids.ToolCursor:     "Cursor",
	ids.ToolCodex:      "Codex",
	ids.ToolOpenCode:   "OpenCode",
}

// toolName returns a human-readable name for a tool, falling back to the raw ID.
func toolName(id ids.ToolID) string {
	if n, ok := toolNames[id]; ok {
		return n
	}
	return string(id)
}
