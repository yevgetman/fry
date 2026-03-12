# Build Plan: Effort-Level Triage System for Fry

## Problem Statement

Simple tasks currently consume excessive resources — a basic HTML/CSS page might get decomposed into 7 sprints when 1-2 would suffice. The system has no concept of task complexity or effort scaling. Sprint count and density are determined solely by the AI's decomposition with no guidance on proportionality.

## Solution Overview

Introduce an **effort level** system that controls:
1. How many sprints the AI generates during `prepare` (Step 2)
2. How dense/detailed each sprint's prompt and iteration budget is
3. How much rumination and verification rigor is applied per sprint

The system supports both **auto-detection** (AI analyzes the plan and assigns an effort level) and **manual override** (user specifies `--effort low|medium|high|max`).

## Effort Level Definitions

| Level    | Sprint Count | Max Iterations/Sprint | Prompt Detail       | Review Rigor | Use Case |
|----------|-------------|----------------------|---------------------|-------------|----------|
| `low`    | 1-2         | 10-15                | Concise, essential  | No review   | Simple pages, config changes, small features |
| `medium` | 2-4         | 15-25                | Moderate detail     | Optional    | Multi-component features, moderate integrations |
| `high`   | 4-10        | 15-35 (current)      | Full 7-part (current) | Full review | Complex systems, current default behavior |
| `max`    | 4-10 (same as high) | 30-50 (extended) | Extended 7-part + analysis sections | Enhanced review + mandatory deviation analysis | Mission-critical, high-stakes builds |

## Architecture

### New Type: `EffortLevel`

Location: `internal/epic/types.go`

```go
type EffortLevel string

const (
    EffortLow    EffortLevel = "low"
    EffortMedium EffortLevel = "medium"
    EffortHigh   EffortLevel = "high"
    EffortMax    EffortLevel = "max"
    EffortAuto   EffortLevel = ""  // empty = auto-detect
)
```

### Data Flow

```
User Input (--effort flag or auto)
    ↓
PrepareOpts.EffortLevel
    ↓
Step 2 prompt (epic generation) ← effort-aware sizing guidance injected
    ↓
@effort directive written to epic.md ← parsed back by epic parser
    ↓
Epic.EffortLevel field populated
    ↓
Sprint execution uses effort to:
  - Scale max_iterations (if not explicitly overridden per-sprint)
  - Control review behavior
  - Adjust prompt assembly context
    ↓
Build summary displays effort level
```

---

## Detailed Changes

### 1. `internal/epic/types.go` — Add EffortLevel to Epic

**What**: Add `EffortLevel` field to the `Epic` struct and define the `EffortLevel` type with constants.

```go
type EffortLevel string

const (
    EffortLow    EffortLevel = "low"
    EffortMedium EffortLevel = "medium"
    EffortHigh   EffortLevel = "high"
    EffortMax    EffortLevel = "max"
)

func ParseEffortLevel(s string) (EffortLevel, error) {
    switch strings.ToLower(strings.TrimSpace(s)) {
    case "low":
        return EffortLow, nil
    case "medium":
        return EffortMedium, nil
    case "high":
        return EffortHigh, nil
    case "max":
        return EffortMax, nil
    case "":
        return "", nil  // auto-detect
    default:
        return "", fmt.Errorf("invalid effort level %q: must be low, medium, high, or max", s)
    }
}

func (e EffortLevel) String() string {
    if e == "" {
        return "auto"
    }
    return string(e)
}

// DefaultMaxIterations returns the default max_iterations for sprints
// at this effort level (used when @max_iterations is not set per-sprint).
func (e EffortLevel) DefaultMaxIterations() int {
    switch e {
    case EffortLow:
        return 12
    case EffortMedium:
        return 20
    case EffortHigh:
        return 25
    case EffortMax:
        return 40
    default:
        return 25 // default = high
    }
}

// MaxSprintCount returns the maximum number of sprints for this effort level.
func (e EffortLevel) MaxSprintCount() int {
    switch e {
    case EffortLow:
        return 2
    case EffortMedium:
        return 4
    case EffortHigh:
        return 10
    case EffortMax:
        return 10
    default:
        return 10
    }
}

type Epic struct {
    // ... existing fields ...
    EffortLevel          EffortLevel  // NEW
    // ... rest of existing fields ...
}
```

