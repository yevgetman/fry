# Plan: Fry Team Runtime Modular Rollout

> **Status:** Milestone A complete in feature branch `feature/team-runtime-milestone-a` (implementation commit `cf193dc`, staged-plan update commit `f5e6746`); explicit `fry run` integration and triage auto-selection remain pending
> **Date:** 2026-04-01
> **Scope:** Build an OMX-style tmux-backed team runtime for Fry as a standalone subsystem first, then integrate it into the main Fry CLI without weakening Fry's current sanity-check, alignment, audit, and review spine
> **Recommended rollout:** working `fry team` product first, then explicit `fry run --execution-mode parallel`, then triage-driven auto-selection

---

## Problem Statement

Fry currently executes one agent loop per sprint. That keeps runtime semantics clear and preserves strong acceptance control, but it limits throughput on large, decomposable tasks. OMX demonstrates a stronger execution topology with tmux workers, task state, worker roles, scaling, and worktrees. Fry needs that execution breadth.

Fry's main advantage, however, is not topology. It is the host-enforced quality spine:

- machine-executable sanity checks
- targeted alignment after failed checks
- two-level sprint audit
- final build audit
- optional sprint review / replanning

The goal is therefore not to replace Fry's current model with an OMX-style worker prompt loop. The goal is to add a parallel runtime around Fry's quality spine.

This plan is intentionally phased and modular. The first deliverable is a complete, working `fry team` subsystem. Only after that subsystem is stable should `fry run` adopt it.

---

## Product Goals

### Primary Goals

1. Ship a durable `fry team` runtime for tmux-backed parallel workers.
2. Support durable task state, worker state, heartbeats, and mailbox-style coordination.
3. Support worker roles such as `executor`, `tester`, `integrator`, `reviewer`, and `researcher`.
4. Support isolated worktrees per worker for conflict reduction and resumability.
5. Support manual worker scaling and recovery.
6. Integrate team state into `fry status`, `fry monitor`, and `fry events`.
7. Preserve Fry's existing sprint-level acceptance spine after team execution is integrated into `fry run`.

### Secondary Goals

1. Make the team runtime reusable by self-improvement workflows.
2. Support both Codex and Claude workers through Fry's existing engine abstraction.
3. Create a future path to auto-selected parallel execution from `fry run`.
4. Prepare for future non-tmux or native-subagent execution backends.

### Non-Goals

1. Replacing the sequential sprint runner in the first implementation.
2. Letting workers independently decide sprint success.
3. Shipping automatic triage-selected parallel execution before the runtime is proven.
4. Building a distributed or remote execution platform in v1.

---

## Locked Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Rollout strategy | working `fry team` first, then explicit `fry run --execution-mode parallel`, then triage auto-selection | Isolates runtime risk from the stable sequential path |
| Acceptance authority | Existing Fry quality spine remains authoritative | Keeps Fry's main value intact |
| Runtime substrate | tmux panes as long-lived worker hosts | Matches OMX's durable topology while remaining local and debuggable |
| Control plane | State-file driven, not ad-hoc pane typing | Better recovery, status, and correctness |
| Runtime home | `.fry/team/<team-id>/...` | Integrates with existing `.fry` artifact model |
| Git isolation | Default to per-worker worktrees for serious tasks | Reduces conflicts and improves resumability |
| Worker roles | Start with `executor`, `tester`, `integrator`, `reviewer`, `researcher` | Small useful initial roster |
| Planner | Conservative, lane-based decomposition | Avoid over-ambitious decomposition early |
| Monitor integration | Extend existing `status`, `monitor`, and `events` | Reuse proven operator surfaces |
| Resume model | Durable on-disk canonical state | Aligns with Fry's `--continue` and `--resume` philosophy |

---

## Delivery Strategy

The rollout is split into three milestones.

### Milestone A: Standalone `fry team`

This milestone ships a complete operator-facing subsystem with its own CLI:

- `fry team start`
- `fry team status`
- `fry team attach`
- `fry team pause`
- `fry team resume`
- `fry team scale`
- `fry team shutdown`

At the end of Milestone A, Fry should have a real tmux-backed team runtime with durable state, task orchestration, worker roles, worktrees, scaling, resume, and monitoring. The main `fry run` path must still default to and depend on the existing sequential sprint runner.

### Milestone B: Explicit `fry run` integration

