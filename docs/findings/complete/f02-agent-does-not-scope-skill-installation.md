# F02: `--agent` does not scope skill installation

- Severity: High
- Category: Scope
- Status: Confirmed

## Problem

The orchestrator does not pass the selected tools to component installers, and the skills installer always invokes its command with `--all`. That flag means all skills for all detected agents, so `render-setup --components skills --agent cursor` can modify Claude Code, Codex, OpenCode, and universal skill directories despite the explicit Cursor scope. The command therefore violates the user's stated mutation boundary and can alter unrelated agent environments.

## Files affected

- `internal/components/skills/skills.go:78-88`

## Proposed solution

Add selected tool IDs to the component options passed by the orchestrator. Build the skills command with one explicit `--agent` value for each selected target, and use `--all` only when the run is intentionally unscoped. Add command-construction and end-to-end tests proving that a Cursor-only request writes only Cursor-owned locations and that unscoped behavior still targets all detected agents.
