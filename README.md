# Render Install Wizard

A standalone setup wizard that installs and configures the Render CLI, agent skills, and the
Render MCP server across your coding agents (Claude Code, Cursor, Codex, OpenCode) from a single
command:

```bash
curl -fsSL render.com/agents.sh | sh
```

The `curl` target is a thin, auditable bootstrap that downloads a checksum-verified wizard binary
(`render-setup`) into a scratch dir, runs it, then deletes it — the bootstrap is ephemeral and
leaves no wizard binary or `PATH` edit behind. The wizard detects your installed agents, writes the
Render MCP server into each one (merge-not-clobber), installs the official skills, and can install
the Render CLI — idempotently, and non-interactively for agents/CI.

> **Read it before you pipe it.** The bootstrap lives in [`scripts/agents.sh`](scripts/agents.sh),
> it's short and `set -eu`, downloads are HTTPS-pinned (redirects included) and checked against
> published SHA-256 checksums before anything executes, and nothing runs with `sudo` — installs live
> under `~/.render` and your tools' own config files.
>
> Checksums prove the bytes are intact, not that the release is ours: the manifest ships alongside the
> artifact it describes. Authenticity comes from the cosign signature and SLSA build provenance on
> every release, which you can verify with `gh attestation verify` — the bootstrap can't, since that
> would require `cosign`/`gh` on your machine. [`docs/SECURITY-MODEL.md`](docs/SECURITY-MODEL.md)
> spells out the whole picture, including where third-party code is involved.

## What it does

- **Detects** installed agents: Claude Code, Cursor, Codex, OpenCode.
- **Render MCP** (`https://mcp.render.com/mcp`): writes the `render` server into each tool's config
  — `~/.claude.json`, `~/.cursor/mcp.json`, `~/.codex/config.toml`, `~/.config/opencode/opencode.json`
  — without clobbering your other MCP servers. Sign-in happens in your agent on first use (OAuth).
- **Skills**: installs Render's official [skills](https://github.com/render-oss/skills) via the
  standard installer.
- **Render CLI**: installs the [CLI](https://github.com/render-oss/cli) if selected.

## Usage

```bash
render-setup                       # interactive picker (when a TTY is present)
render-setup -y                    # non-interactive: install defaults for all detected tools
render-setup --json                # machine-readable summary (implies non-interactive)
render-setup --components mcp      # only configure the MCP server (skip CLI/skills)
render-setup --agent cursor        # scope to specific tool(s); repeatable
render-setup --no-login            # omit the "run render login" next step
render-setup -r                    # remove the Render MCP entry from configured tools
render-setup --version
```

Notable flags: `-y/--yes`, `--components <cli,skills,mcp>`, `--agent <name>` (repeatable),
`--no-login`, `--json`, `-r/--uninstall`, `--dry-run`, `--pin-version <v>`.

`-r` is intentionally scoped: it removes the Render MCP entry from each tool (leaving your other
servers intact) and does **not** remove the CLI or skills.

## Install channels

The bootstrap runs the same wizard everyone else gets; if you installed `render-setup` another way
(Homebrew/WinGet/npm) it stays on disk and you can run it directly. Binaries are published to
[GitHub Releases](https://github.com/render-lab/render-install-wizard/releases) with SHA-256
checksums, a cosign signature over the manifest, and SLSA build provenance per binary;
`RENDER_SETUP_VERSION=vX.Y.Z` pins a version (default: latest).

## Documentation

- [`docs/SPEC.md`](docs/SPEC.md) — problem, architecture, design decisions, and status
- [`docs/SECURITY-MODEL.md`](docs/SECURITY-MODEL.md) — what the install guarantees, and what it doesn't
- [`docs/RELEASE.md`](docs/RELEASE.md) — what's left to launch + release runbook + rollback
- [`docs/FUTURE.md`](docs/FUTURE.md) — not-started / future work (incl. native Windows)

## Development

Requires Go 1.26+.

```bash
make build   # go build ./...
make vet     # go vet ./...
make lint    # gofmt-check + vet (+ golangci-lint if installed)
make test    # go test ./... (includes manifest schema validation)
make all     # build + vet + lint + test
```

End-to-end tests live in [`test/e2e`](test/e2e): a hermetic config harness, a full `curl | sh`
bootstrap check, a failure-injection check, and a realistic Docker env that installs real agents.
Run the config harness locally with `bash test/e2e/harness.sh`.

## License

[Apache-2.0](LICENSE).