Only after Milestone A is stable should Fry add:

- `fry run --execution-mode parallel`
- epic directives for explicit parallel execution
- sprint-runner handoff into the team runtime

This milestone is integration work, not runtime invention.

### Milestone C: Triage-driven auto-selection

Only after Milestones A and B are proven should Fry allow automatic topology selection from triage.

### Hard delivery rule

Do not mix Milestone A runtime construction with Milestone B `fry run` integration in the same implementation slice. If the team runtime is not independently operable and testable, it is not ready to become part of the main execution path.

---

## Implementation Update

### 2026-04-01: Milestone A completed

Milestone A has now been implemented in the standalone worktree branch `feature/team-runtime-milestone-a`. The runtime implementation landed at commit `cf193dc`, and this staged-plan completion note was recorded at commit `f5e6746`.

Delivered scope:

- standalone `fry team` command surface
- durable `.fry/team/<team-id>/...` state model
- tmux-backed worker hosts and hidden worker loop
- task assignment, worker heartbeats, liveness recovery, and scaling
- shared and per-worker-worktree execution modes
- integrated-output fan-in for worktree mode
- shared `status`, `monitor`, and `events` visibility
- documentation and test coverage for the standalone runtime

Verification completed for Milestone A:

- `make test`
- `make build`
- focused runtime tests for shared-mode and worktree-mode execution
- live smoke test of `fry team start` -> task execution -> `fry team status` -> `fry team shutdown`

Remaining planned work is unchanged:

- Milestone B: explicit `fry run --execution-mode parallel` integration
- Milestone C: triage-driven topology auto-selection

---

## Existing Fry Strengths to Reuse

The implementation should explicitly build on these systems:

- `internal/git/strategy.go` for branch/worktree allocation and reuse
- `internal/monitor/` for snapshot rendering and polling
- `internal/steering/` for file-based IPC conventions
- `internal/agent/` for event surfaces and agent prompt/export logic
- `internal/observer/` for structured build events and wake points
- `internal/continuerun/` for resume and canonical-state recovery
- `internal/lock/` for concurrency and ownership primitives
- `internal/sprint/runner.go` for the current sequential sprint acceptance flow
- `internal/heal/`, `internal/audit/`, and `internal/review/` for the quality spine

Do not fork around these systems if reuse is possible.

---

## Architecture Split

### Layer A: Team Runtime

Responsible for:

- worker lifecycle
- task planning and assignment
- pane/process hosting
- worktree allocation
- scaling
- state persistence
- leader/worker messaging
- liveness and recovery
- worker output fan-in

### Layer B: Fry Quality Spine

Responsible for:

- sanity checks
- alignment
- sprint audit
- build audit
- sprint review
- summary
- archive
- final pass/fail decision

Parallel runtime improves execution throughput only. Final acceptance stays centralized.

---

## Modular Build Map

Build the runtime as modules with explicit boundaries. Each module should be testable in isolation before the next module depends on it.

| Module | Responsibility | Milestone |
|---|---|---|
| `team/contracts` | structs, enums, paths, artifact layout | A |
| `team/state` | atomic reads/writes, locks, canonical state access | A |
| `team/tmux` | pane/session lifecycle, capture, send, inspect | A |
| `team/runtime` | start, stop, pause, resume, attach | A |
| `team/tasks` | task graph, assignment, transitions, dependencies | A |
| `team/workers` | worker identity, role, heartbeats, mailbox ownership | A |
| `team/liveness` | stale worker detection and requeue logic | A |
| `team/planner` | conservative lane-based decomposition | A |
| `team/git` | worker worktree setup and recovery | A |
| `team/integration` | merge/fan-in from workers into canonical output | A then B |
| `team/scaling` | add/remove/drain/rebalance workers | A |
| `team/cli` | `fry team` subcommands and operator UX | A |
| `run/backend` | explicit handoff from sprint runner to team executor | B |
| `triage/topology` | auto-select sequential vs parallel execution | C |

### Dependency order

1. contracts
2. state
3. tmux
4. runtime
5. tasks
6. workers
7. liveness
8. planner
9. git isolation
10. integration
11. scaling
12. `fry team` CLI completion
13. `fry run` backend integration
14. triage auto-selection

### Gating rule

No `fry run` integration work should begin until modules 1 through 12 are working together as a standalone `fry team` product slice.

---

## Proposed Package Layout

