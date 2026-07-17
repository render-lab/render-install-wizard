# Render for coding agents

Render meets your agent where it already works. One command wires up the core
Render resources across every coding agent you have installed:

- **Render MCP server** — hosted, OAuth-authenticated access to your Render
  account (services, databases, deploys, logs) from any MCP-capable agent.
- **Agent skills** — Render-authored skills that teach your agent how to deploy,
  inspect, and operate services on Render.
- **Render CLI** — the `render` command line for scripting and local workflows.

## Supported agents

- **Claude Code** — via the Render plugin (skills + MCP bundled).
- **Cursor** — via MCP + agent skills.
- **Codex** — via the Render plugin (skills + MCP bundled).
- **OpenCode** — via MCP + agent skills.

## Next steps

1. Run `curl -fsSL render.com/agents.sh | sh` (or `render-setup`).
2. Approve the Render MCP OAuth prompt on first use.
3. Ask your agent to deploy or inspect a service on Render.

Full guides: https://render.com/agents
