import { Type } from "@sinclair/typebox";
import { writeFileSync, mkdirSync } from "node:fs";
import { join, dirname } from "node:path";
import { validateProjectDir, FRY_PATHS } from "../config.js";
import type { AnyAgentTool } from "openclaw/plugin-sdk";

export function createBuildHoldTool(): AnyAgentTool {
	return {
		name: "fry_build_hold",
		label: "Fry: Hold After Sprint",
		description:
			"Request the build to pause after the current sprint completes (Tier B steering). The build will checkpoint and wait for your decision: continue, provide direction for the next sprint, or replan remaining sprints.",
		parameters: Type.Object({
			project_dir: Type.String({
				description: "Absolute path to the project directory",
			}),
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

			const filePath = join(projectDir, FRY_PATHS.agentHold);
			try {
				mkdirSync(dirname(filePath), { recursive: true });
				writeFileSync(filePath, "", "utf-8");

				return {
					content: [
						{
							type: "text" as const,
							text: "Hold requested. The build will pause after the current sprint completes and checkpoint. I'll let you know when it's ready for your decision.",
						},
					],
				};
			} catch (err) {
				return {
					content: [
						{
							type: "text" as const,
							text: `Failed to request hold: ${err instanceof Error ? err.message : String(err)}`,
						},
					],
				};
			}
		},
	};
}
