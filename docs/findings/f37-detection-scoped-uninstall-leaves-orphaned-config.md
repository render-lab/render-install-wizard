# F37: Detection-scoped uninstall leaves orphaned MCP configuration

- Severity: Medium
- Category: Uninstall
- Status: Confirmed

## Problem

Uninstall resolves the same detected-tool set used for installation. If an agent executable or detection marker is removed after setup, its configuration file is no longer selected even when it still contains the managed Render MCP entry. An apparently successful uninstall can therefore leave active or dormant Render configuration behind, and the existing no-op-oriented result summary further obscures which targets were actually changed.

## Files affected

- `cmd/render-setup/main.go:81-100, 142-144`

## Proposed solution

For uninstall, inspect every known configuration location, or use the union of detected tools and files that contain a recognizable managed Render entry. Apply each tool's configuration-home precedence rules and report targets as changed, absent, skipped, or failed rather than treating detection as proof of cleanup. Add tests where each agent binary is removed after installation and verify uninstall still removes only the managed entry from all four clients while preserving sibling configuration.
