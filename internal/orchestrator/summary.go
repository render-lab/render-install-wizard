package orchestrator

import (
	"fmt"
	"strings"

	"github.com/render-oss/render-install-wizard/internal/ids"
	"github.com/render-oss/render-install-wizard/internal/render"
)

// displayName returns a human-readable name for a tool, falling back to the ID.
func displayName(t ids.ToolID) string {
	switch t {
	case ids.ToolClaudeCode:
		return "Claude Code"
	case ids.ToolCursor:
		return "Cursor"
	case ids.ToolCodex:
		return "Codex"
	case ids.ToolOpenCode:
		return "OpenCode"
	default:
		return string(t)
	}
}

// Summary renders a plain, ANSI-free multi-line summary of the result, suitable
// for a terminal or a log. Each step is prefixed with its action in brackets.
func (r Result) Summary() string {
	var b strings.Builder
	title := "Render setup complete"
	switch {
	case r.DryRun && r.Uninstall:
		title = "Render MCP removal plan (dry run)"
	case r.DryRun:
		title = "Render setup plan (dry run)"
	case r.Uninstall:
		title = "Render MCP removed from your tools"
	}
	b.WriteString(title)
	b.WriteString("\n")

	if len(r.Components) > 0 {
		b.WriteString("\nComponents:\n")
		for _, s := range r.Components {
			writeStep(&b, s)
		}
	}
	if len(r.Tools) > 0 {
		b.WriteString("\nTools:\n")
		for _, s := range r.Tools {
			writeStep(&b, s)
		}
	}
	return b.String()
}

// writeStep appends one aligned step line: "  [action] id — detail".
func writeStep(b *strings.Builder, s StepResult) {
	fmt.Fprintf(b, "  [%s] %s", s.Action, s.ID)
	if s.Detail != "" {
		fmt.Fprintf(b, " — %s", s.Detail)
	}
	b.WriteString("\n")
}

// NextSteps returns the recommended follow-up actions for a plan, drawing plugin
// recommendations from internal/render and gating the login step on NoLogin. It
// returns nothing meaningful for an uninstall beyond a single confirmation line.
func NextSteps(plan Plan) []string {
	if plan.Options.Uninstall {
		return []string{
			"Removed the Render MCP entry from the targeted tools; other MCP servers were left intact.",
			"Note: the Render CLI and agent skills (if installed) are left in place — remove those manually if you want.",
			"Re-run the installer any time to set Render back up.",
		}
	}

	var steps []string
	if contains(plan.Components, ids.ComponentCLI) && !plan.Options.NoLogin {
		steps = append(steps, "Authenticate the CLI: run `render login`.")
	}
	if contains(plan.Components, ids.ComponentMCP) {
		steps = append(steps, "On first use, your agent will prompt you to sign in to Render to authorize the MCP server.")
	}
	for _, t := range plan.Tools {
		p := render.PluginFor(t)
		switch p.Kind {
		case render.PluginInApp:
			steps = append(steps, fmt.Sprintf("For %s, the richer Render plugin installs in-app: %s (%s).", displayName(t), p.Instruction, p.RepoURL))
		case render.PluginShell:
			steps = append(steps, fmt.Sprintf("For %s, an optional Render plugin adds commands and a subagent: %s", displayName(t), p.RepoURL))
		}
	}
	steps = append(steps,
		"Try it: ask your agent \u2014 \"deploy this repo to Render\".",
		"Guides: https://render.com/agents",
	)
	return steps
}

// contains reports whether the component slice includes want.
func contains(list []ids.ComponentID, want ids.ComponentID) bool {
	for _, c := range list {
		if c == want {
			return true
		}
	}
	return false
}
