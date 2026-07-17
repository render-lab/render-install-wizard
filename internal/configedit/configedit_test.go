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

func TestMergeJSONFile_PreservesUnrelatedEntries(t *testing.T) {
	path := copyFixture(t, "cursor_mcp.json")
	original := readJSON(t, path)

	patch := []byte(`{"mcpServers":{"render":{"url":"https://mcp.render.com/mcp"}}}`)
	if err := MergeJSONFile(path, patch); err != nil {
		t.Fatalf("MergeJSONFile: %v", err)
	}

	got := readJSON(t, path)

	// The new render server was added.
	servers, ok := got["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers missing or wrong type: %T", got["mcpServers"])
	}
	render, ok := servers["render"].(map[string]any)
	if !ok {
		t.Fatalf("render server missing: %#v", servers)
	}
	if render["url"] != "https://mcp.render.com/mcp" {
		t.Fatalf("render url wrong: %#v", render)
	}

	// The pre-existing server is untouched.
	origServers := original["mcpServers"].(map[string]any)
	if !reflect.DeepEqual(servers["acme-tools"], origServers["acme-tools"]) {
		t.Fatalf("pre-existing server clobbered:\n got: %#v\nwant: %#v",
			servers["acme-tools"], origServers["acme-tools"])
	}

	// Unrelated top-level keys are preserved.
	for _, k := range []string{"editor.formatOnSave", "workbench.colorTheme"} {
		if !reflect.DeepEqual(got[k], original[k]) {
			t.Fatalf("unrelated key %q changed: got %#v want %#v", k, got[k], original[k])
		}
	}

	assertNoTempFiles(t, path)
}

func TestMergeJSONFile_RoundTripSemanticEqual(t *testing.T) {
	path := copyFixture(t, "cursor_mcp.json")
	original := readJSON(t, path)

	patch := []byte(`{"mcpServers":{"render":{"url":"https://mcp.render.com/mcp","type":"http"}}}`)
	if err := MergeJSONFile(path, patch); err != nil {
		t.Fatalf("MergeJSONFile: %v", err)
	}
	if err := DeleteJSONPath(path, "mcpServers", "render"); err != nil {
		t.Fatalf("DeleteJSONPath: %v", err)
	}

	got := readJSON(t, path)
	// We assert SEMANTIC equality (parsed values), not byte equality: the
	// map-based engine re-orders keys and re-indents, so the raw bytes differ
	// from the original even though the content is identical.
	if !reflect.DeepEqual(got, original) {
		t.Fatalf("round-trip not semantically equal:\n got: %#v\nwant: %#v", got, original)
	}
}

