#!/usr/bin/env bash
# Verifies F03 on a clean Linux host without Homebrew: `render-setup --components
# cli` downloads the Render CLI directly into the wizard-owned ~/.render/bin,
# makes it discoverable on PATH, and the binary runs by absolute path. The
# official install script would instead drop a non-root install in ~/.local/bin
# (outside the directory the wizard manages) and only print PATH guidance, so a
# run could report success while `render` stayed unavailable.
#
# Discoverability via the wizard-managed bin dir is precisely what the following
# skills step relies on to locate `render`, so it is asserted directly here.
#
# Usage: BIN=/path/to/render-setup test/e2e/cli_install_check.sh
set -u

BIN="${BIN:?set BIN to the render-setup binary}"

fail=0
pass() { printf '  PASS: %s\n' "$1"; }
die() {
	printf '  FAIL: %s\n' "$1" >&2
	fail=1
}

# This check only exercises the direct-download path, which runs when Homebrew
# is absent. With brew present the wizard would delegate to it instead.
if command -v brew >/dev/null 2>&1; then
	echo "SKIP: Homebrew present; not a clean non-Homebrew host"
	exit 0
fi

HOME_DIR="$(mktemp -d)"
export HOME="$HOME_DIR"
export RENDER_HOME="$HOME_DIR/.render"
export SHELL=/bin/bash # PATH persistence should target ~/.bashrc
# Neutralize ambient config-home overrides for a fully hermetic run.
unset XDG_CONFIG_HOME CLAUDE_CONFIG_DIR CODEX_HOME OPENCODE_CONFIG

echo "== install the Render CLI (direct download into wizard-owned dir) =="
if ! "$BIN" --components cli --no-login >/dev/null 2>&1; then
	die "installer exited non-zero"
fi

RENDER_BIN="$HOME_DIR/.render/bin/render"

# 1. Installed into the wizard-owned directory (verified by absolute path).
if [ -x "$RENDER_BIN" ]; then
	pass "CLI installed at wizard-owned $RENDER_BIN"
else
	die "CLI not found (or not executable) at $RENDER_BIN"
fi

# 2. Runs by absolute path.
if "$RENDER_BIN" --version >/dev/null 2>&1; then
	pass "CLI runs by absolute path (render --version)"
else
	die "CLI --version failed by absolute path"
fi

# 3. PATH persisted for future shells.
if grep -q '\.render/bin' "$HOME_DIR/.bashrc" 2>/dev/null; then
	pass "wizard-owned bin dir persisted to ~/.bashrc"
else
	die "PATH entry not written to ~/.bashrc"
fi

# 4. Discoverable once the wizard-managed bin dir is on PATH — exactly what the
#    subsequent skills step needs to find the CLI.
if PATH="$HOME_DIR/.render/bin:$PATH" command -v render >/dev/null 2>&1; then
	pass "render is discoverable via the wizard-managed PATH entry"
else
	die "render not discoverable via PATH"
fi

rm -rf "$HOME_DIR"
echo
if [ "$fail" -eq 0 ]; then
	echo "CLI INSTALL E2E PASSED"
	exit 0
fi
echo "CLI INSTALL E2E FAILED"
exit 1
