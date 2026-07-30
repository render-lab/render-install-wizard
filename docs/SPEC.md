# Render Install Wizard — Spec

Single source of truth for **why** this exists, **how** it's built, and **where** it stands.
(Consolidates the former `project.md`, `spec.md`, and `plan.md`.) Release/cutover steps live in
[`RELEASE.md`](RELEASE.md); not-started and future work in [`FUTURE.md`](FUTURE.md).

---

## Problem

There is no unified entry point for installing the Render CLI, MCP, and Skills — they require
separate steps across multiple docs. Railway and others provide a butter-smooth single-command
setup:

```text
curl -fsSL agents.railway.com | sh
Setting up Railway for agents
✓ CLI — v5.26.1 · ~/.railway/bin
✓ Shell PATH — ~/.zshrc updated
✓ Agent skills — Universal (.agents), Claude Code, Cursor
✓ Railway MCP (local) — Claude Code, Cursor
```

## Solution

Ship `curl -fsSL render.com/agents.sh | sh` — a thin bootstrap that downloads a standalone setup
wizard (its own binary, independent of the CLI) that configures the chosen components (CLI, skills,
MCP) across the user's coding agents (Claude Code, Cursor, Codex, OpenCode).

### Launch goal (definition of done)

`curl -fsSL render.com/agents.sh | sh` detects installed agents, installs the chosen components,
configures **all detected tools** (merge-not-clobber), and prints next steps. The same flow works
non-interactively (`-y`, `--json`), is idempotent, uninstalls the MCP entry cleanly (`-r`), and runs
on macOS + Linux + WSL. `render.com/agents.sh` serves the checksum-verified bootstrap; every binary
is checksum-verified before exec.

---

## Status

Phases 0–5 are implemented, reviewed, and committed. **Phase 6 (launch/cutover) is the only
remaining phase** and is mostly out-of-repo coordination — see [`RELEASE.md`](RELEASE.md).

| Phase | Status | Commit |
|---|---|---|
| 0 — Foundations & contracts | ✅ done | `e22c00d` |
| 1 — Bootstrap, detection, configedit, content, TUI | ✅ done | `329420d` |
| 2 — Component & tool installers | ✅ done | `e93ff54` |
| 3 — Integration & non-interactive | ✅ done | `daf391e` |
| 4 — Release pipeline & checksums | ✅ done | `60e03a4` |
| 5 — E2E & OS matrix | ✅ done | `dd187c0` (+ `9655e2b` scoped uninstall) |
| 6 — Launch / cutover | ⏳ remaining | — |

---

## Architecture

### Thin bootstrap → standalone wizard

The shell script only: detects OS/arch → downloads the wizard binary (checksum-verified) into a
scratch dir → runs it (forwarding flags) → deletes it. It's ephemeral: it installs nothing of its
own and leaves no wizard binary or PATH edit behind — the wizard itself installs the Render CLI to
`~/.render/bin` and updates PATH (zsh/bash/fish). All logic is wrapped in a `main()` invoked on the
last line, so a truncated download executes nothing.

Everything interactive lives in the standalone Go wizard binary (`render-setup`), shipped and
versioned independently of the CLI:

- Go gives a real TUI (bubbletea) instead of fragile POSIX `read` loops.
- **TTY problem solved:** when piped to `sh`, stdin *is* the script, so naive prompts hang. The
  bootstrap re-attaches stdin to `/dev/tty` when one is actually openable (probed, not just present).
- Homebrew/WinGet/npm users get the identical wizard by running `render-setup` directly — one code
  path, many channels. The CLI is just another component the wizard installs, not the wizard's host.
- Idempotent and re-runnable.

### Project structure

```
render-install-wizard/
├── README.md                      # "read it before you pipe it" — auditable entry point
├── LICENSE                        # Apache-2.0
├── docs/                          # SPEC.md (this), RELEASE.md, FUTURE.md
├── go.mod / go.sum
├── Makefile                       # build, test, lint, release shortcuts
├── .goreleaser.yaml               # cross-compile matrix + checksum publishing
│
├── scripts/agents.sh              # THE thin bootstrap (source of render.com/agents.sh)
├── cmd/render-setup/main.go       # wizard entrypoint (flags → detect → select → orchestrate)
│
├── internal/
│   ├── ids/                       # canonical component/tool/delivery IDs (single source of truth)
│   ├── render/                    # Render facts: MCP URL/auth, per-tool config paths, skills repo,
│   │                              #   CLI install facts, plugin next-step references
│   ├── wizard/                    # bubbletea TUI: single-axis WHAT picker + summary
│   ├── orchestrator/              # registry + Execute: selection × detected tools → installers
│   ├── manifest/                  # remote metadata matrix: schema, parse/validate, ?version fetch
│   ├── content/                   # fetch render.com/agents/*.md + glamour render + go:embed fallback
│   ├── detect/                    # OS/arch/WSL/TTY + installed-agent detection
│   ├── components/{cli,skills,mcp} # the "WHAT": Installer implementations
│   ├── tools/{claudecode,cursor,codex,opencode} # the "WHERE": Target config writers
│   ├── configedit/                # merge-not-clobber JSON/TOML editors (atomic writes)
│   ├── paths/                     # ~/.render layout, artifact/URL scheme, shell PATH edits
│   ├── cliflags/                  # -y, --components, --agent, --no-login, --json, -r, --version
│   └── logx/                      # leveled text + --json output
│
├── deploy/render-com/             # handed to the render.com frontend repo: route-handler.ts,
│                                  #   vendored agents.sh (byte-identical), README
├── manifest/manifest.json         # metadata matrix (+ manifest.schema.json)
├── testdata/fixtures/             # real-world tool configs for merge tests
├── test/e2e/                      # harness, bootstrap_check, failure_check, detect_check, Dockerfile
└── .github/workflows/             # ci, sync-agents-sh, refresh-content, release, e2e
```

