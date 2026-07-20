# Future Work

Not-started and forward-looking work. None of this blocks launch (see [`RELEASE.md`](RELEASE.md));
architecture and current state are in [`SPEC.md`](SPEC.md).

## Windows support (a parallel PowerShell path) — not started

Today the curl path supports macOS, Linux, and WSL. WSL already works via the `sh` path (it *is*
Linux); native Windows is expected to be covered by the in-flight WinGet manifest.

Adding a *native* Windows one-liner is a **parallel path, not a tweak**: `curl | sh` can't run on
bare Windows (no POSIX `sh`), so the Windows-native equivalent is a PowerShell one-liner —
`irm render.com/agents.ps1 | iex`.

Good news: the wizard binary already cross-compiles clean for `windows/amd64` and `windows/arm64`
(no `/dev/tty` or unix syscalls in the Go code). What's blocking, by effort:

- [ ] **PowerShell bootstrap** — `scripts/agents.ps1` (parallel to `agents.sh`) + a served route
      (`render.com/agents.ps1`) + a vendored copy in `deploy/render-com/`. Detect arch → download
      `render-setup_windows_<arch>.exe` → verify SHA-256 → install to `%USERPROFILE%\.render\bin` →
      update user PATH → exec. Mirror the `agents.sh` guarantees; add a sync check. *(Medium — the
      main new artifact.)*
- [ ] **Release matrix** — add `windows` (amd64 + arm64) to `.goreleaser.yaml` goos; `ArtifactName`
      already appends `.exe`. *(Trivial.)*
- [ ] **Windows PATH updater** — `internal/paths/shellpath.go` only handles zsh/bash/fish; Windows
      needs user PATH via `setx`/registry or the PowerShell `$PROFILE` (or let the PS bootstrap own
      it, as `agents.sh` does today). *(Small.)*
- [ ] **Per-tool config paths + detection** — `internal/render.MCPConfigPath` and `internal/detect`
      assume `~/`-relative unix locations. Verify each tool's Windows location (Claude/Cursor/Codex
      likely home-relative → `%USERPROFILE%`; OpenCode may use `%APPDATA%`) and branch by `GOOS`.
      *(Moderate — the real work/risk; needs per-tool verification against real installs.)*
- [ ] **CI / E2E** — add a `windows-latest` runner; the bash harness needs a pwsh port or Git Bash
      (the realistic Docker env stays Linux-only). *(Moderate.)*

**Open decision:** is a Windows PowerShell one-liner worth it over WinGet? WinGet is the cleaner
native-install UX for humans; the PS path mainly helps agents/CI that want a scriptable one-liner.
Decide this before investing in the per-tool Windows config/detection work (item 4).

**Suggested order (if we proceed):** (1) add `windows` to the GoReleaser matrix; (2) verify + branch
per-tool config paths/detection for `GOOS == "windows"`; (3) write `agents.ps1` + route + sync check;
(4) add the `windows-latest` E2E lane.

## Optional enhancements

- **Wizard next-steps from Sanity** — next-steps are currently built locally (`render.PluginFor` +
  static lines). Optionally fetch per-tool copy from `render.com/agents/*.md` so the wizard stays in
  lockstep with the website. (May want a structured `nextSteps` field — see Open questions.)
- **Skills freshness** — decide background auto-update (as Railway does) vs. re-running setup.

## Open questions

1. **Skills freshness** — background auto-update vs. re-run setup (skills component / CLI behavior).
2. **Sanity schema (frontend-owned)** — do the `/agents/*.md` pages need a structured `nextSteps`
   field for clean TUI rendering, vs. scraping full-page markdown?
