# F07: New agent config files are forced to mode 0644

- Severity: Medium
- Category: Local security
- Status: Confirmed

## Problem

`atomicWrite` defaults new configuration files to mode `0644` and explicitly applies that mode to its temporary file, overriding the safer `0600` mode created by `os.CreateTemp`. On a multi-user system with traversable home directories, other local users can read newly created agent configurations. Those files may later contain MCP authorization headers, API tokens, or, for Claude, unrelated account and session state.

## Files affected

- `internal/configedit/configedit.go:176-219`

## Proposed solution

Create wizard-owned configuration files with mode `0600` and wizard-owned parent directories with mode `0700`. When updating an existing file, preserve its current mode and never broaden permissions; retain stricter modes when present. Add tests under a permissive umask that verify new file and directory permissions and confirm that updating an existing `0600` file does not make it more permissive.
