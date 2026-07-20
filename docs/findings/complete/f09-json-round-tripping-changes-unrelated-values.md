# F09: JSON round-tripping can change unrelated values

- Severity: Medium
- Category: Data integrity
- Status: Confirmed

## Problem

The JSON editor decodes configuration into `map[string]any`, causing `encoding/json` to convert every number to `float64` before re-encoding. Integers larger than 2^53 can lose precision even when they are unrelated to the MCP change. The OpenCode writer also overwrites an existing `$schema` value unconditionally. Editing a Render entry can therefore silently corrupt IDs, counters, timestamps, opaque application state, or a user's selected schema elsewhere in the same file.

## Files affected

- `internal/configedit/configedit.go:303-336`
- `internal/tools/opencode/opencode.go:129-135`

## Proposed solution

Use `json.Decoder.UseNumber` as an immediate precision fix, and preferably adopt a token- or AST-based surgical editor that preserves unrelated JSON bytes and values. Set `$schema` only when creating a new OpenCode file rather than replacing an existing value. Add regression tests containing integers above 2^53 and a custom `$schema`; after install and uninstall, both values must remain byte-for-byte or semantically unchanged.
