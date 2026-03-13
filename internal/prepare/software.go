package prepare

import (
	"fmt"

	"github.com/yevgetman/fry/internal/epic"
)

func mediaSection(mediaManifest string) string {
	if mediaManifest == "" {
		return ""
	}
	return fmt.Sprintf(`
A media/ directory exists with the following assets. Reference these in the plan where appropriate — the build agent will copy or use them as needed:

%s`, mediaManifest)
}

func SoftwareStep0Prompt(executiveContent, mediaManifest string) string {
	return fmt.Sprintf(`You are a senior software architect. Your job is to produce a detailed, actionable build plan.

Read plans/executive.md carefully — it contains the executive context for this project: vision, goals, target users, scope, and constraints.

No detailed build plan exists yet. The user is delegating ALL design, architecture, and implementation decisions to you. You have full authority to choose:
- Language, runtime, and framework
- Database and storage technology
- Project structure and directory layout
- API design and data models
- Testing strategy and tools
- Build and deployment approach
- Any other technical decisions

Generate a comprehensive build plan and write it to plans/plan.md.

The plan should contain enough detail for an AI coding agent to decompose it into implementation sprints. Include:

1. **Technology Stack** — Specific language/runtime versions, frameworks, databases, key libraries. Make definitive choices not wishy-washy options.
2. **Architecture** — Directory structure, module boundaries, key patterns, how components connect.
3. **Data Model** — Database tables or collections with column names, types, constraints, indexes, and relationships. Be explicit.
4. **API Design** — Endpoints or equivalent interfaces with methods, paths, request/response shapes, authentication approach, error format, pagination.
5. **Business Logic** — Core algorithms, validation rules, processing flows, state machines.
6. **Testing Strategy** — Test framework, what gets unit-tested vs integration-tested, required coverage patterns, test data approach.
7. **Configuration & Infrastructure** — Environment variables, Docker setup, build commands, CI/CD approach if relevant.
8. **Constraints & Non-Negotiables** — Security requirements, performance targets, compliance needs, scope boundaries.

CRITICAL:
- Make DECISIVE choices. Do not present alternatives.
- Be SPECIFIC. Include exact file paths, function signatures, table schemas, endpoint definitions.
- Align every decision with the goals, constraints, and scope defined in plans/executive.md.
- If a media/ directory exists, reference the available assets by their paths (e.g., media/logo.png) in the relevant sections of the plan. Describe where and how each asset should be used.
- Write the file directly to plans/plan.md. No other output.
- The output format should be markdown.

Executive context:
%s
%s`, executiveContent, mediaSection(mediaManifest))
}

func SoftwareStep1Prompt(planContent, executiveContent, mediaManifest string) string {
	contextLine := ""
	if executiveContent != "" {
		contextLine = "Also read plans/executive.md for executive context about the project's purpose, goals, and business constraints.\n\n"
	}

	return fmt.Sprintf(`You are generating an AGENTS.md file for an autonomous AI coding agent.

Read plans/plan.md carefully — it contains the holistic build plan for this project.
%sGenerate an AGENTS.md file and write it to .fry/AGENTS.md.

AGENTS.md is an operational rules file that the AI agent reads automatically at the start of every session. It should contain:

1. **Project Overview** — 2-3 sentences: what this project is, what language/framework it uses.
2. **Technology Constraints** — Specific, non-negotiable rules derived from the plan.
3. **Architecture Rules** — Structural patterns from the plan.
4. **Testing Rules** — How tests should be written.
5. **Coding Patterns** — Specific patterns to follow.
6. **Do NOT** — Explicit prohibitions.

Rules should be numbered, specific, and actionable. Each rule should be one line.
Write 15-40 rules total. Do NOT include vague rules like 'write clean code.'

CRITICAL:
- Derive ALL rules from the plan document — do not invent rules not supported by the plan.
- If a media/ directory exists, include a rule that the agent should use assets from media/ as specified in the plan.
- Write the file directly to .fry/AGENTS.md. No other output.

Plan:
%s

Executive context:
%s
%s`, contextLine, planContent, executiveContent, mediaSection(mediaManifest))
}

