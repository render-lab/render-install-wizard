#!/usr/bin/env bash
# End-to-end harness for the Render install wizard.
#
# It builds a hermetic, realistic HOME with pre-existing tool configs (each
# already containing an unrelated MCP server + unrelated keys), runs the wizard
# to configure the Render MCP server into every tool, and asserts:
#   1. the render entry was written with the correct per-tool shape,
#   2. pre-existing servers/keys were preserved (merge-not-clobber),
#   3. re-running is idempotent (byte-identical configs),
#   4. uninstall (-r) removes only the render entry.
#
# Usage: BIN=/path/to/render-setup test/e2e/harness.sh
#        (BIN defaults to a freshly built ./render-setup)
#
# Requires: bash, jq, grep, sha256sum|shasum.
set -u

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BIN="${BIN:-}"
MCP_URL="https://mcp.render.com/mcp"

fail=0
pass() { printf '  PASS: %s\n' "$1"; }
die() {
	printf '  FAIL: %s\n' "$1" >&2
	fail=1
}

# ---- build the binary if not provided ----
if [ -z "$BIN" ]; then
	BIN="$(mktemp -d)/render-setup"
	echo "Building wizard -> $BIN"
	(cd "$REPO_ROOT" && go build -o "$BIN" ./cmd/render-setup) || {
		echo "build failed"
		exit 1
	}
fi

# ---- tool checks ----
for tool in jq grep; do
	command -v "$tool" >/dev/null 2>&1 || {
		echo "missing required tool: $tool"
		exit 1
	}
done

# codex_has SECTION FILE: true if a [mcp_servers.SECTION] table exists.
codex_has() { grep -q "\[mcp_servers.$1\]" "$2"; }

sha() {
	if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi
}

# ---- hermetic HOME with realistic pre-existing configs ----
HOME_DIR="$(mktemp -d)"
export HOME="$HOME_DIR"
export RENDER_HOME="$HOME_DIR/.render"
# Neutralize ambient config-home overrides so each tool's config resolves inside
# this hermetic HOME. CI runners can set XDG_CONFIG_HOME (which OpenCode's path
# resolution honors), which would send the render entry outside HOME_DIR.
unset XDG_CONFIG_HOME CLAUDE_CONFIG_DIR CODEX_HOME OPENCODE_CONFIG

CURSOR="$HOME_DIR/.cursor/mcp.json"
CLAUDE="$HOME_DIR/.claude.json"
CODEX="$HOME_DIR/.codex/config.toml"
OPENCODE="$HOME_DIR/.config/opencode/opencode.json"

mkdir -p "$HOME_DIR/.cursor" "$HOME_DIR/.codex" "$HOME_DIR/.config/opencode"

cat >"$CURSOR" <<'EOF'
{
  "mcpServers": { "other": { "type": "http", "url": "https://example.com/mcp" } },
  "someSetting": true
}
EOF
cat >"$CLAUDE" <<'EOF'
{
  "mcpServers": { "other": { "type": "http", "url": "https://example.com/mcp" } },
  "numStartups": 5
}
EOF
cat >"$CODEX" <<'EOF'
# my codex config
model = "gpt-5"

[mcp_servers.other]
url = "https://example.com/mcp"
EOF
cat >"$OPENCODE" <<'EOF'
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": { "other": { "type": "remote", "url": "https://example.com/mcp", "enabled": true } },
  "theme": "dark"
}
EOF

AGENTS="--agent claude-code --agent cursor --agent codex --agent opencode"

echo "== Phase A: install MCP into all tools =="
# shellcheck disable=SC2086
"$BIN" --components mcp $AGENTS >/dev/null 2>&1 || die "installer exited non-zero"

# JSON assertions (render added, other preserved, unrelated key preserved, OAuth = no header).
jq -e --arg u "$MCP_URL" '.mcpServers.render.url == $u' "$CURSOR" >/dev/null && pass "cursor: render MCP written" || die "cursor: render MCP missing"
jq -e '.mcpServers.render.headers == null' "$CURSOR" >/dev/null && pass "cursor: OAuth (no Authorization header)" || die "cursor: unexpected header in OAuth mode"
jq -e '.mcpServers.other' "$CURSOR" >/dev/null && pass "cursor: pre-existing server preserved" || die "cursor: clobbered other server"
jq -e '.someSetting == true' "$CURSOR" >/dev/null && pass "cursor: unrelated key preserved" || die "cursor: lost unrelated key"

jq -e --arg u "$MCP_URL" '.mcpServers.render.url == $u' "$CLAUDE" >/dev/null && pass "claude: render MCP written" || die "claude: render MCP missing"
jq -e '.mcpServers.other and .numStartups == 5' "$CLAUDE" >/dev/null && pass "claude: existing data preserved" || die "claude: clobbered existing data"

jq -e --arg u "$MCP_URL" '.mcp.render.url == $u and .mcp.render.enabled == true' "$OPENCODE" >/dev/null && pass "opencode: render MCP written" || die "opencode: render MCP missing/incomplete"
jq -e '.mcp.other and .theme == "dark"' "$OPENCODE" >/dev/null && pass "opencode: existing data preserved" || die "opencode: clobbered existing data"

# TOML assertions for codex via grep (portable; no python version dependency).
if codex_has render "$CODEX" && grep -q "$MCP_URL" "$CODEX" && codex_has other "$CODEX" &&
	grep -q '^model' "$CODEX" && ! grep -qi 'authorization' "$CODEX"; then
	pass "codex: render MCP written, others + model preserved, OAuth (no header)"
else
	die "codex: MCP assertion failed"
fi

echo "== Phase B: idempotency (re-run must not change configs) =="
before="$(for f in "$CURSOR" "$CLAUDE" "$CODEX" "$OPENCODE"; do sha "$f"; done)"
# shellcheck disable=SC2086
"$BIN" --components mcp $AGENTS >/dev/null 2>&1 || die "second run exited non-zero"
after="$(for f in "$CURSOR" "$CLAUDE" "$CODEX" "$OPENCODE"; do sha "$f"; done)"
[ "$before" = "$after" ] && pass "re-run is byte-identical (idempotent)" || die "re-run changed configs (not idempotent)"

echo "== Phase C: uninstall removes only the render entry =="
# shellcheck disable=SC2086
"$BIN" -r --components mcp $AGENTS >/dev/null 2>&1 || die "uninstall exited non-zero"
jq -e '.mcpServers.render == null' "$CURSOR" >/dev/null && pass "cursor: render removed" || die "cursor: render not removed"
jq -e '.mcpServers.other' "$CURSOR" >/dev/null && pass "cursor: other server survived uninstall" || die "cursor: uninstall clobbered other"
jq -e '.mcp.render == null and .mcp.other' "$OPENCODE" >/dev/null && pass "opencode: render removed, other survived" || die "opencode: uninstall wrong"
if ! codex_has render "$CODEX" && codex_has other "$CODEX"; then
	pass "codex: render removed, other survived"
else
	die "codex: uninstall wrong"
fi

rm -rf "$HOME_DIR"
echo
if [ "$fail" -eq 0 ]; then
	echo "E2E PASSED"
	exit 0
fi
echo "E2E FAILED"
exit 1
