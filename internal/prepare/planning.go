package prepare

import (
	"fmt"

	"github.com/yevgetman/fry/internal/epic"
)

func PlanningStep0Prompt(executiveContent string) string {
	return fmt.Sprintf(`You are a senior strategic planner. Your job is to produce a detailed, actionable planning document.

Read plans/executive.md carefully — it contains the executive context for this project: vision, goals, target audience, scope, and constraints.

No detailed plan exists yet. The user is delegating ALL planning methodology, analytical framework, and structural decisions to you.

Generate a comprehensive planning document and write it to plans/plan.md.

Include:
1. **Planning Scope & Objectives**
2. **Analytical Frameworks**
3. **Research Requirements**
4. **Document Deliverables** — List every output document with its FULL path under plans/output/.
   Use ordered, categorized filenames following this convention:
     {sequence}--{category}--{name}.md
   Where:
   - {sequence} is a number (1, 2, 3...) indicating production order
   - {category} is a short lowercase grouping label (e.g., research, analysis, strategy, synthesis)
   - {name} is a descriptive lowercase kebab-case name
   Example: plans/output/1--research--market-landscape.md
   Group related documents under the same category. The sequence should reflect
   the logical dependency order (documents that inform later ones come first).
5. **Analytical Depth Requirements**
6. **Cross-Document Dependencies**
7. **Quality Standards**
8. **Constraints & Boundaries**

CRITICAL:
- Make DECISIVE choices.
- Be SPECIFIC. Include exact document names, required section headings, quantitative requirements.
- ALL output documents MUST use paths under plans/output/ — never write deliverables directly to plans/.
  The plans/ directory is reserved for input files (executive.md, plan.md).
- Align every decision with the goals, constraints, and scope defined in plans/executive.md.
- Write the file directly to plans/plan.md. No other output.
- The output format should be markdown.

Executive context:
%s
`, executiveContent)
}

func PlanningStep1Prompt(planContent, executiveContent string) string {
	contextLine := ""
	if executiveContent != "" {
		contextLine = "Also read plans/executive.md for executive context about the project's purpose, goals, and constraints.\n\n"
	}

	return fmt.Sprintf(`You are generating an AGENTS.md file for an autonomous AI planning agent.

Read plans/plan.md carefully — it contains the high-level plan for this project.
%sGenerate an AGENTS.md file and write it to .fry/AGENTS.md.

AGENTS.md is an operational rules file that the AI agent reads automatically at the start of every session. This is a PLANNING project — the agent produces structured documents, not code.

Rules should cover scope and domain boundaries, analytical frameworks and methodology, document quality standards, research and evidence standards, output file conventions, and explicit prohibitions.
Rules should be numbered, specific, and actionable. Each rule should be one line.
Write 15-40 rules total. Do NOT include vague rules.

One rule MUST state: "All document deliverables must be written to plans/output/ using the naming convention {sequence}--{category}--{name}.md. Never write output documents directly to plans/."

CRITICAL:
- Derive ALL rules from the plan document — do not invent rules not supported by the plan.
- This is a PLANNING project. The agent produces documents, analyses, and strategies — NOT code.
- Write the file directly to .fry/AGENTS.md. No other output.

Plan:
%s

Executive context:
%s
`, contextLine, planContent, executiveContent)
}

func PlanningStep2Prompt(planContent, agentsContent, epicExamplePath, userPrompt string, effort epic.EffortLevel) string {
	userPromptLine := ""
	if userPrompt != "" {
		userPromptLine = fmt.Sprintf("\nThe user has provided this top-level directive for the build: %q. Ensure sprint prompts align with this directive.\n", userPrompt)
	}

	effortGuidance := effortSizingGuidancePlanning(effort)

	return fmt.Sprintf(`You are generating an epic.md file for an autonomous AI planning system.

This is a PLANNING project — the AI agent produces structured documents, analyses, and strategies, NOT code. Each sprint delivers one or more written documents as its output.

Read these files carefully:
1. %s — The FORMAT REFERENCE showing the exact @directive syntax. Your output must use this syntax precisely. IGNORE the software-specific content in the examples — only follow the structural format.
2. plans/plan.md — The high-level plan to decompose into planning sprints. This is your primary source material.
3. .fry/AGENTS.md — Operational rules for the AI planning agent.

Generate the epic and write it to .fry/epic.md.
%s
CRITICAL RULES:
- Output ONLY the epic.md file content — write it directly to .fry/epic.md.
- Every @sprint block must have @name, @max_iterations, @promise, and @prompt.
- Sprint prompts must tell the agent to read .fry/AGENTS.md for operational rules.
- Sprint prompts must reference .fry/sprint-progress.txt and .fry/epic-progress.txt for progress tracking.
- ALL document deliverables MUST be written to the plans/output/ directory, NOT to plans/ directly.
  The plans/ directory is reserved for input files (executive.md, plan.md).
- Sprint prompts must specify exact output filenames using the ordered category convention:
    {sequence}--{category}--{name}.md
  Where:
  - {sequence} is a global sequence number across ALL sprints (not per-sprint). If Sprint 1
    produces documents 1-3 and Sprint 2 produces documents 4-5, the numbering is continuous.
  - {category} is a short lowercase grouping label (research, analysis, strategy, synthesis, etc.)
  - {name} is a descriptive lowercase kebab-case name
  Example paths: plans/output/1--research--market-landscape.md, plans/output/4--analysis--pricing-model.md
- Sprint prompts must include the required sections and concrete analytical requirements
  from plans/plan.md for each deliverable — never vague instructions.
- If plans/plan.md lists deliverables with paths that don't follow this convention, translate
  them to the correct convention in the sprint prompts.
- All deliverables are DOCUMENTS, not code.
- Sprint 1 is always research/discovery. The final sprint is always synthesis/review.
- Do NOT include any output other than writing the file. No explanations, no summaries.%s

Plan:
%s

AGENTS.md:
%s
`, epicExamplePath, effortGuidance, userPromptLine, planContent, agentsContent)
}