func SoftwareStep2Prompt(planContent, agentsContent, epicExamplePath, generateEpicPath, userPrompt string, effort epic.EffortLevel, mediaManifest string) string {
	userPromptLine := ""
	if userPrompt != "" {
		userPromptLine = fmt.Sprintf("\nThe user has provided this top-level directive for the build: %q. Ensure sprint prompts align with this directive.\n", userPrompt)
	}

	effortGuidance := effortSizingGuidance(effort)

	return fmt.Sprintf(`You are generating an epic.md file for an autonomous AI build system.

Read these files carefully:
1. %s — Contains your full instructions for how to decompose a plan into sprints. Follow every rule in this document.
2. %s — The FORMAT REFERENCE showing exact syntax and structure. Your output must match this format precisely.
3. plans/plan.md — The build plan to decompose into sprints. This is your primary source material.
4. .fry/AGENTS.md — Operational rules that apply to the project.

Generate the epic and write it to .fry/epic.md.
%s
CRITICAL RULES:
- Output ONLY the epic.md file content — write it directly to .fry/epic.md.
- The file must start with a # comment header and @epic directive.
- Every @sprint block must have @name, @max_iterations, @promise, and @prompt.
- Every @prompt must follow the 7-part structure defined in GENERATE_EPIC.md.
- Sprint prompts must reference specific filenames, function signatures, and concrete details from plans/plan.md — never vague instructions.
- Sprint prompts must tell the agent to read .fry/AGENTS.md for operational rules.
- Sprint prompts must reference .fry/sprint-progress.txt and .fry/epic-progress.txt for progress tracking.
- Verification references should point to .fry/verification.md.
- Sprint 1 is always scaffolding. The final sprint is always wiring + integration + E2E.
- The @promise token inside the prompt text must match the @promise directive value.
- If media assets exist, sprint prompts that need those assets must instruct the agent to copy or reference them from the media/ directory.
- Do NOT include any output other than writing the file. No explanations, no summaries.%s

Plan:
%s

AGENTS.md:
%s
%s`, generateEpicPath, epicExamplePath, effortGuidance, userPromptLine, planContent, agentsContent, mediaSection(mediaManifest))
}

