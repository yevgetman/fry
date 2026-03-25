package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/epic"
)

// --- stub engine ---

type stubEngine struct {
	name       string
	outputs    []string
	prompts    []string
	sideEffect func(projectDir string, callIndex int)
	callIndex  int
}

func (s *stubEngine) Run(_ context.Context, prompt string, opts engine.RunOpts) (string, int, error) {
	s.prompts = append(s.prompts, prompt)
	var output string
	if len(s.outputs) > 0 {
		output = s.outputs[0]
		s.outputs = s.outputs[1:]
	}
	if s.sideEffect != nil {
		s.sideEffect(opts.WorkDir, s.callIndex)
	}
	s.callIndex++
	if opts.Stdout != nil {
		_, _ = opts.Stdout.Write([]byte(output))
	}
	return output, 0, nil
}

func (s *stubEngine) Name() string {
	return s.name
}

// --- helpers ---

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func makeOpts(t *testing.T, eng engine.Engine) AuditOpts {
	t.Helper()
	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, config.SprintProgressFile), "Built the feature.\n")
	return AuditOpts{
		ProjectDir: projectDir,
		Sprint: &epic.Sprint{
			Number:        1,
			Name:          "Setup",
			MaxIterations: 2,
			Promise:       "DONE",
			Prompt:        "Build the setup sprint.",
		},
		Epic: &epic.Epic{
			TotalSprints:       3,
			AuditAfterSprint:   true,
			MaxAuditIterations: 3,
		},
		Engine:  eng,
		GitDiff: "+new line\n-old line\n",
		Verbose: false,
	}
}

// Standard audit findings content used by multiple tests.
const criticalFindings = "## Summary\nBad stuff.\n\n## Findings\n- **Location:** src/main.go:10\n- **Description:** Null pointer dereference\n- **Severity:** CRITICAL\n- **Recommended Fix:** Add nil check\n\n## Verdict\nFAIL\n"
const highFindings = "## Summary\nBugs found.\n\n## Findings\n- **Location:** src/api.go:20\n- **Description:** Missing error handling\n- **Severity:** HIGH\n- **Recommended Fix:** Handle error\n\n## Verdict\nFAIL\n"
const moderateFindings = "## Summary\nMinor issues.\n\n## Findings\n- **Location:** src/util.go:5\n- **Description:** Edge case not handled\n- **Severity:** MODERATE\n- **Recommended Fix:** Add boundary check\n\n## Verdict\nFAIL\n"
const cleanAudit = "## Summary\nAll good.\n\n## Findings\nNone.\n\n## Verdict\nPASS\n"

// --- Finding type tests ---

func TestFindingKey(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "sql injection", Finding{Description: "SQL Injection"}.key())
	assert.Equal(t, "sql injection", Finding{Description: "  SQL Injection  "}.key())
	assert.Equal(t, "sql injection", Finding{Description: "sql injection"}.key())
}

func TestFindingIsActionable(t *testing.T) {
	t.Parallel()

	assert.True(t, Finding{Description: "x", Severity: "CRITICAL"}.isActionable())
	assert.True(t, Finding{Description: "x", Severity: "HIGH"}.isActionable())
	assert.True(t, Finding{Description: "x", Severity: "MODERATE"}.isActionable())
	assert.False(t, Finding{Description: "x", Severity: "LOW"}.isActionable())
	assert.False(t, Finding{Description: "x", Severity: ""}.isActionable())
	assert.False(t, Finding{Description: "x", Severity: "HIGH", Resolved: true}.isActionable())
}

// --- parseFindings tests ---

