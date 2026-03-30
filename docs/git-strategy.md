# Git Strategy

Fry supports multiple **git strategies** for isolating build work. By default, the [triage gate](triage.md) auto-selects the strategy based on task complexity — complex tasks get a worktree, simpler tasks get a branch. You can override this with `--git-strategy`.

## Strategies

| Strategy | Description |
|---|---|
| `auto` | Triage auto-selects: COMPLEX -> worktree, SIMPLE/MODERATE -> branch. When no triage runs (epic already exists), defaults to `current` for backwards compatibility. |
| `current` | Work directly on the current branch. No branch or worktree created. This is the previous default behavior. |
| `branch` | Create a new git branch for the build. Switches to it before the first sprint. |
| `worktree` | Create an isolated git worktree under `.fry-worktrees/`. Build runs entirely inside the worktree. |

## CLI Flags

| Flag | Description |
|---|---|
| `--git-strategy <auto\|current\|branch\|worktree>` | Git isolation strategy (default: `auto`) |
| `--branch-name <name>` | Explicit branch name. Overrides auto-generated name. Cannot be used with `--git-strategy current`. |

## Auto-Resolution Rules

The `auto` strategy resolves to a concrete strategy at runtime:

1. **Triage runs** (no epic exists) -- the triage classifier determines complexity, then:
   - COMPLEX -> `worktree`
   - SIMPLE or MODERATE -> `branch`
2. **No triage** (epic already exists, or `--continue`/`--resume`) -- defaults to `current` for backwards compatibility.
3. **Explicit `--git-strategy`** -- always takes precedence over auto-resolution.

## Branch Names

Branch names follow the pattern `fry/<slug>` where `<slug>` is a lowercased, hyphenated version of the epic name (max 50 characters).

- Auto-generated from the epic `@epic` name (e.g., `@epic My REST API` -> `fry/my-rest-api`)
- Override with `--branch-name my-feature` to use an explicit name
- If no epic name is available, falls back to `fry/build`

## Triage Integration

When triage runs, the classification display includes a **Git:** line showing the resolved strategy:

```
── Triage classification ───────────────────────────────────────
Difficulty:  COMPLEX
Effort:      high
Git:         worktree
Reason:      Multi-service architecture with database migrations.
Action:      Full prepare pipeline (3-4 LLM calls)
─────────────────────────────────────────────────────────────────
Accept this classification? [Y/n/a] (a = adjust)
```

When adjusting (`a`), you can override the git strategy alongside difficulty and effort:

```
Difficulty [COMPLEX] (simple/moderate/complex, or Enter to keep):
Effort [high] (fast/standard/high/max, or Enter to keep):
Git strategy [worktree] (auto/current/branch/worktree, or Enter to keep):
```

## Continue / Resume Behavior

The resolved strategy is persisted to `.fry/git-strategy.txt` after setup. When `--continue` or `--resume` is used:

1. Fry reads `.fry/git-strategy.txt` to recover the strategy, branch name, and working directory
2. For `branch` strategy: checks out the existing branch
3. For `worktree` strategy: reattaches to the existing worktree directory
4. For `current` strategy: no action needed

If the persisted strategy file does not exist (builds started before this feature), `--continue`/`--resume` defaults to `current`.

## Worktree Lifecycle

When strategy is `worktree`:

1. **Creation** -- a git worktree is created at `.fry-worktrees/<slug>/` with a new branch `fry/<slug>`
2. **Artifact copy** -- `.fry/` and `plans/` are copied from the original project directory into the worktree so the sprint runner finds all build artifacts
3. **Build execution** -- all sprint operations (agent runs, sanity checks, alignment, audit) happen inside the worktree directory
4. **Auto-merge on success** -- when the build completes successfully, Fry automatically merges the worktree branch into the original branch, removes the worktree, deletes the branch, and cleans up the strategy file. The log shows:
   ```
     GIT: merging worktree branch fry/my-rest-api into main...
     GIT: worktree merged and cleaned up
   ```
5. **Preservation on failure** -- if the build fails, the worktree is preserved for inspection. Fry prints the path and a removal command:
   ```
     GIT: worktree preserved at .fry-worktrees/my-rest-api
          To remove: git worktree remove .fry-worktrees/my-rest-api
   ```

The `.fry-worktrees/` directory is listed in `.gitignore`.

## Branch Strategy Lifecycle

When strategy is `branch`:

1. **Creation** -- a new branch `fry/<slug>` is created and checked out
2. **Build execution** -- all operations run in the same project directory, on the new branch
3. **Post-build** -- you remain on the feature branch. Merge or delete as needed.

If the branch already exists (and `--continue`/`--resume` is not set), Fry exits with an error suggesting `--branch-name` or manual deletion.

## Artifacts

| File | Purpose |
|---|---|
| `.fry/git-strategy.txt` | Persisted strategy for `--continue`/`--resume` reattachment |
| `.fry-worktrees/` | Parent directory for worktree checkouts (gitignored) |

## Examples

```bash
# Auto strategy (default) — triage decides
fry --user-prompt "add a REST endpoint"

# Force worktree for a complex task
fry --git-strategy worktree --user-prompt "build microservice architecture"

# Force current branch (previous behavior)
fry --git-strategy current

# Use a specific branch name
fry --git-strategy branch --branch-name feature/auth-system

# Continue a build that used worktree strategy
fry --continue

# Resume on a specific sprint (reattaches to persisted strategy)
fry --resume --sprint 4
```

## Interaction with Other Flags

- `--continue` / `--resume`: reads persisted strategy from `.fry/git-strategy.txt`. Ignores `--git-strategy` if set (uses persisted value).
- `--dry-run`: strategy is resolved and displayed but no branch or worktree is created.
- `--no-project-overview`: skips triage confirmation (including git strategy display), but auto-resolution still applies.
- `--full-prepare`: bypasses triage but respects `--git-strategy`. When strategy is `auto` and `--full-prepare` is used, defaults to `current`.
