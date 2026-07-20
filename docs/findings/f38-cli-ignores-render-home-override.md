# F38: The CLI component ignores the supported `RENDER_HOME` override

- Severity: Medium
- Category: Path consistency
- Status: Confirmed

## Problem

The bootstrap and `paths` package honor `RENDER_HOME`, but CLI detection and uninstall hardcode `$HOME/.render/bin`. With a custom Render home, installation state is split across two roots. Status and uninstall can miss the managed executable, and assumptions about where the binary should appear on `PATH` diverge between the bootstrap, component, and same-run follow-up steps.

## Files affected

- `internal/paths/paths.go:37-47`
- `internal/components/cli/cli.go:41-48, 65-68`

## Proposed solution

Resolve and validate the Render home once at startup, require an absolute safe path, and inject the resulting executable and data paths into bootstrap guidance, CLI, skills, and orchestrator code. Remove independent hardcoded reconstruction of `$HOME/.render`. Add tests with a custom `RENDER_HOME` covering install, detection, status, same-run command discovery, and uninstall, and assert that no files are read from or written to the default root.
