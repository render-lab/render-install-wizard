# F30: Unescaped `RENDER_HOME` or `HOME` becomes shell startup syntax

- Severity: Low
- Category: Shell safety
- Status: Risk

## Problem

A caller-controlled `RENDER_HOME`, or `HOME` through the default destination, is interpolated into double-quoted POSIX and fish startup snippets without shell-language escaping. Quotes, newlines, backticks, and command substitutions can remain active in the persisted startup file. Default home-derived paths are safe, but if a less-trusted wrapper or environment source can influence these variables, injected commands can execute the next time the user opens a shell.

## Files affected

- `scripts/agents.sh:29-78, 165`

## Proposed solution

Validate the destination as a non-empty absolute path before persisting it. Either restrict it to a conservative, documented character set or implement correct, separate escaping for POSIX shells and fish so the value is always treated as data. Reject control characters and malformed paths. Add tests with spaces, quotes, newlines, backticks, dollar substitutions, and fish-specific syntax, then parse or execute the generated startup snippets in isolated shells to confirm no injected command runs.
