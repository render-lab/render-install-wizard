# render.com integration — `GET /agents.sh`

This directory is the integration handed to the **render.com frontend repo** (the
existing Next.js + Sanity app behind Cloudflare). It contains everything the
frontend needs to serve the install bootstrap at `render.com/agents.sh`.

## The gap this fills

render.com already serves, via Accept-header content negotiation:

- `GET /agents` → `text/html` (Sanity landing page) for browsers
- `GET /agents` (`Accept: text/markdown`), `/agents.md`, `/agents/*.md`, `/llms.txt`
  → `text/markdown` agent briefing

The only missing route is:

- `GET /agents.sh` → **currently 404**

That 404 is the exact "install.sh 404" fragility called out in `docs/SPEC.md`.
Adding one route handler makes the documented one-liner work:

```bash
curl -fsSL render.com/agents.sh | sh
```

## Files in this directory

| File               | Purpose                                                                 |
| ------------------ | ----------------------------------------------------------------------- |
| `agents.sh`        | **Vendored, byte-identical** copy of [`scripts/agents.sh`](../../scripts/agents.sh). The bytes the frontend actually serves. |
| `route-handler.ts` | Next.js App Router handler template for `app/agents.sh/route.ts`.       |
| `README.md`        | This file.                                                              |

## Why the script is vendored (and kept byte-identical)

The bootstrap is **git-owned, auditable, PR-able, and checksummed** — it is *not*
a Sanity document. The source of truth is `scripts/agents.sh` in
`github.com/render-lab/render-install-wizard`. The copy here is a verbatim mirror
so the frontend can serve it without reaching back into another repo at request
time.

Drift between the two copies is a release hazard (the frontend could serve a stale
or divergent script), so it is enforced by CI:

- `.github/workflows/sync-agents-sh.yml` runs `diff -u scripts/agents.sh
  deploy/render-com/agents.sh` on every push/PR and **fails on any difference**,
  with a message telling the author to re-sync.

To re-sync after editing the source script:

```bash
cp scripts/agents.sh deploy/render-com/agents.sh
```

## How the frontend consumes it

1. Copy `route-handler.ts` into the frontend repo as `app/agents.sh/route.ts`.
2. Copy `agents.sh` next to it (e.g. `app/agents.sh/agents.sh`) — or wire the
   handler's `readFileSync` path to wherever you vendor the script.
3. The handler reads the script from disk once at cold start and returns it with:
   - `Content-Type: text/x-shellscript; charset=utf-8`
   - `Cache-Control: public, max-age=300, s-maxage=300, stale-while-revalidate=86400`
   - `CDN-Cache-Control:` (same) so Cloudflare caches independently of browsers
   - `X-Content-Type-Options: nosniff`

Local verification in the frontend repo:

```bash
curl -i http://localhost:3000/agents.sh
# 200, Content-Type: text/x-shellscript; charset=utf-8, script body
```

## Caching notes

- **5-minute edge cache** (`max-age=300` / `s-maxage=300`). The script changes
  rarely (only on a bootstrap edit + redeploy), so a short TTL keeps the endpoint
  fast and cheap while still propagating changes quickly.
- `stale-while-revalidate=86400` lets Cloudflare serve a slightly stale copy while
  it refreshes in the background, avoiding a cache-miss stampede right after a
  deploy.
- Because the body is deterministic per deploy, the handler is marked
  `force-static` so Next.js can cache the rendered response.
- **Rollback:** revert the frontend PR (or pin a previous deploy). Users can also
  pin behavior at call time via the script's env knobs:
  - `RENDER_INSTALL_BASE_URL` — override the download origin (default `https://render.com`)
  - `RENDER_SETUP_VERSION` — pin the wizard version (default `latest`)
  - `RENDER_HOME` — override the install root (default `~/.render`)

## What the served script does (summary)

Detect OS/arch (macOS, Linux, WSL; native Windows → friendly WinGet message) →
download `render-setup_<version>_<os>_<arch>` + `checksums.txt` →
**sha256-verify** → install to `~/.render/bin/render-setup` → idempotently update
PATH (zsh/bash/fish) → re-attach `/dev/tty` and `exec` the wizard, forwarding args.
No `sudo`; everything lives under `$RENDER_HOME`. The whole script is wrapped in
`main()` invoked on the last line, so a truncated download executes nothing.
