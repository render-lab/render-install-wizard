// Command render-setup installs the Render CLI, skills, and MCP into coding
// agents. Phase 0 provides a placeholder entrypoint that prints a dry-run plan.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/render-oss/render-install-wizard/internal/cliflags"
	"github.com/render-oss/render-install-wizard/internal/logx"
	"github.com/render-oss/render-install-wizard/internal/paths"
)

// version is the wizard version, overridden at build time via -ldflags.
var version = "dev"

func main() {
	flags, err := cliflags.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	if flags.ShowVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	log := logx.New(os.Stdout, flags.JSON)

	binaryPath, err := paths.BinaryPath()
	if err != nil {
		log.Errorf("resolve binary path: %v", err)
		os.Exit(1)
	}

	// TODO(phase 1+): replace this dry-run plan with the real install flow.
	log.Infof("render-setup %s (Phase 0 dry-run plan)", version)
	log.Infof("binary path: %s", binaryPath)
	log.Infof("default artifact: %s", paths.DefaultArtifactName(version))
	log.Infof("agents script: %s", paths.AgentsScriptURL)
	log.Infof("components: %s", joinOrNone(flags.Components))
	log.Infof("agents: %s", joinOrNone(flags.Agents))

	if flags.DryRun {
		log.Infof("dry-run enabled: no changes will be made")
	}
}

func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	return strings.Join(items, ", ")
}
