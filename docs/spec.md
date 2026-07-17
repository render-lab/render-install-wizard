

## Project structure
```
render-install-wizard/
├── README.md                      # "read it before you pipe it" — auditable entry point
├── project.md
├── spec.md
├── LICENSE
├── go.mod
├── go.sum
├── Makefile                       # build, test, lint, release shortcuts
├── .goreleaser.yaml               # cross-compile matrix + checksum publishing
│
├── scripts/
│   └── agents.sh                  # THE thin bootstrap (source of render.com/agents.sh)
│                                  #   set -eu, main()-wrapper, OS/arch detect,
│                                  #   checksum-verify, install to ~/.render/bin,
│                                  #   PATH update, exec wizard w/ /dev/tty
│
├── cmd/
│   └── render-setup/
│       └── main.go                # wizard binary entrypoint (flag parsing → wizard)
│
├── internal/
│   ├── wizard/                    # interactive TUI (bubbletea)
│   │   ├── model.go               # top-level state machine
│   │   ├── picker.go              # single-axis picker: WHAT to install
│   │   │                          #   applied to ALL detected tools (no per-tool prompt)
│   │   ├── detect_defaults.go     # detect-then-default pre-checking
│   │   ├── summary.go             # end-state table + next steps
│   │   │                          #   next-steps copy comes from content pkg, NOT hardcoded
│   │   └── styles.go              # ASCII splash (GROW-2567), spinners
│   │
│   ├── manifest/                  # remote-driven component/tool matrix
│   │   ├── manifest.go            # schema + parsing; carries render.com content URLs
│   │   │                          #   (per-tool guide/next-steps .md), not inline prose
│   │   └── fetch.go               # download + ?version= pinning
│   │
│   ├── content/                   # Sanity-authored copy, fetched at runtime
│   │   ├── fetch.go               # GET render.com/agents/*.md (Accept: text/markdown)
│   │   ├── render.go              # markdown → styled TUI / plain (--json) output
│   │   └── embed.go               # go:embed offline fallback (last-known-good copy)
│   │
│   ├── detect/                    # environment detection
│   │   ├── platform.go            # OS/arch, WSL, TTY presence
│   │   └── tools.go               # Claude Code, Cursor, Codex, OpenCode, ...
│   │
│   ├── components/                # the "WHAT" to install
│   │   ├── component.go           # Installer interface (install/uninstall/status)
│   │   ├── cli/                   # Render CLI
│   │   ├── skills/                # universal (.agents) + tool-specific dirs
│   │   ├── mcp/                   # hosted OAuth MCP (mcp.render.com)
│   │   └── plugins/               # Claude/Codex plugin bundles (skills+MCP)
│   │
│   ├── tools/                     # per-tool config writers (ALL detected tools by default)
│   │   ├── tool.go                # Target interface + plugin-vs-raw resolution
│   │   ├── claudecode/
│   │   ├── cursor/
│   │   ├── codex/
│   │   └── opencode/
│   │
│   ├── configedit/                # merge-not-clobber editors (json/toml/etc.)
│   │   └── ...                    # never overwrite other MCP servers/skills
│   │
│   ├── paths/                     # ~/.render layout, shell PATH (zsh/bash/fish)
│   ├── cliflags/                  # -y, --components, --no-login, --json, -r
│   │                              #   --agent = escape hatch to SCOPE to specific tool(s);
│   │                              #   default (omitted) = configure all detected tools
│   └── logx/                      # structured + --json output
│
│                                  # NOTE: no server in this repo. render.com is an
│                                  # existing Next.js+Sanity app (behind Cloudflare) that
│                                  # ALREADY serves the browser landing page (/agents) and
│                                  # the agent markdown briefing (/agents.md, /llms.txt) via
│                                  # Accept-header content negotiation. We only add ONE route:
│                                  # render.com/agents.sh → serves scripts/agents.sh verbatim.
│
├── deploy/
│   └── render-com/                # integration handed to the render.com frontend repo
│       ├── route-handler.ts       # Next.js route: GET /agents.sh → text/x-shellscript
│       │                          #   returns vendored agents.sh, edge/CDN-cached
│       ├── agents.sh              # vendored copy (byte-identical to scripts/agents.sh)
│       └── README.md              # how the frontend consumes + caches the script
│
├── manifest/
│   └── manifest.json              # source-of-truth matrix served to wizards
│                                  #   references render.com/agents/*.md content URLs
│
├── assets/
│   └── content/                   # go:embed'd last-known-good copy of render.com/agents/*.md
│       └── ...                    # offline/air-gapped/CI fallback; refreshed by CI (see below)
│
├── testdata/
│   └── fixtures/                  # real-world tool configs to test merge logic
│
├── test/
│   └── e2e/                       # end-to-end install/uninstall/idempotency
│
└── .github/
    └── workflows/
        ├── ci.yml                 # build, vet, test, shellcheck agents.sh
        ├── sync-agents-sh.yml     # verify deploy/render-com/agents.sh == scripts/agents.sh
        │                          #   (fail CI on drift; optional PR to frontend repo)
        ├── refresh-content.yml    # snapshot render.com/agents/*.md → assets/content/
        │                          #   (scheduled; opens PR when the Sanity copy changes)
        └── release.yml            # goreleaser → binaries + separate checksums
```

