// Command render-setup installs the Render CLI, skills, and MCP into coding
// agents. In Phase 1 it only detects tools and collects the component selection
// (interactively or not) and prints the resulting plan; it performs no installs.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/render-oss/render-install-wizard/internal/cliflags"
	"github.com/render-oss/render-install-wizard/internal/detect"
	"github.com/render-oss/render-install-wizard/internal/ids"
	"github.com/render-oss/render-install-wizard/internal/logx"
	"github.com/render-oss/render-install-wizard/internal/wizard"
)

// version is the wizard version, overridden at build time via -ldflags.
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

// run parses flags and drives the Phase 1 selection flow, returning the process
// exit code.
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

	log := logx.New(os.Stdout, flags.JSON)
	ctx := context.Background()

	// Best-effort tool detection: on error, proceed with no detected tools.
	detected, err := detect.DetectTools(ctx)
	if err != nil {
		log.Warnf("tool detection failed, continuing with none: %v", err)
		detected = nil
	}

	interactive := detect.DetectPlatform().HasTTY && !flags.Yes && !flags.JSON

	var selection wizard.Selection
	if interactive {
		sel, confirmed, err := wizard.Run(ctx, detected)
		if err != nil {
			log.Errorf("wizard failed: %v", err)
			return 1
		}
		if !confirmed {
			log.Infof("cancelled")
			return 0
		}
		selection = sel
	} else {
		selection = nonInteractiveSelection(log, flags, detected)
	}

	summary := wizard.Summary{Selection: selection, Tools: detected}
	log.Infof("plan (selection only, no changes made): %s", summary.String())
	log.Infof("this is Phase 1: render-setup collects the plan but does not install yet")
	return 0
}

// nonInteractiveSelection resolves the component selection without a TTY. With no
// --components flag it pre-checks all components (detect-then-default); otherwise
// it narrows to the requested components, warning about unknown IDs.
func nonInteractiveSelection(log *logx.Logger, flags *cliflags.Flags, detected []ids.ToolID) wizard.Selection {
	if len(flags.Components) == 0 {
		return wizard.PrecheckDefaults(detected)
	}

	valid := make(map[ids.ComponentID]bool, len(ids.AllComponents()))
	for _, c := range ids.AllComponents() {
		valid[c] = true
	}

	var chosen []ids.ComponentID
	seen := make(map[ids.ComponentID]bool)
	for _, name := range flags.Components {
		id := ids.ComponentID(name)
		if !valid[id] {
			log.Warnf("ignoring unknown component: %s", name)
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
