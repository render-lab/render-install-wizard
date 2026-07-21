// Package configedit provides a merge-not-clobber editing engine for JSON and
// TOML configuration files used by coding-agent tools (Cursor, Claude Code,
// Codex, and friends).
//
// The core guarantee is that merging Render's entries into a tool's config file
// never destroys unrelated data: other MCP servers, unrelated top-level keys,
// and sibling sections are all preserved. Writes are atomic (temp file in the
// same directory followed by os.Rename), so an interrupted write leaves either
// the old file or the new file intact — never a truncated one. Operations are
// idempotent: applying the same patch twice produces byte-identical output on
// the second write.
//
// Merge semantics (map-based helpers): objects (maps) are merged recursively,
// while arrays and scalars in the patch REPLACE the existing value at that key.
// Missing or empty target files are treated as an empty object.
//
// # JSON: surgical, format- and comment-preserving edits
//
// JSON is edited through SetJSONValue/DeleteJSONPath, which operate on the file's
// AST via tailscale/hujson (JWCC: JSON with comments and trailing commas). These
// preserve unrelated bytes exactly — sibling keys, comments, trailing commas,
// key order, formatting, and large-integer literals elsewhere are untouched —
// and SetJSONValue REPLACES the value at its target key wholesale (so stale or
// foreign fields under a wizard-owned entry are never retained). This makes them
// safe on hand-maintained configs, including JSONC (e.g. opencode.jsonc).
//
// # TOML: map-based, no comment/order preservation
//
// TOML is still edited via a map model (MergeTOMLFile/SetTOMLValue/DeleteTOMLPath):
// it unmarshals, mutates, and re-marshals, so TOML comments and original section
// ordering are not preserved. Round-trip tests for TOML therefore assert
// *semantic* equality rather than byte equality.
package configedit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/tailscale/hujson"
)

// MergeJSONFile deep-merges the JSON object encoded in patch into the JSON
// object stored at path and writes the result back atomically.
//
// A missing or empty file at path is treated as an empty object ({}). Parent
// directories are created as needed. Objects are merged recursively; arrays and
// scalars in patch replace the existing value at their key. Keys present in the
// file but absent from patch are preserved untouched.
func MergeJSONFile(path string, patch []byte) error {
	dst, err := readJSONMap(path)
	if err != nil {
		return err
	}
	src, err := parseJSONMap(patch)
	if err != nil {
		return fmt.Errorf("configedit: invalid JSON patch: %w", err)
	}
	deepMerge(dst, src)
	out, err := marshalJSON(dst)
	if err != nil {
		return fmt.Errorf("configedit: encode JSON for %s: %w", path, err)
	}
	return atomicWrite(path, out)
}

// MergeTOMLFile deep-merges the TOML document encoded in patch into the TOML
// document stored at path and writes the result back atomically.
//
// Semantics mirror MergeJSONFile: a missing or empty file is treated as an empty
// table, objects/tables merge recursively, arrays and scalars replace, and
// unrelated keys and sections are preserved. Note that TOML comments and the
// original section ordering are not preserved (see the package documentation).
func MergeTOMLFile(path string, patch []byte) error {
	dst, err := readTOMLMap(path)
	if err != nil {
		return err
	}
	src, err := parseTOMLMap(patch)
	if err != nil {
		return fmt.Errorf("configedit: invalid TOML patch: %w", err)
	}
	deepMerge(dst, src)
	out, err := marshalTOML(dst)
	if err != nil {
		return fmt.Errorf("configedit: encode TOML for %s: %w", path, err)
	}
	return atomicWrite(path, out)
}

