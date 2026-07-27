package codex

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/render-lab/render-install-wizard/internal/ids"
	"github.com/render-lab/render-install-wizard/internal/render"
	"github.com/render-lab/render-install-wizard/internal/tools"
)

// TestMain makes these tests hermetic: it clears the ambient config-home
// overrides (e.g. a CI runner's CODEX_HOME) so path resolution stays under the
// injected temp home. Tests that exercise an override set it via t.Setenv.
func TestMain(m *testing.M) {
	for _, k := range []string{"XDG_CONFIG_HOME", "OPENCODE_CONFIG", "CLAUDE_CONFIG_DIR", "CODEX_HOME"} {
		_ = os.Unsetenv(k)
	}
	os.Exit(m.Run())
}

// readConfig unmarshals ~/.codex/config.toml under home into a generic map.
func readConfig(t *testing.T, home string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var m map[string]any
	if err := toml.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	return m
}

// renderServer returns the mcp_servers.render table, failing if it is absent.
func renderServer(t *testing.T, cfg map[string]any) map[string]any {
	t.Helper()
	servers, ok := cfg["mcp_servers"].(map[string]any)
	if !ok {
		t.Fatalf("mcp_servers missing or wrong type: %#v", cfg["mcp_servers"])
	}
	server, ok := servers[render.MCPServerName].(map[string]any)
	if !ok {
		t.Fatalf("render server missing or wrong type: %#v", servers[render.MCPServerName])
	}
	return server
}

