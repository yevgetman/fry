package audit

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixHistoryForPromptFiltersToRelevantFindings(t *testing.T) {
	t.Parallel()

	relevant := Finding{Location: "a.go:10", Description: "Issue A", Severity: "HIGH"}
	other := Finding{Location: "b.go:20", Description: "Issue B", Severity: "HIGH"}
	history := &FixHistory{}
	history.Record(FixAttempt{
		Cycle:       1,
		Iteration:   1,
		Targeted:    []string{findingLabel(relevant)},
		DiffSummary: "a.go | 2 ++",
		Outcomes:    buildOutcomes([]Finding{relevant}, []verificationResult{{Status: "STILL PRESENT", Notes: "conditional inverted"}}),
	})
	history.Record(FixAttempt{
		Cycle:       1,
		Iteration:   2,
		Targeted:    []string{findingLabel(other)},
		DiffSummary: "b.go | 2 ++",
		Outcomes:    buildOutcomes([]Finding{other}, []verificationResult{{Status: "RESOLVED"}}),
	})

	rendered := history.ForPrompt([]Finding{relevant}, 10_000)
	assert.Contains(t, rendered, "Issue A")
	assert.Contains(t, rendered, "conditional inverted")
	assert.NotContains(t, rendered, "Issue B")
}

func TestFixHistoryForPromptCapDropsOldAttempts(t *testing.T) {
	t.Parallel()

	finding := Finding{Location: "a.go:10", Description: "Issue A", Severity: "HIGH"}
	history := &FixHistory{}
	for i := 0; i < 3; i++ {
		history.Record(FixAttempt{
			Cycle:       1,
			Iteration:   i + 1,
			Targeted:    []string{findingLabel(finding)},
			DiffSummary: strings.Repeat("diff ", 50),
			Outcomes:    buildOutcomes([]Finding{finding}, []verificationResult{{Status: "STILL PRESENT"}}),
		})
	}

	rendered := history.ForPrompt([]Finding{finding}, 250)
	assert.Contains(t, rendered, "Attempt")
	assert.NotContains(t, rendered, "iteration 1")
}

func TestFixHistoryPruneResolved(t *testing.T) {
	t.Parallel()

	active := Finding{Location: "a.go:10", Description: "Issue A", Severity: "HIGH"}
	resolved := Finding{Location: "b.go:20", Description: "Issue B", Severity: "HIGH"}
	history := &FixHistory{}
	history.Record(FixAttempt{
		Cycle:       1,
		Iteration:   1,
		Targeted:    []string{findingLabel(active), findingLabel(resolved)},
		DiffSummary: "2 files changed",
		Outcomes: append(
			buildOutcomes([]Finding{active}, []verificationResult{{Status: "STILL PRESENT"}}),
			buildOutcomes([]Finding{resolved}, []verificationResult{{Status: "RESOLVED"}})...,
		),
	})

	history.PruneResolved([]Finding{active})
	rendered := history.ForPrompt([]Finding{active}, 10_000)
	assert.Contains(t, rendered, "Issue A")
	assert.NotContains(t, rendered, "Issue B")
}

func TestBuildOutcomesIncludesVerifierNotes(t *testing.T) {
	t.Parallel()

	finding := Finding{Location: "a.go:10", Description: "Issue A", Severity: "HIGH"}
	outcomes := buildOutcomes([]Finding{finding}, []verificationResult{{Status: "BEHAVIOR_UNCHANGED", Notes: "guard clause still missing"}})
	require.Len(t, outcomes, 1)
	assert.Equal(t, "guard clause still missing", outcomes[0].Reason)
	assert.Equal(t, verifyStatusBehaviorUnchanged, outcomes[0].Status)
}

func TestFixHistoryBehaviorUnchangedSignals(t *testing.T) {
	t.Parallel()

	finding := Finding{Location: "a.go:10", Description: "Issue A", Severity: "HIGH"}
	history := &FixHistory{}
	history.Record(FixAttempt{
		Cycle:       1,
		Iteration:   1,
		Targeted:    []string{findingLabel(finding)},
		DiffSummary: "a.go | 2 ++",
		Outcomes:    buildOutcomes([]Finding{finding}, []verificationResult{{Status: "BEHAVIOR_UNCHANGED", Notes: "comment added; branch unchanged"}}),
	})
	history.Record(FixAttempt{
		Cycle:       1,
		Iteration:   2,
		Targeted:    []string{findingLabel(finding)},
		DiffSummary: "a.go | 1 +",
		Outcomes:    buildOutcomes([]Finding{finding}, []verificationResult{{Status: "BEHAVIOR_UNCHANGED", Notes: "same nil path still executes"}}),
	})

	signals := history.BehaviorUnchangedSignals([]Finding{finding})

	require.Len(t, signals, 1)
	assert.Equal(t, 2, signals[0].Count)
	assert.Equal(t, "same nil path still executes", signals[0].LatestNote)
}
