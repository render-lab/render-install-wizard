# F06: A TUI failure silently authorizes all default changes

- Severity: High
- Category: Fail-safe behavior
- Status: Confirmed

## Problem

When an interactive terminal is available but `wizard.Run` returns an error, `resolveSelection` logs a warning and returns `DefaultSelection`. A terminal initialization, rendering, cancellation, or context failure is therefore converted into authorization for the full default CLI, skills, and MCP installation. The user never completed the interactive consent flow, yet the process continues with all default mutations.

## Files affected

- `cmd/render-setup/main.go:112-118`

## Proposed solution

Fail closed when the interactive wizard returns an error: surface the cause and exit nonzero before building or executing an installation plan. Select all default components only when the user explicitly supplied `-y` or `--yes`, or when the documented non-TTY behavior applies. Add entrypoint tests that inject TUI initialization, rendering, cancellation, and context failures and assert that no component installer or configuration writer runs.
