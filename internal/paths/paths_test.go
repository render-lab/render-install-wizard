package paths

import (
	"path/filepath"
	"testing"

	"github.com/render-lab/render-install-wizard/internal/ids"
)

func TestArtifactName(t *testing.T) {
	tests := []struct {
		name         string
		goos, goarch string
		want         string
	}{
		{"darwin arm64", "darwin", "arm64", "render-setup_darwin_arm64"},
		{"linux amd64", "linux", "amd64", "render-setup_linux_amd64"},
		{"windows appends exe", "windows", "amd64", "render-setup_windows_amd64.exe"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ArtifactName(tc.goos, tc.goarch); got != tc.want {
				t.Errorf("ArtifactName(%q,%q) = %q, want %q", tc.goos, tc.goarch, got, tc.want)
			}
		})
	}
}

func TestDownloadURL(t *testing.T) {
	if got, want := DownloadURL("latest", "darwin", "arm64"),
		ReleasesBaseURL+"/latest/download/render-setup_darwin_arm64"; got != want {
		t.Errorf("DownloadURL(latest) = %q, want %q", got, want)
	}
	if got, want := DownloadURL("", "linux", "amd64"),
		ReleasesBaseURL+"/latest/download/render-setup_linux_amd64"; got != want {
		t.Errorf("DownloadURL(\"\") = %q, want %q", got, want)
	}
	if got, want := DownloadURL("v1.2.3", "linux", "arm64"),
		ReleasesBaseURL+"/download/v1.2.3/render-setup_linux_arm64"; got != want {
		t.Errorf("DownloadURL(v1.2.3) = %q, want %q", got, want)
	}
	if got, want := ChecksumsURL("v1.2.3"), ReleasesBaseURL+"/download/v1.2.3/checksums.txt"; got != want {
		t.Errorf("ChecksumsURL(v1.2.3) = %q, want %q", got, want)
	}
}

func TestToolContentURL(t *testing.T) {
	// claude-code deliberately maps to the /agents/claude.md slug.
	if got, want := ToolContentURL(ids.ToolClaudeCode), "https://render.com/agents/claude.md"; got != want {
		t.Errorf("ToolContentURL(claude-code) = %q, want %q", got, want)
	}
	if got, want := ToolContentURL(ids.ToolCursor), "https://render.com/agents/cursor.md"; got != want {
		t.Errorf("ToolContentURL(cursor) = %q, want %q", got, want)
	}
}

func TestRenderHomeHonorsEnv(t *testing.T) {
	t.Setenv("RENDER_HOME", "/tmp/custom-render-home")
	home, err := RenderHome()
	if err != nil {
		t.Fatalf("RenderHome: %v", err)
	}
	if home != "/tmp/custom-render-home" {
		t.Errorf("RenderHome = %q, want %q", home, "/tmp/custom-render-home")
	}
	bin, err := BinaryPath()
	if err != nil {
		t.Fatalf("BinaryPath: %v", err)
	}
	if want := filepath.Join("/tmp/custom-render-home", "bin", BinaryName); bin != want {
		t.Errorf("BinaryPath = %q, want %q", bin, want)
	}
}
