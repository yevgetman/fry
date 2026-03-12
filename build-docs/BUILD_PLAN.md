# Build Plan: Planning Output Directory & Ordered Filenames

## Problem Statement

When `fry prepare --planning` runs and the resulting sprints execute, all output documents (deliverables like `brand-positioning.md`, `market-analysis.md`, etc.) land directly in `plans/`. This directory also contains the input files `executive.md` and `plan.md`, polluting the scope and making it hard to distinguish inputs from outputs.

Additionally, the produced documents have flat, unordered filenames with no indication of when they were generated relative to each other or how they relate logically. A user scanning `plans/` sees an undifferentiated list of files with no sense of sequence or grouping.

## Solution Overview

Two changes:

1. **Output directory separation**: Route all planning-mode deliverables to `plans/output/` instead of `plans/`. The input files (`executive.md`, `plan.md`) remain at `plans/`.

2. **Ordered, categorized filenames**: Instruct the epic generator (Step 2) to produce sprint prompts that use a naming convention encoding sequence order and logical category:
   ```
   plans/output/1--research--market-landscape.md
   plans/output/2--research--competitor-profiles.md
   plans/output/3--analysis--positioning-options.md
   plans/output/4--analysis--pricing-model.md
   plans/output/5--strategy--go-to-market.md
   plans/output/6--synthesis--executive-summary.md
   ```

## Architecture: Where Filenames Are Determined

Output filenames are **not hardcoded** in Go. They flow through the system like this:

```
plans/plan.md (Section 4: "Document Deliverables")
    ↓
Step 2 prompt (PlanningStep2Prompt) tells the LLM:
  "Sprint prompts must specify exact output filenames from plans/plan.md"
    ↓
LLM generates epic.md with @sprint blocks whose @prompt text says:
  "Write your analysis to plans/market-analysis.md"
    ↓
Sprint execution: AI agent reads the prompt and writes the file
    ↓
Step 3 prompt (PlanningStep3Prompt) generates verification.md
  with @check_file directives referencing those same paths
```

The fix is therefore primarily **prompt engineering** — we change the instructions given to the LLM in Steps 0, 2, and 3 to specify the new directory and naming convention. We also need minor code changes to ensure the directory exists and to update any hardcoded path references.

---

## Detailed Changes

### 1. `internal/config/config.go` — Add output directory constant

**What**: Add a constant for the planning output directory.

```go
const (
    // ... existing constants ...
    PlanningOutputDir = "plans/output"
)
```

**Why**: Single source of truth for the output path, referenced by both code and prompt templates. Keeps the convention consistent if the path ever changes.

### 2. `internal/prepare/prepare.go` — Ensure output directory exists

**What**: Before running Step 2 in planning mode, create the `plans/output/` directory.

**Where**: In `RunPrepare()`, after the plan file is read (around line 102), add:

```go
if opts.Planning {
    outputDir := filepath.Join(projectDir, config.PlanningOutputDir)
    if err := os.MkdirAll(outputDir, 0o755); err != nil {
        return fmt.Errorf("run prepare: create planning output dir: %w", err)
    }
}
```

**Why**: The AI agent writing output during sprint execution needs the directory to exist. Creating it during prepare is deterministic and early.

### 3. `internal/prepare/planning.go` — Update Step 0 prompt (plan generation)

**What**: When the LLM generates `plans/plan.md` from `plans/executive.md`, it includes a "Document Deliverables" section that lists the output filenames. This section currently doesn't specify a subdirectory or naming convention. Update the Step 0 prompt to instruct the LLM to:

- Place all deliverable documents under `plans/output/`
- Use ordered, categorized filenames

**Where**: `PlanningStep0Prompt()`, line 22-26. Expand the "Document Deliverables" instruction:

Current:
```
4. **Document Deliverables**
```

Updated:
```
4. **Document Deliverables** — List every output document with its FULL path under plans/output/.
   Use ordered, categorized filenames following this convention:
     {sequence}--{category}--{name}.md
   Where:
   - {sequence} is a number (1, 2, 3...) indicating production order
   - {category} is a short grouping label (e.g., research, analysis, strategy, synthesis)
   - {name} is a descriptive kebab-case name
   Example: plans/output/1--research--market-landscape.md
   Group related documents under the same category. The sequence should reflect
   the logical dependency order (documents that inform later ones come first).
```

**Edge cases**:
- Step 0 only runs when `plans/plan.md` doesn't exist yet (conditional on line 74)
- If a user writes `plan.md` manually, their "Document Deliverables" section may not follow this convention — Step 2 should still enforce it (see below)

### 4. `internal/prepare/planning.go` — Update Step 2 prompt (epic generation)

**What**: This is the critical change. The Step 2 prompt generates the `epic.md` file whose sprint prompts contain the actual output filenames the agent will write to. Update `PlanningStep2Prompt()` to enforce the output directory and naming convention.

