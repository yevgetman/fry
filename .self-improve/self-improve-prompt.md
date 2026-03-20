# Self-Improvement Prompt

> This prompt is used by the automated self-improvement cron to generate improvement roadmaps for Fry.

---

You are analyzing the Fry codebase to produce an actionable self-improvement roadmap. Your goal is to identify concrete, high-impact improvements across four categories: **Bug Fixes**, **Testing Improvements**, **New Features**, and **Existing Feature Improvements**.

## Instructions

1. **Read the entire codebase.** Start with `CLAUDE.md`, `README.md`, and `README.LLM.md` to understand conventions, architecture, and invariants. Then read all source files in `internal/`, `cmd/`, and `templates/`.

2. **Read prior roadmaps.** Check all existing `.md` files in `.self-improve/` to understand what has already been identified and what status items have. Do not re-list completed items. Do not duplicate items that are still open in recent roadmaps — instead, reference them and note any new findings.

3. **Identify improvements.** For each finding, provide:
   - A short descriptive title
   - The specific file(s) and line number(s) affected
   - Severity or impact rating (High / Medium / Low)
   - A clear description of the problem or opportunity
   - A concrete implementation plan (numbered steps)
   - Status: `[ ]` (all items start as not-started)

4. **Prioritize.** Order items within each category by impact. End with a suggested execution order table that considers impact, effort, and dependencies.

5. **Be specific.** Reference actual function names, line numbers, types, and file paths. Vague suggestions like "improve error handling" are not useful — say exactly where and what.

6. **Respect invariants.** All suggestions must be compatible with the architecture invariants in CLAUDE.md Section 9. Do not suggest adding dependencies, restructuring the parser, or breaking the engine interface.

7. **Check for regressions.** Run `make test` and `make build` mentally — if your suggestion would break existing tests or compilation, note what else needs to change.

## Output Format

Write the roadmap as a Markdown file with this structure:

```markdown
# Fry Self-Improvement Roadmap

> **Purpose:** Actionable improvements to fry, organized by category. Execute one at a time in future sessions.
> **Generated:** YYYY-MM-DD

---

## Status Key

- [ ] Not started
- [x] Complete

---

## A. Bug Fixes
### A1. Title
- **File:** `path/to/file.go:line`
- **Severity:** High/Medium/Low
- **Description:** ...
- **Fix:** ...
- **Status:** [ ]

## B. Testing Improvements
(same format)

## C. New Features
(same format, with **Impact** and **Effort** instead of Severity)

## D. Existing Feature Improvements
(same format)

## Execution Order (Suggested)
(table with Order, Item, Category, Rationale)
```

## Important

- Save the output to `.self-improve/YYYY-MM-DD-roadmap.md` using today's date.
- After saving, run `make test && make build` to confirm the codebase is healthy.
- Stage the new roadmap file, commit with message `Add self-improvement roadmap YYYY-MM-DD`, and push to origin master.
- Do NOT modify any source code. This is an analysis-only task.
