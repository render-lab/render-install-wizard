# F12: `--pin-version` is parsed and advertised but has no effect

- Severity: Medium
- Category: Versioning
- Status: Confirmed

## Problem

The CLI parser stores `PinVersion`, but no production code reads it. The orchestrator always passes an empty `components.Options`, and the CLI installer ignores `Options.Version`. A user can therefore request `--pin-version` and receive the same mutable or latest-version installation as an unpinned run. The reproducibility and rollback behavior advertised by the flag is not implemented.

## Files affected

- `internal/cliflags/flags.go:27-30, 67-69`

## Proposed solution

Define precisely whether the flag pins the wizard, Render CLI, manifest, skills, or a coordinated release set. Thread explicit version fields from parsed flags through selection, planning, and component options, then resolve and verify every selected artifact at that version. Reject unsupported or incomplete version combinations. Add tests that compare pinned and unpinned command plans and verify that a pinned run downloads only immutable, checksum-verified assets for the requested version.
