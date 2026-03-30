package monitor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yevgetman/fry/internal/agent"
)

func TestEnrichEvents_Empty(t *testing.T) {
	t.Parallel()
	result := EnrichEvents(nil, 3)
	assert.Nil(t, result)
}

func TestEnrichEvents_ElapsedTimes(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	events := []agent.BuildEvent{
		{Type: "build_start", Timestamp: base},
		{Type: "sprint_start", Timestamp: base.Add(10 * time.Second), Sprint: 1},
		{Type: "sprint_complete", Timestamp: base.Add(5*time.Minute + 10*time.Second), Sprint: 1},
		{Type: "sprint_start", Timestamp: base.Add(5*time.Minute + 15*time.Second), Sprint: 2},
		{Type: "build_end", Timestamp: base.Add(10 * time.Minute)},
	}

	enriched := EnrichEvents(events, 3)

	assert.Len(t, enriched, 5)

	// build_start: elapsed=0
	assert.Equal(t, time.Duration(0), enriched[0].ElapsedBuild)

	// sprint_start 1: elapsed=10s, sprint elapsed=0
	assert.Equal(t, 10*time.Second, enriched[1].ElapsedBuild)
	assert.Equal(t, time.Duration(0), enriched[1].ElapsedSprint)

	// sprint_complete 1: elapsed=5m10s, sprint elapsed=5m
	assert.Equal(t, 5*time.Minute+10*time.Second, enriched[2].ElapsedBuild)
	assert.Equal(t, 5*time.Minute, enriched[2].ElapsedSprint)

	// sprint_start 2: sprint elapsed resets to 0
	assert.Equal(t, time.Duration(0), enriched[3].ElapsedSprint)

	// build_end: terminal
	assert.True(t, enriched[4].IsTerminal)
	assert.Equal(t, 10*time.Minute, enriched[4].ElapsedBuild)
}

func TestEnrichEvents_SprintFraction(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	events := []agent.BuildEvent{
		{Type: "build_start", Timestamp: base},
		{Type: "sprint_start", Timestamp: base.Add(1 * time.Second), Sprint: 1},
		{Type: "sprint_start", Timestamp: base.Add(2 * time.Second), Sprint: 2},
	}

	enriched := EnrichEvents(events, 5)

	assert.Equal(t, "", enriched[0].SprintOf) // build_start has no sprint
	assert.Equal(t, "1/5", enriched[1].SprintOf)
	assert.Equal(t, "2/5", enriched[2].SprintOf)
}

func TestEnrichEvents_PhaseTransitions(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	events := []agent.BuildEvent{
		{Type: "triage_start", Timestamp: base},
		{Type: "triage_complete", Timestamp: base.Add(1 * time.Second)},
		{Type: "prepare_start", Timestamp: base.Add(2 * time.Second)},
		{Type: "prepare_complete", Timestamp: base.Add(3 * time.Second)},
		{Type: "build_start", Timestamp: base.Add(4 * time.Second)},
		{Type: "sprint_start", Timestamp: base.Add(5 * time.Second), Sprint: 1},
	}

	enriched := EnrichEvents(events, 3)

	// triage_start: no previous phase.
	assert.Empty(t, enriched[0].PhaseChange)

	// triage_complete: same phase (triage -> triage).
	assert.Empty(t, enriched[1].PhaseChange)

	// prepare_start: triage -> prepare.
	assert.Equal(t, "triage -> prepare", enriched[2].PhaseChange)

	// build_start: prepare -> sprint.
	assert.Equal(t, "prepare -> sprint", enriched[4].PhaseChange)

	// sprint_start: same phase (sprint -> sprint).
	assert.Empty(t, enriched[5].PhaseChange)
}

func TestEnrichNewEvents(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	events := []agent.BuildEvent{
		{Type: "build_start", Timestamp: base},
		{Type: "sprint_start", Timestamp: base.Add(1 * time.Second), Sprint: 1},
		{Type: "sprint_complete", Timestamp: base.Add(2 * time.Second), Sprint: 1},
	}

	// New events start at index 2 (only sprint_complete is new).
	newEnriched := EnrichNewEvents(events, 2, 3)
	assert.Len(t, newEnriched, 1)
	assert.Equal(t, "sprint_complete", newEnriched[0].Type)
	assert.Equal(t, "1/3", newEnriched[0].SprintOf)
}

func TestEnrichNewEvents_OutOfBounds(t *testing.T) {
	t.Parallel()

	events := []agent.BuildEvent{
		{Type: "build_start", Timestamp: time.Now()},
	}

	result := EnrichNewEvents(events, 5, 3)
	assert.Nil(t, result)
}
