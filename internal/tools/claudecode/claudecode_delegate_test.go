package claudecode

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/render-lab/render-install-wizard/internal/ids"
	"github.com/render-lab/render-install-wizard/internal/render"
	"github.com/render-lab/render-install-wizard/internal/tools"
)

// claudeFound is a lookPath stub reporting that the Claude CLI is installed.
func claudeFound(string) (string, error) { return "/usr/local/bin/claude", nil }

// mcpSelection is the selection that triggers MCP configuration.
func mcpSelection() tools.Selection {
	return tools.Selection{Components: []ids.ComponentID{ids.ComponentMCP}}
}

// TestConfigureDelegatesToClaudeCLI pins the delegated invocation. Letting Claude
// write its own state file is what keeps the wizard out of the lost-update race
// against a running Claude Code.
func TestConfigureDelegatesToClaudeCLI(t *testing.T) {
	home := t.TempDir()
	var got []string
	tool := &Tool{
		home:     home,
		auth:     render.AuthModeOAuth,
		lookPath: claudeFound,
		run: func(_ context.Context, name string, args ...string) error {
			got = append([]string{name}, args...)
			return nil
		},
	}

	if err := tool.Configure(context.Background(), mcpSelection()); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	if len(got) != 7 {
		t.Fatalf("unexpected command shape: %v", got)
	}
	if got[0] != "claude" || got[1] != "mcp" || got[2] != "add-json" || got[3] != render.MCPServerName {
		t.Errorf("command = %v, want `claude mcp add-json %s ...`", got, render.MCPServerName)
	}
	// --scope user is what puts the entry under the top-level mcpServers key
	// rather than burying it under the current project.
	if got[5] != "--scope" || got[6] != "user" {
		t.Errorf("expected --scope user, got %v", got[5:])
	}

	var entry map[string]any
	if err := json.Unmarshal([]byte(got[4]), &entry); err != nil {
		t.Fatalf("delegated JSON payload is invalid: %v (%q)", err, got[4])
	}
	if entry["url"] != render.MCPServerURL {
		t.Errorf("url = %v, want %v", entry["url"], render.MCPServerURL)
	}
	// Claude reads a url without a type as a stdio server and skips the entry.
	if entry["type"] != "http" {
		t.Errorf("type = %v, want http", entry["type"])
	}

	// Having delegated, the wizard must not also write the file itself.
	if _, err := os.Stat(filepath.Join(home, ".claude.json")); !os.IsNotExist(err) {
		t.Errorf("wizard wrote ~/.claude.json despite delegating (stat err = %v)", err)
	}
}

// TestConfigureFallsBackWhenCLIFails covers the rerun case too: `add-json` errors
// when the server already exists, and the file writer is idempotent.
func TestConfigureFallsBackWhenCLIFails(t *testing.T) {
	home := t.TempDir()
	tool := &Tool{
		home:     home,
		auth:     render.AuthModeOAuth,
		lookPath: claudeFound,
		run:      func(context.Context, string, ...string) error { return errors.New("already exists") },
	}

	if err := tool.Configure(context.Background(), mcpSelection()); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(home, ".claude.json"))
	if err != nil {
		t.Fatalf("fallback did not write the config: %v", err)
	}
	var cfg struct {
		MCPServers map[string]struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	entry, ok := cfg.MCPServers[render.MCPServerName]
	if !ok {
		t.Fatalf("render entry missing after fallback: %s", data)
	}
	if entry.URL != render.MCPServerURL || entry.Type != "http" {
		t.Errorf("fallback entry = %+v", entry)
	}
}

func TestConfigureUsesFileWriterWithoutCLI(t *testing.T) {
	home := t.TempDir()
	tool := &Tool{
		home:     home,
		auth:     render.AuthModeOAuth,
		lookPath: func(string) (string, error) { return "", errors.New("not found") },
		run: func(context.Context, string, ...string) error {
			t.Error("must not run a command when the CLI is absent")
			return nil
		},
	}

	if err := tool.Configure(context.Background(), mcpSelection()); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude.json")); err != nil {
		t.Errorf("expected the file fallback to write the config: %v", err)
	}
}

func TestUnconfigureDelegatesToClaudeCLI(t *testing.T) {
	var got []string
	tool := &Tool{
		home:     t.TempDir(),
		auth:     render.AuthModeOAuth,
		lookPath: claudeFound,
		run: func(_ context.Context, name string, args ...string) error {
			got = append([]string{name}, args...)
			return nil
		},
	}

	if err := tool.Unconfigure(context.Background()); err != nil {
		t.Fatalf("Unconfigure: %v", err)
	}

	want := []string{"claude", "mcp", "remove", render.MCPServerName, "--scope", "user"}
	if len(got) != len(want) {
		t.Fatalf("command = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("command = %v, want %v", got, want)
		}
	}
}

// TestUnconfigureFallsBackWhenCLIFails must still remove only the Render entry
// and leave a sibling server intact.
func TestUnconfigureFallsBackWhenCLIFails(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".claude.json")
	seed := `{"mcpServers":{"other":{"type":"http","url":"https://example.com/mcp"},"render":{"type":"http","url":"https://mcp.render.com/mcp"}},"keepMe":1}`
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}

	tool := &Tool{
		home:     home,
		auth:     render.AuthModeOAuth,
		lookPath: claudeFound,
		run:      func(context.Context, string, ...string) error { return errors.New("boom") },
	}
	if err := tool.Unconfigure(context.Background()); err != nil {
		t.Fatalf("Unconfigure: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	servers, _ := cfg["mcpServers"].(map[string]any)
	if _, present := servers[render.MCPServerName]; present {
		t.Errorf("render entry not removed by the fallback: %s", data)
	}
	if _, present := servers["other"]; !present {
		t.Errorf("sibling server was clobbered: %s", data)
	}
	if cfg["keepMe"] == nil {
		t.Errorf("unrelated key lost: %s", data)
	}
}

func TestNewWiresCLIDelegation(t *testing.T) {
	tool := New()
	if tool.lookPath == nil || tool.run == nil {
		t.Error("New must wire lookPath and run so the delegated path is reachable in production")
	}
}
