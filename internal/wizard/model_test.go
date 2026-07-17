package wizard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/render-oss/render-install-wizard/internal/ids"
)

// step feeds a key message through Update and returns the model cast back to
// Model, failing the test if the cast is impossible.
func step(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, _ := m.Update(msg)
	out, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", next)
	}
	return out
}

func key(t tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: t}
}

func runeKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestNewPreChecksAllComponents(t *testing.T) {
	m := New([]ids.ToolID{ids.ToolCursor})
	for _, c := range m.components {
		if !c.checked {
			t.Fatalf("component %s not pre-checked", c.id)
		}
	}
	if len(m.components) != len(ids.AllComponents()) {
		t.Fatalf("got %d components, want %d", len(m.components), len(ids.AllComponents()))
	}
}

func TestEnterConfirmsDefaultSelection(t *testing.T) {
	m := New(nil)
	m = step(t, m, key(tea.KeyEnter))

	sel, confirmed := m.Result()
	if !confirmed {
		t.Fatal("expected confirmed after enter")
	}
	want := ids.AllComponents()
	if len(sel.Components) != len(want) {
		t.Fatalf("got %d components, want %d", len(sel.Components), len(want))
	}
	for i, c := range want {
		if sel.Components[i] != c {
			t.Fatalf("component %d = %s, want %s", i, sel.Components[i], c)
		}
	}
}

func TestToggleUnchecksAndRechecks(t *testing.T) {
	m := New(nil)

	// Cursor starts on the first component (cli). Toggle it off.
	m = step(t, m, key(tea.KeySpace))
	m = step(t, m, key(tea.KeyEnter))
	sel, confirmed := m.Result()
	if !confirmed {
		t.Fatal("expected confirmed")
	}
	if containsComponent(sel.Components, ids.AllComponents()[0]) {
		t.Fatalf("expected %s to be unchecked", ids.AllComponents()[0])
	}

	// Fresh model: toggle off then on again -> back to default.
	m = New(nil)
	m = step(t, m, key(tea.KeySpace))
	m = step(t, m, key(tea.KeySpace))
	m = step(t, m, key(tea.KeyEnter))
	sel, _ = m.Result()
	if !containsComponent(sel.Components, ids.AllComponents()[0]) {
		t.Fatalf("expected %s re-checked after double toggle", ids.AllComponents()[0])
	}
}

func TestCursorMovementTogglesSecondComponent(t *testing.T) {
	m := New(nil)
	m = step(t, m, key(tea.KeyDown))
	m = step(t, m, key(tea.KeySpace))
	m = step(t, m, key(tea.KeyEnter))

	sel, _ := m.Result()
	second := ids.AllComponents()[1]
	if containsComponent(sel.Components, second) {
		t.Fatalf("expected %s unchecked after moving cursor down and toggling", second)
	}
	// First component should remain checked.
	if !containsComponent(sel.Components, ids.AllComponents()[0]) {
		t.Fatalf("expected %s still checked", ids.AllComponents()[0])
	}
}

func TestQuitAborts(t *testing.T) {
	m := New(nil)
	m = step(t, m, runeKey('q'))
	if _, confirmed := m.Result(); confirmed {
		t.Fatal("expected not confirmed after q")
	}
}

func TestCtrlCAborts(t *testing.T) {
	m := New(nil)
	m = step(t, m, key(tea.KeyCtrlC))
	if _, confirmed := m.Result(); confirmed {
		t.Fatal("expected not confirmed after ctrl+c")
	}
}

func TestResultUnconfirmedBeforeEnter(t *testing.T) {
	m := New(nil)
	if _, confirmed := m.Result(); confirmed {
		t.Fatal("expected not confirmed before enter")
	}
}

func TestViewContainsSplashAndDetected(t *testing.T) {
	m := New([]ids.ToolID{ids.ToolClaudeCode, ids.ToolCursor})
	view := m.View()
	if !strings.Contains(view, SplashTitle) {
		t.Errorf("view missing splash title: %q", view)
	}
	if !strings.Contains(view, "Claude Code") || !strings.Contains(view, "Cursor") {
		t.Errorf("view missing detected tools: %q", view)
	}
	if !strings.Contains(view, "[x]") {
		t.Errorf("view missing checkbox: %q", view)
	}
}

func TestDefaultSelectionIsAllComponents(t *testing.T) {
	sel := DefaultSelection()
	want := ids.AllComponents()
	if len(sel.Components) != len(want) {
		t.Fatalf("got %d, want %d", len(sel.Components), len(want))
	}
	for i, c := range want {
		if sel.Components[i] != c {
			t.Fatalf("component %d = %s, want %s", i, sel.Components[i], c)
		}
	}
}

func TestPrecheckDefaultsIgnoresDetected(t *testing.T) {
	sel := PrecheckDefaults([]ids.ToolID{ids.ToolCodex})
	if len(sel.Components) != len(ids.AllComponents()) {
		t.Fatalf("got %d components, want %d", len(sel.Components), len(ids.AllComponents()))
	}
}

func TestSummaryStringContainsIDs(t *testing.T) {
	s := Summary{
		Selection: DefaultSelection(),
		Tools:     []ids.ToolID{ids.ToolCursor, ids.ToolCodex},
	}
	out := s.String()
	for _, c := range ids.AllComponents() {
		if !strings.Contains(out, string(c)) {
			t.Errorf("summary %q missing component %s", out, c)
		}
	}
	if !strings.Contains(out, string(ids.ToolCursor)) || !strings.Contains(out, string(ids.ToolCodex)) {
		t.Errorf("summary %q missing detected tools", out)
	}
}

func TestSummaryStringEmpty(t *testing.T) {
	s := Summary{}
	out := s.String()
	if !strings.Contains(out, "(none)") {
		t.Errorf("expected (none) in empty summary, got %q", out)
	}
}

func containsComponent(list []ids.ComponentID, id ids.ComponentID) bool {
	for _, c := range list {
		if c == id {
			return true
		}
	}
	return false
}
