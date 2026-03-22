# Self-Improvement Build Prompt

Read `assets/roadmap.json` for the full list of open items. Choose 2-3 items to implement in this build, then implement them.

## Item Selection

Select items from the roadmap based on effort balance. Pick one of these combinations:

- 1 high-effort item, or
- 1 medium + 1 low, or
- 2 medium, or
- 3 low

Use your judgment — prioritize items that are higher priority, have fewer prior attempts, and where you are confident in the fix plan. Avoid items where the fix description is vague or where you'd need to make architectural decisions not covered in the plan.

Do not select more than 3 items. It is better to do 2 items well than 3 items poorly.

## Instructions

1. **Read `CLAUDE.md` in full** before writing any code. Follow every convention.
2. **Read `README.LLM.md`** for the architectural map — understand the package structure, types, and execution flow before making changes.
3. **For each item**, read the affected files and their existing tests before modifying anything.
4. **Implement each item as a separate commit.** One logical change per commit, imperative present tense message that references the item ID (e.g., "A7: Make report.Write() atomic with temp-file-then-rename").
5. **Write or update tests** for every behavioral change. Every test must call `t.Parallel()`.
6. **Update documentation** for any user-visible or architectural change:
   - Feature behavior → the relevant `docs/` file
   - User-visible flags/commands → `docs/commands.md` and `README.md`
   - Architecture/types/flow → `README.LLM.md`
7. **Run `make test && make build`** after each item. Do not proceed to the next item if tests fail — fix the issue first.

## Scope Rules

- Implement only the items you selected. Do not fix unrelated issues you happen to notice.
- If an item requires changes beyond what the fix plan describes (e.g., a dependency on another unimplemented item), note this in sprint progress and move on.
- Do not add dependencies, restructure the epic parser, or break the engine interface.
- Do not refactor surrounding code, add comments to code you didn't change, or make cosmetic improvements outside the scope of the item.

## Verification

This build runs with `--always-verify`. All verification checks must pass. If a check fails, fix the root cause — do not weaken or remove the check.

## Manifest

After completing all items, write the IDs you implemented (one per line) to `output/worked-items.txt`. Only include items you actually implemented — not items you considered but skipped. Example:

```
B6
D9
D10
```
