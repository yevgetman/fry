package prepare

import (
	"fmt"

	"github.com/yevgetman/fry/internal/epic"
)

func WritingExecutiveFromUserPromptPrompt(userPrompt, mediaManifest, assetsSection string) string {
	return fmt.Sprintf(`You are a senior author and content strategist. The user has described a writing project in a brief prompt. Your job is to produce a well-structured executive context document that captures the project's vision, goals, and constraints.

User's project description:
%s

Generate a markdown document suitable for use as plans/executive.md. Include these sections:

1. **Project Name** — A clear, descriptive name for the writing project.
2. **Vision** — What is this project and why does it exist? (2-3 sentences)
3. **Goals** — Specific, measurable outcomes the project should achieve. (bulleted list)
4. **Target Audience** — Who will read this? What are their needs and expectations?
5. **Scope** — What is in scope and what is explicitly out of scope.
6. **Voice & Tone** — The desired writing style, register, and personality.
7. **Structural Approach** — High-level content organization (e.g., chronological, thematic, problem-solution).
8. **Constraints** — Length, format, deadline, subject-matter, or audience constraints.
9. **Success Criteria** — How will we know the writing project succeeded?

CRITICAL:
- Infer reasonable defaults from the user's description — do not ask questions.
- Be SPECIFIC and DECISIVE. Fill in concrete details based on what the user described.
- If the user's description is vague on a point, make a reasonable assumption and state it clearly.
- If supplementary asset documents are provided below, incorporate their context into your executive summary — they contain reference material that informs the project's scope, constraints, and goals.
- Do NOT write the file to disk — output ONLY the markdown content as your response.
- The output format should be markdown.
%s%s`, userPrompt, mediaSection(mediaManifest), assetsPromptBlock(assetsSection))
}

func WritingStep0Prompt(executiveContent, mediaManifest, assetsSection string) string {
	return fmt.Sprintf(`You are a senior author and content architect. Your job is to produce a detailed, actionable content plan.

Read plans/executive.md carefully — it contains the executive context for this writing project: vision, goals, target audience, scope, voice, and constraints.

No detailed content plan exists yet. The user is delegating ALL structural, editorial, and organizational decisions to you.

Generate a comprehensive content plan and write it to plans/plan.md.

Include:
1. **Content Scope & Objectives** — What the final written work will accomplish.
2. **Table of Contents** — Full chapter or section outline with working titles and one-sentence summaries.
3. **Chapter Outlines** — For each chapter/section: key arguments, required research, target word count, narrative arc.
4. **Style Guide** — Voice, tense, point of view, formatting conventions, terminology decisions.
5. **Research Requirements** — Sources to consult, facts to verify, interviews or data needed.
6. **Document Deliverables** — List every output document with its FULL path under output/.
   Use ordered filenames following this convention:
     {sequence}--{name}.md
   Where:
   - {sequence} is a number (01, 02, 03...) indicating production order
   - {name} is a descriptive lowercase kebab-case name
   Example: output/01--introduction.md, output/02--the-early-years.md
   The final deliverable should be output/manuscript.md (a consolidated document).
7. **Cross-Chapter Dependencies** — What must be established before later chapters can be written.
8. **Quality Standards** — Minimum word counts, citation requirements, readability targets.

CRITICAL:
- Make DECISIVE choices.
- Be SPECIFIC. Include exact document names, required section headings, word count targets.
- ALL output documents MUST use paths under output/ — never write deliverables directly to plans/.
  The plans/ directory is reserved for input files (executive.md, plan.md).
- Align every decision with the goals, constraints, and scope defined in plans/executive.md.
- If a media/ directory exists, reference the available assets by their paths in the relevant sections of the plan. Describe where and how each asset should be used in the deliverables.
- If supplementary asset documents are provided below, use them as reference material for content structure and research needs. Incorporate relevant information from those documents into the plan.
- Write the file directly to plans/plan.md. No other output.
- The output format should be markdown.

Executive context:
%s
%s%s`, executiveContent, mediaSection(mediaManifest), assetsPromptBlock(assetsSection))
}

