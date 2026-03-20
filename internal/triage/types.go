package triage

// Complexity represents the triage classifier's assessment of task difficulty.
type Complexity string

const (
	ComplexitySimple   Complexity = "SIMPLE"
	ComplexityModerate Complexity = "MODERATE"
	ComplexityComplex  Complexity = "COMPLEX"
)

// TriageDecision is the parsed result from the triage classifier LLM call.
type TriageDecision struct {
	Complexity  Complexity
	Reason      string
	SprintCount int // 1 for simple, 1-2 for moderate, 0 for complex (defer to full prepare)
}
