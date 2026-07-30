package configedit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

// TestEnsureTOMLRootKeyPreservesEverythingElse is the reason this helper exists
// instead of SetTOMLValue: it must add the assignment without reserializing the
// document, so comments, key order, and quoting style all survive.
func TestEnsureTOMLRootKeyPreservesEverythingElse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	original := "# keep me\nmodel = \"o3\"\n\n# and me\n[mcp_servers.mine]\nurl = \"https://staging.internal/mcp\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := EnsureTOMLRootKey(path, "experimental_use_rmcp_client", "true"); err != nil {
		t.Fatalf("EnsureTOMLRootKey: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), original) {
		t.Errorf("original bytes not preserved verbatim.\n--- want to contain ---\n%s\n--- got ---\n%s", original, after)
	}
	var cfg map[string]any
	if err := toml.Unmarshal(after, &cfg); err != nil {
		t.Fatalf("result is not valid TOML: %v\n%s", err, after)
	}
	if cfg["experimental_use_rmcp_client"] != true {
		t.Errorf("flag not set: %#v", cfg["experimental_use_rmcp_client"])
	}
}

// TestEnsureTOMLRootKeyPrecedesTables pins the ordering requirement: the caller
// needs the assignment above every [table] header, because that is the only
// position the consuming tool honors.
func TestEnsureTOMLRootKeyPrecedesTables(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[mcp_servers.mine]\nurl = \"https://x/mcp\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureTOMLRootKey(path, "flag", "true"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Index(string(got), "flag") > strings.Index(string(got), "[mcp_servers") {
		t.Errorf("flag must come first:\n%s", got)
	}
}

func TestEnsureTOMLRootKeyIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("model = \"o3\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureTOMLRootKey(path, "flag", "true"); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureTOMLRootKey(path, "flag", "true"); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("second call changed the file:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	if n := strings.Count(string(second), "flag"); n != 1 {
		t.Errorf("flag written %d times, want 1:\n%s", n, second)
	}
}

// TestEnsureTOMLRootKeyRespectsExistingValue must not overwrite a user's own
// choice, even a deliberate opt-out.
func TestEnsureTOMLRootKeyRespectsExistingValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("flag = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureTOMLRootKey(path, "flag", "true"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "flag = false\n" {
		t.Errorf("overwrote an existing root assignment: %q", got)
	}
}

// TestEnsureTOMLRootKeyIgnoresSameNameInsideTable guards the scoping rule: a key
// of the same name nested in a table is a different setting and must not be
// mistaken for a root-level assignment.
func TestEnsureTOMLRootKeyIgnoresSameNameInsideTable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[some_table]\nflag = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureTOMLRootKey(path, "flag", "true"); err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid TOML: %v\n%s", err, data)
	}
	if cfg["flag"] != true {
		t.Errorf("root flag not added despite only a nested one existing: %#v", cfg)
	}
}

func TestEnsureTOMLRootKeyCreatesMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "config.toml")
	if err := EnsureTOMLRootKey(path, "flag", "true"); err != nil {
		t.Fatalf("EnsureTOMLRootKey on missing file: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "flag = true\n" {
		t.Errorf("unexpected new-file contents: %q", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("new config mode = %o, want 600", perm)
	}
}

func TestEnsureTOMLRootKeyRequiresKey(t *testing.T) {
	if err := EnsureTOMLRootKey(filepath.Join(t.TempDir(), "c.toml"), "", "true"); err == nil {
		t.Error("expected an error for an empty key")
	}
}