Create a new package family:

```text
internal/team/
  types.go
  paths.go
  state.go
  runtime.go
  planner.go
  tasks.go
  workers.go
  tmux.go
  dispatch.go
  scaling.go
  liveness.go
  integration.go
  status.go
```

### Responsibilities

- `types.go`
  Shared enums and JSON contracts

- `paths.go`
  Team-specific `.fry/team/...` path helpers

- `state.go`
  Atomic read/write helpers and state locks

- `runtime.go`
  Team start, attach, pause, resume, shutdown orchestration

- `planner.go`
  Convert sprint intent into parallel tasks and worker lanes

- `tasks.go`
  Task CRUD, dependency logic, transitions

- `workers.go`
  Worker identity, status, heartbeat, and assignment ownership

- `tmux.go`
  Pane lifecycle, attach/capture/send primitives

- `dispatch.go`
  Mailbox/inbox writes and worker notification flow

- `scaling.go`
  Add/remove/drain workers and rebalance tasks

- `liveness.go`
  Detect stalled/dead panes and stale heartbeats

- `integration.go`
  Merge worker output into a canonical integrated output directory and later hand back to Fry's quality spine

- `status.go`
  Build a machine-readable team snapshot for CLI and monitor use

---

## Runtime Artifacts

Create a durable runtime tree under `.fry/team/<team-id>/`:

```text
.fry/team/<team-id>/
  config.json
  manifest.json
  events.jsonl
  tasks/
    task-001.json
    task-002.json
  workers/
    worker-1/
      identity.json
      heartbeat.json
      status.json
      mailbox.json
      inbox.json
    worker-2/
      ...
  leader/
    status.json
    mailbox.json
  locks/
    scaling.lock
    assignment.lock
  artifacts/
    plan-summary.md
    merge-report.md
    integrated-output/
```

### Core Contracts

#### `config.json`

```json
{
  "team_id": "sprint-2-executor-20260401T210500Z",
  "project_dir": "/abs/project",
  "build_dir": "/abs/project-or-worktree",
  "build_id": "current-build-id",
  "sprint_number": 2,
  "mode": "software",
  "engine": "codex",
  "status": "running",
  "tmux_session": "fry-team-123",
  "leader_pane_id": "%1",
  "worker_count": 3,
  "max_workers": 8,
  "git_isolation_mode": "per-worker-worktree",
  "created_at": "2026-04-01T21:05:00Z"
}
```

#### `task-001.json`

```json
{
  "id": "001",
  "title": "Implement auth middleware changes",
  "description": "Update middleware and request context flow for sprint 2",
  "role": "executor",
  "status": "pending",
  "owner": "",
  "priority": 10,
  "blocked_by": [],
  "acceptance_hints": [
    "middleware compiles",
    "request context tests pass"
  ],
  "created_at": "2026-04-01T21:05:20Z",
  "updated_at": "2026-04-01T21:05:20Z"
}
```

#### `identity.json`

```json
{
  "worker_id": "worker-2",
  "role": "executor",
  "engine": "codex",
  "model": "gpt-5.4",
  "reasoning_effort": "high",
  "pane_id": "%7",
  "work_dir": "/abs/project/.fry-worktrees/auth-worker",
  "worktree_branch": "fry/sprint-2-auth-worker",
  "status": "running"
}
```

#### `heartbeat.json`

```json
{
  "worker_id": "worker-2",
  "status": "running",
  "current_task": "001",
  "last_seen_at": "2026-04-01T21:08:12Z",
  "iteration": 4,
  "message": "running tests after middleware changes"
}
```

---

## Event Model

Extend Fry's event system with team events.

### New event types

- `team_start`
- `team_worker_ready`
- `team_task_created`
- `team_task_assigned`
- `team_task_started`
- `team_task_completed`
- `team_task_failed`
- `team_scale_up`
- `team_scale_down`
- `team_worker_stalled`
- `team_pause`
- `team_resume`
- `team_shutdown`
- `team_merge_ready`

### Requirements

1. Events must be readable through `fry events`.
2. Team events should appear in `fry monitor` when a team runtime is active.
3. Event payloads must include `team_id`, plus worker/task IDs when relevant.
4. Event emission should be cheap and durable, following current observer/event conventions.

---

## Worker Roles

Start with these roles:

- `executor`
- `tester`
- `integrator`
- `reviewer`
- `researcher`

