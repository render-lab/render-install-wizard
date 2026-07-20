#!/usr/bin/env bash
# Verifies F10 end-to-end: when OpenCode's active config is a JSONC file
# (opencode.jsonc, with comments and trailing commas), the wizard edits THAT
# file in place rather than creating an inactive opencode.json. It also exercises
# F08 (the render entry is replaced wholesale, siblings preserved) and F09/F10
# (comments and formatting survive), across install, re-run (idempotency), and
# uninstall.
#
# Usage: BIN=/path/to/render-setup test/e2e/opencode_jsonc_check.sh
#        (BIN defaults to a freshly built ./render-setup)
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

sha() {
	if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi
}

if [ -z "$BIN" ]; then
	BIN="$(mktemp -d)/render-setup"
	echo "Building wizard -> $BIN"
	(cd "$REPO_ROOT" && go build -o "$BIN" ./cmd/render-setup) || {
		echo "build failed"
		exit 1
	}
fi

HOME_DIR="$(mktemp -d)"
export HOME="$HOME_DIR"
export RENDER_HOME="$HOME_DIR/.render"

DIR="$HOME_DIR/.config/opencode"
JSONC="$DIR/opencode.jsonc"
JSON="$DIR/opencode.json"
mkdir -p "$DIR"

# A valid JSONC config: line comment + trailing comma + a pre-existing MCP server.
cat >"$JSONC" <<'EOF'
{
  // user's preferred theme — must survive edits
  "theme": "dark",
  "mcp": {
    "other": { "type": "remote", "url": "https://example.com/mcp", "enabled": true },
  }
}
EOF

echo "== Phase A: configure MCP into an existing opencode.jsonc =="
if ! "$BIN" --components mcp --agent opencode >/dev/null 2>&1; then
	die "installer exited non-zero"
fi

grep -q "$MCP_URL" "$JSONC" && pass "render MCP written into the active .jsonc" || die "render MCP missing from .jsonc"
grep -q "user's preferred theme" "$JSONC" && pass "comment preserved" || die "comment lost during edit"
grep -q '"other"' "$JSONC" && pass "pre-existing server preserved" || die "sibling server clobbered"
grep -q '"theme": "dark"' "$JSONC" && pass "unrelated key preserved" || die "unrelated key lost"
[ ! -e "$JSON" ] && pass "no inactive opencode.json duplicate created" || die "inactive opencode.json was created"

echo "== Phase B: idempotency (re-run must not change the file) =="
before="$(sha "$JSONC")"
"$BIN" --components mcp --agent opencode >/dev/null 2>&1 || die "second run exited non-zero"
after="$(sha "$JSONC")"
[ "$before" = "$after" ] && pass "re-run is byte-identical (idempotent)" || die "re-run changed the .jsonc (not idempotent)"

echo "== Phase C: uninstall removes only the render entry =="
"$BIN" -r --components mcp --agent opencode >/dev/null 2>&1 || die "uninstall exited non-zero"
grep -q "$MCP_URL" "$JSONC" && die "render entry not removed on uninstall" || pass "render entry removed"
grep -q "user's preferred theme" "$JSONC" && pass "comment survived uninstall" || die "comment lost on uninstall"
grep -q '"other"' "$JSONC" && pass "sibling server survived uninstall" || die "sibling removed on uninstall"

rm -rf "$HOME_DIR"
echo
if [ "$fail" -eq 0 ]; then
	echo "OPENCODE JSONC E2E PASSED"
	exit 0
fi
echo "OPENCODE JSONC E2E FAILED"
exit 1
