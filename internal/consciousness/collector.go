package consciousness

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yevgetman/fry/internal/config"
)

// BuildObservation represents a single canonical observer observation captured during a build.
type BuildObservation struct {
	Timestamp time.Time `json:"timestamp"`
	WakePoint string    `json:"wake_point"`
	SprintNum int       `json:"sprint_num"`
	Thoughts  string    `json:"thoughts"`
}

// BuildRecord is the long-term durable record of a logical build session.
type BuildRecord struct {
	ID                  string              `json:"id"`
	SessionID           string              `json:"session_id,omitempty"`
	StartTime           time.Time           `json:"start_time"`
	EndTime             time.Time           `json:"end_time"`
	Engine              string              `json:"engine"`
	EffortLevel         string              `json:"effort_level"`
	TotalSprints        int                 `json:"total_sprints"`
	Outcome             string              `json:"outcome"`
	Observations        []BuildObservation  `json:"observations"`
	Summary             string              `json:"summary,omitempty"`
	RunSegments         []RunSegment        `json:"run_segments,omitempty"`
	CheckpointCount     int                 `json:"checkpoint_count,omitempty"`
	CheckpointSummaries []CheckpointSummary `json:"checkpoint_summaries,omitempty"`
	ParseFailures       int                 `json:"parse_failures,omitempty"`
	RepairSuccesses     int                 `json:"repair_successes,omitempty"`
	Interrupted         bool                `json:"interrupted,omitempty"`
	UploadState         UploadState         `json:"upload_state,omitempty"`
}

type CollectorOptions struct {
	ProjectDir    string
	EpicName      string
	Engine        string
	EffortLevel   string
	TotalSprints  int
	CurrentSprint int
	Mode          SessionMode
	UploadEnabled bool
}

// Collector persists consciousness checkpoints incrementally and materializes the final build record.
type Collector struct {
	mu            sync.Mutex
	projectDir    string
	outDir        string
	uploadEnabled bool
	session       SessionState
	record        BuildRecord
}

// NewCollector initializes or resumes a logical consciousness session.
func NewCollector(opts CollectorOptions) (*Collector, error) {
	if stringsTrim(opts.ProjectDir) == "" {
		return nil, fmt.Errorf("new collector: project dir is required")
	}

	if opts.Mode == "" {
		opts.Mode = SessionModeNew
	}

	c := &Collector{
		projectDir:    opts.ProjectDir,
		uploadEnabled: opts.UploadEnabled,
	}
	if err := c.init(opts); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Collector) init(opts CollectorOptions) error {
	if opts.Mode == SessionModeNew {
		if err := os.RemoveAll(filepath.Join(c.projectDir, config.ConsciousnessDir)); err != nil {
			return fmt.Errorf("new collector: reset runtime dir: %w", err)
		}
	}
	if err := c.ensureRuntimeDirs(); err != nil {
		return fmt.Errorf("new collector: %w", err)
	}

	now := time.Now().UTC()
	state, stateErr := loadSessionState(filepath.Join(c.projectDir, config.ConsciousnessSessionFile))
	if opts.Mode == SessionModeResume && stateErr == nil {
		c.session = *state
		c.record = c.hydrateRecord(opts, *state)
		c.session.SessionResumedCount++
		c.session.CurrentSprint = opts.CurrentSprint
		c.session.Engine = opts.Engine
		c.session.EffortLevel = opts.EffortLevel
		c.session.TotalSprints = opts.TotalSprints
		c.session.Status = SessionStatusRunning
		c.session.LastUpdatedAt = now
		c.session.RunSegments = append(c.session.RunSegments, RunSegment{
			Index:     len(c.session.RunSegments) + 1,
			StartedAt: now,
			Status:    string(SessionStatusRunning),
			Resumed:   true,
		})
		c.record.RunSegments = append([]RunSegment(nil), c.session.RunSegments...)
		return c.writeSessionLocked()
	}

	buildID := generateBuildID()
	c.session = SessionState{
		SessionID:     buildID,
		BuildID:       buildID,
		ProjectDir:    c.projectDir,
		EpicName:      opts.EpicName,
		Engine:        opts.Engine,
		EffortLevel:   opts.EffortLevel,
		TotalSprints:  opts.TotalSprints,
		StartedAt:     now,
		LastUpdatedAt: now,
		Status:        SessionStatusRunning,
		CurrentSprint: opts.CurrentSprint,
		RunSegments: []RunSegment{
			{
				Index:     1,
				StartedAt: now,
				Status:    string(SessionStatusRunning),
				Resumed:   false,
			},
		},
	}
	c.record = BuildRecord{
		ID:           buildID,
		SessionID:    buildID,
		StartTime:    now,
		Engine:       opts.Engine,
		EffortLevel:  opts.EffortLevel,
		TotalSprints: opts.TotalSprints,
		RunSegments:  append([]RunSegment(nil), c.session.RunSegments...),
	}
	if err := c.writeSessionLocked(); err != nil {
		return err
	}
	if err := c.queueUploadEventLocked(newLifecycleUpload(c.record, 0, UploadEventSessionStarted)); err != nil {
		return err
	}
	return c.writeSessionLocked()
}

