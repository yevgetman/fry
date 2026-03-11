# Self-Heal Prompt Flow

## When it triggers

After a sprint finishes, fry runs **verification checks** (`@check_file`, `@check_file_contains`, `@check_cmd`, `@check_cmd_output`). If any fail, `run_heal_loop()` kicks in.

## Step 1: Collect failures (`collect_failed_checks`)

Re-runs every verification check for that sprint and builds a **failure report** file. Each check type produces a specific failure message:

- **FILE**: `"FAILED: File missing or empty: <path>"`
- **FILE_CONTAINS**: `"FAILED: File '<path>' does not contain pattern: <pattern>"`
- **CMD**: `"FAILED: Command failed: <cmd>"` + truncated output (first 20 lines)
- **CMD_OUTPUT**: `"FAILED: Command output mismatch: <cmd>"` + expected pattern vs actual output (first 10 lines)

The report header shows `"Verification: 2/5 checks passed."` so the agent knows the scope.

## Step 2: Build the heal prompt

The prompt is written to `.fry/prompt.md` (overwriting the sprint prompt) and has this structure:

```markdown
# HEAL MODE — Sprint 3: Setup Auth

## What happened
The sprint finished its work but FAILED independent verification checks.
Your job is to fix ONLY the issues described below. Do not start the sprint over.
Do not refactor or reorganize. Make the minimum changes needed to pass the checks.

## Failed verification checks

Verification: 2/5 checks passed.

Failed checks:
- FAILED: File missing or empty: src/auth.ts
- FAILED: Command failed: npm run build
  Output (truncated):
  <first 20 lines of error output>

## Instructions
1. Read .fry/sprint-progress.txt for context on what was built this sprint
2. Read .fry/epic-progress.txt for context on what was built in prior sprints
3. Read the failed checks above carefully
4. Fix each failure — create missing files, fix build errors, correct config
5. After fixing, do a final sanity check (e.g., run the build command if applicable)
6. Append a brief note to .fry/sprint-progress.txt about what you fixed in this heal pass

## Context files
- Read .fry/sprint-progress.txt for current sprint iteration history
- Read .fry/epic-progress.txt for prior sprint summaries
- Read plans/plan.md for the overall project plan
- Read plans/executive.md for executive context

Do NOT output any promise tokens. Just fix the issues.
```

## Step 3: Run the agent

The agent is invoked with the one-liner:

> *"Read and execute ALL instructions in .fry/prompt.md. This is a HEAL pass — fix the verification failures described in the prompt."*

## Step 4: Re-verify and loop

After the agent finishes, fry re-runs the pre-sprint hook (e.g. `npm install`) then re-runs verification checks. If they pass, the heal succeeds. If they fail:

- The failure report is **appended to `.fry/sprint-progress.txt`** so the next heal attempt has context about what was already tried
- The loop continues up to `@max_heal_attempts` (default 3)

## Key design decisions

1. **Minimal scope** — The prompt explicitly says "do not start over", "do not refactor", "minimum changes only"
2. **No promise tokens** — Heal passes skip the promise/verification-from-output flow; they only care about the hard verification checks passing
3. **Accumulating context** — Each failed heal attempt appends its failure report to `.fry/sprint-progress.txt`, so subsequent attempts know what was already tried and what's still broken
4. **Same agent, fresh context** — Each heal attempt is a fresh agent invocation reading the same `.fry/prompt.md` file, but with richer `.fry/sprint-progress.txt` context from prior attempts
