# Render Install Wizard — Workback Plan

Working backward from the launch goal to the milestones required to get there.
See `spec.md` for architecture and `project.md` for original intent.

## Launch goal (definition of done)

A user (or their agent) runs:

```bash
curl -fsSL render.com/agents.sh | sh
```

and lands in a TUI wizard that detects their installed agents, installs the chosen
components (CLI / skills / MCP), configures **all detected tools** correctly (plugin vs raw,
merge-not-clobber), and prints Sanity-authored next steps. The same flow works non-interactively
(`-y`, `--json`), is idempotent, uninstalls cleanly (`-r`), and runs on macOS + Linux + WSL.
`render.com/agents.sh` serves the checksum-verified bootstrap; every binary is checksummed.

Launch is met when **all** of these hold:
- `curl -fsSL render.com/agents.sh | sh` completes a full install on a clean macOS arm64 machine.
- Re-running it is a no-op (idempotent); `render-setup -r` fully reverses it.
- A no-TTY run (piped, CI) behaves as `-y` and prints a machine-readable summary.
- Checksums published and verified by the bootstrap before exec.
- render.com/agents.sh returns the byte-identical vendored script, Cloudflare-cached.

---

## Phase map & parallelism

```
Phase 0  Foundations & Contracts        ── critical path, blocks everything
              │
   ┌──────────┼───────────┬───────────┬───────────────┐
   ▼          ▼           ▼           ▼               ▼
Phase 1A   Phase 1B    Phase 1C    Phase 1D        Phase 1E
bootstrap  detect +    configedit  manifest +      TUI skeleton
+ deploy   paths       engine      content         (stub ifaces)
   │          │           │           │               │
   │          └─────┬─────┘           │               │
   │                ▼                 │               │
   │           Phase 2                │               │
   │           components + tools     │               │
   │           installers             │               │
   │                └─────────┬───────┴───────────────┘
   │                          ▼
   │                     Phase 3
   │                     integration + non-interactive
   │                          │
   └──────────┬───────────────┤
              ▼               ▼
          Phase 4         (feeds)
          release +
          checksums
              │
              ▼
          Phase 5
          E2E / idempotency / uninstall / OS matrix
              │
              ▼
          Phase 6
          launch / cutover
```

Parallelizable: **1A–1E** all run concurrently after Phase 0. Within Phase 2, each
component and each tool is independently ownable. **Phase 4** can start as soon as the wizard
compiles (end of Phase 1), in parallel with Phases 2–3.

Critical path: **0 → 1B/1C → 2 → 3 → 5 → 6** (plus 4 feeding 5).

---

## Phase 0 — Foundations & Contracts

**Goal:** lock the interfaces and conventions so the parallel tracks can't collide.
**Depends on:** nothing. **Blocks:** all.

Deliverables:
- Repo scaffold: `go.mod`, directory tree from `spec.md`, `Makefile`, `.golangci.yml`, skeleton `.github/workflows/ci.yml`.
- Frozen contracts (compile-only stubs, documented):
  - `components.Installer` interface: `Detect`, `Install`, `Uninstall`, `Status`, `ID`.
  - `tools.Target` interface: `Detect`, `Configure`, `Unconfigure`, `PreferredDelivery` (plugin|raw).
  - `manifest` schema v1 (JSON) with `components[]`, `tools[]`, and `contentURL` fields.
  - Artifact naming: `render-setup_<version>_<os>_<arch>[.exe]`; checksums file `checksums.txt` (sha256), served separately.
  - Install layout: binary → `~/.render/bin/render-setup`; state/config → `~/.render/`.
  - Flag surface: `-y`, `--components`, `--agent` (repeatable), `--no-login`, `--json`, `-r/--uninstall`, `--version`.
  - Content URL convention: `render.com/agents/<tool>.md`, `render.com/agents.md`.

**Acceptance criteria:**
- [ ] `go build ./...` and `go vet ./...` pass on the empty scaffold.
- [ ] `make lint test` runs green (no-op tests allowed).
- [ ] Every interface above exists and is referenced by at least one stub, so signatures are frozen.
- [ ] `manifest.schema.json` committed; a sample `manifest.json` validates against it in CI.
- [ ] CI runs on push (build + vet + lint) and passes.

---

## Phase 1A — Bootstrap script + render.com serving integration

**Goal:** the `curl | sh` entry point and the one route render.com must add.
**Depends on:** Phase 0 (artifact naming + install layout + checksum contract). **Parallel with:** 1B–1E.

