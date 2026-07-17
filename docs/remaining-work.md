# Remaining Work

Single index of what's left. Phases 0–5 are complete and committed; the wizard is built,
integrated, releasable, and E2E-tested. What remains is **Phase 6 (launch/cutover)** — mostly
out-of-repo coordination — plus a couple of optional enhancements and open decisions.

Canonical detail lives in [`plan.md`](plan.md) (status table, Phase 6, Residuals, Open questions)
and [`RELEASE.md`](RELEASE.md) (step-by-step cutover + rollback). This file just gathers it.

## 1. Launch / cutover — required, mostly out-of-repo

These need access/credentials this repo doesn't have. Full steps in [`RELEASE.md`](RELEASE.md).

- [ ] **render.com `/agents.sh` route** — merge the handler in the render.com frontend repo using
      the template in [`../deploy/render-com/`](../deploy/render-com/). Must serve the vendored
      `agents.sh` as `text/x-shellscript`, Cloudflare-cached; keep it byte-identical to
      `scripts/agents.sh` (enforced by `sync-agents-sh.yml`).
- [ ] **First release** — push a `vX.Y.Z` tag; GoReleaser publishes binaries + `checksums.txt` to
      GitHub Releases as a draft; review and publish.
- [ ] **Clean-machine end-to-end** — verify `curl -fsSL render.com/agents.sh | sh` works in prod.
      (CI's e2e matrix exercises the real CLI + skills network installs on push.)
- [ ] **Install analytics** — count / OS-arch / referrer. Depends on the open decision below about
      where binaries are served.
- [ ] **Rollback plan** confirmed (documented in `RELEASE.md`).

## 2. Optional in-repo enhancements

Nice-to-haves; none block launch.

- [ ] **Wizard next-steps from Sanity** — today next-steps are built locally (`render.PluginFor` +
      static lines). Optionally fetch per-tool copy from `render.com/agents/*.md` so it stays in
      lockstep with the website. (Frontend may need a structured `nextSteps` field — see Open Q3.)
- [ ] **Skills freshness** — decide background auto-update vs. re-running setup (Open Q1).

## 3. Open decisions

- **Binary hosting for analytics** — keep binaries on GitHub Releases (current) or have render.com
  proxy downloads to capture install analytics? (spec Phase 4 decision.)
- **Sanity schema** — do the `/agents/*.md` pages need a structured `nextSteps` field for clean TUI
  rendering, vs. scraping full-page markdown? (Frontend-owned.)

## Done — closed residuals (for reference)

- ✅ Non-interactive skills flags (`npx skills add render-oss/skills --all -g`).
- ✅ Failure-injection E2E (`test/e2e/failure_check.sh`).
- ✅ `LICENSE` (Apache-2.0).
- ✅ Download URL scheme reconciled (GitHub Releases, version-less names, `latest` redirect).
- ✅ Bootstrap `/dev/tty`-openable fix for no-TTY environments.
- ✅ Uninstall scoped to Render MCP config removal.

## Platform note

- Native **Windows** is intentionally out of scope for the curl path (macOS/Linux/WSL only); it's
  covered by the WinGet manifest. WSL is exercised via the Linux build (no hosted WSL CI runner).