## render.com serving model (researched)

`render.com` is **Next.js behind Cloudflare**, with **Sanity** as the headless content
backend. Sanity itself does no request routing or User-Agent/Accept negotiation — the
Next.js frontend does (per Sanity's own "serve content to agents" field guide).

Already live today (no work needed from us):
- `GET /agents` → `text/html` (Sanity landing page) for browsers
- `GET /agents` with `Accept: text/markdown`, plus `/agents.md`, `/agents/*.md`, `/llms.txt`
  → `text/markdown` agent briefing (dual-route: `.md` suffix AND Accept header)

The only gap:
- `GET /agents.sh` → currently **404**. We fix it with one Next.js route handler that serves
  the vendored bootstrap as `text/x-shellscript`, Cloudflare-cached. This is the exact
  "install.sh 404" fragility called out in project.md.

Decisions locked in:
- The bootstrap script is git-owned (auditable, PR-able, checksummed) — NOT a Sanity document.
- No dedicated installer service/domain; reuse the existing render.com app.
- No User-Agent sniffing on `/agents`; a dedicated `.sh` route is more robust and matches the
  `curl -fsSL render.com/agents.sh | sh` one-liner in project.md.

## Tool selection: configure all detected tools

There is no interactive "which tools?" step. The wizard detects installed agents (Claude Code,
Cursor, Codex, OpenCode, …) and configures **all of them**. It **does surface the detected list**
for transparency — the user sees exactly where Render will be installed before confirming (e.g.
"Detected: Claude Code, Cursor, Codex — will configure all") — it's just shown, not a checklist
to fill out. Rationale:

- These are the user's own installed tools; asking them to re-pick is needless friction.
- Matches the "detect-then-default / one Enter installs everything" goal from project.md.
- Collapses the old two-axis matrix to one axis: the user chooses only *what* (CLI, skills,
  MCP), and it's applied everywhere it makes sense.

Escape hatches (mostly for agents/CI, not the happy path):
- `--agent <name>` (repeatable) scopes the run to specific tool(s) instead of all detected.
- Per-tool config logic still lives in `internal/tools/*` — "configure all" still means writing
  each tool's config correctly (plugin vs raw, merge-not-clobber). "All tools" = all *detected*.

## Content single-source-of-truth (wizard ↔ render.com)

The wizard's human-facing prose — per-tool "next steps," guide links, the closing summary
blurb — is **the same content Sanity already publishes** at `render.com/agents/*.md`. We do
not duplicate it in Go source. This keeps the installer and the website in lockstep: when the
copy team edits the Cursor guide in Sanity, the wizard's Cursor next-steps update too, with no
new binary release.

How it flows:
- `manifest.json` maps each component/tool to its render.com content URL (e.g. Cursor →
  `render.com/agents/cursor.md`) instead of embedding prose. Retitling a tool or updating its
  copy is a manifest + Sanity edit; adding a *new* tool also needs code (see decision below).
- At runtime `internal/content` fetches those `.md` docs (`Accept: text/markdown`, honoring
  `?version=`), and renders them into the TUI or into `--json` output.
- **Fallbacks, in order:** live fetch → `go:embed`ed snapshot in `assets/content/` (last-known-good)
  → terse built-in string. So the wizard still prints sane next-steps offline, air-gapped, or if
  render.com is unreachable, and never blocks install on a network fetch.
- `refresh-content.yml` periodically snapshots the live `.md` into `assets/content/` and opens a
  PR, so the embedded fallback tracks the Sanity copy over time.

Net: Sanity/`render.com` is the one place copy lives; the wizard is a client of it, with a
git-versioned safety net. No prose is authored twice.

## Tool extensibility: hardcoded tools, metadata-only manifest (decision)

Tools and components are **compiled into the binary** — each is a Go `tools.Target` /
`components.Installer`. The **binary is the authority** on what it can actually do. The manifest
is **metadata only** (name, content URL, default-checked, delivery, ordering, min-version), layered
on top of IDs the binary already knows. Data-driven tool *behavior* (a generic per-tool config-merge
spec authored remotely) was considered and rejected as too expensive for the value.

Consequences:
- Adding a new tool = write a `Target` + ship a release (plus manifest/content edits). You cannot
  add a functioning tool via the manifest alone.
- The JSON schema keeps the **closed `id` enum**: it matches the curated, code-backed set and gives
  free typo/drift protection at authoring time.
- **Runtime must ignore unknown IDs.** Because the manifest is remote and shared across binary
  versions, an older binary will eventually fetch a manifest listing a newer tool. The wizard must
  **skip any tool/component ID it has no compiled handler for, with a logged warning — never error**.
  This keeps existing installs working the day we add a tool. (Today's permissive `manifest.Parse`
  is already compatible; the skip-and-warn logic lands in Phase 3 wiring.)
- Keeping the manifest ID set ⊆ the compiled registry is enforceable with a cheap CI test once the
  registry exists (Phase 3).

This resolves open question about schema-enum vs. forward-compat: **closed schema + must-ignore-unknown runtime.**

## Per-tool install facts (researched from render.com/agents/*.md, Phase 2)

The wizard writes config via the shell-automatable **config-file path** for every tool (this is what
`internal/configedit` exists for). The MCP server is `https://mcp.render.com/mcp`, name `render`.

| Tool | MCP config file | Format / shape |
|---|---|---|
| Claude Code | `~/.claude.json` | JSON `mcpServers.render` = `{type:"http", url}` (also `claude mcp add`) |
| Cursor | `~/.cursor/mcp.json` | JSON `mcpServers.render` = `{type:"http", url}` |
| Codex | `~/.codex/config.toml` | TOML `[mcp_servers.render]` `url = …` |
| OpenCode | `~/.config/opencode/opencode.json` | JSON `mcp.render` = `{type:"remote", url, enabled:true}` |

Decisions (locked in for Phase 2):
- **Auth = OAuth (assumed live imminently).** We write **credential-free, URL-only** MCP entries;
  the tool does browser sign-in on first use. This keeps "installer never touches credentials."
  The exact OAuth config shapes aren't published yet (guides still show API-key), so the URL-only
  forms above are provisional. **API-key is the centralized fallback:** an `AuthMode` in
  `internal/render` flips every tool's writer to add `Authorization: Bearer ${RENDER_API_KEY}`
  (env-ref, never a stored secret; OpenCode uses `{env:RENDER_API_KEY}`). One switch, all tools.
- **Skills: delegate to the official installer** — `render skills install` when the CLI (≥2.10) is
  present, else `npx skills add render-oss/skills` (auto-detects tools, writes per-tool + universal
  dirs). Repo: `github.com/render-oss/skills`.
- **Plugins are NOT shell-automatable for Cursor/Codex** (in-app `/add-plugin render` and the Codex
  plugin library, respectively). The wizard uses the config-file path and **surfaces the plugin as
  recommended next-step copy**. OpenCode's plugin (`install.sh`) and Claude's `claude mcp add` are
  shell-automatable and may be used where selected. This supersedes the earlier
  `PreferredDelivery: plugin` framing for Claude/Codex.
- Render-specific facts (MCP URL/name, auth mode, per-tool config paths, universal skills dir,
  skills repo, plugin next-step references, CLI download) live in one package: `internal/render`.