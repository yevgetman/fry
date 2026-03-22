package audit

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertToSARIFEmptyFindings(t *testing.T) {
	t.Parallel()

	data, err := ConvertToSARIF(nil)
	require.NoError(t, err)

	var log SARIFLog
	require.NoError(t, json.Unmarshal(data, &log))

	assert.Equal(t, "2.1.0", log.Version)
	require.Len(t, log.Runs, 1)
	assert.Empty(t, log.Runs[0].Results)
	assert.Equal(t, "fry", log.Runs[0].Tool.Driver.Name)
}

func TestConvertToSARIFSingleFinding(t *testing.T) {
	t.Parallel()

	findings := []Finding{
		{
			Location:       "src/main.go:10",
			Description:    "Null pointer dereference",
			Severity:       "CRITICAL",
			RecommendedFix: "Add nil check before dereferencing",
		},
	}

	data, err := ConvertToSARIF(findings)
	require.NoError(t, err)

	var log SARIFLog
	require.NoError(t, json.Unmarshal(data, &log))

	require.Len(t, log.Runs, 1)
	require.Len(t, log.Runs[0].Results, 1)

	result := log.Runs[0].Results[0]
	assert.Equal(t, "FRY0001", result.RuleID)
	assert.Equal(t, "error", result.Level, "CRITICAL should map to error")
	assert.Contains(t, result.Message.Text, "Null pointer dereference")
	assert.Contains(t, result.Message.Text, "Add nil check before dereferencing")
	require.Len(t, result.Locations, 1)
	assert.Equal(t, "src/main.go:10", result.Locations[0].PhysicalLocation.ArtifactLocation.URI)
}

// TestConvertToSARIFEmpty verifies that an empty findings slice produces a
// valid SARIF document with an empty results array.
func TestConvertToSARIFEmpty(t *testing.T) {
	t.Parallel()

	data, err := ConvertToSARIF([]Finding{})
	require.NoError(t, err)

	var log SARIFLog
	require.NoError(t, json.Unmarshal(data, &log))
	require.Len(t, log.Runs, 1)
	assert.Empty(t, log.Runs[0].Results)
}

// TestConvertToSARIFSeverityMapping verifies that each severity level maps to
// the correct SARIF level string.
func TestConvertToSARIFSeverityMapping(t *testing.T) {
	t.Parallel()

	findings := []Finding{
		{Description: "critical", Severity: "CRITICAL"},
		{Description: "high", Severity: "HIGH"},
		{Description: "moderate", Severity: "MODERATE"},
		{Description: "low", Severity: "LOW"},
	}
	data, err := ConvertToSARIF(findings)
	require.NoError(t, err)

	var log SARIFLog
	require.NoError(t, json.Unmarshal(data, &log))
	results := log.Runs[0].Results
	require.Len(t, results, 4)
	assert.Equal(t, "error", results[0].Level, "CRITICAL → error")
	assert.Equal(t, "error", results[1].Level, "HIGH → error")
	assert.Equal(t, "warning", results[2].Level, "MODERATE → warning")
	assert.Equal(t, "note", results[3].Level, "LOW → note")
}

// TestConvertToSARIFWithLocation verifies that a finding with a non-empty
// Location field populates the SARIF locations array.
func TestConvertToSARIFWithLocation(t *testing.T) {
	t.Parallel()

	findings := []Finding{
		{Description: "issue", Severity: "HIGH", Location: "internal/foo/bar.go:42"},
	}
	data, err := ConvertToSARIF(findings)
	require.NoError(t, err)

	var log SARIFLog
	require.NoError(t, json.Unmarshal(data, &log))
	result := log.Runs[0].Results[0]
	require.Len(t, result.Locations, 1)
	assert.Equal(t, "internal/foo/bar.go:42", result.Locations[0].PhysicalLocation.ArtifactLocation.URI)
}

// TestConvertToSARIFWithoutFix verifies that a finding with empty RecommendedFix
// produces a message containing only the description (no "Recommended fix:" suffix).
func TestConvertToSARIFWithoutFix(t *testing.T) {
	t.Parallel()

	findings := []Finding{
		{Description: "description only", Severity: "LOW", RecommendedFix: ""},
	}
	data, err := ConvertToSARIF(findings)
	require.NoError(t, err)

	var log SARIFLog
	require.NoError(t, json.Unmarshal(data, &log))
	text := log.Runs[0].Results[0].Message.Text
	assert.Equal(t, "description only", text)
	assert.NotContains(t, text, "Recommended fix:")
}

// TestConvertToSARIFRuleIDFormat verifies that the first result has ruleId "FRY0001".
func TestConvertToSARIFRuleIDFormat(t *testing.T) {
	t.Parallel()

	findings := []Finding{
		{Description: "first finding", Severity: "LOW"},
	}
	data, err := ConvertToSARIF(findings)
	require.NoError(t, err)

	var log SARIFLog
	require.NoError(t, json.Unmarshal(data, &log))
	assert.Equal(t, "FRY0001", log.Runs[0].Results[0].RuleID)
}

func TestConvertToSARIFMultipleSeverities(t *testing.T) {
	t.Parallel()

	findings := []Finding{
		{Description: "Critical issue", Severity: "CRITICAL", Location: "a.go:1"},
		{Description: "High issue", Severity: "HIGH", Location: "b.go:2"},
		{Description: "Moderate issue", Severity: "MODERATE", Location: "c.go:3"},
		{Description: "Low issue", Severity: "LOW", Location: "d.go:4"},
	}

	data, err := ConvertToSARIF(findings)
	require.NoError(t, err)

	var log SARIFLog
	require.NoError(t, json.Unmarshal(data, &log))

	require.Len(t, log.Runs, 1)
	results := log.Runs[0].Results
	require.Len(t, results, 4)

	assert.Equal(t, "error", results[0].Level, "CRITICAL → error")
	assert.Equal(t, "error", results[1].Level, "HIGH → error")
	assert.Equal(t, "warning", results[2].Level, "MODERATE → warning")
	assert.Equal(t, "note", results[3].Level, "LOW → note")

	// Rule IDs are sequential
	assert.Equal(t, "FRY0001", results[0].RuleID)
	assert.Equal(t, "FRY0002", results[1].RuleID)
	assert.Equal(t, "FRY0003", results[2].RuleID)
	assert.Equal(t, "FRY0004", results[3].RuleID)
}
