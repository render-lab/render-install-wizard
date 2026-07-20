package render

import (
	"strings"
	"testing"
)

// TestCLIArchiveURL locks the release-asset naming: the URL path uses a
// "v"-prefixed tag while the archive filename embeds the bare version, matching
// the official installer regardless of whether the caller supplies the "v".
func TestCLIArchiveURL(t *testing.T) {
	want := "https://github.com/render-oss/cli/releases/download/v1.2.3/cli_1.2.3_linux_amd64.zip"
	for _, version := range []string{"1.2.3", "v1.2.3"} {
		if got := CLIArchiveURL(version, "linux", "amd64"); got != want {
			t.Errorf("CLIArchiveURL(%q) = %q, want %q", version, got, want)
		}
	}

	if got := CLIArchiveURL("v2.0.0", "darwin", "arm64"); got != "https://github.com/render-oss/cli/releases/download/v2.0.0/cli_2.0.0_darwin_arm64.zip" {
		t.Errorf("darwin/arm64 URL = %q", got)
	}
}

// TestCLIArchiveName locks the archive filename used for checksum lookup.
func TestCLIArchiveName(t *testing.T) {
	if got := CLIArchiveName("v1.2.3", "linux", "arm64"); got != "cli_1.2.3_linux_arm64.zip" {
		t.Errorf("CLIArchiveName = %q", got)
	}
}

// TestCLIChecksumsURL locks the SHA256SUMS asset URL and its version pinning.
func TestCLIChecksumsURL(t *testing.T) {
	want := "https://github.com/render-oss/cli/releases/download/v1.2.3/cli_1.2.3_SHA256SUMS"
	for _, version := range []string{"1.2.3", "v1.2.3"} {
		if got := CLIChecksumsURL(version); got != want {
			t.Errorf("CLIChecksumsURL(%q) = %q, want %q", version, got, want)
		}
	}
}

// TestSkillsCLISpecIsPinned guards F05: the npx skills package must be pinned to
// an exact version rather than resolving "latest" at run time.
func TestSkillsCLISpecIsPinned(t *testing.T) {
	if SkillsCLISpec == "skills" || !strings.Contains(SkillsCLISpec, "@") {
		t.Errorf("SkillsCLISpec = %q, want a version-pinned spec like skills@x.y.z", SkillsCLISpec)
	}
}
