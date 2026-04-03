package audit

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFindingsCapturesNewEvidence(t *testing.T) {
	t.Parallel()

	content := "## Findings\n- **Location:** tracked.go:12\n- **Description:** Handler still lacks input validation\n- **Severity:** HIGH\n- **Recommended Fix:** Validate the payload before use\n- **New Evidence:** The unchanged handler path still trusts user input after the prior fix.\n"

	findings := parseFindings(content)
	require.Len(t, findings, 1)
	assert.Equal(t, "The unchanged handler path still trusts user input after the prior fix.", findings[0].NewEvidence)
}

func TestClassifyFindingsMergesRepeatedUnchangedActiveFinding(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, "tracked.go"), "package main\n\nfunc run() {\n\tpanic(\"boom\")\n}\n")

	known := decorateFindings(projectDir, []Finding{{
		Location:       "tracked.go:3",
		Description:    "Runner missing panic guard",
		Severity:       "HIGH",
		RecommendedFix: "Add a guard before the panic path",
		OriginCycle:    1,
	}}, 1)
	current := decorateFindings(projectDir, []Finding{{
		Location:       "tracked.go:3",
		Description:    "Runner still missing panic guard",
		Severity:       "HIGH",
		RecommendedFix: "Guard the panic path",
	}}, 2)

	classification := classifyFindings(known, current)
	assert.Len(t, classification.Resolved, 0)
	assert.Len(t, classification.NewFindings, 0)
	require.Len(t, classification.Persisting, 1)
	assert.Len(t, classification.RepeatedUnchanged, 1)
	assert.Equal(t, 1, classification.Persisting[0].OriginCycle)
	assert.Equal(t, 2, classification.Persisting[0].LastSeenCycle)
	assert.Equal(t, known[0].Description, classification.Persisting[0].Description)
}

func TestClassifyReopeningsSuppressesUnchangedWithoutNewEvidence(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, "tracked.go"), "package main\n\nfunc run() {\n\tpanic(\"boom\")\n}\n")

	resolved := decorateFindings(projectDir, []Finding{{
		Location:       "tracked.go:3",
		Description:    "Runner missing panic guard",
		Severity:       "HIGH",
		RecommendedFix: "Add a guard before the panic path",
		OriginCycle:    1,
	}}, 1)
	ledger := newResolvedLedger()
	ledger.add(resolved)

	reopened := decorateFindings(projectDir, []Finding{{
		Location:       "tracked.go:3",
		Description:    "Runner still missing panic guard",
		Severity:       "HIGH",
		RecommendedFix: "Guard the panic path",
	}}, 2)

	classification := classifyReopenings(reopened, ledger)
	assert.Len(t, classification.Admitted, 0)
	assert.Len(t, classification.Suppressed, 1)
	assert.Equal(t, 1, classification.SuppressedUnchanged)
	assert.Equal(t, 0, classification.ReopenedWithNewEvidence)
}

func TestClassifyReopeningsAllowsUnchangedWithNewEvidence(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, "tracked.go"), "package main\n\nfunc run() {\n\tpanic(\"boom\")\n}\n")

	resolved := decorateFindings(projectDir, []Finding{{
		Location:       "tracked.go:3",
		Description:    "Runner missing panic guard",
		Severity:       "HIGH",
		RecommendedFix: "Add a guard before the panic path",
		OriginCycle:    1,
	}}, 1)
	ledger := newResolvedLedger()
	ledger.add(resolved)

	reopened := decorateFindings(projectDir, []Finding{{
		Location:       "tracked.go:3",
		Description:    "Runner still missing panic guard",
		Severity:       "HIGH",
		RecommendedFix: "Guard the panic path",
		NewEvidence:    "The recovery wrapper never covers this code path, so the prior fix did not apply here.",
	}}, 2)

	classification := classifyReopenings(reopened, ledger)
	require.Len(t, classification.Admitted, 1)
	assert.Len(t, classification.Suppressed, 0)
	assert.Equal(t, 0, classification.SuppressedUnchanged)
	assert.Equal(t, 1, classification.ReopenedWithNewEvidence)
	assert.NotEmpty(t, classification.Admitted[0].ReopenOf)
}
