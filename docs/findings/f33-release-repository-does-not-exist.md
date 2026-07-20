# F33: The configured release repository does not exist

- Severity: High
- Category: Release blocker
- Status: Confirmed

## Problem

The source repository is `render-lab/render-install-wizard`, while GoReleaser targets `render-oss/render-install-wizard`. The configured destination and its release and checksum URLs returned 404 during verification. Even if that repository is later created, the workflow's repository-scoped `GITHUB_TOKEN` cannot publish into a different repository. The first tag therefore cannot publish the documented release, and every bootstrap URL that depends on those assets remains unavailable.

## Files affected

- `.goreleaser.yaml:42-45`
- `.github/workflows/release.yml:25-32`

## Proposed solution

Create or transfer the intended public repository, then align the Go module, GoReleaser destination, source links, and bootstrap download URLs to one authoritative coordinate. If cross-repository publishing is intentional, use a least-privilege GitHub App token authorized for the destination rather than the source-scoped token. Add a release preflight that verifies repository existence, token access, tag/commit identity, and expected asset URLs before any publication step runs.
