# F32: Bootstrap replacement is not crash-safe across filesystems

- Severity: Medium
- Category: Bootstrap durability
- Status: Risk

## Problem

The bootstrap moves the verified binary directly from the system temporary directory over the working target and applies executable permissions afterward. When the source and destination are on different filesystems, `mv` degrades to a copy followed by deletion and is not atomic. A crash, full disk, interrupted copy, or failed `chmod` can destroy the previous working installer and leave a partial or non-executable target.

## Files affected

- `scripts/agents.sh:201-206`

## Proposed solution

Create a temporary file inside the destination directory, copy the verified binary into it, set the final restrictive mode and ownership, and `fsync` both file and directory as appropriate. Only then atomically rename it over the existing binary, retaining the old executable until the replacement is ready. Add failure-injection tests for short writes, full disk, interrupted copy, `chmod` failure, and rename failure, asserting that the previous target remains runnable in every pre-rename failure case.
