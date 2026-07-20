# F40: The current cross-platform E2E workflow is not green

- Severity: Medium
- Category: E2E automation
- Status: Confirmed

## Problem

In the inspected workflow run, the `macos-13` job waited until cancellation and `macos-latest` failed to reach the hidden Python test server. The server lifecycle also left an orphan process for runner cleanup. Although the deterministic local MCP harness passes, the cross-platform end-to-end release signal is unavailable because the hosted workflow can fail or hang for infrastructure and lifecycle reasons.

## Files affected

- `.github/workflows/e2e.yml:16-21`
- `test/e2e/bootstrap_check.sh:28-30`

## Proposed solution

Replace unavailable runner labels, bind the test server explicitly to loopback, and poll a readiness endpoint before starting the client. Capture the actual server PID and logs, add per-job and per-command timeouts, and install a trap that always terminates the process. Add failure-path checks for startup errors and unreachable ports, then require the repaired macOS jobs alongside the deterministic harness before release.
