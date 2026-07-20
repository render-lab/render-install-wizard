// Package orchestrator wires the wizard together: it maps a selection of
// components and a set of target tools onto the compiled component installers
// and tool targets, executes install/uninstall (or plans a dry run), and
// collects a structured result for the summary.
//
// The registry is the authority on which components/tools the binary can
// actually act on. IDs that appear in a plan but have no compiled handler are
// skipped with a recorded "skipped" result rather than failing — this is the
// must-ignore-unknown rule that keeps an older binary working against a newer
// remote manifest.
package orchestrator

import (
	"context"

	"github.com/render-oss/render-install-wizard/internal/components"
	"github.com/render-oss/render-install-wizard/internal/components/cli"
	"github.com/render-oss/render-install-wizard/internal/components/mcp"
	"github.com/render-oss/render-install-wizard/internal/components/skills"
	"github.com/render-oss/render-install-wizard/internal/ids"
	"github.com/render-oss/render-install-wizard/internal/tools"
	"github.com/render-oss/render-install-wizard/internal/tools/claudecode"
	"github.com/render-oss/render-install-wizard/internal/tools/codex"
	"github.com/render-oss/render-install-wizard/internal/tools/cursor"
	"github.com/render-oss/render-install-wizard/internal/tools/opencode"
)

// Options controls how a plan is executed.
type Options struct {
	// DryRun plans the work and records intended actions without side effects.
	DryRun bool
	// Uninstall reverses an install: components are uninstalled and tools are
	// unconfigured instead of installed/configured.
	Uninstall bool
	// NoLogin suppresses the "run render login" next step.
	NoLogin bool
	// ScopedAgents is true when the user explicitly restricted the run to a
	// subset of agents (via --agent). When true, Plan.Tools is that explicit
	// scope and is forwarded to component installers so they only touch the
	// selected agents. When false the run is unscoped (all detected agents) and
	// components fall back to their "all detected" behavior.
	ScopedAgents bool
}

// Plan describes what to do: which components to act on, which tools to target,
// and how (Options). Components and Tools may contain IDs the registry does not
// know; those are skipped (must-ignore-unknown).
type Plan struct {
	Components []ids.ComponentID
	Tools      []ids.ToolID
	Options    Options
}

// Action is the outcome recorded for a single component or tool step.
type Action string

// Step outcomes.
const (
	// ActionInstalled means a component was installed.
	ActionInstalled Action = "installed"
	// ActionRemoved means a component/tool entry was removed.
	ActionRemoved Action = "removed"
	// ActionConfigured means a tool was configured.
	ActionConfigured Action = "configured"
	// ActionPlanned means the step was planned only (dry run).
	ActionPlanned Action = "planned"
	// ActionSkipped means the step was skipped (unsupported ID or nothing to do).
	ActionSkipped Action = "skipped"
	// ActionFailed means the step returned an error.
	ActionFailed Action = "failed"
)

// StepResult is the outcome of acting on a single component or tool.
type StepResult struct {
	// ID is the component or tool identifier.
	ID string `json:"id"`
	// Action is the outcome.
	Action Action `json:"action"`
	// Detail is an optional human-readable note (including error text on failure).
	Detail string `json:"detail,omitempty"`
}

// Result aggregates the per-component and per-tool step outcomes for a plan.
type Result struct {
	// DryRun echoes whether the plan was executed as a dry run.
	DryRun bool `json:"dryRun"`
	// Uninstall echoes whether the plan reversed an install.
	Uninstall bool `json:"uninstall"`
	// Components lists per-component outcomes in execution order.
	Components []StepResult `json:"components"`
	// Tools lists per-tool outcomes in execution order.
	Tools []StepResult `json:"tools"`
}

// HasFailures reports whether any component or tool step failed.
func (r Result) HasFailures() bool {
	for _, s := range r.Components {
		if s.Action == ActionFailed {
			return true
		}
	}
	for _, s := range r.Tools {
		if s.Action == ActionFailed {
			return true
		}
	}
	return false
}

// Registry maps component and tool IDs to their compiled handlers. It is the
// authority on what the binary can act on.
type Registry struct {
	components map[ids.ComponentID]components.Installer
	tools      map[ids.ToolID]tools.Target
}

// NewRegistry builds a registry from explicit handler maps. Primarily for tests.
func NewRegistry(comps map[ids.ComponentID]components.Installer, tls map[ids.ToolID]tools.Target) *Registry {
	return &Registry{components: comps, tools: tls}
}

// DefaultRegistry wires the production component installers and tool targets.
func DefaultRegistry() *Registry {
	comps := map[ids.ComponentID]components.Installer{
		ids.ComponentCLI:    cli.New(),
		ids.ComponentSkills: skills.New(),
		ids.ComponentMCP:    mcp.New(),
	}
	tls := map[ids.ToolID]tools.Target{
		ids.ToolClaudeCode: claudecode.New(),
		ids.ToolCursor:     cursor.New(),
		ids.ToolCodex:      codex.New(),
		ids.ToolOpenCode:   opencode.New(),
	}
	return NewRegistry(comps, tls)
}