Deliverables:
- `scripts/agents.sh`: `set -eu`, `main()`-wrapper (no partial exec on truncated download), OS/arch detect, download `render-setup` + `checksums.txt`, **sha256 verify**, install to `~/.render/bin`, PATH update (zsh/bash/fish), re-attach `/dev/tty`, `exec` wizard forwarding all flags. Native-Windows → graceful "use WinGet" message.
- `deploy/render-com/route-handler.ts`: Next.js route `GET /agents.sh` → `text/x-shellscript`, edge/CDN-cached, serving the vendored copy.
- `deploy/render-com/agents.sh` (vendored, byte-identical) + `deploy/render-com/README.md`.
- `.github/workflows/sync-agents-sh.yml`: fails on drift between the two copies.

**Acceptance criteria:**
- [ ] `shellcheck scripts/agents.sh` passes; `sh -n` parse-check passes.
- [ ] Truncating the script mid-download executes nothing (main()-wrapper verified by test).
- [ ] Against a local mock server, the script downloads, **rejects a bad checksum**, accepts a good one, installs to `~/.render/bin`, and updates the correct rc file per shell.
- [ ] Re-running is idempotent (no duplicate PATH lines).
- [ ] `sync-agents-sh.yml` goes red when the vendored copy drifts, green when identical.
- [ ] Route handler returns `200 text/x-shellscript` with the script body in a local Next.js run.

---

## Phase 1B — Detection + paths + shell PATH

**Goal:** know the environment and own `~/.render` + PATH edits.
**Depends on:** Phase 0. **Parallel with:** 1A, 1C, 1D, 1E.

Deliverables:
- `internal/detect/platform.go`: OS, arch, WSL, TTY presence.
- `internal/detect/tools.go`: detect Claude Code, Cursor, Codex, OpenCode (config-path + binary probes).
- `internal/paths`: `~/.render` layout helpers; idempotent PATH updates for zsh/bash/fish.
- `internal/logx`: leveled logging + `--json` sink.

**Acceptance criteria:**
- [ ] Detection unit-tested with fixture home dirs: each tool detected iff its marker present; no false positives on empty home.
- [ ] `platform` correctly reports arch/OS across a table test incl. a simulated WSL marker.
- [ ] PATH update is idempotent (second run adds nothing) and covers zsh/bash/fish; verified against fixture rc files.
- [ ] TTY detection returns false under a pipe (drives non-interactive fallback later).

---

## Phase 1C — configedit (merge-not-clobber engine)

**Goal:** the hardest correctness surface, isolated and heavily tested.
**Depends on:** Phase 0. **Parallel with:** 1A, 1B, 1D, 1E.

Deliverables:
- `internal/configedit`: JSON/TOML readers+writers that merge Render's MCP/skills entries into a tool config **without** touching other servers/keys; preserve formatting/ordering where feasible; atomic writes with backup.

**Acceptance criteria:**
- [ ] `testdata/fixtures/` holds real-world configs (existing MCP servers, comments, mixed content) per tool.
- [ ] Round-trip test: adding Render then removing it returns the file to byte-equal original (modulo documented normalization).
- [ ] Never deletes or reorders unrelated entries (asserted per fixture).
- [ ] Concurrent/interrupted write leaves either old or new file intact (atomic), never a truncated file.
- [ ] Idempotent: applying the same edit twice yields identical output.

---

## Phase 1D — Manifest + content subsystem

**Goal:** remote-driven matrix + Sanity-authored copy with offline fallback.
**Depends on:** Phase 0 (schema + content URL convention). **Parallel with:** 1A, 1B, 1C, 1E.

Deliverables:
- `internal/manifest`: parse/validate manifest, `?version=` pinning, fetch with timeout.
- `internal/content`: fetch `render.com/agents/*.md` (`Accept: text/markdown`), render to TUI/plain, `go:embed` fallback in `assets/content/`.
- `.github/workflows/refresh-content.yml`: snapshot live `.md` → `assets/content/`, open PR on change.

**Acceptance criteria:**
- [ ] Manifest parse rejects unknown schema version and malformed entries with clear errors.
- [ ] Content fetch honors a short timeout and falls back: live → embed → terse built-in (unit-tested by forcing each failure).
- [ ] Rendered markdown is readable in TUI and clean in `--json` (no ANSI in JSON).
- [ ] `refresh-content.yml` produces a diff PR when the fixture live copy changes; no-op when unchanged.
- [ ] Wizard never blocks > timeout on content; offline run still prints next steps.

---

## Phase 1E — TUI skeleton

**Goal:** the interactive shell, buildable against stub interfaces.
**Depends on:** Phase 0. **Parallel with:** 1A–1D.

