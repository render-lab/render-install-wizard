# Release & Launch

What's left to ship `curl -fsSL render.com/agents.sh | sh`, plus the runbook to cut releases and the
rollback plan. Phases 0–5 are complete (see [`SPEC.md`](SPEC.md)); Phase 6 (launch/cutover) is what
remains — mostly out-of-repo coordination.

## Launch punch list

### Blocking — required to launch

- [ ] **render.com `/agents.sh` route** — merge the handler in the render.com frontend repo using the
      template in [`../deploy/render-com/`](../deploy/render-com/). Serves the vendored `agents.sh` as
      `text/x-shellscript`, Cloudflare-cached; kept byte-identical to `scripts/agents.sh` by
      `sync-agents-sh.yml`. Confirm `curl -fsSL render.com/agents.sh` returns the script and a browser
      hitting `/agents` still gets the Sanity page.
- [ ] **First release** — cut and publish (see runbook below). Checksums visible on the release.
- [ ] **Clean-machine end-to-end** — `curl -fsSL render.com/agents.sh | sh` installs and runs in
      production on a fresh macOS and Linux box.

### Decide before / at launch

- [ ] **Binary hosting + install analytics** — keep binaries on GitHub Releases (current) or have
      render.com proxy downloads to capture count / OS-arch / referrer metrics, then wire analytics.
      Analytics can follow launch; the hosting decision shouldn't.
- [ ] **Rollback confirmed** — see below.

### Already handled (no action needed)

Apache-2.0 LICENSE · checksum-verified bootstrap + failure-injection tested · non-interactive skills
install · download URL scheme reconciled (GitHub Releases, `latest` redirect) · scoped uninstall.

### Out of scope for this launch

Native Windows curl/PowerShell path and optional enhancements — see [`FUTURE.md`](FUTURE.md).

---

## Runbook — cutting a release (repeatable)

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
5. Smoke-test the published artifacts (in a throwaway `HOME`):
   ```bash
   RENDER_SETUP_VERSION=vX.Y.Z sh scripts/agents.sh --version
   ```

## Rollback

- **Bad bootstrap script**: revert the `agents.sh` change on `main`, re-sync the vendored copy, and
  redeploy the render.com route (Cloudflare serves the reverted script).
- **Bad binary release**: unpublish/delete the bad GitHub Release (so `latest` falls back to the
  previous one), or tell users to pin `RENDER_SETUP_VERSION=<previous tag>`.