func effortSizingGuidance(effort epic.EffortLevel) string {
	switch effort {
	case epic.EffortLow:
		return `
EFFORT LEVEL: LOW
The user has indicated this is a low-effort task. You MUST:
- Generate AT MOST 2 sprints total
- Use max_iterations of 10-15 per sprint
- Write concise sprint prompts — skip the REFERENCES and STUCK HINT sections
- Combine all work into 1-2 dense but focused sprints
- Skip scaffolding as a separate sprint — include it in Sprint 1's build list
- Focus only on the core deliverables; omit exhaustive edge cases
- Add the @effort low directive to the epic header

If the plan is genuinely trivial (single file, simple config), use exactly 1 sprint.
`
	case epic.EffortMedium:
		return `
EFFORT LEVEL: MEDIUM
The user has indicated this is a medium-effort task. You MUST:
- Generate 2-4 sprints total (prefer the lower end)
- Use max_iterations of 15-25 per sprint
- Write moderately detailed sprint prompts — include all 7 parts but keep them concise
- Merge layers that would be separate at HIGH effort (e.g., combine schema + domain types)
- Include essential edge cases but don't be exhaustive
- Add the @effort medium directive to the epic header
`
	case epic.EffortHigh:
		return `
EFFORT LEVEL: HIGH
This is the standard effort level. Follow all existing epic generation rules as-is.
- Generate 4-10 sprints as appropriate
- Use max_iterations of 15-35 per sprint per the standard sizing guidelines
- Write fully detailed 7-part sprint prompts
- Include comprehensive edge cases and verification
- Add the @effort high directive to the epic header
`
	case epic.EffortMax:
		return `
EFFORT LEVEL: MAX
The user has indicated this is a maximum-effort, mission-critical task. You MUST:
- Generate the same number of sprints as HIGH effort (4-10)
- Use max_iterations of 30-50 per sprint (higher than normal)
- Write EXTENDED sprint prompts that go beyond the standard 7-part structure:
  - Add an 8th section: "ANALYSIS & EDGE CASES" — enumerate every edge case, race condition,
    error scenario, and boundary condition relevant to this sprint
  - Add a 9th section: "QUALITY GATES" — explicit quality criteria beyond verification checks
    (performance targets, security considerations, code review checklist items)
- Include exhaustive edge cases, error handling requirements, and defensive coding instructions
- Specify exact error messages, log formats, and observability requirements
- Add the @effort max directive to the epic header
- Enable @review_between_sprints and @compact_with_agent
- Set @max_heal_attempts to 5 (increased from default 3)
`
	default: // auto-detect
		return `
EFFORT LEVEL: AUTO-DETECT
No effort level was specified. Analyze the plan document and determine the appropriate effort level:

- If the plan describes a simple, well-bounded task (single page, config change, small utility,
  1-3 files to create/modify): use LOW effort (1-2 sprints, @effort low)
- If the plan describes a moderate feature (multiple components, some integration,
  4-15 files): use MEDIUM effort (2-4 sprints, @effort medium)
- If the plan describes a complex system (many components, database, APIs, extensive
  testing, 15+ files): use HIGH effort (4-10 sprints, @effort high)

Add the @effort directive matching your assessment to the epic header.
Do NOT default to HIGH — genuinely evaluate the plan's complexity.

Common over-engineering signals to watch for:
- Creating separate scaffolding sprints for projects that need no scaffolding
- Splitting 3-file changes across 3+ sprints
- Adding schema/migration sprints for projects with no database
- Creating separate "wiring" sprints for simple, flat architectures
`
	}
}

func SoftwareStep3Prompt(planContent, epicContent, verificationExamplePath, userPrompt, mediaManifest string) string {
	userPromptLine := ""
	if userPrompt != "" {
		userPromptLine = fmt.Sprintf("\nThe user has provided this top-level directive: %q. If it affects what should or should not be verified, factor it in.\n", userPrompt)
	}

	return fmt.Sprintf(`You are generating a verification.md file for an autonomous AI build system.

Read these files carefully:
1. %s — The FORMAT REFERENCE showing exact syntax and check primitives. Your output must match this format precisely.
2. plans/plan.md — The build plan describing what is being built. Derive checks from the concrete deliverables described here.
3. .fry/epic.md — The sprint definitions. Each sprint block tells you what files and features that sprint creates. Write checks that verify those specific deliverables.
4. .fry/AGENTS.md — Operational rules that apply to the project.

Generate the verification file and write it to .fry/verification.md.

CRITICAL RULES:
- Output ONLY the verification.md file content — write it directly to .fry/verification.md.
- Use ONLY these four check primitives: @check_file, @check_file_contains, @check_cmd, @check_cmd_output
- Every @sprint block in the epic must have a corresponding @sprint block in verification.md.
- Every check must be a concrete, executable assertion. No prose. No subjective criteria.
- Derive checks from SPECIFIC deliverables in the plan and epic: exact filenames, build commands, required config values, API endpoints.
- Do NOT write checks for things that earlier sprints already verified — only check the current sprint's new deliverables. Cumulative checks are fine.
- If media assets are referenced in the plan, add @check_file checks to verify they were copied to their target locations.
- Do NOT include any output other than writing the file. No explanations, no summaries.%s

Plan:
%s

Epic:
%s
%s`, verificationExamplePath, userPromptLine, planContent, epicContent, mediaSection(mediaManifest))
}