**Edge cases**:
- Empty string = auto-detect (backward compatible)
- Case-insensitive parsing
- Unknown values produce a clear error message

### 2. `internal/epic/parser.go` — Parse @effort directive

**What**: Add `@effort` directive parsing in the `stateGlobal` case.

```go
case "@effort":
    ep.EffortLevel, err = ParseEffortLevel(value)
    if err != nil {
        return nil, fmt.Errorf("parse epic line %d: %w", lineNo, err)
    }
```

**Where**: In the `stateGlobal` switch, alongside existing global directives like `@epic`, `@engine`, etc.

**Edge cases**:
- `@effort` placed in sprint meta section should be warned (it's a global directive)
- `@effort` with no value should default to auto (empty string, no error)

### 3. `internal/epic/validator.go` — Validate effort constraints

**What**: Add validation that sprint count respects effort level constraints.

```go
func ValidateEpic(e *Epic) error {
    // ... existing validation ...

    // Validate sprint count against effort level (if set)
    if e.EffortLevel != "" {
        maxSprints := e.EffortLevel.MaxSprintCount()
        if len(e.Sprints) > maxSprints {
            return fmt.Errorf("effort level %q allows at most %d sprints, but epic has %d",
                e.EffortLevel, maxSprints, len(e.Sprints))
        }
    }

    return nil
}
```

**Edge cases**:
- Effort level "" (auto/unset) skips the check entirely — backward compatible
- Manually authored epics with explicit sprint counts that exceed the limit get a clear error

### 4. `internal/config/config.go` — Add effort-related constants

**What**: Add default constants for effort levels.

```go
const (
    // ... existing constants ...
    DefaultEffortLevel = ""  // auto-detect
)
```

No new iteration constants needed — those live in the `EffortLevel` methods on the type itself.

### 5. `internal/cli/run.go` — Add `--effort` flag to run command

**What**: Add `--effort` flag, pass it through to prepare and validate against parsed epic.

```go
var (
    // ... existing vars ...
    runEffort string
)
```

In `init()`:
```go
runCmd.Flags().StringVar(&runEffort, "effort", "", "Effort level: low, medium, high, max (default: auto)")
```

In `RunE`:
```go
// After parsing the effort flag, validate it
effortLevel, err := epic.ParseEffortLevel(runEffort)
if err != nil {
    return err
}

// Pass to prepare if auto-preparing
if !epicExists {
    if err := prepare.RunPrepare(cmd.Context(), prepare.PrepareOpts{
        // ... existing fields ...
        EffortLevel: effortLevel,
    }); err != nil {
        return err
    }
}

// After parsing epic, apply effort override if user specified one
// and the epic doesn't have one
ep, err := epic.ParseEpic(epicPath)
if err != nil {
    return err
}
if effortLevel != "" && ep.EffortLevel == "" {
    ep.EffortLevel = effortLevel
}
```

Also update `printDryRunReport` to show effort level:
```go
fmt.Fprintf(w, "Effort: %s\n", ep.EffortLevel)
```

And `printBuildSummary` header:
```go
fmt.Fprintln(tw, "SPRINT\tNAME\tSTATUS\tDURATION")
// Add effort level above the table:
if ep.EffortLevel != "" {
    fmt.Fprintf(w, "Effort level: %s\n", ep.EffortLevel)
}
```

Also add to root command flags (since root delegates to run):
```go
rootCmd.Flags().StringVar(&runEffort, "effort", "", "Effort level: low, medium, high, max (default: auto)")
```

**Edge cases**:
- `--effort` on run command overrides epic's `@effort` directive
- `--effort` with no epic file triggers auto-prepare with effort passed through
- Invalid effort values caught early with clear error

### 6. `internal/cli/prepare.go` — Add `--effort` flag to prepare command

**What**: Add `--effort` flag to the prepare command.

```go
var (
    // ... existing vars ...
    prepareEffort string
)
```

In `init()`:
```go
prepareCmd.Flags().StringVar(&prepareEffort, "effort", "", "Effort level: low, medium, high, max (default: auto)")
```

In `RunE`:
```go
effortLevel, err := epic.ParseEffortLevel(prepareEffort)
if err != nil {
    return err
}

return prepare.RunPrepare(cmd.Context(), prepare.PrepareOpts{
    // ... existing fields ...
    EffortLevel: effortLevel,
})
```

### 7. `internal/prepare/prepare.go` — Pass effort through prepare pipeline

**What**: Add `EffortLevel` to `PrepareOpts` and pass it to Step 2 prompts.

```go
type PrepareOpts struct {
    // ... existing fields ...
    EffortLevel epic.EffortLevel
}
```

In `RunPrepare`, modify the `step2Prompt` call:
```go
prompt = step2Prompt(opts.Planning, planContent, agentsContent, epicExamplePath, generateEpicPath, opts.UserPrompt, opts.EffortLevel)
```

Update `step2Prompt` helper:
```go
func step2Prompt(planning bool, planContent, agentsContent, epicExamplePath, generateEpicPath, userPrompt string, effort epic.EffortLevel) string {
    if planning {
        return PlanningStep2Prompt(planContent, agentsContent, epicExamplePath, userPrompt, effort)
    }
    return SoftwareStep2Prompt(planContent, agentsContent, epicExamplePath, generateEpicPath, userPrompt, effort)
}
```

### 8. `internal/prepare/software.go` — Effort-aware epic generation prompts

**What**: Modify `SoftwareStep2Prompt` to inject effort-level sizing guidance.

This is the critical change — the AI that generates the epic needs clear instructions on how effort level affects sprint count and density.

```go
func SoftwareStep2Prompt(planContent, agentsContent, epicExamplePath, generateEpicPath, userPrompt string, effort epic.EffortLevel) string {
    effortGuidance := effortSizingGuidance(effort)
    // ... existing prompt assembly ...
    // Insert effortGuidance into the prompt
}

func effortSizingGuidance(effort epic.EffortLevel) string {
    switch effort {
    case epic.EffortLow:
        return `
EFFORT LEVEL: LOW
The user has indicated this is a low-effort task. You MUST:
- Generate AT MOST 2 sprints total
- Use max_iterations of 10-15 per sprint
- Write concise sprint prompts — skip the REFERENCES and STUCK HINT sections
- Combine all work into 1-2 dense but focused sprints
- Skip scaffolding as a separate sprint — include it in Sprint 1's build list
- Focus only on the core deliverables; omit exhaustive edge cases
- Add the @effort low directive to the epic header

If the plan is genuinely trivial (single file, simple config), use exactly 1 sprint.
`
    case epic.EffortMedium:
        return `
EFFORT LEVEL: MEDIUM
The user has indicated this is a medium-effort task. You MUST:
- Generate 2-4 sprints total (prefer the lower end)
- Use max_iterations of 15-25 per sprint
- Write moderately detailed sprint prompts — include all 7 parts but keep them concise
- Merge layers that would be separate at HIGH effort (e.g., combine schema + domain types)
- Include essential edge cases but don't be exhaustive
- Add the @effort medium directive to the epic header
`
    case epic.EffortHigh:
        return `
EFFORT LEVEL: HIGH
This is the standard effort level. Follow all existing epic generation rules as-is.
- Generate 4-10 sprints as appropriate
- Use max_iterations of 15-35 per sprint per the standard sizing guidelines
- Write fully detailed 7-part sprint prompts
- Include comprehensive edge cases and verification
- Add the @effort high directive to the epic header
`
    case epic.EffortMax:
        return `
EFFORT LEVEL: MAX
The user has indicated this is a maximum-effort, mission-critical task. You MUST:
- Generate the same number of sprints as HIGH effort (4-10)
- Use max_iterations of 30-50 per sprint (higher than normal)
- Write EXTENDED sprint prompts that go beyond the standard 7-part structure:
  - Add an 8th section: "ANALYSIS & EDGE CASES" — enumerate every edge case, race condition,
    error scenario, and boundary condition relevant to this sprint
  - Add a 9th section: "QUALITY GATES" — explicit quality criteria beyond verification checks
    (performance targets, security considerations, code review checklist items)
- Include exhaustive edge cases, error handling requirements, and defensive coding instructions
- Specify exact error messages, log formats, and observability requirements
- Add the @effort max directive to the epic header
- Enable @review_between_sprints and @compact_with_agent
- Set @max_heal_attempts to 5 (increased from default 3)
`
    default: // auto-detect
        return `
EFFORT LEVEL: AUTO-DETECT
No effort level was specified. Analyze the plan document and determine the appropriate effort level:

- If the plan describes a simple, well-bounded task (single page, config change, small utility,
  1-3 files to create/modify): use LOW effort (1-2 sprints, @effort low)
- If the plan describes a moderate feature (multiple components, some integration,
  4-15 files): use MEDIUM effort (2-4 sprints, @effort medium)
- If the plan describes a complex system (many components, database, APIs, extensive
  testing, 15+ files): use HIGH effort (4-10 sprints, @effort high)

Add the @effort directive matching your assessment to the epic header.
Do NOT default to HIGH — genuinely evaluate the plan's complexity.

Common over-engineering signals to watch for:
- Creating separate scaffolding sprints for projects that need no scaffolding
- Splitting 3-file changes across 3+ sprints
- Adding schema/migration sprints for projects with no database
- Creating separate "wiring" sprints for simple, flat architectures
`
    }
}
```

### 9. `internal/prepare/planning.go` — Effort-aware planning prompts

**What**: Same treatment as software.go — modify `PlanningStep2Prompt` to accept and inject effort level guidance.

```go
func PlanningStep2Prompt(planContent, agentsContent, epicExamplePath, userPrompt string, effort epic.EffortLevel) string {
    effortGuidance := effortSizingGuidancePlanning(effort)
    // ... inject into existing prompt ...
}
```

The planning-mode effort guidance is similar but focused on document deliverables rather than code files.

### 10. `templates/GENERATE_EPIC.md` — Update with effort level documentation

**What**: Add effort level section to the template that guides manual epic generation.

Add after the "### 2. Right-sizes each sprint" section:

```markdown
### 2a. Applies effort-level sizing

If an `@effort` level is specified, it constrains sprint count and density:

| Level    | Max Sprints | Max Iterations | Prompt Detail | Notes |
|----------|------------|----------------|---------------|-------|
| `low`    | 2          | 10-15          | Concise       | Combine layers, skip scaffolding sprint |
| `medium` | 4          | 15-25          | Moderate      | Merge related layers |
| `high`   | 10         | 15-35          | Full 7-part   | Current default behavior |
| `max`    | 10         | 30-50          | Extended      | Add analysis + quality gate sections |

If no effort level is specified, auto-detect based on plan complexity:
- 1-3 files → low
- 4-15 files → medium
- 15+ files → high
```

### 11. `templates/epic-example.md` — Add @effort to example

**What**: Add `@effort` directive to the global configuration section.

```markdown
@effort high
```

Add to the directive reference comment block:

```markdown
# @effort <low|medium|high|max>  Effort level — controls sprint count and density
```

### 12. `internal/sprint/runner.go` — Effort-aware iteration scaling

**What**: When `@max_iterations` is not explicitly set for a sprint, use the effort level's default. For `max` effort, also enable more aggressive no-op detection thresholds.

In `RunSprint`, the `MaxIterations` value comes from the parsed epic. The scaling happens at epic generation time (Step 2), so no runtime override is strictly needed. However, we should log the effort level:

```go
frylog.Log("=========================================")
frylog.Log("STARTING SPRINT %d: %s", cfg.Sprint.Number, cfg.Sprint.Name)
frylog.Log("Max iterations: %d", cfg.Sprint.MaxIterations)
if cfg.Epic.EffortLevel != "" {
    frylog.Log("Effort level: %s", cfg.Epic.EffortLevel)
}
frylog.Log("=========================================")
```

For `max` effort, increase the no-op threshold from 2 to 3 consecutive iterations (giving the agent more room to "think" without producing file changes):

```go
noopThreshold := 2
if cfg.Epic.EffortLevel == epic.EffortMax {
    noopThreshold = 3
}
if consecutiveNoop >= noopThreshold && ... {
```

### 13. `internal/sprint/prompt.go` — Effort context in prompt assembly

**What**: For `max` effort, add an extra prompt layer that instructs the agent to be more thorough.

Add to `PromptOpts`:
```go
type PromptOpts struct {
    // ... existing fields ...
    EffortLevel epic.EffortLevel
}
```

In `AssemblePrompt`, after Layer 1.5 (User Directive):

```go
// Layer 1.75: Effort directive (only for max)
if opts.EffortLevel == epic.EffortMax {
    b.WriteString("# ===== QUALITY DIRECTIVE =====\n")
    b.WriteString("# This build is running at MAX effort. Apply heightened rigor:\n")
    b.WriteString("# - Consider and handle ALL edge cases, not just common ones\n")
    b.WriteString("# - Add comprehensive error handling with descriptive messages\n")
    b.WriteString("# - Write defensive code — validate assumptions, check invariants\n")
    b.WriteString("# - Consider performance implications of every data structure choice\n")
    b.WriteString("# - Review your own output each iteration for correctness before proceeding\n\n")
}
```

Update `RunSprint` to pass effort level to `AssemblePrompt`:

```go
if _, err := AssemblePrompt(PromptOpts{
    // ... existing fields ...
    EffortLevel: cfg.Epic.EffortLevel,
}); err != nil {
    return nil, fmt.Errorf("run sprint: %w", err)
}
```

### 14. `internal/review/reviewer.go` — Effort-aware review behavior

**What**: For `low` effort, disable reviews entirely (even if `@review_between_sprints` is set). For `max` effort, make the reviewer more conservative (lower threshold for DEVIATE).

In `cli/run.go`, the review gate:

```go
if ep.ReviewBetweenSprints && !runNoReview && spr.Number < ep.TotalSprints {
    // For low effort, skip reviews
    if ep.EffortLevel == epic.EffortLow {
        continue
    }
    // ... existing review logic ...
}
```

For `max` effort, modify the review prompt bias in `AssembleReviewPrompt`:

```go
if opts.EffortLevel == epic.EffortMax {
    b.WriteString("## Bias: THOROUGH REVIEW\n")
    b.WriteString("At MAX effort level, apply heightened scrutiny. Recommend DEVIATE when:\n")
    b.WriteString("- Any deviation from the plan that could affect system correctness\n")
    b.WriteString("- Missing error handling or edge case coverage in completed sprint\n")
    b.WriteString("- Performance or security concerns that downstream sprints should account for\n")
} else {
    // ... existing CONTINUE bias ...
}
```

Add `EffortLevel` to `ReviewPromptOpts`:
```go
type ReviewPromptOpts struct {
    // ... existing fields ...
    EffortLevel epic.EffortLevel
}
```

### 15. Test Coverage

#### `internal/epic/types_test.go` (new file)
```go
- TestParseEffortLevel_Valid: all four levels + empty + whitespace
- TestParseEffortLevel_Invalid: "extreme", "123", "LOW " (case insensitivity)
- TestParseEffortLevel_CaseInsensitive: "Low", "LOW", "lOw" all → EffortLow
- TestEffortLevel_String: all levels including empty → "auto"
- TestEffortLevel_DefaultMaxIterations: verify each level returns expected value
- TestEffortLevel_MaxSprintCount: verify each level returns expected value
```

#### `internal/epic/parser_test.go` (extend existing)
```go
- TestParseEpic_EffortDirective: epic with @effort medium → EffortMedium
- TestParseEpic_EffortDirectiveInvalid: @effort extreme → error
- TestParseEpic_EffortDirectiveMissing: epic without @effort → "" (auto)
- TestParseEpic_EffortDirectiveCaseInsensitive: @effort LOW → EffortLow
```

#### `internal/epic/validator_test.go` (extend or new)
```go
- TestValidateEpic_EffortLow_TooManySprints: 3 sprints with effort=low → error
- TestValidateEpic_EffortLow_Valid: 2 sprints with effort=low → nil
- TestValidateEpic_EffortMedium_TooManySprints: 5 sprints with effort=medium → error
- TestValidateEpic_EffortUnset_AnySprints: 10 sprints with effort="" → nil
```

#### `internal/prepare/software_test.go` (extend or new)
```go
- TestEffortSizingGuidance_Low: verify LOW guidance includes "AT MOST 2 sprints"
- TestEffortSizingGuidance_Max: verify MAX guidance includes "30-50"
- TestEffortSizingGuidance_Auto: verify auto-detect guidance includes "analyze the plan"
- TestSoftwareStep2Prompt_IncludesEffort: verify effort guidance is embedded in prompt
```

#### `internal/sprint/prompt_test.go` (extend existing)
```go
- TestAssemblePrompt_MaxEffort: verify QUALITY DIRECTIVE section appears
- TestAssemblePrompt_LowEffort: verify QUALITY DIRECTIVE section does NOT appear
- TestAssemblePrompt_NoEffort: verify QUALITY DIRECTIVE section does NOT appear
```

#### `internal/cli/run_test.go` (extend or new)
```go
- TestRunCmd_EffortFlag_Valid: --effort medium parses correctly
- TestRunCmd_EffortFlag_Invalid: --effort extreme returns error
```

---

## Implementation Order

### Sprint 1: Core Types, Parser & Validation
**Files**: `internal/epic/types.go`, `internal/epic/parser.go`, `internal/epic/validator.go`, `internal/epic/types_test.go`, `internal/epic/parser_test.go`
**Scope**: Define `EffortLevel` type + methods, add `@effort` directive parsing, add sprint count validation, full test coverage for all new type logic.

### Sprint 2: CLI Integration & Prepare Pipeline
**Files**: `internal/cli/root.go`, `internal/cli/run.go`, `internal/cli/prepare.go`, `internal/prepare/prepare.go`, `internal/prepare/software.go`, `internal/prepare/planning.go`
**Scope**: Add `--effort` flag to run/prepare/root commands, plumb `EffortLevel` through `PrepareOpts`, inject effort-aware sizing guidance into Step 2 prompts, update dry-run output.

### Sprint 3: Runtime Behavior, Templates & Review
**Files**: `internal/sprint/runner.go`, `internal/sprint/prompt.go`, `internal/review/reviewer.go`, `templates/GENERATE_EPIC.md`, `templates/epic-example.md`, `internal/sprint/prompt_test.go`
**Scope**: Add effort-level logging, max-effort quality directive in prompts, effort-aware no-op threshold, effort-aware review bias, update templates with effort documentation, build summary display.

---

## Risk Analysis

| Risk | Severity | Mitigation |
|------|----------|-----------|
| Breaking existing epic files that don't have `@effort` | LOW | Empty string = auto/backward compatible, no validation when unset |
| AI ignoring effort guidance in Step 2 | MEDIUM | Make guidance prominent (ALL CAPS constraints), validate output sprint count |
| `max` effort producing excessively long prompts | LOW | Cap extended sections, review agent handles the check |
| Effort validation rejecting valid manual epics | MEDIUM | Only validate when `@effort` is explicitly set, not on auto |
| Review behavior change at `low` effort | LOW | Low-effort tasks rarely have `@review_between_sprints` set anyway |

## Backward Compatibility

- **No `@effort` in epic file**: Treated as auto-detect / high (current behavior)
- **No `--effort` flag**: Defaults to empty string (auto)
- **Existing tests**: All pass unchanged — no existing behavior modified
- **Existing epics**: Continue to work identically
- **Existing CLI usage**: `fry run`, `fry prepare` — unchanged behavior when no `--effort` flag

## Files Modified (Complete List)

| File | Type of Change |
|------|---------------|
| `internal/epic/types.go` | Add `EffortLevel` type, methods, add field to `Epic` struct |
| `internal/epic/parser.go` | Parse `@effort` directive |
| `internal/epic/validator.go` | Sprint count validation against effort level |
| `internal/config/config.go` | Add `DefaultEffortLevel` constant |
| `internal/cli/root.go` | Add `--effort` flag |
| `internal/cli/run.go` | Add `--effort` flag, pass to prepare, display in output |
| `internal/cli/prepare.go` | Add `--effort` flag |
| `internal/prepare/prepare.go` | Add `EffortLevel` to `PrepareOpts`, pass to step2 |
| `internal/prepare/software.go` | Add effort-aware sizing guidance function, update Step 2 prompt |
| `internal/prepare/planning.go` | Same as software.go for planning mode |
| `internal/sprint/runner.go` | Log effort level, effort-aware no-op threshold |
| `internal/sprint/prompt.go` | Add `EffortLevel` to `PromptOpts`, quality directive for max |
| `internal/review/reviewer.go` | Effort-aware review bias, add EffortLevel to ReviewPromptOpts |
| `templates/GENERATE_EPIC.md` | Add effort level documentation |
| `templates/epic-example.md` | Add `@effort` to example and directive reference |

## Files Created

| File | Purpose |
|------|---------|
| `internal/epic/types_test.go` | Tests for EffortLevel type, parsing, methods |
