// Package render is the single source of truth for Render-specific installation
// facts: the MCP server endpoint and auth mode, per-tool config file locations,
// the skills source, the universal skills directory, and the recommended plugin
// next-steps per tool. Installer and tool packages read these values so none of
// them hardcode Render URLs, paths, or config shapes independently.
package render

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/render-lab/render-install-wizard/internal/ids"
)

// MCP server identity. The wizard writes an MCP entry named MCPServerName that
// points at MCPServerURL into each detected tool's config.
const (
	// MCPServerName is the key/name used for Render's MCP server entry.
	MCPServerName = "render"
	// MCPServerURL is the hosted Render MCP endpoint.
	MCPServerURL = "https://mcp.render.com/mcp"
)

// SkillsRepo is the GitHub repository the official skills installer pulls from.
// It is a first-party Render repository (same trust domain as the Render CLI).
const SkillsRepo = "render-oss/skills"

// SkillsCLISpec pins the third-party `skills` npm package to an exact version so
// `npx` executes a known, immutable installer rather than resolving whatever is
// latest at run time (a supply-chain hazard, since npx would run unverified code
// on every install). Bump this deliberately when adopting a newer skills CLI.
const SkillsCLISpec = "skills@1.5.19"

// Render CLI install facts.
const (
	// CLIBinaryName is the Render CLI executable name.
	CLIBinaryName = "render"
	// CLIRepo is the Render CLI source repository.
	CLIRepo = "render-oss/cli"
	// CLIInstallScriptURL is the official Linux/macOS CLI install script,
	// suitable for `curl -fsSL <url> | sh`. The wizard does not pipe it to a
	// shell (it installs non-root binaries to ~/.local/bin, outside the
	// wizard-owned tree); it is retained as a reference to the upstream source.
	CLIInstallScriptURL = "https://raw.githubusercontent.com/render-oss/cli/refs/heads/main/bin/install.sh"
	// CLILatestReleaseAPIURL returns metadata (including tag_name) for the most
	// recent Render CLI release. It mirrors what the official install script
	// queries to resolve "latest".
	CLILatestReleaseAPIURL = "https://api.github.com/repos/render-oss/cli/releases/latest"
)

// cliReleaseTag normalizes a version to the "v"-prefixed release tag used in
// GitHub release URLs (the archive filenames embed the version without the "v").
func cliReleaseTag(version string) string {
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

// CLIArchiveName returns the release archive filename for the given version and
// platform, matching the official installer's naming (cli_<version>_<os>_<arch>.zip).
// goos/goarch use Go's conventions (linux/darwin, amd64/arm64), which already
// match the published asset names.
func CLIArchiveName(version, goos, goarch string) string {
	num := strings.TrimPrefix(cliReleaseTag(version), "v")
	return fmt.Sprintf("cli_%s_%s_%s.zip", num, goos, goarch)
}

// CLIArchiveURL returns the download URL of the Render CLI release archive for
// the given version and platform.
func CLIArchiveURL(version, goos, goarch string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", CLIRepo, cliReleaseTag(version), CLIArchiveName(version, goos, goarch))
}

// CLIChecksumsURL returns the download URL of the release's published SHA-256
// checksums file (cli_<version>_SHA256SUMS), served alongside the archives so
// the wizard can verify a downloaded archive before executing it.
func CLIChecksumsURL(version string) string {
	tag := cliReleaseTag(version)
	num := strings.TrimPrefix(tag, "v")
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/cli_%s_SHA256SUMS", CLIRepo, tag, num)
}

// AuthMode selects how the MCP entry authenticates.
type AuthMode string

const (
	// AuthModeOAuth writes credential-free MCP entries; the tool performs a
	// browser sign-in on first use. This is the default.
	AuthModeOAuth AuthMode = "oauth"
	// AuthModeAPIKey writes an Authorization: Bearer header that references an
	// environment variable (never a stored secret). This is the documented
	// fallback for tools/servers that do not yet support MCP OAuth.
	AuthModeAPIKey AuthMode = "api-key"
)

// DefaultAuthMode is the auth mode used unless overridden. OAuth is assumed live;
// flip this (or pass an explicit mode) to fall back to API-key config.
const DefaultAuthMode = AuthModeOAuth

// APIKeyEnvVar is the environment variable the API-key fallback references.
const APIKeyEnvVar = "RENDER_API_KEY"

// AuthorizationHeader returns the Authorization header value to embed in an MCP
// entry for the given auth mode, and whether a header should be written at all.
//
// For OAuth, present is false (no header — sign-in happens on first use). For
// API-key, it returns an env-var reference in shell form ("Bearer $RENDER_API_KEY")
// so no secret is ever written to disk. Callers targeting configs that use a
// different interpolation syntax (e.g. OpenCode's "{env:RENDER_API_KEY}") should
// use APIKeyEnvVar to build their own reference.
func AuthorizationHeader(mode AuthMode) (value string, present bool) {
	if mode == AuthModeAPIKey {
		return "Bearer $" + APIKeyEnvVar, true
	}
	return "", false
}

// UniversalSkillsDir returns the tool-agnostic skills directory (~/.agents/skills)
// under the given home directory.
func UniversalSkillsDir(home string) string {
	return filepath.Join(home, ".agents", "skills")
}

// OpenCodeConfigFiles returns OpenCode's config file candidates under home, in
// precedence order (highest first). OpenCode loads both files and lets
// opencode.jsonc override opencode.json for conflicting keys, so the wizard edits
// the .jsonc form when it exists and otherwise the .json form. The .jsonc form
// permits comments and trailing commas (JSONC), which the JSON editor in
// internal/configedit preserves.
//
// It honors OpenCode's documented overrides: OPENCODE_CONFIG names an explicit
// config file (higher precedence than the global directory, so editing it makes
// the change active), and XDG_CONFIG_HOME relocates the global directory.
func OpenCodeConfigFiles(home string) []string {
	return openCodeConfigFiles(home, os.Getenv)
}

func openCodeConfigFiles(home string, getenv func(string) string) []string {
	if f := getenv("OPENCODE_CONFIG"); f != "" {
		return []string{f}
	}
	base := filepath.Join(xdgConfigHome(home, getenv), "opencode")
	return []string{
		filepath.Join(base, "opencode.jsonc"),
		filepath.Join(base, "opencode.json"),
	}
}

// xdgConfigHome returns $XDG_CONFIG_HOME or the ~/.config default under home.
func xdgConfigHome(home string, getenv func(string) string) string {
	if dir := getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir
	}
	return filepath.Join(home, ".config")
}

