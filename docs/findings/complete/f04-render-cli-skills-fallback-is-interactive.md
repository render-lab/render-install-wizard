# F04: The Render CLI skills fallback is not invoked non-interactively

- Severity: High
- Category: Automation
- Status: Confirmed

## Problem

The fallback executes only `render skills install`. In Render CLI v2.20.0 this command is interactive by default and exposes `--confirm`, `--tool`, and `--skill` for unattended use, but none of those flags are supplied. Because the subprocess has nil standard input, its prompts receive EOF instead of a valid selection. On a clean machine without Node or `npx`, the default setup can therefore install the Render CLI successfully and then fail during skills installation, leaving the machine only partially configured.

## Files affected

- `internal/components/skills/skills.go:82-87`

## Proposed solution

Invoke the fallback with explicit tool, skill, and scope selections plus `--confirm`. Resolve and execute the newly installed CLI by its verified absolute path rather than relying on a possibly stale `PATH`. Add a clean-machine test with Node and `npx` unavailable that supplies no stdin, verifies the exact fallback arguments, and confirms the complete default installation succeeds without prompts.
