package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// pathBlockStart and pathBlockEnd delimit the marked region the wizard manages
// in a shell rc file. Everything between them is owned by the wizard; content
// outside is never touched.
const (
	pathBlockStart = "# >>> render >>>"
	pathBlockEnd   = "# <<< render <<<"
)

// DetectShell returns the basename of the $SHELL environment variable
// (e.g. "zsh", "bash", "fish"). It returns "" when $SHELL is unset.
func DetectShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return ""
	}
	return filepath.Base(shell)
}

// ShellRCFile returns the rc file path for the given shell within home:
// zsh -> ~/.zshrc, bash -> ~/.bashrc, fish -> ~/.config/fish/config.fish.
// It returns an error for unsupported shells.
func ShellRCFile(shell, home string) (string, error) {
	switch shell {
	case "zsh":
		return filepath.Join(home, ".zshrc"), nil
	case "bash":
		return filepath.Join(home, ".bashrc"), nil
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish"), nil
	default:
		return "", fmt.Errorf("paths: unsupported shell %q", shell)
	}
}

// EnsurePATHEntry idempotently ensures binDir is on PATH in rcFile using a
// marked block delimited by pathBlockStart/pathBlockEnd. If the block already
// exists, it returns changed=false and leaves the file untouched. Otherwise it
// creates the file (and parent directories) as needed, appends the block, and
// returns changed=true. The syntax is shell-specific: fish uses
// fish_add_path, while zsh/bash use an exported PATH assignment.
func EnsurePATHEntry(rcFile, shell, binDir string) (changed bool, err error) {
	existing, err := os.ReadFile(rcFile)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if strings.Contains(string(existing), pathBlockStart) {
		return false, nil
	}

	block, err := pathBlock(shell, binDir)
	if err != nil {
		return false, err
	}

	if err := os.MkdirAll(filepath.Dir(rcFile), 0o755); err != nil {
		return false, err
	}

	var buf strings.Builder
	buf.Write(existing)
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		buf.WriteByte('\n')
	}
	buf.WriteString(block)

	if err := os.WriteFile(rcFile, []byte(buf.String()), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// pathBlock builds the marked PATH block for the given shell and binDir.
func pathBlock(shell, binDir string) (string, error) {
	var line string
	switch shell {
	case "fish":
		line = fmt.Sprintf("fish_add_path %s", binDir)
	case "zsh", "bash":
		line = fmt.Sprintf("export PATH=%q", binDir+":$PATH")
	default:
		return "", fmt.Errorf("paths: unsupported shell %q", shell)
	}
	return fmt.Sprintf("%s\n%s\n%s\n", pathBlockStart, line, pathBlockEnd), nil
}