func TestParseFindings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expected []Finding
	}{
		{
			name: "standard format with location",
			content: "## Findings\n- **Location:** src/handler.go:42\n- **Description:** SQL injection\n- **Severity:** HIGH\n- **Recommended Fix:** Use parameterized queries\n",
			expected: []Finding{
				{Location: "src/handler.go:42", Description: "SQL injection", Severity: "HIGH", RecommendedFix: "Use parameterized queries"},
			},
		},
		{
			name: "multiple findings",
			content: "- **Location:** a.go:1\n- **Description:** Issue A\n- **Severity:** HIGH\n- **Location:** b.go:2\n- **Description:** Issue B\n- **Severity:** MODERATE\n",
			expected: []Finding{
				{Location: "a.go:1", Description: "Issue A", Severity: "HIGH"},
				{Location: "b.go:2", Description: "Issue B", Severity: "MODERATE"},
			},
		},
		{
			name: "no location",
			content: "- **Description:** Missing validation\n- **Severity:** MODERATE\n",
			expected: []Finding{
				{Description: "Missing validation", Severity: "MODERATE"},
			},
		},
		{
			name: "description only no severity",
			content: "- **Description:** Some issue\n",
			expected: []Finding{
				{Description: "Some issue"},
			},
		},
		{
			name:     "no findings",
			content:  "## Summary\nAll good.\n## Verdict\nPASS\n",
			expected: nil,
		},
		{
			name:     "empty content",
			content:  "",
			expected: nil,
		},
		{
			name: "consecutive descriptions without location",
			content: "- **Description:** Issue A\n- **Severity:** HIGH\n- **Description:** Issue B\n- **Severity:** LOW\n",
			expected: []Finding{
				{Description: "Issue A", Severity: "HIGH"},
				{Description: "Issue B", Severity: "LOW"},
			},
		},
		{
			name: "plain format without bold",
			content: "- Location: file.go:10\n- Description: Buffer overflow\n- Severity: CRITICAL\n- Recommended Fix: Bounds check\n",
			expected: []Finding{
				{Location: "file.go:10", Description: "Buffer overflow", Severity: "CRITICAL", RecommendedFix: "Bounds check"},
			},
		},
		{
			name: "word boundary severity parsing",
			content: "- **Description:** HIGHLY unusual pattern\n- **Severity:** LOW\n",
			expected: []Finding{
				{Description: "HIGHLY unusual pattern", Severity: "LOW"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := parseFindings(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- parseVerificationStatuses tests ---

func TestParseVerificationStatuses(t *testing.T) {
	t.Parallel()

	findings := []Finding{
		{Description: "Issue A"},
		{Description: "Issue B"},
		{Description: "Issue C"},
	}

	tests := []struct {
		name     string
		content  string
		expected []bool
	}{
		{
			name:     "all resolved",
			content:  "- **Issue:** 1\n- **Status:** RESOLVED\n- **Issue:** 2\n- **Status:** RESOLVED\n- **Issue:** 3\n- **Status:** RESOLVED\n",
			expected: []bool{true, true, true},
		},
		{
			name:     "partial resolution",
			content:  "- **Issue:** 1\n- **Status:** RESOLVED\n- **Issue:** 2\n- **Status:** STILL PRESENT\n- **Issue:** 3\n- **Status:** RESOLVED\n",
			expected: []bool{true, false, true},
		},
		{
			name:     "none resolved",
			content:  "- **Issue:** 1\n- **Status:** STILL PRESENT\n- **Issue:** 2\n- **Status:** STILL PRESENT\n- **Issue:** 3\n- **Status:** STILL PRESENT\n",
			expected: []bool{false, false, false},
		},
		{
			name:     "empty content",
			content:  "",
			expected: []bool{false, false, false},
		},
		{
			name:     "no parseable format",
			content:  "## Findings\n- **Severity:** CRITICAL\n",
			expected: []bool{false, false, false},
		},
		{
			name:     "issue and status on same line",
			content:  "**Issue:** 1 **Status:** RESOLVED\n**Issue:** 2 **Status:** STILL PRESENT\n",
			expected: []bool{true, false, false},
		},
		{
			name:     "out of range issue number ignored",
			content:  "- **Issue:** 99\n- **Status:** RESOLVED\n- **Issue:** 1\n- **Status:** RESOLVED\n",
			expected: []bool{true, false, false},
		},
		{
			name:     "plain format without bold",
			content:  "- Issue: 2\n- Status: RESOLVED\n",
			expected: []bool{false, true, false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := parseVerificationStatuses(tt.content, findings)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- classifyFindings tests ---

func TestClassifyFindings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		known          []Finding
		current        []Finding
		wantResolved   int
		wantPersisting int
		wantNew        int
	}{
		{
			name:           "all resolved",
			known:          []Finding{{Description: "A"}, {Description: "B"}},
			current:        []Finding{},
			wantResolved:   2,
			wantPersisting: 0,
			wantNew:        0,
		},
		{
			name:           "all persisting",
			known:          []Finding{{Description: "A"}, {Description: "B"}},
			current:        []Finding{{Description: "A"}, {Description: "B"}},
			wantResolved:   0,
			wantPersisting: 2,
			wantNew:        0,
		},
		{
			name:           "all new",
			known:          []Finding{},
			current:        []Finding{{Description: "X"}, {Description: "Y"}},
			wantResolved:   0,
			wantPersisting: 0,
			wantNew:        2,
		},
		{
			name:           "mixed",
			known:          []Finding{{Description: "A", OriginCycle: 1}, {Description: "B", OriginCycle: 1}},
			current:        []Finding{{Description: "A"}, {Description: "C"}},
			wantResolved:   1, // B resolved
			wantPersisting: 1, // A persists
			wantNew:        1, // C is new
		},
		{
			name:           "case insensitive match",
			known:          []Finding{{Description: "SQL Injection", OriginCycle: 1}},
			current:        []Finding{{Description: "sql injection"}},
			wantResolved:   0,
			wantPersisting: 1,
			wantNew:        0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resolved, persisting, newFindings := classifyFindings(tt.known, tt.current)
			assert.Equal(t, tt.wantResolved, len(resolved), "resolved count")
			assert.Equal(t, tt.wantPersisting, len(persisting), "persisting count")
			assert.Equal(t, tt.wantNew, len(newFindings), "new count")
		})
	}
}

func TestClassifyFindingsPreservesOriginCycle(t *testing.T) {
	t.Parallel()

	known := []Finding{{Description: "Old issue", OriginCycle: 1, Severity: "HIGH"}}
	current := []Finding{{Description: "Old issue", Severity: "MODERATE"}} // severity may change

	_, persisting, _ := classifyFindings(known, current)
	require.Len(t, persisting, 1)
	assert.Equal(t, 1, persisting[0].OriginCycle, "should preserve original cycle")
}

// --- sortFindingsFIFO tests ---

func TestSortFindingsFIFO(t *testing.T) {
	t.Parallel()

	findings := []Finding{
		{Description: "new-moderate", OriginCycle: 2, Severity: "MODERATE"},
		{Description: "old-high", OriginCycle: 1, Severity: "HIGH"},
		{Description: "new-critical", OriginCycle: 2, Severity: "CRITICAL"},
		{Description: "old-critical", OriginCycle: 1, Severity: "CRITICAL"},
		{Description: "old-moderate", OriginCycle: 1, Severity: "MODERATE"},
	}
	sortFindingsFIFO(findings)

	// Cycle 1 first (oldest), sorted by severity desc within cycle
	assert.Equal(t, "old-critical", findings[0].Description)
	assert.Equal(t, "old-high", findings[1].Description)
	assert.Equal(t, "old-moderate", findings[2].Description)
	// Cycle 2 next
	assert.Equal(t, "new-critical", findings[3].Description)
	assert.Equal(t, "new-moderate", findings[4].Description)
}

// --- mergeFindings tests ---

func TestMergeFindings(t *testing.T) {
	t.Parallel()

	persisting := []Finding{{Description: "old", OriginCycle: 1, Severity: "HIGH"}}
	newFindings := []Finding{{Description: "new", OriginCycle: 2, Severity: "CRITICAL"}}

	merged := mergeFindings(persisting, newFindings)
	require.Len(t, merged, 2)
	assert.Equal(t, "old", merged[0].Description, "oldest first")
	assert.Equal(t, "new", merged[1].Description)
}

// --- groupByCycle tests ---

func TestGroupByCycle(t *testing.T) {
	t.Parallel()

	findings := []Finding{
		{Description: "c2a", OriginCycle: 2},
		{Description: "c1a", OriginCycle: 1},
		{Description: "c1b", OriginCycle: 1},
		{Description: "c3a", OriginCycle: 3},
	}
	groups := groupByCycle(findings)

	require.Len(t, groups, 3)
	assert.Equal(t, 1, groups[0].cycle)
	assert.Len(t, groups[0].findings, 2)
	assert.Equal(t, 2, groups[1].cycle)
	assert.Len(t, groups[1].findings, 1)
	assert.Equal(t, 3, groups[2].cycle)
	assert.Len(t, groups[2].findings, 1)
}

// --- filterUnresolved tests ---

func TestFilterUnresolved(t *testing.T) {
	t.Parallel()

	findings := []Finding{
		{Description: "a", Severity: "HIGH", Resolved: false},
		{Description: "b", Severity: "LOW", Resolved: false},
		{Description: "c", Severity: "CRITICAL", Resolved: true},
		{Description: "d", Severity: "MODERATE", Resolved: false},
		{Description: "e", Severity: "", Resolved: false},
	}
	result := filterUnresolved(findings)
	assert.Len(t, result, 2)
	assert.Equal(t, "a", result[0].Description)
	assert.Equal(t, "d", result[1].Description)
}

// --- hasProgress tests ---

func TestHasProgress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		previous map[string]struct{}
		current  map[string]struct{}
		expected bool
	}{
		{
			name:     "current empty — all resolved",
			previous: map[string]struct{}{"a": {}},
			current:  map[string]struct{}{},
			expected: true,
		},
		{
			name:     "previous empty — first iteration",
			previous: map[string]struct{}{},
			current:  map[string]struct{}{"a": {}},
			expected: true,
		},
		{
			name:     "both empty",
			previous: map[string]struct{}{},
			current:  map[string]struct{}{},
			expected: true,
		},
		{
			name:     "identical findings — no progress",
			previous: map[string]struct{}{"a": {}, "b": {}},
			current:  map[string]struct{}{"a": {}, "b": {}},
			expected: false,
		},
		{
			name:     "fewer findings — progress",
			previous: map[string]struct{}{"a": {}, "b": {}, "c": {}},
			current:  map[string]struct{}{"a": {}, "b": {}},
			expected: true,
		},
		{
			name:     "different findings — progress",
			previous: map[string]struct{}{"a": {}, "b": {}},
			current:  map[string]struct{}{"c": {}, "d": {}},
			expected: true,
		},
		{
			name:     "superset of previous — no progress",
			previous: map[string]struct{}{"a": {}, "b": {}},
			current:  map[string]struct{}{"a": {}, "b": {}, "c": {}},
			expected: false,
		},
		{
			name:     "partial overlap with new — progress",
			previous: map[string]struct{}{"a": {}, "b": {}},
			current:  map[string]struct{}{"a": {}, "c": {}},
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, hasProgress(tt.previous, tt.current))
		})
	}
}

// --- effectiveOuterCycles tests ---

func TestEffectiveOuterCycles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		epic         *epic.Epic
		wantMax      int
		wantProgress bool
	}{
		{
			name:         "medium effort default",
			epic:         &epic.Epic{EffortLevel: epic.EffortMedium, MaxAuditIterations: 3},
			wantMax:      3,
			wantProgress: false,
		},
		{
			name:         "high effort not explicitly set",
			epic:         &epic.Epic{EffortLevel: epic.EffortHigh, MaxAuditIterations: 3},
			wantMax:      config.MaxOuterCyclesHighCap,
			wantProgress: true,
		},
		{
			name:         "max effort not explicitly set",
			epic:         &epic.Epic{EffortLevel: epic.EffortMax, MaxAuditIterations: 3},
			wantMax:      config.MaxOuterCyclesMaxCap,
			wantProgress: true,
		},
		{
			name:         "high effort explicitly set",
			epic:         &epic.Epic{EffortLevel: epic.EffortHigh, MaxAuditIterations: 5, MaxAuditIterationsSet: true},
			wantMax:      5,
			wantProgress: false,
		},
		{
			name:         "low effort",
			epic:         &epic.Epic{EffortLevel: epic.EffortLow, MaxAuditIterations: 3},
			wantMax:      3,
			wantProgress: false,
		},
		{
			name:         "unset effort zero iterations",
			epic:         &epic.Epic{},
			wantMax:      config.DefaultMaxOuterAuditCycles,
			wantProgress: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			maxCycles, progressBased := effectiveOuterCycles(tt.epic)
			assert.Equal(t, tt.wantMax, maxCycles)
			assert.Equal(t, tt.wantProgress, progressBased)
		})
	}
}