func TestMergeJSONFile_Idempotent(t *testing.T) {
	path := copyFixture(t, "cursor_mcp.json")
	patch := []byte(`{"mcpServers":{"render":{"url":"https://mcp.render.com/mcp"}}}`)

	if err := MergeJSONFile(path, patch); err != nil {
		t.Fatalf("first merge: %v", err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after first merge: %v", err)
	}
	if err := MergeJSONFile(path, patch); err != nil {
		t.Fatalf("second merge: %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after second merge: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("merge not idempotent (byte-level):\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestMergeJSONFile_MissingFileCreatesIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "settings.json")
	patch := []byte(`{"mcpServers":{"render":{"url":"https://mcp.render.com/mcp"}}}`)

	if err := MergeJSONFile(path, patch); err != nil {
		t.Fatalf("MergeJSONFile into missing path: %v", err)
	}

	got := readJSON(t, path)
	want := map[string]any{
		"mcpServers": map[string]any{
			"render": map[string]any{"url": "https://mcp.render.com/mcp"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("created file content wrong:\n got: %#v\nwant: %#v", got, want)
	}
	assertNoTempFiles(t, path)
}

func TestMergeJSONFile_EmptyFileTreatedAsObject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(path, []byte("   \n"), 0o644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}
	patch := []byte(`{"a":1}`)
	if err := MergeJSONFile(path, patch); err != nil {
		t.Fatalf("MergeJSONFile: %v", err)
	}
	got := readJSON(t, path)
	if got["a"] != float64(1) {
		t.Fatalf("expected a=1, got %#v", got)
	}
}

func TestMergeJSONFile_ArrayReplacesNotAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "arr.json")
	if err := os.WriteFile(path, []byte(`{"list":[1,2,3],"keep":true}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := MergeJSONFile(path, []byte(`{"list":[9]}`)); err != nil {
		t.Fatalf("MergeJSONFile: %v", err)
	}
	got := readJSON(t, path)
	list, ok := got["list"].([]any)
	if !ok || len(list) != 1 || list[0] != float64(9) {
		t.Fatalf("array should be replaced, got %#v", got["list"])
	}
	if got["keep"] != true {
		t.Fatalf("sibling key clobbered: %#v", got)
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

func TestMergeTOMLFile_PreservesSiblingsAndRoundTrip(t *testing.T) {
	path := copyFixture(t, "codex_config.toml")
	original := readTOML(t, path)

	patch := []byte(`[mcp_servers.render]
command = "npx"
args = ["-y", "@render/mcp"]
`)
	if err := MergeTOMLFile(path, patch); err != nil {
		t.Fatalf("MergeTOMLFile: %v", err)
	}

	got := readTOML(t, path)

	// Unrelated top-level keys and sections survive.
	if got["model"] != "gpt-5" || got["approval_policy"] != "on-request" {
		t.Fatalf("top-level keys clobbered: %#v", got)
	}
	ui, ok := got["ui"].(map[string]any)
	if !ok || ui["theme"] != "dark" {
		t.Fatalf("[ui] section clobbered: %#v", got["ui"])
	}

	servers := got["mcp_servers"].(map[string]any)
	if _, ok := servers["acme-tools"]; !ok {
		t.Fatalf("pre-existing mcp server lost: %#v", servers)
	}
	if _, ok := servers["render"]; !ok {
		t.Fatalf("render server not added: %#v", servers)
	}

	// Round-trip: delete render -> semantically equal to original.
	if err := DeleteTOMLPath(path, "mcp_servers", "render"); err != nil {
		t.Fatalf("DeleteTOMLPath: %v", err)
	}
	back := readTOML(t, path)
	if !reflect.DeepEqual(back, original) {
		t.Fatalf("TOML round-trip not semantically equal:\n got: %#v\nwant: %#v", back, original)
	}
	assertNoTempFiles(t, path)
}

func TestMergeTOMLFile_Idempotent(t *testing.T) {
	path := copyFixture(t, "codex_config.toml")
	patch := []byte(`[mcp_servers.render]
command = "npx"
`)
	if err := MergeTOMLFile(path, patch); err != nil {
		t.Fatalf("first merge: %v", err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := MergeTOMLFile(path, patch); err != nil {
		t.Fatalf("second merge: %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("TOML merge not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestMergeTOMLFile_MissingFileCreatesIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.toml")
	patch := []byte(`[mcp_servers.render]
command = "npx"
`)
	if err := MergeTOMLFile(path, patch); err != nil {
		t.Fatalf("MergeTOMLFile: %v", err)
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

func TestClaudeSettings_MergePreservesNestedPermissions(t *testing.T) {
	path := copyFixture(t, "claude_settings.json")
	original := readJSON(t, path)

	patch := []byte(`{"enableRenderSkills":true,"telemetry":{"renderInstaller":"v1"}}`)
	if err := MergeJSONFile(path, patch); err != nil {
		t.Fatalf("MergeJSONFile: %v", err)
	}
	got := readJSON(t, path)

	// Nested object merged, not replaced: existing telemetry.enabled kept.
	tel := got["telemetry"].(map[string]any)
	if tel["enabled"] != false {
		t.Fatalf("telemetry.enabled lost during nested merge: %#v", tel)
	}
	if tel["renderInstaller"] != "v1" {
		t.Fatalf("telemetry.renderInstaller not added: %#v", tel)
	}
	if got["enableRenderSkills"] != true {
		t.Fatalf("new top-level key not added: %#v", got)
	}
	// Untouched permissions block survives verbatim.
	if !reflect.DeepEqual(got["permissions"], original["permissions"]) {
		t.Fatalf("permissions changed:\n got: %#v\nwant: %#v", got["permissions"], original["permissions"])
	}
}

func TestMergeJSONFile_InvalidPatchErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.json")
	if err := MergeJSONFile(path, []byte(`{not json`)); err == nil {
		t.Fatalf("expected error for invalid JSON patch")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("invalid patch should not create a file")
	}
}

func TestMergeJSONFile_AtomicityValidAndParseable(t *testing.T) {
	path := copyFixture(t, "cursor_mcp.json")
	patch := []byte(`{"mcpServers":{"render":{"url":"https://mcp.render.com/mcp"}}}`)
	if err := MergeJSONFile(path, patch); err != nil {
		t.Fatalf("MergeJSONFile: %v", err)
	}
	// File must be valid, parseable JSON with no leftover temp files.
	_ = readJSON(t, path)
	assertNoTempFiles(t, path)
}

func TestMergeJSONFile_PreservesFileMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mode.json")
	if err := os.WriteFile(path, []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := MergeJSONFile(path, []byte(`{"b":2}`)); err != nil {
		t.Fatalf("MergeJSONFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode not preserved: got %o want 600", info.Mode().Perm())
	}
}

func TestMergeJSONFile_NewFileMode0644(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fresh.json")
	if err := MergeJSONFile(path, []byte(`{"a":1}`)); err != nil {
		t.Fatalf("MergeJSONFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("new file mode wrong: got %o want 644", info.Mode().Perm())
	}
}
