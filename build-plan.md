# Fry v0.1 — Build Plan

> Companion to `product-spec.md`. Turns the spec's commitments into a concrete, ordered, verifiable sequence of work. A competent Go engineer (or fry itself when dogfooded) should be able to execute from this document end-to-end.

---

## 0. Pre-flight (before M1)

Must be true before coding begins:

- [ ] `go version` → 1.22 or newer (we'll use `log/slog`, generics, recent stdlib)
- [ ] `claude --version` → 2.x; verified to work headless with `claude -p`
- [ ] macOS Apple Silicon or Intel (primary target for v0.1)
- [ ] `gh auth status` → authenticated (for pushing the repo)
- [ ] `~/code/fry/` exists and is empty except for `product-spec.md`, `build-plan.md`
- [ ] `~/code/fry-legacy/` preserved read-only for reference

Initial setup (one-time, ~10 min):

```bash
cd ~/code/fry
go mod init github.com/yevgetman/fry
gh repo create yevgetman/fry --private --source . --remote origin
git init && git add . && git commit -m "Initial commit: product spec + build plan"
git push -u origin main
```

---

## 1. Scope recap (from spec §15)

v0.1 ships when **all** of the following are true:

1. Commands functional: `fry new`, `fry start`, `fry stop`, `fry chat`, `fry wake`, `fry status`, `fry logs`, `fry list`.
2. Dogfood mission completes: `fry new demo --prompt sample.md --duration 30m --interval 5m --effort fast && fry start demo` produces ≥5 wake entries, transitions to `complete` within 30m, no overlap-lock failures, `notes.md` meaningfully updated each wake.
3. `fry chat demo` produces a coherent summary in ≤1 turn.
4. README covering all commands + quickstart + file layout.
5. Non-goals list enforced: no optional feature flags, no modes, no plugin registry, no provider abstraction.
6. Core codebase ≤2000 LOC Go (excluding vendored packages).

Anything not in this list is post-v0.1.

---

## 2. Tech stack

| Concern | Choice | Reason |
|---|---|---|
| Language | Go 1.22+ | Binary distribution, stdlib strength, legacy-fry compatibility |
| CLI framework | `github.com/spf13/cobra` | Standard; already in legacy-fry |
| Config | None (flags + env) | YAGNI — no config file in v0.1 |
| Logging | `log/slog` (stdlib) | Zero deps, structured, handler-swappable |
| JSON | `encoding/json` | Stdlib, sufficient for our schemas |
| File locking | `os.Mkdir` (atomic) | No deps, works on macOS without extra binaries |
| Process management | `os/exec` | Stdlib |
| Scheduler (macOS) | LaunchAgent via `launchctl` subprocess | No CGO, no osxfuse, just shell-out |
| Testing | `testing` (stdlib) + `testify/assert` | Minimal |
| Lint | `golangci-lint` with a short config | Standard |

Total external deps should be 2–3 (cobra, testify/assert, maybe `google/uuid` if needed). Every dep must be justified in `go.mod` comments.

---

## 3. Repo layout

```
~/code/fry/
├── product-spec.md           # the WHAT (already exists)
├── build-plan.md             # the HOW (this document)
├── README.md                 # user-facing quickstart (written in M7)
├── go.mod
├── go.sum
├── .gitignore
├── Makefile                  # build, test, install targets
├── cmd/
│   └── fry/
│       └── main.go           # cobra root + command wiring; <100 LOC
├── internal/
│   ├── mission/              # mission directory layout, scaffolding
│   │   ├── layout.go
│   │   ├── layout_test.go
│   │   └── templates/        # embedded text/template files
│   │       ├── notes.md.tmpl
│   │       ├── runner.sh.tmpl
│   │       └── launchagent.plist.tmpl
│   ├── state/                # state.json read/write + validation
│   │   ├── state.go
│   │   ├── state_test.go
│   │   └── transitions.go    # allowed status transitions
│   ├── scheduler/            # scheduler abstraction + backends
│   │   ├── scheduler.go      # interface
│   │   ├── darwin.go         # build tag: darwin; LaunchAgent impl
│   │   ├── linux.go          # build tag: linux; returns ErrUnsupported for v0.1
│   │   └── scheduler_test.go
│   ├── wake/                 # wake execution
│   │   ├── wake.go           # orchestration: lock → prompt → claude → parse → update
│   │   ├── prompt.go         # layered prompt assembly (vendored-adapted from legacy-fry)
│   │   ├── claude.go         # subprocess to claude CLI
│   │   ├── promise.go        # promise-token parsing
│   │   ├── noop.go           # no-op detection
│   │   ├── lock.go           # mkdir-based overlap lock
│   │   └── wake_test.go
│   ├── notes/                # notes.md read/write (section-aware)
│   │   ├── notes.go
│   │   └── notes_test.go
│   ├── wakelog/              # wake_log.jsonl append + read
│   │   ├── wakelog.go
│   │   └── wakelog_test.go
│   ├── chat/                 # fry chat supervisor session
│   │   ├── chat.go
│   │   └── systemprompt.md   # embedded template for appended system prompt
│   └── clock/                # wall-clock helpers (for deterministic testing)
│       └── clock.go
└── vendor-fry/               # physical copies from legacy-fry (see §7)
    └── prompt-layers.go      # adapted from legacy-fry/internal/sprint/prompt*.go
```

**Invariants on layout:**
- No package in `internal/` may import another internal package cyclically.
- `cmd/fry/main.go` is the only place with flag wiring.
- All file templates live in `internal/mission/templates/` (embedded via `//go:embed`).
- No package depends on `internal/chat` except `cmd/fry`.

---

## 4. Milestones overview

| # | Milestone | Est. wakes¹ | Gates |
|---|---|---|---|
| M1 | Filesystem + state | 4–6 | `fry new` + `fry list` + `fry status` work against disk |
| M2 | Scheduler + macOS LaunchAgent | 4–6 | `fry start` + `fry stop` actually install/remove LaunchAgent |
| M3 | Wake: claude invocation + promise token | 5–7 | One real wake end-to-end with trivial prompt |
| M4 | Prompt assembly + notes.md round-trip | 3–5 | Cross-wake memory demonstrated |
| M5 | Chat session | 2–4 | `fry chat demo` → Claude answers "what's happening?" in 1 turn |
| M6 | Deadlines + no-op detection + shutdown | 3–5 | 30m dogfood mission auto-transitions to `complete` |
| M7 | Polish + README + ship | 2–4 | All spec §15 gates green |

¹ "Wakes" here is the implementation time if we dogfood fry to build itself. Roughly 10–12 min each. v0.1 total: ~23–37 wakes = 4–7 hours of focused build time plus polish/fix cycles.

---

## 5. Milestone detail

Each milestone has four sections: **Goal**, **Deliverables**, **Implementation sketch**, **Verification**.

### M1 — Filesystem + state

**Goal.** Scaffolding a mission directory and reading its state works. No scheduler, no wakes, no LLM.

**Deliverables.**
- `cmd/fry/main.go` with `fry new`, `fry list`, `fry status` subcommands wired via cobra.
- `internal/mission/layout.go`: `New(name, opts) (*Mission, error)` that creates the directory tree, copies input files, renders templates.
- `internal/state/state.go`: `Load`, `Save` (atomic via temp file + rename), `Mission` struct, JSON tags.
- `internal/state/transitions.go`: `CanTransition(from, to Status) bool` — only valid transitions allowed (see §9 below).
- Template files: `notes.md.tmpl`, `runner.sh.tmpl`, `launchagent.plist.tmpl`.
- Basic unit tests on state marshal/unmarshal and transition logic.

**Implementation sketch.**

```go
// internal/state/state.go

type Status string

const (
    StatusActive   Status = "active"
    StatusOvertime Status = "overtime"
    StatusComplete Status = "complete"
    StatusStopped  Status = "stopped"
    StatusFailed   Status = "failed"
)

type Mission struct {
    MissionID       string    `json:"mission_id"`
    CreatedAt       time.Time `json:"created_at"`
    PromptPath      string    `json:"prompt_path"`
    InputMode       string    `json:"input_mode"` // "prompt" | "plan" | "prompt+plan" | "spec-dir"
    Effort          string    `json:"effort"`     // "fast" | "standard" | "max"
    IntervalSeconds int       `json:"interval_seconds"`
    DurationHours   float64   `json:"duration_hours"`
    OvertimeHours   float64   `json:"overtime_hours"`
    CurrentWake     int       `json:"current_wake"`
    LastWakeAt      time.Time `json:"last_wake_at,omitempty"`
    Status          Status    `json:"status"`
    HardDeadlineUTC time.Time `json:"hard_deadline_utc"`
}

func Load(dir string) (*Mission, error) { ... }
func (m *Mission) Save(dir string) error { ... }  // atomic: temp + rename
```

```go
// internal/mission/layout.go

type NewOptions struct {
    Name        string
    PromptFile  string
    PlanFile    string
    SpecDir     string
    Effort      string
    Interval    time.Duration
    Duration    time.Duration
    Overtime    time.Duration
    BaseDir     string
}

func (o NewOptions) InputMode() string { ... }  // derive from which fields set

func Scaffold(o NewOptions) (missionDir string, err error) {
    // 1. Validate: exactly one of PromptFile/PlanFile/SpecDir
    // 2. mkdir ~/missions/<name>/{artifacts,lock}
    // 3. Copy input file(s) to prompt.md / plan.md or spec-dir bundle
    // 4. Render notes.md from template (empty sections)
    // 5. Render runner.sh, launchagent.plist with substituted paths
    // 6. Build state.Mission{}, write state.json
    // 7. Touch wake_log.jsonl, supervisor_log.jsonl
    // 8. Return missionDir
}
```

**Verification.**
- `fry new demo --prompt /tmp/p.md --duration 30m --interval 5m --effort fast` creates `~/missions/demo/` with all 9 expected files.
- `fry list` shows `demo` with `status=active wake=0`.
- `fry status demo` prints state fields + "not yet started".
- Unit tests pass: `go test ./internal/state/... ./internal/mission/...`.
- Invalid inputs fail loudly: two input flags → error; missing `--prompt` and `--plan` and `--spec-dir` → error; invalid effort → error.

**Exit gate before M2:** the scaffolding is inspect-able by a human and looks sane. `cat ~/missions/demo/state.json | jq` returns valid JSON. `cat ~/missions/demo/runner.sh` shows a substituted script.

---

### M2 — Scheduler + macOS LaunchAgent backend

**Goal.** `fry start` actually installs a LaunchAgent that, when it fires, runs `runner.sh` which calls `fry wake`. At this milestone, `fry wake` is a stub that just appends a canned log entry.

**Deliverables.**
- `internal/scheduler/scheduler.go`: `type Scheduler interface { Install(m) error; Uninstall(m) error; Status(m) (SchedulerStatus, error); Kickstart(m) error }`.
- `internal/scheduler/darwin.go` (build tag `//go:build darwin`): uses `launchctl load`, `launchctl unload`, `launchctl print`, `launchctl kickstart`. Subprocess `launchctl` via `os/exec`; parse stderr for errors.
- `internal/scheduler/linux.go` (build tag `//go:build linux`): stub returning `ErrUnsupported`.
- `NewScheduler() Scheduler` factory that picks the right backend at compile time.
- `cmd/fry/main.go` gains `fry start`, `fry stop` subcommands.
- `fry wake <name>` subcommand exists as a stub: acquires lock, appends canned wake_log entry, releases lock.
- `internal/wake/lock.go`: `Acquire(dir) (*Lock, error)` via `os.Mkdir`; returns `ErrLocked` if held; `Release()` via `os.Remove`.
- Integration test on darwin: install → kickstart → verify stub wake ran → uninstall.

**Implementation sketch.**

```go
// internal/scheduler/darwin.go

type darwinScheduler struct{}

func (s darwinScheduler) Install(m *state.Mission) error {
    plistPath := fmt.Sprintf("%s/Library/LaunchAgents/com.fry.%s.plist", os.Getenv("HOME"), m.MissionID)
    // plist was already rendered by mission.Scaffold at creation; just `launchctl load <path>`
    cmd := exec.Command("launchctl", "load", plistPath)
    out, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("launchctl load: %w: %s", err, out)
    }
    return nil
}
```

```go
// internal/wake/lock.go

type Lock struct {
    path string
}

func Acquire(missionDir string) (*Lock, error) {
    p := filepath.Join(missionDir, "lock")
    if err := os.Mkdir(p, 0o755); err != nil {
        if os.IsExist(err) {
            return nil, ErrLocked
        }
        return nil, err
    }
    return &Lock{path: p}, nil
}

func (l *Lock) Release() error {
    return os.Remove(l.path)
}
```

**Verification.**
- `fry start demo` → `launchctl list | grep demo` shows the agent loaded.
- After ~5 min (one interval), `cat ~/missions/demo/wake_log.jsonl | wc -l` shows ≥1 entry (from the stub).
- `fry stop demo` → `launchctl list | grep demo` empty.
- `fry start demo && fry wake demo` (manual kickstart of a second wake while first still running) → second invocation exits 0 with stderr "skipped — overlap" or equivalent, no log entry added.

**Exit gate before M3:** end-to-end schedule → fire → runner.sh → `fry wake` → log append → unlock works. Stub content is fine; real LLM comes next.

---

### M3 — Wake: claude invocation + promise token

**Goal.** `fry wake` actually calls `claude -p` with a simple prompt and parses the output for the promise token. A real wake runs end-to-end with a trivial prompt like "write hello to artifacts/out.txt and output ===WAKE_DONE===."

**Deliverables.**
- `internal/wake/claude.go`: `RunClaude(ctx, req) (*Result, error)`. Builds argv from effort level, feeds prompt via stdin, captures stdout/stderr to files under `~/missions/<name>/logs/`.
- `internal/wake/promise.go`: `ExtractPromise(stdout []byte) (found bool, beforeToken []byte)`. Searches for the last line containing `===WAKE_DONE===`. Robust to extra whitespace.
- `internal/wake/wake.go`: orchestration. Flow: acquire lock → assemble prompt (M4 detail, for now stub) → call `RunClaude` → check promise → append wake_log entry → update state → release lock.
- `internal/wakelog/wakelog.go`: `Append(dir, entry Entry) error` (atomic via file lock + append), `TailN(dir, n) ([]Entry, error)`.
- Effort → model/flags mapping hardcoded:

```go
var effortFlags = map[string][]string{
    "fast":     {"--model", "sonnet", "--effort", "medium", "--max-budget-usd", "1"},
    "standard": {"--model", "sonnet", "--effort", "high",   "--max-budget-usd", "2"},
    "max":      {"--model", "opus",   "--effort", "max",    "--max-budget-usd", "5"},
}
```

**Implementation sketch.**

```go
// internal/wake/claude.go

type Request struct {
    MissionDir    string
    Effort        string
    Prompt        string
    WallClockCap  time.Duration // e.g., interval - 30s or 540s max
}

type Result struct {
    ExitCode         int
    Stdout           []byte
    Stderr           []byte
    WallClockSeconds int
    PromiseToken     bool
    CostUSD          float64  // parsed from claude output JSON if present
}

func Run(ctx context.Context, req Request) (*Result, error) {
    args := []string{
        "-p",
        "--permission-mode", "bypassPermissions",
        "--dangerously-skip-permissions",
        "--add-dir", req.MissionDir,
        "--output-format", "json", // for cost parsing
    }
    args = append(args, effortFlags[req.Effort]...)
    
    ctx2, cancel := context.WithTimeout(ctx, req.WallClockCap)
    defer cancel()
    cmd := exec.CommandContext(ctx2, "claude", args...)
    cmd.Stdin = strings.NewReader(req.Prompt)
    
    start := time.Now()
    out, err := cmd.CombinedOutput()
    res := &Result{
        ExitCode:         cmd.ProcessState.ExitCode(),
        Stdout:           out,
        WallClockSeconds: int(time.Since(start).Seconds()),
    }
    res.PromiseToken, _ = ExtractPromise(out)
    res.CostUSD = parseCost(out)
    return res, err
}
```

**Verification.**
- `fry wake demo` (with a manually-set prompt of "write 'hello' to artifacts/out.txt and output ===WAKE_DONE===") produces:
  - `~/missions/demo/artifacts/out.txt` containing "hello"
  - New entry in `wake_log.jsonl` with `promise_token_found: true, exit_code: 0`
- Without promise token in prompt: `promise_token_found: false`, entry still written but flagged.
- Wall-clock timeout enforced: a prompt that infinite-loops gets killed at the cap.

**Exit gate before M4:** one real LLM call per wake works. Cost tracking works. Timeout works. Log entry is valid JSON.

---

### M4 — Prompt assembly + notes.md round-trip

**Goal.** Cross-wake memory works: wake N reads notes.md, updates it; wake N+1 sees the update and acts on it.

**Deliverables.**
- `internal/wake/prompt.go`: layered prompt assembly. Adapted from legacy-fry's `internal/sprint/prompt*.go`. Layers (top to bottom, stable prefix first):
  - L0: wake contract preamble (static template)
  - L1: mission overview (from prompt.md)
  - L2: plan or prompt body (from plan.md if present, else prompt.md continued)
  - L3: current focus + supervisor injections (from notes.md)
  - L4: last N wake_log entries (default N=5)
  - L5: current wake directive ("this is wake <N>; elapsed <H>h; do one unit of work; end with ===WAKE_DONE===")

```go
func Assemble(m *state.Mission, dir string, lastN int) (string, error)
```

- `internal/notes/notes.go`: section-aware markdown read/write. Parses the template sections (Current Focus, Next Wake Should, Decisions, Open Questions, Supervisor Injections) and exposes getters/setters. Write is atomic (temp + rename).

```go
type Notes struct {
    CurrentFocus       string
    NextWakeShould     string
    Decisions          []TimedEntry  // TimedEntry{Timestamp time.Time, Wake int, Text string}
    OpenQuestions      []string
    SupervisorInjects  []TimedEntry
}

func Load(dir string) (*Notes, error)
func (n *Notes) Save(dir string) error  // atomic
func (n *Notes) AppendDecision(wake int, text string)
func (n *Notes) AppendInjection(text string)  // used by chat session
```

- Wake logic (M3's `wake.go`) now uses real assembly + reads/writes notes.md.

**Implementation sketch.**

The prompt template (L0) is embedded as a text/template:

```go
//go:embed templates/wake_contract.md.tmpl
var wakeContractTmpl string

// L0 is rendered once per wake with {{.WakeNumber}}, {{.ElapsedHours}}, etc.
```

Stable prefix = L0 + L1 + L2. Changing part = L3 + L4 + L5. Put stable first, changing last → Claude prompt cache hits on the 80% of tokens that don't change wake-to-wake.

**Verification.**
- Wake 1 prompt asks "append a decision to notes.md with text 'hello wake 1' and output ===WAKE_DONE==="
- After wake 1: `notes.md` has that decision entry.
- Wake 2 prompt asks "read notes.md decisions and output the count as a wake_log actions_taken item"
- After wake 2: wake_log entry mentions "1 decision found: hello wake 1".
- This proves the round-trip.
- `internal/notes/notes_test.go` covers: parse a full template, edit sections, save, re-parse — equivalent output.

**Exit gate before M5:** cross-wake memory is mechanical and testable.

---

### M5 — Chat session

**Goal.** `fry chat <name>` spawns a Claude session with mission context pre-loaded. User can query status and intervene.

**Deliverables.**
- `internal/chat/chat.go`: builds the appended system prompt + spawns `claude` interactively.
- `internal/chat/systemprompt.md` (embedded): the system-prompt template that teaches Claude the mission layout, access rules, audit expectations.
- `fry chat <name>` subcommand in `cmd/fry/main.go`.
- `supervisor_log.jsonl` schema + append helper. The chat session is *expected* to append an entry whenever it changes mission state; enforced by the system prompt, audited after the fact.

**Implementation sketch.**

```go
// internal/chat/chat.go

//go:embed systemprompt.md
var chatSystemPromptTmpl string

func Launch(missionDir string, mission *state.Mission) error {
    sysPrompt := renderTemplate(chatSystemPromptTmpl, templateData{
        MissionID:       mission.MissionID,
        MissionDir:      missionDir,
        CurrentWake:     mission.CurrentWake,
        ElapsedHours:    elapsed(mission),
        // etc.
    })
    
    args := []string{
        "--add-dir", missionDir,
        "--append-system-prompt", sysPrompt,
    }
    
    cmd := exec.Command("claude", args...)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    return cmd.Run()
}
```

The system prompt template is the key design artifact. Rough shape:

```
You are the supervisor for a fry mission called {{.MissionID}}.

Mission directory: {{.MissionDir}}
Current wake: {{.CurrentWake}} | elapsed: {{.ElapsedHours}}h | status: {{.Status}}

== File layout ==
(explains each file)

== What you may do ==
- Read any file in the mission directory.
- Edit notes.md to steer the next wake (e.g. modify "Next Wake Should" section).
- Append entries to supervisor_log.jsonl for audit (REQUIRED whenever you change state).
- Run subcommands via Bash: `fry status <name>`, `fry wake <name>` (manual), `fry stop <name>`.

== What you may NOT do ==
- Modify prompt.md (the user's original input is immutable mid-mission).
- Modify state.json except through fry subcommands.
- Modify wake_log.jsonl (append-only by wakes).
- Start new missions.

== Audit expectation ==
Every edit to notes.md or manual wake/stop MUST be paired with a supervisor_log.jsonl
append describing what and why. If you forget, you've broken the audit trail.
```

**Verification.**
- `fry chat demo` opens an interactive claude session.
- User types "what's happening?" → Claude reads state.json + tail of wake_log + notes.md → responds with a coherent summary in ≤1 turn.
- User types "tell next wake to focus on X" → Claude edits notes.md (Next Wake Should section) AND appends a supervisor_log entry.
- Exit the chat → supervisor_log.jsonl has the new entry with type=intervention.

**Exit gate before M6:** chat-session steering demonstrably affects the next wake's behavior.

---

### M6 — Deadlines + no-op detection + shutdown

**Goal.** Missions self-terminate correctly. Soft deadline triggers agent-driven shutdown. Hard deadline is non-negotiable. No-op detection surfaces stalls.

**Scheduler teardown authority (design principle).**

Agent signals; tool terminates. The agent never calls `launchctl`, `systemctl`, or any scheduler command directly. The separation:

- **Agent's power:** decide when the mission is done and emit the transition signal — a final-line stdout marker `FRY_STATUS_TRANSITION=complete` (and, for overtime justification: `FRY_OVERTIME_REASON=<reason>`). Optionally, a hard-block signal `FRY_STATUS_TRANSITION=failed` with `FRY_FAILURE_REASON=<reason>`.
- **Tool's power:** parse the marker, validate against the FSM in `internal/state/transitions.go`, atomically update `state.json`, call `scheduler.Uninstall(m)`, clean the overlap lock, and (for `failed`) preserve mission artifacts untouched.

Why this matters:

1. **Scheduler is safety-critical.** A failed `launchctl unload` from agent-bash = orphan scheduler firing past budget. Tool owns it → retry logic, exit-code checking, and a fallback to `launchctl bootout` if `unload` fails.
2. **Atomicity.** The tool bundles `state update + scheduler uninstall + lock cleanup` as a single post-wake routine that's either fully-done or cleanly retryable. Agent doing these as separate bash calls can leave partial states.
3. **Auditability.** `supervisor_log.jsonl` can cleanly distinguish: agent-signaled soft shutdown, tool-forced hard-stop at 24h, and user-issued `fry stop`. Option-A style (agent runs the command) collapses all three into "someone unloaded the scheduler."

**What the agent CANNOT do:** run `launchctl`/`systemctl` via Bash. The system prompt in both wake invocation and chat session forbids this explicitly. If the tool detects an agent attempted a scheduler command (via subprocess audit in the Bash tool wrapper, or after-the-fact by reading `wake_log`), it flags a `supervisor_log` entry of type `policy_violation` and continues. It does not crash the mission, but it's a bug report.

**Illegal-transition handling:** if the marker requests a transition the FSM disallows (e.g., `complete → active`), the tool logs it to `supervisor_log.jsonl` with type `illegal_transition`, does NOT apply it, and leaves `state.json` untouched. The wake's other output (wake_log entry, artifact changes) is still accepted.

**Deliverables.**
- Deadline logic in `wake.go`:

```go
func deadlineStatus(m *state.Mission, now time.Time) state.Status {
    elapsed := now.Sub(m.CreatedAt)
    softDeadline := m.CreatedAt.Add(time.Duration(m.DurationHours * float64(time.Hour)))
    hardDeadline := softDeadline.Add(time.Duration(m.OvertimeHours * float64(time.Hour)))
    
    switch {
    case now.After(hardDeadline):
        return state.StatusComplete  // force shutdown
    case now.After(softDeadline):
        return state.StatusOvertime   // agent can continue if justified
    default:
        return state.StatusActive
    }
}
```

- After each wake: if `status == complete`, `internal/scheduler.Uninstall(m)` is called. Agent can also set `status = complete` itself by writing the status field in notes.md via a marker line like `FRY_STATUS_TRANSITION=complete` on stdout (robustly parsed, applied post-wake).
- No-op detection in `internal/wake/noop.go`:

```go
// Returns (isNoop, reason) given the mission directory.
// No-op = last 3 wake_log entries have:
//   - zero files_changed in artifacts/
//   - identical notes.md hash across the 3 wakes
//   - no new decisions appended
func DetectNoop(dir string) (bool, string, error)
```

If detected: append warning to supervisor_log.jsonl (type="noop_warning"), do not auto-stop. Next wake sees the warning and can decide.

- Hard deadline enforcement: when `fry wake` fires past hard deadline, it immediately writes a final wake_log entry and uninstalls the scheduler *without* calling claude. Zero LLM cost on hard stop.

**Verification.**
- Run dogfood mission from §15: `fry new demo --prompt /tmp/p.md --duration 2m --interval 30s --effort fast && fry start demo`.
  - Within ~3 min, `state.status == complete`.
  - LaunchAgent unloaded automatically.
  - `wake_log.jsonl` has ≥3 entries.
  - No orphan lock dir left behind.
- Simulate no-op: feed wake a prompt that does nothing 3 times in a row → supervisor_log shows 1 "noop_warning" entry.
- Simulate hard deadline: set duration=1m, overtime=0, wait 2m → next wake fires but exits immediately without calling claude; state flips to complete.

**Exit gate before M7:** all three deadline scenarios (soft, overtime, hard) behave correctly without human intervention.

---

### M7 — Polish + README + ship

**Goal.** v0.1 is declared ready. A new user can install, create a mission, run it, chat with it, without reading Go code.

**Deliverables.**
- `README.md` covering: 1-paragraph pitch, install via `go install`, 4-command quickstart, file layout diagram, link to product-spec.md for deeper reading.
- `Makefile` targets: `build`, `install`, `test`, `lint`, `clean`.
- `.gitignore` for `~/missions/` pollution if any sneaks in, Go build artifacts.
- `fry --version` subcommand returns build-injected version string (`ldflags`).
- Run the full §15 acceptance test in a clean environment; fix whatever breaks.
- Tag `v0.1.0` on GitHub.

**Verification (spec §15 gates):**
- [ ] All 8 subcommands work end-to-end.
- [ ] Acceptance dogfood mission passes.
- [ ] `fry chat demo` → "what's happening?" → coherent 1-turn answer.
- [ ] README renders correctly on GitHub.
- [ ] Non-goals enforced: grep the code for "optional", "mode", "plugin", "provider" — zero hits in non-test files.
- [ ] `cloc internal/ cmd/` → ≤2000 LOC.

**Ship checklist:**
- [ ] `git log --oneline` shows clean commit history (no "wip" or "fix typo 3" mess).
- [ ] `go vet ./...` clean.
- [ ] `golangci-lint run` clean with agreed config.
- [ ] `go test ./...` clean.
- [ ] `git tag v0.1.0 && git push --tags`.

---

## 6. Testing strategy

Three tiers:

**Unit (stdlib testing + testify/assert):**
- `internal/state/state_test.go` — marshal/unmarshal, transition rules.
- `internal/notes/notes_test.go` — section parsing round-trip.
- `internal/wakelog/wakelog_test.go` — append, tail, concurrent safety.
- `internal/wake/promise_test.go` — promise token extraction edge cases.
- `internal/wake/noop_test.go` — no-op detection on synthetic logs.
- `internal/mission/layout_test.go` — scaffolding with various input modes.

**Integration (stdlib testing + temp dirs):**
- `internal/wake/integration_test.go` — full wake with a mocked claude binary (a shell script that reads stdin, appends to out.txt, prints ===WAKE_DONE===). Verify end-to-end pipeline without real API calls.
- `internal/scheduler/darwin_test.go` — build tag, only runs on macOS. Install → kickstart → verify → uninstall. Uses a unique mission name to avoid test interference.

**Acceptance (Makefile target `make acceptance`):**
- Spins up a throwaway mission, runs for 3 min, asserts post-conditions. Real claude calls — requires auth, so only run manually before tagging releases.

Test principles:
- No global state.
- No network mocks; use local stubs and fake binaries.
- Temp dirs via `t.TempDir()` — never write outside `t.TempDir()` in tests.
- Race detector on: `go test -race ./...`.

---

## 7. Legacy-fry vendor import plan

Physically copy (not Go-import) the following from `~/code/fry-legacy/` into `~/code/fry/vendor-fry/`. Adapt to the new package layout. Do not use `go mod replace` or similar — legacy-fry stays dead.

| From (legacy-fry) | To (fry) | What to keep |
|---|---|---|
| `internal/sprint/prompt*.go` | `internal/wake/prompt.go` | Layered prompt assembly. Strip sprint-specific concepts (epic, sanity checks). Keep the ordering, the stable-prefix insight, the env-var injection. |
| `internal/lock/*.go` | `internal/wake/lock.go` | Simple file-lock patterns if they're cleaner than our mkdir approach. If mkdir is sufficient, skip. |
| `internal/cli/` | Reference only | Cobra command patterns, flag parsing style. Don't copy wholesale — legacy-fry's CLI is broader than ours. |

Explicitly **NOT vendored** (for the record, so nobody's tempted):

- `internal/copilot/` — wakes don't need a coordinator process; scheduler does that.
- `internal/observer/` — superseded by chat session.
- `internal/agentrun/` — assumes multi-agent orchestration.
- `internal/audit/`, `internal/codereview/`, `internal/heal/` — v0.1 doesn't have sanity-check infrastructure to justify these.
- `internal/team/` — no parallel runtime in v0.1.
- `internal/triage/` — deferred to v0.2+.
- `internal/skills/`, `internal/continuerun/`, `internal/consciousness/`, `internal/confirm/` — out of scope.

Vendoring happens at the start of each milestone that needs the code. Don't pre-copy everything at M0.

---

## 8. Risk register

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| LaunchAgent TCC/permission issues on user's machine | Low | High | Fall back to documented error with specific remediation steps. Test on the user's machine early in M2. |
| `claude` CLI auth tokens expire mid-mission | Low | Medium | Wake failure surfaces as nonzero exit; log entry still written with `blockers`. Chat session can notice. |
| `claude` CLI behavior changes across versions | Medium | Medium | Pin minimum version in README. Test on current version and one version back. |
| Prompt cache doesn't actually hit (L0-L2 not truly stable) | Medium | Low | Instrument cost per wake from claude JSON output; if costs don't plateau, investigate which layer is drifting. |
| Overlap lock stale after crash | Medium | Medium | On wake startup, check lock directory's mtime; if older than 2× interval, assume stale and take over. Log warning. |
| No-op detection false positives (agent deliberately pausing) | Low | Low | It's advisory, not auto-stopping. Agent's next wake can acknowledge and clear. |
| Scope creep via "just add this one feature" | **High** | **High** | §12 non-goals list. Every PR against v0.1 must either close an M1–M7 subtask or be rejected. |
| Go binary size balloons from CGO / deps | Low | Low | No CGO allowed. Deps capped at 3. Enforce in CI. |
| Dogfood acceptance test flaky due to real LLM | Medium | Medium | Acceptance tier is manual, not CI. CI runs unit + integration with mocked claude only. |

---

## 9. State transitions (explicit state machine)

The `Status` field in `state.json` follows this graph. Any transition not listed is an error.

```
active       → overtime (when elapsed crosses duration)
active       → complete (agent-initiated OR scheduler-initiated at hard deadline)
active       → stopped  (user ran `fry stop`)
active       → failed   (irrecoverable wake error)
overtime     → complete (agent-initiated OR scheduler-initiated at hard deadline)
overtime     → stopped  (user ran `fry stop`)
overtime     → failed   (irrecoverable wake error)
complete     → (terminal — no transitions out)
stopped      → active   (user ran `fry start` again — resume)
failed       → (terminal — no transitions out)
```

Codified in `internal/state/transitions.go`. Any `Save()` that would create an invalid transition errors out.

---

## 10. Build order & critical path

Milestones are strictly sequential except for testing work which can happen in parallel with the following milestone.

```
M1 ──► M2 ──► M3 ──► M4 ──► M5 ──► M6 ──► M7
                                │         ▲
                                └─────────┘
                                (M5 can start once M4 is code-complete;
                                M5 and M6 can overlap late in M4's testing)
```

**Critical path:** M1 → M2 → M3 → M4 → M6 → M7. M5 can slip by ~1 wake without blocking M7 if needed (chat is important but not on the deadline-logic critical path).

---

## 11. Definition of done (for v0.1)

A pull request labeled `v0.1-ship` merges when:

1. All §1 (scope recap) gates are green.
2. All §6 (testing strategy) tiers pass: unit + integration + manual acceptance.
3. `go vet` and `golangci-lint` are clean.
4. `wc -l internal/**/*.go cmd/**/*.go` ≤2000.
5. `grep -rE "(TODO|FIXME|XXX)" internal/ cmd/` returns nothing (no known-open issues in shipping code).
6. README exists and is accurate.
7. `fry --version` prints a real version.
8. The tag `v0.1.0` is pushed to GitHub.
9. One human (the user) has run the dogfood mission end-to-end on a fresh clone and confirmed it works.

That's v0.1. Ship it, use it for at least two real missions, then write v0.2's spec based on what actually bit.

---

## 12. Notes for future versions (don't build these now)

Captured here so they don't clutter the v0.1 scope, but so they're not lost either:

- **v0.2 candidates:** Linux systemd backend. Triage gate (optional flag). Cost-budget ceiling per mission. `fry pause/resume`. `fry template` for saving + reusing prompt templates.
- **v0.3+:** Whatever real usage demands. Not before.
- **Never:** Multi-agent frameworks. Web UI. Provider abstraction. Custom DSL. Auto-detect anything.

If v0.2+ adds any of these, it must ship with a kill-switch (flag or env var) that restores v0.1-equivalent behavior, so power users can opt out.

---

## Appendix A — Runner.sh template

Embedded in binary; rendered per mission at scaffold time.

```bash
#!/bin/zsh
# runner.sh — generated by fry for mission {{.MissionID}}
# Invoked by LaunchAgent com.fry.{{.MissionID}}

set -uo pipefail

export HOME="{{.Home}}"
export USER="{{.User}}"
export LOGNAME="{{.User}}"
export PATH="{{.Path}}"

MISSION_DIR="{{.MissionDir}}"
cd "$MISSION_DIR" || exit 1

exec {{.FryBinaryPath}} wake {{.MissionID}} >> "$MISSION_DIR/cron.log" 2>&1
```

## Appendix B — LaunchAgent plist template

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.fry.{{.MissionID}}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.MissionDir}}/runner.sh</string>
    </array>
    <key>StartInterval</key>
    <integer>{{.IntervalSeconds}}</integer>
    <key>RunAtLoad</key>
    <false/>
    <key>WorkingDirectory</key>
    <string>{{.MissionDir}}</string>
    <key>StandardOutPath</key>
    <string>{{.MissionDir}}/logs/launchd.stdout.log</string>
    <key>StandardErrorPath</key>
    <string>{{.MissionDir}}/logs/launchd.stderr.log</string>
    <key>AbandonProcessGroup</key>
    <true/>
</dict>
</plist>
```

## Appendix C — The chat system-prompt template (sketch)

Embedded at `internal/chat/systemprompt.md`. Rendered per invocation with mission context.

```markdown
You are the human-facing supervisor for a running fry mission.

Mission: {{.MissionID}}
Directory: {{.MissionDir}}
Current wake: {{.CurrentWake}} | elapsed: {{.ElapsedHours}}h | status: {{.Status}}
Soft deadline: {{.SoftDeadline}} | Hard deadline: {{.HardDeadline}}

Your job: answer the human's questions about the mission, and when they ask you to
intervene, make the change and record an audit entry.

File layout (under {{.MissionDir}}):
- prompt.md — the user's original input. DO NOT modify.
- plan.md — generated or user-provided plan. DO NOT modify unless user explicitly asks.
- state.json — machine state. DO NOT modify directly; use `fry` subcommands.
- notes.md — narrative state. You MAY edit. Append supervisor injections.
- wake_log.jsonl — per-wake log. READ ONLY.
- supervisor_log.jsonl — your audit trail. APPEND whenever you change state.
- artifacts/ — agent's working directory. You MAY read, SHOULD NOT modify.

Available shell commands:
- fry status {{.MissionID}} — quick snapshot
- fry wake {{.MissionID}} — manually fire a wake now (use sparingly)
- fry stop {{.MissionID}} — halt the mission

When you make any state change, append an entry to supervisor_log.jsonl with:
  {timestamp_utc, type, summary, fields_changed, operator: "chat"}

Start by reading state.json and the last 3 entries of wake_log.jsonl so you have context.
Then greet the human briefly.
```

---

**End of build plan.** If this looks tight and doable, the next step is M1 — and at that point fry should build itself.
