#!/usr/bin/env bash
# =============================================================================
# fry-prepare.sh — Generate AGENTS.md, epic.md, and verification.md from plans/plan.md via AI
# =============================================================================
#
# Three-step preparation:
#   Step 1: Generate AGENTS.md from plan.md (+ executive.md if present)
#   Step 2: Generate epic.md from plan.md, AGENTS.md, and format references
#   Step 3: Generate verification.md from plan.md, epic.md, and format reference
#
# Can be run standalone or is called automatically by fry.sh when the
# specified epic file doesn't exist yet.
#
# Supports two AI engines:
#   codex (default): OpenAI Codex CLI
#   claude:          Claude Code CLI
#
# Required:
#   plans/plan.md              Holistic build plan (your source material)
#   epic-example.md            Format reference for epic generation
#   GENERATE_EPIC.md           Generation prompt/instructions
#   verification-example.md    Format reference for verification generation
#
# Optional:
#   plans/executive.md     Executive context (used if present, skipped if not)
#
# Usage:
#   ./fry-prepare.sh                           # Generate with codex (default)
#   ./fry-prepare.sh --engine claude           # Generate with Claude Code
#   ./fry-prepare.sh epic-phase1.md            # Custom epic filename
#   ./fry-prepare.sh epic.md --engine claude   # Custom filename + engine
#   ./fry-prepare.sh --validate-only           # Check prerequisites only
#   ./fry-prepare.sh --keep-agents             # Skip AGENTS.md if it exists
#   ./fry-prepare.sh --keep-epic               # Skip epic.md if it exists
#   ./fry-prepare.sh --keep-verification       # Skip verification.md if it exists
#
# Engine selection: --engine flag > FRY_ENGINE env var > default (codex)
#
# =============================================================================

set -uo pipefail

# =============================================================================
# CONSTANTS
# =============================================================================

readonly PLANS_DIR="plans"
readonly PLAN_FILE="${PLANS_DIR}/plan.md"
readonly CONTEXT_FILE="${PLANS_DIR}/executive.md"
readonly AGENTS_FILE="AGENTS.md"
readonly EXAMPLE_FILE="epic-example.md"
readonly PROMPT_REF="GENERATE_EPIC.md"
readonly VERIFICATION_EXAMPLE="verification-example.md"

# =============================================================================
# HELPERS
# =============================================================================

has_context() {
  [[ -f "$(pwd)/${CONTEXT_FILE}" ]]
}

# =============================================================================
# ENGINE DISPATCH
# =============================================================================

ENGINE="${FRY_ENGINE:-codex}"
KEEP_AGENTS=0
KEEP_EPIC=0
KEEP_VERIFICATION=0

# Execute a prompt via the configured AI engine.
# Usage: run_agent "prompt text"
run_agent() {
  local prompt="$1"
  case "$ENGINE" in
    codex)
      codex exec \
        --dangerously-bypass-approvals-and-sandbox \
        "$prompt"
      ;;
    claude)
      claude -p \
        --dangerously-skip-permissions \
        "$prompt"
      ;;
    *)
      echo "ERROR: Unknown engine '${ENGINE}'. Must be 'codex' or 'claude'."
      return 1
      ;;
  esac
}

# =============================================================================
# STEP 1: Generate AGENTS.md
# =============================================================================

