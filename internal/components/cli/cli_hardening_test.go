package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/render-lab/render-install-wizard/internal/components"
	"github.com/render-lab/render-install-wizard/internal/render"
)

// TestTagFromReleaseLocation covers parsing the release tag out of the Location
// that GitHub's releases/latest alias returns. The tag is interpolated into the
// archive and checksum URLs, so anything that is not an unambiguous release-tag
// URL has to be rejected rather than guessed at.
func TestTagFromReleaseLocation(t *testing.T) {
	base := "https://github.com/" + render.CLIRepo

	t.Run("accepts real GitHub locations", func(t *testing.T) {
		for _, tc := range []struct{ loc, want string }{
			{base + "/releases/tag/v2.22.0", "v2.22.0"},
			// Prerelease and build-metadata tags are legitimate semver.
			{base + "/releases/tag/v2.23.0-rc.1", "v2.23.0-rc.1"},
			{base + "/releases/tag/v1.0.0+build.7", "v1.0.0+build.7"},
			// No "v" prefix, and a trailing slash, are both still resolvable.
			{base + "/releases/tag/2.22.0", "2.22.0"},
			{base + "/releases/tag/v2.22.0/", "v2.22.0"},
			// A relative Location is legal per RFC 7231 even though GitHub sends
			// absolute ones today.
			{"/render-oss/cli/releases/tag/v3.0.0", "v3.0.0"},
		} {
			got, err := tagFromReleaseLocation(tc.loc)
			if err != nil {
				t.Errorf("tagFromReleaseLocation(%q): %v", tc.loc, err)
				continue
			}
			if got != tc.want {
				t.Errorf("tagFromReleaseLocation(%q) = %q, want %q", tc.loc, got, tc.want)
			}
		}
	})

	t.Run("rejects locations that are not a release tag", func(t *testing.T) {
		for _, loc := range []string{
			"",
			base + "/releases",
			base + "/releases/latest",
			// Landing on the releases index means no release exists; inventing a
			// tag from the last path segment would fetch a nonexistent asset.
			base + "/releases/tag/",
			// Traversal, encoded or not, would retarget the download URL at a
			// different path on the release host.
			base + "/releases/tag/../../evil",
			base + "/releases/tag/%2e%2e%2fevil",
			base + "/releases/tag/v1.0.0/../../../evil",
			// A separator inside the tag is the same problem by another route.
			base + "/releases/tag/a/b",
		} {
			if got, err := tagFromReleaseLocation(loc); err == nil {
				t.Errorf("tagFromReleaseLocation(%q) = %q, want an error", loc, got)
			}
		}
	})
}

// TestValidateVersionRejectsPathSteering guards the other entry point for a
// version: --pin-version. It reaches the same URL construction as a resolved tag,
// so a value carrying separators or dot segments must not get that far.
func TestValidateVersionRejectsPathSteering(t *testing.T) {
	for _, v := range []string{"1.2.3", "v1.2.3", "v1.2.3-rc.1", "v1.0.0+build.7"} {
		if err := render.ValidateVersion(v); err != nil {
			t.Errorf("ValidateVersion(%q) = %v, want nil", v, err)
		}
	}
	for _, v := range []string{"", "../../etc", "1.2.3/../../x", "v1 2", "v1.2.3?x=1", "v1.2.3#f", "v1.2.3%2f"} {
		if err := render.ValidateVersion(v); err == nil {
			t.Errorf("ValidateVersion(%q) = nil, want an error", v)
		}
	}
}

// TestInstallRejectsUnsafePinnedVersion checks the validation is actually wired
// into the install path, not merely available: a traversal pin must fail before
// any network request is made.
func TestInstallRejectsUnsafePinnedVersion(t *testing.T) {
	c := &Component{
		home:     t.TempDir(),
		goos:     "linux",
		goarch:   "amd64",
		lookPath: lookPathNone(),
		fetch: func(context.Context, string) ([]byte, error) {
			t.Error("an invalid version must be rejected before any download")
			return nil, errors.New("unexpected fetch")
		},
		ensurePath: func(string) error { return nil },
	}
	err := c.Install(context.Background(), components.Options{Version: "../../../../evil"})
	if err == nil || !strings.Contains(err.Error(), "invalid render CLI version") {
		t.Fatalf("err = %v, want an invalid-version error", err)
	}
}

// zeros is an endless source of NUL bytes, used to build a highly compressible
// payload without allocating it.
type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

