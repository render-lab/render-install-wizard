package orchestrator

import (
	"strings"
	"testing"

	"github.com/render-lab/render-install-wizard/internal/ids"
)

// TestTitleReflectsOutcome guards F14: the headline is derived from actual step
// outcomes, not a fixed "complete".
func TestTitleReflectsOutcome(t *testing.T) {
	cases := []struct {
		name string
		res  Result
		want string
	}{
		{
			name: "all failed",
			res:  Result{Components: []StepResult{{ID: "cli", Action: ActionFailed}}},
			want: "Render setup failed",
		},
		{
			name: "partial",
			res: Result{
				Components: []StepResult{{ID: "cli", Action: ActionInstalled}},
				Tools:      []StepResult{{ID: "cursor", Action: ActionFailed}},
			},
			want: "Render setup completed with errors",
		},
		{
			name: "nothing changed",
			res:  Result{Components: []StepResult{{ID: "cli", Action: ActionUnchanged}}},
			want: "Render setup: already up to date",
		},
		{
			name: "complete",
			res: Result{
				Components: []StepResult{{ID: "cli", Action: ActionInstalled}},
				Tools:      []StepResult{{ID: "cursor", Action: ActionConfigured}},
			},
			want: "Render setup complete",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.res.title(); got != tc.want {
				t.Errorf("title = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestNextStepsSuppressedOnFailure guards F14: follow-up steps whose prerequisite
// step failed are not offered.
func TestNextStepsSuppressedOnFailure(t *testing.T) {
	plan := Plan{
		Components: []ids.ComponentID{ids.ComponentCLI, ids.ComponentMCP},
		Tools:      []ids.ToolID{ids.ToolCursor},
	}
	res := Result{
		Components: []StepResult{{ID: string(ids.ComponentCLI), Action: ActionFailed}},
		Tools:      []StepResult{{ID: string(ids.ToolCursor), Action: ActionFailed}},
	}
	steps := NextSteps(plan, res)
	joined := strings.Join(steps, "\n")
	if strings.Contains(joined, "render login") {
		t.Error("should not suggest `render login` after the CLI step failed")
	}
	if strings.Contains(joined, "authorize the MCP server") {
		t.Error("should not mention MCP authorization when no tool was configured")
	}
	if strings.Contains(joined, "Try it") {
		t.Error("should not offer generic guidance when nothing succeeded")
	}
}

// TestNextStepsOnSuccess confirms the success path still surfaces login, MCP, and
// per-tool plugin guidance for configured tools.
func TestNextStepsOnSuccess(t *testing.T) {
	plan := Plan{
		Components: []ids.ComponentID{ids.ComponentCLI, ids.ComponentMCP},
		Tools:      []ids.ToolID{ids.ToolCursor},
	}
	res := Result{
		Components: []StepResult{
			{ID: string(ids.ComponentCLI), Action: ActionInstalled},
			{ID: string(ids.ComponentMCP), Action: ActionConfigured},
		},
		Tools: []StepResult{{ID: string(ids.ToolCursor), Action: ActionConfigured}},
	}
	joined := strings.Join(NextSteps(plan, res), "\n")
	if !strings.Contains(joined, "render login") {
		t.Error("expected a `render login` step on a successful CLI install")
	}
	if !strings.Contains(joined, "authorize the MCP server") {
		t.Error("expected the MCP authorization note when a tool was configured")
	}
	if !strings.Contains(joined, "/add-plugin render") {
		t.Error("expected Cursor's in-app plugin next step")
	}
}

// TestNextStepsRespectsNoLogin confirms --no-login suppresses the login step even
// on a successful CLI install.
func TestNextStepsRespectsNoLogin(t *testing.T) {
	plan := Plan{
		Components: []ids.ComponentID{ids.ComponentCLI},
		Options:    Options{NoLogin: true},
	}
	res := Result{Components: []StepResult{{ID: string(ids.ComponentCLI), Action: ActionInstalled}}}
	if strings.Contains(strings.Join(NextSteps(plan, res), "\n"), "render login") {
		t.Error("--no-login should suppress the render login step")
	}
}
