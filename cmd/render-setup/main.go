// Command render-setup installs and configures the Render CLI, agent skills, and
// the Render MCP server across a user's coding agents. It runs interactively when
// a TTY is present and behaves non-interactively (install defaults) otherwise,
// which is the path agents and CI use.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/render-oss/render-install-wizard/internal/cliflags"
	"github.com/render-oss/render-install-wizard/internal/detect"
	"github.com/render-oss/render-install-wizard/internal/ids"
	"github.com/render-oss/render-install-wizard/internal/orchestrator"
	"github.com/render-oss/render-install-wizard/internal/wizard"
)

// version is the wizard version, overridden at build time via -ldflags.
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

// run parses flags, resolves the plan (tools + components), executes it via the
// orchestrator, and prints the summary. It returns the process exit code.
func run(args []string) int {
	flags, err := cliflags.Parse(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if flags.ShowVersion {
		fmt.Println(version)
		return 0
	}

	ctx := context.Background()
	reg := orchestrator.DefaultRegistry()
	var warnings []string

	tools := resolveTools(ctx, flags, reg, &warnings)
	if len(tools) == 0 && !flags.Uninstall {
		warnings = append(warnings, "no supported coding agents detected; components that require a tool (MCP) won't be configured anywhere")
	}

	selection, cancelled := resolveSelection(ctx, flags, tools, &warnings)
	if cancelled {
		fmt.Println("Cancelled. No changes made.")
		return 0
	}

	plan := orchestrator.Plan{
		Components: selection.Components,
		Tools:      tools,
		Options: orchestrator.Options{
			DryRun:    flags.DryRun,
			Uninstall: flags.Uninstall,
			NoLogin:   flags.NoLogin,
		},
	}
	result := reg.Execute(ctx, plan)
	nextSteps := orchestrator.NextSteps(plan)

	if flags.JSON {
		emitJSON(result, nextSteps, warnings)
	} else {
		emitText(result, nextSteps, warnings)
	}

	if result.HasFailures() {
		return 1
	}
	return 0
}

// resolveTools determines the target tools: the --agent flags when given
// (unknown agents are warned and dropped), otherwise all detected tools.
func resolveTools(ctx context.Context, flags *cliflags.Flags, reg *orchestrator.Registry, warnings *[]string) []ids.ToolID {
	if len(flags.Agents) > 0 {
		var out []ids.ToolID
		for _, name := range flags.Agents {
			id := ids.ToolID(name)
			if !reg.Known(id) {
				*warnings = append(*warnings, fmt.Sprintf("ignoring unknown agent: %s", name))
				continue
			}
			out = append(out, id)
		}
		return out
	}

	detected, err := detect.DetectTools(ctx)
	if err != nil {
		*warnings = append(*warnings, fmt.Sprintf("tool detection failed, continuing with none: %v", err))
		return nil
	}
	return detected
}

// resolveSelection determines which components to act on. An explicit
// --components flag wins (unknown IDs are warned and dropped). Otherwise, when a
// TTY is present for a fresh install, the interactive picker runs; in all other
// cases (no TTY, -y, --json, or uninstall) it defaults to all components.
func resolveSelection(ctx context.Context, flags *cliflags.Flags, tools []ids.ToolID, warnings *[]string) (wizard.Selection, bool) {
	if len(flags.Components) > 0 {
		return componentsFromFlags(flags.Components, warnings), false
	}

	interactive := !flags.Uninstall && !flags.Yes && !flags.JSON && detect.DetectPlatform().HasTTY
	if interactive {
		sel, confirmed, err := wizard.Run(ctx, tools)
		if err != nil {
			*warnings = append(*warnings, fmt.Sprintf("interactive picker failed, using defaults: %v", err))
			return wizard.DefaultSelection(), false
		}
		if !confirmed {
			return wizard.Selection{}, true
		}
		return sel, false
	}
	return wizard.DefaultSelection(), false
}

// componentsFromFlags maps --components values to known component IDs, warning
// about and dropping any that aren't recognized, and de-duplicating.
func componentsFromFlags(names []string, warnings *[]string) wizard.Selection {
	valid := make(map[ids.ComponentID]bool, len(ids.AllComponents()))
	for _, c := range ids.AllComponents() {
		valid[c] = true
	}
	var chosen []ids.ComponentID
	seen := make(map[ids.ComponentID]bool)
	for _, name := range names {
		id := ids.ComponentID(name)
		if !valid[id] {
			*warnings = append(*warnings, fmt.Sprintf("ignoring unknown component: %s", name))
			continue
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		chosen = append(chosen, id)
	}
	return wizard.Selection{Components: chosen}
}

// jsonOutput is the documented machine-readable shape emitted under --json.
type jsonOutput struct {
	DryRun     bool                      `json:"dryRun"`
	Uninstall  bool                      `json:"uninstall"`
	Warnings   []string                  `json:"warnings,omitempty"`
	Components []orchestrator.StepResult `json:"components"`
	Tools      []orchestrator.StepResult `json:"tools"`
	NextSteps  []string                  `json:"nextSteps"`
}

// emitJSON prints a single JSON object describing the result (no ANSI, no logs).
func emitJSON(result orchestrator.Result, nextSteps, warnings []string) {
	out := jsonOutput{
		DryRun:     result.DryRun,
		Uninstall:  result.Uninstall,
		Warnings:   warnings,
		Components: result.Components,
		Tools:      result.Tools,
		NextSteps:  nextSteps,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	// Encoding a plain struct to stdout does not fail in practice; ignore error.
	_ = enc.Encode(out)
}

// emitText prints warnings to stderr and the human-readable summary + next steps
// to stdout.
func emitText(result orchestrator.Result, nextSteps, warnings []string) {
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "warning: "+w)
	}
	fmt.Println(result.Summary())
	if len(nextSteps) > 0 {
		fmt.Println("\nNext steps:")
		for _, s := range nextSteps {
			fmt.Println("  - " + s)
		}
	}
}