// TestExtractCLIBinaryRejectsCompressionBomb covers the zip size cap with an
// actual bomb: an archive of a few dozen KB that expands past the limit. This is
// the property that matters, since the archive's size on the wire says nothing
// about how much memory unpacking it will demand.
func TestExtractCLIBinaryRejectsCompressionBomb(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("cli_v1.0.0")
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	if _, err := io.CopyN(w, zeros{}, maxArchiveFileBytes+1); err != nil {
		t.Fatalf("write bomb payload: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	if buf.Len() >= maxArchiveFileBytes {
		t.Fatalf("payload did not compress, archive is %d bytes", buf.Len())
	}

	if _, err := extractCLIBinary(buf.Bytes()); err == nil {
		t.Fatal("expected the oversized entry to be rejected")
	} else if !strings.Contains(err.Error(), "limit") {
		t.Fatalf("err = %v, want a size-limit error", err)
	}

	// The cap must not interfere with a normal release archive.
	if _, err := extractCLIBinary(makeCLIZip(t, "cli_v1.0.0", []byte("#!/bin/sh\n"))); err != nil {
		t.Fatalf("an honestly-sized archive must still extract: %v", err)
	}
}

// TestHTTPFetchRejectsOversizedBody exercises the response cap against a real
// server that streams past the limit, which is the shape of the problem: the
// body has no declared length, so only a bounded read stops it.
func TestHTTPFetchRejectsOversizedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		chunk := bytes.Repeat([]byte("A"), 1<<20)
		for sent := 0; sent <= maxFetchBytes; sent += len(chunk) {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	if _, err := httpFetch(context.Background(), srv.URL); err == nil {
		t.Fatal("expected an oversized body to be rejected")
	} else if !strings.Contains(err.Error(), "limit") {
		t.Fatalf("err = %v, want a size-limit error", err)
	}
}

// TestHTTPRedirectTargetReadsLocation checks the redirect is observed rather than
// followed. The default client would follow it and return the destination, at
// which point the Location -- the only thing we want -- is gone.
func TestHTTPRedirectTargetReadsLocation(t *testing.T) {
	t.Run("returns the Location of a redirect", func(t *testing.T) {
		want := "https://github.com/render-oss/cli/releases/tag/v2.22.0"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Location", want)
			w.WriteHeader(http.StatusFound)
		}))
		defer srv.Close()

		got, err := httpRedirectTarget(context.Background(), srv.URL)
		if err != nil {
			t.Fatalf("httpRedirectTarget: %v", err)
		}
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("errors when the response is not a redirect", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		if _, err := httpRedirectTarget(context.Background(), srv.URL); err == nil {
			t.Fatal("expected an error when no redirect is returned")
		}
	})
}

// TestDetectRejectsForeignRenderBinary covers the identity probe. "render" is a
// plausible name for an unrelated program, and accepting one would make the wizard
// skip installing the real CLI while reporting the component present.
func TestDetectRejectsForeignRenderBinary(t *testing.T) {
	ctx := context.Background()

	t.Run("a different program named render is not the CLI", func(t *testing.T) {
		c := &Component{
			home:     t.TempDir(),
			lookPath: lookPathFound(render.CLIBinaryName),
			runOutput: func(context.Context, string, ...string) (string, error) {
				// A template renderer, say. Note it prints its own name, which is
				// why matching on "render" alone would not tell these apart.
				return "render 1.4.0\nusage: render [options] <template>\n", nil
			},
		}
		got, err := c.Detect(ctx)
		if err != nil {
			t.Fatalf("Detect: %v", err)
		}
		if got {
			t.Fatal("a foreign binary named render must not count as the Render CLI")
		}
	})

	t.Run("a binary that will not answer is not the CLI", func(t *testing.T) {
		c := &Component{
			home:     t.TempDir(),
			lookPath: lookPathFound(render.CLIBinaryName),
			runOutput: func(context.Context, string, ...string) (string, error) {
				return "", errors.New("exec format error")
			},
		}
		got, err := c.Detect(ctx)
		if err != nil {
			t.Fatalf("Detect: %v", err)
		}
		if got {
			t.Fatal("an unprobeable binary must resolve to not-installed")
		}
	})

	t.Run("the real CLI is accepted", func(t *testing.T) {
		c := &Component{
			home:      t.TempDir(),
			lookPath:  lookPathFound(render.CLIBinaryName),
			runOutput: fakeRenderCLI("render v2.10.0\n", nil),
		}
		got, err := c.Detect(ctx)
		if err != nil {
			t.Fatalf("Detect: %v", err)
		}
		if !got {
			t.Fatal("the real Render CLI must be detected")
		}
	})

	t.Run("the managed binary is trusted without probing", func(t *testing.T) {
		home := t.TempDir()
		writeBin(t, home)
		c := &Component{
			home:     home,
			lookPath: lookPathNone(),
			runOutput: func(context.Context, string, ...string) (string, error) {
				t.Error("the wizard's own binary must not need a probe")
				return "", errors.New("unexpected probe")
			},
		}
		got, err := c.Detect(ctx)
		if err != nil {
			t.Fatalf("Detect: %v", err)
		}
		if !got {
			t.Fatal("the managed binary must be detected")
		}
	})
}

// TestLatestVersionUsesTheRedirectNotTheAPI guards the rate-limit fix at the
// level that matters: which host is contacted to resolve a version.
func TestLatestVersionUsesTheRedirectNotTheAPI(t *testing.T) {
	var asked string
	c := &Component{
		redirectTarget: func(_ context.Context, url string) (string, error) {
			asked = url
			return "https://github.com/" + render.CLIRepo + "/releases/tag/v4.5.6", nil
		},
		fetch: func(context.Context, string) ([]byte, error) {
			t.Error("resolving a version must not fetch anything")
			return nil, errors.New("unexpected fetch")
		},
	}
	got, err := c.latestVersion(context.Background())
	if err != nil {
		t.Fatalf("latestVersion: %v", err)
	}
	if got != "v4.5.6" {
		t.Fatalf("version = %q, want v4.5.6", got)
	}
	if asked != render.CLILatestReleaseURL {
		t.Fatalf("resolved against %q, want %q", asked, render.CLILatestReleaseURL)
	}
	if strings.Contains(asked, "api.github.com") {
		t.Fatalf("must not use the rate-limited API host: %q", asked)
	}
}
