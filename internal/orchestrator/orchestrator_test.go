package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/render-oss/render-install-wizard/internal/components"
	"github.com/render-oss/render-install-wizard/internal/ids"
	"github.com/render-oss/render-install-wizard/internal/tools"
)

type fakeInstaller struct {
	id           ids.ComponentID
	log          *[]string
	installErr   error
	uninstallErr error
}

func (f *fakeInstaller) ID() ids.ComponentID                  { return f.id }
func (f *fakeInstaller) Detect(context.Context) (bool, error) { return false, nil }
func (f *fakeInstaller) Install(_ context.Context, _ components.Options) error {
	*f.log = append(*f.log, "install:"+string(f.id))
	return f.installErr
}
func (f *fakeInstaller) Uninstall(context.Context) error {
	*f.log = append(*f.log, "uninstall:"+string(f.id))
	return f.uninstallErr
}
func (f *fakeInstaller) Status(context.Context) (components.Status, error) {
	return components.Status{ID: f.id}, nil
}

type fakeTarget struct {
	id        ids.ToolID
	log       *[]string
	confErr   error
	unconfErr error
}

func (f *fakeTarget) ID() ids.ToolID                       { return f.id }
func (f *fakeTarget) Detect(context.Context) (bool, error) { return false, nil }
func (f *fakeTarget) PreferredDelivery() ids.Delivery      { return ids.DeliveryRaw }
func (f *fakeTarget) Configure(_ context.Context, _ tools.Selection) error {
	*f.log = append(*f.log, "configure:"+string(f.id))
	return f.confErr
}
func (f *fakeTarget) Unconfigure(context.Context) error {
	*f.log = append(*f.log, "unconfigure:"+string(f.id))
	return f.unconfErr
}

