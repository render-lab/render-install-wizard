package wizard

import "github.com/render-oss/render-install-wizard/internal/ids"

// Selection captures the components the user has chosen to install.
type Selection struct {
	// Components are the chosen components.
	Components []ids.ComponentID
}

// DefaultSelection returns the selection containing all components.
func DefaultSelection() Selection {
	return Selection{Components: ids.AllComponents()}
}
