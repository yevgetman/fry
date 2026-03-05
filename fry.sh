#!/usr/bin/env bash
# =============================================================================
# fry.sh — Reusable Ralph Wiggum Loop Engine for AI Coding Agents
# =============================================================================
#
# A project-agnostic Ralph loop runner. Reads sprint definitions from an
# "epic" file and executes them as sequential Ralph-style iteration loops
# via an AI coding agent (OpenAI Codex or Claude Code), each iteration
# with fresh context.
#
# Adopts key patterns from the Ralph ecosystem:
#   - prompt.md: Prompt written to disk so the agent can re-read its own
#     instructions. Assembled with a layered context hierarchy.
#   - progress.txt: Append-only iteration memory. Each iteration reads it
#     for context from prior iterations and appends learnings when done.
#
# Required Project Structure:
#
#   project-root/
#   ├── plans/
#   │   ├── executive.md            # Executive context (optional — project vision, goals)
#   │   └── plan.md                 # Holistic build plan (all phases, architecture)
#   ├── AGENTS.md                   # Operational rules (auto-generated if missing)
#   ├── fry.sh              # This script
#   └── <epic>.md                   # Sprint definitions (passed as argument)
#
#   Generated at runtime:
#   ├── prompt.md                   # Assembled per sprint (gitignored)
#   ├── progress.txt                # Iteration memory (committed)
#   └── build-logs/                 # Per-iteration logs (gitignored)
#
#   Companion script (optional, enables auto-generation):
#   └── fry-prepare.sh              # Generates epic.md from plans/plan.md via AI agent
#
# If the epic file passed as argument doesn't exist, fry.sh will automatically
# call fry-prepare.sh to generate it from plans/plan.md. This requires
# epic-example.md and GENERATE_EPIC.md alongside fry-prepare.sh.
#
# Usage:
#   ./fry.sh <epic.md>                    # Run all sprints (default engine: codex)
#   ./fry.sh <epic.md> --engine claude    # Run all sprints with Claude Code
#   ./fry.sh <epic.md> 4                  # Start from sprint 4
#   ./fry.sh <epic.md> 4 4               # Run only sprint 4
#   ./fry.sh <epic.md> 3 5               # Run sprints 3 through 5
#   ./fry.sh <epic.md> --dry-run          # Parse & validate only
#
# Epic File Format:
#
#   Global directives (before any @sprint):
#     @epic <name>                       # Display name for logs and summaries
#     @engine <codex|claude>             # AI engine to use (default: codex)
#     @docker_from_sprint <N>            # Enable docker-compose from sprint N
#     @docker_ready_cmd <command>        # Custom health check (default: wait for healthy)
#     @docker_ready_timeout <seconds>    # Health check timeout (default: 30)
#     @require_tool <tool>               # CLI tool on PATH (repeatable)
#     @preflight_cmd <command>           # Custom preflight command (repeatable)
#     @pre_sprint <command>              # Run before each sprint starts
#     @pre_iteration <command>           # Run before each agent exec call
#     @model <model>                     # Override agent model (alias: @codex_model)
#     @engine_flags <flags>              # Extra flags for agent exec (alias: @codex_flags)
#
#   Per-sprint block:
#     @sprint <N>                        # Sprint number (starts new block)
#     @name <display name>               # Sprint name for logs/summary
#     @max_iterations <N>                # Max iterations before failure
#     @promise <TOKEN>                   # Completion token to detect in output
#     @prompt                            # Marks start of prompt text
#     <prompt text lines...>             # Everything until next @sprint or @end or EOF
#     @end                               # (Optional) explicitly ends prompt block
#
#   Engine support:
#     Codex (default): codex exec --dangerously-bypass-approvals-and-sandbox "prompt"
#     Claude Code:     claude -p --dangerously-skip-permissions "prompt"
#
#   Engine selection (highest precedence wins):
#     1. --engine CLI flag
#     2. @engine epic directive
#     3. FRY_ENGINE environment variable
#     4. Default: codex
#
#   Requires: bash 4.0+ (associative arrays). macOS ships 3.2; install via
#   brew install bash and invoke with /opt/homebrew/bin/bash.
#
# =============================================================================

set -uo pipefail

# --- Bash 4+ required (associative arrays, regex match groups) ---
if [[ -z "${BASH_VERSINFO[0]:-}" ]] || [[ "${BASH_VERSINFO[0]}" -lt 4 ]]; then
  echo "ERROR: fry.sh requires bash 4.0+. Current: ${BASH_VERSION:-unknown}"
  echo "  macOS ships bash 3.2. Install bash 4+ via: brew install bash"
  echo "  Then run: /opt/homebrew/bin/bash $0 $*"
  exit 1
fi

# =============================================================================
# CONSTANTS — enforced project structure
# =============================================================================

readonly PLANS_DIR="plans"
readonly CONTEXT_FILE="${PLANS_DIR}/executive.md"
readonly PLAN_FILE="${PLANS_DIR}/plan.md"
readonly AGENTS_FILE="AGENTS.md"

# =============================================================================
# GLOBALS — populated by parse_epic()
# =============================================================================

EPIC_NAME=""
ENGINE=""
DOCKER_FROM_SPRINT=0
DOCKER_READY_CMD=""
DOCKER_READY_TIMEOUT=30
REQUIRED_TOOLS=""
PREFLIGHT_CMDS=()
PRE_SPRINT_CMD=""
PRE_ITERATION_CMD=""
AGENT_MODEL=""
AGENT_FLAGS=""
TOTAL_SPRINTS=0

declare -A SPRINT_NAMES
declare -A SPRINT_MAX_ITERS
declare -A SPRINT_PROMISES

# =============================================================================
# GLOBALS — runtime state
# =============================================================================

