// Package paths encodes the runtime conventions for the Render install wizard:
// install locations, artifact naming, and content URLs. It is the single source
// of truth for these values so other packages never hardcode them.
package paths

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/render-oss/render-install-wizard/internal/ids"
)

// BinaryName is the on-disk name of the installed wizard binary.
const BinaryName = "render-setup"

// ChecksumsFileName is the name of the release checksums manifest.
const ChecksumsFileName = "checksums.txt"

// AgentsScriptURL is the canonical URL of the bootstrap install script.
const AgentsScriptURL = "https://render.com/agents.sh"

// AgentsBriefingURL is the URL of the agent-facing briefing document.
const AgentsBriefingURL = "https://render.com/agents.md"

// ContentBaseURL is the base URL under which per-tool content guides live.
const ContentBaseURL = "https://render.com/agents"

// RenderHome returns the Render home directory. It honors the RENDER_HOME
// environment variable when set, otherwise falling back to ~/.render.
func RenderHome() (string, error) {
	if home := os.Getenv("RENDER_HOME"); home != "" {
		return home, nil
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(userHome, ".render"), nil
}

// BinDir returns the directory where the wizard binary is installed.
func BinDir() (string, error) {
	home, err := RenderHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "bin"), nil
}

// BinaryPath returns the full path to the installed wizard binary.
func BinaryPath() (string, error) {
	dir, err := BinDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, BinaryName), nil
}

// ArtifactName returns the release artifact filename for the given version and
// platform. On Windows the ".exe" suffix is appended.
func ArtifactName(version, goos, goarch string) string {
	name := "render-setup_" + version + "_" + goos + "_" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

// DefaultArtifactName returns the artifact name for the current build platform.
func DefaultArtifactName(version string) string {
	return ArtifactName(version, runtime.GOOS, runtime.GOARCH)
}

// ToolContentURL returns the content guide URL for the given tool.
func ToolContentURL(t ids.ToolID) string {
	return ContentBaseURL + "/" + ids.ContentSlug(t) + ".md"
}
