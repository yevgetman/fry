import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { definePluginEntry } from "openclaw/plugin-sdk/plugin-entry";
import { getFryBinary, type FryPluginConfig } from "./src/config.js";
import { createBuildStartTool } from "./src/tools/build-start.js";
import { createBuildStatusTool } from "./src/tools/build-status.js";
import { createBuildRestartTool } from "./src/tools/build-restart.js";
import { createBuildLogsTool } from "./src/tools/build-logs.js";
import { createReadProgressTool } from "./src/tools/read-progress.js";
import { createConsciousnessStatsTool } from "./src/tools/consciousness.js";
import { createBuildDirectiveTool } from "./src/tools/build-directive.js";
import { createBuildHoldTool } from "./src/tools/build-hold.js";
import { createBuildRespondTool } from "./src/tools/build-respond.js";
import { createBuildPauseTool } from "./src/tools/build-pause.js";
import { BuildWatcher } from "./src/service/build-watcher.js";

const execFileAsync = promisify(execFile);

let buildWatcher: BuildWatcher | null = null;

export function startWatchingBuild(projectDir: string): void {
	buildWatcher?.startWatching(projectDir);
}

export function stopWatchingBuild(projectDir: string): void {
	buildWatcher?.stopWatching(projectDir);
}

export default definePluginEntry({
	id: "fry",
	name: "Fry Build Orchestration",
	description:
		"Start, monitor, and interact with Fry builds through OpenClaw's conversational interface",

	register(api) {
		const config = (api.pluginConfig ?? {}) as FryPluginConfig;
		const fryBinary = getFryBinary(config);

		// Register tools — Layer 0: Monitoring
		api.registerTool(createBuildStartTool(fryBinary), { optional: true });
		api.registerTool(createBuildStatusTool(fryBinary), { optional: true });
		api.registerTool(createBuildRestartTool(fryBinary), { optional: true });
		api.registerTool(createBuildLogsTool(), { optional: true });
		api.registerTool(createReadProgressTool(), { optional: true });
		api.registerTool(createConsciousnessStatsTool(fryBinary), {
			optional: true,
		});

		// Register tools — Layer 1: Build Steering
		api.registerTool(createBuildDirectiveTool(), { optional: true });
		api.registerTool(createBuildHoldTool(), { optional: true });
		api.registerTool(createBuildRespondTool(), { optional: true });
		api.registerTool(createBuildPauseTool(), { optional: true });

		// Load Fry system prompt once, then inject via cacheable prependSystemContext.
		// Uses a promise so it doesn't block plugin registration or gateway startup.
		let frySystemPrompt: string | null = null;
		const promptReady = execFileAsync(fryBinary, ["agent", "prompt"], {
			timeout: 5_000,
		})
			.then(({ stdout }) => {
				frySystemPrompt = stdout || null;
			})
			.catch(() => {
				// Fry binary unavailable or prompt command failed — skip silently
				frySystemPrompt = null;
			});

		api.on("before_prompt_build", async (_event, _ctx) => {
			// Wait for initial load on first call; subsequent calls resolve immediately
			await promptReady;
			if (frySystemPrompt) {
				return { prependSystemContext: frySystemPrompt };
			}
		});

		// Register build watcher service
		api.registerService({
			id: "fry-build-watcher",

			async start(ctx) {
				buildWatcher = new BuildWatcher(config, (message) => {
					ctx.logger.info(`[fry] ${message}`);
				});
				ctx.logger.info("Fry build watcher ready");
			},

			async stop(ctx) {
				buildWatcher?.stopAll();
				buildWatcher = null;
				ctx.logger.info("Fry build watcher stopped");
			},
		});
	},
});
