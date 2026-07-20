# F24: Disconnected fetch/render code carries latent advisories and unbounded reads

- Severity: Low
- Category: Dependency debt
- Status: Gap

## Problem

Whole-repository vulnerability scanning reports `GO-2026-5856` for Go 1.26.4 and `GO-2026-5320` for goldmark 1.7.13. The shipped command currently reports no reachable vulnerabilities only because the `content` and `manifest` packages are disconnected from production. Both fetchers also call `io.ReadAll` without limiting response size. Connecting the planned remote content or manifest path before addressing these issues would make the advisories reachable and allow an oversized response to exhaust process memory.

## Files affected

- `go.mod:3, 42`
- `internal/content/fetch.go:30-45`

## Proposed solution

Upgrade to Go 1.26.5 or newer and goldmark 1.7.17 or newer. Bound every remote body with `http.MaxBytesReader` or `io.LimitedReader`, reject oversized content, and validate expected content types before parsing. Add tests for oversized, malformed, and wrong-content-type responses. Before wiring these packages into the command, require `govulncheck` against the production binary graph to pass with the new path enabled.
