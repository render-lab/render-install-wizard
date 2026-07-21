# F17: Network subprocesses have no deadline and discard actionable output

- Severity: Medium
- Category: Reliability
- Status: Confirmed

## Problem

The root context has no timeout, `curl` has no maximum duration, and `brew`, `npx`, and `render` subprocesses inherit no deadline. Installation also discards standard output and standard error, returning only an exit status. A stalled network or interactive child process can make setup appear hung indefinitely. When a command does fail, actionable details such as a missing `unzip`, PATH instructions, npm diagnostics, or an unexpected prompt are hidden from both users and JSON consumers.

## Files affected

- `internal/components/cli/cli.go:40-58`

## Proposed solution

Give each network and package-manager step a bounded child context, configure command-specific network timeouts, and retry only operations that are safe and idempotent. Capture or stream subprocess output with secret redaction, and include structured, actionable failure causes in `StepResult`. Add tests for deadline cancellation, a hung child process, redacted output, and propagation of representative `curl`, Homebrew, npm, and Render CLI errors.
