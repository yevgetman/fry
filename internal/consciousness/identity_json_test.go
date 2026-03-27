package consciousness

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadIdentityJSON(t *testing.T) {
	t.Parallel()

	id, err := LoadIdentityJSON()
	require.NoError(t, err)
	require.NotNil(t, id, "identity.json should exist in embedded templates")

	assert.Equal(t, 1, id.Version)
	assert.Equal(t, 0, id.MemoriesIntegrated)
	assert.NotEmpty(t, id.Core.SelfKnowledge)
	assert.Contains(t, id.Core.SelfKnowledge, "Fry")
	assert.Len(t, id.Core.Values, 4)
	assert.Len(t, id.Disposition.Tendencies, 9)
}

func TestLoadIdentityJSON_Values(t *testing.T) {
	t.Parallel()

	id, err := LoadIdentityJSON()
	require.NoError(t, err)
	require.NotNil(t, id)

	// All seed values should have confidence 1.0
	for _, v := range id.Core.Values {
		assert.Equal(t, 1.0, v.Confidence, "seed value confidence should be 1.0: %s", v.Statement)
		assert.Equal(t, 0, v.Reinforcements, "seed value reinforcements should be 0")
		assert.NotEmpty(t, v.Statement)
	}
}

func TestLoadIdentityJSON_Tendencies(t *testing.T) {
	t.Parallel()

	id, err := LoadIdentityJSON()
	require.NoError(t, err)
	require.NotNil(t, id)

	// All seed tendencies should have confidence 0.85 and a valid category
	validCategories := map[string]bool{
		"process": true, "tooling": true, "architecture": true,
		"testing": true, "review": true, "audit": true,
		"healing": true, "planning": true, "domain": true,
	}

	for _, t2 := range id.Disposition.Tendencies {
		assert.Equal(t, 0.85, t2.Confidence, "seed tendency confidence should be 0.85: %s", t2.Statement)
		assert.True(t, validCategories[t2.Category], "invalid category %q for tendency: %s", t2.Category, t2.Statement)
		assert.NotEmpty(t, t2.Statement)
	}
}

func TestRenderIdentityForPrompt(t *testing.T) {
	t.Parallel()

	id := &IdentityJSON{
		Version:            1,
		MemoriesIntegrated: 100,
		Core: IdentityCore{
			SelfKnowledge: "I am a test identity.",
			Values: []IdentityElement{
				{Statement: "Value one.", Confidence: 0.9, Reinforcements: 5},
				{Statement: "Value two.", Confidence: 0.8, Reinforcements: 3},
			},
		},
		Disposition: IdentityDisposition{
			Tendencies: []IdentityTendency{
				{Statement: "Tendency alpha.", Confidence: 0.7, Reinforcements: 2, Category: "process"},
			},
		},
		Domains: map[string]IdentityDomain{
			"api-backend": {
				Tendencies: []IdentityTendency{
					{Statement: "API insight.", Confidence: 0.6, Reinforcements: 1, Category: "architecture"},
				},
			},
		},
	}

	result := RenderIdentityForPrompt(id)

	assert.Contains(t, result, "# Fry — Core Identity")
	assert.Contains(t, result, "I am a test identity.")
	assert.Contains(t, result, "Value one.")
	assert.Contains(t, result, "Value two.")
	assert.Contains(t, result, "# Fry — Disposition")
	assert.Contains(t, result, "Tendency alpha.")
	assert.Contains(t, result, "# Fry — Domain: api-backend")
	assert.Contains(t, result, "API insight.")
}

func TestRenderDispositionForPrompt(t *testing.T) {
	t.Parallel()

	id := &IdentityJSON{
		Core: IdentityCore{
			SelfKnowledge: "I am Fry.",
			Values:        []IdentityElement{{Statement: "A value.", Confidence: 1.0, Reinforcements: 0}},
		},
		Disposition: IdentityDisposition{
			Tendencies: []IdentityTendency{
				{Statement: "Tendency one.", Confidence: 0.8, Reinforcements: 1, Category: "process"},
				{Statement: "Tendency two.", Confidence: 0.7, Reinforcements: 0, Category: "testing"},
			},
		},
	}

	result := RenderDispositionForPrompt(id)

	assert.Contains(t, result, "# Fry — Disposition")
	assert.Contains(t, result, "Tendency one.")
	assert.Contains(t, result, "Tendency two.")
	// Should NOT contain core identity
	assert.NotContains(t, result, "I am Fry.")
	assert.NotContains(t, result, "A value.")
}

func TestRenderIdentityForPrompt_EmptyDomains(t *testing.T) {
	t.Parallel()

	id := &IdentityJSON{
		Core: IdentityCore{
			SelfKnowledge: "Test.",
			Values:        []IdentityElement{{Statement: "V.", Confidence: 1.0, Reinforcements: 0}},
		},
		Disposition: IdentityDisposition{
			Tendencies: []IdentityTendency{
				{Statement: "T.", Confidence: 0.8, Reinforcements: 0, Category: "process"},
			},
		},
		Domains: map[string]IdentityDomain{},
	}

	result := RenderIdentityForPrompt(id)
	assert.NotContains(t, result, "Domain:")
}
