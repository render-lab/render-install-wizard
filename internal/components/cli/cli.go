// Package cli implements the components.Installer contract for the Render CLI.
package cli

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/render-oss/render-install-wizard/internal/components"
	"github.com/render-oss/render-install-wizard/internal/ids"
	"github.com/render-oss/render-install-wizard/internal/paths"
	"github.com/render-oss/render-install-wizard/internal/render"
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
	// fetch downloads the bytes at a URL (the release metadata and archive).
	// Injectable so tests never hit the network.
	fetch func(ctx context.Context, url string) ([]byte, error)
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
		home:     home,
		goos:     runtime.GOOS,
		goarch:   runtime.GOARCH,
		lookPath: exec.LookPath,
		run: func(ctx context.Context, name string, args ...string) error {
			return exec.CommandContext(ctx, name, args...).Run()
		},
		runOutput: func(ctx context.Context, name string, args ...string) (string, error) {
			var buf bytes.Buffer
			cmd := exec.CommandContext(ctx, name, args...)
			cmd.Stdout = &buf
			cmd.Stderr = &buf
			err := cmd.Run()
			return buf.String(), err
		},
		fetch:      httpFetch,
		ensurePath: ensurePathIn(home),
	}
}

// ID returns the canonical identifier for the Render CLI component.
func (c *Component) ID() ids.ComponentID { return ids.ComponentCLI }

// binPath returns the path to the CLI binary managed under the injectable home:
// <home>/.render/bin/render.
func (c *Component) binPath() string {
	return filepath.Join(c.home, ".render", "bin", render.CLIBinaryName)
}

// Detect reports whether the Render CLI is already installed. It returns true if
// the binary resolves on PATH via lookPath, or if the managed binary exists at
// <home>/.render/bin/render.
func (c *Component) Detect(ctx context.Context) (bool, error) {
	if c.lookPath != nil {
		if _, err := c.lookPath(render.CLIBinaryName); err == nil {
			return true, nil
		}
	}
	if info, err := os.Stat(c.binPath()); err == nil && !info.IsDir() {
		return true, nil
	}
	return false, nil
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
func (c *Component) Install(ctx context.Context, opts components.Options) error {
	if opts.DryRun {
		return nil
	}
	if c.lookPath != nil {
		if _, err := c.lookPath("brew"); err == nil {
			if err := c.run(ctx, "brew", "install", render.CLIBinaryName); err != nil {
				return fmt.Errorf("install render CLI via brew: %w", err)
			}
			return nil
		}
	}
	return c.installFromRelease(ctx, opts)
}

// installFromRelease downloads the Render CLI release archive into
// <home>/.render/bin/render and makes that directory discoverable on PATH.
func (c *Component) installFromRelease(ctx context.Context, opts components.Options) error {
	version := opts.Version
	if version == "" {
		v, err := c.latestVersion(ctx)
		if err != nil {
			return fmt.Errorf("resolve latest render CLI version: %w", err)
		}
		version = v
	}

	url := render.CLIArchiveURL(version, c.goos, c.goarch)
	archive, err := c.fetch(ctx, url)
	if err != nil {
		return fmt.Errorf("download render CLI from %s: %w", url, err)
	}
	// Verify the archive against the release's published checksums before it is
	// unpacked or executed, extending the bootstrap's "every binary verified"
	// guarantee to this nested download.
	if err := c.verifyChecksum(ctx, version, archive); err != nil {
		return err
	}
	binary, err := extractCLIBinary(archive)
	if err != nil {
		return fmt.Errorf("extract render CLI from %s: %w", url, err)
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

// latestVersion resolves the newest Render CLI release tag from the GitHub API.
func (c *Component) latestVersion(ctx context.Context) (string, error) {
	body, err := c.fetch(ctx, render.CLILatestReleaseAPIURL)
	if err != nil {
		return "", err
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return "", fmt.Errorf("parse release metadata: %w", err)
	}
	if rel.TagName == "" {
		return "", errors.New("release metadata missing tag_name")
	}
	return rel.TagName, nil
}

// extractCLIBinary returns the CLI executable bytes from a release zip archive.
// The official archive contains the binary named "cli_v*"; we match that first
// and fall back to the first regular file so a future layout change degrades
// gracefully.
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
	rc, err := chosen.Open()
	if err != nil {
		return nil, fmt.Errorf("open %s in archive: %w", chosen.Name, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read %s from archive: %w", chosen.Name, err)
	}
	return data, nil
}

// httpFetch GETs url and returns the response body, erroring on non-200 status.
func httpFetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: unexpected status %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
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
