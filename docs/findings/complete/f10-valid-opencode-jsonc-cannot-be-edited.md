# F10: Valid OpenCode JSONC configurations cannot be edited

- Severity: Medium
- Category: Compatibility
- Status: Confirmed

## Problem

OpenCode officially supports comments, trailing commas, and the `opencode.jsonc` filename, but the wizard looks only for `opencode.json` and parses it with strict `encoding/json`. A user with a valid JSONC configuration can therefore receive a failed install or uninstall, or the wizard can create a second JSON file that is not the configuration OpenCode actually reads. In either case, the requested Render MCP change may not reach the active client configuration.

## Files affected

- `internal/tools/opencode/opencode.go:76-90`

## Proposed solution

Resolve both `opencode.json` and `opencode.jsonc` according to OpenCode's documented precedence rules, then edit the selected file with a JSONC-aware, comment-preserving implementation. Use the same resolver for detection, installation, status, and uninstall. Add fixtures with comments, trailing commas, both filenames, and only `opencode.jsonc`, and verify that the active file is changed without losing formatting or creating an inactive duplicate.
