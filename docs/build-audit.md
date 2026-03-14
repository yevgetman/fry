# Build Audit

The build audit is a final holistic quality gate that runs once the entire epic has completed successfully. A single AI agent session iteratively audits the full codebase, classifies issues by severity, remediates them, and re-audits until the codebase is clean. This complements the per-sprint audit with a cross-cutting review of the finished product.

## How It Works

```
All sprints complete successfully
       |
       v
  Build summary generated
       |
       v
  Build audit agent launched (single session)
       |
       +-- Audit entire codebase
       +-- Classify findings by severity
       +-- Report to audit.md
       +-- If clean (no issues or all LOW) --> stop
       +-- If issues remain --> remediate, then re-audit
       |       (loop up to 10 iterations)
       |
       v
  Git checkpoint ("build-audit")
```

The build audit runs **after** the build summary is generated and **before** the lock is released. Any code changes made during remediation are committed in a final git checkpoint.

## Single-Agent Iterative Design

Unlike the sprint audit (which uses separate audit and fix agents), the build audit uses a **single agent session** that handles the full cycle:

1. **Audit** -- read the entire codebase and evaluate against six criteria
2. **Classify** -- assign severity to each finding
3. **Report** -- write findings to `audit.md` in the project root
4. **Evaluate** -- if all issues are LOW or none exist, stop
5. **Remediate** -- fix all issues (including LOW), then re-audit

The agent repeats this cycle up to 10 iterations. This design gives the agent full context across audit and fix passes, enabling more coherent remediation of cross-cutting issues.

## When It Runs

The build audit runs only when **all** of these conditions are met:

- All sprints completed successfully (no failures)
- The full epic was executed (sprint 1 through the last sprint)
- Audit is enabled (`@audit_after_sprint` or default; not `@no_audit`)
- `--no-audit` flag was not passed
- Effort level is not `low`

Partial sprint ranges (e.g., `fry run epic.md 3 5`) do **not** trigger the build audit, since the full epic has not been executed.

## Audit Criteria

The build audit evaluates the entire codebase against six criteria:

1. **Correctness** -- Code is coherent with the aim and function of the application; no bugs.
2. **Usability** -- No UX friction, confusing flows, or accessibility gaps.
3. **Edge Cases** -- Boundary conditions, empty states, invalid input, and race conditions are handled.
4. **Security** -- No vulnerabilities (injection, auth flaws, data exposure, etc.).
5. **Performance** -- No bottlenecks, memory leaks, or unnecessary complexity.
6. **Code Quality** -- Clean style, consistent patterns, clear naming, appropriate abstractions.

## Severity Classification

| Level | Definition |
|---|---|
| CRITICAL | Data loss, security breach, or crash under normal use |
| HIGH | Significant bug or vulnerability; affects core functionality |
| MODERATE | Noticeable issue; degraded experience or maintainability risk |
| LOW | Minor style, naming, or cosmetic concern |

## Context Provided to the Audit Agent

The build audit prompt includes selected artifacts as inline context:

| Context | Source | Limit |
|---|---|---|
| Executive summary | `plans/executive.md` | First 5,000 characters |
| Implementation plan | `plans/plan.md` | First 50KB |
| Epic definition | `.fry/epic.md` | First 50KB |
| Original user prompt | `.fry/user-prompt.txt` | First 2,000 characters |
| Sprint results | In-memory results table | Full content |

Artifacts deliberately excluded from the prompt:

| Artifact | Reason |
|---|---|
| Build logs (`.fry/build-logs/`) | Noisy; agent reads the codebase directly |
| Epic progress (`.fry/epic-progress.txt`) | Redundant with the epic definition and codebase |
| Build summary (`build-summary.md`) | Just generated; agent can read it from disk if needed |
| Deviation log (`.fry/deviation-log.md`) | Agent can read it from disk if needed |

The agent reads the actual codebase directly during its audit passes, so it has full access to all source files.

## Configuration

The build audit shares configuration with the sprint audit. The same directives and flags control both:

| Directive / Flag | Effect on build audit |
|---|---|
| `@no_audit` | Disables both sprint and build audits |
| `--no-audit` | Disables both sprint and build audits for this run |
| `@audit_engine` | Engine used for the build audit agent |
| `@audit_model` | Model used for the build audit agent |

No additional directives are needed -- the build audit runs automatically when the epic completes successfully with auditing enabled.

## Output

The build audit agent writes its report to `audit.md` in the project root. This file persists after the build and is committed in the git checkpoint.

The report includes:
- Location, description, severity, and recommended fix for each finding
- A verdict indicating whether the codebase passed or issues remain
- If the agent exhausted 10 iterations, an explanation of why issues persist

## Terminal Output

```
[2026-03-10 13:00:00] > BUILD AUDIT  running final holistic audit for "My Project"
[2026-03-10 13:15:00]   BUILD AUDIT: report written to audit.md
```

If the agent fails to produce the report:

```
[2026-03-10 13:15:00]   BUILD AUDIT: WARNING -- agent did not produce audit.md
```

## Build Logs

The build audit session is logged to `.fry/build-logs/`:

```
build_audit_20060102_150405.log
```

## Effort Level Interaction

- **`low`** -- Build audit is skipped entirely, matching sprint audit behavior.
- **`medium`**, **`high`**, **`max`** -- Build audit runs when the epic completes successfully.

## Relationship to Sprint Audit

| Aspect | Sprint Audit | Build Audit |
|---|---|---|
| Scope | Single sprint's changes | Entire codebase |
| Timing | After each sprint passes verification | After all sprints complete |
| Agent design | Two agents (audit + fix) | Single agent (audit + fix in one session) |
| Iterations | Up to `@max_audit_iterations` (default: 3) | Up to 10 |
| Blocking | CRITICAL/HIGH block the sprint | Non-blocking (advisory) |
| Output file | `.fry/sprint-audit.txt` (transient) | `audit.md` (persisted) |
| Context | Sprint diff + sprint progress | Full codebase + plan artifacts |

Both audits use the same six criteria and four severity levels. The sprint audit catches issues incrementally during the build; the build audit catches cross-cutting issues that only become visible when viewing the completed project as a whole.
