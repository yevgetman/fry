package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
)

func TestAppendDeviationLogContinueVerdict(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := AppendDeviationLog(dir, DeviationLogEntry{
		SprintNum:  1,
		SprintName: "Foundation",
		Verdict:    VerdictContinue,
		Impact:     "All tasks completed.",
		Timestamp:  time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, config.DeviationLogFile))
	require.NoError(t, err)
	assert.Contains(t, string(content), "## Review after Sprint 1: Foundation (CONTINUE)")
	assert.Contains(t, string(content), "All tasks completed.")
}

func TestAppendDeviationLogDeviateVerdict(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := AppendDeviationLog(dir, DeviationLogEntry{
		Verdict:         VerdictDeviate,
		Trigger:         "Auth moved",
		AffectedSprints: []int{3, 4},
		RiskAssessment:  "Low",
		Timestamp:       time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, config.DeviationLogFile))
	require.NoError(t, err)
	assert.Contains(t, string(content), "Auth moved")
	assert.Contains(t, string(content), "3, 4")
	assert.Contains(t, string(content), "Low")
}

func TestAppendDeviationLogMultipleEntriesAppend(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := AppendDeviationLog(dir, DeviationLogEntry{
		SprintNum:  1,
		SprintName: "Foundation",
		Verdict:    VerdictContinue,
		Timestamp:  time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	err = AppendDeviationLog(dir, DeviationLogEntry{
		SprintNum:  2,
		SprintName: "Core",
		Verdict:    VerdictContinue,
		Timestamp:  time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, config.DeviationLogFile))
	require.NoError(t, err)
	assert.Equal(t, 2, strings.Count(string(content), "## Review after Sprint"))
	assert.Contains(t, string(content), "## Review after Sprint 1: Foundation (CONTINUE)")
	assert.Contains(t, string(content), "## Review after Sprint 2: Core (CONTINUE)")
}

func TestAppendDeviationSummary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := AppendDeviationSummary(dir, DeviationSummary{
		TotalSprints:      5,
		ReviewsConducted:  5,
		DeviationsApplied: 1,
		Retries:           0,
		AllLowRisk:        true,
	})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, config.DeviationLogFile))
	require.NoError(t, err)
	assert.Contains(t, string(content), "## Build Summary")
	assert.Contains(t, string(content), "Total sprints")
	assert.Contains(t, string(content), "5")
	assert.Contains(t, string(content), "Deviations applied")
	assert.Contains(t, string(content), "1")
	assert.Contains(t, string(content), "All deviations low risk")
}

func TestReadDeviationLogRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := AppendDeviationLog(dir, DeviationLogEntry{
		SprintNum:  1,
		SprintName: "Foundation",
		Verdict:    VerdictContinue,
		Timestamp:  time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	content, err := ReadDeviationLog(dir)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
	assert.Contains(t, content, "## Review after Sprint 1: Foundation (CONTINUE)")
}

func TestReadDeviationLogMissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content, err := ReadDeviationLog(dir)
	require.NoError(t, err)
	assert.Equal(t, "", content)
}
