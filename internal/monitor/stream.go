package monitor

import (
	"context"
	"path/filepath"
	"time"

	"github.com/yevgetman/fry/internal/agent"
	"github.com/yevgetman/fry/internal/config"
)

// Config controls Monitor behavior.
type Config struct {
	ProjectDir   string
	WorktreeDir  string        // resolved build directory; may equal ProjectDir
	Interval     time.Duration // polling interval (default: 2s)
	Wait         bool          // wait for build to start before streaming
	LogTailLines int           // lines to tail from active log (default: 20)
	Verbose      bool          // include granular synthetic events derived from build logs
}

// Monitor polls all data sources and emits composed Snapshots.
type Monitor struct {
	cfg Config

	// Typed source references for snapshot composition.
	events     *EventSource
	phase      *PhaseSource
	status     *StatusSource
	lock       *LockSource
	sprintProg *ProgressSource
	epicProg   *ProgressSource
	logSrc     *LogSource
	logEvents  *LogEventSource
	exitReason *ExitReasonSource

	// Ordered list of all sources for sequential polling.
	sources []Source
}

// New creates a Monitor with all sources configured for the given project.
func New(cfg Config) *Monitor {
	if cfg.Interval <= 0 {
		cfg.Interval = time.Duration(config.MonitorDefaultIntervalSec) * time.Second
	}
	if cfg.LogTailLines <= 0 {
		cfg.LogTailLines = config.MonitorDefaultLogTailLines
	}
	if cfg.WorktreeDir == "" {
		cfg.WorktreeDir = cfg.ProjectDir
	}

	buildDir := cfg.WorktreeDir

	m := &Monitor{
		cfg:        cfg,
		events:     NewEventSource(cfg.ProjectDir),
		phase:      NewPhaseSource(cfg.ProjectDir, buildDir),
		status:     NewStatusSource(buildDir),
		lock:       NewLockSource(cfg.ProjectDir),
		sprintProg: NewProgressSource(filepath.Join(buildDir, config.SprintProgressFile)),
		epicProg:   NewProgressSource(filepath.Join(buildDir, config.EpicProgressFile)),
		logSrc:     NewLogSource(buildDir, cfg.LogTailLines),
		exitReason: NewExitReasonSource(buildDir),
	}
	if cfg.Verbose {
		m.logEvents = NewLogEventSource(buildDir)
	}

	// Poll order: cheapest first, most expensive last.
	m.sources = []Source{
		m.lock,
		m.events,
		m.phase,
		m.status,
		m.exitReason,
		m.sprintProg,
		m.epicProg,
		m.logSrc,
	}
	if m.logEvents != nil {
		m.sources = append(m.sources, m.logEvents)
	}

	return m
}

// Run polls all sources at the configured interval and sends Snapshots to
// the returned channel. The channel is closed when ctx is canceled or the
// build ends. Callers should range over the channel.
func (m *Monitor) Run(ctx context.Context) <-chan Snapshot {
	ch := make(chan Snapshot, 4)

	go func() {
		defer close(ch)

		idleTicks := 0
		firstPoll := true
		wasActive := false
		sawBuildEnd := false
		interval := m.cfg.Interval

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			anyChanged := m.pollAll()
			shouldEmit := firstPoll || anyChanged
			firstPoll = false

			if anyChanged {
				idleTicks = 0
				interval = m.cfg.Interval
			} else {
				idleTicks++
				if idleTicks >= config.MonitorIdleSlowdownTicks {
					interval = time.Duration(config.MonitorSlowIntervalSec) * time.Second
				}
			}

			snap := m.compose()

			// Detect build start (for --wait mode).
			if !wasActive && snap.BuildActive {
				wasActive = true
			}

			// If waiting and build hasn't started, skip emitting snapshots
			// but still send the first one so the caller can render "waiting".
			if m.cfg.Wait && !wasActive && !snap.BuildActive && len(snap.Events) == 0 {
				if shouldEmit {
					select {
					case ch <- snap:
					case <-ctx.Done():
						return
					}
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(interval):
					continue
				}
			}

			// Detect build end.
			if snap.BuildEnded && !sawBuildEnd {
				sawBuildEnd = true
				select {
				case ch <- snap:
				case <-ctx.Done():
				}
				return
			}

			// Detect process death without build_end event.
			if wasActive && !snap.BuildActive && !sawBuildEnd {
				snap.BuildEnded = true
				if snap.ExitReason == "" {
					snap.ExitReason = "process exited unexpectedly"
				}
				select {
				case ch <- snap:
				case <-ctx.Done():
				}
				return
			}

			// Emit snapshot when anything changed or on first iteration.
			if shouldEmit {
				select {
				case ch <- snap:
				case <-ctx.Done():
					return
				}
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
		}
	}()

	return ch
}

