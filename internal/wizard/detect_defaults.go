package wizard

import "github.com/render-lab/render-install-wizard/internal/ids"

// PrecheckDefaults computes the default selection given the detected tools,
// following the "detect-then-default" policy: every user-selectable component is
// pre-checked regardless of which tools were detected, so a single confirmation
// installs everything. The detected list is surfaced for transparency (see the
// wizard View and Summary) but does not narrow the component selection.
func PrecheckDefaults(detected []ids.ToolID) Selection {
	_ = detected
	return DefaultSelection()
}
