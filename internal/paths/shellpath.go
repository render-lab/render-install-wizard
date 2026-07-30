package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
// zsh -> ~/.zshrc, bash -> ~/.bashrc (~/.bash_profile on macOS),
// fish -> ~/.config/fish/config.fish. It returns an error for unsupported shells.
//
// The bash split is deliberate. bash reads ~/.bashrc only for interactive
// non-login shells, and macOS Terminal starts each shell as a *login* shell, which
// reads ~/.bash_profile instead — so on macOS a PATH entry written to ~/.bashrc
// would never take effect in the user's own terminal.
func ShellRCFile(shell, home string) (string, error) {
	return shellRCFile(shell, home, runtime.GOOS)
}

func shellRCFile(shell, home, goos string) (string, error) {
	switch shell {
	case "zsh":
		return filepath.Join(home, ".zshrc"), nil
	case "bash":
		if goos == "darwin" {
			return filepath.Join(home, ".bash_profile"), nil
		}
		return filepath.Join(home, ".bashrc"), nil
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish"), nil
	default:
		return "", fmt.Errorf("paths: unsupported shell %q", shell)
	}
}

// EnsurePATHEntry idempotently ensures binDir is on PATH in rcFile using a
// marked block delimited by pathBlockStart/pathBlockEnd. It creates the file (and
// parent directories) as needed.
//
// When no block exists the block is appended and changed=true. When a block
// already exists its contents are compared against what binDir requires: an
// identical block is left untouched (changed=false), and a differing one — a block
// naming some other directory, say from an earlier install that used a different
// location — is rewritten in place so a stale entry is repaired rather than
// preserved forever. Only the marked region is ever touched; content outside it is
// copied through verbatim.
//
// The syntax is shell-specific: fish uses fish_add_path, while zsh/bash use an
// exported PATH assignment.
func EnsurePATHEntry(rcFile, shell, binDir string) (changed bool, err error) {
	existing, err := os.ReadFile(rcFile)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	block, err := pathBlock(shell, binDir)
	if err != nil {
		return false, err
	}

	updated, found := replacePathBlock(string(existing), block)
	if found && updated == string(existing) {
		return false, nil
	}
	if !found {
		var buf strings.Builder
		buf.Write(existing)
		if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
			buf.WriteByte('\n')
		}
		buf.WriteString(block)
		updated = buf.String()
	}

	if err := os.MkdirAll(filepath.Dir(rcFile), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(rcFile, []byte(updated), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// replacePathBlock swaps the wizard-owned marked region in content for block,
// reporting whether such a region was found. A start marker with no matching end
// marker is treated as running to the end of the file, so a truncated block is
// repaired rather than duplicated.
func replacePathBlock(content, block string) (updated string, found bool) {
	start := strings.Index(content, pathBlockStart)
	if start < 0 {
		return content, false
	}
	rest := content[start:]
	end := strings.Index(rest, pathBlockEnd)
	if end < 0 {
		return content[:start] + block, true
	}
	tail := rest[end+len(pathBlockEnd):]
	// The block already carries its own trailing newline; drop the one that
	// followed the end marker so replacement does not accumulate blank lines.
	tail = strings.TrimPrefix(tail, "\n")
	return content[:start] + block + tail, true
}

// pathBlock builds the marked PATH block for the given shell and binDir.
func pathBlock(shell, binDir string) (string, error) {
	var line string
	switch shell {
	case "fish":
		// Quoted: an unquoted path splits on whitespace, so a home directory
		// containing a space would add two wrong entries instead of one right one.
		line = fmt.Sprintf("fish_add_path %s", fishQuote(binDir))
	case "zsh", "bash":
		// One double-quoted string: $PATH stays expandable while binDir is
		// protected from word splitting and globbing, and escaping binDir keeps a
		// literal $ or " in the path from being reinterpreted.
		line = `export PATH="` + escapeForDoubleQuotes(binDir) + `:$PATH"`
	default:
		return "", fmt.Errorf("paths: unsupported shell %q", shell)
	}
	return fmt.Sprintf("%s\n%s\n%s\n", pathBlockStart, line, pathBlockEnd), nil
}

// fishQuote renders s as a fish single-quoted string. fish only honors \' and \\
// inside single quotes, so escaping those two is sufficient and everything else
// (including $) is literal.
func fishQuote(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	return "'" + r.Replace(s) + "'"
}

// escapeForDoubleQuotes escapes the four characters a POSIX shell still
// interprets inside double quotes: \, ", ` and $.
func escapeForDoubleQuotes(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", `$`, `\$`)
	return r.Replace(s)
}
