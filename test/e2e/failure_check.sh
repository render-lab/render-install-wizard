#!/usr/bin/env bash
# Failure-injection E2E: the bootstrap must abort cleanly — leaving nothing
# installed — when the download fails (missing asset / network error) or the
# downloaded artifact is corrupt (partial download → checksum mismatch).
#
# Prereq: a GoReleaser snapshot has populated ./dist.
# Usage: DIST=./dist test/e2e/failure_check.sh
set -u

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
DIST="${DIST:-$REPO_ROOT/dist}"
PORT="${PORT:-8802}"

os="$(uname -s)"; case "$os" in Darwin) os=darwin ;; Linux) os=linux ;; *) echo "unsupported OS $os"; exit 1 ;; esac
arch="$(uname -m)"; case "$arch" in x86_64 | amd64) arch=amd64 ;; arm64 | aarch64) arch=arm64 ;; *) echo "unsupported arch $arch"; exit 1 ;; esac
artifact="render-setup_${os}_${arch}"

binsrc="$(find "$DIST" -type f -name render-setup -path "*${os}_${arch}*" | head -1)"
[ -n "$binsrc" ] || { echo "FAIL: no snapshot binary for ${os}/${arch} under $DIST"; exit 1; }

fail=0

# expect_abort DESC WEBROOT: serve WEBROOT, run the bootstrap, and assert it
# exits non-zero and installs nothing.
expect_abort() {
	desc="$1"; web="$2"
	home="$(mktemp -d)"
	(cd "$web" && python3 -m http.server "$PORT" >/dev/null 2>&1 &
		echo $! >"$web/.pid")
	sleep 1.2

	if HOME="$home" RENDER_HOME="$home/.render" \
		RENDER_INSTALL_BASE_URL="http://127.0.0.1:${PORT}" RENDER_SETUP_VERSION="testv" \
		sh "$REPO_ROOT/scripts/agents.sh" --version >/dev/null 2>&1; then
		code=0
	else
		code=$?
	fi
	kill "$(cat "$web/.pid")" 2>/dev/null || true

	if [ "$code" -ne 0 ] && [ ! -e "$home/.render/bin/render-setup" ]; then
		echo "  PASS: $desc (aborted, nothing installed)"
	else
		installed=no; [ -e "$home/.render/bin/render-setup" ] && installed=yes
		echo "  FAIL: $desc (exit=$code, installed=$installed)" >&2
		fail=1
	fi
	rm -rf "$home"
}

# Case A: the binary asset is missing (404 / network error), checksums present.
a="$(mktemp -d)"; mkdir -p "$a/download/testv"
cp "$DIST/checksums.txt" "$a/download/testv/checksums.txt"
expect_abort "missing binary asset" "$a"
rm -rf "$a"

# Case B: partial/corrupt download — truncated binary vs. the real checksum.
b="$(mktemp -d)"; mkdir -p "$b/download/testv"
head -c 1024 "$binsrc" >"$b/download/testv/$artifact"   # truncated (partial download)
cp "$DIST/checksums.txt" "$b/download/testv/checksums.txt"
expect_abort "corrupt/partial download (checksum mismatch)" "$b"
rm -rf "$b"

echo
[ "$fail" -eq 0 ] && echo "FAILURE-INJECTION E2E PASSED" || echo "FAILURE-INJECTION E2E FAILED"
exit "$fail"
