package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/render-oss/render-install-wizard/internal/components"
	"github.com/render-oss/render-install-wizard/internal/ids"
	"github.com/render-oss/render-install-wizard/internal/render"
)

func TestDetectFindsMCPEntry(t *testing.T) {
	home := t.TempDir()
	path, ok := render.MCPConfigPath(ids.ToolCursor, home)
	if !ok {
		t.Fatal("expected a config path for cursor")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	contents := `{"mcpServers":{"render":{"type":"http","url":"` + render.MCPServerURL + `"}}}`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &Component{home: home}
	got, err := c.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !got {
		t.Fatal("expected Detect true when a tool config contains the MCP URL")
	}
}

func TestDetectEmptyHome(t *testing.T) {
	c := &Component{home: t.TempDir()}
	got, err := c.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if got {
		t.Fatal("expected Detect false in empty home")
	}
}

func TestInstallUninstallAreNoOps(t *testing.T) {
	home := t.TempDir()
	c := &Component{home: home}

	if err := c.Install(context.Background(), components.Options{}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := c.Uninstall(context.Background()); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	// Neither should have written anything into home.
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no files written, found %d entries", len(entries))
	}
}

func TestStatusReflectsDetect(t *testing.T) {
	home := t.TempDir()
	c := &Component{home: home}

	st, err := c.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.State != components.StateNotInstalled {
		t.Fatalf("state = %q, want %q", st.State, components.StateNotInstalled)
	}

	path, _ := render.MCPConfigPath(ids.ToolCodex, home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	toml := "[mcp_servers.render]\nurl = \"" + render.MCPServerURL + "\"\n"
	if err := os.WriteFile(path, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err = c.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.State != components.StateInstalled {
		t.Fatalf("state = %q, want %q", st.State, components.StateInstalled)
	}
}
