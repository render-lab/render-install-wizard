# Render Install Wizard

A standalone setup wizard that installs and configures the Render CLI, agent skills, and the
Render MCP server across your coding agents (Claude Code, Cursor, Codex, OpenCode) from a single
command:

```bash
curl -fsSL render.com/agents.sh | sh
```

The `curl` target is a thin, auditable bootstrap that downloads a checksum-verified wizard binary
(`render-setup`) into `~/.render/bin`, updates your `PATH`, and runs it. The wizard detects your
installed agents, writes the Render MCP server into each one (merge-not-clobber), installs the
official skills, and can install the Render CLI — idempotently, and non-interactively for agents/CI.

> **Read it before you pipe it.** The bootstrap lives in [`scripts/agents.sh`](scripts/agents.sh),
> it's short and `set -eu`, every downloaded binary is verified against published SHA-256 checksums,
> and nothing runs with `sudo` — installs live under `~/.render` and your tools' own config files.

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

The bootstrap installs the same wizard everyone else gets; you can also run `render-setup` directly
if you installed it another way. Binaries are published to
[GitHub Releases](https://github.com/render-lab/render-install-wizard/releases) with SHA-256
checksums; `RENDER_SETUP_VERSION=vX.Y.Z` pins a version (default: latest).

## Documentation

- [`docs/SPEC.md`](docs/SPEC.md) — problem, architecture, design decisions, and status
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
