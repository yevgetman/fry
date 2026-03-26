package consciousness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCollector(t *testing.T) {
	t.Parallel()

	c := NewCollector("claude", "high", 5)
	require.NotNil(t, c)
	assert.NotEmpty(t, c.BuildID())
	assert.Equal(t, "claude", c.record.Engine)
	assert.Equal(t, "high", c.record.EffortLevel)
	assert.Equal(t, 5, c.record.TotalSprints)
	assert.Equal(t, 0, c.ObservationCount())
}

func TestAddObservation(t *testing.T) {
	t.Parallel()

	c := NewCollector("claude", "medium", 3)

	c.AddObservation("Sprint 1 went smoothly.", "after_sprint", 1)
	assert.Equal(t, 1, c.ObservationCount())

	c.AddObservation("Sprint 2 required healing.", "after_sprint", 2)
	assert.Equal(t, 2, c.ObservationCount())

	c.AddObservation("Build completed successfully.", "build_end", 3)
	assert.Equal(t, 3, c.ObservationCount())
}

func TestFinalize(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c := NewCollector("codex", "max", 4)
	c.outDir = dir

	c.AddObservation("First observation.", "after_sprint", 1)
	c.AddObservation("Second observation.", "build_end", 4)

	err := c.Finalize("success")
	require.NoError(t, err)

	// Verify file exists
	filename := "build-" + c.BuildID() + ".json"
	path := filepath.Join(dir, filename)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	// Verify valid JSON and content
	var record BuildRecord
	err = json.Unmarshal(data, &record)
	require.NoError(t, err)

	assert.Equal(t, c.BuildID(), record.ID)
	assert.Equal(t, "codex", record.Engine)
	assert.Equal(t, "max", record.EffortLevel)
	assert.Equal(t, 4, record.TotalSprints)
	assert.Equal(t, "success", record.Outcome)
	assert.False(t, record.StartTime.IsZero())
	assert.False(t, record.EndTime.IsZero())
	require.Len(t, record.Observations, 2)
	assert.Equal(t, "First observation.", record.Observations[0].Thoughts)
	assert.Equal(t, "after_sprint", record.Observations[0].WakePoint)
	assert.Equal(t, 1, record.Observations[0].SprintNum)
	assert.Equal(t, "Second observation.", record.Observations[1].Thoughts)
	assert.Equal(t, "build_end", record.Observations[1].WakePoint)
}

func TestFinalize_CreatesDirectory(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "nested", "experiences")
	c := NewCollector("claude", "low", 1)
	c.outDir = dir

	err := c.Finalize("success")
	require.NoError(t, err)

	// Verify directory was created and file exists
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestFinalize_FailureOutcome(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c := NewCollector("claude", "high", 3)
	c.outDir = dir

	c.AddObservation("Build failed at sprint 2.", "build_end", 2)
	err := c.Finalize("failure")
	require.NoError(t, err)

	filename := "build-" + c.BuildID() + ".json"
	data, err := os.ReadFile(filepath.Join(dir, filename))
	require.NoError(t, err)

	var record BuildRecord
	require.NoError(t, json.Unmarshal(data, &record))
	assert.Equal(t, "failure", record.Outcome)
}

func TestBuildID_Unique(t *testing.T) {
	t.Parallel()

	c1 := NewCollector("claude", "high", 3)
	c2 := NewCollector("claude", "high", 3)
	assert.NotEqual(t, c1.BuildID(), c2.BuildID())
}

func TestCollector_ConcurrentAdd(t *testing.T) {
	t.Parallel()

	c := NewCollector("claude", "max", 10)
	const goroutines = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			c.AddObservation("concurrent observation", "after_sprint", n)
		}(i)
	}
	wg.Wait()

	assert.Equal(t, goroutines, c.ObservationCount())
}

func TestSetSummary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c := NewCollector("claude", "high", 3)
	c.outDir = dir

	c.AddObservation("Some thoughts.", "after_sprint", 1)
	c.SetSummary("This build went smoothly with no surprises.")

	err := c.Finalize("success")
	require.NoError(t, err)

	filename := "build-" + c.BuildID() + ".json"
	data, err := os.ReadFile(filepath.Join(dir, filename))
	require.NoError(t, err)

	var record BuildRecord
	require.NoError(t, json.Unmarshal(data, &record))
	assert.Equal(t, "This build went smoothly with no surprises.", record.Summary)
}

func TestGetRecord(t *testing.T) {
	t.Parallel()

	c := NewCollector("claude", "medium", 2)
	c.AddObservation("First.", "after_sprint", 1)

	record := c.GetRecord()
	assert.Equal(t, c.BuildID(), record.ID)
	assert.Equal(t, "claude", record.Engine)
	assert.Equal(t, 1, len(record.Observations))

	// GetRecord returns a snapshot — adding more observations doesn't affect it
	c.AddObservation("Second.", "build_end", 2)
	assert.Equal(t, 1, len(record.Observations))
	assert.Equal(t, 2, c.ObservationCount())
}

func TestFinalize_EmptyObservations(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c := NewCollector("claude", "medium", 1)
	c.outDir = dir

	err := c.Finalize("success")
	require.NoError(t, err)

	filename := "build-" + c.BuildID() + ".json"
	data, err := os.ReadFile(filepath.Join(dir, filename))
	require.NoError(t, err)

	var record BuildRecord
	require.NoError(t, json.Unmarshal(data, &record))
	assert.Empty(t, record.Observations)
	assert.Equal(t, "success", record.Outcome)
}
