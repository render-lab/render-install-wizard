// Package cli implements the components.Installer contract for the Render CLI.
package cli

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/render-lab/render-install-wizard/internal/components"
	"github.com/render-lab/render-install-wizard/internal/execx"
	"github.com/render-lab/render-install-wizard/internal/ids"
	"github.com/render-lab/render-install-wizard/internal/paths"
	"github.com/render-lab/render-install-wizard/internal/render"
)

// Per-step network/package-manager timeouts (F17). Each attempt gets its own
// bounded deadline so a stalled step fails with a clear error instead of hanging
// indefinitely, and a hung preferred installer doesn't starve the fallback.
const (
	brewTimeout     = 5 * time.Minute
	downloadTimeout = 5 * time.Minute
	// versionProbeTimeout bounds the identity probe in Detect. It is short because
	// this runs during planning, before anything is installed, and a binary that
	// will not answer a help request promptly is not one to depend on.
	versionProbeTimeout = 10 * time.Second
)

// renderCLIHelpMarker is the header the Render CLI prints atop its help output
// ("Render CLI v<version>", from cmd/helptemplate.go upstream). It identifies the
// binary as Render's rather than some other program that happens to be named
// "render"; see Component.isRenderCLI.
const renderCLIHelpMarker = "Render CLI"

// Size caps on untrusted input. Both the HTTP bodies and the zip entries the
// wizard reads are fully buffered in memory, and both are attacker-controlled if
// the release host is compromised or a proxy interposes: without a cap, a response
// that never ends or a zip entry that expands enormously would grow the heap until
// the process is killed. Reading one byte past the cap is enough to detect the
// overrun, so the excess is bounded too.
//
// The limits sit far above any legitimate asset — the CLI archive is single-digit
// MiB and the checksums file is a few hundred bytes — so a real release cannot
// trip them.
const (
	maxFetchBytes       = 64 << 20 // 64 MiB
	maxArchiveFileBytes = 64 << 20 // 64 MiB
)

// Component installs and manages the Render CLI.
//
// All external effects are routed through injectable, unexported dependencies so
// that tests can exercise behavior without touching the real filesystem home or
// shelling out to brew/curl/render.
type Component struct {
	// home is the user's home directory. The Render CLI is installed to, looked
	// for, and removed from <home>/.render/bin/render. Injectable so tests can
	// point at a t.TempDir() instead of the real home.
	home string
	// goos/goarch identify the target platform for release asset selection.
	// Injectable so tests exercise a fixed platform regardless of the host.
	goos   string
	goarch string
	// lookPath resolves an executable name to a path, like exec.LookPath.
	lookPath func(string) (string, error)
	// run executes a command and returns only its error, discarding output.
	run func(ctx context.Context, name string, args ...string) error
	// runOutput executes a command and returns its combined output plus error.
	runOutput func(ctx context.Context, name string, args ...string) (string, error)
	// fetch downloads the bytes at a URL (the release archive and checksums).
	// Injectable so tests never hit the network.
	fetch func(ctx context.Context, url string) ([]byte, error)
	// redirectTarget returns where a URL redirects to, without following it. Used
	// to resolve the latest release tag. Injectable so tests never hit the network.
	redirectTarget func(ctx context.Context, url string) (string, error)
	// ensurePath makes binDir usable in this process and persists it to the
	// user's shell configuration. Injectable so tests avoid mutating the real
	// environment or shell rc files.
	ensurePath func(binDir string) error
}

// New returns a new Render CLI component wired with production defaults: the
// real user home, exec.LookPath for resolution, and exec.CommandContext-based
// runners for command execution.
func New() *Component {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return &Component{
		home:           home,
		goos:           runtime.GOOS,
		goarch:         runtime.GOARCH,
		lookPath:       exec.LookPath,
		run:            execx.Run,
		runOutput:      execx.CombinedOutput,
		fetch:          httpFetch,
		redirectTarget: httpRedirectTarget,
		ensurePath:     ensurePathIn(home),
	}
}

// ID returns the canonical identifier for the Render CLI component.
func (c *Component) ID() ids.ComponentID { return ids.ComponentCLI }

// binPath returns the path to the CLI binary managed under the injectable home:
// <home>/.render/bin/render.
func (c *Component) binPath() string {
	return filepath.Join(c.home, ".render", "bin", render.CLIBinaryName)
}

