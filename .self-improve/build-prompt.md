# Self-Improvement Build Prompt

Implement the items listed below from the self-improvement roadmap. Each item includes a description of the problem and a concrete fix plan — follow the fix plan.

## Items to Implement

The orchestrator has selected 2-3 items from `assets/roadmap.json`. They are reproduced here for convenience:

<!-- ORCHESTRATOR: Replace this section with the selected items before each run -->

## Instructions

1. **Read `CLAUDE.md` in full** before writing any code. Follow every convention.
2. **Read `README.LLM.md`** for the architectural map — understand the package structure, types, and execution flow before making changes.
3. **For each item**, read the affected files and their existing tests before modifying anything.
4. **Implement each item as a separate commit.** One logical change per commit, imperative present tense message (e.g., "Fix lock release error logging in signal handler").
5. **Write or update tests** for every behavioral change. Every test must call `t.Parallel()`.
6. **Update documentation** for any user-visible or architectural change:
   - Feature behavior → the relevant `docs/` file
   - User-visible flags/commands → `docs/commands.md` and `README.md`
   - Architecture/types/flow → `README.LLM.md`
7. **Run `make test && make build`** after each item. Do not proceed to the next item if tests fail — fix the issue first.

## Scope Rules

- Implement only the items listed above. Do not fix unrelated issues you happen to notice.
- If an item requires changes beyond what the fix plan describes (e.g., a dependency on another unimplemented item), note this in sprint progress and move on.
- Do not add dependencies, restructure the epic parser, or break the engine interface.
- Do not refactor surrounding code, add comments to code you didn't change, or make cosmetic improvements outside the scope of the item.

## Verification

This build runs with `--always-verify`. All verification checks must pass. If a check fails, fix the root cause — do not weaken or remove the check.
