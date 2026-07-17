# Render for OpenCode

OpenCode is configured with the **raw** delivery path: the wizard merges the
Render MCP server and agent skills into your OpenCode config, preserving any
other servers and settings you already have.

## What gets installed

- **Render MCP server** — added to your OpenCode config (merge-not-clobber).
- **Agent skills** — Render skills placed in your `.agents` skills directory.
- **Render CLI** — installed to `~/.render/bin` and added to your PATH.

## Next steps

1. Restart OpenCode so it picks up the new MCP server.
2. Approve the Render MCP OAuth prompt on first use.
3. Ask OpenCode to deploy a service or inspect logs on Render.

Guide: https://render.com/agents/opencode
