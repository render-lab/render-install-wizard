# F42: Deployment handoff documents the wrong download defaults and names

- Severity: Low
- Category: Documentation
- Status: Confirmed

## Problem

The deployment handoff says the default download origin is `render.com` and that release artifact filenames include the version. In practice, the bootstrap defaults to GitHub Releases and GoReleaser emits versionless artifact names. Frontend integration and incident-response procedures can therefore be implemented against URLs that never exist, delaying deployment or recovery when operators follow the documented examples.

## Files affected

- `deploy/render-com/README.md:83-94`

## Proposed solution

Correct the origin, artifact names, and example URLs so they match the bootstrap and GoReleaser configuration. Prefer generating the relevant documentation snippets from shared release constants, or add tests that extract every documented URL and compare it with script and release outputs. Include the documentation check in release preflight so future changes to download origin or naming cannot leave the handoff stale.
