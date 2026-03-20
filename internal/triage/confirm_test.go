package triage

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yevgetman/fry/internal/epic"
)

func baseDecision() *TriageDecision {
	return &TriageDecision{
		Complexity:  ComplexityModerate,
		EffortLevel: epic.EffortMedium,
		Reason:      "REST endpoint with tests across 6 files.",
		SprintCount: 2,
	}
}

func TestConfirmDecision(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		decision   *TriageDecision
		wantErr    error
		wantComplex Complexity
		wantEffort epic.EffortLevel
	}{
		{
			name:        "accept with Y",
			input:       "Y\n",
			decision:    baseDecision(),
			wantComplex: ComplexityModerate,
			wantEffort:  epic.EffortMedium,
		},
		{
			name:        "accept with empty",
			input:       "\n",
			decision:    baseDecision(),
			wantComplex: ComplexityModerate,
			wantEffort:  epic.EffortMedium,
		},
		{
			name:        "accept with yes",
			input:       "yes\n",
			decision:    baseDecision(),
			wantComplex: ComplexityModerate,
			wantEffort:  epic.EffortMedium,
		},
		{
			name:     "decline with n",
			input:    "n\n",
			decision: baseDecision(),
			wantErr:  ErrTriageDeclined,
		},
		{
			name:     "decline with no",
			input:    "no\n",
			decision: baseDecision(),
			wantErr:  ErrTriageDeclined,
		},
		{
			name:     "decline with arbitrary text",
			input:    "blah\n",
			decision: baseDecision(),
			wantErr:  ErrTriageDeclined,
		},
		{
			name:     "decline on EOF",
			input:    "",
			decision: baseDecision(),
			wantErr:  ErrTriageDeclined,
		},
		{
			name:        "adjust keep both",
			input:       "a\n\n\n",
			decision:    baseDecision(),
			wantComplex: ComplexityModerate,
			wantEffort:  epic.EffortMedium,
		},
		{
			name:        "adjust difficulty to complex",
			input:       "a\ncomplex\n\n",
			decision:    baseDecision(),
			wantComplex: ComplexityComplex,
			wantEffort:  epic.EffortMedium,
		},
		{
			name:        "adjust effort to high",
			input:       "a\n\nhigh\n",
			decision:    baseDecision(),
			wantComplex: ComplexityModerate,
			wantEffort:  epic.EffortHigh,
		},
		{
			name:        "adjust both",
			input:       "a\nsimple\nlow\n",
			decision:    baseDecision(),
			wantComplex: ComplexitySimple,
			wantEffort:  epic.EffortLow,
		},
		{
			name:        "invalid difficulty keeps original",
			input:       "a\nEASY\n\n",
			decision:    baseDecision(),
			wantComplex: ComplexityModerate,
			wantEffort:  epic.EffortMedium,
		},
		{
			name:        "invalid effort keeps original",
			input:       "a\n\nextreme\n",
			decision:    baseDecision(),
			wantComplex: ComplexityModerate,
			wantEffort:  epic.EffortMedium,
		},
		{
			name: "max on simple keeps previous effort",
			input: "a\nsimple\nmax\n",
			decision: baseDecision(),
			wantComplex: ComplexitySimple,
			wantEffort:  epic.EffortMedium,
		},
		{
			name: "max on complex allowed",
			input: "a\ncomplex\nmax\n",
			decision: baseDecision(),
			wantComplex: ComplexityComplex,
			wantEffort:  epic.EffortMax,
		},
		{
			name:  "downgrade complex to moderate inheriting max downgrades to high",
			input: "a\nmoderate\n\n",
			decision: &TriageDecision{
				Complexity:  ComplexityComplex,
				EffortLevel: epic.EffortMax,
				Reason:      "Multi-service architecture.",
				SprintCount: 0,
			},
			wantComplex: ComplexityModerate,
			wantEffort:  epic.EffortHigh,
		},
		{
			name:        "case insensitive difficulty and effort",
			input:       "a\nSIMPLE\nHIGH\n",
			decision:    baseDecision(),
			wantComplex: ComplexitySimple,
			wantEffort:  epic.EffortHigh,
		},
		{
			name:     "EOF during adjustment difficulty",
			input:    "a\n",
			decision: baseDecision(),
			wantErr:  ErrTriageDeclined,
		},
		{
			name:     "EOF during adjustment effort",
			input:    "a\nsimple\n",
			decision: baseDecision(),
			wantErr:  ErrTriageDeclined,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			result, err := ConfirmDecision(ConfirmOpts{
				Decision: tt.decision,
				Stdin:    strings.NewReader(tt.input),
				Stdout:   &stdout,
			})

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantComplex, result.Complexity)
			assert.Equal(t, tt.wantEffort, result.EffortLevel)
		})
	}
}

