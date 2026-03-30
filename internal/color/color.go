// Package color provides ANSI color utilities for terminal output.
// Colors are applied only when stdout is a TTY and NO_COLOR is not set.
package color

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
)

// isTerminal reports whether the given file is connected to a terminal.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// Code represents an ANSI SGR (Select Graphic Rendition) parameter.
type Code int

const (
	Reset  Code = 0
	Bold   Code = 1
	Dim    Code = 2
	Red    Code = 31
	Green  Code = 32
	Yellow Code = 33
	Cyan   Code = 36
)

var (
	enabled    atomic.Bool
	initOnce   sync.Once
	overridden atomic.Bool
)

// Enabled reports whether color output should be used.
// Returns true when stdout is a TTY, NO_COLOR is not set, and TERM is not "dumb".
func Enabled() bool {
	initOnce.Do(func() {
		if overridden.Load() {
			return
		}
		if os.Getenv("NO_COLOR") != "" {
			return
		}
		if os.Getenv("TERM") == "dumb" {
			return
		}
		enabled.Store(isTerminal(os.Stdout))
	})
	return enabled.Load()
}

// SetEnabled explicitly enables or disables color output,
// overriding automatic TTY detection.
func SetEnabled(v bool) {
	overridden.Store(true)
	enabled.Store(v)
	initOnce.Do(func() {}) // mark as done so Enabled() won't re-detect
}

// Colorize wraps text with the given ANSI code if color is enabled.
func Colorize(text string, code Code) string {
	if !Enabled() {
		return text
	}
	return fmt.Sprintf("\033[%dm%s\033[0m", code, text)
}

// Convenience functions for common colors.

func RedText(s string) string    { return Colorize(s, Red) }
func GreenText(s string) string  { return Colorize(s, Green) }
func YellowText(s string) string { return Colorize(s, Yellow) }
func CyanText(s string) string   { return Colorize(s, Cyan) }
