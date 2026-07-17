// Package logx provides a minimal leveled logger supporting text and JSON output.
package logx

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Logger writes leveled log lines to an io.Writer in either text or JSON mode.
type Logger struct {
	w    io.Writer
	json bool
}

// New returns a Logger that writes to w. When jsonMode is true, each line is a
// JSON object; otherwise output is "LEVEL: msg".
func New(w io.Writer, jsonMode bool) *Logger {
	return &Logger{w: w, json: jsonMode}
}

// Infof logs a formatted message at info level.
func (l *Logger) Infof(format string, args ...any) {
	l.log("info", fmt.Sprintf(format, args...))
}

// Warnf logs a formatted message at warn level.
func (l *Logger) Warnf(format string, args ...any) {
	l.log("warn", fmt.Sprintf(format, args...))
}

// Errorf logs a formatted message at error level.
func (l *Logger) Errorf(format string, args ...any) {
	l.log("error", fmt.Sprintf(format, args...))
}

func (l *Logger) log(level, msg string) {
	if l.json {
		b, err := json.Marshal(struct {
			Level string `json:"level"`
			Msg   string `json:"msg"`
		}{Level: level, Msg: msg})
		if err != nil {
			return
		}
		_, _ = l.w.Write(append(b, '\n'))
		return
	}
	_, _ = fmt.Fprintf(l.w, "%s: %s\n", strings.ToUpper(level), msg)
}