// OpenCodeSchemaURL is the value written to "$schema" when the wizard creates a
// new OpenCode config file (for editor completion). An existing file's $schema
// is never modified.
const OpenCodeSchemaURL = "https://opencode.ai/config.json"

// MCPConfigPath returns the MCP/config file path for the given tool under home,
// honoring each tool's documented configuration-home environment overrides
// (CLAUDE_CONFIG_DIR, CODEX_HOME, OPENCODE_CONFIG, XDG_CONFIG_HOME). It returns
// false for tools without a known config-file location. For OpenCode this is the
// default file; use OpenCodeConfigFiles to honor an existing .jsonc file's
// precedence when reading or writing.
func MCPConfigPath(tool ids.ToolID, home string) (string, bool) {
	return mcpConfigPath(tool, home, os.Getenv)
}

func mcpConfigPath(tool ids.ToolID, home string, getenv func(string) string) (string, bool) {
	switch tool {
	case ids.ToolClaudeCode:
		// CLAUDE_CONFIG_DIR relocates ~/.claude.json to $CLAUDE_CONFIG_DIR/.claude.json.
		return filepath.Join(claudeConfigDir(home, getenv), ".claude.json"), true
	case ids.ToolCursor:
		return filepath.Join(home, ".cursor", "mcp.json"), true
	case ids.ToolCodex:
		// CODEX_HOME relocates the whole ~/.codex tree.
		return filepath.Join(codexHome(home, getenv), "config.toml"), true
	case ids.ToolOpenCode:
		files := openCodeConfigFiles(home, getenv)
		return files[len(files)-1], true
	default:
		return "", false
	}
}

// claudeConfigDir returns the directory holding Claude Code's .claude.json:
// $CLAUDE_CONFIG_DIR when set, otherwise home (the default ~/.claude.json).
func claudeConfigDir(home string, getenv func(string) string) string {
	if dir := getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	return home
}

// codexHome returns Codex's config root: $CODEX_HOME when set, otherwise ~/.codex.
func codexHome(home string, getenv func(string) string) string {
	if dir := getenv("CODEX_HOME"); dir != "" {
		return dir
	}
	return filepath.Join(home, ".codex")
}

// PluginKind describes how a tool's Render plugin is installed.
type PluginKind string

const (
	// PluginInApp means the plugin is installed from inside the tool (not from a
	// shell), so the wizard can only surface it as a recommended next step.
	PluginInApp PluginKind = "in-app"
	// PluginShell means the plugin has a shell installer the wizard could run.
	PluginShell PluginKind = "shell"
	// PluginNone means the tool has no dedicated Render plugin.
	PluginNone PluginKind = "none"
)

// Plugin describes a tool's recommended Render plugin path for next-step copy.
type Plugin struct {
	// Kind is how the plugin is installed (in-app, shell, or none).
	Kind PluginKind
	// Instruction is the human-facing step (e.g. "/add-plugin render") or shell
	// command; empty when Kind is PluginNone.
	Instruction string
	// RepoURL links the plugin's source, when one exists.
	RepoURL string
}

// PluginFor returns the recommended plugin path for a tool, for next-step copy.
func PluginFor(tool ids.ToolID) Plugin {
	switch tool {
	case ids.ToolCursor:
		return Plugin{Kind: PluginInApp, Instruction: "/add-plugin render", RepoURL: "https://github.com/render-oss/render-cursor-plugin"}
	case ids.ToolCodex:
		return Plugin{Kind: PluginInApp, Instruction: "open the plugin library and install \"Render\"", RepoURL: "https://github.com/render-oss/render-codex-plugin"}
	case ids.ToolOpenCode:
		return Plugin{Kind: PluginShell, Instruction: "curl -fsSL https://raw.githubusercontent.com/render-oss/render-opencode-plugin/main/install.sh | bash", RepoURL: "https://github.com/render-oss/render-opencode-plugin"}
	default:
		return Plugin{Kind: PluginNone}
	}
}