### V1 semantics

- `executor`
  Implements assigned code changes

- `tester`
  Adds tests, fixtures, repros, and validation notes

- `integrator`
  Consolidates worker changes and resolves merge handoff issues

- `reviewer`
  Performs bounded read-only review or evidence collection

- `researcher`
  Performs codebase mapping, dependency inspection, and repo lookups

### Constraint

These runtime roles do not replace Fry's existing prompt or session semantics. They are execution lanes, not a new language of acceptance.

---

## Git Isolation Model

Reuse and extend the existing worktree system in `internal/git/strategy.go`.

### Supported team isolation modes

- `shared`
  all workers use the same build directory

- `per-worker-branch`
  workers share a workdir but commit or stage via separate branches

- `per-worker-worktree`
  each worker gets its own worktree and branch

### Default

Default to `per-worker-worktree` for:

- 3+ workers
- cross-cutting code changes
- high/max effort runs
- non-trivial sprint decomposition

### Requirements

1. Do not duplicate worktree allocation logic unnecessarily.
2. Extract reusable git/worktree helpers where needed.
3. Persist worker worktree metadata in team state.
4. Team resume must be able to reattach or recreate worktrees safely.

---

## User Experience and CLI

### Milestone A command surface

```bash
fry team start --workers 3 --role executor --sprint 2
fry team status --team <team-id>
fry team scale --team <team-id> --add 2
fry team scale --team <team-id> --remove worker-3
fry team assign --team <team-id> --file tasks.json
fry team pause --team <team-id>
fry team resume --team <team-id>
fry team shutdown --team <team-id>
fry team attach --team <team-id>
```

### Milestone B convenience forms

```bash
fry run --execution-mode parallel --parallel-workers 4
```

### Milestone B epic directives

```text
@execution_mode parallel
@parallel_workers 4
@parallel_roles executor,tester,integrator
```

### Milestone C

`fry run` triage chooses sequential vs parallel execution automatically.

---

## Team Planner

The planner should begin conservative.

### Inputs

- current sprint prompt
- project state
- `.fry/codebase.md` if available
- epic progress
- user prompt and directives
- effort level

### Outputs

- task list
- task dependencies
- suggested worker lanes
- initial worker count

### V1 decomposition strategy

Prefer lane-oriented decomposition:

1. implementation lane A
2. implementation lane B
3. regression/evidence lane
4. integration lane

Avoid sophisticated decomposition heuristics until the runtime is proven.

---

## Monitoring and Operator UX

Integrate with current Fry operator surfaces.

### `fry status`

Add a team summary when a team runtime is active:

- `team_id`
- worker counts
- idle/running/stalled workers
- pending/in-progress/completed tasks

### `fry monitor`

Add a team-aware view:

- per-worker status
- task board summary
- scaling events
- heartbeat freshness
- integration state

### `fry events`

Expose team lifecycle events in the same stream as build events.

### Future

Optional dedicated dashboard:

```bash
fry team dashboard --team <team-id>
```

Not required for initial delivery.

---

## Resume and Recovery

Team runtime must be durable from the beginning.

### Required resume behavior

On `fry team resume`:

1. load team config
2. inspect tmux session and pane liveness
3. inspect worker heartbeats
4. inspect unfinished tasks
5. mark dead workers as failed or lost
6. requeue safe tasks
7. optionally recreate workers

### Failure classes to support

- pane dead
- worker process hung
- missing worktree
- stale heartbeat
- leader process died but tmux still exists
- partial or corrupted team state

These should be explicit runtime cases with tests.

---

## Implementation Phases

## Milestone A: Standalone `fry team`

### Phase 0: Design Hardening and Contracts

**Goal:** Lock state, event, and runtime contracts before writing the full runtime.

**Files to create:**

- `internal/team/types.go`
- `internal/team/paths.go`

**Deliverables:**

1. Team config, task, worker identity, heartbeat, and event structs
2. Path helpers for `.fry/team/...`
3. Team status enums:
   - `starting`
   - `running`
   - `paused`
   - `draining`
   - `failed`
   - `complete`
   - `shutdown`
4. Task status enums:
   - `pending`
   - `assigned`
   - `in_progress`
   - `completed`
   - `failed`
   - `blocked`
5. Worker status enums:
   - `starting`
   - `idle`
   - `running`
   - `draining`
   - `stalled`
   - `dead`

