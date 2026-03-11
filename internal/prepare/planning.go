package prepare

import "fmt"

func PlanningStep0Prompt(executiveContent string) string {
	return fmt.Sprintf(`You are a senior strategic planner. Your job is to produce a detailed, actionable planning document.

Read plans/executive.md carefully — it contains the executive context for this project: vision, goals, target audience, scope, and constraints.

No detailed plan exists yet. The user is delegating ALL planning methodology, analytical framework, and structural decisions to you.

Generate a comprehensive planning document and write it to plans/plan.md.

Include:
1. **Planning Scope & Objectives**
2. **Analytical Frameworks**
3. **Research Requirements**
4. **Document Deliverables**
5. **Analytical Depth Requirements**
6. **Cross-Document Dependencies**
7. **Quality Standards**
8. **Constraints & Boundaries**

CRITICAL:
- Make DECISIVE choices.
- Be SPECIFIC. Include exact document names, required section headings, quantitative requirements.
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

Rules should cover scope and domain boundaries, analytical frameworks and methodology, document quality standards, research and evidence standards, and explicit prohibitions.
Rules should be numbered, specific, and actionable. Each rule should be one line.
Write 15-40 rules total. Do NOT include vague rules.

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

func PlanningStep2Prompt(planContent, agentsContent, epicExamplePath, userPrompt string) string {
	userPromptLine := ""
	if userPrompt != "" {
		userPromptLine = fmt.Sprintf("\nThe user has provided this top-level directive for the build: %q. Ensure sprint prompts align with this directive.\n", userPrompt)
	}

	return fmt.Sprintf(`You are generating an epic.md file for an autonomous AI planning system.

This is a PLANNING project — the AI agent produces structured documents, analyses, and strategies, NOT code. Each sprint delivers one or more written documents as its output.

Read these files carefully:
1. %s — The FORMAT REFERENCE showing the exact @directive syntax. Your output must use this syntax precisely. IGNORE the software-specific content in the examples — only follow the structural format.
2. plans/plan.md — The high-level plan to decompose into planning sprints. This is your primary source material.
3. .fry/AGENTS.md — Operational rules for the AI planning agent.

Generate the epic and write it to .fry/epic.md.

CRITICAL RULES:
- Output ONLY the epic.md file content — write it directly to .fry/epic.md.
- Every @sprint block must have @name, @max_iterations, @promise, and @prompt.
- Sprint prompts must tell the agent to read .fry/AGENTS.md for operational rules.
- Sprint prompts must reference .fry/sprint-progress.txt and .fry/epic-progress.txt for progress tracking.
- Sprint prompts must specify exact output filenames, required sections, and concrete analytical requirements from plans/plan.md — never vague instructions.
- All deliverables are DOCUMENTS, not code.
- Sprint 1 is always research/discovery. The final sprint is always synthesis/review.
- Do NOT include any output other than writing the file. No explanations, no summaries.%s

Plan:
%s

AGENTS.md:
%s
`, epicExamplePath, userPromptLine, planContent, agentsContent)
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
- Do NOT write checks for deliverables from earlier sprints — only check the current sprint's new documents.
- Do NOT include any output other than writing the file. No explanations, no summaries.%s

Plan:
%s

Epic:
%s
`, verificationExamplePath, userPromptLine, planContent, epicContent)
}
