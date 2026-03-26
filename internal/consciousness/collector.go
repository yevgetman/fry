package consciousness

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// BuildObservation represents a single observer observation captured during a build.
type BuildObservation struct {
	Timestamp time.Time `json:"timestamp"`
	WakePoint string    `json:"wake_point"`
	SprintNum int       `json:"sprint_num"`
	Thoughts  string    `json:"thoughts"`
}

// BuildRecord is the complete record of a build's observations and metadata.
type BuildRecord struct {
	ID           string             `json:"id"`
	StartTime    time.Time          `json:"start_time"`
	EndTime      time.Time          `json:"end_time"`
	Engine       string             `json:"engine"`
	EffortLevel  string             `json:"effort_level"`
	TotalSprints int                `json:"total_sprints"`
	Outcome      string             `json:"outcome"`
	Observations []BuildObservation `json:"observations"`
}

// Collector accumulates observer observations throughout a build and writes
// them as a structured JSON record at build end.
type Collector struct {
	mu      sync.Mutex
	record  BuildRecord
	outDir  string // override for testing; empty uses default ~/.fry/experiences
}

// NewCollector creates a new observation collector for a build.
func NewCollector(engine, effort string, totalSprints int) *Collector {
	return &Collector{
		record: BuildRecord{
			ID:           generateBuildID(),
			StartTime:    time.Now(),
			Engine:       engine,
			EffortLevel:  effort,
			TotalSprints: totalSprints,
		},
	}
}

// AddObservation records an observer observation from a wake-up session.
// Safe for concurrent use.
func (c *Collector) AddObservation(thoughts, wakePoint string, sprintNum int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.record.Observations = append(c.record.Observations, BuildObservation{
		Timestamp: time.Now(),
		WakePoint: wakePoint,
		SprintNum: sprintNum,
		Thoughts:  thoughts,
	})
}

// Finalize sets the build outcome and writes the complete build record to
// ~/.fry/experiences/build-<id>.json.
func (c *Collector) Finalize(outcome string) error {
	c.mu.Lock()
	c.record.Outcome = outcome
	c.record.EndTime = time.Now()
	record := c.record // copy under lock
	c.mu.Unlock()

	dir, err := c.experiencesDir()
	if err != nil {
		return fmt.Errorf("finalize build record: %w", err)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("finalize build record: create dir: %w", err)
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("finalize build record: marshal: %w", err)
	}

	filename := fmt.Sprintf("build-%s.json", record.ID)
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("finalize build record: write: %w", err)
	}

	return nil
}

// ObservationCount returns the number of observations collected so far.
func (c *Collector) ObservationCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.record.Observations)
}

// BuildID returns the collector's build ID.
func (c *Collector) BuildID() string {
	return c.record.ID
}

func (c *Collector) experiencesDir() (string, error) {
	if c.outDir != "" {
		return c.outDir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".fry", "experiences"), nil
}

// generateBuildID creates a UUID v4 string using crypto/rand.
func generateBuildID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	// Set version 4 and variant bits per RFC 4122
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
