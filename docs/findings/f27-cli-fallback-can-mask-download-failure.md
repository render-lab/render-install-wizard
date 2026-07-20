# F27: The CLI fallback can return success when its download failed

- Severity: High
- Category: False success
- Status: Confirmed

## Problem

The fallback runs `sh -c 'curl -fsSL URL | sh'` without enabling `pipefail`. POSIX pipeline status is determined by the final `sh` process, so if `curl` fails and `sh` receives an empty program, `sh` can exit zero. This behavior has been reproduced. The CLI component is then recorded as installed, the overall setup can exit successfully, and subsequent guidance tells the user to run an executable that was never downloaded.

## Files affected

- `internal/components/cli/cli.go:126-129`

## Proposed solution

Remove the download-and-execute pipeline. Download into a temporary file as a separately checked operation, verify its authenticity, execute it only after verification, and remove it safely afterward. Before returning success, resolve the expected installed binary and verify its version. Add tests where the download fails, returns an empty body, fails verification, or executes without installing the binary; every case must return a failed step and nonzero overall result.
