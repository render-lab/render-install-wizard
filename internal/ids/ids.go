// Package ids defines the canonical component, tool, and delivery identifiers
// shared across the wizard. It is the single source of truth for these values so
// that packages (manifest, components, tools, paths, detect, …) never redefine them.
package ids

// ComponentID identifies an installable component — the "what" to install.
type ComponentID string

// User-selectable components. Plugins are an internal delivery mechanism, not a
// user-facing component, so they are intentionally not listed here.
const (
	ComponentCLI    ComponentID = "cli"
	ComponentSkills ComponentID = "skills"
	ComponentMCP    ComponentID = "mcp"
)

// AllComponents lists the user-selectable components in display order.
func AllComponents() []ComponentID {
	return []ComponentID{ComponentCLI, ComponentSkills, ComponentMCP}
}

// ToolID identifies a coding agent the wizard can configure — the "where".
type ToolID string

// Supported tools.
const (
	ToolClaudeCode ToolID = "claude-code"
	ToolCursor     ToolID = "cursor"
	ToolCodex      ToolID = "codex"
	ToolOpenCode   ToolID = "opencode"
)

// AllTools lists the supported tools in display order.
func AllTools() []ToolID {
	return []ToolID{ToolClaudeCode, ToolCursor, ToolCodex, ToolOpenCode}
}

// Delivery is how a tool receives skills + MCP: either raw config files written
// directly, or a bundled plugin that carries both.
type Delivery string

// Delivery mechanisms.
const (
	DeliveryRaw    Delivery = "raw"
	DeliveryPlugin Delivery = "plugin"
)

// contentSlug maps a ToolID to the slug used in its render.com content URL.
// Note the deliberate mismatch: claude-code's guide lives at /agents/claude.md.
var contentSlug = map[ToolID]string{
	ToolClaudeCode: "claude",
	ToolCursor:     "cursor",
	ToolCodex:      "codex",
	ToolOpenCode:   "opencode",
}

// ContentSlug returns the render.com/agents/<slug>.md content slug for a tool
// (e.g. claude-code -> "claude"). Unknown tools fall back to their raw ID.
func ContentSlug(t ToolID) string {
	if s, ok := contentSlug[t]; ok {
		return s
	}
	return string(t)
}
