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
- Re-running it is a no-op (idempotent); `render-setup -r` removes the Render MCP entry cleanly.
- A no-TTY run (piped, CI) behaves as `-y` and prints a machine-readable summary.
- Checksums published and verified by the bootstrap before exec.
- render.com/agents.sh returns the byte-identical vendored script, Cloudflare-cached.

---

## Status

Phases 0–5 are implemented, reviewed, and committed. **Phase 6 (launch/cutover) is the only
remaining phase** and is mostly out-of-repo coordination (render.com PR, first release, analytics).

| Phase | Status | Commit |
|---|---|---|
| 0 — Foundations & contracts | ✅ done | `e22c00d` |
| 1 — Bootstrap, detection, configedit, content, TUI | ✅ done | `329420d` |
| 2 — Component & tool installers | ✅ done | `e93ff54` |
| 3 — Integration & non-interactive | ✅ done | `daf391e` |
| 4 — Release pipeline & checksums | ✅ done | `60e03a4` |
| 5 — E2E & OS matrix | ✅ done | `dd187c0` (+ `9655e2b` scoped uninstall) |
| 6 — Launch / cutover | ⏳ remaining | — |

Checkbox legend: `[x]` done · `[~]` partial or evolved from the original wording · `[ ]` not started.

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
- [x] `go build ./...` and `go vet ./...` pass.
- [x] `make lint test` runs green (gofmt+vet locally; golangci advisory in CI).
- [x] Every interface exists and is referenced by a stub; a shared `internal/ids` holds canonical IDs.
- [x] `manifest.schema.json` committed; `manifest.json` validates against it via a Go test.
- [x] CI runs on push (build + vet + gofmt + test + tidy gating; golangci advisory).

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
- [x] `sh -n` parse-check passes (local); `shellcheck` runs in CI.
- [x] All logic is nested in `main()` with `main "$@"` as the last line (truncation executes nothing).
- [x] Full local E2E (served over HTTP, Phases 4–5): downloads, **rejects a bad checksum**, accepts a
      good one, installs to `~/.render/bin`, and updates the correct rc file per shell.
- [x] Re-running is idempotent (marked PATH block, added once).
- [x] `sync-agents-sh.yml` diffs the two copies and fails on drift.
- [ ] Route handler returns `200 text/x-shellscript` in a live Next.js run — **deferred to Phase 6**
      (lands in the render.com frontend repo; the handler template ships in `deploy/render-com/`).

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
- [x] Detection unit-tested with fixture home dirs (injected home + fake PATH lookup); no false positives.
- [x] `platform` reports arch/OS; WSL detection factored into an injectable, table-tested helper.
- [x] PATH update is idempotent (marked block) and covers zsh/bash/fish; verified against temp rc files.
- [x] TTY detection via `golang.org/x/term`; drives the non-interactive fallback in Phase 3.

---

## Phase 1C — configedit (merge-not-clobber engine)

**Goal:** the hardest correctness surface, isolated and heavily tested.
**Depends on:** Phase 0. **Parallel with:** 1A, 1B, 1D, 1E.

Deliverables:
- `internal/configedit`: JSON/TOML readers+writers that merge Render's MCP/skills entries into a tool config **without** touching other servers/keys; preserve formatting/ordering where feasible; atomic writes with backup.

**Acceptance criteria:**
- [x] `testdata/fixtures/` holds real-world configs (existing MCP servers, comments, mixed content).
- [~] Round-trip test: add-then-remove returns the file to a **semantically** equal state. Byte-equality
      is NOT preserved (map-based engine reorders keys / drops TOML comments) — an accepted, documented
      Phase-1 trade-off; tests assert semantic equality.
- [x] Never deletes or reorders unrelated entries (asserted per fixture).
- [x] Atomic write (temp file in same dir + rename); never a truncated file.
- [x] Idempotent: applying the same edit twice yields byte-identical output.

---

## Phase 1D — Manifest + content subsystem

