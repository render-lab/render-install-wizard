# F39: The linter gate is failing but deliberately masked

- Severity: Medium
- Category: CI quality gate
- Status: Confirmed

## Problem

The inspected CI run installed golangci-lint 1.64.8, which cannot analyze this Go 1.26 project, so the lint job failed. The step is configured with `continue-on-error: true`, allowing the overall workflow to remain green. Lint regressions can therefore merge under a successful status, and the repository currently receives no enforceable signal from its configured broad static-analysis suite.

## Files affected

- `.github/workflows/ci.yml:41-56`

## Proposed solution

Pin a golangci-lint action and binary version that explicitly supports Go 1.26, migrate the linter configuration as required, and fix the resulting findings. Remove `continue-on-error` so a linter failure fails the required check. Add a CI self-check or version assertion that verifies the selected linter can load the repository's Go version, preventing a future toolchain upgrade from silently disabling the quality gate.
