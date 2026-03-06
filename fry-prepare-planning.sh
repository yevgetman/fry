#!/usr/bin/env bash
# =============================================================================
# fry-prepare-planning.sh — Generate AGENTS.md, epic.md, and verification.md
#                           for planning projects (non-code output)
# =============================================================================
#
# A planning-domain fork of fry-prepare.sh. Instead of generating sprints
# for building software, this generates sprints for producing structured
# planning documents — business plans, trip itineraries, research reports,
# strategic analyses, or any endeavor that requires rigorous, phased
# document creation.
#
# The output files use the same @sprint/@prompt/@promise format consumed
# by fry.sh. The execution engine is identical — only the generation
# prompts differ.
#
# Three-step preparation:
#   Step 1: Generate AGENTS.md — operational rules for the AI planning agent
#   Step 2: Generate epic.md — sprint phases producing document deliverables
#   Step 3: Generate verification.md — checks that deliverables exist and
#           contain required content
#
# Required:
#   plans/plan.md              High-level plan (your source material)
#   epic-example.md            Format reference for epic syntax
#   verification-example.md    Format reference for verification syntax
#
# Optional:
#   plans/executive.md     Executive context (vision, goals, constraints)
#
# Usage:
#   ./fry-prepare-planning.sh                           # Generate with codex
#   ./fry-prepare-planning.sh --engine claude           # Generate with Claude
#   ./fry-prepare-planning.sh epic-phase1.md            # Custom epic filename
#   ./fry-prepare-planning.sh --validate-only           # Check prerequisites
#   ./fry-prepare-planning.sh --keep-agents             # Preserve AGENTS.md
#   ./fry-prepare-planning.sh --keep-epic               # Preserve epic.md
#   ./fry-prepare-planning.sh --keep-verification       # Preserve verification.md
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
readonly VERIFICATION_EXAMPLE="verification-example.md"
readonly AGENTS_PLACEHOLDER="# AGENTS.md — PLACEHOLDER"

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
# STEP 1: Generate AGENTS.md (planning domain)
# =============================================================================

