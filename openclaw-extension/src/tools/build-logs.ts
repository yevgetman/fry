import { Type } from "@sinclair/typebox";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { FRY_PATHS, validateProjectDir } from "../config.js";
import type { AnyAgentTool } from "openclaw/plugin-sdk";

export function createBuildLogsTool(): AnyAgentTool {
	return {
		name: "fry_build_logs",
		label: "Fry: Read Build Logs",
		description:
			"Read recent build logs from a Fry build. Returns the last N lines from sprint logs, heal logs, audit logs, or the most recent log of any type.",
		parameters: Type.Object({
			project_dir: Type.String({
				description: "Absolute path to the project directory",
			}),
			log_type: Type.Optional(
				Type.String({
					description:
						"Log type: sprint, heal, audit, latest (default: latest)",
				}),
			),
			lines: Type.Optional(
				Type.Number({
					description: "Number of lines to return (default: 50)",
				}),
			),
		}),

		async execute(
			_toolCallId: string,
			params: Record<string, unknown>,
		) {
			const projectDir = params.project_dir as string;
			const dirErr = validateProjectDir(projectDir);
			if (dirErr) {
				return { content: [{ type: "text" as const, text: dirErr }] };
			}
			const logType = (params.log_type as string) || "latest";
			const lines = (params.lines as number) || 50;
			const logsDir = join(projectDir, FRY_PATHS.buildLogs);

			try {
				const entries = readdirSync(logsDir).filter((f) =>
					f.endsWith(".log"),
				);

				let matched: string[];
				switch (logType) {
					case "sprint":
						matched = entries.filter(
							(f) =>
								f.startsWith("sprint") &&
								!f.includes("_iter") &&
								!f.includes("_heal"),
						);
						break;
					case "heal":
						matched = entries.filter((f) => f.includes("_heal"));
						break;
					case "audit":
						matched = entries.filter((f) => f.includes("audit"));
						break;
					default:
						matched = entries;
						break;
				}

				if (matched.length === 0) {
					return {
						content: [
							{
								type: "text" as const,
								text: `No ${logType} logs found.`,
							},
						],
					};
				}

				// Sort by name descending (timestamps in filenames sort lexicographically)
				matched.sort((a, b) => b.localeCompare(a));
				const latest = matched[0];
				const content = readFileSync(join(logsDir, latest), "utf-8");

				// Return last N lines
				const allLines = content.split("\n");
				const tail =
					allLines.length > lines
						? allLines.slice(-lines).join("\n")
						: content;

				return {
					content: [
						{
							type: "text" as const,
							text: `--- ${latest} (last ${lines} lines) ---\n${tail}`,
						},
					],
				};
			} catch (err) {
				return {
					content: [
						{
							type: "text" as const,
							text: `Failed to read logs: ${err instanceof Error ? err.message : String(err)}`,
						},
					],
				};
			}
		},
	};
}
