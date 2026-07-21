// Package execx runs external commands with combined-output capture, surfacing
// that output in errors so a failed install carries an actionable cause (a
// missing unzip, PATH guidance, npm diagnostics, an unexpected prompt) instead
// of a bare exit status. Callers bound each command with a context deadline.
package execx

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// maxOutput caps how much captured output is embedded in an error. Command
// errors usually appear at the end, so the tail is kept.
const maxOutput = 2000

// Run executes name with args, capturing combined stdout+stderr. On success the
// output is discarded and nil is returned. On failure the returned error wraps
// the underlying error together with the trimmed, tail-capped output.
func Run(ctx context.Context, name string, args ...string) error {
	out, err := CombinedOutput(ctx, name, args...)
	if err == nil {
		return nil
	}
	if trimmed := strings.TrimSpace(out); trimmed != "" {
		return fmt.Errorf("%w: %s", err, tail(trimmed, maxOutput))
	}
	return err
}

// CombinedOutput executes name with args and returns its combined stdout+stderr
// together with any error. It is used where the caller needs the output itself
// (e.g. reading `render --version`).
func CombinedOutput(ctx context.Context, name string, args ...string) (string, error) {
	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

// tail returns at most max trailing bytes of s, prefixed with an ellipsis when
// truncated.
func tail(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return "…" + s[len(s)-max:]
}
