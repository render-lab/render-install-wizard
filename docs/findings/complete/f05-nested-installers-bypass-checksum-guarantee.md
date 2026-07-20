# F05: Nested installers bypass the bootstrap's checksum guarantee

- Severity: High
- Category: Supply chain
- Status: Confirmed

## Problem

Although the bootstrap verifies the top-level wizard binary, the default run then executes a mutable CLI installer from the `main` branch through `curl | sh`. That script currently downloads a CLI archive without verifying the published `SHA256SUMS` or signature. Skills installation also executes an unversioned `npx skills` package against an unpinned repository. A compromise or unexpected update in either nested delivery path can execute unverified remote code after the verified wizard starts, so the documented guarantee that every downloaded binary is verified does not hold end to end.

## Files affected

- `internal/render/render.go:26-35`

## Proposed solution

Download pinned Render CLI assets directly and verify their signed checksums before execution or installation. Pin the npm installer to an exact version and select an explicit skills revision, or replace that path with a first-party verified distribution. Add tests that reject checksum/signature mismatches and assert that generated download and package commands contain immutable versions rather than mutable branches or unversioned package references.
