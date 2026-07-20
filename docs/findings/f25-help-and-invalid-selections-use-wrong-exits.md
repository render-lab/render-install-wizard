# F25: Help and explicit invalid selections use misleading exit semantics

- Severity: Low
- Category: CLI quality
- Status: Confirmed

## Problem

Requesting `--help` is surfaced as `flag: help requested` and exits with status 2 even though displaying help is a normal successful operation. Unknown explicit agents and components are only warned about and dropped. If every supplied value is invalid, the process can still perform unrelated default work or exit zero. Shell automation therefore treats help as a failure, while misspelled scopes can produce a successful operation materially different from what the user requested.

## Files affected

- `cmd/render-setup/main.go:29-47, 127-148`

## Proposed solution

Recognize `flag.ErrHelp`, print usage, and return exit status 0 without executing a plan. Validate explicit agent and component sets before selection: fail when any requested operation has no applicable target, and at minimum fail when an explicit set is wholly invalid. Add table-driven entrypoint tests for help, mixed valid and invalid values, all-invalid values, and an operation with no applicable targets, asserting both exit codes and zero unintended mutations.