// Detect reports whether the Render CLI is already installed: the managed binary
// exists at <home>/.render/bin/render, or a binary named "render" on PATH proves
// itself to be the Render CLI.
//
// The managed path is checked first and trusted without a probe, since this wizard
// is what put the binary there.
//
// A PATH hit is verified rather than believed. "render" is a plausible name for an
// unrelated program — a template renderer, a graphics tool, a local wrapper script
// — and treating any such binary as the CLI would make the wizard report the
// component installed and skip installing it, leaving later steps that shell out
// to `render` (a skills install, for instance) to fail against a program that
// knows nothing about Render.
//
// An ambiguous probe resolves to "not installed", so the failure mode is a
// redundant install rather than a missing one. That is safe in both directions:
// installing lands the real CLI in the wizard-owned bin dir, which is prepended to
// PATH and so takes precedence over the impostor.
func (c *Component) Detect(ctx context.Context) (bool, error) {
	if info, err := os.Stat(c.binPath()); err == nil && !info.IsDir() {
		return true, nil
	}
	if c.lookPath != nil {
		if path, err := c.lookPath(render.CLIBinaryName); err == nil {
			return c.isRenderCLI(ctx, path), nil
		}
	}
	return false, nil
}

// isRenderCLI reports whether the executable at path is the Render CLI, by asking
// it for help and looking for the header the CLI prints there.
//
// --help is the probe rather than --version because it is self-contained: the
// CLI's --version also performs an upgrade check against the network, which would
// make detection depend on connectivity, and its output ("render v2.20.0") names
// only the binary — which any program called "render" would also print. The help
// header ("Render CLI v…") is specific to this tool.
//
// This is a heuristic, and deliberately a conservative one: an upstream change to
// that header would cost a redundant install, not a broken one. When no runner is
// wired the probe is unavailable, so the PATH hit is taken at face value.
func (c *Component) isRenderCLI(ctx context.Context, path string) bool {
	if c.runOutput == nil {
		return true
	}
	pctx, cancel := context.WithTimeout(ctx, versionProbeTimeout)
	defer cancel()
	out, err := c.runOutput(pctx, path, "--help")
	if err != nil {
		return false
	}
	// Substring rather than prefix: the CLI styles the header, so escape sequences
	// may surround it, and help output is preceded by nothing else worth matching.
	return strings.Contains(out, renderCLIHelpMarker)
}

// Status returns the current status of the Render CLI component. When detected,
// it attempts to capture the version via `render --version`; if that fails the
// state is still reported as installed with an explanatory detail.
func (c *Component) Status(ctx context.Context) (components.Status, error) {
	detected, err := c.Detect(ctx)
	if err != nil {
		return components.Status{ID: ids.ComponentCLI, State: components.StateUnknown}, err
	}
	if !detected {
		return components.Status{ID: ids.ComponentCLI, State: components.StateNotInstalled}, nil
	}

	status := components.Status{ID: ids.ComponentCLI, State: components.StateInstalled}
	if c.runOutput != nil {
		out, verErr := c.runOutput(ctx, render.CLIBinaryName, "--version")
		if verErr != nil {
			status.Detail = "installed; version could not be determined"
		} else {
			status.Version = strings.TrimSpace(out)
		}
	}
	return status, nil
}

// Install installs the Render CLI according to opts. A dry run performs no side
// effects.
//
// It prefers Homebrew when available (`brew install render`); brew manages its
// own prefix, which is already on PATH, so no further wiring is needed there.
// Without Homebrew it downloads the official release archive directly into the
// wizard-owned bin dir (<home>/.render/bin) and ensures that directory is on
// PATH. This is deliberately different from piping the official install script
// to a shell: that script drops non-root installs in ~/.local/bin — outside the
// directory the wizard detects and manages — and only prints PATH guidance that
// a piped run would discard, so `render` can be reported installed yet remain
// unavailable to the wizard's later steps and the user's shell.
//
// If Homebrew is present but the install fails (a stale brew, registry outage,
// or partial local state), the direct release download is tried as a fallback
// rather than failing outright (F36); an aggregate error is returned only when
// both paths fail.
//
// An explicit opts.Version bypasses Homebrew entirely. `brew install render`
// installs whatever the formula currently points at and the formula has no
// versioned variants to pin to, so honoring a pin through brew is impossible —
// taking the release path is the only way a requested version is the version
// actually installed.
func (c *Component) Install(ctx context.Context, opts components.Options) error {
	if opts.DryRun {
		return nil
	}
	if c.lookPath != nil && opts.Version == "" {
		if _, err := c.lookPath("brew"); err == nil {
			bctx, cancel := context.WithTimeout(ctx, brewTimeout)
			brewErr := c.run(bctx, "brew", "install", render.CLIBinaryName)
			cancel()
			if brewErr == nil {
				return nil
			}
			if relErr := c.installFromRelease(ctx, opts); relErr != nil {
				return fmt.Errorf("install render CLI: brew failed (%v); direct download also failed: %w", brewErr, relErr)
			}
			return nil
		}
	}
	return c.installFromRelease(ctx, opts)
}