// --- effectiveInnerIter tests ---

func TestEffectiveInnerIter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		epic *epic.Epic
		want int
	}{
		{name: "default", epic: &epic.Epic{}, want: config.DefaultMaxInnerFixIter},
		{name: "medium", epic: &epic.Epic{EffortLevel: epic.EffortMedium}, want: config.DefaultMaxInnerFixIter},
		{name: "high", epic: &epic.Epic{EffortLevel: epic.EffortHigh}, want: config.MaxInnerFixIterHigh},
		{name: "max", epic: &epic.Epic{EffortLevel: epic.EffortMax}, want: config.MaxInnerFixIterMax},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, effectiveInnerIter(tt.epic))
		})
	}
}

// --- Prompt tests ---

func TestAuditPromptContainsDiff(t *testing.T) {
	t.Parallel()

	opts := makeOpts(t, &stubEngine{name: "codex"})
	prompt := buildAuditPrompt(opts, nil)
	assert.Contains(t, prompt, "+new line")
	assert.Contains(t, prompt, "-old line")
}

func TestAuditPromptWritingMode(t *testing.T) {
	t.Parallel()

	opts := makeOpts(t, &stubEngine{name: "codex"})
	opts.Mode = "writing"
	prompt := buildAuditPrompt(opts, nil)
	assert.Contains(t, prompt, "content auditor")
	assert.Contains(t, prompt, "Coherence")
	assert.Contains(t, prompt, "Tone & Voice")
	assert.Contains(t, prompt, "Depth")
	assert.NotContains(t, prompt, "code auditor")
	assert.NotContains(t, prompt, "Security")
}

