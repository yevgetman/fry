package monitor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yevgetman/fry/internal/agent"
	"github.com/yevgetman/fry/internal/config"
)

var (
	sprintIterLogPattern   = regexp.MustCompile(`^sprint(\d+)_iter(\d+)_\d{8}_\d{6}\.log$`)
	sprintAuditLogPattern  = regexp.MustCompile(`^sprint(\d+)_audit(\d+)_\d{8}_\d{6}\.log$`)
	auditFixLogPattern     = regexp.MustCompile(`^sprint(\d+)_auditfix_(\d+)_(\d+)_\d{8}_\d{6}\.log$`)
	auditVerifyLogPattern  = regexp.MustCompile(`^sprint(\d+)_auditverify_(\d+)_(\d+)_\d{8}_\d{6}\.log$`)
	sprintReviewLogPattern = regexp.MustCompile(`^sprint(\d+)_review_\d{8}_\d{6}\.log$`)
	observerLogPattern     = regexp.MustCompile(`^observer_(.+)_\d{8}_\d{6}\.log$`)
	buildAuditLogPattern   = regexp.MustCompile(`^build_audit_\d{8}_\d{6}\.log$`)
)

// LogEventSource emits synthetic monitor events from build-log session files.
// It only emits on new log file creation so verbose mode remains useful rather
// than producing a line for every append.
type LogEventSource struct {
	logsDir   string
	known     map[string]int64
	events    []agent.BuildEvent
	newEvents []agent.BuildEvent
}

// NewLogEventSource creates a LogEventSource for the given build directory.
func NewLogEventSource(buildDir string) *LogEventSource {
	return &LogEventSource{
		logsDir: filepath.Join(buildDir, config.BuildLogsDir),
		known:   make(map[string]int64),
	}
}

func (s *LogEventSource) Name() string { return "log_events" }

func (s *LogEventSource) Poll() (bool, error) {
	s.newEvents = nil

	entries, err := os.ReadDir(s.logsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("log event source: readdir: %w", err)
	}

	var paths []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		paths = append(paths, filepath.Join(s.logsDir, entry.Name()))
	}
	sort.Strings(paths)

	var changed bool
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		prevSize, seen := s.known[path]
		s.known[path] = info.Size()
		if seen {
			if info.Size() != prevSize {
				changed = true
			}
			continue
		}

		if evt, ok := synthesizeLogEvent(filepath.Base(path), info.ModTime()); ok {
			s.events = append(s.events, evt)
			s.newEvents = append(s.newEvents, evt)
			changed = true
		}
	}

	return changed, nil
}

// Events returns all synthetic log-derived events seen so far.
func (s *LogEventSource) Events() []agent.BuildEvent { return s.events }

// NewEvents returns the synthetic events discovered in the most recent poll.
func (s *LogEventSource) NewEvents() []agent.BuildEvent { return s.newEvents }

func synthesizeLogEvent(name string, ts time.Time) (agent.BuildEvent, bool) {
	if match := sprintIterLogPattern.FindStringSubmatch(name); match != nil {
		sprintNum, _ := strconv.Atoi(match[1])
		iter, _ := strconv.Atoi(match[2])
		return verboseEvent(ts, "agent_deploy", sprintNum, map[string]string{
			"iteration": strconv.Itoa(iter),
			"log":       name,
			"session":   "sprint",
		}), true
	}
	if match := auditFixLogPattern.FindStringSubmatch(name); match != nil {
		sprintNum, _ := strconv.Atoi(match[1])
		cycle, _ := strconv.Atoi(match[2])
		fix, _ := strconv.Atoi(match[3])
		return verboseEvent(ts, "audit_fix_start", sprintNum, map[string]string{
			"cycle": strconv.Itoa(cycle),
			"fix":   strconv.Itoa(fix),
			"log":   name,
		}), true
	}
	if match := auditVerifyLogPattern.FindStringSubmatch(name); match != nil {
		sprintNum, _ := strconv.Atoi(match[1])
		cycle, _ := strconv.Atoi(match[2])
		fix, _ := strconv.Atoi(match[3])
		return verboseEvent(ts, "audit_verify_start", sprintNum, map[string]string{
			"cycle": strconv.Itoa(cycle),
			"fix":   strconv.Itoa(fix),
			"log":   name,
		}), true
	}
	if match := sprintAuditLogPattern.FindStringSubmatch(name); match != nil {
		sprintNum, _ := strconv.Atoi(match[1])
		cycle, _ := strconv.Atoi(match[2])
		return verboseEvent(ts, "audit_cycle_start", sprintNum, map[string]string{
			"cycle": strconv.Itoa(cycle),
			"log":   name,
		}), true
	}
	if match := sprintReviewLogPattern.FindStringSubmatch(name); match != nil {
		sprintNum, _ := strconv.Atoi(match[1])
		return verboseEvent(ts, "review_start", sprintNum, map[string]string{
			"log": name,
		}), true
	}
	if match := observerLogPattern.FindStringSubmatch(name); match != nil {
		return verboseEvent(ts, "observer_wake", 0, map[string]string{
			"log":  name,
			"wake": match[1],
		}), true
	}
	if buildAuditLogPattern.MatchString(name) {
		return verboseEvent(ts, "build_audit_start", 0, map[string]string{
			"log": name,
		}), true
	}
	return agent.BuildEvent{}, false
}

func verboseEvent(ts time.Time, typ string, sprint int, data map[string]string) agent.BuildEvent {
	return agent.BuildEvent{
		Type:      typ,
		Timestamp: ts,
		Sprint:    sprint,
		Data:      data,
	}
}

func mergeEventsByTimestamp(a, b []agent.BuildEvent) []agent.BuildEvent {
	switch {
	case len(a) == 0:
		return append([]agent.BuildEvent(nil), b...)
	case len(b) == 0:
		return append([]agent.BuildEvent(nil), a...)
	}

	merged := make([]agent.BuildEvent, 0, len(a)+len(b))
	merged = append(merged, a...)
	merged = append(merged, b...)
	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].Timestamp.Before(merged[j].Timestamp)
	})
	return merged
}
