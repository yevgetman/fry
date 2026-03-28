import { Type } from "@sinclair/typebox";
import { writeFileSync, unlinkSync, mkdirSync, renameSync, existsSync } from "node:fs";
import { join, dirname } from "node:path";
import { validateProjectDir, FRY_PATHS } from "../config.js";
import type { AnyAgentTool } from "openclaw/plugin-sdk";

export function createBuildRespondTool(): AnyAgentTool {
	return {
		name: "fry_build_respond",
		label: "Fry: Respond to Decision",
		description:
			'Respond to a build that is waiting for a decision after a hold (Tier B steering). Response can be "continue" (proceed as planned), a directive for the next sprint, or "replan: <instructions>" to replan remaining sprints.',
		parameters: Type.Object({
			project_dir: Type.String({
				description: "Absolute path to the project directory",
			}),
			response: Type.String({
				description:
					'Your response: "continue", a directive for the next sprint, or "replan: <instructions>"',
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

			const response = (params.response as string).trim();
			if (!response) {
				return {
					content: [
						{ type: "text" as const, text: "Response cannot be empty." },
					],
				};
			}

			const directivePath = join(projectDir, FRY_PATHS.agentDirective);
			const decisionPath = join(projectDir, FRY_PATHS.decisionNeeded);

			// Verify the build is actually waiting for a decision
			if (!existsSync(decisionPath)) {
				return {
					content: [
						{
							type: "text" as const,
							text: "No decision is pending. The build is not currently holding. Use fry_build_directive for mid-iteration guidance instead.",
						},
					],
				};
			}

			try {
				mkdirSync(dirname(directivePath), { recursive: true });
				// Atomic write
				const tmpPath = directivePath + ".tmp";
				writeFileSync(tmpPath, response, "utf-8");
				renameSync(tmpPath, directivePath);

				// Clear the decision-needed file
				try {
					unlinkSync(decisionPath);
				} catch {
					// file may not exist
				}

				// Match Go-side detection: "replan:" prefix OR exact "replan"
				const lower = response.toLowerCase();
				const isReplan =
					lower.startsWith("replan:") || lower === "replan";
				const isContinue = lower === "continue";

				let summary: string;
				if (isReplan) {
					summary =
						"Replan request sent. The build will replan remaining sprints and continue.";
				} else if (isContinue) {
					summary =
						"Continuing as planned. The build will proceed to the next sprint.";
				} else {
					summary = `Directive sent for the next sprint: "${response.slice(0, 100)}${response.length > 100 ? "..." : ""}"`;
				}

				return {
					content: [{ type: "text" as const, text: summary }],
				};
			} catch (err) {
				return {
					content: [
						{
							type: "text" as const,
							text: `Failed to send response: ${err instanceof Error ? err.message : String(err)}`,
						},
					],
				};
			}
		},
	};
}
