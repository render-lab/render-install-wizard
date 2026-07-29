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

## Runbook — release + go live (turnkey)

Binaries are hosted on **GitHub Releases**; render.com serves **only** the `agents.sh`
script (see [`../deploy/render-com/`](../deploy/render-com/)). The script downloads the
binary from GitHub, so both must be in place. Do these in order.

### 1. Release the wizard binary

1. Ensure `main` is green (CI: build / vet / gofmt / golangci-lint / test / tidy).
2. Tag and push a semver tag:
   ```bash
   git tag vX.Y.Z && git push origin vX.Y.Z
   ```
   `release.yml` runs a preflight (the workflow must run in `render-lab/render-install-wizard`,
   else a repo-scoped token can't publish) then GoReleaser, producing
   `render-setup_<os>_<arch>` (darwin/linux × amd64/arm64) + `checksums.txt` as a **draft** release.
3. **Publish** the draft — drafts are *not* served by the `latest/download` redirect:
   ```bash
   gh release edit vX.Y.Z --draft=false --repo render-lab/render-install-wizard
   ```
   The repo must be **public** (one-time) so assets download anonymously. Verify:
   ```bash
   curl -fsSL -o /dev/null -w 'binary %{http_code}\n' \
     https://github.com/render-lab/render-install-wizard/releases/latest/download/render-setup_darwin_arm64
   curl -fsSL -o /dev/null -w 'sums   %{http_code}\n' \
     https://github.com/render-lab/render-install-wizard/releases/latest/download/checksums.txt
   ```

### 2. Ship the script on render.com

4. If `scripts/agents.sh` changed, re-vendor it into the website repo (`renderinc/website`) and
   open a PR — `app/agents.sh/route.ts` + a **byte-identical** `app/agents.sh/agents.sh`. Render
   builds a preview automatically.
5. Preview smoke — confirm serving, then run the full pipe across clean container images:
   ```bash
   curl -i https://<preview-host>/agents.sh                 # 200, text/x-shellscript, nosniff
   test/e2e/live_check.sh https://<preview-host>/agents.sh  # Docker matrix (glibc/musl; add PLATFORMS for arches)
   ```
6. Merge the website PR → production deploy.

### 3. Production smoke

7. Cloudflare fronts render.com, so use a cache-buster on the first hit:
   ```bash
   curl -fsSL "https://render.com/agents.sh?nocache=$(date +%s)" | sh
   test/e2e/live_check.sh https://render.com/agents.sh
   ```

> Tip for a fully contained local check without Docker: run with a throwaway home so nothing
> touches your machine — `SBX=$(mktemp -d); (export HOME=$SBX RENDER_HOME=$SBX/.render;
> curl -fsSL <url>/agents.sh | sh -s -- --dry-run --json); rm -rf "$SBX"`.

## Rollback

- **Bad bootstrap script**: revert the `agents.sh` change on `main`, re-sync the vendored copy, and
  redeploy the render.com route (Cloudflare serves the reverted script).
- **Bad binary release**: unpublish/delete the bad GitHub Release (so `latest` falls back to the
  previous one), or tell users to pin `RENDER_SETUP_VERSION=<previous tag>`.
