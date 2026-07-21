package render

import (
	"strings"
	"testing"

	"github.com/render-oss/render-install-wizard/internal/ids"
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

func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// TestMCPConfigPathHonorsOverrides guards F11: each tool's documented config-home
// environment override is honored, defaults are used otherwise, and unaffected
// tools ignore unrelated variables.
func TestMCPConfigPathHonorsOverrides(t *testing.T) {
	const home = "/home/u"
	cases := []struct {
		name string
		tool ids.ToolID
		env  map[string]string
		want string
	}{
		{"claude default", ids.ToolClaudeCode, nil, "/home/u/.claude.json"},
		{"claude override", ids.ToolClaudeCode, map[string]string{"CLAUDE_CONFIG_DIR": "/cfg/claude"}, "/cfg/claude/.claude.json"},
		{"codex default", ids.ToolCodex, nil, "/home/u/.codex/config.toml"},
		{"codex override", ids.ToolCodex, map[string]string{"CODEX_HOME": "/cfg/codex"}, "/cfg/codex/config.toml"},
		{"cursor ignores xdg", ids.ToolCursor, map[string]string{"XDG_CONFIG_HOME": "/x"}, "/home/u/.cursor/mcp.json"},
		{"opencode default", ids.ToolOpenCode, nil, "/home/u/.config/opencode/opencode.json"},
		{"opencode xdg", ids.ToolOpenCode, map[string]string{"XDG_CONFIG_HOME": "/x"}, "/x/opencode/opencode.json"},
		{"opencode explicit file", ids.ToolOpenCode, map[string]string{"OPENCODE_CONFIG": "/custom/oc.jsonc"}, "/custom/oc.jsonc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := mcpConfigPath(tc.tool, home, envMap(tc.env))
			if !ok || got != tc.want {
				t.Errorf("mcpConfigPath(%s) = %q (ok=%v), want %q", tc.tool, got, ok, tc.want)
			}
		})
	}
}

// TestOpenCodeConfigFilesHonorsOverrides guards F11's OpenCode resolution and
// precedence: OPENCODE_CONFIG (explicit file) wins over the XDG/global directory.
func TestOpenCodeConfigFilesHonorsOverrides(t *testing.T) {
	const home = "/home/u"
	eq := func(got, want []string) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range want {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	if got := openCodeConfigFiles(home, envMap(nil)); !eq(got, []string{"/home/u/.config/opencode/opencode.jsonc", "/home/u/.config/opencode/opencode.json"}) {
		t.Errorf("default = %v", got)
	}
	if got := openCodeConfigFiles(home, envMap(map[string]string{"XDG_CONFIG_HOME": "/x"})); !eq(got, []string{"/x/opencode/opencode.jsonc", "/x/opencode/opencode.json"}) {
		t.Errorf("xdg = %v", got)
	}
	// OPENCODE_CONFIG names the exact active file and takes precedence over XDG.
	if got := openCodeConfigFiles(home, envMap(map[string]string{"OPENCODE_CONFIG": "/c/oc.jsonc", "XDG_CONFIG_HOME": "/x"})); !eq(got, []string{"/c/oc.jsonc"}) {
		t.Errorf("explicit = %v", got)
	}
}
