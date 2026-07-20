# F21: Home-directory lookup failures can fall back to project-relative config paths

- Severity: Low
- Category: Path safety
- Status: Risk

## Problem

Tool constructors ignore errors from `os.UserHomeDir` and retain an empty home directory. `MCPConfigPath` accepts that empty or non-absolute value, so `filepath.Join` produces relative paths such as `.claude.json` and `.cursor/mcp.json`. In an unusual container or UID environment, a run with an explicit `--agent` can therefore perform a supposedly global configuration change inside the current working repository instead of failing safely.

## Files affected

- `internal/render/render.go:78-92`

## Proposed solution

Make home-directory and configuration-path resolution return errors, and require a non-empty absolute home before constructing any mutation target. Propagate the error through planning so no installer or writer runs when path resolution fails. Add tests for `os.UserHomeDir` failure, empty and relative homes, and explicit-agent operation from a writable project directory; each invalid case must exit nonzero without creating project-relative configuration files.