// AddCheckpoint persists a checkpoint immediately after an observer wake or interruption boundary.
func (c *Collector) AddCheckpoint(checkpoint ObservationCheckpoint) (ObservationCheckpoint, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UTC()
	c.session.LastSequence++
	c.session.LastUpdatedAt = now
	c.session.LastFlushedAt = now
	c.session.CurrentSprint = checkpoint.SprintNum
	c.session.CheckpointsPersisted++
	checkpoint.SessionID = c.session.SessionID
	checkpoint.Sequence = c.session.LastSequence
	if checkpoint.Timestamp.IsZero() {
		checkpoint.Timestamp = now
	}
	if checkpoint.CheckpointType == "" {
		checkpoint.CheckpointType = CheckpointTypeObservation
	}

	switch checkpoint.ParseStatus {
	case ParseStatusFailed:
		c.session.ParseFailures++
	case ParseStatusRepaired:
		c.session.RepairSuccesses++
	}
	if checkpoint.CheckpointType == CheckpointTypeInterruption {
		c.session.Status = SessionStatusInterrupted
		c.record.Interrupted = true
	}

	if checkpoint.Observation != nil && stringsTrim(checkpoint.Observation.Thoughts) != "" {
		c.record.Observations = append(c.record.Observations, *checkpoint.Observation)
	}
	c.record.CheckpointCount = c.session.CheckpointsPersisted
	c.record.ParseFailures = c.session.ParseFailures
	c.record.RepairSuccesses = c.session.RepairSuccesses

	if err := writeJSONAtomic(filepath.Join(c.projectDir, config.ConsciousnessCheckpointsDir, checkpointFilename(checkpoint.Sequence)), checkpoint); err != nil {
		return ObservationCheckpoint{}, fmt.Errorf("add checkpoint: write checkpoint: %w", err)
	}
	if err := appendJSONL(filepath.Join(c.projectDir, config.ConsciousnessCheckpointsFile), checkpoint); err != nil {
		return ObservationCheckpoint{}, fmt.Errorf("add checkpoint: append checkpoint log: %w", err)
	}
	if stringsTrim(checkpoint.ScratchpadDelta) != "" {
		entry := scratchpadHistoryEntry{
			SessionID: c.session.SessionID,
			Sequence:  checkpoint.Sequence,
			Timestamp: checkpoint.Timestamp,
			Delta:     checkpoint.ScratchpadDelta,
		}
		if err := appendJSONL(filepath.Join(c.projectDir, config.ConsciousnessScratchpadHistory), entry); err != nil {
			return ObservationCheckpoint{}, fmt.Errorf("add checkpoint: append scratchpad history: %w", err)
		}
	}

	if err := c.writeSessionLocked(); err != nil {
		return ObservationCheckpoint{}, err
	}
	return checkpoint, nil
}

// RecordDistillation persists a checkpoint summary and queues it for upload.
func (c *Collector) RecordDistillation(summary CheckpointSummary) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if summary.Timestamp.IsZero() {
		summary.Timestamp = time.Now().UTC()
	}
	summary.SessionID = c.session.SessionID
	summary.UploadState = UploadStatePending
	c.record.CheckpointSummaries = upsertCheckpointSummary(c.record.CheckpointSummaries, summary)
	c.record.CheckpointCount = c.session.CheckpointsPersisted
	c.session.DistillationsSucceeded++
	c.session.LastUpdatedAt = summary.Timestamp
	c.record.RunSegments = append([]RunSegment(nil), c.session.RunSegments...)

	if err := writeJSONAtomic(filepath.Join(c.projectDir, config.ConsciousnessDistilledDir, checkpointFilename(summary.Sequence)), summary); err != nil {
		return fmt.Errorf("record distillation: write summary: %w", err)
	}
	if err := c.queueUploadEventLocked(newCheckpointUpload(c.record, summary)); err != nil {
		return fmt.Errorf("record distillation: queue upload: %w", err)
	}
	return c.writeSessionLocked()
}