// installFromRelease downloads the Render CLI release archive into
// <home>/.render/bin/render and makes that directory discoverable on PATH. The
// whole download+verify sequence is bounded by a single deadline (F17).
func (c *Component) installFromRelease(ctx context.Context, opts components.Options) error {
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	version := opts.Version
	if version == "" {
		v, err := c.latestVersion(ctx)
		if err != nil {
			return fmt.Errorf("resolve latest render CLI version: %w", err)
		}
		version = v
	} else if err := render.ValidateVersion(version); err != nil {
		// A --pin-version value is interpolated into the release URL and the
		// archive filename, so it is checked before it can steer either.
		return fmt.Errorf("invalid render CLI version: %w", err)
	}

	archiveURL := render.CLIArchiveURL(version, c.goos, c.goarch)
	archive, err := c.fetch(ctx, archiveURL)
	if err != nil {
		return fmt.Errorf("download render CLI from %s: %w", archiveURL, err)
	}
	// Verify the archive against the release's published checksums before it is
	// unpacked or executed, extending the bootstrap's "every binary verified"
	// guarantee to this nested download.
	if err := c.verifyChecksum(ctx, version, archive); err != nil {
		return err
	}
	binary, err := extractCLIBinary(archive)
	if err != nil {
		return fmt.Errorf("extract render CLI from %s: %w", archiveURL, err)
	}

	dest := c.binPath()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create render CLI dir %s: %w", filepath.Dir(dest), err)
	}
	if err := os.WriteFile(dest, binary, 0o755); err != nil {
		return fmt.Errorf("write render CLI to %s: %w", dest, err)
	}

	// Make the bin dir usable to this process's later steps (e.g. a skills
	// install that shells out to `render`) and to future shells, so the CLI is
	// actually discoverable rather than merely present on disk.
	if c.ensurePath != nil {
		if err := c.ensurePath(filepath.Dir(dest)); err != nil {
			return fmt.Errorf("add %s to PATH: %w", filepath.Dir(dest), err)
		}
	}
	return nil
}

// verifyChecksum downloads the release's SHA-256 checksums file and confirms the
// downloaded archive matches the published digest for its filename. Any missing
// entry or mismatch is a hard error: an unverified archive is never installed.
func (c *Component) verifyChecksum(ctx context.Context, version string, archive []byte) error {
	name := render.CLIArchiveName(version, c.goos, c.goarch)
	sumsURL := render.CLIChecksumsURL(version)
	sums, err := c.fetch(ctx, sumsURL)
	if err != nil {
		return fmt.Errorf("download render CLI checksums from %s: %w", sumsURL, err)
	}
	want, ok := checksumFor(sums, name)
	if !ok {
		return fmt.Errorf("render CLI checksums (%s) list no entry for %s", sumsURL, name)
	}
	sum := sha256.Sum256(archive)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("render CLI checksum mismatch for %s: got %s, want %s", name, got, want)
	}
	return nil
}

// checksumFor returns the hex digest listed for filename in a SHA256SUMS-format
// document (lines of "<hex>␠␠<filename>"; a leading "*" binary marker on the
// name is tolerated).
func checksumFor(sums []byte, filename string) (string, bool) {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		if name == filename {
			return fields[0], true
		}
	}
	return "", false
}

// latestVersion resolves the newest Render CLI release tag by reading where
// render.CLILatestReleaseURL redirects, rather than by querying the GitHub API.
// See that constant for why: the API's unauthenticated rate limit is per source
// IP and shared, so a NAT or CI runner can exhaust it with unrelated traffic and
// leave the wizard unable to resolve a version at all.
func (c *Component) latestVersion(ctx context.Context) (string, error) {
	loc, err := c.redirectTarget(ctx, render.CLILatestReleaseURL)
	if err != nil {
		return "", err
	}
	return tagFromReleaseLocation(loc)
}

// tagFromReleaseLocation extracts the release tag from the Location that
// GitHub's releases/latest alias redirects to (.../releases/tag/<tag>).
//
// The tag is required to be the final path segment of a well-formed release-tag
// URL and is then validated, because it goes on to build the download and
// checksum URLs: a Location that does not have this exact shape means we are not
// reading what we think we are, and guessing at it risks fetching from a path we
// did not intend.
func tagFromReleaseLocation(loc string) (string, error) {
	u, err := url.Parse(loc)
	if err != nil {
		return "", fmt.Errorf("parse latest-release redirect %q: %w", loc, err)
	}
	// u.Path is percent-decoded, so the checks below see the real segments rather
	// than an encoded form that could hide a separator.
	idx := strings.Index(u.Path, render.CLILatestReleaseTagSegment)
	if idx < 0 {
		return "", fmt.Errorf("latest-release redirect %q is not a release-tag URL", loc)
	}
	tag := strings.Trim(u.Path[idx+len(render.CLILatestReleaseTagSegment):], "/")
	if tag == "" {
		return "", fmt.Errorf("latest-release redirect %q carries no tag", loc)
	}
	if err := render.ValidateVersion(tag); err != nil {
		return "", fmt.Errorf("latest-release redirect %q: %w", loc, err)
	}
	return tag, nil
}