func WritingStep1Prompt(planContent, executiveContent, mediaManifest string) string {
	contextLine := ""
	if executiveContent != "" {
		contextLine = "Also read plans/executive.md for executive context about the project's purpose, goals, audience, and voice.\n\n"
	}

	return fmt.Sprintf(`You are generating an AGENTS.md file for an autonomous AI writing agent.

Read plans/plan.md carefully — it contains the content plan for this writing project.
%sGenerate an AGENTS.md file and write it to .fry/AGENTS.md.

AGENTS.md is an operational rules file that the AI agent reads automatically at the start of every session. This is a WRITING project — the agent produces written content (chapters, sections, manuscripts), not code.

Rules should cover:
- Voice and tone consistency requirements
- Structural and formatting conventions
- Research and citation standards
- Content quality minimums (word counts, depth)
- Prohibited patterns (placeholder text, thin content, off-topic tangents)
- Output file conventions

Rules should be numbered, specific, and actionable. Each rule should be one line.
Write 15-40 rules total. Do NOT include vague rules.

One rule MUST state: "All content deliverables must be written to output/ using the naming convention {sequence}--{name}.md. Never write output documents directly to plans/."
One rule MUST state: "Never use placeholder text like [TODO], [TBD], [INSERT], or [PLACEHOLDER]. Every section must contain complete, publishable prose."

CRITICAL:
- Derive ALL rules from the plan document — do not invent rules not supported by the plan.
- This is a WRITING project. The agent produces written content — NOT code.
- If a media/ directory exists, include a rule that the agent should reference assets from media/ as specified in the plan.
- Write the file directly to .fry/AGENTS.md. No other output.

Plan:
%s

Executive context:
%s
%s`, contextLine, planContent, executiveContent, mediaSection(mediaManifest))
}

func WritingStep2Prompt(planContent, agentsContent, epicExamplePath, userPrompt string, effort epic.EffortLevel, enableReview bool, mediaManifest, assetsSection string) string {
	userPromptLine := ""
	if userPrompt != "" {
		userPromptLine = fmt.Sprintf("\nThe user has provided this top-level directive for the project: %q. Ensure sprint prompts align with this directive.\n", userPrompt)
	}

	reviewLine := ""
	if enableReview {
		reviewLine = "\n- Include the @review_between_sprints directive in the epic header. The user has opted into sprint review."
	}

	effortGuidance := effortSizingGuidanceWriting(effort)

	return fmt.Sprintf(`You are generating an epic.md file for an autonomous AI writing system.

This is a WRITING project — the AI agent produces written content (chapters, sections, manuscripts), NOT code. Each sprint delivers one or more written documents as its output.

Read these files carefully:
1. %s — The FORMAT REFERENCE showing the exact @directive syntax. Your output must use this syntax precisely. IGNORE the software-specific content in the examples — only follow the structural format.
2. plans/plan.md — The content plan to decompose into writing sprints. This is your primary source material.
3. .fry/AGENTS.md — Operational rules for the AI writing agent.

Generate the epic and write it to .fry/epic.md.
%s
CRITICAL RULES:
- Output ONLY the epic.md file content — write it directly to .fry/epic.md.
- Every @sprint block must have @name, @max_iterations, @promise, and @prompt.
- Sprint prompts must tell the agent to read .fry/AGENTS.md for operational rules.
- Sprint prompts must reference .fry/sprint-progress.txt and .fry/epic-progress.txt for progress tracking.
- ALL content deliverables MUST be written to the output/ directory, NOT to plans/ directly.
  The plans/ directory is reserved for input files (executive.md, plan.md).
- Sprint prompts must specify exact output filenames using the ordered convention:
    {sequence}--{name}.md
  Where:
  - {sequence} is a global sequence number across ALL sprints (not per-sprint), zero-padded (01, 02, etc.)
  - {name} is a descriptive lowercase kebab-case name
  Example paths: output/01--introduction.md, output/02--the-early-years.md
- Sprint prompts must include the required sections, word count targets, and concrete writing
  requirements from plans/plan.md for each deliverable — never vague instructions.
- If plans/plan.md lists deliverables with paths that don't follow this convention, translate
  them to the correct convention in the sprint prompts.
- All deliverables are WRITTEN CONTENT, not code.
- Sprint 1 is always research and outlining. The final sprint is always revision, consolidation
  into output/manuscript.md, and final polish.
- If media assets exist, sprint prompts that need those assets must instruct the agent to reference them from the media/ directory.
- If supplementary asset documents are provided below, ensure sprint prompts reference relevant source material or research from those documents where applicable.
- Do NOT include any output other than writing the file. No explanations, no summaries.%s%s

Plan:
%s

AGENTS.md:
%s
%s%s`, epicExamplePath, effortGuidance, userPromptLine, reviewLine, planContent, agentsContent, mediaSection(mediaManifest), assetsPromptBlock(assetsSection))
}

