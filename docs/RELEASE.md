# Release & Launch/Cutover Checklist

How to cut a release and flip on the `curl -fsSL render.com/agents.sh | sh` path.
See `docs/plan.md` Phase 6 for context.

## Cutting a release (repeatable)

1. Ensure `main` is green (CI: build/vet/gofmt/test/tidy + E2E).
2. Pick a semver tag `vX.Y.Z` and push it:
   ```bash
   git tag vX.Y.Z
   git push origin vX.Y.Z
   ```
3. `release.yml` runs GoReleaser, which builds the darwin/linux × amd64/arm64 binaries
   (`render-setup_<os>_<arch>`) plus `checksums.txt` and creates a **draft** GitHub Release.
4. Review the draft's assets, then publish it. `latest` now resolves to this release via
   `https://github.com/render-oss/render-install-wizard/releases/latest/download/<asset>`.
5. Smoke-test the published artifacts:
   ```bash
   RENDER_SETUP_VERSION=vX.Y.Z sh scripts/agents.sh --version   # in a throwaway HOME
   ```

## First launch / cutover (one-time)

- [ ] **render.com `/agents.sh` route** — merge the handler in the render.com frontend repo using
      the template in [`deploy/render-com/`](../deploy/render-com/). It must serve the vendored
      `agents.sh` as `text/x-shellscript`, Cloudflare-cached. Confirm `curl -fsSL render.com/agents.sh`
      returns the script and a browser hitting `/agents` still gets the Sanity page.
- [ ] **Keep the vendored copy in sync** — `deploy/render-com/agents.sh` must stay byte-identical to
      `scripts/agents.sh` (enforced by `sync-agents-sh.yml`). Re-sync on any bootstrap change.
- [ ] **First public release** published (see above); checksums visible on the release.
- [ ] **End-to-end from a clean machine**: `curl -fsSL render.com/agents.sh | sh` installs and runs.
- [ ] **Install analytics** — decide whether binary downloads stay on GitHub Releases or render.com
      proxies them for count/OS-arch/referrer metrics (see spec Phase 4 decision), then wire it.
- [ ] **Rollback plan** — documented below.

## Rollback

- **Bad bootstrap script**: revert the `agents.sh` change on `main`, re-sync the vendored copy, and
  redeploy the render.com route (Cloudflare serves the reverted script).
- **Bad binary release**: unpublish/delete the bad GitHub Release (so `latest` falls back to the
  previous one), or tell users to pin `RENDER_SETUP_VERSION=<previous tag>`.

## Pre-launch residuals (from `docs/plan.md`)

- Confirm the Render CLI's own `render skills install` non-interactive behavior (the npx path already
  uses `--all -g`; hang risk is eliminated via unset child stdin).
- Optional: wire the wizard's next-steps to consume Sanity `/agents/*.md` instead of local copy.
- Windows: the curl path scopes to macOS/Linux/WSL; native Windows is covered by the WinGet manifest.