Deliverables:
- `internal/wizard`: bubbletea `model.go`, `picker.go` (single-axis WHAT picker, pre-checked), `detect_defaults.go`, `summary.go`, `styles.go` (ASCII splash GROW-2567, spinners). Surfaces "Detected: … — will configure all".
- `cmd/render-setup/main.go`: flag parse → wizard entry.

**Acceptance criteria:**
- [ ] `render-setup` launches, renders splash + detected-tools line + pre-checked WHAT picker, and exits cleanly on quit.
- [ ] One-Enter path selects all defaults (Railway-equivalent).
- [ ] Renders correctly at 80-col width; degrades gracefully without a TTY (defers to non-interactive path, Phase 3).
- [ ] Wired to stubs only — no real installs yet — behind an `--dry-run` that prints the plan.

---

## Phase 2 — Component & tool installers

**Goal:** actually install things and write correct per-tool config.
**Depends on:** 1B (detect/paths), 1C (configedit). **Parallel with:** 1D/1E finishing, and Phase 4.
Internally parallel: each component and each tool is independently ownable.

Deliverables:
- `internal/components/{cli,skills,mcp,plugins}` implementing `Installer`.
- `internal/tools/{claudecode,cursor,codex,opencode}` implementing `Target`, incl. plugin-vs-raw resolution and no-double-install dedup.

**Acceptance criteria:**
- [ ] CLI component installs the Render CLI to `~/.render/bin` and reports version; uninstall removes it.
- [ ] Skills component writes universal `.agents` skills + tool-specific dirs; MCP component writes hosted OAuth entry (no credentials written).
- [ ] For Claude Code & Codex, wizard picks the **plugin** path and never also writes raw skills+mcp (dedup asserted).
- [ ] For Cursor & OpenCode, raw config path used; entries merged via `configedit` (Phase 1C guarantees).
- [ ] Each tool: `Configure` then `Unconfigure` returns config to original (per fixture).
- [ ] `Status` accurately reports installed/absent for every component and tool.

---

## Phase 3 — Integration & non-interactive mode

**Goal:** wire TUI → components/tools/manifest/content; make agents-first non-interactive real.
**Depends on:** 1D, 1E, 2. **Blocks:** 5.

Deliverables:
- Orchestrator connecting picker selection → detected tools → installers → summary.
- `internal/cliflags` fully wired: `-y`, `--components`, `--agent`, `--no-login`, `--json`, `-r`.
- No-TTY auto-`-y` with printed summary; end-state summary table + Sanity next steps.

**Acceptance criteria:**
- [~] Interactive/real-install happy path is wired (picker → orchestrator → installers); verified via
      dry-run + unit tests. Real mutation not exercised on the dev machine by design.
- [x] No-TTY / `-y` defaults to all components and prints a summary; exit 0 (dry-run verified).
- [x] `--json` emits a single valid documented object (parseable by `jq`), no ANSI.
- [x] `--components cli,skills` scopes components; `--agent cursor` scopes tools.
- [x] `--no-login` drops the login next step; it's otherwise present and clearly marked.
- [x] Summary lists per-tool result (e.g. "[configured] cursor"). Plugins are surfaced in the
      Next steps section (via `internal/render.PluginFor`), not as a delivery mode in the summary.
- [x] Unknown manifest tool/component IDs (no compiled handler) are skipped with a recorded
      "skipped" result, never fatal (must-ignore-unknown; verified in orchestrator tests + smoke).
- [x] Skills install won't hang in `-y`/no-TTY mode: the orchestrator's command runner leaves child
      stdin unset (→ /dev/null), so a prompting installer gets EOF instead of blocking. RESIDUAL:
      confirm the exact non-interactive flags for `render skills install` / `npx skills add` so it
      selects all skills rather than erroring on EOF.
- [x] Retired the vestigial `internal/components/plugins` stub (plugins are next-step copy via
      `internal/render.PluginFor`, not a component).

---

## Phase 4 — Release pipeline & checksums

**Goal:** publish verifiable cross-platform binaries the bootstrap can trust.
**Depends on:** wizard compiles (end of Phase 1). **Parallel with:** Phases 2–3.

Deliverables:
- `.goreleaser.yaml`: build matrix (darwin/linux × arm64/amd64), archive naming per Phase 0 contract.
- `.github/workflows/release.yml`: tag-triggered build → artifacts + `checksums.txt` served separately.
- `?version=` pinning support validated against real artifacts.
- **Reconcile the download URL scheme with `scripts/agents.sh`.** The bootstrap currently assumes
  `${BASE}/agents/download/<version>/<artifact>` + `/checksums.txt`, and a literal `latest` version
  token. Phase 4 must either make the release hosting serve that layout (incl. a `latest` alias that
  resolves to the newest artifact + checksums) or update the script to match the real layout. This is
  a hard dependency for the end-to-end `curl | sh` path.

