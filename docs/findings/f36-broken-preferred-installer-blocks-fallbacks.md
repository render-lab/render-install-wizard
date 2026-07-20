# F36: A present but broken preferred installer prevents viable fallbacks

- Severity: Medium
- Category: Fallback logic
- Status: Confirmed

## Problem

When Homebrew is present, any failure from `brew install render` is returned immediately instead of trying the official installer. Likewise, when `npx` exists, any skills installation failure prevents use of an available Render CLI fallback. A stale package manager, registry outage, or partial local installation can therefore fail setup even when the next supported delivery method is available and would succeed.

## Files affected

- `internal/components/cli/cli.go:118-129`
- `internal/components/skills/skills.go:78-88`

## Proposed solution

Classify installer failures as recoverable or terminal and attempt the next supported method after recoverable failures. Preserve and show diagnostics from each attempt, and return an aggregate error only when all applicable methods fail. Do not fall back after integrity or policy failures that should stop execution. Add tests for Homebrew failure followed by official-installer success, `npx` failure followed by Render CLI success, all-method failure, and a non-recoverable verification failure that prevents fallback.