// RecordDistillationFailure updates pipeline counters without failing the build.
func (c *Collector) RecordDistillationFailure() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.session.DistillationsFailed++
	c.session.LastUpdatedAt = time.Now().UTC()
	return c.writeSessionLocked()
}

// SetCurrentSprint updates the session's active sprint for status reporting.
func (c *Collector) SetCurrentSprint(sprintNum int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.session.CurrentSprint = sprintNum
	c.session.LastUpdatedAt = time.Now().UTC()
	return c.writeSessionLocked()
}

// Finalize writes the current build record and marks the session state.
func (c *Collector) Finalize(outcome string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UTC()
	c.record.Outcome = outcome
	c.record.EndTime = now
	c.record.RunSegments = append([]RunSegment(nil), c.session.RunSegments...)
	c.record.CheckpointCount = c.session.CheckpointsPersisted
	c.record.ParseFailures = c.session.ParseFailures
	c.record.RepairSuccesses = c.session.RepairSuccesses

	if len(c.session.RunSegments) > 0 {
		last := &c.session.RunSegments[len(c.session.RunSegments)-1]
		last.EndedAt = &now
		switch {
		case c.record.Interrupted:
			last.Status = string(SessionStatusInterrupted)
		case outcome == "success":
			last.Status = string(SessionStatusCompleted)
		default:
			last.Status = string(SessionStatusFailed)
		}
	}
	c.record.RunSegments = append([]RunSegment(nil), c.session.RunSegments...)

	switch {
	case c.record.Interrupted:
		c.session.Status = SessionStatusInterrupted
	case outcome == "success":
		c.session.Status = SessionStatusCompleted
	default:
		c.session.Status = SessionStatusFailed
	}
	c.session.LastUpdatedAt = now
	c.session.LastFlushedAt = now

	dir, err := c.experiencesDir()
	if err != nil {
		return fmt.Errorf("finalize build record: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("finalize build record: create dir: %w", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("build-%s.json", c.record.ID))
	eventType := UploadEventSessionCompleted
	if c.record.Interrupted {
		eventType = UploadEventSessionInterrupted
	}
	if err := c.queueUploadEventLocked(newLifecycleUpload(c.record, c.session.LastSequence, eventType)); err != nil {
		return err
	}
	if err := writeJSONAtomic(path, c.record); err != nil {
		return fmt.Errorf("finalize build record: write: %w", err)
	}
	return c.writeSessionLocked()
}

// ObservationCount returns the number of canonical observations collected so far.
func (c *Collector) ObservationCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.record.Observations)
}

func (c *Collector) CheckpointCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.session.CheckpointsPersisted
}

// BuildID returns the collector's stable logical build ID.
func (c *Collector) BuildID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.record.ID
}

func (c *Collector) SessionID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.session.SessionID
}

// SetSummary stores the final experience summary on the build record.
func (c *Collector) SetSummary(summary string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.record.Summary = summary
}

// GetRecord returns a snapshot of the current build record without finalizing.
func (c *Collector) GetRecord() BuildRecord {
	c.mu.Lock()
	defer c.mu.Unlock()
	record := c.record
	record.RunSegments = append([]RunSegment(nil), c.record.RunSegments...)
	record.Observations = append([]BuildObservation(nil), c.record.Observations...)
	record.CheckpointSummaries = append([]CheckpointSummary(nil), c.record.CheckpointSummaries...)
	return record
}

func (c *Collector) GetSessionState() SessionState {
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.session
	state.RunSegments = append([]RunSegment(nil), c.session.RunSegments...)
	return state
}

func (c *Collector) experiencesDir() (string, error) {
	if c.outDir != "" {
		return c.outDir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, config.ExperiencesDir), nil
}

func (c *Collector) ensureRuntimeDirs() error {
	dirs := []string{
		filepath.Join(c.projectDir, config.ConsciousnessDir),
		filepath.Join(c.projectDir, config.ConsciousnessCheckpointsDir),
		filepath.Join(c.projectDir, config.ConsciousnessDistilledDir),
		filepath.Join(c.projectDir, config.ConsciousnessUploadQueueDir),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}
	return nil
}

