# F43: PATH block management is racy and not self-healing

- Severity: Low
- Category: Shell integration
- Status: Risk

## Problem

The bootstrap checks for a start marker and appends a PATH block without locking the shell startup file. Any start marker is treated as a complete valid block, and a later `RENDER_HOME` change does not replace the old path. Concurrent installs can append duplicates, while an interrupted write or changed install can leave a stale or incomplete PATH entry. Bash always targets `.bashrc`, so macOS login shells that read `.bash_profile` or `.profile` may never load the update.

## Files affected

- `scripts/agents.sh:45-68`

## Proposed solution

Under an advisory lock, parse and replace exactly one complete managed block rather than performing a marker check followed by append. Validate both markers and the current resolved path, repairing incomplete blocks and replacing stale values. Select the startup file according to the shell's login behavior. Add tests for concurrent runs, an interrupted half-block, a changed `RENDER_HOME`, duplicate blocks, and Bash login versus interactive startup behavior on macOS and Linux.