func fullRegistry(log *[]string) *Registry {
	comps := map[ids.ComponentID]components.Installer{
		ids.ComponentCLI:    &fakeInstaller{id: ids.ComponentCLI, log: log},
		ids.ComponentSkills: &fakeInstaller{id: ids.ComponentSkills, log: log},
		ids.ComponentMCP:    &fakeInstaller{id: ids.ComponentMCP, log: log},
	}
	tls := map[ids.ToolID]tools.Target{
		ids.ToolCursor: &fakeTarget{id: ids.ToolCursor, log: log},
		ids.ToolCodex:  &fakeTarget{id: ids.ToolCodex, log: log},
	}
	return NewRegistry(comps, tls)
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestExecuteInstallOrderAndActions(t *testing.T) {
	var log []string
	reg := fullRegistry(&log)
	plan := Plan{
		Components: []ids.ComponentID{ids.ComponentMCP, ids.ComponentCLI, ids.ComponentSkills}, // out of order on purpose
		Tools:      []ids.ToolID{ids.ToolCursor, ids.ToolCodex},
	}
	res := reg.Execute(context.Background(), plan)

	// Components must run in canonical order regardless of plan order.
	want := []string{"install:cli", "install:skills", "install:mcp", "configure:cursor", "configure:codex"}
	if !eq(log, want) {
		t.Fatalf("call log = %v, want %v", log, want)
	}
	if res.HasFailures() {
		t.Errorf("unexpected failures: %+v", res)
	}
	for _, s := range res.Components {
		if s.Action != ActionInstalled {
			t.Errorf("component %s action = %s, want installed", s.ID, s.Action)
		}
	}
	for _, s := range res.Tools {
		if s.Action != ActionConfigured {
			t.Errorf("tool %s action = %s, want configured", s.ID, s.Action)
		}
	}
}

func TestExecuteDryRunPerformsNoSideEffects(t *testing.T) {
	var log []string
	reg := fullRegistry(&log)
	plan := Plan{
		Components: ids.AllComponents(),
		Tools:      []ids.ToolID{ids.ToolCursor},
		Options:    Options{DryRun: true},
	}
	res := reg.Execute(context.Background(), plan)
	if len(log) != 0 {
		t.Fatalf("dry run invoked handlers: %v", log)
	}
	for _, s := range append(res.Components, res.Tools...) {
		if s.Action != ActionPlanned {
			t.Errorf("%s action = %s, want planned", s.ID, s.Action)
		}
	}
}

func TestExecuteUninstallOnlyUnconfiguresTools(t *testing.T) {
	var log []string
	reg := fullRegistry(&log)
	plan := Plan{
		Components: ids.AllComponents(), // ignored on uninstall
		Tools:      []ids.ToolID{ids.ToolCursor, ids.ToolCodex},
		Options:    Options{Uninstall: true},
	}
	res := reg.Execute(context.Background(), plan)

	// Scoped uninstall: only tool MCP entries are removed; components untouched.
	if !eq(log, []string{"unconfigure:cursor", "unconfigure:codex"}) {
		t.Fatalf("uninstall touched components or wrong order: %v", log)
	}
	if len(res.Components) != 0 {
		t.Errorf("uninstall reported components: %+v", res.Components)
	}
	for _, s := range res.Tools {
		if s.Action != ActionRemoved {
			t.Errorf("tool %s action = %s, want removed", s.ID, s.Action)
		}
	}
}

func TestExecuteSkipsUnknownIDs(t *testing.T) {
	var log []string
	reg := fullRegistry(&log)
	plan := Plan{
		Components: []ids.ComponentID{ids.ComponentCLI, ids.ComponentMCP, "totally-unknown"},
		Tools:      []ids.ToolID{ids.ToolCursor, "not-a-tool"},
	}
	res := reg.Execute(context.Background(), plan)

	if !eq(log, []string{"install:cli", "install:mcp", "configure:cursor"}) {
		t.Fatalf("unexpected handler calls: %v", log)
	}
	if got := findAction(res.Components, "totally-unknown"); got != ActionSkipped {
		t.Errorf("unknown component action = %s, want skipped", got)
	}
	if got := findAction(res.Tools, "not-a-tool"); got != ActionSkipped {
		t.Errorf("unknown tool action = %s, want skipped", got)
	}
}

func TestExecuteToolsSkippedWhenMCPNotSelected(t *testing.T) {
	var log []string
	reg := fullRegistry(&log)
	plan := Plan{
		Components: []ids.ComponentID{ids.ComponentSkills}, // no MCP
		Tools:      []ids.ToolID{ids.ToolCursor},
	}
	res := reg.Execute(context.Background(), plan)
	if !eq(log, []string{"install:skills"}) {
		t.Fatalf("tool should not be configured without MCP: %v", log)
	}
	if got := findAction(res.Tools, string(ids.ToolCursor)); got != ActionSkipped {
		t.Errorf("cursor action = %s, want skipped", got)
	}
}

func TestExecuteContinuesPastFailures(t *testing.T) {
	var log []string
	comps := map[ids.ComponentID]components.Installer{
		ids.ComponentCLI:    &fakeInstaller{id: ids.ComponentCLI, log: &log, installErr: errors.New("boom")},
		ids.ComponentSkills: &fakeInstaller{id: ids.ComponentSkills, log: &log},
		ids.ComponentMCP:    &fakeInstaller{id: ids.ComponentMCP, log: &log},
	}
	tls := map[ids.ToolID]tools.Target{ids.ToolCursor: &fakeTarget{id: ids.ToolCursor, log: &log}}
	reg := NewRegistry(comps, tls)

	res := reg.Execute(context.Background(), Plan{Components: ids.AllComponents(), Tools: []ids.ToolID{ids.ToolCursor}})

	if !res.HasFailures() {
		t.Fatal("expected HasFailures true")
	}
	// skills and mcp still ran despite cli failing.
	if !eq(log, []string{"install:cli", "install:skills", "install:mcp", "configure:cursor"}) {
		t.Fatalf("execution did not continue past failure: %v", log)
	}
	if got := findAction(res.Components, string(ids.ComponentCLI)); got != ActionFailed {
		t.Errorf("cli action = %s, want failed", got)
	}
}

func findAction(steps []StepResult, id string) Action {
	for _, s := range steps {
		if s.ID == id {
			return s.Action
		}
	}
	return ""
}
