//go:build unix

package execx

import (
	"context"
	"strings"
	"testing"
)

func TestRunSuccessDiscardsOutput(t *testing.T) {
	if err := Run(context.Background(), "sh", "-c", "echo hello"); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// TestRunFailureSurfacesOutput guards F17: a failed command's output is embedded
// in the error so the cause isn't hidden behind a bare exit status.
func TestRunFailureSurfacesOutput(t *testing.T) {
	err := Run(context.Background(), "sh", "-c", "echo NEEDS_UNZIP >&2; exit 3")
	if err == nil {
		t.Fatal("expected error for nonzero exit")
	}
	if !strings.Contains(err.Error(), "NEEDS_UNZIP") {
		t.Errorf("error should surface subprocess output, got %q", err.Error())
	}
}

func TestCombinedOutputReturnsOutput(t *testing.T) {
	out, err := CombinedOutput(context.Background(), "sh", "-c", "echo captured")
	if err != nil {
		t.Fatalf("CombinedOutput: %v", err)
	}
	if !strings.Contains(out, "captured") {
		t.Errorf("output = %q, want it to contain 'captured'", out)
	}
}

// TestRunHonorsContextCancellation guards F17's deadline behavior: a cancelled
// context stops the command.
func TestRunHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Run(ctx, "sh", "-c", "sleep 30"); err == nil {
		t.Fatal("expected error from a cancelled context")
	}
}
