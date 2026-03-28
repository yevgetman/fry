import { Type } from "@sinclair/typebox";
import { spawn } from "node:child_process";
import { startWatchingBuild } from "../../index.js";
import { validateProjectDir } from "../config.js";

import type { AnyAgentTool } from "openclaw/plugin-sdk";

const ALLOWED_FLAGS = new Set([
	"--review",
	"--no-review",
	"--no-audit",
	"--always-verify",
	"--no-project-overview",
	"--full-prepare",
	"--verbose",
	"--no-observer",
	"--planning",
	"--dry-run",
]);

const VALID_EFFORTS = new Set(["low", "medium", "high", "max", "auto", ""]);
const VALID_ENGINES = new Set(["claude", "codex", "ollama", ""]);
const VALID_MODES = new Set(["software", "planning", "writing", ""]);

export function createBuildStartTool(fryBinary: string): AnyAgentTool {
	return {
		name: "fry_build_start",
		label: "Fry: Start Build",
		description:
			"Start a new Fry build on a project directory. Spawns `fry run` as a subprocess and begins monitoring. Returns the build ID and initial state.",
		parameters: Type.Object({
			project_dir: Type.String({
				description: "Absolute path to the project directory",
			}),
			effort: Type.Optional(
				Type.String({
					description:
						"Effort level: low, medium, high, max, or auto (default: auto)",
				}),
			),
			engine: Type.Optional(
				Type.String({
					description:
						"LLM engine: claude, codex, ollama (default: claude)",
				}),
			),
			mode: Type.Optional(
				Type.String({
					description:
						"Build mode: software, planning, writing (default: software)",
				}),
			),
			user_prompt: Type.Optional(
				Type.String({ description: "Additional user directive for the build" }),
			),
			flags: Type.Optional(
				Type.Array(Type.String(), {
					description:
						"Extra CLI flags (e.g. --review, --no-audit, --always-verify)",
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

			const effort = (params.effort as string) || "";
			const engine = (params.engine as string) || "";
			const mode = (params.mode as string) || "";
			const userPrompt = (params.user_prompt as string) || "";
			const flags = (params.flags as string[]) || [];

			if (effort && !VALID_EFFORTS.has(effort)) {
				return {
					content: [{ type: "text" as const, text: `Invalid effort: "${effort}". Valid: ${[...VALID_EFFORTS].filter(Boolean).join(", ")}` }],
				};
			}
			if (engine && !VALID_ENGINES.has(engine)) {
				return {
					content: [{ type: "text" as const, text: `Invalid engine: "${engine}". Valid: ${[...VALID_ENGINES].filter(Boolean).join(", ")}` }],
				};
			}
			if (mode && !VALID_MODES.has(mode)) {
				return {
					content: [{ type: "text" as const, text: `Invalid mode: "${mode}". Valid: ${[...VALID_MODES].filter(Boolean).join(", ")}` }],
				};
			}

			const args = ["run", "--project-dir", projectDir];
			if (effort) args.push("--effort", effort);
			if (engine) args.push("--engine", engine);
			if (mode) args.push("--mode", mode);
			if (userPrompt) args.push("--user-prompt", userPrompt);
			// Validate flags against allowlist to prevent injection
			for (const flag of flags) {
				if (!ALLOWED_FLAGS.has(flag)) {
					return {
						content: [
							{
								type: "text" as const,
								text: `Rejected unknown flag: ${flag}. Allowed: ${[...ALLOWED_FLAGS].join(", ")}`,
							},
						],
					};
				}
				args.push(flag);
			}
			// Always produce JSON report for structured results
			args.push("--json-report");
			// Enable telemetry by default for consciousness pipeline
			args.push("--telemetry");

			const child = spawn(fryBinary, args, {
				cwd: projectDir,
				detached: true,
				stdio: "ignore",
			});

			// Don't let the child block the parent from exiting
			child.unref();

			const pid = child.pid ?? 0;

			// Start the build watcher to send proactive notifications
			startWatchingBuild(projectDir);

			return {
				content: [
					{
						type: "text" as const,
						text: `Build started. PID: ${pid}, project: ${projectDir}. Use fry_build_status to check progress.`,
					},
				],
			};
		},
	};
}
