package observer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/epic"
)

func TestBuildObserverPrompt_IncludesAllSections(t *testing.T) {
	t.Parallel()

	opts := ObserverOpts{
		ProjectDir:   t.TempDir(),
		EpicName:     "TestEpic",
		WakePoint:    WakeAfterSprint,
		SprintNum:    2,
		TotalSprints: 5,
		EffortLevel:  epic.EffortHigh,
	}
	identity := "# Observer Identity\nI observe things.\n"
	scratchpad := "Sprint 1 was smooth.\n"
	events := []Event{
		{Timestamp: "2026-01-01T00:00:00Z", Type: EventBuildStart},
		{Timestamp: "2026-01-01T00:01:00Z", Type: EventSprintComplete, Sprint: 1},
	}

	prompt := buildObserverPrompt(opts, identity, scratchpad, events)

	assert.Contains(t, prompt, "# OBSERVER")
	assert.Contains(t, prompt, "**Epic:** TestEpic")
	assert.Contains(t, prompt, "**Sprint:** 2/5")
	assert.Contains(t, prompt, "**Effort:** high")
	assert.Contains(t, prompt, "## Your Identity")
	assert.Contains(t, prompt, "I observe things")
	assert.Contains(t, prompt, "## Your Scratchpad")
	assert.Contains(t, prompt, "Sprint 1 was smooth")
	assert.Contains(t, prompt, "## Recent Build Events")
	assert.Contains(t, prompt, "build_start")
	assert.Contains(t, prompt, "## Wake-Point Context")
	assert.Contains(t, prompt, "after sprint 2")
	assert.Contains(t, prompt, "## Output Format")
	assert.Contains(t, prompt, "<thoughts>")
	assert.Contains(t, prompt, "<scratchpad>")
	assert.NotContains(t, prompt, "identity_update")
}

func TestBuildObserverPrompt_TruncatesLargeIdentity(t *testing.T) {
	t.Parallel()

	opts := ObserverOpts{
		ProjectDir:   t.TempDir(),
		EpicName:     "Test",
		WakePoint:    WakeBuildEnd,
		EffortLevel:  epic.EffortHigh,
		TotalSprints: 1,
	}
	largeIdentity := strings.Repeat("x", config.MaxObserverIdentityBytes+1000)

	prompt := buildObserverPrompt(opts, largeIdentity, "", nil)

	assert.Contains(t, prompt, "...(truncated)")
	// The identity in the prompt should be capped
	identitySection := extractSection(prompt, "## Your Identity", "## Your Scratchpad")
	assert.LessOrEqual(t, len(identitySection), config.MaxObserverIdentityBytes+200) // allow for truncation marker
}

func TestBuildObserverPrompt_TruncatesLargeScratchpad(t *testing.T) {
	t.Parallel()

	opts := ObserverOpts{
		ProjectDir:   t.TempDir(),
		EpicName:     "Test",
		WakePoint:    WakeBuildEnd,
		EffortLevel:  epic.EffortHigh,
		TotalSprints: 1,
	}
	largeScratchpad := strings.Repeat("y", config.MaxObserverScratchpadBytes+1000)

	prompt := buildObserverPrompt(opts, "", largeScratchpad, nil)

	assert.Contains(t, prompt, "...(truncated)")
}

func TestBuildObserverPrompt_EmptyEvents(t *testing.T) {
	t.Parallel()

	opts := ObserverOpts{
		ProjectDir:   t.TempDir(),
		EpicName:     "Test",
		WakePoint:    WakeBuildEnd,
		EffortLevel:  epic.EffortMedium,
		TotalSprints: 1,
	}

	prompt := buildObserverPrompt(opts, "", "", nil)

	assert.Contains(t, prompt, "(No events recorded yet.)")
}

func TestBuildObserverPrompt_WakePointContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		wake     WakePoint
		contains string
	}{
		{"after_sprint", WakeAfterSprint, "after sprint"},
		{"after_build_audit", WakeAfterBuildAudit, "after the build-level audit"},
		{"build_end", WakeBuildEnd, "at build end"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			opts := ObserverOpts{
				ProjectDir:   t.TempDir(),
				EpicName:     "Test",
				WakePoint:    tc.wake,
				SprintNum:    1,
				TotalSprints: 1,
				EffortLevel:  epic.EffortHigh,
			}
			prompt := buildObserverPrompt(opts, "", "", nil)
			assert.Contains(t, prompt, tc.contains)
		})
	}
}

