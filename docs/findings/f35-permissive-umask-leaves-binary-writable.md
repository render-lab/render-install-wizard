# F35: A permissive umask can leave a PATH-installed binary writable

- Severity: Medium
- Category: Local security
- Status: Risk

## Problem

The bootstrap creates the destination using the caller's umask, moves the downloaded file with its inherited mode, and runs only `chmod +x`. It never removes group or world write permissions. Under a permissive umask or in a shared home, another local user can replace the executable in a directory that is later added to `PATH`. When the account owner next runs `render-setup`, the replacement executes with that user's privileges.

## Files affected

- `scripts/agents.sh:201-210`

## Proposed solution

Set `umask 077` while staging and installing, create wizard-owned directories with explicit restrictive modes, and set the final binary mode rather than only adding execute bits. Verify the destination and file are owned by the current user before replacement, then use the same-directory atomic installation sequence. Add tests under permissive umasks and shared-directory modes that verify group and world write bits are absent and unsafe ownership causes installation to fail closed.