func effortSizingGuidanceWriting(effort epic.EffortLevel) string {
	switch effort {
	case epic.EffortLow:
		return `
EFFORT LEVEL: LOW
The user has indicated this is a low-effort writing task. You MUST:
- Generate AT MOST 2 sprints total
- Use max_iterations of 10-15 per sprint
- Write concise sprint prompts focused on essential deliverables
- Combine research and writing into a single sprint where possible
- Add the @effort low directive to the epic header
`
	case epic.EffortMedium:
		return `
EFFORT LEVEL: MEDIUM
The user has indicated this is a medium-effort writing task. You MUST:
- Generate 2-4 sprints total (prefer the lower end)
- Use max_iterations of 15-25 per sprint
- Write moderately detailed sprint prompts
- Merge related writing phases where practical
- Add the @effort medium directive to the epic header
`
	case epic.EffortHigh:
		return `
EFFORT LEVEL: HIGH
This is the standard effort level for writing projects. Follow all existing rules as-is.
- Generate 4-10 sprints as appropriate
- Use max_iterations of 15-35 per sprint
- Write fully detailed sprint prompts with comprehensive writing requirements
- Add the @effort high directive to the epic header
`
	case epic.EffortMax:
		return `
EFFORT LEVEL: MAX
The user has indicated this is a maximum-effort, high-stakes writing project. You MUST:
- Generate the same number of sprints as HIGH effort (4-10)
- Use max_iterations of 30-50 per sprint (higher than normal)
- Write EXTENDED sprint prompts with additional editorial requirements
- Include exhaustive research requirements and quality gates
- Add the @effort max directive to the epic header
- Enable @review_between_sprints and @compact_with_agent
`
	default: // auto-detect
		return `
EFFORT LEVEL: AUTO-DETECT
No effort level was specified. Analyze the content plan and determine the appropriate effort level:

- If the plan describes a short, focused piece (1-2 documents, under 5000 words total): use LOW effort (1-2 sprints, @effort low)
- If the plan describes a moderate work (3-5 documents or chapters): use MEDIUM effort (2-4 sprints, @effort medium)
- If the plan describes a substantial work (6+ chapters, book-length, or multi-part series): use HIGH effort (4-10 sprints, @effort high)

Add the @effort directive matching your assessment to the epic header.
Do NOT default to HIGH — genuinely evaluate the plan's complexity.
`
	}
}

func WritingSanityCheckPrompt(planContent, executiveContent, userPrompt string, effort epic.EffortLevel, mediaManifest, assetsSection string) string {
	return buildSanityCheckPrompt("senior author and content strategist", planContent, executiveContent, userPrompt, effort, mediaManifest, assetsSection)
}

func WritingStep3Prompt(planContent, epicContent, verificationExamplePath, userPrompt, mediaManifest string) string {
	userPromptLine := ""
	if userPrompt != "" {
		userPromptLine = fmt.Sprintf("\nThe user has provided this top-level directive: %q. If it affects what should or should not be verified, factor it in.\n", userPrompt)
	}

	return fmt.Sprintf(`You are generating a verification.md file for an autonomous AI writing system.

This is a WRITING project — the AI agent produces written content, NOT code. Verification checks must validate that content deliverables exist and meet quality standards.

Read these files carefully:
1. %s — The FORMAT REFERENCE showing exact syntax and check primitives. IGNORE the software-specific examples — adapt the primitives for content verification.
2. plans/plan.md — The content plan describing what is being written.
3. .fry/epic.md — The sprint definitions.
4. .fry/AGENTS.md — Operational rules for the AI writing agent.

Generate the verification file and write it to .fry/verification.md.

CRITICAL RULES:
- Output ONLY the verification.md file content — write it directly to .fry/verification.md.
- Use ONLY these four check primitives: @check_file, @check_file_contains, @check_cmd, @check_cmd_output
- @check_file_contains uses grep -E (extended regex / ERE). For alternation use | not \| (e.g. "foo|bar" matches foo OR bar). Using \| will search for a literal pipe character.
- Every @sprint block in the epic must have a corresponding @sprint block in verification.md.
- Every check must be a concrete, executable assertion. No prose. No subjective criteria.
- Verify content files exist, contain required section headings, include key topics and terms
  specified in the sprint prompt, and meet minimum word count requirements.
- Use @check_cmd with word count checks like: test $(wc -w < output/01--introduction.md) -ge 2000
- Use @check_file_contains to verify section headings: @check_file_contains output/01--introduction.md "## "
- All content deliverable paths in @check_file and @check_file_contains directives must reference
  the output/ directory (e.g., output/01--introduction.md).
  Do NOT reference plans/ directly for output documents.
- For the final sprint, verify output/manuscript.md exists and meets the total word count target.
- Do NOT write checks for deliverables from earlier sprints — only check the current sprint's new documents.
- Do NOT include any output other than writing the file. No explanations, no summaries.%s

Plan:
%s

Epic:
%s
%s`, verificationExamplePath, userPromptLine, planContent, epicContent, mediaSection(mediaManifest))
}
