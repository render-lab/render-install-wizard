# Windows Support

**Status: Not started.**

Today the `curl -fsSL render.com/agents.sh | sh` path supports macOS, Linux, and WSL only. WSL
already works via the `sh` path (it *is* Linux). Native Windows is currently expected to be covered
by the in-flight WinGet manifest.

This doc scopes what it would take to add a **native Windows install path**.

## Framing: it's a parallel path, not a tweak

`curl | sh` cannot run on bare Windows — there's no POSIX `sh` in `cmd.exe` or PowerShell. So
"Windows support" means shipping the Windows-native equivalent idiom, a PowerShell one-liner:

```powershell
irm render.com/agents.ps1 | iex
```

The wizard binary itself is already close: it **cross-compiles clean for `windows/amd64` and
`windows/arm64`** with no `/dev/tty` use or unix syscalls in the Go code. The gating work is in the
shell/bootstrap layer and a handful of OS-specific paths, not the core logic.

## Work breakdown

- [ ] **PowerShell bootstrap** — `scripts/agents.ps1` (parallel to `scripts/agents.sh`) plus a served
      route (`render.com/agents.ps1`) and the vendored copy in `deploy/render-com/`. It must:
      detect arch → download `render-setup_windows_<arch>.exe` → verify SHA-256 against `checksums.txt`
      → install to `%USERPROFILE%\.render\bin` → update the user PATH → exec the wizard, forwarding
      args. Mirror the `agents.sh` guarantees (fail-closed on truncated download, no admin elevation).
      *(Medium — the main new artifact. Also add a `sync-agents-ps1` check like the sh one.)*
- [ ] **Release matrix** — add `windows` (amd64 + arm64) to `goos` in `.goreleaser.yaml`.
      `paths.ArtifactName` already appends `.exe`, and the URL scheme is unchanged. *(Trivial.)*
- [ ] **Windows PATH updater** — `internal/paths/shellpath.go` only handles zsh/bash/fish. Windows
      needs the user `PATH` updated via `setx`/the registry or the PowerShell `$PROFILE`. Simplest is
      to let the PS bootstrap own PATH (as `agents.sh` does today) and keep the Go side out of it.
      *(Small.)*
- [ ] **Per-tool config paths + detection** — `internal/render.MCPConfigPath` and `internal/detect`
      assume `~/`-relative unix locations. Verify each tool's real Windows config location and branch
      by `GOOS` where needed:
      - Claude Code (`~/.claude.json`), Cursor (`~/.cursor/mcp.json`), Codex (`~/.codex/config.toml`)
        are likely home-relative → `%USERPROFILE%\...`, which `filepath.Join(home, …)` already yields.
      - OpenCode (`~/.config/opencode/opencode.json`) may live under `%APPDATA%` on Windows — verify.
      - Detection markers/binary names differ (`.exe`/`.cmd`; `exec.LookPath` handles PATHEXT).
      *(Moderate — the real work and risk; needs per-tool verification against real installs.)*
- [ ] **CI / E2E** — add a `windows-latest` runner. The bash harness (`test/e2e/harness.sh`) won't run
      natively; either port it to PowerShell (`pwsh`) or run it under Git Bash. The realistic Docker
      env stays Linux-only (Windows containers are heavy). *(Moderate.)*

## Open decision

Is a Windows PowerShell one-liner worth it **over WinGet**? WinGet (`winget install …`) is the cleaner
native-install UX for humans; the PS path mainly helps agents/CI that want a scriptable one-liner.
Recommend deciding this before investing in the per-tool Windows config/detection work (item 4),
since that's where most of the effort and risk lives.

## Suggested order (if we proceed)

1. Add `windows` to the GoReleaser matrix (trivial; starts publishing Windows binaries that WinGet
   can also consume).
2. Verify + branch the per-tool config paths and detection for `GOOS == "windows"`.
3. Write `scripts/agents.ps1` + the render.com route + a sync check.
4. Add the `windows-latest` E2E lane.
