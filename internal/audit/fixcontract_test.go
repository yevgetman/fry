package audit

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFixContractExtractsIssueIDsAndTargets(t *testing.T) {
	t.Parallel()

	contract := newFixContract([]Finding{
		{Location: "internal/audit/audit.go:123", Description: "Issue A", RecommendedFix: "Add the missing guard clause."},
		{Description: "Issue B"},
	})

	require.Len(t, contract.Issues, 2)
	assert.Equal(t, []int{1, 2}, contract.IssueIDs())
	assert.Equal(t, []string{"internal/audit/audit.go"}, contract.TargetFiles())
	assert.Contains(t, contract.Issues[0].ExpectedEvidence, "Add the missing guard clause")
	assert.Len(t, contract.Issues[1].TargetFiles, 0)
}

func TestAssessFixDiffRejectsCommentOnlyChanges(t *testing.T) {
	t.Parallel()

	contract := newFixContract([]Finding{{Location: "internal/audit/audit.go:123", Description: "Issue A"}})
	diff := stringsJoin(
		"diff --git a/internal/audit/audit.go b/internal/audit/audit.go",
		"--- a/internal/audit/audit.go",
		"+++ b/internal/audit/audit.go",
		"@@ -1,2 +1,2 @@",
		"-// old note",
		"+// new note",
	)

	assessment := assessFixDiff(contract, diff, "", "")
	assert.Equal(t, diffClassificationCommentOnly, assessment.DiffClassification)
	assert.Equal(t, fixValidationRejected, assessment.ValidationResult)
	assert.Equal(t, []string{"internal/audit/audit.go"}, assessment.ChangedFiles)
}

func TestAssessFixDiffRoutesAlreadyFixedClaimToVerify(t *testing.T) {
	t.Parallel()

	contract := newFixContract([]Finding{{Location: "internal/audit/audit.go:123", Description: "Issue A"}})

	assessment := assessFixDiff(contract, "", "", "No changes needed. This is already fixed.")
	assert.Equal(t, diffClassificationEmpty, assessment.DiffClassification)
	assert.Equal(t, fixValidationVerifyOnly, assessment.ValidationResult)
	assert.True(t, assessment.AlreadyFixedClaim)
}

func TestAssessFixDiffRejectsOutOfScopeChanges(t *testing.T) {
	t.Parallel()

	contract := newFixContract([]Finding{{Location: "internal/audit/audit.go:123", Description: "Issue A"}})
	diff := stringsJoin(
		"diff --git a/internal/cli/root.go b/internal/cli/root.go",
		"--- a/internal/cli/root.go",
		"+++ b/internal/cli/root.go",
		"@@ -1,2 +1,2 @@",
		"-oldLine()",
		"+newLine()",
	)

	assessment := assessFixDiff(contract, diff, "", "")
	assert.Equal(t, diffClassificationOutOfScope, assessment.DiffClassification)
	assert.Equal(t, fixValidationRejected, assessment.ValidationResult)
}

func TestAssessFixDiffDetectsOutOfScopeFilesInAcceptedDiff(t *testing.T) {
	t.Parallel()

	contract := newFixContract([]Finding{{Location: "internal/audit/audit.go:123", Description: "Issue A"}})
	diff := stringsJoin(
		"diff --git a/internal/audit/audit.go b/internal/audit/audit.go",
		"--- a/internal/audit/audit.go",
		"+++ b/internal/audit/audit.go",
		"@@ -1,2 +1,2 @@",
		"-oldLine()",
		"+newLine()",
		"diff --git a/internal/cli/root.go b/internal/cli/root.go",
		"--- a/internal/cli/root.go",
		"+++ b/internal/cli/root.go",
		"@@ -1,2 +1,2 @@",
		"-oldCliLine()",
		"+newCliLine()",
	)

	assessment := assessFixDiff(contract, diff, "", "")
	assert.Equal(t, diffClassificationBehavioral, assessment.DiffClassification)
	assert.Equal(t, fixValidationAccepted, assessment.ValidationResult)
	assert.Equal(t, []string{"internal/audit/audit.go", "internal/cli/root.go"}, assessment.ChangedFiles)
	assert.Equal(t, []string{"internal/cli/root.go"}, assessment.OutOfScopeFiles)
}

func TestAssessFixDiffNoOutOfScopeFilesWhenAllInScope(t *testing.T) {
	t.Parallel()

	contract := newFixContract([]Finding{{Location: "internal/audit/audit.go:123", Description: "Issue A"}})
	diff := stringsJoin(
		"diff --git a/internal/audit/audit.go b/internal/audit/audit.go",
		"--- a/internal/audit/audit.go",
		"+++ b/internal/audit/audit.go",
		"@@ -1,2 +1,2 @@",
		"-oldLine()",
		"+newLine()",
	)

	assessment := assessFixDiff(contract, diff, "", "")
	assert.Equal(t, fixValidationAccepted, assessment.ValidationResult)
	assert.Empty(t, assessment.OutOfScopeFiles)
}

func TestOutOfScopeFilesHelper(t *testing.T) {
	t.Parallel()

	targets := []string{"a.go", "b.go"}
	changed := []string{"a.go", "c.go", "d.go"}
	result := outOfScopeFiles(targets, changed)
	assert.Equal(t, []string{"c.go", "d.go"}, result)
}

func TestOutOfScopeFilesHelperAllInScope(t *testing.T) {
	t.Parallel()

	targets := []string{"a.go", "b.go"}
	changed := []string{"a.go"}
	result := outOfScopeFiles(targets, changed)
	assert.Empty(t, result)
}

func stringsJoin(lines ...string) string {
	return strings.Join(lines, "\n")
}
