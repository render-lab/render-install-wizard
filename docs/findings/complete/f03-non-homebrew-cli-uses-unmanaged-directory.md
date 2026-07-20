# F03: The non-Homebrew CLI lands outside the directory the wizard manages

- Severity: High
- Category: Clean install
- Status: Confirmed

## Problem

The CLI component assumes the executable will be installed at `~/.render/bin/render`, but the fetched official installer currently places a non-root installation at `~/.local/bin/render`. The bootstrap adds only `~/.render/bin` to `PATH`, and the subprocess output containing the official installer's PATH guidance is discarded. A normal Linux installation can consequently be reported as successful while `render login` remains unavailable in the current shell. Detection, status reporting, and the following skills step can also miss the executable that was just installed.

## Files affected

- `internal/components/cli/cli.go:65-69, 114-130`

## Proposed solution

Either download and install the CLI directly into a wizard-owned destination, or capture and validate the actual destination selected by the official installer. Add that directory to both the current process `PATH` and the appropriate persistent shell configuration before later steps run. Add a clean-Linux test without Homebrew that verifies the executable by absolute path, confirms `render` is immediately discoverable, and exercises the subsequent skills installation.
