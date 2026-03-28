package agent

import "github.com/yevgetman/fry/internal/config"

// ArtifactSchema returns the complete manifest of .fry/ artifacts that Fry
// produces during a build. Used to generate system prompts for any LLM agent.
func ArtifactSchema() []ArtifactInfo {
	return []ArtifactInfo{
		{
			Path:        config.ObserverEventsFile,
			Format:      "jsonl",
			Description: "Structured build lifecycle events. One JSON object per line. Event types: build_start, sprint_start, sprint_complete, alignment_complete, audit_complete, review_complete, build_audit_done, build_end. Each event has ts (ISO 8601), type, optional sprint number, and optional data map.",
			Lifecycle:   "per-build",
		},
		{
			Path:        config.SprintProgressFile,
			Format:      "markdown",
			Description: "Append-only iteration log for the current sprint. The sprint agent appends progress notes after each iteration with what was completed, what remains, and any issues encountered.",
			Lifecycle:   "per-sprint",
		},
		{
			Path:        config.EpicProgressFile,
			Format:      "markdown",
			Description: "Compacted summaries of completed sprints. After each sprint, its progress is compacted into a concise summary and appended here. Provides cross-sprint context.",
			Lifecycle:   "per-build",
		},
		{
			Path:        config.BuildLogsDir,
			Format:      "text",
			Description: "Directory containing per-iteration and per-sprint logs. Naming: sprintN_iterM_TIMESTAMP.log (per-iteration), sprintN_TIMESTAMP.log (consolidated sprint), sprintN_healM_TIMESTAMP.log (alignment passes).",
			Lifecycle:   "per-iteration",
		},
		{
			Path:        config.DefaultVerificationFile,
			Format:      "markdown",
			Description: "Sanity check definitions. Types: file (exists and non-empty), file_contains (regex match), cmd (exit code 0), cmd_output (output regex match), test (go test/pytest/jest). Checks are scoped to specific sprints or global.",
			Lifecycle:   "static",
		},
		{
			Path:        config.PromptFile,
			Format:      "markdown",
			Description: "The assembled prompt for the current iteration. Contains 8 layers: executive context, media, user directive, disposition, quality directive, strategic plan, sprint instructions, iteration memory.",
			Lifecycle:   "per-iteration",
		},
		{
			Path:        config.DeferredFailuresFile,
			Format:      "markdown",
			Description: "Sanity check failures that were below the failure threshold and deferred. Tracked for end-of-build audit.",
			Lifecycle:   "per-build",
		},
		{
			Path:        config.SprintAuditFile,
			Format:      "text",
			Description: "Audit findings for the current sprint. Severity levels: CRITICAL and HIGH block progress, MODERATE gets one fix attempt, LOW is acceptable.",
			Lifecycle:   "per-sprint",
		},
		{
			Path:        config.BuildReportFile,
			Format:      "json",
			Description: "Structured build report (when --json-report is used). Contains epic name, timing, per-sprint results with verification check details and token usage.",
			Lifecycle:   "per-build",
		},
		{
			Path:        config.ObserverScratchpadFile,
			Format:      "markdown",
			Description: "Observer's metacognitive working memory within a build. Contains observations and hypotheses carried between wake-ups.",
			Lifecycle:   "per-build",
		},
		{
			Path:        config.BuildModeFile,
			Format:      "text",
			Description: "Single word indicating build mode: software, planning, or writing.",
			Lifecycle:   "per-build",
		},
		{
			Path:        config.BuildExitReasonFile,
			Format:      "text",
			Description: "Reason the build exited. Present after build completion or failure.",
			Lifecycle:   "per-build",
		},
		{
			Path:        config.DeviationLogFile,
			Format:      "markdown",
			Description: "Log of sprint review deviations. Records when the review system issued a DEVIATE verdict and what changed.",
			Lifecycle:   "per-build",
		},
		{
			Path:        config.AgentsFile,
			Format:      "markdown",
			Description: "Generated agent instructions (AGENTS.md). Contains project rules and context for sprint agents.",
			Lifecycle:   "per-build",
		},
	}
}
