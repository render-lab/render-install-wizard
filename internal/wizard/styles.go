package wizard

import "github.com/charmbracelet/lipgloss"

// SplashTitle is the title shown on the wizard's splash screen. Kept ASCII-safe
// so it renders cleanly on any terminal.
const SplashTitle = "Setting up Render for agents"

// SplashSubtitle is the subtitle shown beneath the splash title.
const SplashSubtitle = "CLI, skills, and MCP for your coding agents"

// Lipgloss styles for the wizard TUI. Colors degrade gracefully on terminals
// without color support.
var (
	// titleStyle renders the splash title.
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("13"))
	// subtitleStyle renders the splash subtitle.
	subtitleStyle = lipgloss.NewStyle().Faint(true)
	// detectedStyle renders the "Detected: … — will configure all" line.
	detectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	// promptStyle renders the picker's question prompt.
	promptStyle = lipgloss.NewStyle().Bold(true)
	// cursorStyle renders the cursor indicator next to the active row.
	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	// checkedStyle renders a checked "[x]" box.
	checkedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	// helpStyle renders the key-hint help line.
	helpStyle = lipgloss.NewStyle().Faint(true)
)
