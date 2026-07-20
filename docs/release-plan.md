# Release Plan

What's left to ship `curl -fsSL render.com/agents.sh | sh`. Phases 0–5 are complete — the wizard is
built, integrated, releasable, and E2E-tested. The step-by-step runbook and rollback live in
[`RELEASE.md`](RELEASE.md); this is the punch list of what still needs doing.

## Blocking — required to launch

- [ ] **render.com `/agents.sh` route** — merge the handler in the render.com frontend repo using the
      template in [`../deploy/render-com/`](../deploy/render-com/). Serves the vendored `agents.sh` as
      `text/x-shellscript`, Cloudflare-cached; kept byte-identical to `scripts/agents.sh` by
      `sync-agents-sh.yml`.
- [ ] **First release** — push a `vX.Y.Z` tag; GoReleaser publishes the darwin/linux × amd64/arm64
      binaries + `checksums.txt` to GitHub Releases as a draft; review and publish. `latest` then
      resolves via GitHub's redirect.
- [ ] **Clean-machine end-to-end** — verify `curl -fsSL render.com/agents.sh | sh` installs and runs
      in production on a fresh macOS and Linux box.

## Decide before / at launch

- [ ] **Binary hosting + install analytics** — keep binaries on GitHub Releases (current) or have
      render.com proxy downloads to capture count / OS-arch / referrer metrics, then wire analytics.
      Analytics can follow launch; the hosting decision shouldn't.
- [ ] **Rollback confirmed** — revert route / unpublish release / pin previous version
      (see [`RELEASE.md`](RELEASE.md)).

## Already handled (no action needed)

Apache-2.0 LICENSE · checksum-verified bootstrap + failure-injection tested · non-interactive skills
install · download URL scheme reconciled (GitHub Releases, `latest` redirect) · scoped uninstall.
Details in [`plan.md`](plan.md).

## Out of scope for this launch

- Native **Windows** curl/PowerShell path — [`windows-support.md`](windows-support.md) (not started).
- Optional enhancements (Sanity-sourced next-steps, skills freshness) — tracked in [`plan.md`](plan.md).
