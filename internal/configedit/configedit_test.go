package configedit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

// fixturesDir is the path to the shared, real-world config fixtures. Tests copy
// these into a t.TempDir() before mutating them so the originals stay pristine.
const fixturesDir = "../../testdata/fixtures"

// copyFixture copies the named fixture into a fresh temp directory and returns
// the path to the working copy.
func copyFixture(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join(fixturesDir, name)
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read fixture %s: %v", src, err)
	}
	dst := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("write working copy %s: %v", dst, err)
	}
	return dst
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse JSON %s: %v", path, err)
	}
	return m
}

func readTOML(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]any
	if err := toml.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse TOML %s: %v", path, err)
	}
	return m
}

// assertNoTempFiles fails if any leftover atomic-write temp files remain in the
// directory containing path.
func assertNoTempFiles(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".configedit-") {
			t.Fatalf("leftover temp file found: %s", e.Name())
		}
	}
}

func TestDeleteJSONPath_AbsentIsNoOp(t *testing.T) {
	path := copyFixture(t, "cursor_mcp.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := DeleteJSONPath(path, "mcpServers", "does-not-exist"); err != nil {
		t.Fatalf("DeleteJSONPath: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("delete of absent key modified the file")
	}
}

func TestDeleteJSONPath_MissingFileIsNoOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.json")
	if err := DeleteJSONPath(path, "a", "b"); err != nil {
		t.Fatalf("DeleteJSONPath on missing file: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("delete should not create the file")
	}
}

func TestDeleteJSONPath_PreservesSiblings(t *testing.T) {
	path := copyFixture(t, "cursor_mcp.json")
	if err := DeleteJSONPath(path, "mcpServers", "acme-tools"); err != nil {
		t.Fatalf("DeleteJSONPath: %v", err)
	}
	got := readJSON(t, path)
	servers := got["mcpServers"].(map[string]any)
	if _, ok := servers["acme-tools"]; ok {
		t.Fatalf("target key not deleted")
	}
	if got["workbench.colorTheme"] != "Default Dark Modern" {
		t.Fatalf("sibling top-level key lost: %#v", got)
	}
}

func TestDeleteTOMLPath_AbsentIsNoOp(t *testing.T) {
	path := copyFixture(t, "codex_config.toml")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := DeleteTOMLPath(path, "mcp_servers", "ghost"); err != nil {
		t.Fatalf("DeleteTOMLPath: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("delete of absent TOML key modified the file")
	}
}

// The tests below replace an equivalent set that exercised MergeJSONFile /
// MergeTOMLFile. Those functions were removed as dead code (no caller outside
// their own tests), but several of the properties they covered belong to
// machinery that is still live -- atomicWrite's permission handling, and the
// map round-trip that SetTOMLValue and DeleteTOMLPath share -- so the coverage is
// carried over onto the surviving API rather than dropped with them.

// TestSetJSONValuePreservesFileMode guards F07 on an existing file: updating a
// config must not broaden its permissions. Also covered on unix by
// TestAtomicWritePermissions, which additionally forces a permissive umask; this
// one runs everywhere.
func TestSetJSONValuePreservesFileMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mode.json")
	if err := os.WriteFile(path, []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := SetJSONValue(path, 2, "b"); err != nil {
		t.Fatalf("SetJSONValue: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode not preserved: got %o want 600", info.Mode().Perm())
	}
}

// TestSetJSONValueNewFileMode0600 guards F07 on creation: new wizard-owned config
// files must be private, since they can carry MCP Authorization headers or
// account/session state.
func TestSetJSONValueNewFileMode0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fresh.json")
	if err := SetJSONValue(path, 1, "a"); err != nil {
		t.Fatalf("SetJSONValue: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("new file mode wrong: got %o want 600", info.Mode().Perm())
	}
}

// TestSetJSONValueEmptyFileTreatedAsObject covers a whitespace-only config, which
// is neither absent (so the missing-file path does not apply) nor parseable as an
// object. It has to be treated as {} rather than erroring.
func TestSetJSONValueEmptyFileTreatedAsObject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(path, []byte("   \n"), 0o644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}
	if err := SetJSONValue(path, 1, "a"); err != nil {
		t.Fatalf("SetJSONValue: %v", err)
	}
	if got := readJSON(t, path); got["a"] != float64(1) {
		t.Fatalf("expected a=1, got %#v", got)
	}
	assertNoTempFiles(t, path)
}

// TestSetJSONValueLeavesFileValidAndClean checks the atomic-write contract on a
// real-world fixture: the result parses, and no temp file is left behind.
func TestSetJSONValueLeavesFileValidAndClean(t *testing.T) {
	path := copyFixture(t, "cursor_mcp.json")
	entry := map[string]any{"url": "https://mcp.render.com/mcp"}
	if err := SetJSONValue(path, entry, "mcpServers", "render"); err != nil {
		t.Fatalf("SetJSONValue: %v", err)
	}
	_ = readJSON(t, path)
	assertNoTempFiles(t, path)
}

// TestSetJSONValueNestedPathPreservesSiblings uses Claude's real settings shape to
// check that writing a deep key leaves its siblings alone.
//
// This is the analogue of the old nested-merge test, not a copy of it: SetJSONValue
// replaces the value at its target key wholesale rather than deep-merging, so the
// way to add one field without disturbing another is to address the deeper path.
// Passing an object at "telemetry" would -- correctly -- replace the whole table.
func TestSetJSONValueNestedPathPreservesSiblings(t *testing.T) {
	path := copyFixture(t, "claude_settings.json")
	original := readJSON(t, path)

	if err := SetJSONValue(path, "v1", "telemetry", "renderInstaller"); err != nil {
		t.Fatalf("SetJSONValue: %v", err)
	}
	if err := SetJSONValue(path, true, "enableRenderSkills"); err != nil {
		t.Fatalf("SetJSONValue: %v", err)
	}
	got := readJSON(t, path)

	tel, ok := got["telemetry"].(map[string]any)
	if !ok {
		t.Fatalf("telemetry missing or wrong type: %T", got["telemetry"])
	}
	if tel["renderInstaller"] != "v1" {
		t.Fatalf("telemetry.renderInstaller not added: %#v", tel)
	}
	if tel["enabled"] != false {
		t.Fatalf("sibling telemetry.enabled lost: %#v", tel)
	}
	if got["enableRenderSkills"] != true {
		t.Fatalf("new top-level key not added: %#v", got)
	}
	if !reflect.DeepEqual(got["permissions"], original["permissions"]) {
		t.Fatalf("permissions changed:\n got: %#v\nwant: %#v", got["permissions"], original["permissions"])
	}
}

// TestSetTOMLValuePreservesSiblingsAndRoundTrip covers the TOML map round-trip that
// SetTOMLValue and DeleteTOMLPath share: unrelated keys, tables, and sibling
// servers survive the write, and removing what was added returns the document to
// its original content.
//
// Equality is semantic rather than byte-for-byte because the TOML path
// unmarshals and re-marshals, which does not preserve comments or original
// ordering (see the package documentation).
func TestSetTOMLValuePreservesSiblingsAndRoundTrip(t *testing.T) {
	path := copyFixture(t, "codex_config.toml")
	original := readTOML(t, path)

	server := map[string]any{"command": "npx", "args": []any{"-y", "@render/mcp"}}
	if err := SetTOMLValue(path, server, "mcp_servers", "render"); err != nil {
		t.Fatalf("SetTOMLValue: %v", err)
	}

	got := readTOML(t, path)
	if got["model"] != "gpt-5" || got["approval_policy"] != "on-request" {
		t.Fatalf("top-level keys clobbered: %#v", got)
	}
	ui, ok := got["ui"].(map[string]any)
	if !ok || ui["theme"] != "dark" {
		t.Fatalf("[ui] section clobbered: %#v", got["ui"])
	}
	servers, ok := got["mcp_servers"].(map[string]any)
	if !ok {
		t.Fatalf("mcp_servers missing: %#v", got)
	}
	if _, ok := servers["acme-tools"]; !ok {
		t.Fatalf("pre-existing mcp server lost: %#v", servers)
	}
	if _, ok := servers["render"]; !ok {
		t.Fatalf("render server not added: %#v", servers)
	}

	if err := DeleteTOMLPath(path, "mcp_servers", "render"); err != nil {
		t.Fatalf("DeleteTOMLPath: %v", err)
	}
	if back := readTOML(t, path); !reflect.DeepEqual(back, original) {
		t.Fatalf("TOML round-trip not semantically equal:\n got: %#v\nwant: %#v", back, original)
	}
	assertNoTempFiles(t, path)
}

// TestSetTOMLValueIdempotent checks a repeated write is byte-identical, which is
// what keeps a wizard re-run from churning the file's mtime and waking
// file-watchers in running agents.
func TestSetTOMLValueIdempotent(t *testing.T) {
	path := copyFixture(t, "codex_config.toml")
	server := map[string]any{"command": "npx"}

	if err := SetTOMLValue(path, server, "mcp_servers", "render"); err != nil {
		t.Fatalf("first write: %v", err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := SetTOMLValue(path, server, "mcp_servers", "render"); err != nil {
		t.Fatalf("second write: %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("TOML write not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// TestSetTOMLValueMissingFileCreatesIt covers a first run on a machine where the
// tool has no config yet: the file and its parent directory are created.
func TestSetTOMLValueMissingFileCreatesIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.toml")
	if err := SetTOMLValue(path, map[string]any{"command": "npx"}, "mcp_servers", "render"); err != nil {
		t.Fatalf("SetTOMLValue: %v", err)
	}
	got := readTOML(t, path)
	servers, ok := got["mcp_servers"].(map[string]any)
	if !ok {
		t.Fatalf("mcp_servers missing: %#v", got)
	}
	if _, ok := servers["render"]; !ok {
		t.Fatalf("render not created: %#v", servers)
	}
	assertNoTempFiles(t, path)
}