### render.com serving model (researched)

`render.com` is **Next.js behind Cloudflare** with **Sanity** as the headless content backend.
Sanity does no request routing; the Next.js frontend does. Already live: `GET /agents` → HTML for
browsers, and markdown (`/agents.md`, `/agents/*.md`, `/llms.txt`, or `Accept: text/markdown`) for
agents. The **only gap** is `GET /agents.sh` (currently 404) — fixed with one Next.js route handler
serving the vendored bootstrap as `text/x-shellscript`, Cloudflare-cached.

Decisions: the bootstrap is git-owned (auditable, PR-able, checksummed), NOT a Sanity document; no
dedicated installer service (reuse the render.com app); a dedicated `.sh` route rather than
User-Agent sniffing.

### Tool selection: configure all detected tools

No interactive "which tools?" step. The wizard detects installed agents and configures **all of
them**, but **surfaces the detected list** for transparency ("Detected: Claude Code, Cursor, Codex —
will configure all"). Rationale: they're the user's own tools; re-picking is friction; matches the
"one Enter installs everything" goal. The user chooses only *what* (CLI/skills/MCP). Escape hatch:
`--agent <name>` (repeatable) scopes the run for agents/CI.

### Content single-source-of-truth (wizard ↔ render.com)

Human-facing prose (per-tool next steps, guide links) is the **same content Sanity publishes** at
`render.com/agents/*.md` — not duplicated in Go. `manifest.json` maps each tool to its content URL;
`internal/content` fetches it (`Accept: text/markdown`, `?version` aware) with fallbacks **live →
`go:embed` snapshot (`internal/content/embedded/`) → terse built-in**, so an offline run still
prints next steps and never blocks on the network. `refresh-content.yml` snapshots the live copy and
opens a PR on change.

### Tool extensibility: hardcoded tools, metadata-only manifest

Tools/components are **compiled into the binary** (Go `tools.Target` / `components.Installer`); the
binary is the authority. The manifest is **metadata only** (name, content URL, default, delivery,
ordering). Data-driven tool *behavior* was considered and rejected as too expensive.

Consequences: adding a tool = write a `Target` + ship a release. The JSON schema keeps a **closed
`id` enum** (author-time typo protection). **Runtime must ignore unknown IDs** — an older binary that
fetches a newer manifest skips IDs it has no handler for, with a warning, never erroring. Resolves
schema-enum-vs-forward-compat as **closed schema + must-ignore-unknown runtime**.

### Per-tool install facts (researched from render.com/agents/*.md)

The wizard writes config via the shell-automatable **config-file path** for every tool. MCP server:
`https://mcp.render.com/mcp`, name `render`.

| Tool | MCP config file | Format / shape |
|---|---|---|
| Claude Code | `~/.claude.json` | JSON `mcpServers.render` = `{type:"http", url}` |
| Cursor | `~/.cursor/mcp.json` | JSON `mcpServers.render` = `{type:"http", url}` |
| Codex | `~/.codex/config.toml` | TOML `[mcp_servers.render]` `url = …` |
| OpenCode | `~/.config/opencode/opencode.json` | JSON `mcp.render` = `{type:"remote", url, enabled:true}` |

---

## Key design decisions

- **Detect-then-default.** Pre-check everything for detected tools; one Enter = "install everything".
  Granularity is opt-out, not a 10-question interrogation.
- **Config-file path for all tools; plugins are next-steps.** Cursor/Codex plugins install *in-app*
  (`/add-plugin render`, plugin library) and aren't shell-automatable, so the wizard writes config
  directly and surfaces the plugin as recommended next-step copy (`internal/render.PluginFor`). This
  supersedes the earlier "plugin delivery for Claude/Codex" framing.
- **MCP auth = OAuth (assumed live), API-key fallback.** Write credential-free, URL-only MCP entries;
  the tool signs in on first use. `AuthMode` in `internal/render` flips every writer to an
  `Authorization: Bearer $RENDER_API_KEY` env-ref (never a stored secret; OpenCode uses
  `{env:RENDER_API_KEY}`). The installer never writes credentials.
- **Skills delegate to the official installer.** Primary: `npx skills add render-oss/skills --all -g`
  (all skills, all detected agents, global, no prompts). Fallback: `render skills install` (CLI ≥2.10),
  with child stdin left unset (→ `/dev/null`) so any prompt gets EOF rather than hanging.
- **Uninstall (`-r`) is scoped to MCP config removal.** Removes the Render MCP entry from each target
  tool (merge-not-clobber delete) and intentionally does NOT remove the CLI or skills — a half-working
  uninstall misleads. Help text and summary say so.
- **Ephemeral bootstrap.** `agents.sh` downloads the wizard to a scratch dir under `$HOME`, runs it,
  and deletes it (via an EXIT/INT/TERM trap) — it installs nothing of its own and makes no PATH edit,
  so the bootstrap has no footprint to clean up. Only what the wizard sets up persists (the CLI under
  `~/.render/bin`, skills, per-tool MCP config); the wizard owns the CLI's PATH entry. Re-running is
  always a fresh download. (Package-manager channels — Homebrew/WinGet/npm — still install
  `render-setup` persistently; only the curl path is ephemeral.)
- **Binaries on GitHub Releases, version-less names.** `render-setup_<os>_<arch>` (raw binaries) so
  `latest` resolves via GitHub's `/releases/latest/download/` redirect and pinned versions via the
  tag. Scheme centralized in `internal/paths` and mirrored by `agents.sh`. `render.com/agents.sh`
  serves the script; binaries live on GitHub (render.com could later proxy for analytics).

## Security & robustness checklist

- `set -eu`, `main()`-wrapper (no partial execution on truncated downloads).
- Checksum-verify every downloaded binary before executing it. This proves integrity, not
  authenticity: the manifest ships from the same release over the same channel as the artifact, so it
  does not defend against a compromised release host or an intercepted transport. Authenticity comes
  from the cosign signature and SLSA provenance on each release, which the bootstrap cannot check
  itself (that needs `cosign`/`gh` locally). Transport is pinned to HTTPS across redirects so the
  channel cannot be downgraded. See [`SECURITY-MODEL.md`](SECURITY-MODEL.md).
- No `sudo` — the wizard installs the Render CLI under `~/.render/bin` and edits the tools' own
  config files only; the curl bootstrap runs the wizard ephemerally (nothing of its own persists).
- Read prompts from `/dev/tty` only when openable; full non-interactive fallback otherwise (CI/agents).
- Idempotent and re-runnable; merges never overwrite other MCP servers/skills.
- Scoped uninstall (`-r`) shipped from day one.
- OS/arch matrix: macOS arm64/x86_64, Linux arm64/x86_64, WSL; graceful "use WinGet" on native Windows.

---

## Implementation phases (all complete; detail in git history)

- **Phase 0 — Foundations & contracts:** repo scaffold, CI, frozen `components.Installer` /
  `tools.Target` interfaces, `manifest` schema v1 + validation test, shared `internal/ids`,
  artifact/URL and flag conventions.
- **Phase 1 (parallel A–E):** `agents.sh` bootstrap + render.com route template + sync CI (1A);
  detection + paths + shell PATH (1B); `configedit` merge-not-clobber engine (1C); manifest + content
  subsystem with embedded fallback (1D); bubbletea TUI skeleton (1E).
- **Phase 2 — installers:** `internal/render` facts; `components/{cli,skills,mcp}`;
  `tools/{claudecode,cursor,codex,opencode}` writing MCP config via `configedit`. Grounded in the
  researched per-tool config shapes.
- **Phase 3 — integration & non-interactive:** `internal/orchestrator` (registry + `Execute`) wiring
  selection × detected tools → installers → summary; full flag surface; text + `--json` output;
  must-ignore-unknown; non-interactive skills.
- **Phase 4 — release pipeline:** GoReleaser matrix (darwin/linux × amd64/arm64) → version-less
  binaries + `checksums.txt`; tag-triggered `release.yml`; download URL scheme reconciled. A local
  bootstrap E2E verified download→checksum→install→exec; a `/dev/tty`-openable bug was found & fixed.
- **Phase 5 — E2E & OS matrix:** `test/e2e/` harness (config correctness / no-clobber / idempotency /
  uninstall), bootstrap + failure-injection checks, a realistic Docker env (real Claude Code +
  OpenCode installs, detection of all four), and `e2e.yml` across the OS matrix.

### Cross-cutting guarantees (verified)

- No `sudo`; all writes under `~/.render` or tool config dirs.
- Idempotent and re-runnable; merges never clobber other MCP servers/skills.
- Installer never writes credentials (OAuth default; API-key fallback is an env-ref).
- `--json` available wherever there's output; non-interactive is first-class.
- Every downloaded binary checksum-verified before use.

---

## Reference: how Railway does it (analysis that shaped this)

- The curl script is a thin bootstrap, not the wizard; smart logic lives in the compiled binary.
- Composable subcommands; the curl path and the Homebrew path converge on the same commands.
- Detects installed agents and merges its MCP entry without clobbering other servers; skills to a
  universal `~/.agents/skills` dir plus tool-specific dirs.
- Non-interactive is first-class (`-y`, `--agent`, `-r`, `--json`) — because agents run the installer.
- The vanity URL is multi-purpose (script to curl, markdown briefing to agents), making the installer
  a distribution channel.