**Where**: `PlanningStep2Prompt()`, lines 87-97. Add new rules and modify existing ones:

Current line 94:
```
- Sprint prompts must specify exact output filenames, required sections, and concrete
  analytical requirements from plans/plan.md — never vague instructions.
```

Replace with:
```
- ALL document deliverables MUST be written to the plans/output/ directory, NOT to plans/ directly.
  The plans/ directory is reserved for input files (executive.md, plan.md).
- Sprint prompts must specify exact output filenames using the ordered category convention:
    {sequence}--{category}--{name}.md
  Where:
  - {sequence} is a global sequence number across ALL sprints (not per-sprint). If Sprint 1
    produces documents 1-3 and Sprint 2 produces documents 4-5, the numbering is continuous.
  - {category} is a short grouping label (research, analysis, strategy, synthesis, etc.)
  - {name} is a descriptive kebab-case name
  Example paths: plans/output/1--research--market-landscape.md, plans/output/4--analysis--pricing-model.md
- Sprint prompts must include the required sections and concrete analytical requirements
  from plans/plan.md for each deliverable — never vague instructions.
- If plans/plan.md lists deliverables with paths that don't follow this convention, translate
  them to the correct convention in the sprint prompts.
```

**Why**: The Step 2 prompt is the authoritative source for output filenames. Even if `plan.md` was authored manually without the convention, Step 2 normalizes everything.

### 5. `internal/prepare/planning.go` — Update Step 3 prompt (verification generation)

**What**: The verification file uses `@check_file` directives to verify output documents exist. These must reference the new `plans/output/` paths.

**Where**: `PlanningStep3Prompt()`, lines 186-188. Add a rule:

```
- All document deliverable paths in @check_file and @check_file_contains directives
  must reference the plans/output/ directory (e.g., plans/output/1--research--market-landscape.md).
  Do NOT reference plans/ directly for output documents.
```

### 6. `internal/prepare/planning.go` — Update Step 1 prompt (AGENTS.md generation)

**What**: The AGENTS.md rules should include a rule about the output directory convention so the sprint-executing agent knows where to write.

**Where**: `PlanningStep1Prompt()`, lines 53-55. Add to the rule topics:

Current:
```
Rules should cover scope and domain boundaries, analytical frameworks and methodology,
document quality standards, research and evidence standards, and explicit prohibitions.
```

Updated:
```
Rules should cover scope and domain boundaries, analytical frameworks and methodology,
document quality standards, research and evidence standards, output file conventions,
and explicit prohibitions.

One rule MUST state: "All document deliverables must be written to plans/output/ using
the naming convention {sequence}--{category}--{name}.md. Never write output documents
directly to plans/."
```

### 7. `internal/sprint/prompt.go` — No changes needed

The sprint prompt assembly (`AssemblePrompt`) references `plans/plan.md` as strategic context (line 67). This is correct — `plan.md` is an input file and stays in `plans/`. The actual output paths are embedded in the sprint prompt text from `epic.md`, which is already updated by changes #4 above.

### 8. `internal/review/reviewer.go` — No changes needed

The reviewer references `plans/plan.md` (line 92) as the original plan. Output document paths only appear in the sprint prompts that the reviewer reads from `epic.md`, which are already correctly formatted by the time review runs.

### 9. `internal/verify/runner.go` — No changes needed

