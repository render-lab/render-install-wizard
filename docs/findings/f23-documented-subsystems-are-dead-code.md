# F23: Large documented subsystems and interface methods are test-only dead code

- Severity: Low
- Category: Maintainability
- Status: Confirmed

## Problem

The production dependency graph does not include the `manifest`, `content`, `logx`, or `paths` packages. Installer methods such as `Detect`, `Status`, `ID`, and `PreferredDelivery`, along with wizard summary and default helpers, are likewise used only by tests. Tests therefore create confidence in behavior the shipped binary never executes. The dormant manifest delivery metadata and embedded plugin copy have already drifted from the runtime's raw delivery path, increasing maintenance cost and obscuring the real architecture.

## Files affected

- `internal/manifest/`
- `internal/content/`
- `internal/logx/`
- `internal/paths/`

## Proposed solution

Choose one production architecture. Either connect these packages and interface methods through a single end-to-end installation plan with explicit failure semantics, or delete the unused code and tests. Update architecture documentation to describe the resulting runtime path, then add a dependency or integration check that exercises the chosen implementation from `cmd/render-setup` so test-only substitutes cannot silently drift from shipped behavior.
