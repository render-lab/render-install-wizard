package cursor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/render-lab/render-install-wizard/internal/ids"
	"github.com/render-lab/render-install-wizard/internal/render"
	"github.com/render-lab/render-install-wizard/internal/tools"
)

// readConfig reads and parses the Cursor mcp.json under home, failing the test
// on any error.
func readConfig(t *testing.T, home string) map[string]any {
	t.Helper()
	path := filepath.Join(home, ".cursor", "mcp.json")
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

// renderEntry extracts mcpServers.render as an object from a parsed config.
func renderEntry(t *testing.T, cfg map[string]any) map[string]any {
	t.Helper()
	servers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers missing or not an object: %#v", cfg["mcpServers"])
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

	entry := renderEntry(t, readConfig(t, home))
	if got := entry["url"]; got != render.MCPServerURL {
		t.Errorf("url = %v, want %v", got, render.MCPServerURL)
	}
	if got := entry["type"]; got != "http" {
		t.Errorf("type = %v, want http", got)
	}
	if _, present := entry["headers"]; present {
		t.Errorf("OAuth mode should not write headers, got %#v", entry["headers"])
	}
}

func TestConfigureAPIKeyWritesHeader(t *testing.T) {
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
	want, _ := render.AuthorizationHeader(render.AuthModeAPIKey)
	if got := headers["Authorization"]; got != want {
		t.Errorf("Authorization = %v, want %v", got, want)
	}
}

func TestConfigureMergeNotClobber(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".cursor", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	existing := `{
  "topLevel": "keep-me",
  "mcpServers": {
    "other": {"type": "http", "url": "https://example.com/mcp"}
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
	if got := cfg["topLevel"]; got != "keep-me" {
		t.Errorf("topLevel = %v, want keep-me", got)
	}
	servers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers missing: %#v", cfg["mcpServers"])
	}
	if _, ok := servers["other"].(map[string]any); !ok {
		t.Errorf("pre-existing 'other' server was clobbered: %#v", servers["other"])
	}
	if _, ok := servers[render.MCPServerName].(map[string]any); !ok {
		t.Errorf("render entry not added: %#v", servers)
	}
}

// TestConfigureReplacesStaleRenderFields guards F08: an existing render entry
// (e.g. from a prior API-key config) is replaced wholesale, so stale headers and
// transport fields are not carried forward, while sibling servers are kept.
func TestConfigureReplacesStaleRenderFields(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".cursor", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{
  "mcpServers": {
    "other": {"type": "http", "url": "https://example.com/mcp"},
    "render": {
      "type": "stdio",
      "command": "old-binary",
      "args": ["--legacy"],
      "url": "https://old.example.com/mcp",
      "headers": {"Authorization": "Bearer SECRET"}
    }
  }
}`
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}

	tool := &Tool{home: home, auth: render.AuthModeOAuth}
	if err := tool.Configure(context.Background(), tools.Selection{Components: []ids.ComponentID{ids.ComponentMCP}}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	// Raw check: the old secret/transport fields must be gone entirely.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stale := range []string{"SECRET", "old-binary", "--legacy", "old.example.com"} {
		if strings.Contains(string(raw), stale) {
			t.Errorf("stale content %q carried forward:\n%s", stale, raw)
		}
	}

	entry := renderEntry(t, readConfig(t, home))
	if entry["type"] != "http" || entry["url"] != render.MCPServerURL {
		t.Errorf("render entry not replaced cleanly: %#v", entry)
	}
	for _, k := range []string{"headers", "command", "args"} {
		if _, present := entry[k]; present {
			t.Errorf("stale field %q retained in render entry: %#v", k, entry)
		}
	}
	cfg := readConfig(t, home)
	if servers, _ := cfg["mcpServers"].(map[string]any); servers["other"] == nil {
		t.Error("sibling 'other' server was clobbered")
	}
}

func TestConfigureWithoutMCPDoesNothing(t *testing.T) {
	home := t.TempDir()
	tool := &Tool{home: home, auth: render.AuthModeOAuth}

	sel := tools.Selection{Components: []ids.ComponentID{ids.ComponentCLI, ids.ComponentSkills}}
	if err := tool.Configure(context.Background(), sel); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	path := filepath.Join(home, ".cursor", "mcp.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected no config file created, stat err = %v", err)
	}
}

func TestUnconfigureRemovesOnlyRender(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".cursor", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	existing := `{
  "mcpServers": {
    "other": {"type": "http", "url": "https://example.com/mcp"},
    "render": {"type": "http", "url": "https://mcp.render.com/mcp"}
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
	servers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers missing: %#v", cfg["mcpServers"])
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
	if tool.ID() != ids.ToolCursor {
		t.Errorf("ID = %v, want %v", tool.ID(), ids.ToolCursor)
	}
	if tool.PreferredDelivery() != ids.DeliveryRaw {
		t.Errorf("PreferredDelivery = %v, want %v", tool.PreferredDelivery(), ids.DeliveryRaw)
	}
}