The verification runner reads `@check_file` paths from `verification.md`. As long as Step 3 generates the correct `plans/output/` paths (change #5), the runner works as-is.

### 10. `internal/config/config.go` — Update `AgentInvocationPrompt`

**What**: The agent invocation prompt tells the AI to "read plans/plan.md for strategic context." This is correct and unchanged. But verify there are no references to output file locations in this constant.

Current (line 23):
```go
AgentInvocationPrompt = "Read and execute ALL instructions in .fry/prompt.md. Before starting,
read .fry/sprint-progress.txt for context from previous iterations in this sprint, and
.fry/epic-progress.txt for summaries of prior sprints. Also read plans/plan.md for
strategic context on how this sprint fits the overall plan. After completing your work,
append your progress to .fry/sprint-progress.txt."
```

**Status**: No change needed. The output paths are in the sprint prompt, not here.

---

## Naming Convention Specification

### Format
```
{sequence}--{category}--{name}.md
```

### Rules

| Component | Format | Examples |
|-----------|--------|----------|
| `sequence` | Integer, 1-indexed, globally unique across all sprints | `1`, `2`, `10` |
| `category` | Lowercase kebab-case, 1-2 words | `research`, `analysis`, `strategy`, `synthesis`, `deep-dive` |
| `name` | Lowercase kebab-case, descriptive | `market-landscape`, `competitor-profiles`, `pricing-model` |
| Separator | Double dash `--` | Distinguishes from single `-` used within kebab-case segments |

### Sequencing

- Numbering is **global** across all sprints, not per-sprint
- Numbers reflect **production order** (Sprint 1 outputs come before Sprint 2 outputs)
- Within a sprint, documents are numbered in the order they should be read/referenced
- Gaps are acceptable if documents are removed during replanning

### Categories

Categories are determined by the LLM based on the plan content, but common patterns include:

| Category | Typical Use |
|----------|------------|
| `research` | Primary research, data gathering, landscape surveys |
| `analysis` | Analytical frameworks applied to research |
| `deep-dive` | Focused investigation of a specific subtopic |
| `strategy` | Strategic recommendations, positioning, roadmaps |
| `specs` | Detailed specifications, requirements |
| `synthesis` | Cross-cutting summaries, executive reports |
| `appendix` | Supporting data, reference materials |

### Example Directory Layout

```
plans/
├── executive.md                              # INPUT: human-authored context
├── plan.md                                   # INPUT: generated planning methodology
└── output/                                   # OUTPUT: all deliverables
    ├── 1--research--market-landscape.md
    ├── 2--research--competitor-profiles.md
    ├── 3--research--user-interviews.md
    ├── 4--analysis--market-positioning.md
    ├── 5--analysis--competitive-gaps.md
    ├── 6--strategy--brand-positioning.md
    ├── 7--strategy--pricing-model.md
    ├── 8--strategy--go-to-market.md
    └── 9--synthesis--executive-summary.md
```

---

## Implementation Order

### Sprint 1: Config & Directory Setup
**Files**: `internal/config/config.go`, `internal/prepare/prepare.go`
**Scope**: Add `PlanningOutputDir` constant, create `plans/output/` during prepare in planning mode.
**Tests**: Verify directory creation in `prepare_test.go`.

### Sprint 2: Prompt Engineering — Steps 0, 1, 2, 3
**Files**: `internal/prepare/planning.go`
**Scope**: Update all four planning step prompts to enforce the `plans/output/` directory and the `{sequence}--{category}--{name}.md` naming convention. This is the core change — everything else is supporting infrastructure.
**Tests**: Verify prompt strings contain the new instructions in `planning_test.go` or `prepare_test.go`.

### Sprint 3: Documentation & End-to-End Validation
**Files**: `docs/` (if applicable), `templates/` (if planning examples exist)
**Scope**: Update any docs that reference the `plans/` directory for output. Run a full `fry prepare --planning` cycle against a test `executive.md` to verify the LLM produces correctly-pathed output filenames in `epic.md` and `verification.md`.

---

## Risk Analysis

| Risk | Severity | Mitigation |
|------|----------|-----------|
| LLM ignores naming convention in Step 2 output | MEDIUM | Make instructions prominent with examples. Could add post-generation validation that checks epic.md sprint prompts reference `plans/output/` |
| LLM ignores naming convention in Step 0 plan | LOW | Step 2 has a fallback instruction to translate non-conforming paths |
| Existing manually-authored plan.md files reference old paths | LOW | Step 2 explicitly handles this: "If plans/plan.md lists deliverables with paths that don't follow this convention, translate them" |
| Breaking existing verification checks | LOW | Verification is regenerated each time `fry prepare` runs; old checks are overwritten |
| Category names become inconsistent across runs | LOW | Acceptable — categories are descriptive labels, not schema. The sequence number provides the canonical ordering |
| `plans/output/` not in `.gitignore` | N/A | `plans/` is already in `.gitignore` (line 6), which covers `plans/output/` |

## Backward Compatibility

- **Software mode** (`fry prepare` without `--planning`): Completely unaffected. Software mode uses different prompt functions (`SoftwareStep*Prompt`) and writes output files to the project source tree, not `plans/`.
- **Existing planning runs**: The `plans/output/` directory won't exist from prior runs. New runs create it. Old output files in `plans/` are not migrated — they remain as-is.
- **Manual plan.md files**: Step 2 prompt explicitly handles the case where `plan.md` deliverables don't follow the convention.

## Files Modified (Complete List)

| File | Type of Change |
|------|---------------|
| `internal/config/config.go` | Add `PlanningOutputDir` constant |
| `internal/prepare/prepare.go` | Create `plans/output/` in planning mode |
| `internal/prepare/planning.go` | Update Steps 0, 1, 2, 3 prompts for output dir and naming convention |

## Files NOT Modified (and why)

| File | Reason |
|------|--------|
| `internal/sprint/prompt.go` | References `plans/plan.md` (input file), not output paths |
| `internal/review/reviewer.go` | References `plans/plan.md` (input file); output paths come from epic.md |
| `internal/verify/runner.go` | Reads paths from `verification.md`, which is regenerated with correct paths |
| `internal/epic/parser.go` | No file path awareness; just parses directives |
| `internal/sprint/runner.go` | Executes prompts; doesn't construct output paths |