EPIC_FILE=""
PROJECT_DIR=""
LOG_DIR=""
TIMESTAMP=""
START_SPRINT=0
END_SPRINT=0
BUILD_START_TIME=0
WORK_DIR=""
DOCKER_COMPOSE=""
DRY_RUN=0
LOCK_FILE=""
CLI_ENGINE=""
CLI_PREPARE_ENGINE=""

declare -A SPRINT_RESULTS
declare -A SPRINT_DURATIONS

# =============================================================================
# USAGE
# =============================================================================

usage() {
  cat <<'EOF'
Usage: ./fry.sh <epic.md> [start_sprint] [end_sprint] [options]

Arguments:
  epic.md              Path to the epic definition file
  start_sprint         First sprint to run (default: 1)
  end_sprint           Last sprint to run (default: last sprint in epic)

Options:
  --engine <eng>       AI engine: codex or claude (default: codex)
  --prepare-engine <eng>  Engine for auto-generating epic via fry-prepare.sh
  --dry-run            Parse epic and show plan without running anything
  --help, -h           Show this message

Engine selection precedence (highest wins):
  1. --engine flag    2. @engine epic directive
  3. FRY_ENGINE env   4. default (codex)

If epic.md doesn't exist, fry-prepare.sh is called automatically to
generate AGENTS.md (if missing) and epic.md from plans/plan.md.

Required project structure (before first run):
  plans/plan.md          Holistic build plan (all phases, architecture)

Optional (used if present):
  plans/executive.md     Executive context (project vision, goals, scope)
  AGENTS.md              Operational rules (auto-generated from plan.md if missing)

Examples:
  ./fry.sh epic.md                              # Run all sprints with codex
  ./fry.sh epic.md --engine claude              # Run all sprints with claude code
  ./fry.sh epic.md 4                            # Start from sprint 4 to end
  ./fry.sh epic.md 4 4                          # Run only sprint 4
  ./fry.sh epic.md 3 5                          # Run sprints 3 through 5
  ./fry.sh epic.md --dry-run                    # Validate epic, show plan
  ./fry.sh epic.md --prepare-engine claude      # Use claude for epic generation
  FRY_ENGINE=claude ./fry.sh epic.md            # Set engine via env var

Run ./fry.sh --help for this message.
EOF
  exit 0
}

# =============================================================================
# ENGINE RESOLUTION & DISPATCH
# =============================================================================

# Resolve the effective engine: CLI flag > epic directive > env var > default
resolve_engine() {
  if [[ -n "$CLI_ENGINE" ]]; then
    ENGINE="$CLI_ENGINE"
  elif [[ -z "$ENGINE" ]]; then
    # No epic directive set — fall back to env var or default
    ENGINE="${FRY_ENGINE:-codex}"
  fi
  # Validate
  case "$ENGINE" in
    codex|claude) ;;
    *)
      echo "ERROR: Unknown engine '${ENGINE}'. Must be 'codex' or 'claude'."
      exit 1
      ;;
  esac
}

# Execute a prompt via the configured AI engine.
# Usage: run_agent "prompt text"
# Outputs to stdout; caller handles logging/teeing.
run_agent() {
  local prompt="$1"
  local -a cmd

  case "$ENGINE" in
    codex)
      cmd=(codex exec --dangerously-bypass-approvals-and-sandbox)
      ;;
    claude)
      cmd=(claude -p --dangerously-skip-permissions)
      ;;
    *)
      echo "ERROR: Unknown engine '${ENGINE}'. Must be 'codex' or 'claude'."
      return 1
      ;;
  esac

  if [[ -n "$AGENT_MODEL" ]]; then
    cmd+=(--model "$AGENT_MODEL")
  fi

  if [[ -n "$AGENT_FLAGS" ]]; then
    local -a extra_flags
    read -ra extra_flags <<< "$AGENT_FLAGS"
    cmd+=("${extra_flags[@]}")
  fi

  cmd+=("$prompt")
  "${cmd[@]}"
}

# =============================================================================
# LOGGING
# =============================================================================

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "${LOG_DIR}/build_${TIMESTAMP}.log"
}

# =============================================================================
# EPIC PARSER
# =============================================================================

