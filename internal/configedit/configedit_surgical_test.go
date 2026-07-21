package configedit

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSetJSONValueReplacesLeafAndPreservesRest exercises F08 (wholesale replace
// of the target leaf), F09 (unrelated large integers preserved exactly), and
// F10 (comments preserved) in one surgical edit.
func TestSetJSONValueReplacesLeafAndPreservesRest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.json")
	orig := `{
  // keep this comment
  "bigId": 9007199254740993,
  "mcpServers": {
    "other": {"type": "http", "url": "https://example.com/mcp"},
    "render": {"type": "stdio", "command": "old", "args": ["--legacy"], "headers": {"Authorization": "Bearer SECRET"}}
  }
}`
	if err := os.WriteFile(path, []byte(orig), 0o600); err != nil {
		t.Fatal(err)
	}

	entry := map[string]any{"type": "http", "url": "https://mcp.render.com/mcp"}
	if err := SetJSONValue(path, entry, "mcpServers", "render"); err != nil {
		t.Fatalf("SetJSONValue: %v", err)
	}

	s := readString(t, path)
	if !strings.Contains(s, "// keep this comment") {
		t.Error("F10: comment was not preserved")
	}
	if !strings.Contains(s, "9007199254740993") {
		t.Error("F09: large integer lost precision or was rewritten")
	}
	if strings.Contains(s, "SECRET") || strings.Contains(s, `"command"`) || strings.Contains(s, `"args"`) {
		t.Errorf("F08: stale render fields carried forward:\n%s", s)
	}
	if !strings.Contains(s, "https://mcp.render.com/mcp") {
		t.Error("render entry url not written")
	}
	if !strings.Contains(s, `"other"`) {
		t.Error("sibling server was clobbered")
	}
}

// TestSetJSONValueCreatesFileAndParents verifies a missing file and missing
// intermediate objects are created.
func TestSetJSONValueCreatesFileAndParents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.json")
	if err := SetJSONValue(path, map[string]any{"url": "u"}, "mcpServers", "render"); err != nil {
		t.Fatalf("SetJSONValue: %v", err)
	}
	var m map[string]any
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	servers, _ := m["mcpServers"].(map[string]any)
	entry, _ := servers["render"].(map[string]any)
	if entry["url"] != "u" {
		t.Fatalf("mcpServers.render.url = %v, want u", entry["url"])
	}
}

// TestSetJSONValueIdempotent confirms re-applying the same set is byte-identical.
func TestSetJSONValueIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"other":{"url":"x"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	entry := map[string]any{"type": "http", "url": "https://mcp.render.com/mcp"}
	if err := SetJSONValue(path, entry, "mcpServers", "render"); err != nil {
		t.Fatal(err)
	}
	first := readBytes(t, path)
	if err := SetJSONValue(path, entry, "mcpServers", "render"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, readBytes(t, path)) {
		t.Error("second identical SetJSONValue changed the file (not idempotent)")
	}
}

// TestSetJSONValueNoOpPreservesInode guards F13: a second identical write does
// not rewrite the file, so its identity (inode) and mtime are stable and agent
// file-watchers aren't triggered on a no-op rerun.
func TestSetJSONValueNoOpPreservesInode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"other":{"url":"x"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	entry := map[string]any{"type": "http", "url": "https://mcp.render.com/mcp"}
	if err := SetJSONValue(path, entry, "mcpServers", "render"); err != nil {
		t.Fatal(err)
	}
	fi1, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := SetJSONValue(path, entry, "mcpServers", "render"); err != nil {
		t.Fatal(err)
	}
	fi2, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(fi1, fi2) {
		t.Error("no-op rewrite changed the file identity (inode)")
	}
	if !fi1.ModTime().Equal(fi2.ModTime()) {
		t.Errorf("no-op rewrite changed mtime: %v -> %v", fi1.ModTime(), fi2.ModTime())
	}
}

// TestDeleteJSONPathPreservesComments guards F10 on uninstall: removing the
// render entry from a JSONC file keeps comments and sibling entries.
func TestDeleteJSONPathPreservesComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.jsonc")
	orig := `{
  // top comment
  "mcpServers": {
    "other": {"url": "x"},
    "render": {"url": "y"}
  }
}`
	if err := os.WriteFile(path, []byte(orig), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := DeleteJSONPath(path, "mcpServers", "render"); err != nil {
		t.Fatalf("DeleteJSONPath: %v", err)
	}
	s := readString(t, path)
	if !strings.Contains(s, "// top comment") {
		t.Error("comment not preserved on delete")
	}
	if strings.Contains(s, `"render"`) {
		t.Error("render entry not removed")
	}
	if !strings.Contains(s, `"other"`) {
		t.Error("sibling removed on delete")
	}
}

// TestSetTOMLValueReplacesLeaf guards F08 for TOML: the render table is replaced
// wholesale (stale http_headers dropped) while siblings survive.
func TestSetTOMLValueReplacesLeaf(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	orig := "[mcp_servers.other]\nurl = \"https://example.com/mcp\"\n\n" +
		"[mcp_servers.render]\nurl = \"https://old.example.com/mcp\"\n\n" +
		"[mcp_servers.render.http_headers]\nAuthorization = \"Bearer SECRET\"\n"
	if err := os.WriteFile(path, []byte(orig), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SetTOMLValue(path, map[string]any{"url": "https://mcp.render.com/mcp"}, "mcp_servers", "render"); err != nil {
		t.Fatalf("SetTOMLValue: %v", err)
	}
	s := readString(t, path)
	if strings.Contains(s, "SECRET") || strings.Contains(s, "http_headers") {
		t.Errorf("F08: stale render table fields carried forward:\n%s", s)
	}
	if !strings.Contains(s, "https://mcp.render.com/mcp") {
		t.Error("render url not written")
	}
	if !strings.Contains(s, "other") {
		t.Error("sibling table clobbered")
	}
}

func readString(t *testing.T, path string) string {
	t.Helper()
	return string(readBytes(t, path))
}

func readBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