**Acceptance criteria:**

1. JSON contracts are defined and documented in code comments.
2. Path helpers are deterministic and covered by tests.
3. No runtime behavior yet.

**Tests:**

- serialization tests
- path helper tests
- enum/state transition helper tests

**Mandatory checks:** `make test && make build`

### Phase 1: Team State and tmux Foundation

**Goal:** Start a team runtime, create durable state, create panes, and inspect status.

**Files to create:**

- `internal/team/state.go`
- `internal/team/tmux.go`
- `internal/team/runtime.go`
- `internal/team/status.go`

**Files to modify:**

- `internal/cli/team.go`
- `internal/cli/root.go`
- event-related surfaces as needed

**Capabilities:**

1. `fry team start`
2. create `.fry/team/<team-id>/config.json`
3. create worker identities
4. create tmux panes
5. create initial team events
6. `fry team status`
7. `fry team shutdown`

**Acceptance criteria:**

1. Team start creates durable state and panes.
2. Team status returns a machine-readable snapshot.
3. Shutdown tears down panes and marks team state terminal.
4. `fry events` can surface team events.

**Tests:**

- team state writes are atomic
- status assembly with partial state
- tmux wrapper command-building tests
- simulated start/shutdown integration tests

**Mandatory checks:** `make test && make build`

### Phase 2: Task Engine and Worker Loop

**Goal:** Add durable tasks, worker polling, and task claim/complete/fail flow.

**Files to create:**

- `internal/team/tasks.go`
- `internal/team/workers.go`
- `internal/team/dispatch.go`
- `internal/team/liveness.go`

**Capabilities:**

1. create task files
2. assign tasks to workers
3. worker loop reads mailbox and task state
4. worker heartbeats update durably
5. workers can mark tasks completed or failed
6. stalled/dead workers are detectable

**Acceptance criteria:**

1. A worker can claim one eligible task at a time.
2. Task transitions are durable and validated.
3. Heartbeats and liveness state update correctly.
4. Dead workers do not leave tasks in ambiguous ownership.

**Tests:**

- task dependency resolution
- task claim race tests
- heartbeat staleness tests
- dead-worker task requeue tests

**Mandatory checks:** `make test && make build`

### Phase 3: Worker Roles and Team Planner

**Goal:** Decompose work into conservative parallel lanes and assign role-aware tasks.

**Files to create:**

- `internal/team/planner.go`

**Capabilities:**

1. convert team intent into an initial task graph
2. suggest worker roles and worker counts
3. support a conservative lane-based decomposition
4. generate a persisted planning artifact under `.fry/team/<team-id>/artifacts/`

**Acceptance criteria:**

1. Planner produces deterministic output shape.
2. Planner can be overridden with explicit task file input.
3. Planner favors conservative decomposition and task independence.

**Tests:**

- planner output shape tests
- role-lane mapping tests
- override behavior tests

**Mandatory checks:** `make test && make build`

### Phase 4: Worktree-Aware Worker Execution

**Goal:** Give workers isolated worktrees and reusable git metadata.

**Files to create or modify:**

- `internal/team/runtime.go`
- `internal/team/workers.go`
- `internal/team/integration.go`
- reused or extracted helpers from `internal/git/strategy.go`

**Capabilities:**

1. allocate per-worker worktrees
2. persist worktree path and branch metadata
3. recover or recreate worktrees on resume
4. keep worker output attributable

**Acceptance criteria:**

1. Workers can run in isolated worktrees.
2. Team status exposes worker worktree metadata.
3. Resume can reattach or safely recreate worktrees.

**Tests:**

- worktree allocation tests
- resume after existing worktree tests
- missing worktree recovery tests

**Mandatory checks:** `make test && make build`

### Phase 5: Standalone Fan-In and Integration

**Goal:** Make `fry team` capable of completing a full parallel work cycle and producing one canonical integrated output directory, without yet changing the main `fry run` path.

**Files to create or modify:**

- `internal/team/integration.go`
- `internal/team/runtime.go`
- `internal/team/status.go`
- `internal/cli/team.go`

**Capabilities:**

1. team runtime produces a merged canonical team output directory
2. integrator role can consolidate worker output into that canonical output
3. merge and conflict artifacts are persisted for operator review
4. `fry team status` reflects integration state and merge readiness
5. the subsystem is now complete enough to be used independently of `fry run`

