# F22: The dormant skills uninstaller removes the entire universal skills directory

- Severity: Low
- Category: Latent data loss
- Status: Risk

## Problem

The skills component's `Uninstall` method calls `os.RemoveAll` on `~/.agents/skills`, a universal directory shared by skills from multiple vendors. It does not record or distinguish Render-owned entries. The current scoped `-r` path does not invoke this method, but wiring it into a future full uninstall would recursively delete unrelated user-installed skills and their local changes.

## Files affected

- `internal/components/skills/skills.go:91-100`

## Proposed solution

Record the specific Render-owned skill directories or files created during installation and remove only those known entries during uninstall. Validate each deletion target remains beneath the intended skills root and leave the shared root and foreign children intact. If ownership cannot be tracked safely, keep the method unreachable and remove the misleading uninstall capability from the interface. Add a test fixture containing Render and non-Render skills and assert only the managed entries are removed.
