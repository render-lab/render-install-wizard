package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/render-oss/render-install-wizard/internal/ids"
	"github.com/render-oss/render-install-wizard/internal/render"
	"github.com/render-oss/render-install-wizard/internal/tools"
)

// readConfig reads and parses the OpenCode config under home, failing the test
// on any error.
func readConfig(t *testing.T, home string) map[string]any {
	t.Helper()
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse config %s: %v", path, err)
	}
	return m
}

// renderEntry extracts mcp.render as an object from a parsed config.
func renderEntry(t *testing.T, cfg map[string]any) map[string]any {
	t.Helper()
	servers, ok := cfg["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("mcp missing or not an object: %#v", cfg["mcp"])
	}
	entry, ok := servers[render.MCPServerName].(map[string]any)
	if !ok {
		t.Fatalf("render entry missing or not an object: %#v", servers[render.MCPServerName])
	}
	return entry
}

func TestConfigureOAuthWritesURLNoHeader(t *testing.T) {
	home := t.TempDir()
	tool := &Tool{home: home, auth: render.AuthModeOAuth}

	sel := tools.Selection{Components: []ids.ComponentID{ids.ComponentMCP}}
	if err := tool.Configure(context.Background(), sel); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	cfg := readConfig(t, home)
	if got := cfg["$schema"]; got != "https://opencode.ai/config.json" {
		t.Errorf("$schema = %v, want https://opencode.ai/config.json", got)
	}
	entry := renderEntry(t, cfg)
	if got := entry["url"]; got != render.MCPServerURL {
		t.Errorf("url = %v, want %v", got, render.MCPServerURL)
	}
	if got := entry["type"]; got != "remote" {
		t.Errorf("type = %v, want remote", got)
	}
	if got := entry["enabled"]; got != true {
		t.Errorf("enabled = %v, want true", got)
	}
	if _, present := entry["headers"]; present {
		t.Errorf("OAuth mode should not write headers, got %#v", entry["headers"])
	}
}

func TestConfigureAPIKeyWritesEnvRefHeader(t *testing.T) {
	home := t.TempDir()
	tool := &Tool{home: home, auth: render.AuthModeAPIKey}

	sel := tools.Selection{Components: []ids.ComponentID{ids.ComponentMCP}}
	if err := tool.Configure(context.Background(), sel); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	entry := renderEntry(t, readConfig(t, home))
	headers, ok := entry["headers"].(map[string]any)
	if !ok {
		t.Fatalf("headers missing or not an object: %#v", entry["headers"])
	}
	want := fmt.Sprintf("Bearer {env:%s}", render.APIKeyEnvVar)
	if got := headers["Authorization"]; got != want {
		t.Errorf("Authorization = %v, want %v", got, want)
	}
}

func TestConfigureMergeNotClobber(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	existing := `{
  "theme": "keep-me",
  "mcp": {
    "other": {"type": "remote", "url": "https://example.com/mcp", "enabled": true}
  }
}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	tool := &Tool{home: home, auth: render.AuthModeOAuth}
	sel := tools.Selection{Components: []ids.ComponentID{ids.ComponentMCP}}
	if err := tool.Configure(context.Background(), sel); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	cfg := readConfig(t, home)
	if got := cfg["theme"]; got != "keep-me" {
		t.Errorf("theme = %v, want keep-me", got)
	}
	servers, ok := cfg["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("mcp missing: %#v", cfg["mcp"])
	}
	if _, ok := servers["other"].(map[string]any); !ok {
		t.Errorf("pre-existing 'other' server was clobbered: %#v", servers["other"])
	}
	if _, ok := servers[render.MCPServerName].(map[string]any); !ok {
		t.Errorf("render entry not added: %#v", servers)
	}
}

// TestConfigurePreservesExistingSchemaAndLargeInts guards F09: editing an
// existing config must not overwrite a user's $schema and must not corrupt
// large integers elsewhere in the file.
func TestConfigurePreservesExistingSchemaAndLargeInts(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{
  "$schema": "https://example.com/custom-schema.json",
  "bigId": 9007199254740993,
  "mcp": {}
}`
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}

	tool := &Tool{home: home, auth: render.AuthModeOAuth}
	if err := tool.Configure(context.Background(), tools.Selection{Components: []ids.ComponentID{ids.ComponentMCP}}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "https://example.com/custom-schema.json") {
		t.Error("F09: user's $schema was overwritten")
	}
	if strings.Contains(s, render.OpenCodeSchemaURL) {
		t.Error("F09: wizard replaced an existing $schema with its own")
	}
	if !strings.Contains(s, "9007199254740993") {
		t.Errorf("F09: large integer lost precision:\n%s", s)
	}
	if !strings.Contains(s, render.MCPServerURL) {
		t.Error("render entry not written")
	}
}