**Acceptance criteria:**

1. `fry team` can execute tasks, integrate worker output, and stop in a coherent final state.
2. Canonical output location and merge artifacts are durable and inspectable.
3. Integration failures are visible without involving the sequential sprint runner.

**Tests:**

- multi-worker integration test with canonical output
- merge artifact generation test
- integrator-state/status reporting test

**Mandatory checks:** `make test && make build`

### Phase 6: Scaling and Recovery Completion

**Goal:** Add worker scale-up, scale-down, drain behavior, and finish operational recovery semantics.

**Files to create:**

- `internal/team/scaling.go`

**Files to modify:**

- `internal/team/runtime.go`
- `internal/team/liveness.go`
- `internal/team/status.go`
- `internal/cli/team.go`

**Capabilities:**

1. `fry team scale --add N`
2. `fry team scale --remove worker-X`
3. drain workers before shutdown/removal
4. rebalance assignable tasks
5. recover from stalled workers and dead panes

**Acceptance criteria:**

1. Scale-up creates new workers and state cleanly.
2. Scale-down does not kill active work abruptly.
3. Draining workers stop receiving new tasks.
4. Rebalancing is deterministic and observable.
5. Resume and recovery are operator-usable, not just nominally implemented.

**Tests:**

- scale-up tests
- drain/removal tests
- rebalancing tests
- scale-down while active task is running
- resume after pane death
- resume after stale heartbeat

**Mandatory checks:** `make test && make build`

### Milestone A exit criteria

Milestone A is complete only when all of the following are true:

Status on 2026-04-01: complete in feature branch `feature/team-runtime-milestone-a` (implementation `cf193dc`, staged-plan note `f5e6746`).

1. `fry team` can start, attach, pause, resume, scale, and shut down cleanly.
2. workers can claim tasks, heartbeat, fail, recover, and requeue safely.
3. workers can run in isolated worktrees with durable metadata.
4. the team runtime can integrate worker output into one canonical result directory.
5. state and events are visible through `fry team status` and shared Fry monitoring surfaces.
6. the standalone runtime can be exercised without depending on `fry run`.

Only after this milestone should the main Fry execution path be modified.

---

## Milestone B: Main CLI Integration

### Phase 7: Explicit `fry run` Parallel Integration

**Goal:** Allow normal Fry runs to use the team runtime explicitly.

**Files to modify:**

- `internal/cli/root.go`
- `internal/cli/run.go`
- `internal/epic/types.go`
- `internal/epic/parser.go`
- `internal/sprint/runner.go`
- docs and templates

**New flags/directives:**

- `--execution-mode sequential|parallel`
- `--parallel-workers N`
- `@execution_mode parallel`
- `@parallel_workers N`
- `@parallel_roles ...`

**Capabilities:**

1. `fry run --execution-mode parallel`
2. selected sprints can use the team runtime
3. explicit, user-controlled parallelism
4. team output feeds into the existing quality spine with no acceptance bypass

**Acceptance criteria:**

1. Sequential behavior remains default and unchanged.
2. Parallel mode is explicit and stable.
3. Build reporting shows sequential vs parallel execution mode clearly.
4. `fry run` and `fry team` share the same team runtime codepath rather than separate implementations.

**Tests:**

- CLI flag parsing
- epic directive parsing
- end-to-end parallel sprint run
- team runtime handoff into sanity/alignment/audit/review
- regression test proving sequential `fry run` behavior remains unchanged

**Mandatory checks:** `make test && make build`

---

## Milestone C: Triage Optimization

### Phase 8: Triage-Driven Auto-Selection

**Goal:** Let `fry run` choose team mode automatically when the task justifies it.

**Files to modify:**

- `internal/triage/`
- `internal/cli/run.go`
- docs

**Capabilities:**

1. triage recommends execution topology
2. complex decomposable tasks can default to parallel mode
3. user can override

**Acceptance criteria:**

1. Auto-selection never removes the ability to force sequential mode.
2. Triage output clearly surfaces topology choice.
3. Topology choice becomes part of persisted build state for resume.

**Tests:**

- triage topology-selection tests
- override precedence tests
- continue/resume with persisted topology tests

**Mandatory checks:** `make test && make build`

---

## Integration Into Existing Fry Runtime

This is the most important implementation rule.

### Current sequential flow

