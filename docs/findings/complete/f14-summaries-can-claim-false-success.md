# F14: Summaries and next steps can claim success after doing nothing or failing

- Severity: Medium
- Category: Result semantics
- Status: Confirmed

## Problem

The summary title remains “Render setup complete” when individual steps fail. The MCP component's `Install` method is a no-op but is still recorded as installed, and next steps are derived from the requested plan rather than successful results. A run with no applicable target tools can also exit zero. Users and automation can therefore be told to run `render login` or use OAuth when the CLI or every tool configuration failed, while an explicit MCP-only request with no targets can appear successful despite making no change.

## Files affected

- `internal/orchestrator/summary.go:27-99`

## Proposed solution

Build the overall status, title, and next steps from actual `Result` and `StepResult` outcomes. Use distinct complete, partial, failed, skipped, and unchanged states; derive the MCP aggregate from per-tool configuration results. Fail an explicit MCP request when zero targets can be configured. Add tests for total failure, partial success, no applicable tools, MCP-only no-op, and next-step suppression when prerequisite steps fail.