func (c *Collector) writeSessionLocked() error {
	return writeJSONAtomic(filepath.Join(c.projectDir, config.ConsciousnessSessionFile), c.session)
}

func (c *Collector) hydrateRecord(opts CollectorOptions, state SessionState) BuildRecord {
	record := BuildRecord{
		ID:              state.BuildID,
		SessionID:       state.SessionID,
		StartTime:       state.StartedAt,
		Engine:          state.Engine,
		EffortLevel:     state.EffortLevel,
		TotalSprints:    state.TotalSprints,
		CheckpointCount: state.CheckpointsPersisted,
		ParseFailures:   state.ParseFailures,
		RepairSuccesses: state.RepairSuccesses,
		RunSegments:     append([]RunSegment(nil), state.RunSegments...),
	}

	if existing, err := c.readExistingRecord(state.BuildID); err == nil {
		record = existing
		record.RunSegments = append([]RunSegment(nil), state.RunSegments...)
		record.CheckpointCount = state.CheckpointsPersisted
		record.ParseFailures = state.ParseFailures
		record.RepairSuccesses = state.RepairSuccesses
		record.SessionID = state.SessionID
		record.Engine = opts.Engine
		record.EffortLevel = opts.EffortLevel
		record.TotalSprints = opts.TotalSprints
	}

	checkpoints := loadCheckpoints(filepath.Join(c.projectDir, config.ConsciousnessCheckpointsFile))
	record.Observations = record.Observations[:0]
	for _, checkpoint := range checkpoints {
		if checkpoint.Observation != nil && stringsTrim(checkpoint.Observation.Thoughts) != "" {
			record.Observations = append(record.Observations, *checkpoint.Observation)
		}
		if checkpoint.CheckpointType == CheckpointTypeInterruption {
			record.Interrupted = true
		}
	}

	record.CheckpointSummaries = loadCheckpointSummaries(filepath.Join(c.projectDir, config.ConsciousnessDistilledDir))
	record.CheckpointCount = state.CheckpointsPersisted
	record.ParseFailures = state.ParseFailures
	record.RepairSuccesses = state.RepairSuccesses
	return record
}

func (c *Collector) readExistingRecord(buildID string) (BuildRecord, error) {
	dir, err := c.experiencesDir()
	if err != nil {
		return BuildRecord{}, err
	}
	path := filepath.Join(dir, fmt.Sprintf("build-%s.json", buildID))
	data, err := os.ReadFile(path)
	if err != nil {
		return BuildRecord{}, err
	}
	var record BuildRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return BuildRecord{}, err
	}
	return record, nil
}

func loadSessionState(path string) (*SessionState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func loadCheckpoints(path string) []ObservationCheckpoint {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var checkpoints []ObservationCheckpoint
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var checkpoint ObservationCheckpoint
		if err := json.Unmarshal(scanner.Bytes(), &checkpoint); err == nil {
			checkpoints = append(checkpoints, checkpoint)
		}
	}
	sort.Slice(checkpoints, func(i, j int) bool {
		return checkpoints[i].Sequence < checkpoints[j].Sequence
	})
	return checkpoints
}

func loadCheckpointSummaries(dir string) []CheckpointSummary {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	summaries := make([]CheckpointSummary, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var summary CheckpointSummary
		if err := json.Unmarshal(data, &summary); err != nil {
			continue
		}
		summaries = append(summaries, summary)
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Sequence < summaries[j].Sequence
	})
	return summaries
}

func upsertCheckpointSummary(existing []CheckpointSummary, next CheckpointSummary) []CheckpointSummary {
	replaced := false
	for i := range existing {
		if existing[i].Sequence == next.Sequence {
			existing[i] = next
			replaced = true
			break
		}
	}
	if !replaced {
		existing = append(existing, next)
	}
	sort.Slice(existing, func(i, j int) bool {
		return existing[i].Sequence < existing[j].Sequence
	})
	return existing
}

func writeJSONAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func appendJSONL(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	line, err := json.Marshal(value)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(line)
	return err
}

func checkpointFilename(sequence int) string {
	return fmt.Sprintf("%04d.json", sequence)
}

// generateBuildID creates a UUID v4 string using crypto/rand.
func generateBuildID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func stringsTrim(s string) string {
	return strings.TrimSpace(s)
}