Today, `internal/sprint/runner.go` does:

1. assemble prompt
2. run one agent loop
3. run sanity checks
4. run alignment if needed
5. run sprint audit
6. git checkpoint
7. optional sprint review

### Future parallel flow

After Milestones A and B, only step 2 changes:

1. assemble prompt
2. team planner decomposes work into tasks
3. team runtime launches workers and runs tasks
4. integrator consolidates worker output into the canonical sprint build dir
5. run the same sanity checks
6. run the same alignment flow if checks fail
7. run the same sprint audit
8. git checkpoint
9. optional sprint review

### Hard rule

Workers can produce code and evidence, but they do not decide sprint success.

The existing Fry quality spine remains the pass/fail authority.

---

## Suggested Refactor Boundary

After Phase 7 is stable, introduce or formalize an execution backend interface if needed:

```go
type SprintExecutor interface {
    Run(ctx context.Context, cfg RunConfig) (*SprintResult, error)
}
```

Implementations:

- `SequentialSprintExecutor`
- `TeamSprintExecutor`

Do not force this refactor before the standalone team runtime works and explicit `fry run` integration is proven.

---

## Testing Strategy

Parallel runtime is a high-risk subsystem. Testing must be heavy.

### Unit tests

- task state transitions
- worker state transitions
- event payload correctness
- path construction
- planner output shape
- scaling decisions
- liveness detection

### Integration tests

- team start/status/shutdown
- fake worker loop with durable tasks
- worker crash and task requeue
- per-worker worktree allocation
- standalone multi-worker fan-in
- team-to-sanity-check handoff
- team-to-audit handoff

### Failure-mode tests

- dead pane
- dead worker with owned task
- stale heartbeat
- missing worktree on resume
- partial/corrupt team state
- scale-down during active execution

### Important test principle

Do not require real Codex or Claude sessions for early runtime tests.

Use:

- fake worker processes
- simulated task runners
- mocked tmux wrappers where possible

Add real smoke tests only after the state machine is stable.

---

## Documentation Work Required

Update these docs as the build lands:

- `README.md`
- `README.LLM.md`
- `docs/architecture.md`
- `docs/commands.md`
- `docs/project-structure.md`
- `docs/monitor.md`
- `docs/git-strategy.md`
- `docs/observer.md`

If epic directives are added:

- `docs/epic-format.md`
- `templates/epic-example.md`
- `templates/GENERATE_EPIC.md`

If agent/runtime prompt contracts are changed:

- `AGENTS.md`
- `openclaw-skill/SKILL.md`
- `internal/agent/prompt.go`

---

## Branch and Delivery Strategy

Recommended branch sequence:

1. `feature/team-runtime-contracts`
2. `feature/team-runtime-foundation`
3. `feature/team-runtime-tasks-workers`
4. `feature/team-runtime-planner`
5. `feature/team-runtime-worktrees`
6. `feature/team-runtime-fan-in`
7. `feature/team-runtime-scaling`
8. `feature/team-runtime-cli-finish`
9. `feature/team-runtime-run-integration`
10. `feature/team-runtime-triage-selection`

Keep each branch or PR narrow and reviewable.

Do not combine Milestone A runtime work with Milestone B `fry run` integration in one branch.

---

## Success Criteria

The project is successful when all of the following are true:

1. `fry team start` can launch a durable tmux-backed worker runtime.
2. Team state is visible via `fry status`, `fry monitor`, and `fry events`.
3. Workers can run in isolated worktrees.
4. Team runtime can resume safely after interruption.
5. `fry team` is a usable standalone runtime before `fry run` depends on it.
6. A parallel sprint still passes through Fry's existing sanity-check, alignment, audit, and review sequence once integrated.
7. Sequential `fry run` remains stable and unchanged by default.
8. Parallel mode can later be made triage-selectable without architectural rework.

---

## Bottom Line

This should be built in phases and modules, with the full architecture designed up front.

The correct implementation order is:

1. contracts
2. runtime foundation
3. task engine
4. planner
5. worktree integration
6. standalone fan-in
7. scaling and recovery completion
8. complete `fry team`
9. explicit `fry run` integration
10. triage auto-selection

That order minimizes runtime risk, keeps Fry's current strengths intact, gives Fry a real standalone team subsystem early, and creates a path to OMX-style parallel execution without sacrificing Fry's stronger quality guarantees.
