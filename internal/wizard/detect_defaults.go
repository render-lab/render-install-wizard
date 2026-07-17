package wizard

import "github.com/render-oss/render-install-wizard/internal/ids"

// PrecheckDefaults computes the default selection given the detected tools,
// following a "detect-then-default" strategy. It is currently a stub.
func PrecheckDefaults(detected []ids.ToolID) Selection {
	_ = detected
	// TODO(phase 1E): tailor defaults based on which tools were detected.
	return DefaultSelection()
}
