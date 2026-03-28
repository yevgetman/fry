import { Type } from "@sinclair/typebox";
import { writeFileSync, mkdirSync } from "node:fs";
import { join, dirname } from "node:path";
import { validateProjectDir, FRY_PATHS } from "../config.js";
import type { AnyAgentTool } from "openclaw/plugin-sdk";

export function createBuildPauseTool(): AnyAgentTool {
	return {
		name: "fry_build_pause",
		label: "Fry: Pause Build",
		description:
			"Pause a running build (Tier C steering). The current iteration will finish, work will be checkpointed via git, and the build process will exit cleanly. Use fry_build_restart with --continue to resume.",
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

			const filePath = join(projectDir, FRY_PATHS.agentPause);
			try {
				mkdirSync(dirname(filePath), { recursive: true });
				writeFileSync(filePath, "", "utf-8");

				return {
					content: [
						{
							type: "text" as const,
							text: "Pause requested. The build will stop after the current iteration finishes and checkpoint the work. I'll notify you when it's paused.",
						},
					],
				};
			} catch (err) {
				return {
					content: [
						{
							type: "text" as const,
							text: `Failed to request pause: ${err instanceof Error ? err.message : String(err)}`,
						},
					],
				};
			}
		},
	};
}
