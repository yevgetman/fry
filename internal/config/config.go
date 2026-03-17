package config

const (
	FryDir                    = ".fry"
	PlansDir                  = "plans"
	BuildLogsDir              = ".fry/build-logs"
	DefaultEngine             = "codex"
	DefaultPrepareEngine      = "claude"
	DefaultPlanningEngine     = "claude"
	DefaultWritingEngine      = "claude"
	DefaultMaxHealAttempts    = 3
	DefaultMaxFailPercent     = 20
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
	PlanningOutputDir         = "output"
	WritingOutputDir          = "output"
	MediaDir                  = "media"
	AssetsDir                 = "assets"
	AgentsFile                = ".fry/AGENTS.md"
	Version                   = "0.1.0"
	AgentInvocationPrompt     = "Read and execute ALL instructions in .fry/prompt.md. Before starting, read .fry/sprint-progress.txt for context from previous iterations in this sprint, and .fry/epic-progress.txt for summaries of prior sprints. Also read plans/plan.md for strategic context on how this sprint fits the overall plan. If a media/ directory exists, it contains assets (images, PDFs, etc.) that may be referenced in the plan — use or copy them as instructed. After completing your work, append your progress to .fry/sprint-progress.txt."
	HealInvocationPrompt      = "Read and execute ALL instructions in .fry/prompt.md. This is a HEAL pass — fix the verification failures described in the prompt."
	DefaultEffortLevel        = "" // auto-detect
	RetryHealMultiplier       = 2
	RetryMinHealAttempts      = 6

	// Effort-level heal constants
	HealAttemptsHigh       = 10
	HealStuckThresholdHigh = 2
	HealStuckThresholdMax  = 3
	HealMinAttemptsMax     = 10 // min attempts before mid-loop threshold exit in max effort
	HealSafetyCapMax       = 50 // hard safety cap for unlimited max-effort healing
	MaxFailPercentMax      = 10 // stricter threshold for max effort

	// Audit constants
	SprintAuditFile            = ".fry/sprint-audit.txt"
	AuditPromptFile            = ".fry/audit-prompt.md"
	DefaultMaxAuditIterations  = 3
	MaxAuditDiffBytes          = 100_000
	AuditInvocationPrompt      = "Read and execute ALL instructions in .fry/audit-prompt.md. You are a code auditor. Review the sprint's work and write your findings to .fry/sprint-audit.txt. Do NOT modify any source code."
	AuditFixInvocationPrompt   = "Read and execute ALL instructions in .fry/audit-prompt.md. Fix the issues described in the prompt."

	// Two-level audit loop constants
	DefaultMaxOuterAuditCycles = 3  // outer audit cycles (medium/default)
	DefaultMaxInnerFixIter     = 3  // fix attempts per audit report (medium/default)
	MaxOuterCyclesHighCap      = 10 // outer audit cycles at high effort
	MaxOuterCyclesMaxCap       = 15 // outer audit cycles at max effort
	MaxInnerFixIterHigh        = 5  // inner fix cap at high effort
	MaxInnerFixIterMax         = 8  // inner fix cap at max effort

	DeferredFailuresFile = ".fry/deferred-failures.md"

	// Summary constants
	SummaryFile       = "build-summary.md"
	SummaryPromptFile = ".fry/summary-prompt.md"

	// Build audit constants
	BuildAuditFile             = "audit.md"
	BuildAuditPromptFile       = ".fry/build-audit-prompt.md"
	BuildAuditInvocationPrompt = "Read and execute ALL instructions in .fry/build-audit-prompt.md. You are performing a final holistic audit of the entire codebase. Audit, classify, report, remediate, and re-audit as instructed in the prompt."

	// Build mode persistence
	BuildModeFile = ".fry/build-mode.txt"

	// Continue constants
	ContinuePromptFile        = ".fry/continue-prompt.md"
	ContinueDecisionFile      = ".fry/continue-decision.txt"
	ContinueReportFile        = ".fry/continue-report.md"
	ContinueInvocationPrompt  = "Read and execute ALL instructions in .fry/continue-prompt.md. You are a build analyst. Review the build state report and output your decision to .fry/continue-decision.txt. Do NOT modify any source code."
)