func TestAuditPromptCondensesExecutive(t *testing.T) {
	t.Parallel()

	opts := makeOpts(t, &stubEngine{name: "codex"})
	long := make([]byte, 3000)
	for i := range long {
		long[i] = 'x'
	}
	writeFile(t, filepath.Join(opts.ProjectDir, config.ExecutiveFile), string(long))

	prompt := buildAuditPrompt(opts, nil)
	assert.Contains(t, prompt, "...(truncated)")
	assert.Contains(t, prompt, "## Project Context")
}

func TestAuditPromptTruncatesSprintProgress(t *testing.T) {
	t.Parallel()

	opts := makeOpts(t, &stubEngine{name: "codex"})
	large := make([]byte, 60000)
	for i := range large {
		large[i] = 'y'
	}
	writeFile(t, filepath.Join(opts.ProjectDir, config.SprintProgressFile), string(large))

	prompt := buildAuditPrompt(opts, nil)
	assert.Contains(t, prompt, "...(sprint progress truncated at 50KB)")
	assert.Contains(t, prompt, "## What Was Done")
}

func TestAuditPromptWithPreviousFindings(t *testing.T) {
	t.Parallel()

	opts := makeOpts(t, &stubEngine{name: "codex"})
	prev := []Finding{
		{Location: "src/main.go:10", Description: "Null pointer", Severity: "CRITICAL", OriginCycle: 1},
		{Description: "Missing validation", Severity: "HIGH", OriginCycle: 1},
	}
	prompt := buildAuditPrompt(opts, prev)

	assert.Contains(t, prompt, "## Previously Identified Issues")
	assert.Contains(t, prompt, "[src/main.go:10] Null pointer (CRITICAL)")
	assert.Contains(t, prompt, "Missing validation (HIGH)")
	assert.Contains(t, prompt, "## Verified Previous Issues")
	assert.Contains(t, prompt, "RESOLVED | STILL PRESENT")
}

func TestAuditPromptNoPreviousFindings(t *testing.T) {
	t.Parallel()

	opts := makeOpts(t, &stubEngine{name: "codex"})
	prompt := buildAuditPrompt(opts, nil)

	assert.NotContains(t, prompt, "## Previously Identified Issues")
	assert.NotContains(t, prompt, "## Verified Previous Issues")
}

func TestAuditPromptSkipsResolvedPreviousFindings(t *testing.T) {
	t.Parallel()

	opts := makeOpts(t, &stubEngine{name: "codex"})
	prev := []Finding{
		{Description: "Resolved issue", Severity: "HIGH", Resolved: true},
		{Description: "Active issue", Severity: "CRITICAL", Resolved: false},
	}
	prompt := buildAuditPrompt(opts, prev)

	assert.Contains(t, prompt, "## Previously Identified Issues")
	assert.NotContains(t, prompt, "Resolved issue")
	assert.Contains(t, prompt, "Active issue")
}

func TestAuditFixPromptFIFO(t *testing.T) {
	t.Parallel()

	opts := makeOpts(t, &stubEngine{name: "codex"})
	findings := []Finding{
		{Description: "Old issue", Severity: "HIGH", OriginCycle: 1},
		{Description: "New issue", Severity: "CRITICAL", OriginCycle: 2},
	}
	prompt := buildAuditFixPrompt(opts, findings)

	assert.Contains(t, prompt, "## Issues to Fix")
	assert.Contains(t, prompt, "oldest issues first")
	assert.Contains(t, prompt, "priority order")
	assert.Contains(t, prompt, "Old issue")
	assert.Contains(t, prompt, "New issue")
	// Multiple cycles → shows cycle headers
	assert.Contains(t, prompt, "### From Audit Cycle 1")
	assert.Contains(t, prompt, "### From Audit Cycle 2")
}

func TestAuditFixPromptSingleCycleNoCycleHeader(t *testing.T) {
	t.Parallel()

	opts := makeOpts(t, &stubEngine{name: "codex"})
	findings := []Finding{
		{Description: "Issue A", Severity: "HIGH", OriginCycle: 1},
		{Description: "Issue B", Severity: "MODERATE", OriginCycle: 1},
	}
	prompt := buildAuditFixPrompt(opts, findings)

	assert.NotContains(t, prompt, "### From Audit Cycle")
}

func TestAuditFixPromptWritingMode(t *testing.T) {
	t.Parallel()

	opts := makeOpts(t, &stubEngine{name: "codex"})
	opts.Mode = "writing"
	prompt := buildAuditFixPrompt(opts, []Finding{{Description: "weak transition", Severity: "MODERATE", OriginCycle: 1}})
	assert.Contains(t, prompt, "content audit found issues")
	assert.Contains(t, prompt, "minimal editorial changes")
}

func TestBuildVerifyPrompt(t *testing.T) {
	t.Parallel()

	opts := makeOpts(t, &stubEngine{name: "codex"})
	findings := []Finding{
		{Location: "src/main.go:10", Description: "Null pointer", Severity: "CRITICAL"},
		{Description: "Missing validation", Severity: "HIGH"},
	}
	prompt := buildVerifyPrompt(opts, findings)

	assert.Contains(t, prompt, "# VERIFY FIXES")
	assert.Contains(t, prompt, "Do NOT look for new issues")
	assert.Contains(t, prompt, "## Issues to Verify")
	assert.Contains(t, prompt, "1. [src/main.go:10] Null pointer (CRITICAL)")
	assert.Contains(t, prompt, "2. Missing validation (HIGH)")
	assert.Contains(t, prompt, "RESOLVED | STILL PRESENT")
}

// --- Severity parsing tests ---

