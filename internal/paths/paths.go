// Package paths encodes the runtime conventions for the Render install wizard:
// install locations, artifact naming, and content URLs. It is the single source
// of truth for these values so other packages never hardcode them.
package paths

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/render-lab/render-install-wizard/internal/ids"
)

// BinaryName is the on-disk name of the installed wizard binary.
const BinaryName = "render-setup"

// ChecksumsFileName is the name of the release checksums manifest.
const ChecksumsFileName = "checksums.txt"

// ReleasesBaseURL is the base URL for published wizard release artifacts. Assets
// are hosted on GitHub Releases so the "latest" download redirect is available.
const ReleasesBaseURL = "https://github.com/render-lab/render-install-wizard/releases"

// LatestVersion is the version token that resolves to the newest release via
// GitHub's /releases/latest/download/ redirect.
const LatestVersion = "latest"

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

// ArtifactName returns the release artifact (raw binary) filename for the given
// platform. Names are intentionally version-less so the "latest" download URL is
// stable; the concrete version is carried by the release tag in the URL path (or
// reported by `render-setup --version`). On Windows a ".exe" suffix is appended.
func ArtifactName(goos, goarch string) string {
	name := "render-setup_" + goos + "_" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

// DefaultArtifactName returns the artifact name for the current build platform.
func DefaultArtifactName() string {
	return ArtifactName(runtime.GOOS, runtime.GOARCH)
}

// downloadDir returns the release download directory for a version token. An
// empty string or LatestVersion uses GitHub's latest-release redirect; any other
// value is treated as a release tag (e.g. "v1.2.3").
func downloadDir(version string) string {
	if version == "" || version == LatestVersion {
		return ReleasesBaseURL + "/latest/download"
	}
	return ReleasesBaseURL + "/download/" + version
}

// DownloadURL returns the download URL of the wizard binary for the given
// version token and platform.
func DownloadURL(version, goos, goarch string) string {
	return downloadDir(version) + "/" + ArtifactName(goos, goarch)
}

// ChecksumsURL returns the download URL of the checksums file for the given
// version token.
func ChecksumsURL(version string) string {
	return downloadDir(version) + "/" + ChecksumsFileName
}

// ToolContentURL returns the content guide URL for the given tool.
func ToolContentURL(t ids.ToolID) string {
	return ContentBaseURL + "/" + ids.ContentSlug(t) + ".md"
}
