//go:build unix

package configedit

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestAtomicWritePermissions guards F07: new config files and the directories
// the wizard creates must be private, regardless of the process umask, and
// updating an existing file must never broaden its mode.
func TestAtomicWritePermissions(t *testing.T) {
	// Force a permissive umask so any reliance on default 0644/0755 modes would
	// surface as world/group-readable output; our explicit modes must win.
	old := syscall.Umask(0)
	defer syscall.Umask(old)

	t.Run("new file is 0600 and created dir is 0700", func(t *testing.T) {
		base := t.TempDir()
		path := filepath.Join(base, "sub", "config.json")
		if err := SetJSONValue(path, 1, "a"); err != nil {
			t.Fatalf("SetJSONValue: %v", err)
		}

		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := fi.Mode().Perm(); got != 0o600 {
			t.Errorf("new file mode = %o, want 0600", got)
		}

		di, err := os.Stat(filepath.Join(base, "sub"))
		if err != nil {
			t.Fatal(err)
		}
		if got := di.Mode().Perm(); got != 0o700 {
			t.Errorf("created dir mode = %o, want 0700", got)
		}
	})

	t.Run("existing file mode is preserved, not broadened", func(t *testing.T) {
		base := t.TempDir()
		path := filepath.Join(base, "config.json")
		if err := os.WriteFile(path, []byte(`{"a":1}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := SetJSONValue(path, 2, "b"); err != nil {
			t.Fatalf("SetJSONValue: %v", err)
		}
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := fi.Mode().Perm(); got != 0o600 {
			t.Errorf("updated file mode = %o, want preserved 0600", got)
		}
	})
}
