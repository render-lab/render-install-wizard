#!/usr/bin/env bash
# Exercises the full `curl | sh` bootstrap path against a locally-served snapshot
# build: it serves dist/ over HTTP in the pinned-release layout and runs
# scripts/agents.sh, asserting the wizard is downloaded, checksum-verified,
# executed (reporting --version), and then removed — the bootstrap is ephemeral
# and must leave no render-setup binary behind. Runs in a hermetic HOME.
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

# shellcheck source=test/e2e/serve.sh
. "$(dirname "$0")/serve.sh"
if ! serve_start "$web" "$PORT"; then
	echo "BOOTSTRAP E2E FAILED (file server did not start)"
	rm -rf "$tmp"
	exit 1
fi

echo "== bootstrap curl|sh flow (served snapshot, ${os}/${arch}) =="
out="$tmp/out.txt"
if HOME="$home" RENDER_HOME="$home/.render" \
	RENDER_INSTALL_BASE_URL="http://127.0.0.1:${PORT}" RENDER_SETUP_VERSION="testv" \
	sh "$REPO_ROOT/scripts/agents.sh" --version >"$out" 2>&1; then
	code=0
else
	code=$?
fi
cat "$out"

serve_stop

ok=0
[ "$code" -eq 0 ] || { echo "FAIL: bootstrap exited $code"; ok=1; }
# The download + verify + exec path actually ran.
grep -q 'Verified sha256 checksum' "$out" || { echo "FAIL: checksum step did not run"; ok=1; }
grep -q 'Starting the Render setup wizard' "$out" || { echo "FAIL: wizard was not started"; ok=1; }
# Ephemeral: the bootstrap must not leave a render-setup binary or scratch dir.
[ ! -e "$home/.render/bin/render-setup" ] || { echo "FAIL: render-setup persisted (bootstrap should be ephemeral)"; ok=1; }
if find "$home" -name 'render-setup*' -type f 2>/dev/null | grep -q .; then echo "FAIL: a render-setup binary was left behind"; ok=1; fi
if find "$home" -maxdepth 1 -name '.render-setup.*' 2>/dev/null | grep -q .; then echo "FAIL: scratch dir was left behind"; ok=1; fi
rm -rf "$tmp"

[ "$ok" -eq 0 ] && echo "BOOTSTRAP E2E PASSED" || echo "BOOTSTRAP E2E FAILED"
exit "$ok"
