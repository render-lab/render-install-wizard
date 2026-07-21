// Command render-setup installs and configures the Render CLI, agent skills, and
// the Render MCP server across a user's coding agents. It runs interactively when
// a TTY is present and behaves non-interactively (install defaults) otherwise,
// which is the path agents and CI use.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/render-lab/render-install-wizard/internal/cliflags"
	"github.com/render-lab/render-install-wizard/internal/detect"
	"github.com/render-lab/render-install-wizard/internal/ids"
	"github.com/render-lab/render-install-wizard/internal/orchestrator"
	"github.com/render-lab/render-install-wizard/internal/wizard"
)

// version is the wizard version, overridden at build time via -ldflags.
var version = "dev"

// runWizard and detectHasTTY are indirections over the interactive picker and
// TTY detection so tests can inject failures/among TTY states without a real
// terminal. Production wiring points at the real implementations.
var (
	runWizard    = wizard.Run
	detectHasTTY = func() bool { return detect.DetectPlatform().HasTTY }
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// run parses flags, resolves the plan (tools + components), executes it via the
// orchestrator, and prints the summary. It returns the process exit code.
func run(args []string) int {
	flags, err := cliflags.Parse(args)
	if err != nil {
		// -h/--help is a successful, non-mutating path: usage has already been
		// printed by the flag package, so exit 0 without treating it as an error.
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
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

	selection, cancelled, err := resolveSelection(ctx, flags, tools, &warnings)
	if err != nil {
		// Fail closed: an interactive session that errored never completed the
		// consent flow, so we must not proceed to install the defaults. Surface
		// the cause and exit nonzero before any plan is built or executed.
		for _, w := range warnings {
			fmt.Fprintln(os.Stderr, "warning: "+w)
		}
		fmt.Fprintln(os.Stderr, "error: "+err.Error())
		return 1
	}
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
			// An explicit --agent restricts the run to those agents; forward that
			// scope so component installers (skills) don't touch other agents.
			ScopedAgents: len(flags.Agents) > 0,
			// --pin-version pins the Render CLI release.
			Version: flags.PinVersion,
		},
	}
	result := reg.Execute(ctx, plan)
	nextSteps := orchestrator.NextSteps(plan, result)

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
// documented non-interactive cases (no TTY, -y, --json, or uninstall) it
// defaults to all components.
//
// The bool return reports user cancellation (a clean, no-change exit). The error
// return is reserved for an interactive picker that failed to run: rather than
// silently treating that as consent for the full default install, it is surfaced
// so the caller fails closed.
func resolveSelection(ctx context.Context, flags *cliflags.Flags, tools []ids.ToolID, warnings *[]string) (wizard.Selection, bool, error) {
	if len(flags.Components) > 0 {
		return componentsFromFlags(flags.Components, warnings), false, nil
	}

	interactive := !flags.Uninstall && !flags.Yes && !flags.JSON && detectHasTTY()
	if interactive {
		sel, confirmed, err := runWizard(ctx, tools)
		if err != nil {
			return wizard.Selection{}, false, fmt.Errorf("interactive setup failed; no changes made: %w", err)
		}
		if !confirmed {
			return wizard.Selection{}, true, nil
		}
		return sel, false, nil
	}
	return wizard.DefaultSelection(), false, nil
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
