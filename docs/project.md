**Problem**

There is no unified entry point for installing the Render CLI, MCP, and Skills. They require separate installation steps across multiple docs.

Meanwhile, Railway and others provide a butter-smooth experience for local dev setup when you run a single command: 
curl -fsSL agents.railway.com | sh

```bash
Setting up Railway for agents
Installing railway, please wait…
✓ CLI — v5.26.1 · ~/.railway/bin
✓ Shell PATH — ~/.zshrc updated
✓ Agent skills — Universal (.agents), Claude Code, Cursor
✓ Railway MCP (local) — Claude Code, Cursor
```

**Solution**

Ship `curl -fsSL render.com/agents.sh | sh` — a thin bootstrap that downloads a standalone setup wizard (its own binary, independent of the CLI) that lets the user pick, with granularity, which components (CLI, skills, MCP, plugins) to install and which tools (Claude Code, Cursor, Codex, OpenCode, …) to configure.

### How Railway does it (analysis)

- **The curl script is a thin bootstrap, not the wizard.** `curl -fsSL agents.railway.com | sh` installs the CLI to `~/.railway/bin`, updates PATH, then delegates everything else to `railway setup agent`. All the smart logic lives in the CLI, not the shell script (Every (Agents) Connection to Railway).
- **Composable CLI subcommands underneath:** `railway setup agent` (full flow), `railway mcp install [--agent codex] [--remote]`, `railway skills install --agent <name>`. The curl path and the "I already have the CLI via Homebrew" path converge on the same commands.
- **Tool detection:** it detects installed agents (Claude Code, Cursor, Codex, OpenCode, GitHub Copilot, Factory Droid) and merges the Railway MCP entry into each tool's config without clobbering other servers. Skills go to the universal `~/.agents/skills` directory plus tool-specific dirs (Skill Issue).
- **Non-interactive is first-class:** `-y` skips prompts, `--agent` targets specific tools, `--remote` swaps in the hosted OAuth MCP, `-r` uninstalls, `--json` everywhere. This matters because *agents themselves* frequently run the installer.
- **The URL is triple-purpose.** `agents.railway.com` serves a shell script to `curl` and an agent-facing markdown briefing ("Set yourself up for Railway — do this first") to crawlers/agents, so an agent that merely browses railway.com discovers the one-liner and suggests it to the user. **Content negotiation on one vanity domain makes the installer a distribution channel, not just a script.** (Note: Railway's agent-facing pages even embed prompt-injection payloads targeting visiting AI agents — evidence these pages are read by agents at scale.)
- **Auto-updating skills:** with CLI auto-update enabled, the CLI refreshes skills from the `railway-skills` repo in the background, so skills stay current without re-running the installer.
- **Auth is inherited, not configured:** the local MCP ships inside the CLI and reuses the `railway login` token — no token pasting anywhere in the flow.

### Hosting the install script

1. **Own a vanity endpoint** — `render.com/agents.sh`. Serve it from a Render static site or service, and it fixes the GitHub-raw fragility we hit at the localhost workshop (install.sh 404).
2. **Content-negotiate on User-Agent:** `curl`/`wget` → shell script; browsers → the revamped render.com/agents landing page; agents/crawlers → a markdown setup briefing. One URL, three audiences.
3. **Keep the script's source in a public render-oss repo** with the endpoint deploying from it — auditable ("read it before you pipe it"), PR-able, versioned. Support `?version=` pinning and publish checksums for downloaded binaries.
4. **Instrument it.** Owning the endpoint gives us install analytics (count, OS/arch, referrer) that GitHub raw never will — feeds the Zero-to-Production CLI/MCP adoption metrics.
5. **Windows:** scope the curl path to macOS/Linux/WSL; the in-flight WinGet manifest covers native Windows.

### Script architecture: thin bootstrap → standalone wizard

The shell script only: detects OS/arch → downloads the standalone wizard binary (checksum-verified) → installs it to `~/.render/bin` → updates PATH (zsh/bash/fish) → `exec`s the wizard, forwarding flags. Wrap the whole script in a `main()` invoked on the last line so a truncated download executes nothing.

Everything interactive lives in a standalone wizard binary (e.g. `render-setup`), shipped and versioned independently of the CLI:

- The wizard is Go, so we get a real TUI (multi-select checkboxes, spinners, the ASCII-art splash already ticketed in GROW-2567) instead of fragile POSIX `read` loops.
- **This solves the TTY problem.** When a script is piped to `sh`, stdin *is the script*, so naive prompts hang or read garbage (see Starship issue #7133). The bootstrap re-attaches stdin to `/dev/tty` when a TTY exists; the compiled binary then prompts reliably.
- Homebrew/WinGet/npm users get the identical wizard by installing and running `render-setup` directly — one code path, many install channels. The CLI is just another component the wizard can install, not the host for the wizard.
- Idempotent and re-runnable: rerun to add Cursor later, update skills, or switch MCP modes.
- Precedent in-house: `render workflows init` is already an interactive flow with non-interactive flags (`--confirm --language --install-deps --output text`), and pulls its "next steps" copy from remote `template.yaml` files. Reuse that trick: drive the wizard's component/tool matrix from a remote manifest so we can add new tools without shipping a new wizard binary.

### The interactive wizard

The picker is a two-axis matrix: *what* to install × *where* to configure it.

```bash
Setting up Render for agents (v1.x.x)

✓ Setup wizard — v1.x.x · ~/.render/bin      (installed by bootstrap)
✓ Shell PATH — ~/.zshrc updated

Detected: Claude Code, Cursor, Codex

? What should we set up? (space to toggle, enter to confirm)
❯ ◉ Render CLI          v1.x.x
  ◉ Agent skills        deploy, debug, monitor + 18 more
  ◉ Render MCP          hosted, OAuth — sign in on first use
  ◉ Universal (.agents) skills for any agent

? Configure which tools?
❯ ◉ Claude Code   (via Render plugin)
  ◉ Cursor
  ◉ Codex         (via Render plugin)
  ◯ OpenCode

? Log in to Render now? (opens browser) [Y/n]
```

Key design decisions:

- **Detect-then-default:** pre-check everything for detected tools; one Enter = the Railway-equivalent "install everything" path. Granularity is opt-out, not a 10-question interrogation.
- **Plugins are an implementation detail, not a checkbox.** For Claude Code and Codex, the correct delivery vehicle for "skills + MCP" *is* the plugin (marketplace-listed, bundles both). The wizard picks plugin vs. raw skills-dir + `mcp.json` per tool automatically and never double-installs. Surface it as "Claude Code ✓ (via plugin)" in output.
- **Default to the hosted MCP** at `mcp.render.com` — that's what OAuth just unlocked. A local/remote distinction only matters if we later ship a CLI-embedded local MCP like Railway's.
- **Non-interactive mode.** Agents will run this installer. Support `-y` (all defaults), `--components cli,skills,mcp`, `--agent claude-code --agent cursor`, `--no-login`, `--json`, and `-r`/`--uninstall`. With no TTY present, behave as `-y` and print a summary of what was configured.
- **End state:** print a summary table + next steps ("run `render login`", "ask your agent: *deploy this repo to Render*")

### Where MCP OAuth fits

- **The installer never touches credentials.** Pre-OAuth, configuring MCP meant putting an API key in each tool's config. Now the wizard writes config with the pre-registered client ID per tool (claude / cursor / codex), and browser sign-in happens lazily on first tool use.
- **Two auth prompts remain; be honest about them:** `render login` for CLI path (CLI token), and OAuth consent for MCP. Make the final "log in now?" step optional and let MCP OAuth be first-touch auth for MCP-only users. Longer-term, MCP-OAuth-as-signup makes the installer an acquisition surface.

### Security & robustness checklist

- `set -eu`, `main()`-wrapper pattern (no partial execution on truncated downloads)
- Checksum-verify every binary the bootstrap downloads; serve checksums separately from artifacts
- No `sudo` — install to `~/.render/bin` only
- Read prompts from `/dev/tty`; full non-interactive fallback when absent (CI, agents)
- Idempotent: safe to re-run; upgrade existing installs; merge (never overwrite) other MCP servers/skills in tool configs
- Ship an uninstall path (`-r`) from day one
- OS/arch matrix: macOS arm64/x86_64, Linux arm64/x86_64, WSL; graceful "use WinGet" message on native Windows

### Open questions

1. **Skills freshness:** auto-update skills in the background (Railway does) or require re-running setup?
2. **Plugin/skills dedup rules** need specifying per tool before the wizard can be correct.
3. **The agent-facing briefing page** (what agents.render.com returns to non-curl clients) is a content deliverable of its own and slots into the render.com/agents revamp already underway.