package monitor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/yevgetman/fry/internal/agent"
	"github.com/yevgetman/fry/internal/config"
)

// Source reads a single data artifact and reports whether it changed since the
// last read. Implementations must be safe for sequential calls from Monitor.
type Source interface {
	Name() string
	Poll() (changed bool, err error)
}

// ---------------------------------------------------------------------------
// EventSource — tracks .fry/observer/events.jsonl via byte offset
// ---------------------------------------------------------------------------

// EventSource polls the observer events file and tracks new events via
// byte-offset seeking, avoiding full re-reads on each poll.
type EventSource struct {
	path      string
	offset    int64
	events    []agent.BuildEvent
	newEvents []agent.BuildEvent
}

// NewEventSource creates an EventSource for the given project directory.
func NewEventSource(projectDir string) *EventSource {
	return &EventSource{
		path: filepath.Join(projectDir, config.ObserverEventsFile),
	}
}

func (s *EventSource) Name() string { return "events" }

func (s *EventSource) Poll() (bool, error) {
	s.newEvents = nil

	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("event source: open: %w", err)
	}
	defer f.Close()

	if s.offset > 0 {
		if _, err := f.Seek(s.offset, 0); err != nil {
			return false, fmt.Errorf("event source: seek: %w", err)
		}
	}

	reader := bufio.NewReader(f)
	var found bool
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			s.offset += int64(len(line))
			trimmed := bytes.TrimSpace(line)
			if len(trimmed) == 0 {
				continue
			}
			var evt agent.BuildEvent
			if jsonErr := json.Unmarshal(trimmed, &evt); jsonErr != nil {
				continue
			}
			s.events = append(s.events, evt)
			s.newEvents = append(s.newEvents, evt)
			found = true
		}
		if readErr != nil {
			break
		}
	}
	return found, nil
}

// Events returns all events read so far.
func (s *EventSource) Events() []agent.BuildEvent { return s.events }

// NewEvents returns events read in the most recent Poll call.
func (s *EventSource) NewEvents() []agent.BuildEvent { return s.newEvents }

// ---------------------------------------------------------------------------
// PhaseSource — tracks .fry/build-phase.txt via mtime
// ---------------------------------------------------------------------------

// PhaseSource polls the build phase file and detects changes via mtime.
type PhaseSource struct {
	paths   []string
	phase   string
	lastMod time.Time
}

// NewPhaseSource creates a PhaseSource checking the given directories.
func NewPhaseSource(dirs ...string) *PhaseSource {
	paths := make([]string, len(dirs))
	for i, dir := range dirs {
		paths[i] = filepath.Join(dir, config.BuildPhaseFile)
	}
	return &PhaseSource{paths: paths}
}

func (s *PhaseSource) Name() string { return "phase" }

func (s *PhaseSource) Poll() (bool, error) {
	for _, path := range s.paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.ModTime().Equal(s.lastMod) && s.phase != "" {
			return false, nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		phase := strings.TrimSpace(string(data))
		if phase == s.phase {
			s.lastMod = info.ModTime()
			return false, nil
		}
		s.phase = phase
		s.lastMod = info.ModTime()
		return true, nil
	}
	return false, nil
}

// Phase returns the current build phase.
func (s *PhaseSource) Phase() string { return s.phase }

// ---------------------------------------------------------------------------
// StatusSource — tracks .fry/build-status.json via mtime
// ---------------------------------------------------------------------------

// StatusSource polls the build status JSON file and detects changes via mtime.
type StatusSource struct {
	path    string
	status  *agent.BuildStatus
	lastMod time.Time
	changed bool
}

// NewStatusSource creates a StatusSource for the given build directory.
func NewStatusSource(buildDir string) *StatusSource {
	return &StatusSource{
		path: filepath.Join(buildDir, config.BuildStatusFile),
	}
}

func (s *StatusSource) Name() string { return "status" }

func (s *StatusSource) Poll() (bool, error) {
	s.changed = false
	info, err := os.Stat(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("status source: stat: %w", err)
	}
	if info.ModTime().Equal(s.lastMod) && s.status != nil {
		return false, nil
	}
	s.lastMod = info.ModTime()

	data, err := os.ReadFile(s.path)
	if err != nil {
		return false, fmt.Errorf("status source: read: %w", err)
	}
	var st agent.BuildStatus
	if err := json.Unmarshal(data, &st); err != nil {
		return false, fmt.Errorf("status source: parse: %w", err)
	}
	s.status = &st
	s.changed = true
	return true, nil
}

// Status returns the last read BuildStatus (may be nil).
func (s *StatusSource) Status() *agent.BuildStatus { return s.status }

// Changed reports whether the status changed in the most recent Poll.
func (s *StatusSource) Changed() bool { return s.changed }

// ---------------------------------------------------------------------------
// LockSource — tracks .fry/.fry.lock for process liveness
// ---------------------------------------------------------------------------

// LockSource polls the lock file and checks if the owning process is alive.
type LockSource struct {
	projectDir string
	active     bool
	pid        int
}

// NewLockSource creates a LockSource for the given project directory.
func NewLockSource(projectDir string) *LockSource {
	return &LockSource{projectDir: projectDir}
}

func (s *LockSource) Name() string { return "lock" }