func TestParseAuditSeverity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		content  string
		expected string
	}{
		{"## Findings\n- **Severity:** CRITICAL\n", "CRITICAL"},
		{"Severity: HIGH\nSeverity: MODERATE\n", "HIGH"},
		{"- **Severity:** MODERATE\nedge case\n", "MODERATE"},
		{"- **Severity:** LOW\nstyle issue\n", "LOW"},
		{"## Verdict\nPASS\n", ""},
		{"No issues found.", ""},
		{"CRITICAL bug found here", ""},
		{"This is HIGH priority work", ""},
		{"- **Severity:** LOW\n- **Severity:** HIGH\n- **Severity:** MODERATE\n", "HIGH"},
		{"Severity: CRITICAL\nSeverity: LOW\n", "CRITICAL"},
		{"**Severity:** LOW — HIGHLY unusual but cosmetic\n", "LOW"},
		{"**Severity:** LOW — HIGHLIGHTED concern\n", "LOW"},
		{"**Severity:** LOW — CRITICALLY important style\n", "LOW"},
		{"**Severity:** MODERATE — ALLOW this pattern\n", "MODERATE"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, parseAuditSeverity(tt.content), "content: %q", tt.content)
	}
}

func TestIsAuditPass(t *testing.T) {
	t.Parallel()

	assert.True(t, isAuditPass(""))
	assert.True(t, isAuditPass("LOW"))
	assert.False(t, isAuditPass("MODERATE"))
	assert.False(t, isAuditPass("HIGH"))
	assert.False(t, isAuditPass("CRITICAL"))
}

func TestIsBlockingSeverity(t *testing.T) {
	t.Parallel()

	assert.True(t, isBlockingSeverity("CRITICAL"))
	assert.True(t, isBlockingSeverity("HIGH"))
	assert.False(t, isBlockingSeverity("MODERATE"))
	assert.False(t, isBlockingSeverity("LOW"))
	assert.False(t, isBlockingSeverity(""))
}

// --- countAuditSeverities tests ---

func TestCountAuditSeverities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expected map[string]int
	}{
		{
			name:     "single CRITICAL",
			content:  "## Findings\n- **Severity:** CRITICAL\n",
			expected: map[string]int{"CRITICAL": 1},
		},
		{
			name: "mixed severities",
			content: "- **Severity:** CRITICAL\n- **Severity:** HIGH\n- **Severity:** HIGH\n" +
				"- **Severity:** MODERATE\n- **Severity:** MODERATE\n- **Severity:** MODERATE\n" +
				"- **Severity:** LOW\n",
			expected: map[string]int{"CRITICAL": 1, "HIGH": 2, "MODERATE": 3, "LOW": 1},
		},
		{
			name:     "no severity lines",
			content:  "## Summary\nAll good.\n## Verdict\nPASS\n",
			expected: map[string]int{},
		},
		{
			name:     "only LOW",
			content:  "- **Severity:** LOW\n- **Severity:** LOW\n",
			expected: map[string]int{"LOW": 2},
		},
		{
			name:     "non-label lines ignored",
			content:  "CRITICAL bug found here\nThis is HIGH priority\n",
			expected: map[string]int{},
		},
		{
			name:     "word boundary — HIGHLY should not match HIGH",
			content:  "**Severity:** LOW — HIGHLY unusual\n",
			expected: map[string]int{"LOW": 1},
		},
		{
			name:     "empty content",
			content:  "",
			expected: map[string]int{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := countAuditSeverities(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- formatSeverityCounts tests ---

func TestFormatSeverityCounts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		counts   map[string]int
		expected string
	}{
		{
			name:     "all levels",
			counts:   map[string]int{"CRITICAL": 1, "HIGH": 2, "MODERATE": 4, "LOW": 1},
			expected: "1 CRITICAL, 2 HIGH, 4 MODERATE, 1 LOW",
		},
		{
			name:     "only high and moderate",
			counts:   map[string]int{"HIGH": 3, "MODERATE": 1},
			expected: "3 HIGH, 1 MODERATE",
		},
		{
			name:     "single level",
			counts:   map[string]int{"LOW": 5},
			expected: "5 LOW",
		},
		{
			name:     "empty map",
			counts:   map[string]int{},
			expected: "none",
		},
		{
			name:     "nil map",
			counts:   nil,
			expected: "none",
		},
		{
			name:     "zero counts omitted",
			counts:   map[string]int{"CRITICAL": 0, "HIGH": 1},
			expected: "1 HIGH",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, formatSeverityCounts(tt.counts))
		})
	}
}

func TestFormatCounts(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "1 CRITICAL, 2 HIGH", FormatCounts(map[string]int{"CRITICAL": 1, "HIGH": 2}))
	assert.Equal(t, "none", FormatCounts(nil))
}

// --- Cleanup tests ---

func TestCleanup(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, config.SprintAuditFile), "findings\n")
	writeFile(t, filepath.Join(projectDir, config.AuditPromptFile), "prompt\n")

	require.NoError(t, Cleanup(projectDir))

	_, err := os.Stat(filepath.Join(projectDir, config.SprintAuditFile))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(projectDir, config.AuditPromptFile))
	assert.True(t, os.IsNotExist(err))
}

func TestCleanupMissingFiles(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, Cleanup(projectDir))
}

// --- RunAuditLoop integration tests ---

func TestRunAuditLoopPassesImmediately(t *testing.T) {
	t.Parallel()

	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			writeFile(t, filepath.Join(projectDir, config.SprintAuditFile), cleanAudit)
		},
	}
	opts := makeOpts(t, eng)

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Equal(t, 1, result.Iterations)
	// Only audit agent called, not fix or verify
	assert.Len(t, eng.prompts, 1)
	assert.Equal(t, config.AuditInvocationPrompt, eng.prompts[0])
}

func TestRunAuditLoopExhaustsCritical(t *testing.T) {
	t.Parallel()

	// Stub always writes CRITICAL findings. The verify parser won't find
	// Issue/Status format, so nothing gets resolved. Inner loop stales
	// after 2 fix iterations per cycle.
	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			writeFile(t, filepath.Join(projectDir, config.SprintAuditFile), criticalFindings)
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.MaxAuditIterations = 2

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.True(t, result.Blocking)
	assert.Equal(t, 2, result.Iterations) // 2 outer cycles
	assert.Equal(t, "CRITICAL", result.MaxSeverity)

	// Call sequence per cycle: audit + (fix + verify)*2 = 5
	// Two cycles: 5*2 + 1 final = 11
	assert.Len(t, eng.prompts, 11)
}

