#!/usr/bin/env bash
# Verifies the wizard detects the coding agents currently installed in this
# environment (real installs + seeded config dirs), by parsing the tool list
# from `render-setup --dry-run --json`.
#
# Usage: BIN=/path/to/render-setup test/e2e/detect_check.sh <tool-id> [tool-id...]
# Example: BIN=... detect_check.sh claude-code cursor codex opencode
set -u

BIN="${BIN:?set BIN to the render-setup binary}"
command -v jq >/dev/null 2>&1 || {
	echo "missing required tool: jq"
	exit 1
}

out="$("$BIN" --dry-run --json 2>/dev/null)"
if ! printf '%s' "$out" | jq . >/dev/null 2>&1; then
	echo "FAIL: --json output is not valid JSON:" >&2
	printf '%s\n' "$out" >&2
	exit 1
fi

fail=0
for want in "$@"; do
	if printf '%s' "$out" | jq -e --arg t "$want" '.tools[]? | select(.id == $t)' >/dev/null; then
		echo "  PASS: detected $want"
	else
		echo "  FAIL: did not detect $want" >&2
		fail=1
	fi
done

[ "$fail" -eq 0 ] && echo "DETECTION PASSED" || echo "DETECTION FAILED"
exit "$fail"