**Decisions (locked in):**
- **Binaries are hosted on GitHub Releases** (`github.com/render-oss/render-install-wizard/releases`),
  published by GoReleaser. `render.com/agents.sh` still serves the *script*; only binaries live on
  GitHub. (render.com could later proxy binary downloads for install analytics — future option.)
- **Version-less asset names** (`render-setup_<os>_<arch>`, raw binaries via GoReleaser
  `formats: [binary]`). This makes the `latest` URL stable: `latest` → GitHub's
  `/releases/latest/download/<asset>` redirect; a pinned `RENDER_SETUP_VERSION=vX.Y.Z` →
  `/releases/download/vX.Y.Z/<asset>`. The URL scheme is centralized in `internal/paths`
  (`DownloadURL`/`ChecksumsURL`) and mirrored by `scripts/agents.sh`.
- `render-setup --version` is stamped via `-ldflags -X main.version={{.Version}}`.

**Acceptance criteria:**
- [x] A snapshot release produced all four OS/arch artifacts + `checksums.txt` (correct version-less
      names). A tagged release does the same via `release.yml`.
- [x] Checksums match artifacts; a full local bootstrap E2E (served over HTTP) downloaded,
      **verified sha256, installed, updated PATH, and exec'd** the wizard; a tampered binary was
      correctly rejected (exit 1, not installed).
- [x] `render-setup --version` reports the injected version (verified on the built snapshot binary).
- [x] Pinned `RENDER_SETUP_VERSION` downloads the tagged artifact; `latest` uses the GitHub redirect.
- [x] Bug fixed during E2E: the bootstrap now probes that `/dev/tty` is *openable* (not just present)
      before redirecting, so it no longer crashes in no-TTY environments (CI/agents/containers).

---

## Phase 5 — E2E, idempotency, uninstall, OS matrix

**Goal:** prove the whole thing on real environments.
**Depends on:** 3 + 4.

Deliverables:
- `test/e2e`: containerized/VM runs of the full `curl | sh` path per OS/arch.
- Idempotency + uninstall reversibility suites.

**Acceptance criteria:**
- [ ] Clean-machine E2E passes on macOS arm64, macOS amd64, Linux arm64, Linux amd64, WSL.
- [ ] Second run is a verified no-op (no duplicate config/PATH entries; diff-clean).
- [ ] `render-setup -r` removes binary, PATH edits, skills, and tool config entries — leaving unrelated config untouched (asserted).
- [ ] Native Windows shows the graceful WinGet message and exits non-destructively.
- [ ] Failure injection (network down, partial download, bad checksum) never leaves a half-configured tool.

---

## Phase 6 — Launch / cutover

**Goal:** flip it on for the world.
**Depends on:** 5.

Deliverables:
- render.com frontend PR merged: `/agents.sh` live and Cloudflare-cached.
- First public release tagged; checksums published; README "read it before you pipe it" finalized.
- Install analytics wired (count, OS/arch, referrer) per project.md.

**Acceptance criteria:**
- [ ] `curl -fsSL render.com/agents.sh | sh` works from a clean machine, end to end, in production.
- [ ] `render.com/agents.sh` returns byte-identical vendored script; `sync-agents-sh.yml` green on `main`.
- [ ] Rollback path documented (revert route handler / pin previous version).
- [ ] Analytics dashboard shows installs by OS/arch.

---

## Cross-cutting acceptance (every phase)

- [ ] No `sudo`; all writes under `~/.render` or tool config dirs.
- [ ] Idempotent and re-runnable; merges never clobber other MCP servers/skills.
- [ ] Installer never writes credentials (MCP OAuth is first-touch, lazy).
- [ ] `--json` available wherever there's output; non-interactive == first-class.
- [ ] Every downloaded binary checksum-verified before use.

## Open questions to resolve before/within phases (from project.md + spec.md)

1. Skills freshness: background auto-update vs re-run setup (affects Phase 2 skills component).
2. Plugin/skills dedup rules per tool need finalizing before Phase 2 tool work is "correct".
3. Sanity schema: do the `/agents/*.md` pages need a structured `nextSteps` field for clean TUI
   rendering, vs. scraping full-page markdown? (Frontend-owned; affects Phase 1D content parsing.)
