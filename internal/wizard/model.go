// Package wizard drives the interactive install experience: a single-axis
// "what to install" picker built on bubbletea. Phase 1E collects and displays
// the selection only; performing installs lands in a later phase.
package wizard

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/render-lab/render-install-wizard/internal/ids"
)

// Model is the bubbletea model backing the single-axis component picker. It
// tracks the togglable component rows, the cursor position, the detected tools
// (shown for transparency), and whether the user confirmed or aborted.
type Model struct {
	components []component
	cursor     int
	detected   []ids.ToolID
	done       bool
	aborted    bool
}

// New constructs a Model with every component pre-checked, surfacing the given
// detected tools. It implements the detect-then-default policy.
func New(detected []ids.ToolID) Model {
	return Model{
		components: defaultComponents(),
		detected:   detected,
	}
}

// Init implements tea.Model. The picker has no startup command.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. It handles cursor movement, toggling, and the
// confirm/cancel keys, quitting the program on enter (confirm) or q/ctrl+c
// (cancel).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c", "q", "esc":
			m.aborted = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.components)-1 {
				m.cursor++
			}
		case " ", "x":
			toggle(m.components, m.cursor)
		case "enter":
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// View implements tea.Model, rendering the splash, the detected-tools line, and
// the checkbox list with a cursor indicator.
func (m Model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(SplashTitle))
	b.WriteByte('\n')
	b.WriteString(subtitleStyle.Render(SplashSubtitle))
	b.WriteString("\n\n")

	b.WriteString(detectedStyle.Render(m.detectedLine()))
	b.WriteString("\n\n")

	b.WriteString(promptStyle.Render("What should we set up?"))
	b.WriteByte('\n')

	for i, c := range m.components {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("> ")
		}
		box := "[ ]"
		if c.checked {
			box = checkedStyle.Render("[x]")
		}
		b.WriteString(cursor)
		b.WriteString(box)
		b.WriteByte(' ')
		b.WriteString(componentLabel(c.id))
		b.WriteByte('\n')
	}

	b.WriteByte('\n')
	b.WriteString(helpStyle.Render("up/down move - space toggle - enter confirm - q cancel"))
	b.WriteByte('\n')

	return b.String()
}

// detectedLine renders the "Detected: … — will configure all" transparency line.
func (m Model) detectedLine() string {
	if len(m.detected) == 0 {
		return "Detected: (none)"
	}
	names := make([]string, len(m.detected))
	for i, t := range m.detected {
		names[i] = toolName(t)
	}
	return "Detected: " + strings.Join(names, ", ") + " - will configure all"
}

// Result returns the chosen Selection and whether the user confirmed it.
// confirmed is false when the wizard was aborted (q/ctrl+c) or never finished.
func (m Model) Result() (Selection, bool) {
	if !m.done || m.aborted {
		return Selection{}, false
	}
	return selection(m.components), true
}

// Run launches the interactive picker, blocking until the user confirms or
// cancels. It returns the resulting Selection, whether it was confirmed, and any
// error from the tea program. The provided context cancels the program.
func Run(ctx context.Context, detected []ids.ToolID) (Selection, bool, error) {
	p := tea.NewProgram(New(detected), tea.WithContext(ctx))
	final, err := p.Run()
	if err != nil {
		return Selection{}, false, err
	}
	model, ok := final.(Model)
	if !ok {
		return Selection{}, false, nil
	}
	sel, confirmed := model.Result()
	return sel, confirmed, nil
}