parse_epic() {
  local epic_file=$1
  local state="GLOBAL"
  local current_sprint=0
  local prompt_file=""

  if [[ ! -f "$epic_file" ]]; then
    echo "ERROR: Epic file not found: ${epic_file}"
    exit 1
  fi

  while IFS= read -r line || [[ -n "$line" ]]; do

    if [[ "$line" =~ ^@sprint[[:space:]]+([0-9]+)[[:space:]]*$ ]]; then
      current_sprint=$((10#${BASH_REMATCH[1]}))
      if [[ $current_sprint -eq 0 ]]; then
        echo "ERROR: Sprint numbers must start at 1 (found @sprint 0)."
        exit 1
      fi
      if [[ -f "${WORK_DIR}/sprint_${current_sprint}_prompt.txt" ]]; then
        echo "ERROR: Duplicate @sprint ${current_sprint} found in epic file."
        exit 1
      fi
      if [[ $current_sprint -gt $TOTAL_SPRINTS ]]; then
        TOTAL_SPRINTS=$current_sprint
      fi
      prompt_file="${WORK_DIR}/sprint_${current_sprint}_prompt.txt"
      > "$prompt_file"
      state="SPRINT_META"
      continue
    fi

    case "$state" in
      GLOBAL)
        [[ "$line" =~ ^@epic[[:space:]]+(.+) ]]                    && EPIC_NAME="${BASH_REMATCH[1]}" && continue
        [[ "$line" =~ ^@engine[[:space:]]+(.+) ]]                  && ENGINE="${BASH_REMATCH[1]}" && continue
        [[ "$line" =~ ^@docker_from_sprint[[:space:]]+([0-9]+) ]]   && DOCKER_FROM_SPRINT="${BASH_REMATCH[1]}" && continue
        [[ "$line" =~ ^@docker_ready_cmd[[:space:]]+(.+) ]]        && DOCKER_READY_CMD="${BASH_REMATCH[1]}" && continue
        [[ "$line" =~ ^@docker_ready_timeout[[:space:]]+([0-9]+) ]] && DOCKER_READY_TIMEOUT="${BASH_REMATCH[1]}" && continue
        [[ "$line" =~ ^@require_tool[[:space:]]+(.+) ]]            && REQUIRED_TOOLS="${REQUIRED_TOOLS:+$REQUIRED_TOOLS }${BASH_REMATCH[1]}" && continue
        [[ "$line" =~ ^@preflight_cmd[[:space:]]+(.+) ]]           && PREFLIGHT_CMDS+=("${BASH_REMATCH[1]}") && continue
        [[ "$line" =~ ^@pre_sprint[[:space:]]+(.+) ]]              && PRE_SPRINT_CMD="${BASH_REMATCH[1]}" && continue
        [[ "$line" =~ ^@pre_iteration[[:space:]]+(.+) ]]           && PRE_ITERATION_CMD="${BASH_REMATCH[1]}" && continue
        # @model and @engine_flags (with backward-compat aliases @codex_model, @codex_flags)
        [[ "$line" =~ ^@(model|codex_model)[[:space:]]+(.+) ]]     && AGENT_MODEL="${BASH_REMATCH[2]}" && continue
        [[ "$line" =~ ^@(engine_flags|codex_flags)[[:space:]]+(.+) ]] && AGENT_FLAGS="${BASH_REMATCH[2]}" && continue
        # Warn on unrecognized @ directives (likely typos)
        if [[ "$line" =~ ^@[a-z] ]]; then
          echo "WARNING: Unrecognized global directive: ${line}"
        fi
        ;;
      SPRINT_META)
        [[ "$line" =~ ^@name[[:space:]]+(.+) ]]              && SPRINT_NAMES[$current_sprint]="${BASH_REMATCH[1]}" && continue
        [[ "$line" =~ ^@max_iterations[[:space:]]+([0-9]+) ]] && SPRINT_MAX_ITERS[$current_sprint]="${BASH_REMATCH[1]}" && continue
        [[ "$line" =~ ^@promise[[:space:]]+(.+) ]]           && SPRINT_PROMISES[$current_sprint]="${BASH_REMATCH[1]}" && continue
        if [[ "$line" =~ ^@prompt[[:space:]]*$ ]]; then
          state="SPRINT_PROMPT"
          continue
        fi
        # Warn on unrecognized @ directives (likely typos)
        if [[ "$line" =~ ^@[a-z] ]]; then
          echo "WARNING: Unrecognized directive in sprint ${current_sprint}: ${line}"
        fi
        ;;
      SPRINT_PROMPT)
        if [[ "$line" =~ ^@end[[:space:]]*$ ]]; then
          state="GLOBAL"
          continue
        fi
        echo "$line" >> "$prompt_file"
        ;;
    esac
  done < "$epic_file"

  # Strip prompt bleed (markdown dividers between sprint blocks)
  for n in $(seq 1 "$TOTAL_SPRINTS"); do
    local pfile="${WORK_DIR}/sprint_${n}_prompt.txt"
    [[ -f "$pfile" ]] || continue
    local cleaned
    cleaned=$(awk '
      { lines[NR] = $0 }
      END {
        last = NR
        while (last > 0 && lines[last] ~ /^[[:space:]]*$/) last--
        if (last > 0 && lines[last] ~ /^#[[:space:]]*=+/) {
          last--
          while (last > 0 && lines[last] ~ /^#/) last--
          while (last > 0 && lines[last] ~ /^[[:space:]]*$/) last--
        }
        for (i = 1; i <= last; i++) print lines[i]
      }
    ' "$pfile")
    if [[ -z "$cleaned" ]] || [[ "$cleaned" =~ ^[[:space:]]*$ ]]; then
      > "$pfile"
    else
      printf '%s\n' "$cleaned" > "$pfile"
    fi
  done
}

# =============================================================================
# VALIDATION
# =============================================================================

validate_epic() {
  local errors=0

  if [[ $TOTAL_SPRINTS -eq 0 ]]; then
    echo "ERROR: No sprints found in epic file."
    errors=1
  fi

  for n in $(seq 1 "$TOTAL_SPRINTS"); do
    local pfile="${WORK_DIR}/sprint_${n}_prompt.txt"
    if [[ ! -f "$pfile" ]]; then
      echo "ERROR: Sprint ${n} is missing (sprints must be numbered 1..N with no gaps)."
      errors=1
      continue
    fi
    if [[ -z "${SPRINT_NAMES[$n]:-}" ]]; then
      echo "ERROR: Sprint ${n} missing @name"
      errors=1
    fi
    if [[ -z "${SPRINT_MAX_ITERS[$n]:-}" ]]; then
      echo "ERROR: Sprint ${n} missing @max_iterations"
      errors=1
    fi
    if [[ ! -s "$pfile" ]]; then
      echo "ERROR: Sprint ${n} has empty prompt (missing @prompt block?)"
      errors=1
    fi
  done

  if [[ $errors -ne 0 ]]; then
    echo "Epic validation failed."
    exit 1
  fi
}

# =============================================================================
# DRY RUN
# =============================================================================

dry_run_report() {
  echo ""
  echo "========================================="
  echo "  EPIC: ${EPIC_NAME:-<unnamed>}"
  echo "  File: ${EPIC_FILE}"
  echo "  Sprints: ${TOTAL_SPRINTS}"
  echo "  Run range: ${START_SPRINT} — ${END_SPRINT}"
  echo "========================================="
  echo ""
  echo "  Project Structure:"
  for f in "${PLANS_DIR}/" "$PLAN_FILE" "$AGENTS_FILE"; do
    if [[ -e "${PROJECT_DIR}/${f}" ]]; then
      echo "    [ok]    ${f}"
    else
      echo "    [MISS]  ${f}"
    fi
  done
  if [[ -f "${PROJECT_DIR}/${CONTEXT_FILE}" ]]; then
    echo "    [ok]    ${CONTEXT_FILE}"
  else
    echo "    [skip]  ${CONTEXT_FILE} (optional)"
  fi
  echo ""
  echo "  Config:"
  echo "    engine:              ${ENGINE}"
  if [[ $DOCKER_FROM_SPRINT -gt 0 ]]; then
    echo "    docker_from_sprint:  ${DOCKER_FROM_SPRINT}"
  else
    echo "    docker_from_sprint:  disabled"
  fi
  echo "    docker_ready_cmd:    ${DOCKER_READY_CMD:-<default: wait for healthy>}"
  echo "    docker_ready_timeout: ${DOCKER_READY_TIMEOUT}s"
  echo "    required_tools:      ${REQUIRED_TOOLS:-<none>}"
  echo "    pre_sprint:          ${PRE_SPRINT_CMD:-<none>}"
  echo "    pre_iteration:       ${PRE_ITERATION_CMD:-<none>}"
  echo "    model:               ${AGENT_MODEL:-<default>}"
  echo "    engine_flags:        ${AGENT_FLAGS:-<none>}"
  if [[ ${#PREFLIGHT_CMDS[@]} -gt 0 ]]; then
    echo "    preflight_cmds:"
    for cmd in "${PREFLIGHT_CMDS[@]}"; do
      echo "      - ${cmd}"
    done
  fi
  echo ""

  for n in $(seq 1 "$TOTAL_SPRINTS"); do
    local in_range=" "
    if [[ $n -ge $START_SPRINT ]] && [[ $n -le $END_SPRINT ]]; then
      in_range=">"
    fi
    local pfile="${WORK_DIR}/sprint_${n}_prompt.txt"
    local prompt_lines=0
    if [[ -f "$pfile" ]]; then
      prompt_lines=$(wc -l < "$pfile" | tr -d ' ')
    fi
    echo "  ${in_range} Sprint ${n}: ${SPRINT_NAMES[$n]:-<unnamed>}"
    echo "      max_iterations: ${SPRINT_MAX_ITERS[$n]:-?}"
    echo "      promise:        ${SPRINT_PROMISES[$n]:-<none — will run all iterations>}"
    echo "      prompt:         ${prompt_lines} lines"
  done

  echo ""
  echo "Dry run complete. No sprints were executed."
}

# =============================================================================
# PREFLIGHT CHECKS
# =============================================================================

preflight() {
  log "Running preflight checks..."
  local failed=0

  # --- AI engine CLI ---
  case "$ENGINE" in
    codex)
      if ! command -v codex &> /dev/null; then
        log "ERROR: 'codex' CLI not found. Install: npm i -g @openai/codex"
        failed=1
      fi
      ;;
    claude)
      if ! command -v claude &> /dev/null; then
        log "ERROR: 'claude' CLI not found. Install: npm i -g @anthropic-ai/claude-code"
        failed=1
      fi
      ;;
  esac

  # --- Git ---
  if ! command -v git &> /dev/null; then
    log "ERROR: 'git' not found. Git is required."
    failed=1
  fi

  # --- Enforced project structure ---
  if [[ ! -d "${PROJECT_DIR}/${PLANS_DIR}" ]]; then
    log "ERROR: Required directory missing: ${PLANS_DIR}/"
    log "       Create it: mkdir -p ${PLANS_DIR}"
    failed=1
  fi

  if [[ ! -f "${PROJECT_DIR}/${PLAN_FILE}" ]]; then
    log "ERROR: Required file missing: ${PLAN_FILE}"
    log "       This is the holistic build plan (all phases, architecture)."
    failed=1
  fi

  if [[ -f "${PROJECT_DIR}/${CONTEXT_FILE}" ]]; then
    log "Found: ${CONTEXT_FILE}"
  else
    log "Note:  ${CONTEXT_FILE} not found (optional — prompt.md will omit executive context layer)"
  fi

  if [[ ! -f "${PROJECT_DIR}/${AGENTS_FILE}" ]]; then
    log "ERROR: Required file missing: ${AGENTS_FILE}"
    log "       Run fry-prepare.sh first, or create it manually."
    failed=1
  fi

  # --- Docker (only if needed within the sprint range being run) ---
  if [[ $DOCKER_FROM_SPRINT -gt 0 ]] && [[ $END_SPRINT -ge $DOCKER_FROM_SPRINT ]]; then
    if ! command -v docker &> /dev/null; then
      log "ERROR: 'docker' not found (required from sprint ${DOCKER_FROM_SPRINT})."
      failed=1
    elif ! docker info &> /dev/null; then
      log "ERROR: Docker daemon not running."
      failed=1
    fi

    if docker compose version &> /dev/null; then
      DOCKER_COMPOSE="docker compose"
      log "Using Docker Compose V2"
    elif command -v docker-compose &> /dev/null; then
      DOCKER_COMPOSE="docker-compose"
      log "Using Docker Compose V1"
    else
      log "ERROR: Neither 'docker compose' (V2) nor 'docker-compose' (V1) found."
      failed=1
    fi
  fi

  # --- Required tools from epic ---
  for tool in $REQUIRED_TOOLS; do
    if ! command -v "$tool" &> /dev/null; then
      log "ERROR: Required tool '${tool}' not found on PATH."
      failed=1
    else
      log "Found: ${tool}"
    fi
  done

  # --- Disk space (warn < 2GB) ---
  local free_kb
  free_kb=$(df -k "$PROJECT_DIR" 2>/dev/null | awk 'NR==2 {print $4}')
  if [[ "$free_kb" =~ ^[0-9]+$ ]] && [[ "$free_kb" -lt 2097152 ]]; then
    log "WARNING: Less than 2GB free disk space."
  fi

  # --- Custom preflight commands from epic ---
  for cmd in "${PREFLIGHT_CMDS[@]+"${PREFLIGHT_CMDS[@]}"}"; do
    log "Running preflight command: ${cmd}"
    if ! eval "$cmd"; then
      log "ERROR: Preflight command failed: ${cmd}"
      failed=1
    fi
  done

  if [[ $failed -ne 0 ]]; then
    log "Preflight checks FAILED. Fix errors above and retry."
    exit 1
  fi

  log "Preflight checks passed."
}

# =============================================================================
# DOCKER MANAGEMENT
# =============================================================================

ensure_docker_up() {
  if [[ -z "$DOCKER_COMPOSE" ]]; then return 0; fi
  if [[ ! -f "${PROJECT_DIR}/docker-compose.yml" ]] && [[ ! -f "${PROJECT_DIR}/compose.yml" ]]; then
    return 0
  fi

  cd "$PROJECT_DIR"
  local running
  if [[ "$DOCKER_COMPOSE" == "docker compose" ]]; then
    running=$($DOCKER_COMPOSE ps --status running -q 2>/dev/null | wc -l | tr -d ' ')
  else
    # V1 ps -q lists all containers regardless of state; grep table output instead
    running=$($DOCKER_COMPOSE ps 2>/dev/null | grep -c "Up" || echo "0")
  fi

  if [[ "$running" -eq 0 ]]; then
    log "Containers not running. Starting docker-compose..."
    $DOCKER_COMPOSE up -d 2>&1 | tee -a "${LOG_DIR}/build_${TIMESTAMP}.log"

    log "Waiting for services to be ready (timeout: ${DOCKER_READY_TIMEOUT}s)..."
    local attempts=0
    if [[ -n "$DOCKER_READY_CMD" ]]; then
      while [[ $attempts -lt $DOCKER_READY_TIMEOUT ]]; do
        if eval "$DOCKER_READY_CMD" &>/dev/null; then
          log "Services ready (custom check passed)."
          return 0
        fi
        sleep 1
        attempts=$((attempts + 1))
      done
    else
      while [[ $attempts -lt $DOCKER_READY_TIMEOUT ]]; do
        local unhealthy
        unhealthy=$($DOCKER_COMPOSE ps 2>/dev/null | grep -c -i "starting\|unhealthy" || true)
        if [[ "$unhealthy" -eq 0 ]]; then
          log "All Docker services are ready."
          return 0
        fi
        sleep 1
        attempts=$((attempts + 1))
      done
    fi
    log "WARNING: Services may not be ready after ${DOCKER_READY_TIMEOUT}s. Continuing."
  fi
}

# =============================================================================
# GIT — init & checkpointing
# =============================================================================

init_git() {
  cd "$PROJECT_DIR"
  if [[ ! -d .git ]]; then
    log "Initializing git repo..."
    git init 2>/dev/null || true
  fi

  # Ensure git has a user identity for commits (local to this repo only)
  if ! git config user.name &>/dev/null; then
    git config user.name "fry" 2>/dev/null || true
  fi
  if ! git config user.email &>/dev/null; then
    git config user.email "fry@automated" 2>/dev/null || true
  fi

  # Ensure .gitignore has our entries
  local needs_update=0
  if [[ ! -f .gitignore ]]; then
    needs_update=1
  else
    grep -q '^prompt\.md$' .gitignore 2>/dev/null || needs_update=1
  fi

  if [[ $needs_update -eq 1 ]]; then
    cat >> .gitignore << 'GITIGNORE'

# fry.sh generated files
build-logs/
prompt.md
.env
.fry.lock
.DS_Store
GITIGNORE
    log "Updated .gitignore"
  fi

  git add -A 2>/dev/null || true
  git commit -m "${EPIC_NAME:-Ralph}: initial state before build" \
    --allow-empty 2>/dev/null || true
}

git_checkpoint() {
  local sprint_num=$1
  local label="${2:-complete}"
  cd "$PROJECT_DIR"

  if [[ -d .git ]]; then
    git add -A 2>/dev/null || true
    git commit -m "${EPIC_NAME:-Ralph}: Sprint ${sprint_num} ${label} [automated]" \
      --allow-empty 2>/dev/null || true
    log "Git checkpoint: Sprint ${sprint_num} ${label}"
  fi
}

# =============================================================================
# CLEANUP TRAPS
# =============================================================================

on_exit() {
  if [[ -n "${LOCK_FILE:-}" ]] && [[ -f "$LOCK_FILE" ]]; then
    rm -f "$LOCK_FILE"
  fi
  if [[ -n "${WORK_DIR:-}" ]] && [[ -d "$WORK_DIR" ]]; then
    rm -rf "$WORK_DIR"
  fi
}

on_signal() {
  local signal=$1
  log ""
  log "Build interrupted (${signal})."
  cd "$PROJECT_DIR"
  if [[ -d .git ]]; then
    git add -A 2>/dev/null || true
    git commit -m "${EPIC_NAME:-Ralph}: interrupted (partial) [automated]" \
      --allow-empty 2>/dev/null || true
    log "Partial work committed."
  fi
  print_summary 2>/dev/null || true
  exit 130
}

# =============================================================================
# SPRINT RUNNER — the Ralph loop
# =============================================================================
#
# prompt.md is assembled with a layered context hierarchy:
#   1. PROJECT CONTEXT  — plans/executive.md (injected, orientation only)
#   2. STRATEGIC PLAN   — pointer to plans/plan.md (agent reads from disk)
#   3. SPRINT INSTRUCTIONS — tactical tasks from @prompt block
#   4. ITERATION MEMORY — progress.txt read/write instructions
#   5. COMPLETION SIGNAL — promise token
# =============================================================================

run_sprint() {
  local sprint_num=$1
  local sprint_name="${SPRINT_NAMES[$sprint_num]:-Sprint ${sprint_num}}"
  local max_iter="${SPRINT_MAX_ITERS[$sprint_num]:-10}"
  local promise="${SPRINT_PROMISES[$sprint_num]:-}"
  local prompt_src="${WORK_DIR}/sprint_${sprint_num}_prompt.txt"
  local sprint_start_time
  sprint_start_time=$(date +%s)

  log "========================================="
  log "STARTING SPRINT ${sprint_num}: ${sprint_name}"
  log "Max iterations: ${max_iter}"
  [[ -n "$promise" ]] && log "Promise token: ${promise}"
  log "========================================="

  if [[ $DOCKER_FROM_SPRINT -gt 0 ]] && [[ $sprint_num -ge $DOCKER_FROM_SPRINT ]]; then
    ensure_docker_up
  fi

  if [[ -n "$PRE_SPRINT_CMD" ]]; then
    cd "$PROJECT_DIR"
    log "Running pre-sprint hook: ${PRE_SPRINT_CMD}"
    eval "$PRE_SPRINT_CMD" 2>/dev/null || true
  fi

  cd "$PROJECT_DIR"

  if [[ ! -s "$prompt_src" ]]; then
    log "ERROR: Sprint ${sprint_num} prompt file is empty or missing."
    SPRINT_RESULTS[$sprint_num]="FAIL (no prompt)"
    return 1
  fi

  # --- Initialize progress.txt ---
  local progress_file="${PROJECT_DIR}/progress.txt"
  if [[ ! -f "$progress_file" ]]; then
    # First run ever — create fresh
    cat > "$progress_file" << PROGRESS_HEADER
# Progress Log — ${EPIC_NAME:-Ralph Build}
# Sprint ${sprint_num}: ${sprint_name}
# Started: $(date)
# ---
# This file is your memory across iterations. READ it at the start of each
# iteration to understand what previous iterations accomplished. APPEND to
# it at the end of each iteration with what you completed and what you learned.
# ---
PROGRESS_HEADER
    log "  Initialized progress.txt"
  elif [[ $START_SPRINT -eq 1 && $sprint_num -eq 1 ]]; then
    # Explicit restart from sprint 1 — overwrite stale progress
    cat > "$progress_file" << PROGRESS_HEADER
# Progress Log — ${EPIC_NAME:-Ralph Build}
# Sprint ${sprint_num}: ${sprint_name}
# Started: $(date)
# ---
# This file is your memory across iterations. READ it at the start of each
# iteration to understand what previous iterations accomplished. APPEND to
# it at the end of each iteration with what you completed and what you learned.
# ---
PROGRESS_HEADER
    log "  Reset progress.txt (fresh run from sprint 1)"
  else
    # Resuming or continuing — append sprint header, preserve history
    cat >> "$progress_file" << PROGRESS_SECTION

# ---
# Sprint ${sprint_num}: ${sprint_name}
# Started: $(date)
# ---
PROGRESS_SECTION
    log "  Appended sprint header to existing progress.txt"
  fi

  # --- Assemble prompt.md ---
  local prompt_md="${PROJECT_DIR}/prompt.md"
  {
    # Layer 1: Executive context (injected — only if executive.md exists)
    if [[ -f "${PROJECT_DIR}/${CONTEXT_FILE}" ]]; then
      cat << CONTEXT_HEADER
# ===== PROJECT CONTEXT =====
# The following is the executive context for this project. Use it to understand
# the project's purpose, goals, and scope. This is for orientation only — do
# NOT derive implementation decisions from this section.

CONTEXT_HEADER
      cat "${PROJECT_DIR}/${CONTEXT_FILE}"
      echo ""
      echo ""
    fi

    # Layer 2: Strategic plan reference (pointer)
    cat << PLAN_POINTER
# ===== STRATEGIC PLAN =====
# Read \`${PLAN_FILE}\` for the holistic build plan. It describes the full
# project architecture, all phases, and how they connect. This sprint implements
# one phase of that plan. Use it as your "true north" for understanding:
#   - How this sprint's work fits into the larger system
#   - What other phases will build on top of what you create here
#   - Architectural decisions and constraints that span phases
#
# Do NOT implement work from other phases — only use the plan for context.

PLAN_POINTER

    # Layer 3: Sprint instructions (tactical)
    echo "# ===== SPRINT INSTRUCTIONS ====="
    echo ""
    cat "$prompt_src"

    # Layer 4: Iteration memory
    cat << 'PROGRESS_INSTRUCTIONS'

# ===== ITERATION MEMORY =====
# BEFORE you begin work, read `progress.txt` in the project root. It contains
# a log of what previous iterations accomplished and any learnings or context
# they recorded. Use this to avoid redoing completed work and to understand
# what remains.
#
# AFTER you finish your work for this iteration (whether successful or not),
# APPEND a brief entry to `progress.txt` with:
#   - What you accomplished this iteration
#   - What remains to be done
#   - Any discoveries, gotchas, or context that would help the next iteration
#   - Files you created or modified
#
# Format your entry like:
#   ## Iteration N — [date/time]
#   **Completed:** ...
#   **Remaining:** ...
#   **Notes:** ...
PROGRESS_INSTRUCTIONS

    # Layer 5: Completion signal
    if [[ -n "$promise" ]]; then
      cat << PROMISE_SIGNAL

# ===== COMPLETION SIGNAL =====
# When ALL tasks described above are complete and all verifications pass,
# output exactly: <promise>${promise}</promise>
# If tasks remain incomplete, describe what you accomplished and what remains.
PROMISE_SIGNAL
    fi
  } > "$prompt_md"

  log "  Wrote prompt.md ($(wc -l < "$prompt_md" | tr -d ' ') lines)"

  # --- Iteration loop ---
  local iter=0
  local found_promise=0
  local last_exit_code=0

  while [[ $iter -lt $max_iter ]]; do
    iter=$((iter + 1))
    log "  Sprint ${sprint_num}, iteration ${iter}/${max_iter}..."

    if [[ -n "$PRE_ITERATION_CMD" ]]; then
      cd "$PROJECT_DIR"
      eval "$PRE_ITERATION_CMD" 2>/dev/null || true
    fi

    cd "$PROJECT_DIR"
    local iter_log="${LOG_DIR}/sprint${sprint_num}_iter${iter}_${TIMESTAMP}.log"

    run_agent \
      "Read and execute ALL instructions in prompt.md in the project root. Before starting, read progress.txt for context from previous iterations. Also read ${PLAN_FILE} for strategic context on how this sprint fits the overall plan. After completing your work, append your progress to progress.txt." \
      2>&1 | tee -a "$iter_log" "${LOG_DIR}/sprint${sprint_num}_${TIMESTAMP}.log"

    last_exit_code=${PIPESTATUS[0]}

    if [[ -n "$promise" ]] && grep -qF "<promise>${promise}</promise>" "$iter_log" 2>/dev/null; then
      log "  Promise '${promise}' found on iteration ${iter}."
      found_promise=1
      break
    fi

    if [[ $last_exit_code -ne 0 ]]; then
      log "  Iteration ${iter} exited with code ${last_exit_code}. Retrying with fresh context..."
    fi

    if [[ $iter -lt $max_iter ]]; then
      log "  Iteration ${iter} complete. ${promise:+Promise not yet found. }Continuing..."
    else
      log "  Iteration ${iter} complete (final).${promise:+ Promise not found.}"
    fi
  done

  # --- Record results ---
  local sprint_end_time
  sprint_end_time=$(date +%s)
  local duration=$(( sprint_end_time - sprint_start_time ))
  local minutes=$(( duration / 60 ))
  local seconds=$(( duration % 60 ))
  SPRINT_DURATIONS[$sprint_num]="${minutes}m ${seconds}s"

  if [[ -n "$promise" ]]; then
    if [[ $found_promise -eq 1 ]]; then
      log "SPRINT ${sprint_num} COMPLETED (${minutes}m ${seconds}s, ${iter} iterations)"
      SPRINT_RESULTS[$sprint_num]="PASS"
      git_checkpoint "$sprint_num" "complete"
    else
      log "SPRINT ${sprint_num} FAILED — promise not found after ${max_iter} iterations (${minutes}m ${seconds}s)"
      log "Resume: $0 ${EPIC_FILE} ${sprint_num}"
      SPRINT_RESULTS[$sprint_num]="FAIL (no promise after ${max_iter} iters)"
      git_checkpoint "$sprint_num" "failed-partial"
      return 1
    fi
  else
    log "SPRINT ${sprint_num} FINISHED (${minutes}m ${seconds}s, ${iter} iterations, no promise check)"
    SPRINT_RESULTS[$sprint_num]="PASS"
    git_checkpoint "$sprint_num" "complete"
  fi
}

# =============================================================================
# BUILD SUMMARY
# =============================================================================

print_summary() {
  local build_end_time
  build_end_time=$(date +%s)
  local total_duration=$(( build_end_time - BUILD_START_TIME ))
  local total_min=$(( total_duration / 60 ))
  local total_sec=$(( total_duration % 60 ))

  log ""
  log "========================================="
  log "  BUILD SUMMARY: ${EPIC_NAME:-Ralph Build}"
  log "========================================="
  log ""

  for sprint_num in $(seq "$START_SPRINT" "$END_SPRINT"); do
    local result="${SPRINT_RESULTS[$sprint_num]:-SKIPPED}"
    local duration="${SPRINT_DURATIONS[$sprint_num]:-n/a}"
    local name="${SPRINT_NAMES[$sprint_num]:-Sprint ${sprint_num}}"
    local icon="?"
    [[ "$result" = "PASS" ]] && icon="PASS"
    [[ "$result" == FAIL* ]] && icon="FAIL"
    log "  [${icon}] Sprint ${sprint_num}: ${name}"
    log "          ${result}  (${duration})"
  done

  log ""
  log "  Total time: ${total_min}m ${total_sec}s"
  log "  Logs: ${LOG_DIR}/"
  log "========================================="
}

# =============================================================================
# MAIN
# =============================================================================

main() {
  if [[ $# -eq 0 ]] || [[ "$1" == "--help" ]] || [[ "$1" == "-h" ]]; then
    usage
  fi

  EPIC_FILE="$1"
  shift

  PROJECT_DIR="$(pwd)"

  # Always resolve EPIC_FILE to absolute path
  if [[ -f "$EPIC_FILE" ]]; then
    EPIC_FILE="$(cd "$(dirname "$EPIC_FILE")" && pwd)/$(basename "$EPIC_FILE")"
  else
    # File doesn't exist yet — resolve to project root (where fry-prepare.sh creates it)
    EPIC_FILE="${PROJECT_DIR}/$(basename "$EPIC_FILE")"
  fi

  local positional=()
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --dry-run)
        DRY_RUN=1
        shift
        ;;
      --engine)
        if [[ -z "${2:-}" ]]; then
          echo "ERROR: --engine requires a value (codex or claude)."
          exit 1
        fi
        CLI_ENGINE="$2"
        shift 2
        ;;
      --prepare-engine)
        if [[ -z "${2:-}" ]]; then
          echo "ERROR: --prepare-engine requires a value (codex or claude)."
          exit 1
        fi
        CLI_PREPARE_ENGINE="$2"
        shift 2
        ;;
      *)
        positional+=("$1")
        shift
        ;;
    esac
  done

  LOG_DIR="${PROJECT_DIR}/build-logs"
  TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
  BUILD_START_TIME=$(date +%s)
  WORK_DIR=$(mktemp -d "${TMPDIR:-/tmp}/fry.XXXXXX")

  trap 'on_exit' EXIT
  mkdir -p "$LOG_DIR"

  # --- Lockfile to prevent concurrent runs (skip for dry-run) ---
  if [[ $DRY_RUN -eq 0 ]]; then
    LOCK_FILE="${PROJECT_DIR}/.fry.lock"
    if [[ -f "$LOCK_FILE" ]]; then
      local lock_pid
      lock_pid=$(cat "$LOCK_FILE" 2>/dev/null)
      if [[ -n "$lock_pid" ]] && kill -0 "$lock_pid" 2>/dev/null; then
        echo "ERROR: Another fry.sh instance is running (PID ${lock_pid})."
        echo "If this is stale, remove ${LOCK_FILE} and retry."
        exit 1
      else
        echo "Removing stale lock file (PID ${lock_pid:-unknown} not running)."
        rm -f "$LOCK_FILE"
      fi
    fi
    echo $$ > "$LOCK_FILE"
  fi

  # --- Auto-generate epic if it doesn't exist ---
  if [[ ! -f "$EPIC_FILE" ]]; then
    local script_dir
    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    local prepare_script="${script_dir}/fry-prepare.sh"
    local epic_basename
    epic_basename="$(basename "$EPIC_FILE")"

    if [[ -x "$prepare_script" ]] || [[ -f "$prepare_script" ]]; then
      log "Epic file not found: ${epic_basename}"
      log "Running fry-prepare.sh to generate it from plans/plan.md..."
      echo ""

      local -a prepare_args=("$epic_basename")
      # Determine which engine to pass to fry-prepare.sh
      local prep_engine="${CLI_PREPARE_ENGINE:-${CLI_ENGINE:-${FRY_ENGINE:-}}}"
      if [[ -n "$prep_engine" ]]; then
        prepare_args+=(--engine "$prep_engine")
      fi

      bash "$prepare_script" "${prepare_args[@]}"
      local prep_exit=$?

      if [[ $prep_exit -ne 0 ]]; then
        log "ERROR: Epic generation failed (exit ${prep_exit})."
        exit 1
      fi

      if [[ -f "$EPIC_FILE" ]]; then
        log "Generated: ${EPIC_FILE}"
        echo ""
      else
        log "ERROR: fry-prepare.sh ran but ${epic_basename} was not created."
        exit 1
      fi
    else
      echo "ERROR: Epic file '${EPIC_FILE}' not found."
      echo "  Either create it manually or place fry-prepare.sh alongside this script"
      echo "  to auto-generate it from plans/plan.md."
      exit 1
    fi
  fi

  parse_epic "$EPIC_FILE"
  resolve_engine
  validate_epic

  START_SPRINT="${positional[0]:-1}"
  END_SPRINT="${positional[1]:-$TOTAL_SPRINTS}"

  if ! [[ "$START_SPRINT" =~ ^[0-9]+$ ]] || ! [[ "$END_SPRINT" =~ ^[0-9]+$ ]]; then
    echo "ERROR: Sprint numbers must be positive integers (got: ${START_SPRINT}, ${END_SPRINT})."
    exit 1
  fi
  if [[ $START_SPRINT -lt 1 ]]; then
    echo "ERROR: Start sprint must be >= 1 (got: ${START_SPRINT})."
    exit 1
  fi
  if [[ $END_SPRINT -gt $TOTAL_SPRINTS ]]; then
    echo "ERROR: End sprint ${END_SPRINT} exceeds max sprint ${TOTAL_SPRINTS} in epic."
    exit 1
  fi
  if [[ $START_SPRINT -gt $END_SPRINT ]]; then
    echo "ERROR: Start sprint (${START_SPRINT}) is after end sprint (${END_SPRINT})."
    exit 1
  fi

  if [[ $DRY_RUN -eq 1 ]]; then
    dry_run_report
    exit 0
  fi

  trap 'on_signal SIGINT' INT
  trap 'on_signal SIGTERM' TERM

  log "========================================="
  log "${EPIC_NAME:-Ralph Build}"
  log "Engine: ${ENGINE}"
  log "Sprints: ${START_SPRINT} through ${END_SPRINT}"
  log "Project: ${PROJECT_DIR}"
  log "Logs: ${LOG_DIR}"
  log "========================================="

  init_git
  preflight

  for sprint_num in $(seq "$START_SPRINT" "$END_SPRINT"); do
    run_sprint "$sprint_num"

    if [[ "${SPRINT_RESULTS[$sprint_num]:-}" == FAIL* ]]; then
      log ""
      log "Build stopped at Sprint ${sprint_num}."
      log "Fix the issue, then resume: $0 ${EPIC_FILE} ${sprint_num}"
      print_summary
      exit 1
    fi
  done

  print_summary
  log "ALL SPRINTS COMPLETE!"
}

main "$@"
