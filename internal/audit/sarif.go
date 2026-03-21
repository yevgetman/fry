package audit

import (
	"encoding/json"
	"fmt"

	"github.com/yevgetman/fry/internal/config"
)

// SARIF 2.1.0 schema types.
// Spec: https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html

// SARIFLog is the top-level SARIF document.
type SARIFLog struct {
	Version string     `json:"version"`
	Schema  string     `json:"$schema"`
	Runs    []SARIFRun `json:"runs"`
}

// SARIFRun represents a single analysis run.
type SARIFRun struct {
	Tool    SARIFTool     `json:"tool"`
	Results []SARIFResult `json:"results"`
}

// SARIFTool identifies the analysis tool.
type SARIFTool struct {
	Driver SARIFDriver `json:"driver"`
}

// SARIFDriver holds tool metadata.
type SARIFDriver struct {
	Name           string `json:"name"`
	Version        string `json:"version"`
	InformationURI string `json:"informationUri,omitempty"`
}

// SARIFResult represents a single finding.
type SARIFResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   SARIFMessage    `json:"message"`
	Locations []SARIFLocation `json:"locations,omitempty"`
}

// SARIFMessage holds the human-readable finding text.
type SARIFMessage struct {
	Text string `json:"text"`
}

// SARIFLocation identifies where a finding occurs.
type SARIFLocation struct {
	PhysicalLocation SARIFPhysicalLocation `json:"physicalLocation"`
}

// SARIFPhysicalLocation wraps the artifact location.
type SARIFPhysicalLocation struct {
	ArtifactLocation SARIFArtifactLocation `json:"artifactLocation"`
}

// SARIFArtifactLocation holds the URI of the affected file.
type SARIFArtifactLocation struct {
	URI string `json:"uri"`
}

// severityToSARIFLevel maps audit severity to a SARIF level string.
// SARIF levels: "error", "warning", "note", "none".
func severityToSARIFLevel(severity string) string {
	switch severity {
	case "CRITICAL", "HIGH":
		return "error"
	case "MODERATE":
		return "warning"
	case "LOW":
		return "note"
	default:
		return "warning"
	}
}

// ConvertToSARIF converts audit findings to a SARIF 2.1.0 JSON document.
// An empty findings slice produces a valid SARIF document with an empty results array.
func ConvertToSARIF(findings []Finding) ([]byte, error) {
	results := make([]SARIFResult, 0, len(findings))
	for i, f := range findings {
		ruleID := fmt.Sprintf("FRY%04d", i+1)

		text := f.Description
		if f.RecommendedFix != "" {
			text = fmt.Sprintf("%s\n\nRecommended fix: %s", f.Description, f.RecommendedFix)
		}

		result := SARIFResult{
			RuleID:  ruleID,
			Level:   severityToSARIFLevel(f.Severity),
			Message: SARIFMessage{Text: text},
		}

		if f.Location != "" {
			result.Locations = []SARIFLocation{
				{
					PhysicalLocation: SARIFPhysicalLocation{
						ArtifactLocation: SARIFArtifactLocation{URI: f.Location},
					},
				},
			}
		}

		results = append(results, result)
	}

	log := SARIFLog{
		Version: "2.1.0",
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
		Runs: []SARIFRun{
			{
				Tool: SARIFTool{
					Driver: SARIFDriver{
						Name:    "fry",
						Version: config.Version,
					},
				},
				Results: results,
			},
		},
	}

	return json.MarshalIndent(log, "", "  ")
}