// TestConfigureEditsJsoncActiveFile guards F10: when opencode.jsonc exists it is
// the file OpenCode reads, so the wizard must edit it (preserving comments and
// trailing commas) rather than creating an inactive opencode.json.
func TestConfigureEditsJsoncActiveFile(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	jsoncPath := filepath.Join(dir, "opencode.jsonc")
	existing := `{
  // user's preferred theme
  "theme": "dark",
  "mcp": {
    "other": {"type": "remote", "url": "https://example.com/mcp", "enabled": true},
  }
}`
	if err := os.WriteFile(jsoncPath, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}

	tool := &Tool{home: home, auth: render.AuthModeOAuth}
	if err := tool.Configure(context.Background(), tools.Selection{Components: []ids.ComponentID{ids.ComponentMCP}}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	raw, err := os.ReadFile(jsoncPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "// user's preferred theme") {
		t.Error("F10: comment not preserved in edited .jsonc")
	}
	if !strings.Contains(s, render.MCPServerURL) {
		t.Error("render entry not written to the active .jsonc file")
	}
	if !strings.Contains(s, `"other"`) {
		t.Error("sibling server clobbered")
	}
	// No inactive duplicate must be created.
	if _, err := os.Stat(filepath.Join(dir, "opencode.json")); !os.IsNotExist(err) {
		t.Errorf("F10: inactive opencode.json was created (stat err=%v)", err)
	}
}

// TestConfigureHonorsOpenCodeConfig guards F11: with OPENCODE_CONFIG naming an
// explicit file, the wizard edits that file (the active, highest-precedence one)
// and does not create the default ~/.config/opencode/opencode.json.
func TestConfigureHonorsOpenCodeConfig(t *testing.T) {
	root := t.TempDir()
	explicit := filepath.Join(root, "custom", "oc.jsonc")
	if err := os.MkdirAll(filepath.Dir(explicit), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENCODE_CONFIG", explicit)

	home := filepath.Join(root, "home")
	tool := &Tool{home: home, auth: render.AuthModeOAuth}
	if err := tool.Configure(context.Background(), tools.Selection{Components: []ids.ComponentID{ids.ComponentMCP}}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	data, err := os.ReadFile(explicit)
	if err != nil {
		t.Fatalf("explicit OPENCODE_CONFIG file not written: %v", err)
	}
	if !strings.Contains(string(data), render.MCPServerURL) {
		t.Error("render MCP entry not written to the OPENCODE_CONFIG file")
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "opencode", "opencode.json")); !os.IsNotExist(err) {
		t.Errorf("default opencode.json created despite OPENCODE_CONFIG override (stat err=%v)", err)
	}
}

func TestConfigureWithoutMCPDoesNothing(t *testing.T) {
	home := t.TempDir()
	tool := &Tool{home: home, auth: render.AuthModeOAuth}

	sel := tools.Selection{Components: []ids.ComponentID{ids.ComponentCLI, ids.ComponentSkills}}
	if err := tool.Configure(context.Background(), sel); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected no config file created, stat err = %v", err)
	}
}

func TestUnconfigureRemovesOnlyRender(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	existing := `{
  "mcp": {
    "other": {"type": "remote", "url": "https://example.com/mcp", "enabled": true},
    "render": {"type": "remote", "url": "https://mcp.render.com/mcp", "enabled": true}
  }
}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	tool := &Tool{home: home, auth: render.AuthModeOAuth}
	if err := tool.Unconfigure(context.Background()); err != nil {
		t.Fatalf("Unconfigure: %v", err)
	}

	cfg := readConfig(t, home)
	servers, ok := cfg["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("mcp missing: %#v", cfg["mcp"])
	}
	if _, ok := servers["other"]; !ok {
		t.Errorf("sibling 'other' server was removed: %#v", servers)
	}
	if _, ok := servers[render.MCPServerName]; ok {
		t.Errorf("render entry not removed: %#v", servers)
	}
}

func TestUnconfigureMissingFileNoError(t *testing.T) {
	home := t.TempDir()
	tool := &Tool{home: home, auth: render.AuthModeOAuth}
	if err := tool.Unconfigure(context.Background()); err != nil {
		t.Errorf("Unconfigure on missing file should be a no-op, got %v", err)
	}
}

func TestNewDefaults(t *testing.T) {
	tool := New()
	if tool.auth != render.DefaultAuthMode {
		t.Errorf("auth = %v, want %v", tool.auth, render.DefaultAuthMode)
	}
	if tool.ID() != ids.ToolOpenCode {
		t.Errorf("ID = %v, want %v", tool.ID(), ids.ToolOpenCode)
	}
	if tool.PreferredDelivery() != ids.DeliveryRaw {
		t.Errorf("PreferredDelivery = %v, want %v", tool.PreferredDelivery(), ids.DeliveryRaw)
	}
}
