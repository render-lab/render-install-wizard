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
	b.WriteString(r.title())
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

// title derives the summary headline from the actual outcome rather than always
// claiming success, so a failed, partial, or no-op run is described honestly.
func (r Result) title() string {
	switch {
	case r.DryRun && r.Uninstall:
		return "Render MCP removal plan (dry run)"
	case r.DryRun:
		return "Render setup plan (dry run)"
	case r.Uninstall:
		if r.HasFailures() {
			return "Render MCP removal completed with errors"
		}
		return "Render MCP removed from your tools"
	}
	switch {
	case r.allFailed():
		return "Render setup failed"
	case r.HasFailures():
		return "Render setup completed with errors"
	case r.nothingChanged():
		return "Render setup: already up to date"
	default:
		return "Render setup complete"
	}
}

// stepActions returns the action of every recorded component and tool step.
func (r Result) stepActions() []Action {
	actions := make([]Action, 0, len(r.Components)+len(r.Tools))
	for _, s := range r.Components {
		actions = append(actions, s.Action)
	}
	for _, s := range r.Tools {
		actions = append(actions, s.Action)
	}
	return actions
}

// allFailed reports that at least one step ran and none of them succeeded.
func (r Result) allFailed() bool {
	sawFailure, sawSuccess := false, false
	for _, a := range r.stepActions() {
		switch a {
		case ActionFailed:
			sawFailure = true
		case ActionInstalled, ActionConfigured, ActionUnchanged, ActionRemoved:
			sawSuccess = true
		}
	}
	return sawFailure && !sawSuccess
}

// nothingChanged reports that every step was a no-op (unchanged or skipped).
func (r Result) nothingChanged() bool {
	any := false
	for _, a := range r.stepActions() {
		any = true
		if a != ActionUnchanged && a != ActionSkipped {
			return false
		}
	}
	return any
}

// reconcileMCPResult rewrites the MCP component step to reflect the actual
// per-tool configuration outcome — the MCP component installer itself is a no-op,
// so reporting it as "installed" would be misleading. An explicit MCP request
// with no tools to target, or where every target failed, becomes a failure so
// the run exits nonzero instead of falsely reporting success.
func reconcileMCPResult(res *Result, plan Plan) {
	if plan.Options.DryRun || plan.Options.Uninstall {
		return
	}
	if !contains(plan.Components, ids.ComponentMCP) {
		return
	}
	idx := -1
	for i, s := range res.Components {
		if s.ID == string(ids.ComponentMCP) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	configured, failed := 0, 0
	for _, t := range res.Tools {
		switch t.Action {
		case ActionConfigured:
			configured++
		case ActionFailed:
			failed++
		}
	}
	switch {
	case len(plan.Tools) == 0:
		res.Components[idx] = StepResult{ID: string(ids.ComponentMCP), Action: ActionFailed, Detail: "no coding agents to configure the Render MCP server into"}
	case configured == 0:
		res.Components[idx] = StepResult{ID: string(ids.ComponentMCP), Action: ActionFailed, Detail: fmt.Sprintf("MCP configuration failed for all %d target tool(s)", failed)}
	default:
		res.Components[idx] = StepResult{ID: string(ids.ComponentMCP), Action: ActionConfigured, Detail: fmt.Sprintf("configured in %d tool(s)", configured)}
	}
}

// writeStep appends one aligned step line: "  [action] id — detail".
func writeStep(b *strings.Builder, s StepResult) {
	fmt.Fprintf(b, "  [%s] %s", s.Action, s.ID)
	if s.Detail != "" {
		fmt.Fprintf(b, " — %s", s.Detail)
	}
	b.WriteString("\n")
}

// NextSteps returns the recommended follow-up actions, derived from the actual
// result rather than the requested plan: steps whose prerequisite step did not
// succeed are suppressed, so a run is never told to `render login` after the CLI
// install failed, or to authorize MCP when no tool was configured. It returns a
// fixed set of confirmation lines for an uninstall.
func NextSteps(plan Plan, res Result) []string {
	if plan.Options.Uninstall {
		return []string{
			"Removed the Render MCP entry from the targeted tools; other MCP servers were left intact.",
			"Note: the Render CLI and agent skills (if installed) are left in place — remove those manually if you want.",
			"Re-run the installer any time to set Render back up.",
		}
	}

	var steps []string
	if !plan.Options.NoLogin && componentSucceeded(res, ids.ComponentCLI) {
		steps = append(steps, "Authenticate the CLI: run `render login`.")
	}

	var configuredTools []ids.ToolID
	for _, t := range plan.Tools {
		if toolConfigured(res, t) {
			configuredTools = append(configuredTools, t)
		}
	}
	if len(configuredTools) > 0 {
		steps = append(steps, "On first use, your agent will prompt you to sign in to Render to authorize the MCP server.")
	}
	for _, t := range configuredTools {
		p := render.PluginFor(t)
		switch p.Kind {
		case render.PluginInApp:
			steps = append(steps, fmt.Sprintf("For %s, the richer Render plugin installs in-app: %s (%s).", displayName(t), p.Instruction, p.RepoURL))
		case render.PluginShell:
			steps = append(steps, fmt.Sprintf("For %s, an optional Render plugin adds commands and a subagent: %s", displayName(t), p.RepoURL))
		}
	}

	// Generic guidance only when at least one step actually succeeded; a fully
	// failed run should not end with an encouraging "try it".
	if anySuccess(res) {
		steps = append(steps,
			"Try it: ask your agent \u2014 \"deploy this repo to Render\".",
			"Guides: https://render.com/agents",
		)
	}
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

// componentSucceeded reports whether the given component's step installed cleanly
// or was already satisfied (unchanged).
func componentSucceeded(res Result, id ids.ComponentID) bool {
	for _, s := range res.Components {
		if s.ID == string(id) {
			return s.Action == ActionInstalled || s.Action == ActionUnchanged
		}
	}
	return false
}

// toolConfigured reports whether the given tool was successfully configured.
func toolConfigured(res Result, id ids.ToolID) bool {
	for _, s := range res.Tools {
		if s.ID == string(id) {
			return s.Action == ActionConfigured
		}
	}
	return false
}

// anySuccess reports whether any component or tool step succeeded.
func anySuccess(res Result) bool {
	for _, a := range res.stepActions() {
		switch a {
		case ActionInstalled, ActionConfigured, ActionUnchanged, ActionRemoved:
			return true
		}
	}
	return false
}
