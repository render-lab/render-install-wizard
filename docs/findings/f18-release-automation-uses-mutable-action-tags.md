# F18: Write-capable release automation depends on mutable action tags

- Severity: High
- Category: Release security
- Status: Risk

## Problem

The release workflow uses mutable major-version tags for checkout and GoReleaser while the job has `contents: write` permission and checkout credentials. The GoReleaser version is selected dynamically, the release job does not run tests, and it runs `go mod tidy` without failing if the module graph changes. An upstream action-tag compromise could access release credentials or tamper with assets. A tag can also publish binaries from a dependency graph that differs from the tagged source. Because the same release identity publishes both binaries and checksums, checksum matching is not an independent integrity control.

## Files affected

- `.github/workflows/release.yml:8-32`

## Proposed solution

Pin every action and release tool to an immutable commit SHA or digest, disable persisted checkout credentials, and use a protected release environment with least-privilege credentials. Require the exact commit that passed tests, replace release-time `go mod tidy` with a clean-tree/module-consistency check, and independently sign or attest artifacts. Add a release preflight that verifies pinned references, a clean checkout, expected commit identity, and successful verification of signatures or provenance.
