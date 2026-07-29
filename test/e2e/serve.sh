#!/usr/bin/env bash
# Shared static file server for the bootstrap E2E scripts.
#
# It uses a tiny Go server (Go is always present via setup-go in CI) instead of
# `python3 -m http.server`, which on some macOS runners is absent or hangs on
# per-request reverse-DNS logging — the cause of the flaky connect-timeout E2E
# failures. The server binds loopback-only and readiness is polled before the
# client runs, so a failed start fails fast (with the server log) instead of
# hanging the downloader for the full connect timeout.
#
# Source it, then:
#   serve_start <web-root> <port>   # sets SERVE_PID; returns nonzero on failure
#   ... run the client against http://127.0.0.1:<port> ...
#   serve_stop

_serve_bin=""

# _serve_build compiles the file server once per shell (into a temp path).
_serve_build() {
	if [ -n "$_serve_bin" ]; then
		return 0
	fi
	_serve_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
	_serve_bin="$(mktemp -d)/fileserver"
	if ! (cd "$_serve_root" && go build -o "$_serve_bin" ./test/e2e/fileserver); then
		echo "serve: failed to build the E2E file server" >&2
		_serve_bin=""
		return 1
	fi
}

# serve_start <dir> <port>: start the server and block until it is accepting
# connections (or fail). On success SERVE_PID is set.
serve_start() {
	_serve_build || return 1
	_serve_dir="$1"
	_serve_port="$2"
	SERVE_LOG="$(mktemp)"
	"$_serve_bin" "$_serve_dir" "127.0.0.1:${_serve_port}" >"$SERVE_LOG" 2>&1 &
	SERVE_PID=$!
	# Detach from job control so the shell doesn't print "Terminated" when
	# serve_stop kills it; we manage its lifetime explicitly.
	disown "$SERVE_PID" 2>/dev/null || true

	_i=0
	while [ "$_i" -lt 80 ]; do # up to ~20s
		if curl -fsS -o /dev/null "http://127.0.0.1:${_serve_port}/" 2>/dev/null; then
			return 0
		fi
		if ! kill -0 "$SERVE_PID" 2>/dev/null; then
			echo "serve: server exited before becoming ready:" >&2
			cat "$SERVE_LOG" >&2
			return 1
		fi
		sleep 0.25
		_i=$((_i + 1))
	done
	echo "serve: server not ready on 127.0.0.1:${_serve_port} after timeout:" >&2
	cat "$SERVE_LOG" >&2
	kill "$SERVE_PID" 2>/dev/null || true
	SERVE_PID=""
	return 1
}

# serve_stop: terminate the server started by serve_start (if any).
serve_stop() {
	if [ -n "${SERVE_PID:-}" ]; then
		kill "$SERVE_PID" 2>/dev/null || true
	fi
	SERVE_PID=""
}
