# F15: Config updates have no lock or backup and replace the original inode

- Severity: Medium
- Category: Data integrity
- Status: Risk

## Problem

Each configuration change performs an unlocked read-modify-rename. Concurrent writes by the wizard or an agent can race and lose one side's update. Renaming over the destination replaces symlink-managed dotfiles and drops inode-associated metadata such as ACLs and extended attributes. TOML comments and ordering are intentionally discarded, and no recoverable backup is created. A routine MCP update can therefore overwrite concurrent user state, break dotfile management, or irreversibly remove user-authored Codex comments.

## Files affected

- `internal/configedit/configedit.go:54-92, 176-219`

## Proposed solution

Take an advisory lock around the full read-modify-write operation and use a compare-and-swap or pre-rename conflict check. Define and enforce an explicit symlink policy, preserve ownership and relevant metadata, use surgical JSON and TOML edits, and create a recoverable backup before replacement. Add concurrency, symlink, ACL/xattr where supported, comment-preservation, and rollback tests, including a simulated conflict between the initial read and final write.
