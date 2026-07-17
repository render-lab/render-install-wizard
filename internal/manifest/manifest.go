// Package manifest defines the remote manifest that describes the components and
// tools the wizard can install, along with parsing and validation helpers.
package manifest

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/render-oss/render-install-wizard/internal/ids"
)

// SchemaVersion is the manifest schema version this package understands.
const SchemaVersion = "1"

// ErrNotImplemented is returned by stubs whose behavior is not yet implemented.
var ErrNotImplemented = errors.New("not implemented")

// Manifest is the top-level remote manifest describing installable components
// and configurable tools.
type Manifest struct {
	Version    string      `json:"version"`
	Wizard     Wizard      `json:"wizard"`
	Components []Component `json:"components"`
	Tools      []Tool      `json:"tools"`
}

// Wizard carries wizard-level metadata such as the minimum supported version.
type Wizard struct {
	MinVersion string `json:"minVersion,omitempty"`
}

// Component describes an installable component ("what" to install).
type Component struct {
	ID         ids.ComponentID `json:"id"`
	Name       string          `json:"name"`
	Default    bool            `json:"default"`
	ContentURL string          `json:"contentURL,omitempty"`
}

// Tool describes a coding agent the wizard can configure ("where" to install).
type Tool struct {
	ID         ids.ToolID   `json:"id"`
	Name       string       `json:"name"`
	Delivery   ids.Delivery `json:"delivery"`
	ContentURL string       `json:"contentURL,omitempty"`
}

// Parse unmarshals manifest JSON and validates the resulting Manifest.
func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate checks that the Manifest is well-formed: a supported version, at
// least one component and tool, and a valid delivery for every tool.
func (m *Manifest) Validate() error {
	if m.Version != SchemaVersion {
		return fmt.Errorf("unsupported manifest version %q, want %q", m.Version, SchemaVersion)
	}
	if len(m.Components) == 0 {
		return errors.New("manifest has no components")
	}
	if len(m.Tools) == 0 {
		return errors.New("manifest has no tools")
	}
	for _, t := range m.Tools {
		switch t.Delivery {
		case ids.DeliveryRaw, ids.DeliveryPlugin:
		default:
			return fmt.Errorf("tool %q has invalid delivery %q", t.ID, t.Delivery)
		}
	}
	return nil
}