// Snapshot takes a single point-in-time snapshot without continuous polling.
func (m *Monitor) Snapshot() (Snapshot, error) {
	m.pollAll()
	return m.compose(), nil
}

// pollAll polls every source sequentially and returns true if any changed.
func (m *Monitor) pollAll() bool {
	anyChanged := false
	for _, src := range m.sources {
		changed, _ := src.Poll()
		if changed {
			anyChanged = true
		}
	}
	return anyChanged
}

// compose assembles a Snapshot from all source states.
func (m *Monitor) compose() Snapshot {
	totalSprints := 0
	var statusStart time.Time
	if st := m.status.Status(); st != nil {
		totalSprints = st.Build.TotalSprints
		if !st.Build.StartedAt.IsZero() {
			statusStart = st.Build.StartedAt
		}
	}

	allEvents := m.events.Events()
	newRaw := m.events.NewEvents()
	if m.logEvents != nil {
		allEvents = mergeEventsByTimestamp(allEvents, m.logEvents.Events())
		newRaw = mergeEventsByTimestamp(newRaw, m.logEvents.NewEvents())
	}
	buildStart := currentBuildStartTime(allEvents, statusStart)
	allEvents = filterEventsFrom(allEvents, buildStart)
	newRaw = filterEventsFrom(newRaw, buildStart)

	var enrichedAll []EnrichedEvent
	var enrichedNew []EnrichedEvent
	if len(allEvents) > 0 {
		enrichedAll = EnrichEvents(allEvents, totalSprints)
		if len(newRaw) > 0 {
			newStart := len(allEvents) - len(newRaw)
			if newStart >= 0 && newStart < len(enrichedAll) {
				enrichedNew = enrichedAll[newStart:]
			}
		}
	}

	// Detect build_end in events.
	buildEnded := false
	for _, evt := range newRaw {
		if evt.Type == "build_end" {
			buildEnded = true
			break
		}
	}
	// A stale exit-reason file from a prior run should not terminate an
	// active restarted build.
	if !buildEnded && !m.lock.Active() && m.exitReason.Exists() {
		buildEnded = true
	}

	return Snapshot{
		Timestamp:      time.Now(),
		ProjectDir:     m.cfg.ProjectDir,
		WorktreeDir:    m.cfg.WorktreeDir,
		BuildActive:    m.lock.Active(),
		PID:            m.lock.PID(),
		Phase:          m.phase.Phase(),
		Events:         enrichedAll,
		NewEvents:      enrichedNew,
		BuildStatus:    m.status.Status(),
		StatusChanged:  m.status.Changed(),
		SprintProgress: m.sprintProg.Content(),
		EpicProgress:   m.epicProg.Content(),
		ActiveLogPath:  m.logSrc.ActivePath(),
		ActiveLogTail:  m.logSrc.Tail(),
		BuildEnded:     buildEnded,
		ExitReason:     m.exitReason.Reason(),
	}
}

func currentBuildStartTime(events []agent.BuildEvent, fallback time.Time) time.Time {
	for _, evt := range events {
		if evt.Type == "build_start" {
			return evt.Timestamp
		}
	}
	return fallback
}

func filterEventsFrom(events []agent.BuildEvent, cutoff time.Time) []agent.BuildEvent {
	if cutoff.IsZero() || len(events) == 0 {
		return events
	}

	filtered := events[:0]
	for _, evt := range events {
		if evt.Timestamp.Before(cutoff) {
			continue
		}
		filtered = append(filtered, evt)
	}
	return filtered
}
