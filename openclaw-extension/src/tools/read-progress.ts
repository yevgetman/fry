import { Type } from "@sinclair/typebox";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { FRY_PATHS, validateProjectDir } from "../config.js";
import type { AnyAgentTool } from "openclaw/plugin-sdk";

export function createReadProgressTool(): AnyAgentTool {
	return {
		name: "fry_read_progress",
		label: "Fry: Read Progress",
		description:
			"Read sprint progress (current sprint's iteration log) or epic progress (compacted summaries of completed sprints).",
		parameters: Type.Object({
			project_dir: Type.String({
				description: "Absolute path to the project directory",
			}),
			scope: Type.Optional(
				Type.String({
					description: "Progress scope: sprint or epic (default: sprint)",
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
			const scope = (params.scope as string) || "sprint";

			const file =
				scope === "epic"
					? FRY_PATHS.epicProgress
					: FRY_PATHS.sprintProgress;

			try {
				const content = readFileSync(join(projectDir, file), "utf-8");
				if (!content.trim()) {
					return {
						content: [
							{
								type: "text" as const,
								text: `No ${scope} progress recorded yet.`,
							},
						],
					};
				}
				return {
					content: [{ type: "text" as const, text: content }],
				};
			} catch (err) {
				if (
					err instanceof Error &&
					"code" in err &&
					(err as NodeJS.ErrnoException).code === "ENOENT"
				) {
					return {
						content: [
							{
								type: "text" as const,
								text: `No ${scope} progress file found. Build may not have started yet.`,
							},
						],
					};
				}
				return {
					content: [
						{
							type: "text" as const,
							text: `Failed to read ${scope} progress: ${err instanceof Error ? err.message : String(err)}`,
						},
					],
				};
			}
		},
	};
}
