#!/bin/sh
# render.com/agents.sh - thin, auditable bootstrap for the Render install wizard.
#
# Usage:
#   curl -fsSL render.com/agents.sh | sh
#   curl -fsSL render.com/agents.sh | sh -s -- -y --json
#
# What it does: detect OS/arch -> download the checksum-verified `render-setup`
# wizard binary -> install it to ~/.render/bin -> update PATH -> exec the wizard,
# forwarding any args. No sudo; everything lives under $RENDER_HOME.
#
# Read it before you pipe it. The source of truth for this script lives at
# scripts/agents.sh in github.com/render-oss/render-install-wizard. The copy
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

	# update_path <bin-dir>: add <bin-dir> to PATH in the user's shell rc,
	# exactly once, inside a marked block. Shell is chosen from $SHELL.
	update_path() {
		up_bin_dir="$1"
		up_shell="${SHELL:-}"
		up_shell="${up_shell##*/}"

		case "${up_shell}" in
		fish)
			up_rc="${HOME}/.config/fish/config.fish"
			up_style="fish"
			;;
		zsh)
			up_rc="${HOME}/.zshrc"
			up_style="posix"
			;;
		bash)
			up_rc="${HOME}/.bashrc"
			up_style="posix"
			;;
		*)
			up_rc="${HOME}/.profile"
			up_style="posix"
			;;
		esac

		if [ -f "${up_rc}" ] && grep -qF '# >>> render >>>' "${up_rc}"; then
			ok "Shell PATH - ${up_rc} already configured"
			return 0
		fi

		mkdir -p "$(dirname "${up_rc}")"
		if [ "${up_style}" = "fish" ]; then
			printf '\n# >>> render >>>\nfish_add_path "%s"\n# <<< render <<<\n' "${up_bin_dir}" >>"${up_rc}"
		else
			# $PATH is written literally into the rc file so it expands at shell
			# startup, not now; the single-quoted format string is intentional.
			# shellcheck disable=SC2016
			printf '\n# >>> render >>>\nexport PATH="%s:$PATH"\n# <<< render <<<\n' "${up_bin_dir}" >>"${up_rc}"
		fi
		ok "Shell PATH - ${up_rc} updated (restart your shell or 'source' it)"
	}

	# ---- configuration (env with defaults) ----
	# Binaries are published to GitHub Releases; RENDER_INSTALL_BASE_URL points at
	# the releases base and can be overridden (e.g. for a mirror or a local test).
	base_url="${RENDER_INSTALL_BASE_URL:-https://github.com/render-oss/render-install-wizard/releases}"
	base_url="${base_url%/}"
	version="${RENDER_SETUP_VERSION:-latest}"
	render_home="${RENDER_HOME:-${HOME}/.render}"

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

	# ---- download into a scratch dir; clean it up on exit ----
	tmp="$(mktemp -d 2>/dev/null || mktemp -d -t render-setup)"
	trap 'rm -rf "${tmp}"' EXIT

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

	# ---- install to $RENDER_HOME/bin/render-setup ----
	bin_dir="${render_home}/bin"
	target="${bin_dir}/render-setup"
	mkdir -p "${bin_dir}"
	mv -f "${tmp}/${artifact}" "${target}"
	chmod +x "${target}"
	ok "Installed - ${target}"

	# ---- idempotently update PATH in the user's shell rc ----
	update_path "${bin_dir}"

	# ---- clean up before exec (exec skips the EXIT trap) ----
	rm -rf "${tmp}"
	trap - EXIT

	# ---- re-attach a TTY and hand off to the wizard, forwarding args ----
	# When stdin is not a terminal (e.g. piped from `curl | sh`), reconnect it to
	# the controlling terminal so the wizard can prompt. /dev/tty may EXIST but not
	# be openable (no controlling terminal, as in CI/containers/agents), so probe
	# that it can actually be opened before redirecting; otherwise run headless and
	# let the wizard fall back to its non-interactive path.
	info "Starting the Render setup wizard..."
	if [ ! -t 0 ] && (exec 3</dev/tty) 2>/dev/null; then
		exec "${target}" "$@" </dev/tty
	else
		exec "${target}" "$@"
	fi
}

main "$@"
