# F01: Positional arguments can turn an intended dry run into a live install

- Severity: High
- Category: CLI safety
- Status: Confirmed

## Problem

The flag parser does not reject values left in `fs.Args()`, and Go's flag parser stops processing flags at the first positional operand. For example, `render-setup install --dry-run` treats `install`, `--dry-run`, and everything after it as operands, so `DryRun` remains false. The command then proceeds with the default real installation even though the user supplied the conventional safety flag. Arbitrary trailing operands are also silently accepted, making misspelled or unsupported command forms dangerous rather than self-correcting.

## Files affected

- `internal/cliflags/flags.go:71-84`

## Proposed solution

After parsing, reject any positional operands in `fs.Args()` and return a clear validation error. Handle `flag.ErrHelp` separately so help remains a successful, non-mutating path. Add entrypoint tests covering operands before and after safety flags, including `install --dry-run`, and assert that invalid forms exit nonzero without invoking any installer or changing state.
