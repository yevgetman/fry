export interface FryPluginConfig {
	fry_binary?: string;
	default_effort?: string;
	default_engine?: string;
	notifications?: "all" | "milestones" | "errors";
}

export function getFryBinary(config?: FryPluginConfig): string {
	return config?.fry_binary || process.env.FRY_BINARY || "fry";
}

export function getNotificationLevel(
	config?: FryPluginConfig,
): "all" | "milestones" | "errors" {
	return config?.notifications || "milestones";
}

import { isAbsolute } from "node:path";

// Validate that a project directory path is absolute and doesn't contain traversal.
export function validateProjectDir(dir: string): string | null {
	if (!dir || !isAbsolute(dir)) {
		return "project_dir must be an absolute path";
	}
	if (dir.includes("..")) {
		return "project_dir must not contain '..' path segments";
	}
	return null;
}

// Artifact paths — mirrors Go internal/config/config.go constants.
// If the Go side changes these paths, update them here.
export const FRY_PATHS = {
	sprintProgress: ".fry/sprint-progress.txt",
	epicProgress: ".fry/epic-progress.txt",
	buildLogs: ".fry/build-logs",
	events: ".fry/observer/events.jsonl",
	verification: ".fry/verification.md",
	buildReport: ".fry/build-report.json",
	buildMode: ".fry/build-mode.txt",
	buildExitReason: ".fry/build-exit-reason.txt",
	sprintAudit: ".fry/sprint-audit.txt",
	deferredFailures: ".fry/deferred-failures.md",
} as const;
