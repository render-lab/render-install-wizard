// Package render is the single source of truth for Render-specific installation
// facts: the MCP server endpoint and auth mode, per-tool config file locations,
// the skills source, the universal skills directory, and the recommended plugin
// next-steps per tool. Installer and tool packages read these values so none of
// them hardcode Render URLs, paths, or config shapes independently.
package render

import (
	"path/filepath"

	"github.com/render-oss/render-install-wizard/internal/ids"
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
const SkillsRepo = "render-oss/skills"

// Render CLI install facts.
const (
	// CLIBinaryName is the Render CLI executable name.
	CLIBinaryName = "render"
	// CLIRepo is the Render CLI source repository.
	CLIRepo = "render-oss/cli"
	// CLIInstallScriptURL is the official Linux/macOS CLI install script,
	// suitable for `curl -fsSL <url> | sh`.
	CLIInstallScriptURL = "https://raw.githubusercontent.com/render-oss/cli/refs/heads/main/bin/install.sh"
)

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

// MCPConfigPath returns the MCP/config file path for the given tool under home,
// and false for tools without a known config-file location.
func MCPConfigPath(tool ids.ToolID, home string) (string, bool) {
	switch tool {
	case ids.ToolClaudeCode:
		return filepath.Join(home, ".claude.json"), true
	case ids.ToolCursor:
		return filepath.Join(home, ".cursor", "mcp.json"), true
	case ids.ToolCodex:
		return filepath.Join(home, ".codex", "config.toml"), true
	case ids.ToolOpenCode:
		return filepath.Join(home, ".config", "opencode", "opencode.json"), true
	default:
		return "", false
	}
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
