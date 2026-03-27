package consciousness

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/templates"
)

// IdentityJSON represents the structured identity format produced by Reflection.
// This replaces the .md-based identity files with a format optimized for both
// LLM consumption and programmatic updates by the Reflection pipeline.
type IdentityJSON struct {
	Version            int                       `json:"version"`
	LastReflection     *string                   `json:"last_reflection"`
	MemoriesIntegrated int                       `json:"memories_integrated"`
	Core               IdentityCore              `json:"core"`
	Disposition        IdentityDisposition       `json:"disposition"`
	Domains            map[string]IdentityDomain `json:"domains"`
}

// IdentityCore contains Fry's fundamental self-knowledge and values.
type IdentityCore struct {
	SelfKnowledge string            `json:"self_knowledge"`
	Values        []IdentityElement `json:"values"`
}

// IdentityDisposition contains behavioral tendencies derived from experience.
type IdentityDisposition struct {
	Tendencies []IdentityTendency `json:"tendencies"`
}

// IdentityDomain contains domain-specific wisdom for a particular area.
type IdentityDomain struct {
	Tendencies []IdentityTendency `json:"tendencies"`
}

// IdentityElement is a single identity statement with confidence metadata.
type IdentityElement struct {
	Statement      string  `json:"statement"`
	Confidence     float64 `json:"confidence"`
	Reinforcements int     `json:"reinforcements"`
}

// IdentityTendency is an identity element with a memory category.
type IdentityTendency struct {
	Statement      string  `json:"statement"`
	Confidence     float64 `json:"confidence"`
	Reinforcements int     `json:"reinforcements"`
	Category       string  `json:"category"`
}

// LoadIdentityJSON attempts to load identity.json from embedded templates.
// Returns nil and no error if the file does not exist (pre-reflection state).
func LoadIdentityJSON() (*IdentityJSON, error) {
	data, err := fs.ReadFile(templates.TemplateFS, config.IdentityJSONFile)
	if err != nil {
		// File doesn't exist yet — not an error, just pre-reflection state
		return nil, nil
	}

	var id IdentityJSON
	if err := json.Unmarshal(data, &id); err != nil {
		return nil, fmt.Errorf("parse identity.json: %w", err)
	}
	return &id, nil
}

// RenderIdentityForPrompt converts IdentityJSON into a markdown string suitable
// for injection into agent prompts. This replaces the raw .md identity files
// with a rendered view of the structured JSON identity.
func RenderIdentityForPrompt(id *IdentityJSON) string {
	var b strings.Builder

	// Core identity
	b.WriteString("# Fry — Core Identity\n\n")
	b.WriteString(id.Core.SelfKnowledge)
	b.WriteString("\n\n## What I Value\n\n")
	for _, v := range id.Core.Values {
		b.WriteString("- ")
		b.WriteString(v.Statement)
		b.WriteByte('\n')
	}

	// Disposition
	b.WriteString("\n# Fry — Disposition\n\n")
	b.WriteString(renderTendencies(id.Disposition.Tendencies))

	// Domains
	for name, domain := range id.Domains {
		if len(domain.Tendencies) == 0 {
			continue
		}
		b.WriteString("\n# Fry — Domain: ")
		b.WriteString(name)
		b.WriteString("\n\n")
		b.WriteString(renderTendencies(domain.Tendencies))
	}

	return b.String()
}

// RenderDispositionForPrompt extracts only the disposition tendencies formatted
// as markdown. This is injected into sprint agent prompts to subtly influence
// behavior, matching the role of the old disposition.md file.
func RenderDispositionForPrompt(id *IdentityJSON) string {
	var b strings.Builder
	b.WriteString("# Fry — Disposition\n\n")
	b.WriteString("These are behavioral tendencies that have emerged from accumulated build experience.\n\n")
	b.WriteString(renderTendencies(id.Disposition.Tendencies))
	return b.String()
}

func renderTendencies(tendencies []IdentityTendency) string {
	var b strings.Builder
	for _, t := range tendencies {
		b.WriteString("- ")
		b.WriteString(t.Statement)
		b.WriteByte('\n')
	}
	return b.String()
}
