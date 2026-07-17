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
// Merge semantics: objects (maps) are merged recursively, while arrays and
// scalars in the patch REPLACE the existing value at that key. Missing or empty
// target files are treated as an empty object.
//
// # Accepted Phase-1 limitation: no formatting preservation
//
// This implementation models each file as a map[string]any: it unmarshals the
// existing file and the patch, deep-merges the maps, and re-marshals the result.
// As a consequence it does NOT preserve key ordering, comments, or exact byte
// formatting. JSON keys are re-emitted in sorted order and re-indented; TOML
// comments and original section ordering are lost. This is an accepted trade-off
// for Phase 1 in exchange for a small, obviously-correct implementation.
//
// Because of this, round-trip tests must assert *semantic* equality (unmarshal
// both sides and compare the resulting values) rather than byte equality: adding
// then removing Render returns the file to a semantically equal — but not
// necessarily byte-identical — state relative to the original.
//
// If faithful formatting/ordering/comment preservation later becomes a hard
// requirement, a future order-preserving editor (e.g. an AST/CST-based rewriter)
// may replace this map-based engine behind the same API.
package configedit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
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

// DeleteJSONPath removes the nested key identified by keys (e.g. "mcpServers",
// "render") from the JSON object at path and writes the result back atomically.
//
// It is a no-op — leaving the file untouched — when the file does not exist, is
// empty, or the key path is absent. Sibling keys along the path are preserved.
// At least one key must be supplied.
func DeleteJSONPath(path string, keys ...string) error {
	if len(keys) == 0 {
		return errors.New("configedit: DeleteJSONPath requires at least one key")
	}
	m, existed, err := loadJSONForDelete(path)
	if err != nil || !existed {
		return err
	}
	if !deleteKeyPath(m, keys) {
		return nil
	}
	out, err := marshalJSON(m)
	if err != nil {
		return fmt.Errorf("configedit: encode JSON for %s: %w", path, err)
	}
	return atomicWrite(path, out)
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
// created as needed. The written file's permissions match the pre-existing file
// at path, or 0644 when creating a new file. On any failure the temp file is
// removed so no partial output is left behind.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("configedit: create dir %s: %w", dir, err)
	}

	perm := os.FileMode(0o644)
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

// loadJSONForDelete reads the JSON object at path for a delete operation. The
// existed return is false (with a nil error) when the file is missing or empty,
// signaling that the caller should treat the delete as a no-op.
func loadJSONForDelete(path string) (m map[string]any, existed bool, err error) {
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
	m, err = parseJSONMap(data)
	if err != nil {
		return nil, false, fmt.Errorf("configedit: parse %s: %w", path, err)
	}
	return m, true, nil
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
// to an empty (non-nil) map so callers can always merge into it safely.
func parseJSONMap(data []byte) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
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
