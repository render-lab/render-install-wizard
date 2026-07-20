# F34: Project-local npm state can redirect the global skills installer

- Severity: High
- Category: Local supply chain
- Status: Risk

## Problem

The unversioned `npx skills` invocation inherits the caller's current working directory, environment, local `node_modules/.bin`, and npm configuration. Launching the global setup from an untrusted checkout allows a project-local binary or repository `.npmrc` to influence which code executes and where packages are fetched from. Attacker-controlled code can run as the user and then persist malicious instructions in agent skill directories.

## Files affected

- `internal/components/skills/skills.go:39-46, 78-87`

## Proposed solution

Run package installation from a controlled empty directory with a sanitized npm environment and configuration, and prevent resolution through project-local binaries. Pin the installer package and skills source to exact immutable versions or revisions. Prefer a verified first-party installer that does not inherit npm project state. Add tests with malicious local `node_modules/.bin` and `.npmrc` fixtures, asserting that neither is read or executed and that only the pinned trusted distribution is used.
