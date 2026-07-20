// Package cliflags defines and parses the command-line flag surface for the
// render-setup wizard.
package cliflags

import (
	"flag"
	"fmt"
	"strings"
)

// Flags holds the parsed command-line options for the wizard.
type Flags struct {
	// Yes accepts all prompts non-interactively.
	Yes bool
	// Components lists the components to install (cli/skills/mcp).
	Components []string
	// Agents lists the coding agents to configure.
	Agents []string
	// NoLogin skips the Render login step.
	NoLogin bool
	// JSON emits machine-readable JSON logs.
	JSON bool
	// Uninstall removes the Render MCP entry from configured tools. It does not
	// remove the CLI or skills.
	Uninstall bool
	// ShowVersion prints the version and exits.
	ShowVersion bool
	// DryRun prints the plan without making changes.
	DryRun bool
	// PinVersion pins the wizard/CLI to a specific version.
	PinVersion string
}

// stringSliceValue is a flag.Value that accumulates repeated string flags.
type stringSliceValue []string

// String returns the comma-joined accumulated values.
func (s *stringSliceValue) String() string {
	return strings.Join(*s, ",")
}

// Set appends a value to the slice.
func (s *stringSliceValue) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// Parse parses the given argument list into a Flags value. Parse errors from the
// underlying flag set are propagated to the caller.
func Parse(args []string) (*Flags, error) {
	f := &Flags{}
	fs := flag.NewFlagSet("render-setup", flag.ContinueOnError)

	fs.BoolVar(&f.Yes, "y", false, "accept all prompts")
	fs.BoolVar(&f.Yes, "yes", false, "accept all prompts")

	fs.BoolVar(&f.Uninstall, "r", false, "remove the Render MCP entry from configured tools (leaves the CLI and skills in place)")
	fs.BoolVar(&f.Uninstall, "uninstall", false, "remove the Render MCP entry from configured tools (leaves the CLI and skills in place)")

	var components string
	fs.StringVar(&components, "components", "", "comma-separated components to install (cli,skills,mcp)")

	var agents stringSliceValue
	fs.Var(&agents, "agent", "coding agent to configure (repeatable)")

	fs.BoolVar(&f.NoLogin, "no-login", false, "skip the Render login step")
	fs.BoolVar(&f.JSON, "json", false, "emit machine-readable JSON logs")
	fs.BoolVar(&f.ShowVersion, "version", false, "print the version and exit")
	fs.BoolVar(&f.DryRun, "dry-run", false, "print the plan without making changes")
	fs.StringVar(&f.PinVersion, "pin-version", "", "pin to a specific version")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	// Reject leftover positional operands. Go's flag parser stops at the first
	// non-flag token, so a stray operand (e.g. a subcommand like "install")
	// would silently swallow the flags after it — turning `install --dry-run`
	// into a real install. This tool takes only flags, so any operand is an
	// error rather than a dangerous no-op. flag.ErrHelp is handled above and
	// never reaches here.
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unexpected argument(s): %s; render-setup takes only flags (did you mean a flag like --dry-run?)", strings.Join(fs.Args(), " "))
	}

	for _, c := range strings.Split(components, ",") {
		c = strings.TrimSpace(c)
		if c != "" {
			f.Components = append(f.Components, c)
		}
	}

	f.Agents = agents

	return f, nil
}