generate_agents() {
  if [[ $KEEP_AGENTS -eq 1 ]] && [[ -f "${AGENTS_FILE}" ]]; then
    if head -1 "${AGENTS_FILE}" | grep -qF "$AGENTS_PLACEHOLDER"; then
      echo "AGENTS.md is a placeholder — generating despite --keep-agents."
    else
      echo "AGENTS.md already exists. Skipping generation (--keep-agents)."
      return 0
    fi
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
    context_line="Also read \`${CONTEXT_FILE}\` for executive context about the project's purpose, goals, and constraints."
  fi

  run_agent \
    "You are generating an AGENTS.md file for an autonomous AI planning agent.

Read \`${PLAN_FILE}\` carefully — it contains the high-level plan for this project.
${context_line}

Generate an AGENTS.md file and write it to \`${AGENTS_FILE}\` in the project root.

AGENTS.md is an operational rules file that the AI agent reads automatically at the start of every session. This is a PLANNING project — the agent produces structured documents, not code. The rules should contain:

1. **Project Overview** — 2-3 sentences: what this planning effort is about, what domain it covers, what the final deliverables are.

2. **Scope & Domain Boundaries** — Non-negotiable rules about what is in and out of scope:
   - Geographic, temporal, or market boundaries (e.g., 'Focus on US market only', 'Plan covers 2025-2027')
   - Domain constraints (e.g., 'Budget must not exceed \$500K', 'Only consider organic growth strategies')
   - Stakeholder boundaries (e.g., 'Primary audience is the board of directors', 'Written for a solo founder')

3. **Analytical Frameworks & Methodology** — Required approaches:
   - Specific frameworks to use (e.g., 'Use SWOT for competitive analysis', 'Apply MECE principle to all categorizations')
   - Research standards (e.g., 'Cite sources for all market data', 'Use conservative estimates for financial projections')
   - Quantitative requirements (e.g., 'Include 3-year financial projections', 'Provide at least 3 scenarios per risk')

4. **Document Quality Standards** — How deliverables should be structured:
   - Required sections per document type
   - Minimum depth expectations (e.g., 'Each strategy must include rationale, implementation steps, and success metrics')
   - Formatting conventions (e.g., 'Use markdown headers for sections', 'Include executive summary in documents over 1000 words')

5. **Research & Evidence Standards** — How claims should be supported:
   - Citation requirements
   - Data source preferences
   - Assumption labeling (e.g., 'Clearly mark all assumptions with [ASSUMPTION]')

6. **Do NOT** — Explicit prohibitions:
   - No unsupported claims or vague recommendations
   - No generic filler content (e.g., 'leverage synergies', 'best-in-class')
   - Domain-specific anti-patterns derived from the plan

Rules should be numbered, specific, and actionable. Each rule should be one line.
Write 15-40 rules total. Do NOT include vague rules like 'write good documents.'

CRITICAL:
- Derive ALL rules from the plan document — do not invent rules not supported by the plan.
- This is a PLANNING project. The agent produces documents, analyses, and strategies — NOT code.
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
# STEP 2: Generate epic.md (planning domain)
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
  echo "  Reference: ${EXAMPLE_FILE} (format syntax only)"
  echo ""

  local context_line
  if has_context; then
    context_line="4. \`${CONTEXT_FILE}\` — Executive context for understanding the project's purpose and scope."
  else
    context_line="4. (No executive context file present — derive project understanding from the plan and AGENTS.md.)"
  fi

  run_agent \
    "You are generating an epic.md file for an autonomous AI planning system.

This is a PLANNING project — the AI agent produces structured documents, analyses, and strategies, NOT code. Each sprint delivers one or more written documents as its output.

Read these files carefully:
1. \`${EXAMPLE_FILE}\` — The FORMAT REFERENCE showing the exact @directive syntax (@sprint, @name, @max_iterations, @promise, @prompt). Your output must use this syntax precisely. IGNORE the software-specific content in the examples — only follow the structural format.
2. \`${PLAN_FILE}\` — The high-level plan to decompose into planning sprints. This is your primary source material.
3. \`${AGENTS_FILE}\` — Operational rules for the AI planning agent.
${context_line}

Generate the epic and write it to \`${output_file}\` in the project root.

## Sprint Decomposition for Planning Projects

Analyze the plan and produce 4-10 sequential sprints following this general phasing (adapt to the specific project):

| Sprint Position | Phase | Example Deliverables |
|---|---|---|
| 1 (always) | Research & Discovery | Landscape analysis, stakeholder mapping, data gathering, baseline assessment |
| 2-3 | Deep Analysis | Market analysis, risk assessment, competitive landscape, needs analysis, feasibility study |
| 4-6 | Strategy & Design | Strategic options, decision frameworks, resource plans, timelines, budgets |
| 7-8 | Detailed Planning | Implementation roadmaps, operational plans, contingency plans, metrics/KPIs |
| N (always last) | Synthesis & Review | Executive summary, integrated plan document, cross-reference validation, final recommendations |

## Sprint Prompt Structure

Each @prompt block MUST follow this 7-part structure:

**OPENER**: \"Sprint N: [What] for [Project].\"

**REFERENCES**: \"Read AGENTS.md first, then plans/plan.md [relevant sections].\"

**DELIVERABLES LIST**: Numbered list with EXACT specifications:
- Exact output file paths (e.g., \`plans/market-analysis.md\`, \`plans/budget-projections.md\`)
- Required sections within each document (e.g., \"Must include: Executive Summary, Market Size, Growth Trends, Key Players, Opportunities, Threats\")
- Specific analytical frameworks to apply (e.g., \"Use Porter's Five Forces for industry analysis\")
- Quantitative requirements (e.g., \"Include 3-year revenue projections with conservative, moderate, and aggressive scenarios\")
- Cross-references to earlier sprint deliverables where relevant

DO NOT write vague instructions like \"analyze the market.\"
DO write specific instructions like \"plans/market-analysis.md — TAM/SAM/SOM analysis for [target market]. Sections: Market Size (with sources), Growth Rate (CAGR), Customer Segments (minimum 3, with demographics and willingness-to-pay), Competitive Landscape (minimum 5 direct competitors with positioning map).\"

**CONSTRAINTS**: \"CRITICAL: [requirement that will undermine the plan if ignored].\"
These come from the plan — budget limits, geographic scope, timeline constraints, methodology requirements.

**VERIFICATION**: Reference verification.md for the concrete checks that fry.sh runs independently. Summarize key outcomes:
- \"Verification checks are defined in verification.md (sprint N).\"
- Brief bullets of what success looks like (document exists, contains required sections, meets minimum depth)

**STUCK HINT**: \"If stuck after N iterations: [most likely cause + fix].\"
Common planning pitfalls:
- Sprint 1: Scope creep — trying to analyze everything instead of gathering baseline data
- Analysis sprints: Going too deep on one area while neglecting others
- Strategy sprints: Being too vague or generic — force specific recommendations
- Final sprint: Not cross-referencing earlier deliverables

**PROMISE**: \"Output <promise>SPRINTN_DONE</promise> when [exit criteria].\"

## Global Directives

Do NOT include @docker_from_sprint, @docker_ready_cmd, @require_tool, or @pre_sprint unless the plan specifically requires external tools. Planning projects typically need no infrastructure directives.

CRITICAL RULES:
- Output ONLY the epic.md file content — write it directly to \`${output_file}\`.
- The file must start with a # comment header and @epic directive.
- Every @sprint block must have @name, @max_iterations, @promise, and @prompt.
- Every @prompt must follow the 7-part structure above.
- Sprint prompts must specify exact output filenames (in plans/ or a subdirectory), required sections, and concrete analytical requirements from ${PLAN_FILE} — never vague instructions.
- The @promise token inside the prompt text must match the @promise directive value.
- All deliverables are DOCUMENTS (markdown files), not code. The agent writes plans, analyses, strategies, and recommendations.
- Sprint 1 is always research/discovery. The final sprint is always synthesis/review.
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
    echo "Try running again or hand-author the epic using epic-example.md as a syntax reference."
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
# STEP 3: Generate verification.md (planning domain)
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
  echo "  Reference: ${VERIFICATION_EXAMPLE} (syntax only)"
  echo ""

  local context_line=""
  if has_context; then
    context_line="Also read \`${CONTEXT_FILE}\` for executive context about the project."
  fi

  run_agent \
    "You are generating a verification.md file for an autonomous AI planning system.

This is a PLANNING project — the AI agent produces structured documents, NOT code. Verification checks must validate that document deliverables exist and contain the required content.

Read these files carefully:
1. \`${VERIFICATION_EXAMPLE}\` — The FORMAT REFERENCE showing exact syntax and check primitives. Your output must use this syntax. IGNORE the software-specific examples — adapt the primitives for document verification.
2. \`${PLAN_FILE}\` — The high-level plan describing what is being planned. Derive checks from the concrete deliverables described here.
3. \`${epic_file}\` — The sprint definitions. Each sprint block specifies which documents and analyses that sprint produces. Write checks that verify those specific deliverables.
4. \`${AGENTS_FILE}\` — Operational rules for the AI planning agent.
${context_line}

Generate the verification file and write it to \`${vfile}\` in the project root.

## Verification Strategies for Planning Documents

Use the four check primitives to verify document deliverables:

**@check_file <path>** — Verify the document was created:
  @check_file plans/market-analysis.md
  @check_file plans/budget-projections.md

**@check_file_contains <path> <pattern>** — Verify required content and sections:
  @check_file_contains plans/market-analysis.md \"## Market Size\"
  @check_file_contains plans/market-analysis.md \"TAM|SAM|SOM\"
  @check_file_contains plans/budget-projections.md \"Year 1|Year 2|Year 3\"
  @check_file_contains plans/risk-assessment.md \"## Mitigation\"

**@check_cmd <command>** — Verify structural requirements:
  # Document has minimum depth (at least 500 words)
  @check_cmd test \$(wc -w < plans/market-analysis.md) -ge 500
  # Document has required number of sections
  @check_cmd test \$(grep -c '^## ' plans/market-analysis.md) -ge 4

**@check_cmd_output <command> | <pattern>** — Verify specific structural elements:
  # Has at least 5 second-level headings
  @check_cmd_output grep -c '^## ' plans/strategy.md | ^[5-9]
  # Contains quantitative data
  @check_cmd_output grep -c '[0-9]' plans/budget-projections.md | ^[1-9]

CRITICAL RULES:
- Output ONLY the verification.md file content — write it directly to \`${vfile}\`.
- Use ONLY these four check primitives: @check_file, @check_file_contains, @check_cmd, @check_cmd_output
- Every @sprint block in the epic must have a corresponding @sprint block in verification.md.
- Every check must be a concrete, executable assertion. No prose. No subjective criteria like 'analysis is thorough.'
- Verify documents exist, contain required section headings, include key terminology and frameworks specified in the sprint prompt, and meet minimum depth requirements.
- @check_cmd commands must exit 0 on success and non-0 on failure.
- @check_cmd_output uses a pipe (|) to separate the command from the grep pattern.
- Do NOT write checks for deliverables from earlier sprints — only check the current sprint's new documents. Cross-document checks (like verifying a final summary references earlier work) are fine in the final sprint.
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

  for f in "$PLAN_FILE" "$EXAMPLE_FILE" "$VERIFICATION_EXAMPLE"; do
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
    echo "  ${PLAN_FILE}              Your high-level plan"
    echo "  ${EXAMPLE_FILE}          Format reference for epic syntax (ships with fry)"
    echo "  ${VERIFICATION_EXAMPLE}  Format reference for verification syntax (ships with fry)"
    echo ""
    echo "Optional files:"
    echo "  ${CONTEXT_FILE}        Executive context (vision, goals, constraints)"
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
