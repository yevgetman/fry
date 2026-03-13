package config

const (
	FryDir                    = ".fry"
	PlansDir                  = "plans"
	BuildLogsDir              = ".fry/build-logs"
	DefaultEngine             = "codex"
	DefaultMaxHealAttempts    = 3
	DefaultDockerReadyTimeout = 30
	DefaultMaxDeviationScope  = 3
	DefaultVerificationFile   = ".fry/verification.md"
	PromptFile                = ".fry/prompt.md"
	SprintProgressFile        = ".fry/sprint-progress.txt"
	EpicProgressFile          = ".fry/epic-progress.txt"
	ReviewPromptFile          = ".fry/review-prompt.md"
	DeviationLogFile          = ".fry/deviation-log.md"
	LockFile                  = ".fry/.fry.lock"
	UserPromptFile            = ".fry/user-prompt.txt"
	PlanFile                  = "plans/plan.md"
	ExecutiveFile             = "plans/executive.md"
	PlanningOutputDir         = "plans/output"
	MediaDir                  = "media"
	AgentsFile                = ".fry/AGENTS.md"
	Version                   = "0.1.0"
	AgentInvocationPrompt     = "Read and execute ALL instructions in .fry/prompt.md. Before starting, read .fry/sprint-progress.txt for context from previous iterations in this sprint, and .fry/epic-progress.txt for summaries of prior sprints. Also read plans/plan.md for strategic context on how this sprint fits the overall plan. If a media/ directory exists, it contains assets (images, PDFs, etc.) that may be referenced in the plan — use or copy them as instructed. After completing your work, append your progress to .fry/sprint-progress.txt."
	HealInvocationPrompt      = "Read and execute ALL instructions in .fry/prompt.md. This is a HEAL pass — fix the verification failures described in the prompt."
	DefaultEffortLevel        = "" // auto-detect

	// Audit constants
	SprintAuditFile           = ".fry/sprint-audit.txt"
	AuditPromptFile           = ".fry/audit-prompt.md"
	DefaultMaxAuditIterations = 3
	MaxAuditDiffBytes         = 100_000
	AuditInvocationPrompt     = "Read and execute ALL instructions in .fry/audit-prompt.md. You are a code auditor. Review the sprint's work and write your findings to .fry/sprint-audit.txt. Do NOT modify any source code."
	AuditFixInvocationPrompt  = "Read and execute ALL instructions in .fry/audit-prompt.md. Fix the issues identified in .fry/sprint-audit.txt."
)