// SetJSONValue sets (creating or wholesale-replacing) value at the nested key
// path keys in the JSON/JSONC file at path, writing the result atomically.
//
// The value at the final key is REPLACED in its entirety — not deep-merged — so
// stale or foreign fields under a wizard-owned entry are never carried forward.
// Every other byte is preserved: sibling keys, comments, trailing commas,
// ordering, formatting, and unrelated large-integer literals. Intermediate
// objects along the path are created as needed. A missing or empty file is
// treated as {}. At least one key must be supplied, and value must be
// JSON-marshalable.
func SetJSONValue(path string, value any, keys ...string) error {
	if len(keys) == 0 {
		return errors.New("configedit: SetJSONValue requires at least one key")
	}
	data, err := readJSONForEdit(path)
	if err != nil {
		return err
	}
	root, err := hujson.Parse(data)
	if err != nil {
		return fmt.Errorf("configedit: parse %s: %w", path, err)
	}
	ops, err := jsonSetOps(root, keys, value)
	if err != nil {
		return fmt.Errorf("configedit: set %s in %s: %w", strings.Join(keys, "."), path, err)
	}
	patch, err := json.Marshal(ops)
	if err != nil {
		return fmt.Errorf("configedit: build JSON patch: %w", err)
	}
	if err := root.Patch(patch); err != nil {
		return fmt.Errorf("configedit: patch %s: %w", path, err)
	}
	return atomicWrite(path, root.Pack())
}

// SetTOMLValue sets (creating or wholesale-replacing) value at the nested key
// path keys in the TOML file at path, writing the result atomically. Like
// SetJSONValue it replaces the leaf wholesale and preserves sibling tables and
// keys, but — as with all TOML editing here — comments and section ordering are
// not preserved.
func SetTOMLValue(path string, value any, keys ...string) error {
	if len(keys) == 0 {
		return errors.New("configedit: SetTOMLValue requires at least one key")
	}
	m, err := readTOMLMap(path)
	if err != nil {
		return err
	}
	if err := setKeyPath(m, keys, value); err != nil {
		return fmt.Errorf("configedit: set %s in %s: %w", strings.Join(keys, "."), path, err)
	}
	out, err := marshalTOML(m)
	if err != nil {
		return fmt.Errorf("configedit: encode TOML for %s: %w", path, err)
	}
	return atomicWrite(path, out)
}

