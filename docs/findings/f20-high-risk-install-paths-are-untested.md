# F20: The highest-risk entrypoint and clean install paths are not tested

- Severity: Medium
- Category: Testing
- Status: Gap

## Problem

Test coverage for `cmd/render-setup` is 0.0%, leaving the process entrypoint and its highest-blast-radius decisions unexercised. The hermetic end-to-end test covers only explicit MCP configuration; it does not execute the default CLI-and-skills installation, verify that `--agent` scopes skills, or assert Claude cleanup during uninstall. Parser safety, no-tool behavior, summary truthfulness, PATH propagation, and clean-machine automation can therefore regress while every current check passes.

## Files affected

- `cmd/render-setup/main.go:29-76`

## Proposed solution

Refactor entrypoint dependencies behind injectable interfaces for terminal selection, process execution, filesystem access, and networking. Add table-driven tests for parsing, target resolution, exit codes, summaries, and failed consent flows. Add a network-stubbed clean-machine end-to-end test for the default CLI-and-skills path, including no Node, PATH updates, and scoped agents. Extend uninstall assertions to verify removal for Claude, Cursor, Codex, and OpenCode.
