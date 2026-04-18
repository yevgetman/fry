package state

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	m := &Mission{
		MissionID:       "demo",
		CreatedAt:       now,
		InputMode:       "prompt",
		Effort:          "standard",
		IntervalSeconds: 600,
		DurationHours:   12.0,
		OvertimeHours:   12.0,
		CurrentWake:     0,
		Status:          StatusActive,
		HardDeadlineUTC: now.Add(24 * time.Hour),
	}

	data, err := json.Marshal(m)
	require.NoError(t, err)

	var m2 Mission
	require.NoError(t, json.Unmarshal(data, &m2))

	assert.Equal(t, m.MissionID, m2.MissionID)
	assert.Equal(t, m.Status, m2.Status)
	assert.Equal(t, m.DurationHours, m2.DurationHours)
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	m := &Mission{
		MissionID:       "test",
		CreatedAt:       now,
		InputMode:       "plan",
		Effort:          "fast",
		IntervalSeconds: 300,
		DurationHours:   1.0,
		OvertimeHours:   0.0,
		CurrentWake:     3,
		Status:          StatusActive,
		HardDeadlineUTC: now.Add(time.Hour),
	}

	require.NoError(t, m.Save(dir))

	loaded, err := Load(dir)
	require.NoError(t, err)

	assert.Equal(t, m.MissionID, loaded.MissionID)
	assert.Equal(t, m.CurrentWake, loaded.CurrentWake)
	assert.Equal(t, m.Status, loaded.Status)
}

func TestAtomicSave(t *testing.T) {
	dir := t.TempDir()
	m := &Mission{MissionID: "x", Status: StatusActive, CreatedAt: time.Now()}
	require.NoError(t, m.Save(dir))

	// tmp file must not remain
	_, err := os.Stat(dir + "/state.json.tmp")
	assert.True(t, os.IsNotExist(err))
}

func TestTransitions(t *testing.T) {
	assert.True(t, CanTransition(StatusActive, StatusComplete))
	assert.True(t, CanTransition(StatusActive, StatusOvertime))
	assert.True(t, CanTransition(StatusOvertime, StatusComplete))
	assert.True(t, CanTransition(StatusStopped, StatusActive))

	assert.False(t, CanTransition(StatusComplete, StatusActive))
	assert.False(t, CanTransition(StatusFailed, StatusActive))
	assert.False(t, CanTransition(StatusActive, StatusActive))
}

func TestTransitionApply(t *testing.T) {
	m := &Mission{Status: StatusActive}
	require.NoError(t, m.Transition(StatusOvertime))
	assert.Equal(t, StatusOvertime, m.Status)

	err := m.Transition(StatusActive) // illegal
	assert.Error(t, err)
	assert.Equal(t, StatusOvertime, m.Status) // unchanged
}
