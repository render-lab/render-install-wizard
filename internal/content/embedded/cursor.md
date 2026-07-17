# Render for Cursor

Cursor is configured with the **raw** delivery path: the wizard merges the
Render MCP server and agent skills into your Cursor config without touching your
other MCP servers or rules.

## What gets installed

- **Render MCP server** — added to your Cursor MCP config (merge-not-clobber).
- **Agent skills** — Render skills placed in your `.agents` skills directory.
- **Render CLI** — installed to `~/.render/bin` and added to your PATH.

## Next steps

1. Reload Cursor so it picks up the new MCP server.
2. Approve the Render MCP OAuth prompt on first use.
3. Ask Cursor to deploy a service or tail logs on Render.

Guide: https://render.com/agents/cursor
