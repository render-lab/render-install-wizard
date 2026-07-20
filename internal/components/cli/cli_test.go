package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/render-oss/render-install-wizard/internal/components"
	"github.com/render-oss/render-install-wizard/internal/ids"
	"github.com/render-oss/render-install-wizard/internal/render"
)

// recordedCall captures a single invocation of the fake runner.
type recordedCall struct {
	name string
	args []string
}

// recorder is a fake runner that records the commands it is asked to execute.
type recorder struct {
	calls []recordedCall
	err   error
}

func (r *recorder) run(_ context.Context, name string, args ...string) error {
	r.calls = append(r.calls, recordedCall{name: name, args: args})
	return r.err
}

// lookPathFound returns a lookPath that resolves only the given names.
func lookPathFound(names ...string) func(string) (string, error) {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return func(name string) (string, error) {
		if set[name] {
			return "/fake/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
}

// lookPathNone resolves nothing.
func lookPathNone() func(string) (string, error) {
	return func(string) (string, error) { return "", errors.New("not found") }
}

// writeBin creates <home>/.render/bin/render so filesystem detection succeeds.
func writeBin(t *testing.T, home string) string {
	t.Helper()
	dir := filepath.Join(home, ".render", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, render.CLIBinaryName)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}
	return path
}

// makeCLIZip builds an in-memory zip containing a single file with the given
// name and content, mirroring the official CLI release archive layout.
func makeCLIZip(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// fakeCLIFetch returns a fetch func that serves canned release metadata for the
// GitHub API URL, a SHA256SUMS document (with the correct digest) for the
// checksums URL, and a zip archive for the download URL, appending each
// requested URL to urls so tests can assert what was fetched.
func fakeCLIFetch(t *testing.T, tag, goos, goarch string, binContent []byte, urls *[]string) func(context.Context, string) ([]byte, error) {
	t.Helper()
	zipBytes := makeCLIZip(t, "cli_"+tag, binContent)
	sum := sha256.Sum256(zipBytes)
	sums := hex.EncodeToString(sum[:]) + "  " + render.CLIArchiveName(tag, goos, goarch) + "\n"
	return func(_ context.Context, url string) ([]byte, error) {
		*urls = append(*urls, url)
		switch {
		case strings.Contains(url, "api.github.com"):
			return []byte(`{"tag_name":"` + tag + `"}`), nil
		case strings.Contains(url, "SHA256SUMS"):
			return []byte(sums), nil
		default:
			return zipBytes, nil
		}
	}
}

func TestDetect(t *testing.T) {
	ctx := context.Background()

	t.Run("found on PATH", func(t *testing.T) {
		c := &Component{home: t.TempDir(), lookPath: lookPathFound(render.CLIBinaryName)}
		got, err := c.Detect(ctx)
		if err != nil {
			t.Fatalf("Detect: %v", err)
		}
		if !got {
			t.Fatal("expected detected via PATH")
		}
	})

	t.Run("found in home bin", func(t *testing.T) {
		home := t.TempDir()
		writeBin(t, home)
		c := &Component{home: home, lookPath: lookPathNone()}
		got, err := c.Detect(ctx)
		if err != nil {
			t.Fatalf("Detect: %v", err)
		}
		if !got {
			t.Fatal("expected detected via home bin")
		}
	})

	t.Run("not found", func(t *testing.T) {
		c := &Component{home: t.TempDir(), lookPath: lookPathNone()}
		got, err := c.Detect(ctx)
		if err != nil {
			t.Fatalf("Detect: %v", err)
		}
		if got {
			t.Fatal("expected not detected")
		}
	})
}

func TestInstall(t *testing.T) {
	ctx := context.Background()

	t.Run("dry run does nothing", func(t *testing.T) {
		rec := &recorder{}
		c := &Component{home: t.TempDir(), lookPath: lookPathFound("brew"), run: rec.run}
		if err := c.Install(ctx, components.Options{DryRun: true}); err != nil {
			t.Fatalf("Install: %v", err)
		}
		if len(rec.calls) != 0 {
			t.Fatalf("expected no runner calls, got %v", rec.calls)
		}
	})

	t.Run("brew present", func(t *testing.T) {
		rec := &recorder{}
		c := &Component{home: t.TempDir(), lookPath: lookPathFound("brew"), run: rec.run}
		if err := c.Install(ctx, components.Options{}); err != nil {
			t.Fatalf("Install: %v", err)
		}
		if len(rec.calls) != 1 {
			t.Fatalf("expected 1 call, got %v", rec.calls)
		}
		call := rec.calls[0]
		if call.name != "brew" {
			t.Fatalf("expected brew, got %q", call.name)
		}
		want := []string{"install", render.CLIBinaryName}
		if len(call.args) != len(want) || call.args[0] != want[0] || call.args[1] != want[1] {
			t.Fatalf("expected args %v, got %v", want, call.args)
		}
	})

	t.Run("brew absent downloads and verifies release into owned dir", func(t *testing.T) {
		home := t.TempDir()
		var urls []string
		var pathDir string
		content := []byte("#!/bin/sh\necho render\n")
		c := &Component{
			home:       home,
			goos:       "linux",
			goarch:     "amd64",
			lookPath:   lookPathNone(),
			fetch:      fakeCLIFetch(t, "v9.9.9", "linux", "amd64", content, &urls),
			ensurePath: func(binDir string) error { pathDir = binDir; return nil },
		}
		if err := c.Install(ctx, components.Options{}); err != nil {
			t.Fatalf("Install: %v", err)
		}

		// The binary lands in the wizard-owned dir, executable.
		dest := filepath.Join(home, ".render", "bin", render.CLIBinaryName)
		got, err := os.ReadFile(dest)
		if err != nil {
			t.Fatalf("read installed binary: %v", err)
		}
		if !bytes.Equal(got, content) {
			t.Fatalf("installed content = %q, want %q", got, content)
		}
		info, err := os.Stat(dest)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("binary not executable, mode=%v", info.Mode())
		}

		// PATH wiring targets the wizard-owned bin dir.
		if want := filepath.Dir(dest); pathDir != want {
			t.Fatalf("ensurePath got %q, want %q", pathDir, want)
		}

		// Resolved latest via the API, fetched the linux/amd64 archive, and
		// verified it against the release's immutable-versioned checksums.
		if len(urls) != 3 || !strings.Contains(urls[0], "api.github.com") {
			t.Fatalf("expected API, archive, checksums fetches, got %v", urls)
		}
		if !strings.Contains(urls[1], "download/v9.9.9/cli_9.9.9_linux_amd64.zip") {
			t.Fatalf("archive URL = %q, want versioned cli_9.9.9_linux_amd64.zip", urls[1])
		}
		if !strings.Contains(urls[2], "download/v9.9.9/cli_9.9.9_SHA256SUMS") {
			t.Fatalf("checksums URL = %q, want versioned SHA256SUMS", urls[2])
		}
	})

	t.Run("pinned version skips API lookup", func(t *testing.T) {
		home := t.TempDir()
		var urls []string
		c := &Component{
			home:       home,
			goos:       "darwin",
			goarch:     "arm64",
			lookPath:   lookPathNone(),
			fetch:      fakeCLIFetch(t, "v1.2.3", "darwin", "arm64", []byte("bin"), &urls),
			ensurePath: func(string) error { return nil },
		}
		if err := c.Install(ctx, components.Options{Version: "1.2.3"}); err != nil {
			t.Fatalf("Install: %v", err)
		}
		// Archive then checksums, no API call.
		if len(urls) != 2 {
			t.Fatalf("expected archive + checksums fetch (no API), got %v", urls)
		}
		if !strings.Contains(urls[0], "download/v1.2.3/cli_1.2.3_darwin_arm64.zip") {
			t.Fatalf("archive URL = %q", urls[0])
		}
	})

	t.Run("checksum mismatch is rejected", func(t *testing.T) {
		home := t.TempDir()
		c := &Component{
			home:     home,
			goos:     "linux",
			goarch:   "amd64",
			lookPath: lookPathNone(),
			fetch: func(_ context.Context, url string) ([]byte, error) {
				if strings.Contains(url, "SHA256SUMS") {
					return []byte("deadbeef  " + render.CLIArchiveName("1.0.0", "linux", "amd64") + "\n"), nil
				}
				return makeCLIZip(t, "cli_v1.0.0", []byte("tampered")), nil
			},
			ensurePath: func(string) error { return nil },
		}
		err := c.Install(ctx, components.Options{Version: "1.0.0"})
		if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
			t.Fatalf("expected checksum mismatch error, got %v", err)
		}
		// A rejected archive must never be written to disk.
		if _, statErr := os.Stat(filepath.Join(home, ".render", "bin", render.CLIBinaryName)); !os.IsNotExist(statErr) {
			t.Fatalf("binary should not be installed on checksum failure, stat err=%v", statErr)
		}
	})

	t.Run("fetch error is wrapped", func(t *testing.T) {
		c := &Component{
			home:       t.TempDir(),
			goos:       "linux",
			goarch:     "amd64",
			lookPath:   lookPathNone(),
			fetch:      func(context.Context, string) ([]byte, error) { return nil, errors.New("boom") },
			ensurePath: func(string) error { return nil },
		}
		if err := c.Install(ctx, components.Options{Version: "1.0.0"}); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestUninstall(t *testing.T) {
	ctx := context.Background()

	t.Run("removes existing binary", func(t *testing.T) {
		home := t.TempDir()
		path := writeBin(t, home)
		c := &Component{home: home}
		if err := c.Uninstall(ctx); err != nil {
			t.Fatalf("Uninstall: %v", err)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected binary removed, stat err=%v", err)
		}
	})

	t.Run("absent is no error", func(t *testing.T) {
		c := &Component{home: t.TempDir()}
		if err := c.Uninstall(ctx); err != nil {
			t.Fatalf("Uninstall: %v", err)
		}
	})
}

func TestStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("not installed", func(t *testing.T) {
		c := &Component{home: t.TempDir(), lookPath: lookPathNone()}
		st, err := c.Status(ctx)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if st.State != components.StateNotInstalled {
			t.Fatalf("expected not_installed, got %q", st.State)
		}
	})

	t.Run("installed with version", func(t *testing.T) {
		c := &Component{
			home:     t.TempDir(),
			lookPath: lookPathFound(render.CLIBinaryName),
			runOutput: func(_ context.Context, _ string, _ ...string) (string, error) {
				return "render 2.10.0\n", nil
			},
		}
		st, err := c.Status(ctx)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if st.State != components.StateInstalled {
			t.Fatalf("expected installed, got %q", st.State)
		}
		if st.Version != "render 2.10.0" {
			t.Fatalf("expected trimmed version, got %q", st.Version)
		}
	})

	t.Run("installed but version fails", func(t *testing.T) {
		c := &Component{
			home:     t.TempDir(),
			lookPath: lookPathFound(render.CLIBinaryName),
			runOutput: func(_ context.Context, _ string, _ ...string) (string, error) {
				return "", errors.New("boom")
			},
		}
		st, err := c.Status(ctx)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if st.State != components.StateInstalled {
			t.Fatalf("expected installed, got %q", st.State)
		}
		if st.Version != "" {
			t.Fatalf("expected empty version, got %q", st.Version)
		}
		if st.Detail == "" {
			t.Fatal("expected a detail note")
		}
	})
}

func TestID(t *testing.T) {
	if got := New().ID(); got != ids.ComponentCLI {
		t.Fatalf("expected %q, got %q", ids.ComponentCLI, got)
	}
}