// DeleteJSONPath removes the nested key identified by keys (e.g. "mcpServers",
// "render") from the JSON/JSONC file at path and writes the result back
// atomically, preserving all other bytes (siblings, comments, formatting).
//
// It is a no-op — leaving the file untouched — when the file does not exist, is
// empty, or the key path is absent. At least one key must be supplied.
func DeleteJSONPath(path string, keys ...string) error {
	if len(keys) == 0 {
		return errors.New("configedit: DeleteJSONPath requires at least one key")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("configedit: read %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	root, err := hujson.Parse(data)
	if err != nil {
		return fmt.Errorf("configedit: parse %s: %w", path, err)
	}
	if !jsonKeyPathExists(jsonStructure(root), keys) {
		return nil
	}
	patch, err := json.Marshal([]any{map[string]string{"op": "remove", "path": jsonPointer(keys)}})
	if err != nil {
		return fmt.Errorf("configedit: build JSON patch: %w", err)
	}
	if err := root.Patch(patch); err != nil {
		return fmt.Errorf("configedit: patch %s: %w", path, err)
	}
	return atomicWrite(path, root.Pack())
}

// DeleteTOMLPath removes the nested key identified by keys from the TOML document
// at path and writes the result back atomically. Its behavior mirrors
// DeleteJSONPath: it is a no-op when the file is missing/empty or the path is
// absent, and it preserves sibling keys and sections.
func DeleteTOMLPath(path string, keys ...string) error {
	if len(keys) == 0 {
		return errors.New("configedit: DeleteTOMLPath requires at least one key")
	}
	m, existed, err := loadTOMLForDelete(path)
	if err != nil || !existed {
		return err
	}
	if !deleteKeyPath(m, keys) {
		return nil
	}
	out, err := marshalTOML(m)
	if err != nil {
		return fmt.Errorf("configedit: encode TOML for %s: %w", path, err)
	}
	return atomicWrite(path, out)
}

// deepMerge recursively merges src into dst. When a key maps to an object
// (map[string]any) in both dst and src, the two objects are merged recursively.
// Otherwise the value from src replaces the value in dst — this includes arrays
// and scalars, which are treated as opaque replacements rather than merged.
func deepMerge(dst, src map[string]any) {
	for k, sv := range src {
		if sm, ok := sv.(map[string]any); ok {
			if dm, ok := dst[k].(map[string]any); ok {
				deepMerge(dm, sm)
				continue
			}
		}
		dst[k] = sv
	}
}

// deleteKeyPath removes the value at the nested key path keys from m, returning
// true if a deletion occurred. It descends only through map[string]any nodes and
// reports false (no change) if any intermediate key is missing or not an object,
// or if the final key is absent.
func deleteKeyPath(m map[string]any, keys []string) bool {
	if len(keys) == 1 {
		if _, ok := m[keys[0]]; ok {
			delete(m, keys[0])
			return true
		}
		return false
	}
	child, ok := m[keys[0]].(map[string]any)
	if !ok {
		return false
	}
	return deleteKeyPath(child, keys[1:])
}

// atomicWrite writes data to path atomically by creating a temp file in the same
// directory, flushing it, and renaming it into place. Parent directories are
// created as needed (mode 0700 for any the wizard creates). A new file is
// created private (mode 0600); an existing file's mode is preserved as-is and
// never broadened. On any failure the temp file is removed so no partial output
// is left behind.
//
// Privacy matters here: these config files can hold MCP Authorization headers or
// (for Claude) account/session state, so they must not be world- or
// group-readable on a shared host.
func atomicWrite(path string, data []byte) error {
	// Operational idempotency (F13): if the file already contains exactly these
	// bytes, do nothing — this avoids rewriting the file, so its mtime and inode
	// are stable and file-watchers in running agents aren't triggered on a no-op
	// rerun.
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return nil
	}

	dir := filepath.Dir(path)
	// MkdirAll only applies this mode to directories it creates; existing
	// directories keep their current mode.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("configedit: create dir %s: %w", dir, err)
	}

	perm := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("configedit: stat %s: %w", path, err)
	}

	tmp, err := os.CreateTemp(dir, ".configedit-*.tmp")
	if err != nil {
		return fmt.Errorf("configedit: create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup: harmless no-op once the rename below succeeds.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("configedit: write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("configedit: sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("configedit: close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("configedit: chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("configedit: rename temp file to %s: %w", path, err)
	}
	return nil
}

// readJSONMap reads and parses the JSON object at path, returning an empty map
// for a missing or empty file.
func readJSONMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("configedit: read %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	m, err := parseJSONMap(data)
	if err != nil {
		return nil, fmt.Errorf("configedit: parse %s: %w", path, err)
	}
	return m, nil
}

// readTOMLMap reads and parses the TOML document at path, returning an empty map
// for a missing or empty file.
func readTOMLMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("configedit: read %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	m, err := parseTOMLMap(data)
	if err != nil {
		return nil, fmt.Errorf("configedit: parse %s: %w", path, err)
	}
	return m, nil
}

// loadTOMLForDelete reads the TOML document at path for a delete operation,
// following the same no-op-on-missing/empty contract as loadJSONForDelete.
func loadTOMLForDelete(path string) (m map[string]any, existed bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("configedit: read %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, false, nil
	}
	m, err = parseTOMLMap(data)
	if err != nil {
		return nil, false, fmt.Errorf("configedit: parse %s: %w", path, err)
	}
	return m, true, nil
}

// parseJSONMap unmarshals a JSON object into a map, normalizing a null document
// to an empty (non-nil) map so callers can always merge into it safely. It uses
// a json.Decoder with UseNumber so integers larger than 2^53 survive a
// decode/encode round trip without being coerced to float64 (F09).
func parseJSONMap(data []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

// parseTOMLMap unmarshals a TOML document into a map, normalizing an empty
// document to an empty (non-nil) map.
func parseTOMLMap(data []byte) (map[string]any, error) {
	var m map[string]any
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

// marshalJSON encodes m as indented JSON with a trailing newline. encoding/json
// emits map keys in sorted order, giving deterministic (idempotent) output.
func marshalJSON(m map[string]any) ([]byte, error) {
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// marshalTOML encodes m as TOML. go-toml/v2 emits map keys in a deterministic
// (sorted) order, giving idempotent output.
func marshalTOML(m map[string]any) ([]byte, error) {
	return toml.Marshal(m)
}

// jsonPatchOp is a single RFC 6902 operation used to drive hujson.Patch. Value
// is always emitted (add ops require it, and empty intermediate objects must
// marshal as {}), so it must not be omitempty.
type jsonPatchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

// jsonSetOps builds the RFC 6902 "add" operations that set value at keys,
// creating any missing intermediate objects first. hujson.Patch requires each
// parent to exist, and an "add" onto an existing object member replaces it
// wholesale — which is exactly the replace-not-merge behavior we want.
func jsonSetOps(root hujson.Value, keys []string, value any) ([]jsonPatchOp, error) {
	cur := jsonStructure(root)
	var ops []jsonPatchOp
	for i := 0; i < len(keys)-1; i++ {
		k := keys[i]
		child, ok := cur[k]
		if !ok {
			ops = append(ops, jsonPatchOp{Op: "add", Path: jsonPointer(keys[:i+1]), Value: map[string]any{}})
			next := map[string]any{}
			cur[k] = next
			cur = next
			continue
		}
		next, isObj := child.(map[string]any)
		if !isObj {
			return nil, fmt.Errorf("%q is not a JSON object", k)
		}
		cur = next
	}
	ops = append(ops, jsonPatchOp{Op: "add", Path: jsonPointer(keys), Value: value})
	return ops, nil
}

// jsonStructure returns a standard-JSON map view of v for structural inspection
// (which intermediate objects already exist). It operates on a minimized clone;
// UseNumber avoids mangling numbers, though only object structure is consulted.
func jsonStructure(v hujson.Value) map[string]any {
	c := v.Clone()
	c.Minimize()
	dec := json.NewDecoder(bytes.NewReader(c.Pack()))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil || m == nil {
		return map[string]any{}
	}
	return m
}

// jsonKeyPathExists reports whether the full nested key path resolves to a value
// in the standard-JSON structure map.
func jsonKeyPathExists(m map[string]any, keys []string) bool {
	cur := m
	for i, k := range keys {
		val, ok := cur[k]
		if !ok {
			return false
		}
		if i == len(keys)-1 {
			return true
		}
		next, isObj := val.(map[string]any)
		if !isObj {
			return false
		}
		cur = next
	}
	return false
}

// jsonPointer renders keys as an RFC 6901 JSON Pointer, escaping ~ and /.
func jsonPointer(keys []string) string {
	esc := strings.NewReplacer("~", "~0", "/", "~1")
	var b strings.Builder
	for _, k := range keys {
		b.WriteByte('/')
		b.WriteString(esc.Replace(k))
	}
	return b.String()
}

// readJSONForEdit reads path for a surgical edit, returning "{}" for a missing or
// empty file so callers can create it.
func readJSONForEdit(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []byte("{}"), nil
		}
		return nil, fmt.Errorf("configedit: read %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return []byte("{}"), nil
	}
	return data, nil
}

// setKeyPath sets value at the nested key path in a map (TOML/JSON), creating
// intermediate maps as needed and replacing the leaf wholesale. It errors if an
// intermediate key exists but is not a map.
func setKeyPath(m map[string]any, keys []string, value any) error {
	cur := m
	for i := 0; i < len(keys)-1; i++ {
		k := keys[i]
		child, ok := cur[k]
		if !ok {
			next := map[string]any{}
			cur[k] = next
			cur = next
			continue
		}
		next, isMap := child.(map[string]any)
		if !isMap {
			return fmt.Errorf("%q is not a table", k)
		}
		cur = next
	}
	cur[keys[len(keys)-1]] = value
	return nil
}
