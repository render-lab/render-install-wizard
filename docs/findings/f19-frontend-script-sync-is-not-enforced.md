# F19: CI cannot enforce that the separately deployed frontend script stays in sync

- Severity: Medium
- Category: Operations
- Status: Gap

## Problem

The synchronization workflow compares only two files within this repository. Deployment instructions require manually copying the vendored installer into a separate frontend repository, but CI neither reads nor updates that deployed source. The production `render.com/agents.sh` endpoint can consequently remain stale after a security, correctness, or compatibility fix while this repository's synchronization check stays green.

## Files affected

- `.github/workflows/sync-agents-sh.yml:21-34`

## Proposed solution

Automate a pinned cross-repository update using least-privilege credentials, or make the frontend consume a versioned artifact produced by this repository. Add a post-deployment smoke check that compares the production response bytes or digest with the approved version. The release and rollback procedure should also purge relevant caches and verify production bytes after either operation, with CI failing when the deployed script diverges.
