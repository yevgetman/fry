import { Type } from "@sinclair/typebox";
import { execFileSync } from "node:child_process";
import type { AnyAgentTool } from "openclaw/plugin-sdk";

export function createBuildStatusTool(fryBinary: string): AnyAgentTool {
	return {
		name: "fry_build_status",
		label: "Fry: Build Status",
		description:
			"Get the current state of a Fry build. Returns structured JSON with sprint number, iteration, status, last event, and more.",
		parameters: Type.Object({
			project_dir: Type.Optional(
				Type.String({
					description:
						"Project directory (default: current working directory)",
				}),
			),
		}),

		async execute(
			_toolCallId: string,
			params: Record<string, unknown>,
		) {
			const projectDir = (params.project_dir as string) || ".";
			try {
				const output = execFileSync(
					fryBinary,
					["status", "--json", "--project-dir", projectDir],
					{ encoding: "utf-8", timeout: 10_000 },
				);
				const state = JSON.parse(output);
				return {
					content: [
						{ type: "text" as const, text: JSON.stringify(state, null, 2) },
					],
				};
			} catch (err) {
				return {
					content: [
						{
							type: "text" as const,
							text: `Failed to get build status: ${err instanceof Error ? err.message : String(err)}`,
						},
					],
				};
			}
		},
	};
}
