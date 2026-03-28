import { Type } from "@sinclair/typebox";
import { execFileSync } from "node:child_process";
import type { AnyAgentTool } from "openclaw/plugin-sdk";

export function createConsciousnessStatsTool(
	fryBinary: string,
): AnyAgentTool {
	return {
		name: "fry_consciousness_stats",
		label: "Fry: Consciousness Stats",
		description:
			"Query Fry's consciousness pipeline status: memory counts, last reflection, identity version, build counter.",
		parameters: Type.Object({}),

		async execute() {
			try {
				const output = execFileSync(
					fryBinary,
					["status", "--consciousness"],
					{ encoding: "utf-8", timeout: 15_000 },
				);
				return {
					content: [{ type: "text" as const, text: output }],
				};
			} catch (err) {
				return {
					content: [
						{
							type: "text" as const,
							text: `Failed to fetch consciousness stats: ${err instanceof Error ? err.message : String(err)}`,
						},
					],
				};
			}
		},
	};
}
