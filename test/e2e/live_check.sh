#!/usr/bin/env bash
# Cross-image smoke test for the LIVE `curl | sh` bootstrap.
#
# Runs the served agents.sh (render.com, a Render preview URL, or any host)
# inside disposable containers across base images and CPU arches, asserting that
# the bootstrap downloads + checksum-verifies + executes the wizard and then
# removes it (ephemeral) — in a clean room, so nothing touches your host machine.
#
# It forwards `--dry-run --json` to the wizard by default: that still exercises
# the full download -> verify -> exec path (the binary is downloaded and run,
# then deleted), but makes no agent-config changes and avoids the "MCP needs a
# detected tool" failure in a bare container. Override WIZARD_ARGS for a deeper
# run (e.g. seed an agent marker and pass `-y`).
#
# Prereqs: Docker. Non-native arches need qemu/buildx (Docker Desktop provides
# it). The wizard release must be published so the GitHub download URLs resolve.
#
# Usage:
#   test/e2e/live_check.sh [URL]                       # URL defaults to render.com/agents.sh
#   PLATFORMS="linux/amd64 linux/arm64" test/e2e/live_check.sh https://<preview-host>/agents.sh
#   WIZARD_ARGS="-y --json --components cli" test/e2e/live_check.sh <url>
set -u

URL="${1:-https://render.com/agents.sh}"
WIZARD_ARGS="${WIZARD_ARGS:---dry-run --json}"

# Default to the host arch; set PLATFORMS to test more than one.
if [ -z "${PLATFORMS:-}" ]; then
	case "$(uname -m)" in
	x86_64 | amd64) PLATFORMS="linux/amd64" ;;
	arm64 | aarch64) PLATFORMS="linux/arm64" ;;
	*) PLATFORMS="linux/amd64" ;;
	esac
fi

command -v docker >/dev/null 2>&1 || {
	echo "docker is required for live_check.sh" >&2
	exit 1
}

# Each case: "name|image|prep" where prep installs curl (+ ca-certs) as needed.
# The trio covers glibc without Node (Render CLI skills fallback), glibc with
# Node (npx skills path), and musl (static-binary compatibility).
cases=(
	"debian-glibc|debian:bookworm-slim|apt-get update -qq && apt-get install -y -qq curl ca-certificates >/dev/null"
	"node-glibc|node:22-bookworm-slim|apt-get update -qq && apt-get install -y -qq curl ca-certificates >/dev/null"
	"alpine-musl|alpine:3.20|apk add --no-cache curl >/dev/null"
)

fail=0
results=""

for plat in $PLATFORMS; do
	for c in "${cases[@]}"; do
		name="${c%%|*}"
		rest="${c#*|}"
		image="${rest%%|*}"
		prep="${rest#*|}"
		label="${plat} ${name} (${image})"
		printf '\n=== %s ===\n' "$label"

		# Clean-room run: fetch+exec the served bootstrap (set -e makes a nonzero
		# bootstrap fail the case), then confirm it was ephemeral — no render-setup
		# left behind. ${HOME} is escaped so it expands inside the container
		# (HOME=/root), not on the host.
		container_script="set -e
${prep}
curl -fsSL '${URL}' | sh -s -- ${WIZARD_ARGS}
if [ -e \"\${HOME}/.render/bin/render-setup\" ]; then echo 'FAIL: render-setup persisted (should be ephemeral)' >&2; exit 1; fi
echo '[ok] bootstrap ran and left no wizard binary behind (ephemeral)'"

		if docker run --rm --platform "$plat" "$image" sh -c "$container_script"; then
			results="${results}PASS  ${label}\n"
		else
			results="${results}FAIL  ${label}\n"
			fail=1
		fi
	done
done

printf '\n===== live_check summary (URL=%s, args=%s) =====\n' "$URL" "$WIZARD_ARGS"
printf '%b' "$results"
if [ "$fail" -eq 0 ]; then
	echo "ALL PASSED"
else
	echo "SOME FAILED"
fi
exit "$fail"
