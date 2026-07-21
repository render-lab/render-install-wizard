package claudecode

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/render-lab/render-install-wizard/internal/ids"
	"github.com/render-lab/render-install-wizard/internal/render"
	"github.com/render-lab/render-install-wizard/internal/tools"
)

// readConfig unmarshals ~/.claude.json under home into a generic map.
func readConfig(t *testing.T, home string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, ".claude.json"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	return m
}

// renderEntry returns the mcpServers.render entry, failing if it is absent.
func renderEntry(t *testing.T, cfg map[string]any) map[string]any {
	t.Helper()
	servers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers missing or wrong type: %#v", cfg["mcpServers"])
	}
	entry, ok := servers[render.MCPServerName].(map[string]any)
	if !ok {
		t.Fatalf("render entry missing or wrong type: %#v", servers[render.MCPServerName])
	}
	return entry
}

// TestConfigureHonorsClaudeConfigDir guards F11: with CLAUDE_CONFIG_DIR set, the
// MCP entry is written to $CLAUDE_CONFIG_DIR/.claude.json, not ~/.claude.json.
func TestConfigureHonorsClaudeConfigDir(t *testing.T) {
	root := t.TempDir()
	cfgDir := filepath.Join(root, "claude-cfg")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", cfgDir)

	home := filepath.Join(root, "home")
	tool := &Tool{home: home, auth: render.AuthModeOAuth}
	if err := tool.Configure(context.Background(), tools.Selection{Components: []ids.ComponentID{ids.ComponentMCP}}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	if _, err := os.Stat(filepath.Join(cfgDir, ".claude.json")); err != nil {
		t.Errorf("config not written to CLAUDE_CONFIG_DIR: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude.json")); !os.IsNotExist(err) {
		t.Errorf("config written to default ~/.claude.json despite override (stat err=%v)", err)
	}
}

func TestConfigureOAuth(t *testing.T) {
	home := t.TempDir()
	tool := &Tool{home: home, auth: render.AuthModeOAuth}

	if err := tool.Configure(context.Background(), tools.Selection{Components: []ids.ComponentID{ids.ComponentMCP}}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	entry := renderEntry(t, readConfig(t, home))
	if got := entry["url"]; got != render.MCPServerURL {
		t.Errorf("url = %v, want %v", got, render.MCPServerURL)
	}
	if got := entry["type"]; got != "http" {
		t.Errorf("type = %v, want http", got)
	}
	if _, ok := entry["headers"]; ok {
		t.Errorf("OAuth mode should not write headers, got %#v", entry["headers"])
	}
}

func TestConfigureAPIKey(t *testing.T) {
	home := t.TempDir()
	tool := &Tool{home: home, auth: render.AuthModeAPIKey}

	if err := tool.Configure(context.Background(), tools.Selection{Components: []ids.ComponentID{ids.ComponentMCP}}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	entry := renderEntry(t, readConfig(t, home))
	headers, ok := entry["headers"].(map[string]any)
	if !ok {
		t.Fatalf("headers missing or wrong type: %#v", entry["headers"])
	}
	wantValue, _ := render.AuthorizationHeader(render.AuthModeAPIKey)
	if got := headers["Authorization"]; got != wantValue {
		t.Errorf("Authorization = %v, want %v", got, wantValue)
	}
	if wantValue != "Bearer $"+render.APIKeyEnvVar {
		t.Errorf("expected shell env-ref form, got %q", wantValue)
	}
}

func TestConfigureMergeNotClobber(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".claude.json")
	existing := map[string]any{
		"mcpServers": map[string]any{
			"other": map[string]any{"type": "http", "url": "https://example.com/mcp"},
		},
		"unrelatedTopLevel": "keep-me",
	}
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		t.Fatalf("marshal existing: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	tool := &Tool{home: home, auth: render.AuthModeOAuth}
	if err := tool.Configure(context.Background(), tools.Selection{Components: []ids.ComponentID{ids.ComponentMCP}}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	cfg := readConfig(t, home)
	if got := cfg["unrelatedTopLevel"]; got != "keep-me" {
		t.Errorf("unrelated top-level key lost: %#v", got)
	}
	servers, _ := cfg["mcpServers"].(map[string]any)
	if _, ok := servers["other"].(map[string]any); !ok {
		t.Errorf("unrelated MCP server lost: %#v", servers["other"])
	}
	if _, ok := servers[render.MCPServerName]; !ok {
		t.Errorf("render entry not added: %#v", servers)
	}
}

func TestConfigureWithoutMCP(t *testing.T) {
	home := t.TempDir()
	tool := &Tool{home: home, auth: render.AuthModeOAuth}

	if err := tool.Configure(context.Background(), tools.Selection{Components: []ids.ComponentID{ids.ComponentSkills}}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	if _, err := os.Stat(filepath.Join(home, ".claude.json")); !os.IsNotExist(err) {
		t.Errorf("config should not be created without MCP selected, stat err = %v", err)
	}
}

func TestUnconfigure(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".claude.json")
	existing := map[string]any{
		"mcpServers": map[string]any{
			"other":              map[string]any{"type": "http", "url": "https://example.com/mcp"},
			render.MCPServerName: map[string]any{"type": "http", "url": render.MCPServerURL},
		},
		"unrelatedTopLevel": "keep-me",
	}
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		t.Fatalf("marshal existing: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	tool := &Tool{home: home, auth: render.AuthModeOAuth}
	if err := tool.Unconfigure(context.Background()); err != nil {
		t.Fatalf("Unconfigure: %v", err)
	}

	cfg := readConfig(t, home)
	servers, _ := cfg["mcpServers"].(map[string]any)
	if _, ok := servers[render.MCPServerName]; ok {
		t.Errorf("render entry not removed: %#v", servers)
	}
	if _, ok := servers["other"].(map[string]any); !ok {
		t.Errorf("sibling MCP server removed: %#v", servers["other"])
	}
	if got := cfg["unrelatedTopLevel"]; got != "keep-me" {
		t.Errorf("unrelated top-level key lost: %#v", got)
	}
}

func TestUnconfigureMissingFile(t *testing.T) {
	home := t.TempDir()
	tool := &Tool{home: home, auth: render.AuthModeOAuth}

	if err := tool.Unconfigure(context.Background()); err != nil {
		t.Fatalf("Unconfigure on missing file should be a no-op, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude.json")); !os.IsNotExist(err) {
		t.Errorf("Unconfigure should not create a file, stat err = %v", err)
	}
}

func TestNewDefaults(t *testing.T) {
	tool := New()
	if tool.auth != render.DefaultAuthMode {
		t.Errorf("auth = %v, want %v", tool.auth, render.DefaultAuthMode)
	}
	if tool.ID() != ids.ToolClaudeCode {
		t.Errorf("ID = %v, want %v", tool.ID(), ids.ToolClaudeCode)
	}
	if tool.PreferredDelivery() != ids.DeliveryRaw {
		t.Errorf("PreferredDelivery = %v, want %v", tool.PreferredDelivery(), ids.DeliveryRaw)
	}
}
