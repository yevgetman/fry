# Self-Improvement Planning Prompt

Scan the Fry codebase and identify actionable improvements. Output NEW findings as valid JSON.

## Context

Fry is a Go 1.22 CLI tool that orchestrates AI agents through multi-sprint build loops. Start by reading these files for orientation — do not scan blindly:

- `README.LLM.md` — architectural map: package structure, types, execution flow, CLI flags, constants
- `CLAUDE.md` — coding conventions, architecture invariants, testing standards
- `assets/existing-issues.json` — existing tracked issues (do NOT re-discover these)

Then read the source files relevant to each category below.

## Categories

Investigate each category. It is fine to return zero findings for a category — do not fabricate issues to fill a quota.

### A. Bugs
Search for correctness issues: logic errors, race conditions, unhandled errors, incorrect behavior under edge cases. Assign severity based on user impact.

### B. Testing
Identify gaps in test coverage or weak tests that don't meaningfully validate behavior. Look for: missing edge case tests, tests that pass trivially, untested exported functions, tests that don't call `t.Parallel()`.

### C. New Features
Propose new capabilities that would make Fry more useful. Features must be compatible with the architecture invariants in `CLAUDE.md` Section 9 (single binary, no new dependencies, minimal interface changes). Include a concrete implementation outline.

### D. Improve Existing Features
Evaluate whether existing features are robust, effective, and solve meaningful problems. Suggest extensions, better defaults, or improved behavior. Reference the specific feature and explain what's lacking.

### E. Sunset
Identify dead code, unused exports, vestigial features, or capabilities that add complexity without sufficient value. Propose what to remove and why.

### F. Refactor
Find code that produces correct results but could be faster, more robust, or more maintainable through structural improvement. Same inputs, same outputs, better internals.

### G. Security
Look for: command injection vectors, unsafe input handling, secrets in code, overly permissive file operations, missing input validation at system boundaries.

### H. UI/UX
Evaluate terminal output, progress reporting, error messages, and user-facing text. Identify confusing flows, missing status information, or output that could be more helpful.

### I. Documentation
Check that documentation in `docs/`, `README.md`, `README.LLM.md`, and `CLAUDE.md` is complete, accurate, and consistent with the current code. Flag stale references, missing sections, and undocumented features.

## Rules

1. **Read `assets/existing-issues.json` first.** Do not re-discover items that are already tracked there. If you find a variation of an existing item, skip it.
2. **Be specific.** Every finding must reference actual file paths and line numbers. Vague findings like "improve error handling" are not acceptable — say exactly where and what.
3. **Verify line numbers.** Read the actual current source files before citing line numbers. They shift between runs.
4. **Aim for 1-3 items per category, max 15 items total.** Quality over quantity. One well-described, actionable finding is worth more than five vague ones.
5. **Do not modify any source code.** This is an analysis-only run.
6. **Effort reflects implementation scope:** `low` = contained to 1-2 files with no API changes, `medium` = touches several files or changes function signatures, `high` = cross-cutting refactor across multiple packages. When in doubt, round up.

## Output Format

Write a JSON file to `output/new-findings.json` containing an array of new items. Do NOT include an `id` field — the orchestrator assigns IDs via GitHub Issues.

```json
[
  {
    "category": "bug",
    "title": "Short descriptive title",
    "priority": "high",
    "effort": "low",
    "files": ["internal/cli/run.go:296"],
    "description": "Detailed description of the problem...",
    "fix": "Concrete implementation plan..."
  }
]
```

If no issues are found in any category, write an empty array: `[]`
