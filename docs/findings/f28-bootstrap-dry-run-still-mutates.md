# F28: Bootstrap `--dry-run` still persists the wizard and shell PATH changes

- Severity: Medium
- Category: Dry-run semantics
- Status: Confirmed

## Problem

The shell bootstrap moves `render-setup` into `RENDER_HOME` and edits the user's shell startup file before forwarding `--dry-run` to the Go process. As a result, the documented `curl`-pipe entrypoint changes the machine even when the user supplies the conventional no-mutation flag. The Go component plan may be simulated, but the executable and persistent PATH configuration have already been installed.

## Files affected

- `scripts/agents.sh:201-227`

## Proposed solution

Detect `--dry-run` in the wrapper before any persistent operation. Download and verify the wizard into a temporary location, run that binary with `--dry-run`, and remove it without creating `RENDER_HOME` or editing shell files. If the intended contract is only a component-level dry run, rename the option and document that narrower behavior explicitly. Add bootstrap tests that snapshot the home directory and shell startup files and assert no changes after dry-run success or failure.
