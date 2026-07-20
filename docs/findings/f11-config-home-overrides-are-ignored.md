# F11: Supported tool configuration-home overrides are ignored

- Severity: Medium
- Category: Compatibility
- Status: Confirmed

## Problem

Agent configuration paths are hardcoded beneath the operating-system home directory. The wizard does not honor `CLAUDE_CONFIG_DIR`, `CODEX_HOME`, `XDG_CONFIG_HOME`, or `OPENCODE_CONFIG`, even though the corresponding tools support those overrides. With a custom configuration home, the wizard can detect or write an inactive file while the agent continues using a different location. Uninstall can likewise report success after removing an entry from the wrong file while the active Render entry remains.

## Files affected

- `internal/render/render.go:78-92`

## Proposed solution

Centralize configuration-path resolution for each supported tool and implement its documented environment-variable and precedence rules. Inject the resolved paths into detection, configuration, status, and uninstall so every operation uses the same location. Add table-driven tests for each override, its default path, precedence when multiple variables are set, and install/uninstall behavior against the resolved active file.
