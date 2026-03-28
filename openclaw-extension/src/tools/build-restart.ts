import { Type } from "@sinclair/typebox";
import { spawn } from "node:child_process";
import { startWatchingBuild } from "../../index.js";
import { validateProjectDir } from "../config.js";
import type { AnyAgentTool } from "openclaw/plugin-sdk";

const VALID_MODES = new Set(["continue", "resume", "simple-continue"]);

export function createBuildRestartTool(fryBinary: string): AnyAgentTool {
	return {
		name: "fry_build_restart",
		label: "Fry: Restart Build",
		description:
			"Restart a stopped or failed Fry build. Uses --continue (LLM-driven resume), --resume (skip to healing), or --simple-continue (lightweight resume).",
		parameters: Type.Object({
			project_dir: Type.String({
				description: "Absolute path to the project directory",
			}),
			mode: Type.Optional(
				Type.String({
					description:
						"Resume mode: continue (default, LLM-driven), resume (skip to healing), simple-continue (lightweight)",
				}),
			),
			user_prompt: Type.Optional(
				Type.String({
					description:
						"Additional directive for the resumed build (injected via --user-prompt)",
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

			const mode = (params.mode as string) || "continue";
			const userPrompt = (params.user_prompt as string) || "";

			if (!VALID_MODES.has(mode)) {
				return {
					content: [
						{
							type: "text" as const,
							text: `Invalid mode: "${mode}". Valid: ${[...VALID_MODES].join(", ")}`,
						},
					],
				};
			}

			const args = ["run", "--project-dir", projectDir];

			switch (mode) {
				case "resume":
					args.push("--resume");
					break;
				case "simple-continue":
					args.push("--simple-continue");
					break;
				default:
					args.push("--continue");
					break;
			}

			if (userPrompt) {
				args.push("--user-prompt", userPrompt);
			}
			args.push("--json-report", "--telemetry");

			const child = spawn(fryBinary, args, {
				cwd: projectDir,
				detached: true,
				stdio: "ignore",
			});
			child.unref();

			// Start the build watcher for proactive notifications
			startWatchingBuild(projectDir);

			return {
				content: [
					{
						type: "text" as const,
						text: `Build restarting with --${mode}. PID: ${child.pid ?? "unknown"}. Use fry_build_status to monitor.`,
					},
				],
			};
		},
	};
}