// extractCLIBinary returns the CLI executable bytes from a release zip archive.
// The official archive contains the binary named "cli_v*"; we match that first
// and fall back to the first regular file so a future layout change degrades
// gracefully.
//
// The extracted entry is capped at maxArchiveFileBytes. A zip stores the
// decompressed size in its own header, so a small archive can claim — or simply
// deliver — gigabytes of output; the cap is enforced against both the declared
// size and the bytes actually read, since a hostile archive can understate one to
// get past a check on the other.
func extractCLIBinary(zipBytes []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	var chosen *zip.File
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if strings.HasPrefix(path.Base(f.Name), "cli_") {
			chosen = f
			break
		}
		if chosen == nil {
			chosen = f
		}
	}
	if chosen == nil {
		return nil, errors.New("archive contained no files")
	}
	if chosen.UncompressedSize64 > maxArchiveFileBytes {
		return nil, fmt.Errorf("archive entry %s declares %d bytes, exceeding the %d byte limit", chosen.Name, chosen.UncompressedSize64, maxArchiveFileBytes)
	}
	rc, err := chosen.Open()
	if err != nil {
		return nil, fmt.Errorf("open %s in archive: %w", chosen.Name, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, maxArchiveFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s from archive: %w", chosen.Name, err)
	}
	if len(data) > maxArchiveFileBytes {
		return nil, fmt.Errorf("archive entry %s exceeds the %d byte limit", chosen.Name, maxArchiveFileBytes)
	}
	return data, nil
}

// httpFetch GETs rawURL and returns the response body, erroring on non-200 status
// or on a body exceeding maxFetchBytes.
func httpFetch(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: unexpected status %s", rawURL, resp.Status)
	}
	// Read one byte past the cap so an oversized body is detected rather than
	// silently truncated: a truncated archive would fail checksum verification,
	// reporting a corrupt download for what is really a size overrun.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxFetchBytes {
		return nil, fmt.Errorf("GET %s: response exceeds %d byte limit", rawURL, maxFetchBytes)
	}
	return body, nil
}

// httpRedirectTarget issues a HEAD for rawURL with redirect following disabled and
// returns the Location the server points at. It errors unless the response is a
// redirect carrying a Location.
//
// HEAD keeps this to headers only, and http.ErrUseLastResponse is what makes the
// redirect observable: the default client would transparently follow it and hand
// back the destination page, discarding the Location that is the entire point of
// the request.
func httpRedirectTarget(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 300 || resp.StatusCode > 399 {
		return "", fmt.Errorf("HEAD %s: expected a redirect, got %s", rawURL, resp.Status)
	}
	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("HEAD %s: %s carried no Location header", rawURL, resp.Status)
	}
	return loc, nil
}

// ensurePathIn returns an ensurePath function bound to home. It prepends binDir
// to this process's PATH (so subsequent steps can find the CLI) and persists it
// to the user's shell rc for future shells. Persistence is best-effort for
// unknown shells: the binary is already installed and detectable, so an
// unsupported shell must not fail the install.
func ensurePathIn(home string) func(string) error {
	return func(binDir string) error {
		prependProcessPATH(binDir)
		shell := paths.DetectShell()
		if shell == "" {
			return nil
		}
		rc, err := paths.ShellRCFile(shell, home)
		if err != nil {
			return nil
		}
		if _, err := paths.EnsurePATHEntry(rc, shell, binDir); err != nil {
			return err
		}
		return nil
	}
}

// prependProcessPATH puts binDir at the front of this process's PATH unless it
// is already present.
func prependProcessPATH(binDir string) {
	current := os.Getenv("PATH")
	if current == "" {
		_ = os.Setenv("PATH", binDir)
		return
	}
	for _, p := range filepath.SplitList(current) {
		if p == binDir {
			return
		}
	}
	_ = os.Setenv("PATH", binDir+string(os.PathListSeparator)+current)
}

// Uninstall removes the CLI best-effort. It deletes the managed binary at
// <home>/.render/bin/render when present and returns nil when there is nothing
// to do. A Homebrew-installed CLI is not removed here; uninstall it with
// `brew uninstall render`.
func (c *Component) Uninstall(ctx context.Context) error {
	path := c.binPath()
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat render CLI at %s: %w", path, err)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove render CLI at %s: %w", path, err)
	}
	return nil
}

var _ components.Installer = (*Component)(nil)