// Execute runs the plan and returns a structured Result. It never returns an
// error: individual step failures are recorded and execution continues, so a
// single misbehaving tool can't abort the whole run. Unknown IDs are skipped.
//
// Uninstall is deliberately scoped: it only removes the Render MCP entry from
// each target tool. The CLI and agent skills are left in place (removing them is
// best-effort and would make -r a misleading half-uninstall), so components are
// not touched on an uninstall run.
func (r *Registry) Execute(ctx context.Context, plan Plan) Result {
	res := Result{DryRun: plan.Options.DryRun, Uninstall: plan.Options.Uninstall}
	if plan.Options.Uninstall {
		res.Tools = r.executeTools(ctx, plan)
		return res
	}
	res.Components = r.executeComponents(ctx, plan)
	res.Tools = r.executeTools(ctx, plan)
	return res
}

// executeComponents installs the selected components in dependency order (CLI,
// then skills, then MCP). It is only used for installs; uninstall does not touch
// components.
func (r *Registry) executeComponents(ctx context.Context, plan Plan) []StepResult {
	selected := make(map[ids.ComponentID]bool, len(plan.Components))
	for _, c := range plan.Components {
		selected[c] = true
	}

	var out []StepResult
	acted := make(map[ids.ComponentID]bool)
	for _, id := range ids.AllComponents() {
		if !selected[id] {
			continue
		}
		acted[id] = true
		inst, ok := r.components[id]
		if !ok {
			out = append(out, StepResult{ID: string(id), Action: ActionSkipped, Detail: "unsupported component (no compiled handler)"})
			continue
		}
		out = append(out, r.runComponent(ctx, id, inst, plan))
	}

	// Any selected IDs not in the canonical set are unknown: skip them.
	for _, id := range plan.Components {
		if !acted[id] {
			out = append(out, StepResult{ID: string(id), Action: ActionSkipped, Detail: "unsupported component (no compiled handler)"})
			acted[id] = true
		}
	}
	return out
}

// runComponent installs one component, honoring dry run. When the plan is
// explicitly scoped to a subset of agents, that scope is forwarded so component
// installers (skills) only touch the selected agents; otherwise the component
// installs for all detected agents.
func (r *Registry) runComponent(ctx context.Context, id ids.ComponentID, inst components.Installer, plan Plan) StepResult {
	if plan.Options.DryRun {
		return StepResult{ID: string(id), Action: ActionPlanned, Detail: "would install"}
	}
	copts := components.Options{}
	if plan.Options.ScopedAgents {
		copts.Agents = plan.Tools
	}
	if err := inst.Install(ctx, copts); err != nil {
		return StepResult{ID: string(id), Action: ActionFailed, Detail: err.Error()}
	}
	return StepResult{ID: string(id), Action: ActionInstalled}
}

// executeTools configures or unconfigures each target tool. On install, tools
// are only configured when the MCP component is selected (that is the only
// per-tool config the wizard writes today); otherwise the tool is skipped.
func (r *Registry) executeTools(ctx context.Context, plan Plan) []StepResult {
	mcpSelected := false
	for _, c := range plan.Components {
		if c == ids.ComponentMCP {
			mcpSelected = true
			break
		}
	}

	var out []StepResult
	for _, id := range plan.Tools {
		target, ok := r.tools[id]
		if !ok {
			out = append(out, StepResult{ID: string(id), Action: ActionSkipped, Detail: "unsupported tool (no compiled handler)"})
			continue
		}
		out = append(out, r.runTool(ctx, id, target, mcpSelected, plan))
	}
	return out
}

// runTool configures or unconfigures one tool, honoring dry run and the
// MCP-selected gate on install.
func (r *Registry) runTool(ctx context.Context, id ids.ToolID, target tools.Target, mcpSelected bool, plan Plan) StepResult {
	if plan.Options.Uninstall {
		if plan.Options.DryRun {
			return StepResult{ID: string(id), Action: ActionPlanned, Detail: "would unconfigure"}
		}
		if err := target.Unconfigure(ctx); err != nil {
			return StepResult{ID: string(id), Action: ActionFailed, Detail: err.Error()}
		}
		return StepResult{ID: string(id), Action: ActionRemoved}
	}

	if !mcpSelected {
		return StepResult{ID: string(id), Action: ActionSkipped, Detail: "MCP not selected; nothing to configure"}
	}
	if plan.Options.DryRun {
		return StepResult{ID: string(id), Action: ActionPlanned, Detail: "would configure MCP"}
	}
	if err := target.Configure(ctx, tools.Selection{Components: plan.Components}); err != nil {
		return StepResult{ID: string(id), Action: ActionFailed, Detail: err.Error()}
	}
	return StepResult{ID: string(id), Action: ActionConfigured}
}

// Known reports whether the registry has a handler for the given tool ID.
func (r *Registry) Known(tool ids.ToolID) bool {
	_, ok := r.tools[tool]
	return ok
}
