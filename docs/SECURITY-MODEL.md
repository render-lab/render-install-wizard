# Security model

What `curl -fsSL render.com/agents.sh | sh` actually guarantees, and what it does not. This exists
because "checksum-verified" is easy to read as a stronger claim than it is, and because anyone
piping a script into a shell deserves a precise answer rather than a reassuring one.

## What runs, and from where

| Step | Source | Executed as |
| --- | --- | --- |
| `agents.sh` bootstrap | render.com (Cloudflare-fronted) | your shell, no `sudo` |
| `render-setup` wizard | GitHub Releases | a binary in a scratch dir, deleted on exit |
| Render CLI (optional) | Homebrew, or GitHub Releases | a binary under `~/.render/bin` |
| Agent skills (optional) | `skills` npm package via `npx` | Node, in a temp install |

Nothing uses `sudo`. Writes are confined to `~/.render`, the skills directories, and your agents'
own config files.

## What checksum verification proves

Both the bootstrap and the wizard's CLI download compute a SHA-256 over what they fetched and
compare it against a published manifest, refusing to execute anything that does not match.

That establishes **integrity**: the bytes that arrived are the bytes the manifest describes. It
catches a truncated or corrupted download, a flaky proxy, and a mirror that serves a stale or
mangled artifact.

## What checksum verification does not prove

It does not establish **authenticity** — that the release is ours.

The manifest travels with the artifact. `checksums.txt` is fetched from the same release, over the
same connection, from the same host as the binary it describes. Anyone in a position to substitute
the binary is equally in a position to substitute the manifest, and the comparison would then
succeed against attacker-chosen bytes. Concretely, checksum verification alone does not defend
against a compromised release host, a stolen publishing token, or an attacker who can intercept the
transport.

This is why the earlier claim that checksums are "served separately from artifacts" was wrong, and
why the guarantee is described here as integrity rather than trust.

## What closes that gap

**Transport.** The bootstrap pins the protocol with `--proto '=https' --proto-redir '=https'`, so
neither the initial request nor any redirect can be downgraded to cleartext. Without the redirect
pin, an intercepted response could bounce the download to `http://` and the checksum would prove
nothing, having been fetched over the same rewritten channel. A non-loopback, non-HTTPS
`RENDER_INSTALL_BASE_URL` is refused rather than downgraded silently. On the `wget` fallback only the
initial URL's scheme is enforced, because `wget` has no per-retrieval equivalent — which is one
reason `curl` is preferred when both are present.

**Provenance and signing.** Each release carries SLSA build provenance for every binary and a cosign
signature (keyless, Sigstore) over the checksums manifest. These are bound to this repository's
release workflow via its OIDC identity, so unlike a checksum they cannot be reproduced by whoever
serves the file. Verify either:

```bash
gh attestation verify render-setup_darwin_arm64 --repo render-lab/render-install-wizard

cosign verify-blob checksums.txt \
  --signature checksums.txt.sig --certificate checksums.txt.pem \
  --certificate-identity-regexp '^https://github.com/render-lab/render-install-wizard/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

**The bootstrap does not check either one.** Verifying a Sigstore signature requires `cosign` (or
`gh`) on the machine, which a dependency-light `curl | sh` cannot assume. So these raise the floor
for anyone who chooses to audit a release, and for detecting a bad release after the fact — they are
not part of what the one-liner enforces on your behalf.

**Truncation.** All of `agents.sh` is wrapped in `main()`, invoked on the final line, so a download
cut short parses and executes nothing rather than running half an installer.

## Third-party code, and where the guarantees are weaker

Installing skills runs the third-party [`skills`](https://www.npmjs.com/package/skills) npm package
through `npx`. It is pinned to an exact version (`render.SkillsCLISpec`), and npm versions are
immutable, so this resolves to a fixed tarball whose integrity npm checks against the registry.

It is **not** equivalent to the verification applied to our own binaries. No digest is pinned on our
side that a compromised registry would have to match, and the package's own dependencies are
declared as version ranges, so the tree beneath it is resolved fresh on every install and cannot be
pinned from here. The invocation passes `--ignore-scripts`, which removes lifecycle scripts across
that whole tree — the usual vehicle for a malicious npm package — but installing skills still
executes third-party code that is version-pinned rather than independently verified.

Installing the Render CLI via Homebrew inherits Homebrew's trust model. The direct-download path
(used when brew is absent, and always when `--pin-version` is given, since the formula cannot be
pinned) is checksum-verified as described above.

## Trust roots

Using the one-liner means trusting, in order: render.com and its CDN to serve the script; GitHub
Releases to serve the wizard and CLI binaries; the transport between you and both; and, if you
install skills, the npm registry and the `skills` package. Provenance and signing let you verify the
second of those independently; the rest are accepted by using this install path at all.

## Reducing what you have to trust

- Read the script first: it is short, `set -eu`, and lives at [`scripts/agents.sh`](../scripts/agents.sh).
  The copy render.com serves is kept byte-identical by CI (`sync-agents-sh.yml`).
- Pin a version with `RENDER_SETUP_VERSION=vX.Y.Z` instead of tracking `latest`.
- Skip the bootstrap: download a release binary yourself, verify it with `gh attestation verify`,
  and run it directly.
- Preview without changes using `--dry-run`, and narrow the blast radius with `--components` and
  `--agent`.
