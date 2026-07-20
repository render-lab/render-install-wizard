# F31: The E2E Docker context has no secret-exclusion boundary

- Severity: Low
- Category: Build hygiene
- Status: Risk

## Problem

The repository has no `.dockerignore`, and the end-to-end build stage uses `COPY . .`. Local `.env` files, Git metadata, credentials, editor state, and build artifacts can therefore be sent into the Docker context and retained in local or remote builder cache layers. The final runtime image copies only the compiled binary, but that does not remove sensitive files from shared build infrastructure or previously created cache layers.

## Files affected

- `test/e2e/Dockerfile:14-19`

## Proposed solution

Add a restrictive root `.dockerignore` that excludes secrets, Git metadata, local configuration, build output, and unrelated files. Narrow the Dockerfile to copy only `go.mod`, `go.sum`, required Go source, and explicit test fixtures. Add a build-context test or policy check with sentinel secret filenames that fails if excluded files become visible to a build stage, and document the minimal files intentionally included.