generate_agents() {
  if [[ $KEEP_AGENTS -eq 1 ]] && [[ -f "${AGENTS_FILE}" ]]; then
    echo "AGENTS.md already exists. Skipping generation (--keep-agents)."
    return 0
  fi

  echo "Step 1: Generating ${AGENTS_FILE} from ${PLAN_FILE} (engine: ${ENGINE})..."
  if has_context; then
    echo "  Sources: ${PLAN_FILE}, ${CONTEXT_FILE}"
  else
    echo "  Sources: ${PLAN_FILE}"
  fi
  echo ""

  local context_line=""
  if has_context; then
    context_line="Also read \`${CONTEXT_FILE}\` for executive context about the project's purpose, goals, and business constraints."
  fi

  run_agent \
    "You are generating an AGENTS.md file for an autonomous AI coding agent.

Read \`${PLAN_FILE}\` carefully — it contains the holistic build plan for this project.
${context_line}

Generate an AGENTS.md file and write it to \`${AGENTS_FILE}\` in the project root.

AGENTS.md is an operational rules file that the AI agent reads automatically at the start of every session. It should contain:

1. **Project Overview** — 2-3 sentences: what this project is, what language/framework it uses.

2. **Technology Constraints** — Specific, non-negotiable rules derived from the plan:
   - Language and runtime version (e.g., 'Go 1.22', 'Node 20', 'Python 3.12')
   - Database and ORM rules (e.g., 'Use raw SQL with pgx. NO ORMs.')
   - Framework choices (e.g., 'chi for HTTP routing', 'Express with Zod validation')
   - Package/dependency rules (e.g., 'No lodash — use native methods')

3. **Architecture Rules** — Structural patterns from the plan:
   - Directory structure conventions
   - Naming conventions (files, packages, variables)
   - Multi-tenancy rules (e.g., 'Every query must include project_id')
   - API conventions (versioning, error format, pagination style)

4. **Testing Rules** — How tests should be written:
   - Test framework and assertion library
   - Unit vs integration test location/tagging
   - Required test coverage patterns

5. **Coding Patterns** — Specific patterns to follow:
   - Error handling conventions
   - Logging approach
   - Configuration loading pattern

6. **Do NOT** — Explicit prohibitions:
   - Things the agent commonly gets wrong for this stack
   - Anti-patterns specific to this project

Rules should be numbered, specific, and actionable. Each rule should be one line.
Write 15-40 rules total. Do NOT include vague rules like 'write clean code.'

CRITICAL:
- Derive ALL rules from the plan document — do not invent rules not supported by the plan.
- Write the file directly to \`${AGENTS_FILE}\`. No other output."

  local exit_code=$?

  if [[ $exit_code -ne 0 ]]; then
    echo "ERROR: AGENTS.md generation failed (exit ${exit_code})."
    return 1
  fi

  if [[ ! -f "${AGENTS_FILE}" ]]; then
    echo "ERROR: Agent did not produce ${AGENTS_FILE}."
    return 1
  fi

  local rule_count
  rule_count=$(grep -cE '^[0-9]+\.' "${AGENTS_FILE}" 2>/dev/null || echo "0")
  echo "Generated ${AGENTS_FILE} (${rule_count} numbered rules)."
  echo ""
}

# =============================================================================
# STEP 2: Generate epic.md
# =============================================================================

generate_epic() {
  local output_file=$1

  if [[ $KEEP_EPIC -eq 1 ]] && [[ -f "${output_file}" ]]; then
    echo "${output_file} already exists. Skipping generation (--keep-epic)."
    return 0
  fi

  echo "Step 2: Generating ${output_file} from ${PLAN_FILE} (engine: ${ENGINE})..."
  echo "  Plan:      ${PLAN_FILE}"
  has_context && echo "  Context:   ${CONTEXT_FILE}"
  echo "  Agents:    ${AGENTS_FILE}"
  echo "  Reference: ${EXAMPLE_FILE}"
  echo "  Prompt:    ${PROMPT_REF}"
  echo ""

  local context_line
  if has_context; then
    context_line="4. \`${CONTEXT_FILE}\` — Executive context for understanding the project's purpose and scope."
  else
    context_line="4. (No executive context file present — derive project understanding from the plan and AGENTS.md.)"
  fi

  run_agent \
    "You are generating an epic.md file for an autonomous AI build system.

Read these files carefully:
1. \`${PROMPT_REF}\` — Contains your full instructions for how to decompose a plan into sprints. Follow every rule in this document.
2. \`${EXAMPLE_FILE}\` — The FORMAT REFERENCE showing exact syntax and structure. Your output must match this format precisely.
3. \`${PLAN_FILE}\` — The build plan to decompose into sprints. This is your primary source material.
${context_line}
5. \`${AGENTS_FILE}\` — Operational rules that apply to the project.

Generate the epic and write it to \`${output_file}\` in the project root.

CRITICAL RULES:
- Output ONLY the epic.md file content — write it directly to \`${output_file}\`.
- The file must start with a # comment header and @epic directive.
- Every @sprint block must have @name, @max_iterations, @promise, and @prompt.
- Every @prompt must follow the 7-part structure defined in ${PROMPT_REF}.
- Sprint prompts must reference specific filenames, function signatures, and concrete details from ${PLAN_FILE} — never vague instructions.
- Sprint 1 is always scaffolding. The final sprint is always wiring + integration + E2E.
- The @promise token inside the prompt text must match the @promise directive value.
- Do NOT include any output other than writing the file. No explanations, no summaries."

  local exit_code=$?

  if [[ $exit_code -ne 0 ]]; then
    echo ""
    echo "ERROR: Agent exited with code ${exit_code}."
    echo "Check that the ${ENGINE} CLI is authenticated and working."
    exit 1
  fi

  if [[ ! -f "${output_file}" ]]; then
    echo ""
    echo "ERROR: Agent did not produce ${output_file}."
    echo "Try running again or generate manually using GENERATE_EPIC.md."
    exit 1
  fi

  local sprint_count
  sprint_count=$(grep -c '^@sprint ' "${output_file}" 2>/dev/null || echo "0")

  if [[ "$sprint_count" -eq 0 ]]; then
    echo ""
    echo "ERROR: Generated ${output_file} contains no @sprint blocks."
    echo "The file may be malformed. Review it manually or regenerate."
    exit 1
  fi

  echo ""
  echo "Generated ${output_file} with ${sprint_count} sprints."
  echo "Review it before running: cat ${output_file}"
  echo "Validate with: ./fry.sh ${output_file} --dry-run"
}

# =============================================================================
# STEP 3: Generate verification.md
# =============================================================================

generate_verification() {
  local epic_file=$1
  local vfile="verification.md"

  if [[ $KEEP_VERIFICATION -eq 1 ]] && [[ -f "${vfile}" ]]; then
    echo "${vfile} already exists. Skipping generation (--keep-verification)."
    return 0
  fi

  echo "Step 3: Generating ${vfile} from ${PLAN_FILE} + ${epic_file} (engine: ${ENGINE})..."
  echo "  Plan:      ${PLAN_FILE}"
  echo "  Epic:      ${epic_file}"
  echo "  Reference: ${VERIFICATION_EXAMPLE}"
  echo ""

  local context_line=""
  if has_context; then
    context_line="Also read \`${CONTEXT_FILE}\` for executive context about the project."
  fi

  run_agent \
    "You are generating a verification.md file for an autonomous AI build system.

Read these files carefully:
1. \`${VERIFICATION_EXAMPLE}\` — The FORMAT REFERENCE showing exact syntax and check primitives. Your output must match this format precisely.
2. \`${PLAN_FILE}\` — The build plan describing what is being built. Derive checks from the concrete deliverables described here.
3. \`${epic_file}\` — The sprint definitions. Each sprint block tells you what files and features that sprint creates. Write checks that verify those specific deliverables.
4. \`${AGENTS_FILE}\` — Operational rules that apply to the project.
${context_line}

Generate the verification file and write it to \`${vfile}\` in the project root.

CRITICAL RULES:
- Output ONLY the verification.md file content — write it directly to \`${vfile}\`.
- Use ONLY these four check primitives: @check_file, @check_file_contains, @check_cmd, @check_cmd_output
- Every @sprint block in the epic must have a corresponding @sprint block in verification.md.
- Every check must be a concrete, executable assertion. No prose. No subjective criteria.
- @check_cmd commands must exit 0 on success and non-0 on failure.
- @check_cmd_output uses a pipe (|) to separate the command from the grep pattern.
- Derive checks from SPECIFIC deliverables in the plan and epic: exact filenames, build commands, required config values, API endpoints.
- Do NOT write checks for things that earlier sprints already verified — only check the current sprint's new deliverables. Cumulative checks (like 'npm run build') are fine since they validate nothing is broken.
- Do NOT include any output other than writing the file. No explanations, no summaries."

  local exit_code=$?

  if [[ $exit_code -ne 0 ]]; then
    echo ""
    echo "WARNING: Verification file generation failed (exit ${exit_code})."
    echo "Continuing without verification.md — promise-only behavior will be used."
    return 0
  fi

  if [[ ! -f "${vfile}" ]]; then
    echo ""
    echo "WARNING: Agent did not produce ${vfile}."
    echo "Continuing without verification.md — promise-only behavior will be used."
    return 0
  fi

  local check_count
  check_count=$(grep -c '^@check_' "${vfile}" 2>/dev/null || echo "0")

  if [[ "$check_count" -eq 0 ]]; then
    echo ""
    echo "WARNING: Generated ${vfile} contains no @check_* directives."
    echo "The file may be malformed. Review it manually or regenerate."
    echo "Continuing without effective verification checks."
    return 0
  fi

  local sprint_count
  sprint_count=$(grep -c '^@sprint ' "${vfile}" 2>/dev/null || echo "0")

  echo ""
  echo "Generated ${vfile} with ${sprint_count} sprint blocks and ${check_count} checks."
}

# =============================================================================
# MAIN
# =============================================================================

main() {
  local output_file="epic.md"
  local validate_only=0

  # Parse arguments
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --validate-only)
        validate_only=1
        shift
        ;;
      --engine)
        if [[ -z "${2:-}" ]]; then
          echo "ERROR: --engine requires a value (codex or claude)."
          exit 1
        fi
        ENGINE="$2"
        shift 2
        ;;
      --keep-agents)
        KEEP_AGENTS=1
        shift
        ;;
      --keep-epic)
        KEEP_EPIC=1
        shift
        ;;
      --keep-verification)
        KEEP_VERIFICATION=1
        shift
        ;;
      *)
        output_file="$1"
        shift
        ;;
    esac
  done

  # --- Check prerequisites ---
  local failed=0

  for f in "$PLAN_FILE" "$EXAMPLE_FILE" "$PROMPT_REF" "$VERIFICATION_EXAMPLE"; do
    if [[ ! -f "${f}" ]]; then
      echo "ERROR: Required file missing: ${f}"
      failed=1
    fi
  done

  # Optional: executive.md
  if has_context; then
    echo "Found: ${CONTEXT_FILE}"
  else
    echo "Note:  ${CONTEXT_FILE} not found (optional — using ${PLAN_FILE} only for context)"
  fi

  # Optional: AGENTS.md (generated in Step 1 if missing)
  if [[ -f "${AGENTS_FILE}" ]]; then
    echo "Found: ${AGENTS_FILE}"
  else
    echo "Note:  ${AGENTS_FILE} not found (will be generated in Step 1)"
  fi

  # AI engine CLI
  case "$ENGINE" in
    codex)
      if ! command -v codex &> /dev/null; then
        echo "ERROR: 'codex' CLI not found. Install: npm i -g @openai/codex"
        failed=1
      fi
      ;;
    claude)
      if ! command -v claude &> /dev/null; then
        echo "ERROR: 'claude' CLI not found. Install: npm i -g @anthropic-ai/claude-code"
        failed=1
      fi
      ;;
    *)
      echo "ERROR: Unknown engine '${ENGINE}'. Must be 'codex' or 'claude'."
      failed=1
      ;;
  esac

  if [[ $failed -ne 0 ]]; then
    echo ""
    echo "Required files:"
    echo "  ${PLAN_FILE}              Your build plan"
    echo "  ${EXAMPLE_FILE}          Format reference (ships with fry.sh)"
    echo "  ${PROMPT_REF}          Generation instructions (ships with fry.sh)"
    echo "  ${VERIFICATION_EXAMPLE}  Verification format reference (ships with fry.sh)"
    echo ""
    echo "Optional files:"
    echo "  ${CONTEXT_FILE}        Executive context (project vision/goals)"
    echo "  ${AGENTS_FILE}                 Operational rules (auto-generated if missing)"
    exit 1
  fi

  if [[ $validate_only -eq 1 ]]; then
    echo "All prerequisites present. Ready to generate (engine: ${ENGINE})."
    exit 0
  fi

  echo ""

  # --- Step 1: Generate AGENTS.md (if missing) ---
  if ! generate_agents; then
    exit 1
  fi

  # --- Step 2: Generate epic.md ---
  generate_epic "$output_file"

  # --- Step 3: Generate verification.md ---
  if [[ -f "$output_file" ]]; then
    generate_verification "$output_file"
  fi
}

main "$@"
