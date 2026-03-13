# Project Structure

## Your Project with fry

```
your-project/
  .gitignore                             # Auto-updated with .fry/ entry
  plans/                                 # YOUR INPUT (committed to your repo)
    plan.md                              #   Detailed build plan
    executive.md                         #   Executive context (optional)
    output/                              #   Planning mode deliverables (--planning only)
      1--research--market-landscape.md   #     Ordered, categorized output documents
      2--analysis--positioning.md        #     {sequence}--{category}--{name}.md
  media/                                 # OPTIONAL ASSETS (committed to your repo)
    logo.png                             #   Images, PDFs, fonts, data files, etc.
    wireframe.pdf                        #   Referenced in plans, copied into builds
  .fry/                                  # Generated artifacts (gitignored)
    AGENTS.md                            #   Operational rules for the AI agent
    epic.md                              #   Sprint definitions
    verification.md                      #   Independent verification checks
    prompt.md                            #   Assembled per sprint
    user-prompt.txt                      #   Persisted user directive (optional)
    sprint-progress.txt                  #   Per-sprint iteration memory
    epic-progress.txt                    #   Cross-sprint compacted summary
    deviation-log.md                     #   Review decision audit trail
    review-prompt.md                     #   Assembled reviewer prompt (transient)
    replan-prompt.md                     #   Assembled replanner prompt (transient)
    audit-prompt.md                      #   Assembled audit prompt (transient, cleaned up after audit)
    sprint-audit.txt                     #   Audit findings (transient, cleaned up after audit)
    build-logs/                          #   Per-iteration logs
    .fry.lock                            #   Concurrency lock
```

Unlike the bash version, fry is installed as a standalone binary — it does not live inside your project's `.fry/` directory. The `.fry/` directory contains only generated artifacts.

## File Reference

### Input Files (Your Authorship)

| File | Purpose | Required |
|---|---|---|
| `plans/plan.md` | Detailed build plan with technical decisions | At least one of plan.md or executive.md |
| `plans/executive.md` | Executive context: vision, goals, scope | At least one of plan.md or executive.md |
| `plans/output/` | Planning mode deliverables (ordered, categorized `.md` files) | Created automatically in `--planning` mode |
| `media/` | Images, PDFs, fonts, data files, and other assets referenced in plans | No — entirely optional |

### Generated Artifacts

| File | Purpose | Created by |
|---|---|---|
| `.fry/AGENTS.md` | Operational rules for the AI agent | `fry prepare` (Step 1) |
| `.fry/epic.md` | Sprint definitions | `fry prepare` (Step 2) |
| `.fry/verification.md` | Independent verification checks | `fry prepare` (Step 3) |
| `.fry/prompt.md` | Assembled per-sprint prompt | `fry run` at runtime |
| `.fry/user-prompt.txt` | Persisted user directive | `fry run` or `fry prepare` |
| `.fry/sprint-progress.txt` | Per-sprint iteration memory | `fry run` at runtime |
| `.fry/epic-progress.txt` | Cross-sprint compacted summary | `fry run` at runtime |
| `.fry/deviation-log.md` | Audit trail of review decisions | `fry run` at runtime |
| `.fry/review-prompt.md` | Assembled reviewer prompt (transient) | `fry run` at runtime |
| `.fry/replan-prompt.md` | Assembled replanner prompt (transient) | `fry run` at runtime |
| `.fry/audit-prompt.md` | Assembled audit/fix prompt (transient) | `fry run` at runtime |
| `.fry/sprint-audit.txt` | Audit findings from the audit agent (transient) | `fry run` at runtime |
| `.fry/build-logs/` | Per-iteration and per-sprint logs | `fry run` at runtime |
| `.fry/.fry.lock` | Concurrency lock | `fry run` at runtime |

### Auto-Generation Behavior

- **`fry run`** calls `fry prepare` only when the epic file does not exist on disk
- **`fry prepare`** always **overwrites** all `.fry/` artifacts when run
- If `plan.md` was auto-generated (Step 0), it persists in `plans/` and is treated as user-authored on subsequent runs — delete it manually to force re-generation
- To re-run fry with a new plan, update your input files and delete `.fry/epic.md` (or run `fry prepare` directly)

## Git Integration

fry automatically:
- Initializes a git repository if one doesn't exist
- Adds `.fry/`, `.env`, and `.DS_Store` to `.gitignore`
- Sets a local git identity (`fry` / `fry@automated`) if none is configured
- Creates git checkpoints after each sprint with descriptive commit messages

## Concurrency Control

The `.fry/.fry.lock` file prevents concurrent fry instances from running in the same project. The lock file contains the PID of the running process. Stale locks from dead processes are automatically cleaned up.