func TestConfigureOAuth(t *testing.T) {
	home := t.TempDir()
	tool := &Tool{home: home, auth: render.AuthModeOAuth}

	if err := tool.Configure(context.Background(), tools.Selection{Components: []ids.ComponentID{ids.ComponentMCP}}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	server := renderServer(t, readConfig(t, home))
	if got := server["url"]; got != render.MCPServerURL {
		t.Errorf("url = %v, want %v", got, render.MCPServerURL)
	}
	if _, ok := server["http_headers"]; ok {
		t.Errorf("OAuth mode should not write http_headers, got %#v", server["http_headers"])
	}
}

func TestConfigureAPIKey(t *testing.T) {
	home := t.TempDir()
	tool := &Tool{home: home, auth: render.AuthModeAPIKey}

	if err := tool.Configure(context.Background(), tools.Selection{Components: []ids.ComponentID{ids.ComponentMCP}}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	server := renderServer(t, readConfig(t, home))
	headers, ok := server["http_headers"].(map[string]any)
	if !ok {
		t.Fatalf("http_headers missing or wrong type: %#v", server["http_headers"])
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
	path := filepath.Join(home, ".codex", "config.toml")
	existing := map[string]any{
		"model": "keep-me",
		"mcp_servers": map[string]any{
			"other": map[string]any{"url": "https://example.com/mcp"},
		},
	}
	data, err := toml.Marshal(existing)
	if err != nil {
		t.Fatalf("marshal existing: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	tool := &Tool{home: home, auth: render.AuthModeOAuth}
	if err := tool.Configure(context.Background(), tools.Selection{Components: []ids.ComponentID{ids.ComponentMCP}}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	cfg := readConfig(t, home)
	if got := cfg["model"]; got != "keep-me" {
		t.Errorf("unrelated top-level key lost: %#v", got)
	}
	servers, _ := cfg["mcp_servers"].(map[string]any)
	if _, ok := servers["other"].(map[string]any); !ok {
		t.Errorf("unrelated MCP server lost: %#v", servers["other"])
	}
	if _, ok := servers[render.MCPServerName]; !ok {
		t.Errorf("render server not added: %#v", servers)
	}
}

// TestConfigureReplacesStaleRenderTable guards F08 for TOML: an existing render
// table is replaced wholesale so a stale http_headers Authorization is not
// carried into a new OAuth config, while sibling tables survive.
func TestConfigureReplacesStaleRenderTable(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".codex", "config.toml")
	existing := map[string]any{
		"mcp_servers": map[string]any{
			"other": map[string]any{"url": "https://example.com/mcp"},
			render.MCPServerName: map[string]any{
				"url":          "https://old.example.com/mcp",
				"http_headers": map[string]any{"Authorization": "Bearer SECRET"},
			},
		},
	}
	data, err := toml.Marshal(existing)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	tool := &Tool{home: home, auth: render.AuthModeOAuth}
	if err := tool.Configure(context.Background(), tools.Selection{Components: []ids.ComponentID{ids.ComponentMCP}}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	server := renderServer(t, readConfig(t, home))
	if _, present := server["http_headers"]; present {
		t.Errorf("F08: stale http_headers carried forward: %#v", server)
	}
	if got := server["url"]; got != render.MCPServerURL {
		t.Errorf("render url = %v, want %v", got, render.MCPServerURL)
	}
	if servers, _ := readConfig(t, home)["mcp_servers"].(map[string]any); servers["other"] == nil {
		t.Error("sibling 'other' table was clobbered")
	}
}

func TestConfigureWithoutMCP(t *testing.T) {
	home := t.TempDir()
	tool := &Tool{home: home, auth: render.AuthModeOAuth}

	if err := tool.Configure(context.Background(), tools.Selection{Components: []ids.ComponentID{ids.ComponentSkills}}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	if _, err := os.Stat(filepath.Join(home, ".codex", "config.toml")); !os.IsNotExist(err) {
		t.Errorf("config should not be created without MCP selected, stat err = %v", err)
	}
}

// TestConfigureHonorsCodexHome guards F11: with CODEX_HOME set, config is written
// to $CODEX_HOME/config.toml, not the default ~/.codex/config.toml.
func TestConfigureHonorsCodexHome(t *testing.T) {
	root := t.TempDir()
	custom := filepath.Join(root, "custom-codex")
	if err := os.MkdirAll(custom, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", custom)

	home := filepath.Join(root, "home")
	tool := &Tool{home: home, auth: render.AuthModeOAuth}
	if err := tool.Configure(context.Background(), tools.Selection{Components: []ids.ComponentID{ids.ComponentMCP}}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	if _, err := os.Stat(filepath.Join(custom, "config.toml")); err != nil {
		t.Errorf("config not written to CODEX_HOME: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "config.toml")); !os.IsNotExist(err) {
		t.Errorf("config written to default location despite CODEX_HOME override (stat err=%v)", err)
	}
}

func TestUnconfigure(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".codex", "config.toml")
	existing := map[string]any{
		"model": "keep-me",
		"mcp_servers": map[string]any{
			"other":              map[string]any{"url": "https://example.com/mcp"},
			render.MCPServerName: map[string]any{"url": render.MCPServerURL},
		},
	}
	data, err := toml.Marshal(existing)
	if err != nil {
		t.Fatalf("marshal existing: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	tool := &Tool{home: home, auth: render.AuthModeOAuth}
	if err := tool.Unconfigure(context.Background()); err != nil {
		t.Fatalf("Unconfigure: %v", err)
	}

	cfg := readConfig(t, home)
	servers, _ := cfg["mcp_servers"].(map[string]any)
	if _, ok := servers[render.MCPServerName]; ok {
		t.Errorf("render server not removed: %#v", servers)
	}
	if _, ok := servers["other"].(map[string]any); !ok {
		t.Errorf("sibling MCP server removed: %#v", servers["other"])
	}
	if got := cfg["model"]; got != "keep-me" {
		t.Errorf("unrelated top-level key lost: %#v", got)
	}
}

func TestUnconfigureMissingFile(t *testing.T) {
	home := t.TempDir()
	tool := &Tool{home: home, auth: render.AuthModeOAuth}

	if err := tool.Unconfigure(context.Background()); err != nil {
		t.Fatalf("Unconfigure on missing file should be a no-op, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "config.toml")); !os.IsNotExist(err) {
		t.Errorf("Unconfigure should not create a file, stat err = %v", err)
	}
}

func TestNewDefaults(t *testing.T) {
	tool := New()
	if tool.auth != render.DefaultAuthMode {
		t.Errorf("auth = %v, want %v", tool.auth, render.DefaultAuthMode)
	}
	if tool.ID() != ids.ToolCodex {
		t.Errorf("ID = %v, want %v", tool.ID(), ids.ToolCodex)
	}
	if tool.PreferredDelivery() != ids.DeliveryRaw {
		t.Errorf("PreferredDelivery = %v, want %v", tool.PreferredDelivery(), ids.DeliveryRaw)
	}
}
