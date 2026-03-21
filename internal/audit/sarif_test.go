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