func TestRunAuditLoopExhaustsModerateAdvisory(t *testing.T) {
	t.Parallel()

	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			writeFile(t, filepath.Join(projectDir, config.SprintAuditFile), moderateFindings)
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.MaxAuditIterations = 2

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.False(t, result.Blocking) // MODERATE is advisory
	assert.Equal(t, 2, result.Iterations)
	assert.Equal(t, "MODERATE", result.MaxSeverity)
}

func TestRunAuditLoopExhaustsHighBlocking(t *testing.T) {
	t.Parallel()

	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			writeFile(t, filepath.Join(projectDir, config.SprintAuditFile), highFindings)
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.MaxAuditIterations = 2

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.True(t, result.Blocking) // HIGH is blocking
	assert.Equal(t, 2, result.Iterations)
	assert.Equal(t, "HIGH", result.MaxSeverity)
}

func TestRunAuditLoopNoFindingsFile(t *testing.T) {
	t.Parallel()

	eng := &stubEngine{name: "codex"}
	opts := makeOpts(t, eng)

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Equal(t, 1, result.Iterations)
}

func TestRunAuditLoopContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	eng := &stubEngine{name: "codex"}
	opts := makeOpts(t, eng)

	_, err := RunAuditLoop(ctx, opts)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRunAuditLoopNilEpic(t *testing.T) {
	t.Parallel()

	_, err := RunAuditLoop(context.Background(), AuditOpts{
		Engine: &stubEngine{name: "codex"},
		Sprint: &epic.Sprint{Number: 1, Name: "One"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "epic and sprint are required")
}

func TestRunAuditLoopNilEngine(t *testing.T) {
	t.Parallel()

	_, err := RunAuditLoop(context.Background(), AuditOpts{
		Epic:   &epic.Epic{},
		Sprint: &epic.Sprint{Number: 1, Name: "One"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "engine is required")
}

func TestRunAuditLoopPopulatesSeverityCounts(t *testing.T) {
	t.Parallel()

	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			writeFile(t, filepath.Join(projectDir, config.SprintAuditFile),
				"## Findings\n- **Description:** A\n- **Severity:** HIGH\n- **Description:** B\n- **Severity:** MODERATE\n- **Description:** C\n- **Severity:** MODERATE\n\n## Verdict\nFAIL\n")
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.MaxAuditIterations = 1

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.NotNil(t, result.SeverityCounts)
	assert.Equal(t, 1, result.SeverityCounts["HIGH"])
	assert.Equal(t, 2, result.SeverityCounts["MODERATE"])
}

// --- Inner loop resolution tests ---

func TestRunAuditLoopInnerLoopResolvesAll(t *testing.T) {
	t.Parallel()

	// Cycle 1: audit finds 2 issues.
	// Fix 1: verify reports both resolved.
	// Cycle 2 (re-audit): clean audit → pass.
	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			path := filepath.Join(projectDir, config.SprintAuditFile)
			switch callIndex {
			case 0: // cycle 1 audit
				writeFile(t, path,
					"## Findings\n- **Description:** Issue A\n- **Severity:** HIGH\n- **Description:** Issue B\n- **Severity:** MODERATE\n\n## Verdict\nFAIL\n")
			case 1: // fix 1 (no write)
			case 2: // verify 1 → all resolved
				writeFile(t, path,
					"- **Issue:** 1\n- **Status:** RESOLVED\n- **Issue:** 2\n- **Status:** RESOLVED\n")
			case 3: // cycle 2 audit → clean
				writeFile(t, path, cleanAudit)
			}
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.MaxAuditIterations = 3

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Equal(t, 2, result.Iterations)
}

func TestRunAuditLoopInnerLoopPartialResolution(t *testing.T) {
	t.Parallel()

	// Use a prompt-based approach to distinguish call types.
	// Audit/verify calls use AuditInvocationPrompt, fix calls use AuditFixInvocationPrompt.
	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			path := filepath.Join(projectDir, config.SprintAuditFile)
			// Always write CRITICAL findings. The verify parser won't find Issue/Status
			// format, so nothing resolves → inner loop stales after 2 fix attempts.
			writeFile(t, path,
				"## Findings\n- **Description:** Unfixable issue\n- **Severity:** CRITICAL\n\n## Verdict\nFAIL\n")
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.MaxAuditIterations = 1

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.True(t, result.Blocking)
	assert.Equal(t, "CRITICAL", result.MaxSeverity)
	assert.Equal(t, 1, result.Iterations)
}

func TestRunAuditLoopNewIssuesInReAudit(t *testing.T) {
	t.Parallel()

	// Cycle 1: finds issue A. Fix resolves it.
	// Cycle 2 (re-audit): issue A resolved, but new issue B found.
	// Fix resolves issue B.
	// Cycle 3 (re-audit): all clean → pass.
	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			path := filepath.Join(projectDir, config.SprintAuditFile)
			switch callIndex {
			case 0: // cycle 1 audit
				writeFile(t, path,
					"## Findings\n- **Description:** Issue A\n- **Severity:** HIGH\n\n## Verdict\nFAIL\n")
			case 2: // verify: issue A resolved
				writeFile(t, path,
					"- **Issue:** 1\n- **Status:** RESOLVED\n")
			case 3: // cycle 2 audit: A resolved, B new
				writeFile(t, path,
					"## Findings\n- **Description:** Issue B\n- **Severity:** MODERATE\n\n## Verdict\nFAIL\n")
			case 5: // verify: issue B resolved
				writeFile(t, path,
					"- **Issue:** 1\n- **Status:** RESOLVED\n")
			case 6: // cycle 3 audit: all clean
				writeFile(t, path, cleanAudit)
			}
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.MaxAuditIterations = 5

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Equal(t, 3, result.Iterations) // 3 outer cycles
}

// --- Progress-based loop tests ---

func TestRunAuditLoopProgressStopsOnStale(t *testing.T) {
	t.Parallel()

	// High effort: same CRITICAL finding every time. Outer stale detection
	// should stop after 3 consecutive stale cycles.
	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			writeFile(t, filepath.Join(projectDir, config.SprintAuditFile), criticalFindings)
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.EffortLevel = epic.EffortHigh
	opts.Epic.MaxAuditIterationsSet = false

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.True(t, result.Blocking)
	assert.Equal(t, "CRITICAL", result.MaxSeverity)

	// Cycle 1: baseline (no stale check on cycle 1).
	// Cycle 2: outer stale=1. Cycle 3: outer stale=2. Cycle 4: outer stale=3 → break before inner.
	// Cycles 1-3 each: audit + (fix+verify)*2 (inner stale) = 5
	// Cycle 4: audit only (breaks immediately after stale detection) = 1
	// Plus 1 final audit = 3*5 + 1 + 1 = 17
	assert.Len(t, eng.prompts, 17)
}

func TestRunAuditLoopProgressContinues(t *testing.T) {
	t.Parallel()

	// Max effort with explicit cap: different findings each cycle → progress always made → runs full cap.
	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			desc := fmt.Sprintf("Unique issue %d", callIndex)
			writeFile(t, filepath.Join(projectDir, config.SprintAuditFile),
				fmt.Sprintf("## Findings\n- **Description:** %s\n- **Severity:** HIGH\n\n## Verdict\nFAIL\n", desc))
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.EffortLevel = epic.EffortMax
	opts.Epic.MaxAuditIterationsSet = true
	opts.Epic.MaxAuditIterations = 20

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.True(t, result.Blocking)

	// 20 explicit cycles, MaxInnerFixIterMax=10
	// Each cycle: audit + (fix+verify)*2 (inner stale since same content every call) = 5
	// Except: findings are DIFFERENT each time because callIndex varies.
	// The inner verify also writes unique findings, so verify parser finds no Issue/Status format.
	// Inner always stales after 2 fix attempts.
	// Outer: each cycle gets a different description (based on callIndex of the audit call),
	// so progress is detected (different finding keys).
	// 20 cycles * 5 calls + 1 final = 101
	assert.Len(t, eng.prompts, 101)
}

func TestRunAuditLoopExplicitCapAtHighEffort(t *testing.T) {
	t.Parallel()

	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			writeFile(t, filepath.Join(projectDir, config.SprintAuditFile), highFindings)
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.EffortLevel = epic.EffortHigh
	opts.Epic.MaxAuditIterations = 2
	opts.Epic.MaxAuditIterationsSet = true

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.True(t, result.Blocking)
	assert.Equal(t, 2, result.Iterations)
}

func TestRunAuditLoopMediumEffortBounded(t *testing.T) {
	t.Parallel()

	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			writeFile(t, filepath.Join(projectDir, config.SprintAuditFile), moderateFindings)
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.EffortLevel = epic.EffortMedium
	opts.Epic.MaxAuditIterations = 3

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.False(t, result.Blocking)
	assert.Equal(t, 3, result.Iterations)
}

// --- applyResolutionsByKey tests ---

func TestApplyResolutionsByKey(t *testing.T) {
	t.Parallel()

	all := []Finding{
		{Description: "Issue A", Severity: "HIGH"},
		{Description: "Issue B", Severity: "MODERATE"},
		{Description: "Issue C", Severity: "CRITICAL"},
	}
	checked := []Finding{
		{Description: "Issue A", Severity: "HIGH"},
		{Description: "Issue C", Severity: "CRITICAL"},
	}
	resolved := []bool{true, false}

	applyResolutionsByKey(all, checked, resolved)

	assert.True(t, all[0].Resolved, "Issue A should be resolved")
	assert.False(t, all[1].Resolved, "Issue B was not checked")
	assert.False(t, all[2].Resolved, "Issue C was not resolved")
}

// --- findingKeySet tests ---

func TestFindingKeySet(t *testing.T) {
	t.Parallel()

	findings := []Finding{
		{Description: "Active HIGH", Severity: "HIGH"},
		{Description: "Active MODERATE", Severity: "MODERATE"},
		{Description: "Low Issue", Severity: "LOW"},
		{Description: "Resolved", Severity: "HIGH", Resolved: true},
		{Description: "No Severity", Severity: ""},
	}

	keys := findingKeySet(findings)
	assert.Len(t, keys, 2)
	assert.Contains(t, keys, "active high")
	assert.Contains(t, keys, "active moderate")
}

// --- UnresolvedFindings in result test ---

func TestRunAuditLoopUnresolvedFindingsInResult(t *testing.T) {
	t.Parallel()

	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			writeFile(t, filepath.Join(projectDir, config.SprintAuditFile), criticalFindings)
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.MaxAuditIterations = 1

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.NotEmpty(t, result.UnresolvedFindings)
	assert.Equal(t, "Null pointer dereference", result.UnresolvedFindings[0].Description)
}

// --- filterFixable tests ---

func TestFilterFixable(t *testing.T) {
	t.Parallel()

	findings := []Finding{
		{Description: "Critical bug", Severity: "CRITICAL"},
		{Description: "High bug", Severity: "HIGH"},
		{Description: "Moderate issue", Severity: "MODERATE"},
		{Description: "Low style", Severity: "LOW"},
		{Description: "No severity", Severity: ""},
		{Description: "Resolved high", Severity: "HIGH", Resolved: true},
		{Description: "Resolved low", Severity: "LOW", Resolved: true},
	}

	// Without LOW: same as filterUnresolved
	withoutLow := filterFixable(findings, false)
	assert.Len(t, withoutLow, 3)
	assert.Equal(t, "Critical bug", withoutLow[0].Description)
	assert.Equal(t, "High bug", withoutLow[1].Description)
	assert.Equal(t, "Moderate issue", withoutLow[2].Description)

	// With LOW: includes unresolved LOW
	withLow := filterFixable(findings, true)
	assert.Len(t, withLow, 4)
	assert.Equal(t, "Critical bug", withLow[0].Description)
	assert.Equal(t, "High bug", withLow[1].Description)
	assert.Equal(t, "Moderate issue", withLow[2].Description)
	assert.Equal(t, "Low style", withLow[3].Description)
}

func TestCountFixable(t *testing.T) {
	t.Parallel()

	findings := []Finding{
		{Description: "A", Severity: "HIGH"},
		{Description: "B", Severity: "LOW"},
		{Description: "C", Severity: "MODERATE", Resolved: true},
		{Description: "D", Severity: "LOW", Resolved: true},
	}

	// countFixable counts ALL findings matching severity filter (resolved or not)
	// because it serves as the total denominator in progress logs.
	assert.Equal(t, 2, countFixable(findings, false))  // HIGH + resolved MODERATE
	assert.Equal(t, 4, countFixable(findings, true))   // all four
}

func TestCountUnresolvedLow(t *testing.T) {
	t.Parallel()

	findings := []Finding{
		{Description: "A", Severity: "HIGH"},
		{Description: "B", Severity: "LOW"},
		{Description: "C", Severity: "LOW", Resolved: true},
		{Description: "D", Severity: "LOW"},
	}

	assert.Equal(t, 2, countUnresolvedLow(findings))
}

func TestFixIncludesLow(t *testing.T) {
	t.Parallel()

	assert.False(t, fixIncludesLow(&epic.Epic{EffortLevel: epic.EffortLow}))
	assert.False(t, fixIncludesLow(&epic.Epic{EffortLevel: epic.EffortMedium}))
	assert.False(t, fixIncludesLow(&epic.Epic{EffortLevel: ""}))
	assert.True(t, fixIncludesLow(&epic.Epic{EffortLevel: epic.EffortHigh}))
	assert.True(t, fixIncludesLow(&epic.Epic{EffortLevel: epic.EffortMax}))
}

// --- Effort-aware LOW fix integration tests ---

func TestRunAuditLoopHighEffortFixesLow(t *testing.T) {
	t.Parallel()

	// High effort: audit finds MODERATE + LOW. Fix agent should receive both.
	// Verify resolves both. Re-audit is clean → pass.
	mixedFindings := "## Findings\n- **Description:** Edge case gap\n- **Severity:** MODERATE\n- **Description:** Variable naming\n- **Severity:** LOW\n\n## Verdict\nFAIL\n"

	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			path := filepath.Join(projectDir, config.SprintAuditFile)
			switch callIndex {
			case 0: // cycle 1 audit
				writeFile(t, path, mixedFindings)
			case 1: // fix 1 (no write needed)
			case 2: // verify 1 → both resolved
				writeFile(t, path,
					"- **Issue:** 1\n- **Status:** RESOLVED\n- **Issue:** 2\n- **Status:** RESOLVED\n")
			case 3: // cycle 2 audit → clean
				writeFile(t, path, cleanAudit)
			}
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.EffortLevel = epic.EffortHigh
	opts.Epic.MaxAuditIterationsSet = true
	opts.Epic.MaxAuditIterations = 5

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.True(t, result.Passed)

	// Fix prompt should have been called and should contain both issues
	require.True(t, len(eng.prompts) >= 2, "expected at least audit + fix prompts")
	// The fix agent prompt (index 1) is AuditFixInvocationPrompt
	assert.Equal(t, config.AuditFixInvocationPrompt, eng.prompts[1])
}

func TestRunAuditLoopMediumEffortIgnoresLow(t *testing.T) {
	t.Parallel()

	// Medium effort: audit finds MODERATE + LOW. Fix agent should receive only MODERATE.
	// Verify resolves it. Re-audit finds only LOW → pass (LOW-only = pass).
	mixedFindings := "## Findings\n- **Description:** Edge case gap\n- **Severity:** MODERATE\n- **Description:** Variable naming\n- **Severity:** LOW\n\n## Verdict\nFAIL\n"
	lowOnlyFindings := "## Findings\n- **Description:** Variable naming\n- **Severity:** LOW\n\n## Verdict\nPASS\n"

	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			path := filepath.Join(projectDir, config.SprintAuditFile)
			switch callIndex {
			case 0: // cycle 1 audit
				writeFile(t, path, mixedFindings)
			case 1: // fix 1
			case 2: // verify 1 → MODERATE resolved
				writeFile(t, path,
					"- **Issue:** 1\n- **Status:** RESOLVED\n")
			case 3: // cycle 2 audit → only LOW remains
				writeFile(t, path, lowOnlyFindings)
			}
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.EffortLevel = epic.EffortMedium
	opts.Epic.MaxAuditIterations = 5

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Equal(t, 2, result.Iterations)
}

func TestRunAuditLoopLowOnlyMaxEffortRunsOneFix(t *testing.T) {
	t.Parallel()

	lowOnlyFindings := "## Findings\n- **Description:** Variable naming\n- **Severity:** LOW\n\n## Verdict\nPASS\n"

	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			path := filepath.Join(projectDir, config.SprintAuditFile)
			switch callIndex {
			case 0: // cycle 1 audit → LOW only
				writeFile(t, path, lowOnlyFindings)
			case 1: // fix pass (single LOW fix attempt)
			}
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.EffortLevel = epic.EffortMax

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Equal(t, 1, result.Iterations)
	// Expect: audit + fix = 2 calls. No re-audit after fix.
	assert.Len(t, eng.prompts, 2)
	assert.Equal(t, config.AuditInvocationPrompt, eng.prompts[0])
	assert.Equal(t, config.AuditFixInvocationPrompt, eng.prompts[1])
}

func TestRunAuditLoopLowOnlyNonMaxExitsImmediately(t *testing.T) {
	t.Parallel()

	lowOnlyFindings := "## Findings\n- **Description:** Variable naming\n- **Severity:** LOW\n\n## Verdict\nPASS\n"

	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			writeFile(t, filepath.Join(projectDir, config.SprintAuditFile), lowOnlyFindings)
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.EffortLevel = epic.EffortMedium

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Equal(t, 1, result.Iterations)
	// Only audit called, no fix agent
	assert.Len(t, eng.prompts, 1)
	assert.Equal(t, config.AuditInvocationPrompt, eng.prompts[0])
}

func TestRunAuditLoopMaxEffortHighFindingsStillLoops(t *testing.T) {
	t.Parallel()

	// Max effort with MODERATE findings should loop normally, not exit early.
	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			writeFile(t, filepath.Join(projectDir, config.SprintAuditFile), moderateFindings)
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.EffortLevel = epic.EffortMax
	opts.Epic.MaxAuditIterationsSet = true
	opts.Epic.MaxAuditIterations = 2

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.False(t, result.Passed, "MODERATE findings should not pass")
}