func TestConfirmDecisionDisplay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		decision *TriageDecision
		contains []string
	}{
		{
			name: "simple display",
			decision: &TriageDecision{
				Complexity:  ComplexitySimple,
				EffortLevel: epic.EffortLow,
				Reason:      "Fix a typo in README.",
				SprintCount: 1,
			},
			contains: []string{
				"SIMPLE",
				"low",
				"Fix a typo in README.",
				"1-sprint epic programmatically",
			},
		},
		{
			name: "moderate display",
			decision: &TriageDecision{
				Complexity:  ComplexityModerate,
				EffortLevel: epic.EffortMedium,
				Reason:      "REST endpoint with tests.",
				SprintCount: 2,
			},
			contains: []string{
				"MODERATE",
				"medium",
				"REST endpoint with tests.",
				"2-sprint epic programmatically",
			},
		},
		{
			name: "complex display",
			decision: &TriageDecision{
				Complexity:  ComplexityComplex,
				EffortLevel: epic.EffortHigh,
				Reason:      "Multi-service architecture.",
				SprintCount: 0,
			},
			contains: []string{
				"COMPLEX",
				"high",
				"Multi-service architecture.",
				"full prepare pipeline",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			// Accept immediately so we just test display output.
			_, _ = ConfirmDecision(ConfirmOpts{
				Decision: tt.decision,
				Stdin:    strings.NewReader("Y\n"),
				Stdout:   &stdout,
			})

			output := stdout.String()
			for _, s := range tt.contains {
				assert.Contains(t, output, s)
			}
		})
	}
}

func TestConfirmDecisionWarnings(t *testing.T) {
	t.Parallel()

	t.Run("invalid difficulty shows warning", func(t *testing.T) {
		t.Parallel()
		var stdout bytes.Buffer
		result, err := ConfirmDecision(ConfirmOpts{
			Decision: baseDecision(),
			Stdin:    strings.NewReader("a\nEASY\n\n"),
			Stdout:   &stdout,
		})
		require.NoError(t, err)
		assert.Equal(t, ComplexityModerate, result.Complexity)
		assert.Contains(t, stdout.String(), "Invalid difficulty")
	})

	t.Run("invalid effort shows warning", func(t *testing.T) {
		t.Parallel()
		var stdout bytes.Buffer
		result, err := ConfirmDecision(ConfirmOpts{
			Decision: baseDecision(),
			Stdin:    strings.NewReader("a\n\nextreme\n"),
			Stdout:   &stdout,
		})
		require.NoError(t, err)
		assert.Equal(t, epic.EffortMedium, result.EffortLevel)
		assert.Contains(t, stdout.String(), "Invalid effort")
	})

	t.Run("max on simple shows warning", func(t *testing.T) {
		t.Parallel()
		var stdout bytes.Buffer
		result, err := ConfirmDecision(ConfirmOpts{
			Decision: baseDecision(),
			Stdin:    strings.NewReader("a\nsimple\nmax\n"),
			Stdout:   &stdout,
		})
		require.NoError(t, err)
		assert.Equal(t, epic.EffortMedium, result.EffortLevel)
		assert.Contains(t, stdout.String(), "reserved for complex tasks")
	})

	t.Run("downgrade complex inheriting max shows warning", func(t *testing.T) {
		t.Parallel()
		var stdout bytes.Buffer
		result, err := ConfirmDecision(ConfirmOpts{
			Decision: &TriageDecision{
				Complexity:  ComplexityComplex,
				EffortLevel: epic.EffortMax,
				Reason:      "Multi-service architecture.",
			},
			Stdin:  strings.NewReader("a\nmoderate\n\n"),
			Stdout: &stdout,
		})
		require.NoError(t, err)
		assert.Equal(t, epic.EffortHigh, result.EffortLevel)
		assert.Contains(t, stdout.String(), "reserved for complex tasks")
	})
}

func TestParseComplexityInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		want    Complexity
		wantErr bool
	}{
		{"simple", ComplexitySimple, false},
		{"SIMPLE", ComplexitySimple, false},
		{"Simple", ComplexitySimple, false},
		{"moderate", ComplexityModerate, false},
		{"MODERATE", ComplexityModerate, false},
		{"complex", ComplexityComplex, false},
		{"COMPLEX", ComplexityComplex, false},
		{"  simple  ", ComplexitySimple, false},
		{"", "", true},
		{"easy", "", true},
		{"hard", "", true},
	}

	for _, tt := range tests {
		name := tt.input
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := parseComplexityInput(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestActionDescription(t *testing.T) {
	t.Parallel()

	assert.Contains(t, actionDescription(ComplexitySimple, 1), "1-sprint")
	assert.Contains(t, actionDescription(ComplexityModerate, 2), "2-sprint")
	assert.Contains(t, actionDescription(ComplexityModerate, 0), "1-sprint")
	assert.Contains(t, actionDescription(ComplexityComplex, 0), "full prepare")
}
