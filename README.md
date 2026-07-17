# Render Install Wizard

A standalone setup wizard that installs and configures the Render CLI, agent skills, and the
Render MCP across your coding agents (Claude Code, Cursor, Codex, OpenCode, …) from a single
command:

```bash
curl -fsSL render.com/agents.sh | sh
```

The `curl` target is a thin, auditable bootstrap that downloads a checksum-verified wizard
binary (`render-setup`) into `~/.render/bin` and runs it. The wizard is idempotent, works
non-interactively for agents/CI, and uninstalls cleanly.

> Read it before you pipe it: the bootstrap script lives in [`scripts/agents.sh`](scripts/agents.sh)
> and every downloaded binary is verified against published checksums.

## Documentation

- [`docs/project.md`](docs/project.md) — problem statement and intent
- [`docs/spec.md`](docs/spec.md) — architecture and design decisions
- [`docs/plan.md`](docs/plan.md) — phased workback plan with acceptance criteria

## Development

```bash
make build   # go build ./...
make vet     # go vet ./...
make lint    # gofmt-check + vet (+ golangci-lint if installed)
make test    # go test ./... (includes manifest schema validation)
make all     # build + vet + lint + test
```

Requires Go 1.26+.

## Status

Early development. See `docs/plan.md` for phase status.
