# F26: The new Windows plan omits runtime component incompatibilities

- Severity: Low
- Category: Windows planning
- Status: Gap

## Problem

The project cross-compiles for both Windows targets, but the default CLI component still falls through to `sh -c 'curl … | sh'`, which is not a native Windows installation path. Tool-specific path resolution and atomic configuration writes are also not exercised on Windows. Adding only a PowerShell bootstrap and release artifacts would therefore produce a binary that starts on Windows but cannot complete its default setup reliably.

## Files affected

- `docs/remaining-work.md:51-84`

## Proposed solution

Expand the Windows plan to include a native, checksum-verified CLI installer; Windows-specific home, configuration, and executable path resolution; and atomic-write behavior that is correct for Windows filesystem semantics. Add native Windows runtime end-to-end coverage for default install, agent configuration, rerun, and uninstall. Do not describe the binary as functionally Windows-ready until those runtime tests pass, even if cross-compilation succeeds.
