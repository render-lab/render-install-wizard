# F16: The centralized API-key fallback emits invalid syntax for three clients

- Severity: Medium
- Category: Authentication
- Status: Confirmed

## Problem

The centralized API-key fallback emits `Bearer $RENDER_API_KEY` for every target, but the clients require different environment-variable syntax. Claude expects `${RENDER_API_KEY}`, Cursor expects `${env:RENDER_API_KEY}`, and Codex requires `bearer_token_env_var` or `env_http_headers` instead of a static `http_headers` value. If OAuth is disabled and this advertised fallback is enabled in a build, Claude, Cursor, and Codex send a literal placeholder or fail authentication. Only the OpenCode representation is correct.

## Files affected

- `internal/render/render.go:57-69`

## Proposed solution

Move authentication rendering into each target implementation and generate the exact schema and placeholder syntax required by that client. Keep the API key in the environment rather than materializing its value in a config file. Add parser or integration fixtures for all four clients that resolve a test environment variable and verify the resulting request carries the expected bearer credential without storing the secret.
