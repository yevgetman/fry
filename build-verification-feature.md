# Build Plan: Independent Sprint Verification

## Problem

fry.sh relies on an "honor system" for sprint completion. The AI agent self-reports
via a `<promise>TOKEN</promise>` output. Nothing independently verifies that the
agent's work actually meets the sprint's requirements. If the agent outputs the
promise token without completing all tasks, fry.sh moves on.

## Solution

Add an independent verification layer:

1. A new `verification.md` file format with machine-parseable per-sprint checks
2. A `verification-example.md` format reference (ships with fry)
3. Generation of `verification.md` by `fry-prepare.sh` (Step 3)
4. Independent check execution by `fry.sh` after promise token detection
5. Updated epic prompt structure to reference `verification.md` instead of
   duplicating prose verification checklists

## Deliverables

### 1. `verification-example.md` — Format Reference

A documented template file (like `epic-example.md`) that shows the verification
file format. Must include:

- File header explaining its purpose
- Per-sprint blocks keyed by `@sprint N`
- Four check primitives with documentation:

  | Directive | Behavior | Passes when |
  |---|---|---|
  | `@check_file <path>` | File exists and is non-empty | `test -s <path>` |
  | `@check_file_contains <path> <pattern>` | File contains string/regex | `grep -qE <pattern> <path>` |
  | `@check_cmd <command>` | Run a shell command | Exit code 0 |
  | `@check_cmd_output <command> \| <pattern>` | Run command, check output | stdout matches pattern |

- Example checks for a typical scaffolding sprint
- Example checks for a typical integration sprint
- Guidelines on writing good checks (concrete, no prose, no subjective criteria)

### 2. `fry-prepare.sh` — Step 3: Generate `verification.md`

After generating AGENTS.md (Step 1) and epic.md (Step 2), add:

**Step 3: Generate verification.md**

- Reads `plans/plan.md`, the just-generated `epic.md`, and `verification-example.md`
- Produces executable checks per sprint derived from the plan and sprint tasks
- The generation prompt must emphasize: every check must be a concrete command
  or file assertion, no prose, no subjective criteria
- Uses the same `run_agent` dispatch and `--keep-verification` flag pattern
- Validates that the output contains `@sprint` blocks and `@check_*` directives

Update prerequisites check to include `verification-example.md`.

### 3. `fry.sh` — Verification Parser and Runner

#### 3a. Parser: `parse_verification()`

Add a parser function that reads `verification.md` and stores checks per sprint.
Use a bash array to store checks for each sprint. The parser should handle:

- `@sprint N` blocks (same pattern as epic parser)
- `@check_file <path>` — store as `FILE|<path>`
- `@check_file_contains <path> <pattern>` — store as `FILE_CONTAINS|<path>|<pattern>`
- `@check_cmd <command>` — store as `CMD|<command>`
- `@check_cmd_output <command> | <pattern>` — store as `CMD_OUTPUT|<command>|<pattern>`
- Ignore comments (lines starting with `#`) and blank lines
- Warn on unrecognized `@check_*` directives

Storage: Use a flat array `VERIFICATION_CHECKS` with entries formatted as
`<sprint_num>|<type>|<args>`. The runner filters by sprint number.

#### 3b. Runner: `run_verification_checks()`

A function that takes a sprint number and runs all checks for that sprint.
Returns 0 if all pass, 1 if any fail. Behavior:

- Iterate over all checks for the given sprint
- For each check, log what it's running and whether it passed/failed
- On failure, log the specific check that failed but continue running remaining
  checks (report all failures, don't stop at first)
- Return summary: "N/M checks passed"

#### 3c. Integration into `run_sprint()`

Modify the sprint runner to call `run_verification_checks()` at two points:

**After promise token found (primary path):**
- Run checks. If all pass → sprint PASS (current behavior, now verified).
- If any fail → sprint FAIL with message "Promise found but verification failed"
- Result: `SPRINT_RESULTS[$sprint_num]="FAIL (verification: X/Y checks passed)"`

**After max_iterations exhausted with no promise:**
- Run checks. If all pass → sprint WARN with message "All checks passed but
  promise token not found. Treating as PASS."
- Set result to PASS (auto-recovery from missing promise).
- If any fail → sprint FAIL (current behavior, unchanged).

#### 3d. Verification file is optional

If `verification.md` does not exist, skip all verification steps silently.
The existing promise-only behavior should be fully preserved when no
verification file is present. This ensures backward compatibility.

#### 3e. Dry-run support

Update `dry_run_report()` to show verification check counts per sprint if
`verification.md` exists. Show `[ok]` or `[MISS]` for the file in the
project structure section.

#### 3f. Global directive for verification file path

Add `@verification <filename>` global directive to the epic parser. Default
value: `verification.md`. This allows custom filenames (e.g., per-phase
verification files).

### 4. `epic-example.md` — Update Verification Section

Replace the prose "Verify:" section in sprint prompt templates with a reference
to verification.md:

```
Verification criteria are defined in verification.md (sprint N).
Review these checks — they will be run independently after you signal completion.
```

Keep the "Verify:" section concept but note that concrete checks live in
verification.md. The sprint prompt should still describe what success looks like
in prose (for the agent's understanding), but point to verification.md as the
source of truth for pass/fail.

### 5. `GENERATE_EPIC.md` — Update for Verification Awareness

Update the generation prompt and quality checklist to reflect that:
- Sprint prompts should reference verification.md for concrete checks
- The "Verify:" section in sprint prompts should describe success criteria
  in prose and reference verification.md
- Add a checklist item: "Verification references point to verification.md"

### 6. `README.md` — Documentation Updates

Update the following sections:
- "How It Works" diagram: add verification.md to the flow
- "Key mechanisms" table: add verification.md entry
- "What fry ships with" file list: add verification-example.md
- "What gets generated at runtime" file list: add verification.md
- "Epic File Format" global directives table: add @verification
- "fry-prepare.sh" section: mention Step 3
- "File Reference" table: add verification.md and verification-example.md
- Add a "Verification" section explaining the independent check system

### 7. `.gitignore` — No Changes Needed

`verification.md` should be committed (like `epic.md`). No gitignore changes.

## Implementation Order

1. `verification-example.md` (format reference — needed by other steps)
2. `fry.sh` changes (parser + runner + integration + dry-run + directive)
3. `fry-prepare.sh` changes (Step 3 generation)
4. `epic-example.md` updates (verification references)
5. `GENERATE_EPIC.md` updates (prompt and checklist)
6. `README.md` updates (documentation)

## Design Constraints

- Verification file is always optional — backward compatibility is mandatory
- All four check primitives must be supported
- Checks run to completion (report all failures, don't stop at first)
- The parser must warn on unrecognized directives (same pattern as epic parser)
- No new dependencies — only bash builtins, grep, test
- Sprint prompts in epic.md should reference verification.md, not duplicate checks
- `@verification` directive defaults to `verification.md`
- `--keep-verification` flag prevents overwriting existing verification.md