func TestBuildObserverPrompt_BuildDataSorted(t *testing.T) {
	t.Parallel()

	opts := ObserverOpts{
		ProjectDir:   t.TempDir(),
		EpicName:     "Test",
		WakePoint:    WakeAfterBuildAudit,
		TotalSprints: 3,
		EffortLevel:  epic.EffortHigh,
		BuildData: map[string]string{
			"z_key": "last",
			"a_key": "first",
			"m_key": "middle",
		},
	}

	prompt := buildObserverPrompt(opts, "", "", nil)

	assert.Contains(t, prompt, "### Additional Build Data")
	assert.Contains(t, prompt, "**a_key:** first")
	assert.Contains(t, prompt, "**m_key:** middle")
	assert.Contains(t, prompt, "**z_key:** last")

	// Verify sorted order: a_key appears before m_key which appears before z_key
	aIdx := strings.Index(prompt, "a_key")
	mIdx := strings.Index(prompt, "m_key")
	zIdx := strings.Index(prompt, "z_key")
	assert.Less(t, aIdx, mIdx)
	assert.Less(t, mIdx, zIdx)
}

func TestParseObserverResponse_AllSections(t *testing.T) {
	t.Parallel()

	response := `Here are my observations:

<thoughts>
The build is progressing well. Sprint 1 completed without alignment loops.
</thoughts>

<scratchpad>
Watch sprint 2 for potential test failures in the auth module.
</scratchpad>

<identity_update>
# Observer Identity
I am the Observer. I have seen one build complete smoothly.
</identity_update>

<directives>
NOTE: Sprint 1 used only 3 of 10 allocated iterations
SUGGEST: Consider lowering effort for similar tasks
</directives>`

	obs, err := parseObserverResponse(response)
	require.NoError(t, err)
	assert.Contains(t, obs.Thoughts, "build is progressing well")
	assert.Contains(t, obs.ScratchpadDelta, "Watch sprint 2")

	require.Len(t, obs.Directives, 2)
	assert.Equal(t, "NOTE", obs.Directives[0].Type)
	assert.Contains(t, obs.Directives[0].Value, "Sprint 1 used only 3")
	assert.Equal(t, "SUGGEST", obs.Directives[1].Type)
}

func TestParseObserverResponse_ThoughtsOnly(t *testing.T) {
	t.Parallel()

	response := `<thoughts>
Just some initial thoughts about this build.
</thoughts>

<scratchpad>
Nothing noteworthy yet.
</scratchpad>`

	obs, err := parseObserverResponse(response)
	require.NoError(t, err)
	assert.Contains(t, obs.Thoughts, "initial thoughts")
	assert.Contains(t, obs.ScratchpadDelta, "Nothing noteworthy")

	assert.Nil(t, obs.Directives)
}

func TestParseObserverResponse_MalformedFallback(t *testing.T) {
	t.Parallel()

	response := "This is just plain text with no tags at all."

	obs, err := parseObserverResponse(response)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no structured tags")
	assert.Equal(t, "This is just plain text with no tags at all.", obs.Thoughts)
	assert.Empty(t, obs.ScratchpadDelta)

}

func TestParseObserverResponse_WithDirectives(t *testing.T) {
	t.Parallel()

	response := `<thoughts>Observations here.</thoughts>
<scratchpad>Notes here.</scratchpad>
<directives>
WARN: alignment loop on sprint 3 appears stuck
NOTE: audit found same issue in 3 sprints
SUGGEST: add a pre-iteration check for dependency resolution
</directives>`

	obs, err := parseObserverResponse(response)
	require.NoError(t, err)
	require.Len(t, obs.Directives, 3)
	assert.Equal(t, "WARN", obs.Directives[0].Type)
	assert.Contains(t, obs.Directives[0].Value, "alignment loop")
	assert.Equal(t, "NOTE", obs.Directives[1].Type)
	assert.Equal(t, "SUGGEST", obs.Directives[2].Type)
}

func TestParseObserverResponse_EmptyInput(t *testing.T) {
	t.Parallel()

	obs, err := parseObserverResponse("")
	require.NoError(t, err)
	assert.Empty(t, obs.Thoughts)
	assert.Empty(t, obs.ScratchpadDelta)

}

// extractSection is a test helper that extracts text between two section headers.
func extractSection(text, startHeader, endHeader string) string {
	startIdx := strings.Index(text, startHeader)
	if startIdx < 0 {
		return ""
	}
	rest := text[startIdx+len(startHeader):]
	endIdx := strings.Index(rest, endHeader)
	if endIdx < 0 {
		return rest
	}
	return rest[:endIdx]
}