func effortSizingGuidancePlanning(effort epic.EffortLevel) string {
	switch effort {
	case epic.EffortLow:
		return `
EFFORT LEVEL: LOW
The user has indicated this is a low-effort planning task. You MUST:
- Generate AT MOST 2 sprints total
- Use max_iterations of 10-15 per sprint
- Write concise sprint prompts focused on essential deliverables
- Combine research and analysis into a single sprint where possible
- Add the @effort low directive to the epic header
`
	case epic.EffortMedium:
		return `
EFFORT LEVEL: MEDIUM
The user has indicated this is a medium-effort planning task. You MUST:
- Generate 2-4 sprints total (prefer the lower end)
- Use max_iterations of 15-25 per sprint
- Write moderately detailed sprint prompts
- Merge related analysis phases where practical
- Add the @effort medium directive to the epic header
`
	case epic.EffortHigh:
		return `
EFFORT LEVEL: HIGH
This is the standard effort level for planning. Follow all existing rules as-is.
- Generate 4-10 sprints as appropriate
- Use max_iterations of 15-35 per sprint
- Write fully detailed sprint prompts with comprehensive analytical requirements
- Add the @effort high directive to the epic header
`
	case epic.EffortMax:
		return `
EFFORT LEVEL: MAX
The user has indicated this is a maximum-effort, mission-critical planning task. You MUST:
- Generate the same number of sprints as HIGH effort (4-10)
- Use max_iterations of 30-50 per sprint (higher than normal)
- Write EXTENDED sprint prompts with additional analysis sections
- Include exhaustive research requirements and quality gates
- Add the @effort max directive to the epic header
- Enable @review_between_sprints and @compact_with_agent
`
	default: // auto-detect
		return `
EFFORT LEVEL: AUTO-DETECT
No effort level was specified. Analyze the plan document and determine the appropriate effort level:

- If the plan describes a simple, well-bounded task (1-2 documents): use LOW effort (1-2 sprints, @effort low)
- If the plan describes a moderate analysis (3-5 documents): use MEDIUM effort (2-4 sprints, @effort medium)
- If the plan describes a complex, multi-dimensional analysis (6+ documents): use HIGH effort (4-10 sprints, @effort high)

Add the @effort directive matching your assessment to the epic header.
Do NOT default to HIGH — genuinely evaluate the plan's complexity.
`
	}
}

func PlanningStep3Prompt(planContent, epicContent, verificationExamplePath, userPrompt string) string {
	userPromptLine := ""
	if userPrompt != "" {
		userPromptLine = fmt.Sprintf("\nThe user has provided this top-level directive: %q. If it affects what should or should not be verified, factor it in.\n", userPrompt)
	}

	return fmt.Sprintf(`You are generating a verification.md file for an autonomous AI planning system.

This is a PLANNING project — the AI agent produces structured documents, NOT code. Verification checks must validate that document deliverables exist and contain the required content.

Read these files carefully:
1. %s — The FORMAT REFERENCE showing exact syntax and check primitives. IGNORE the software-specific examples — adapt the primitives for document verification.
2. plans/plan.md — The high-level plan describing what is being planned.
3. .fry/epic.md — The sprint definitions.
4. .fry/AGENTS.md — Operational rules for the AI planning agent.

Generate the verification file and write it to .fry/verification.md.

CRITICAL RULES:
- Output ONLY the verification.md file content — write it directly to .fry/verification.md.
- Use ONLY these four check primitives: @check_file, @check_file_contains, @check_cmd, @check_cmd_output
- Every @sprint block in the epic must have a corresponding @sprint block in verification.md.
- Every check must be a concrete, executable assertion. No prose. No subjective criteria.
- Verify documents exist, contain required section headings, include key terminology and frameworks specified in the sprint prompt, and meet minimum depth requirements.
- All document deliverable paths in @check_file and @check_file_contains directives must reference
  the plans/output/ directory (e.g., plans/output/1--research--market-landscape.md).
  Do NOT reference plans/ directly for output documents.
- Do NOT write checks for deliverables from earlier sprints — only check the current sprint's new documents.
- Do NOT include any output other than writing the file. No explanations, no summaries.%s

Plan:
%s

Epic:
%s
`, verificationExamplePath, userPromptLine, planContent, epicContent)
}
