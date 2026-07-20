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

## Windows support (a parallel PowerShell path)

Today the curl path is macOS/Linux/WSL only; WSL already works via the `sh` path (it's Linux), and
native Windows is covered by the in-flight WinGet manifest.

Adding a *native* Windows one-liner is feasible but it's a **parallel path, not a tweak**:
`curl | sh` can't run on bare Windows (no POSIX `sh`), so the Windows-native equivalent is a
PowerShell one-liner — `irm render.com/agents.ps1 | iex`.

Good news: the wizard binary already cross-compiles clean for `windows/amd64` and `windows/arm64`
(no `/dev/tty` or unix syscalls in the Go code). What's blocking, by effort:

- [ ] **PowerShell bootstrap** `scripts/agents.ps1` + a served route (`render.com/agents.ps1`): detect
      arch → download `render-setup_windows_<arch>.exe` → verify SHA-256 → install to
      `%USERPROFILE%\.render\bin` → update user PATH → exec. *(Medium — the main new artifact.)*
- [ ] **Release matrix**: add `windows` (amd64 + arm64) to `.goreleaser.yaml` goos; `ArtifactName`
      already appends `.exe`. *(Trivial.)*
- [ ] **Windows PATH**: `internal/paths/shellpath.go` only handles zsh/bash/fish; Windows needs user
      PATH via `setx`/registry or the PowerShell profile (or let the PS bootstrap own it). *(Small.)*
- [ ] **Per-tool config paths + detection**: `render.MCPConfigPath` and `internal/detect` assume
      `~/`-relative unix locations. Verify each tool's Windows location (Claude/Cursor/Codex are
      likely home-relative → `%USERPROFILE%`; OpenCode may use `%APPDATA%`) and branch by `GOOS`.
      *(Moderate — the real work/risk.)*
- [ ] **CI/E2E**: add a `windows-latest` runner; the bash harness needs a pwsh port or Git Bash (the
      realistic Docker env stays Linux-only). *(Moderate.)*

**Open decision:** is a Windows PS one-liner worth it over WinGet? WinGet is the cleaner native
install for humans; the PS path mainly helps agents/CI that want a scriptable one-liner. Worth
deciding before investing in the per-tool Windows config/detection work (item 4).

## Platform note

- WSL is exercised via the Linux build (no hosted WSL CI runner).
