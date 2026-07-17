#!/usr/bin/env bash
# Exercises the full `curl | sh` bootstrap path against a locally-served snapshot
# build: it serves dist/ over HTTP in the pinned-release layout and runs
# scripts/agents.sh, asserting the wizard is downloaded, checksum-verified,
# installed, and exec'd (reporting --version). Runs in a hermetic HOME.
#
# Prereq: a GoReleaser snapshot has populated ./dist (checksums.txt + binaries).
# Usage: DIST=./dist test/e2e/bootstrap_check.sh
set -u

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
DIST="${DIST:-$REPO_ROOT/dist}"
PORT="${PORT:-8799}"

os="$(uname -s)"; case "$os" in Darwin) os=darwin ;; Linux) os=linux ;; *) echo "unsupported OS $os"; exit 1 ;; esac
arch="$(uname -m)"; case "$arch" in x86_64 | amd64) arch=amd64 ;; arm64 | aarch64) arch=arm64 ;; *) echo "unsupported arch $arch"; exit 1 ;; esac
artifact="render-setup_${os}_${arch}"

binsrc="$(find "$DIST" -type f -name render-setup -path "*${os}_${arch}*" | head -1)"
[ -n "$binsrc" ] || { echo "FAIL: no snapshot binary for ${os}/${arch} under $DIST"; exit 1; }
[ -f "$DIST/checksums.txt" ] || { echo "FAIL: $DIST/checksums.txt missing"; exit 1; }

tmp="$(mktemp -d)"; web="$tmp/web"; home="$tmp/home"
mkdir -p "$web/download/testv" "$home"
cp "$binsrc" "$web/download/testv/$artifact"
cp "$DIST/checksums.txt" "$web/download/testv/checksums.txt"

(cd "$web" && python3 -m http.server "$PORT" >/dev/null 2>&1 &
	echo $! >"$tmp/pid")
sleep 1.5

echo "== bootstrap curl|sh flow (served snapshot, ${os}/${arch}) =="
if HOME="$home" RENDER_HOME="$home/.render" \
	RENDER_INSTALL_BASE_URL="http://127.0.0.1:${PORT}" RENDER_SETUP_VERSION="testv" \
	sh "$REPO_ROOT/scripts/agents.sh" --version; then
	code=0
else
	code=$?
fi

kill "$(cat "$tmp/pid")" 2>/dev/null || true

ok=0
[ "$code" -eq 0 ] || { echo "FAIL: bootstrap exited $code"; ok=1; }
[ -x "$home/.render/bin/render-setup" ] || { echo "FAIL: binary not installed"; ok=1; }
rm -rf "$tmp"

[ "$ok" -eq 0 ] && echo "BOOTSTRAP E2E PASSED" || echo "BOOTSTRAP E2E FAILED"
exit "$ok"
