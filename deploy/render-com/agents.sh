#!/bin/sh
# render.com/agents.sh - thin, auditable bootstrap for the Render install wizard.
#
# Usage:
#   curl -fsSL render.com/agents.sh | sh
#   curl -fsSL render.com/agents.sh | sh -s -- -y --json
#
# What it does: detect OS/arch -> download the checksum-verified `render-setup`
# wizard binary into a scratch dir -> run it (forwarding any args) -> delete it.
# The bootstrap is ephemeral: it installs nothing of its own and leaves no
# render-setup binary or PATH edit behind. Only what the wizard sets up (the
# Render CLI, agent skills, MCP config) persists; the wizard manages the CLI's
# PATH itself. No sudo.
#
# Read it before you pipe it. The source of truth for this script lives at
# scripts/agents.sh in github.com/render-lab/render-install-wizard. The copy
# served by render.com (deploy/render-com/agents.sh) is kept byte-identical by CI.
#
# ALL logic is wrapped in main() and invoked on the very last line, so a
# truncated download parses/executes nothing.
set -eu

main() {
	# ---- output helpers (plain ASCII; no ANSI, no unicode) ----
	info() { printf '%s\n' "$*"; }
	ok() { printf '[ok] %s\n' "$*"; }
	err() {
		printf 'error: %s\n' "$*" >&2
		exit 1
	}

	# ---- configuration (env with defaults) ----
	# Binaries are published to GitHub Releases; RENDER_INSTALL_BASE_URL points at
	# the releases base and can be overridden (e.g. for a mirror or a local test).
	base_url="${RENDER_INSTALL_BASE_URL:-https://github.com/render-lab/render-install-wizard/releases}"
	base_url="${base_url%/}"
	version="${RENDER_SETUP_VERSION:-latest}"

	# ---- OS detection ----
	uname_s="$(uname -s 2>/dev/null || echo unknown)"
	case "${uname_s}" in
	Darwin)
		os="darwin"
		;;
	Linux)
		os="linux"
		;;
	MINGW* | MSYS* | CYGWIN*)
		info "Windows detected."
		info ""
		info "The curl bootstrap supports macOS, Linux, and WSL only."
		info "On native Windows, install with the WinGet package instead:"
		info ""
		info "    winget install Render.RenderSetup"
		info ""
		info "(Running inside WSL? Re-run this command from your Linux shell.)"
		exit 0
		;;
	*)
		err "unsupported operating system: ${uname_s}"
		;;
	esac

	# ---- WSL detection (still treated as linux, surfaced for transparency) ----
	if [ "${os}" = "linux" ]; then
		uname_r="$(uname -r 2>/dev/null || echo)"
		proc_version=""
		if [ -r /proc/version ]; then
			proc_version="$(cat /proc/version 2>/dev/null || echo)"
		fi
		case "${uname_r}${proc_version}" in
		*microsoft* | *Microsoft* | *WSL*)
			info "WSL detected - installing the Linux build."
			;;
		esac
	fi

	# ---- arch detection ----
	uname_m="$(uname -m 2>/dev/null || echo unknown)"
	case "${uname_m}" in
	x86_64 | amd64)
		arch="amd64"
		;;
	arm64 | aarch64)
		arch="arm64"
		;;
	*)
		err "unsupported architecture: ${uname_m}"
		;;
	esac

	info "Setting up Render for agents (${os}/${arch})"

	# ---- pick a downloader: prefer curl, fall back to wget ----
	downloader=""
	if command -v curl >/dev/null 2>&1; then
		downloader="curl"
	elif command -v wget >/dev/null 2>&1; then
		downloader="wget"
	else
		err "need either curl or wget installed to download the wizard"
	fi

	# fetch <url> <dest-file>
	fetch() {
		if [ "${downloader}" = "curl" ]; then
			curl -fsSL "$1" -o "$2"
		else
			wget -q -O "$2" "$1"
		fi
	}

	# ---- artifact + URL layout (mirrors internal/paths.DownloadURL) ----
	# Version-less artifact name so the "latest" redirect URL is stable; the
	# concrete version is carried by the release tag in the URL path. "latest"
	# uses GitHub's latest-release redirect, any other value is a release tag.
	artifact="render-setup_${os}_${arch}"
	checksums_file="checksums.txt"
	if [ "${version}" = "latest" ]; then
		download_dir="${base_url}/latest/download"
	else
		download_dir="${base_url}/download/${version}"
	fi
	binary_url="${download_dir}/${artifact}"
	checksums_url="${download_dir}/${checksums_file}"

	# ---- download into a scratch dir; remove it (and the binary) on exit ----
	# Place the scratch under $HOME rather than /tmp: /tmp is sometimes mounted
	# noexec, which would block running the wizard from it. Cleaned up on exit
	# (including Ctrl-C / TERM) so the bootstrap leaves no render-setup behind.
	tmp="$(mktemp -d "${HOME:-.}/.render-setup.XXXXXX" 2>/dev/null || mktemp -d)"
	trap 'rm -rf "${tmp}"' EXIT
	trap 'exit 130' INT
	trap 'exit 143' TERM

	if ! fetch "${binary_url}" "${tmp}/${artifact}"; then
		err "failed to download wizard from ${binary_url}"
	fi
	if ! fetch "${checksums_url}" "${tmp}/${checksums_file}"; then
		err "failed to download checksums from ${checksums_url}"
	fi
	ok "Downloaded render-setup (version ${version})"

	# ---- verify sha256 against the checksums manifest ----
	expected="$(awk -v want="${artifact}" '
		{ name = $2; sub(/^[*]/, "", name); if (name == want) { print $1; exit } }
	' "${tmp}/${checksums_file}")"
	if [ -z "${expected}" ]; then
		err "no checksum entry for ${artifact} in ${checksums_file}"
	fi

	if command -v sha256sum >/dev/null 2>&1; then
		actual="$(sha256sum "${tmp}/${artifact}" | awk '{print $1}')"
	elif command -v shasum >/dev/null 2>&1; then
		actual="$(shasum -a 256 "${tmp}/${artifact}" | awk '{print $1}')"
	else
		err "need sha256sum or shasum to verify the download"
	fi

	if [ "${expected}" != "${actual}" ]; then
		err "checksum mismatch for ${artifact} (expected ${expected}, got ${actual}); aborting"
	fi
	ok "Verified sha256 checksum"

	# ---- make the downloaded wizard executable in place ----
	target="${tmp}/${artifact}"
	chmod +x "${target}"

	# ---- run the wizard, then let the EXIT trap delete it ----
	# Deliberately do NOT `exec`: control must return here so the scratch dir
	# (and the wizard binary in it) is removed, keeping the bootstrap ephemeral.
	# When stdin is not a terminal (e.g. piped from `curl | sh`), reconnect it to
	# the controlling terminal so the wizard can prompt. /dev/tty may EXIST but not
	# be openable (no controlling terminal, as in CI/containers/agents), so probe
	# that it can actually be opened before redirecting; otherwise run headless and
	# let the wizard fall back to its non-interactive path.
	info "Starting the Render setup wizard..."
	set +e
	if [ ! -t 0 ] && (exec 3</dev/tty) 2>/dev/null; then
		"${target}" "$@" </dev/tty
	else
		"${target}" "$@"
	fi
	status=$?
	set -e
	exit "${status}"
}

main "$@"