func (s *LockSource) Poll() (bool, error) {
	lockPath := filepath.Join(s.projectDir, config.LockFile)
	data, err := os.ReadFile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			prev := s.active
			s.active = false
			s.pid = 0
			return prev, nil // changed if it was previously active
		}
		return false, fmt.Errorf("lock source: read: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		prev := s.active
		s.active = false
		s.pid = 0
		return prev, nil
	}

	alive := syscall.Kill(pid, 0) == nil
	prevActive := s.active
	s.active = alive
	s.pid = pid
	return alive != prevActive, nil
}

// Active reports whether the build process is alive.
func (s *LockSource) Active() bool { return s.active }

// PID returns the build process PID (0 if not locked).
func (s *LockSource) PID() int { return s.pid }

// ---------------------------------------------------------------------------
// ProgressSource — tracks sprint-progress.txt or epic-progress.txt via size
// ---------------------------------------------------------------------------

// ProgressSource polls a progress file and detects changes via file size.
type ProgressSource struct {
	path     string
	content  string
	lastSize int64
}

// NewProgressSource creates a ProgressSource for the given file path.
func NewProgressSource(path string) *ProgressSource {
	return &ProgressSource{path: path}
}

func (s *ProgressSource) Name() string {
	return "progress:" + filepath.Base(s.path)
}

func (s *ProgressSource) Poll() (bool, error) {
	info, err := os.Stat(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			if s.content != "" {
				s.content = ""
				s.lastSize = 0
				return true, nil
			}
			return false, nil
		}
		return false, fmt.Errorf("progress source: stat: %w", err)
	}
	if info.Size() == s.lastSize && s.content != "" {
		return false, nil
	}
	s.lastSize = info.Size()
	data, err := os.ReadFile(s.path)
	if err != nil {
		return false, fmt.Errorf("progress source: read: %w", err)
	}
	content := string(data)
	if content == s.content {
		return false, nil
	}
	s.content = content
	return true, nil
}

// Content returns the full content of the progress file.
func (s *ProgressSource) Content() string { return s.content }

// ---------------------------------------------------------------------------
// LogSource — finds and tails the most recent build log
// ---------------------------------------------------------------------------

// LogSource scans the build-logs directory for the most recently modified log
// file and reads its tail.
type LogSource struct {
	logsDir    string
	activePath string
	tail       string
	lastSize   int64
	tailLines  int
}

// NewLogSource creates a LogSource for the given build directory.
func NewLogSource(buildDir string, tailLines int) *LogSource {
	if tailLines <= 0 {
		tailLines = 20
	}
	return &LogSource{
		logsDir:   filepath.Join(buildDir, config.BuildLogsDir),
		tailLines: tailLines,
	}
}

func (s *LogSource) Name() string { return "logs" }

func (s *LogSource) Poll() (bool, error) {
	entries, err := os.ReadDir(s.logsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("log source: readdir: %w", err)
	}

	// Find the most recently modified .log file.
	var newest string
	var newestTime time.Time
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(newestTime) {
			newestTime = info.ModTime()
			newest = filepath.Join(s.logsDir, e.Name())
		}
	}
	if newest == "" {
		return false, nil
	}

	info, err := os.Stat(newest)
	if err != nil {
		return false, nil
	}

	if newest == s.activePath && info.Size() == s.lastSize {
		return false, nil
	}

	s.activePath = newest
	s.lastSize = info.Size()

	data, err := os.ReadFile(newest)
	if err != nil {
		return false, fmt.Errorf("log source: read: %w", err)
	}

	s.tail = lastNLines(string(data), s.tailLines)
	return true, nil
}

// ActivePath returns the path of the most recent log file.
func (s *LogSource) ActivePath() string { return s.activePath }

// Tail returns the last N lines of the active log.
func (s *LogSource) Tail() string { return s.tail }

func lastNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	// Remove trailing empty line from final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// ---------------------------------------------------------------------------
// ExitReasonSource — tracks .fry/build-exit-reason.txt
// ---------------------------------------------------------------------------

// ExitReasonSource polls the build exit reason file.
type ExitReasonSource struct {
	path   string
	reason string
	exists bool
}

// NewExitReasonSource creates an ExitReasonSource for the given build directory.
func NewExitReasonSource(buildDir string) *ExitReasonSource {
	return &ExitReasonSource{
		path: filepath.Join(buildDir, config.BuildExitReasonFile),
	}
}

func (s *ExitReasonSource) Name() string { return "exit_reason" }

func (s *ExitReasonSource) Poll() (bool, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			if s.exists {
				s.exists = false
				s.reason = ""
				return true, nil
			}
			return false, nil
		}
		return false, fmt.Errorf("exit reason source: read: %w", err)
	}
	reason := strings.TrimSpace(string(data))
	prev := s.exists
	s.exists = true
	if reason == s.reason && prev {
		return false, nil
	}
	s.reason = reason
	return true, nil
}

// Reason returns the exit reason (empty if file doesn't exist).
func (s *ExitReasonSource) Reason() string { return s.reason }

// Exists reports whether the exit reason file exists.
func (s *ExitReasonSource) Exists() bool { return s.exists }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// sortedKeys returns map keys in sorted order.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
