import { Type } from "@sinclair/typebox";
import { writeFileSync, mkdirSync, renameSync } from "node:fs";
import { join, dirname } from "node:path";
import { validateProjectDir, FRY_PATHS } from "../config.js";

const MAX_DIRECTIVE_BYTES = 10_000; // 10KB cap on directive content
import type { AnyAgentTool } from "openclaw/plugin-sdk";

export function createBuildDirectiveTool(): AnyAgentTool {
	return {
		name: "fry_build_directive",
		label: "Fry: Send Directive",
		description:
			"Send a directive to a running Fry build (Tier A steering). The directive is injected into the next iteration's prompt. Use this when the user wants to adjust focus, add context, or provide guidance without stopping the build.",
		parameters: Type.Object({
			project_dir: Type.String({
				description: "Absolute path to the project directory",
			}),
			directive: Type.String({
				description: "The directive text to inject into the next iteration",
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

			const directive = params.directive as string;
			if (!directive.trim()) {
				return {
					content: [
						{ type: "text" as const, text: "Directive cannot be empty." },
					],
				};
			}
			if (Buffer.byteLength(directive) > MAX_DIRECTIVE_BYTES) {
				return {
					content: [
						{ type: "text" as const, text: `Directive too large (${Buffer.byteLength(directive)} bytes). Max: ${MAX_DIRECTIVE_BYTES}.` },
					],
				};
			}

			const filePath = join(projectDir, FRY_PATHS.agentDirective);
			try {
				mkdirSync(dirname(filePath), { recursive: true });
				// Atomic write: write to tmp, then rename
				const tmpPath = filePath + ".tmp";
				writeFileSync(tmpPath, directive, "utf-8");
				renameSync(tmpPath, filePath);

				return {
					content: [
						{
							type: "text" as const,
							text: `Directive sent. It will be picked up on the next iteration. Preview: "${directive.slice(0, 100)}${directive.length > 100 ? "..." : ""}"`,
						},
					],
				};
			} catch (err) {
				return {
					content: [
						{
							type: "text" as const,
							text: `Failed to write directive: ${err instanceof Error ? err.message : String(err)}`,
						},
					],
				};
			}
		},
	};
}
