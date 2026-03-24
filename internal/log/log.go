package log

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

var Verbose bool

// Logger provides structured logging with an optional file writer.
type Logger struct {
	mu          sync.Mutex
	logFile     io.Writer
	stdout      io.Writer              // optional; defaults to os.Stdout when nil
	colorizeLog func(string) string    // optional; applies color to stdout copy only
}

// NewLogger creates a Logger that writes to the given writer (in addition to stdout).
func NewLogger(w io.Writer) *Logger {
	return &Logger{logFile: w}
}

func (l *Logger) SetLogFile(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.logFile = w
}

// SetStdout overrides the default stdout destination for log output.
// Pass nil to revert to os.Stdout.
func (l *Logger) SetStdout(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stdout = w
}

func (l *Logger) resolveStdout() io.Writer {
	if l.stdout != nil {
		return l.stdout
	}
	return os.Stdout
}

// SetColorize sets a function that applies color to lines written to stdout.
// The log file always receives plain (uncolored) text.
func (l *Logger) SetColorize(fn func(string) string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.colorizeLog = fn
}

func (l *Logger) Log(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), message)

	l.mu.Lock()
	defer l.mu.Unlock()

	stdoutLine := line
	if l.colorizeLog != nil {
		stdoutLine = l.colorizeLog(line)
	}
	_, _ = io.WriteString(l.resolveStdout(), stdoutLine)
	if l.logFile != nil {
		if _, err := io.WriteString(l.logFile, line); err != nil {
			fmt.Fprintf(os.Stderr, "fry: log write failed: %v\n", err)
		}
	}
}

func (l *Logger) AgentBanner(sprintNum, totalSprints int, sprintName string, iter, maxIter int, engine, model string) {
	if model == "" {
		model = "default"
	}
	banner := fmt.Sprintf(
		"▶ AGENT  Sprint %d/%d \"%s\"  iter %d/%d  engine=%s  model=%s",
		sprintNum,
		totalSprints,
		sprintName,
		iter,
		maxIter,
		engine,
		model,
	)
	l.Log("%s", banner)
}

// Package-level convenience functions — delegate to defaultLogger.

var defaultLogger = &Logger{}

func SetLogFile(w io.Writer) {
	defaultLogger.SetLogFile(w)
}

func SetStdout(w io.Writer) {
	defaultLogger.SetStdout(w)
}

func SetColorize(fn func(string) string) {
	defaultLogger.SetColorize(fn)
}

func Log(format string, args ...interface{}) {
	defaultLogger.Log(format, args...)
}

func AgentBanner(sprintNum, totalSprints int, sprintName string, iter, maxIter int, engine, model string) {
	defaultLogger.AgentBanner(sprintNum, totalSprints, sprintName, iter, maxIter, engine, model)
}
