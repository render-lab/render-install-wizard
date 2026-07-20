# Realistic Linux E2E environment for the Render install wizard.
#
# It builds the wizard, then in a fresh image installs *real* coding agents
# where a headless install exists (Claude Code + OpenCode) and seeds config
# dirs for the GUI/opaque ones (Cursor, Codex), so detection and config-writing
# run against a realistic home. It then runs:
#   - detect_check.sh  (the wizard detects the installed agents), and
#   - harness.sh       (hermetic install / no-clobber / idempotency / uninstall).
#
# Build for a specific arch with:  docker build --platform linux/amd64 -f test/e2e/Dockerfile .
# (repo root is the build context)

# ---- build stage ----
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/render-setup ./cmd/render-setup

# ---- runtime / test stage ----
FROM node:22-bookworm-slim
RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates curl jq git bash \
	&& rm -rf /var/lib/apt/lists/*

# Real agent installs that support headless installation.
RUN npm install -g @anthropic-ai/claude-code
RUN curl -fsSL https://opencode.ai/install | bash

# Seed config dirs for agents without a headless CLI install, so detection and
# merge-not-clobber run against realistic homes.
RUN mkdir -p /root/.cursor /root/.codex

# Make sure the OpenCode install location is on PATH for detection.
ENV PATH="/root/.opencode/bin:/root/.local/bin:${PATH}"

COPY --from=build /out/render-setup /usr/local/bin/render-setup
COPY test/e2e/harness.sh test/e2e/detect_check.sh test/e2e/cli_install_check.sh test/e2e/opencode_jsonc_check.sh /e2e/
RUN chmod +x /e2e/*.sh

# Verify detection against the really-installed + seeded agents, confirm the CLI
# installs into (and is discoverable from) the wizard-owned dir on this clean
# non-Homebrew host, exercise the JSONC (opencode.jsonc) edit path, then run the
# full hermetic install/idempotency/uninstall harness.
CMD BIN=/usr/local/bin/render-setup /e2e/detect_check.sh claude-code cursor codex opencode \
	&& BIN=/usr/local/bin/render-setup /e2e/cli_install_check.sh \
	&& BIN=/usr/local/bin/render-setup /e2e/opencode_jsonc_check.sh \
	&& BIN=/usr/local/bin/render-setup /e2e/harness.sh