**Goal:** remote-driven matrix + Sanity-authored copy with offline fallback.
**Depends on:** Phase 0 (schema + content URL convention). **Parallel with:** 1A, 1B, 1C, 1E.

Deliverables:
- `internal/manifest`: parse/validate manifest, `?version=` pinning, fetch with timeout.
- `internal/content`: fetch `render.com/agents/*.md` (`Accept: text/markdown`), render to TUI/plain, `go:embed` fallback in `assets/content/`.
- `.github/workflows/refresh-content.yml`: snapshot live `.md` → `assets/content/`, open PR on change.

**Acceptance criteria:**
- [x] Manifest parse rejects unknown schema version and malformed entries with clear errors.
- [x] Content fetch has a timeout and falls back: live → embed → terse built-in (each failure unit-tested).
- [x] `Render` (glamour) for TUI, `RenderPlain` for `--json` (no ANSI). Embedded snapshot dir committed.
- [x] `refresh-content.yml` authored (scheduled snapshot → PR on change; runs in CI).
- [x] Content fetch is best-effort with fallback, so an offline run still prints next steps.
      NOTE: embeds live in `internal/content/embedded/` (go:embed can't reach `assets/content/`).

---

## Phase 1E — TUI skeleton

**Goal:** the interactive shell, buildable against stub interfaces.
**Depends on:** Phase 0. **Parallel with:** 1A–1D.

Deliverables:
- `internal/wizard`: bubbletea `model.go`, `picker.go` (single-axis WHAT picker, pre-checked), `detect_defaults.go`, `summary.go`, `styles.go` (ASCII splash GROW-2567, spinners). Surfaces "Detected: … — will configure all".
- `cmd/render-setup/main.go`: flag parse → wizard entry.

**Acceptance criteria:**
- [x] bubbletea model renders splash + detected-tools line + pre-checked WHAT picker; quits cleanly
      (Update transitions unit-tested without a real TTY).
- [x] One-Enter path selects all defaults (Railway-equivalent).
- [x] Degrades gracefully without a TTY (defers to the non-interactive path).
- [x] Entrypoint wired; in Phase 1E it collected/printed the selection, now fully wired in Phase 3.

---

## Phase 2 — Component & tool installers

**Goal:** actually install things and write correct per-tool config.
**Depends on:** 1B (detect/paths), 1C (configedit). **Parallel with:** 1D/1E finishing, and Phase 4.
Internally parallel: each component and each tool is independently ownable.

Deliverables:
- `internal/components/{cli,skills,mcp,plugins}` implementing `Installer`.
- `internal/tools/{claudecode,cursor,codex,opencode}` implementing `Target`, incl. plugin-vs-raw resolution and no-double-install dedup.

**Acceptance criteria:**
- [x] CLI component detects the CLI (PATH or `~/.render/bin`), installs via `brew`/official script,
      reports version via `render --version`. (Install delegates to the official installer, so the CLI
      may land in a brew prefix rather than `~/.render/bin`.)
- [x] Skills component delegates to the official installer (`render skills install` / `npx skills add`),
      which writes universal + per-tool dirs. MCP entries are credential-free (OAuth default).
- [x] **Revised after research:** all tools use the shell-automatable config-file (raw) path; the
      Cursor/Codex plugins are in-app and surfaced as next-steps (via `render.PluginFor`), not
      installed — so there is no raw-vs-plugin double-install to dedup.
- [x] Cursor, OpenCode, Claude, Codex: MCP entry merged via `configedit` (merge-not-clobber).
- [x] Each tool: `Configure` then `Unconfigure` returns config to a semantically equal state (tested).
- [x] `Status` reports installed/absent for every component and tool.

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
- [x] Skills install is non-interactive: the primary path is `npx skills add render-oss/skills --all -g`
      (`--all` = all skills to all detected agents without prompts; `-g` = global). Render CLI
      (`render skills install`) is the fallback, with child stdin unset (→ /dev/null) as a belt-and-braces
      against prompts.
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
- **Reconcile the download URL scheme with `scripts/agents.sh`** — ✅ RESOLVED (see Decisions below):
  binaries are on GitHub Releases with version-less asset names; the bootstrap and `internal/paths`
  both use `latest`→redirect / pinned→tag URLs.

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
- `test/e2e/harness.sh`: seeds realistic per-tool configs (each with an unrelated MCP server +
  unrelated keys), installs the Render MCP into all tools, and asserts correctness, merge-not-clobber,
  idempotency, and clean uninstall. Hermetic HOME; deterministic (no network).
- `test/e2e/detect_check.sh`: asserts the wizard detects the installed agents (`--dry-run --json`).
- `test/e2e/bootstrap_check.sh`: serves a snapshot build and runs the full `curl | sh` path.
- `test/e2e/Dockerfile`: realistic Linux env that installs *real* agents (Claude Code via npm,
  OpenCode via install.sh) and seeds the opaque ones (Cursor, Codex), then runs detection + harness.
- `.github/workflows/e2e.yml`: OS matrix (Linux amd64/arm64, macOS arm64/amd64) running the harness,
  the bootstrap path, and the realistic Docker env.

**Acceptance criteria:**
- [x] Config E2E passes on macOS arm64 (local) and Linux arm64 (Docker, real agents); CI matrix
      extends to macOS amd64 + Linux amd64. (WSL: covered by the Linux build; no hosted WSL CI runner.)
- [x] Second run is a verified no-op — the harness asserts byte-identical configs (sha256) on re-run.
- [x] `render-setup -r` is scoped: it removes the Render MCP entry from every target tool (leaving
      unrelated servers/keys intact, asserted per tool) and intentionally does NOT remove the CLI or
      skills (avoids a misleading half-uninstall; stated in help text + summary).
- [x] Native Windows path: the bootstrap prints the WinGet message and exits 0 (non-destructive).
- [x] Failure injection never leaves a half-configured install: `test/e2e/failure_check.sh` proves the
      bootstrap aborts (exit 1, nothing installed) on a missing binary asset (network error) and on a
      corrupt/partial download (checksum mismatch). Wired into `e2e.yml`.
- [x] Realistic detection verified: with real Claude Code + OpenCode installed (Docker), the wizard
      detects all four agents.

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

- [x] No `sudo`; all writes under `~/.render` or tool config dirs.
- [x] Idempotent and re-runnable; merges never clobber other MCP servers/skills.
- [x] Installer never writes credentials (OAuth default; API-key fallback is an env-ref, never stored).
- [x] `--json` available wherever there's output; non-interactive == first-class.
- [x] Every downloaded binary checksum-verified before use (bootstrap; verified in E2E).

## Open questions

1. **Open** — Skills freshness: background auto-update vs re-run setup (skills component / CLI behavior).
2. ✅ **Resolved** — Plugin/skills dedup: plugins are in-app next-steps, not installed by the wizard, so
   there's no raw-vs-plugin dedup; all tools take the config-file path.
3. **Open (frontend-owned)** — Sanity schema: a structured `nextSteps` field vs. full-page markdown.
   Today the wizard builds next-steps locally (`render.PluginFor` + static lines) rather than from the
   per-tool `.md`; wiring the wizard to consume Sanity next-steps is deferred.

## Residuals

- ✅ Non-interactive skills flags — resolved (`npx skills add render-oss/skills --all -g`).
- ✅ Failure-injection E2E — resolved (`test/e2e/failure_check.sh`, wired into `e2e.yml`).
- ✅ `LICENSE` — resolved (Apache-2.0).
- **Open** — a full clean-machine install (incl. CLI + skills *network* installs, non-dry-run) is best
  validated in the CI matrix; local validation covered MCP config + the bootstrap flow.
- **Open (frontend-owned)** — wire the wizard's next-steps to Sanity `/agents/*.md` (vs. local copy).
