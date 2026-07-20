# F08: Recursive merge retains stale or foreign fields inside the `render` entry

- Severity: Medium
- Category: Secret handling
- Status: Confirmed

## Problem

The configuration editor recursively merges into an existing server named `render` instead of replacing the wizard-owned entry. An OAuth patch omits headers, so headers from an earlier API-key configuration remain; stale `command`, `args`, and other transport fields remain as well. A migration or name collision can therefore create an invalid hybrid configuration. More seriously, sensitive headers belonging to a pre-existing `render` server can be retained and sent to the newly written Render URL.

## Files affected

- `internal/configedit/configedit.go:141-154`

## Proposed solution

Replace the entire wizard-owned `render` server value while preserving sibling server entries. Before replacement, identify whether an existing `render` value is a compatible managed entry; if it points elsewhere or is otherwise foreign, require explicit confirmation or preserve it under a clearly named backup. Add tests for API-key-to-OAuth migration, stale transport-field removal, sibling preservation, and a conflicting non-Render entry to ensure no foreign headers are carried forward.
