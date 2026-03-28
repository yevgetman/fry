import { spawn, type ChildProcess } from "node:child_process";
import { createInterface } from "node:readline";
import { getFryBinary, getNotificationLevel } from "../config.js";
import type { FryPluginConfig } from "../config.js";

interface BuildEvent {
	type: string;
	ts: string;
	sprint?: number;
	data?: Record<string, string>;
}

type NotifyFn = (message: string) => void;

/**
 * Manages fry event watchers for active builds. Each project directory gets
 * its own `fry events --follow --json` subprocess.
 */
export class BuildWatcher {
	private watchers = new Map<string, ChildProcess>();
	private config: FryPluginConfig;
	private notify: NotifyFn;
	private fryBinary: string;

	constructor(config: FryPluginConfig, notify: NotifyFn) {
		this.config = config;
		this.notify = notify;
		this.fryBinary = getFryBinary(config);
	}

	/**
	 * Start watching a project directory for build events.
	 */
	startWatching(projectDir: string): void {
		if (this.watchers.has(projectDir)) return;

		const child = spawn(
			this.fryBinary,
			["events", "--follow", "--json", "--project-dir", projectDir],
			{ stdio: ["ignore", "pipe", "ignore"] },
		);

		// Handle spawn failure (e.g., binary not found)
		child.on("error", (err) => {
			this.watchers.delete(projectDir);
			this.notify(`Build watcher failed for ${projectDir}: ${err.message}`);
		});

		if (!child.stdout) {
			// stdout is null — kill the orphaned child
			child.kill();
			return;
		}

		const rl = createInterface({ input: child.stdout });
		rl.on("line", (line) => {
			try {
				const evt: BuildEvent = JSON.parse(line);
				this.handleEvent(projectDir, evt);
			} catch {
				// skip malformed lines
			}
		});

		// Guard against race: only delete if this is still our child
		child.on("exit", () => {
			if (this.watchers.get(projectDir) === child) {
				this.watchers.delete(projectDir);
			}
		});

		this.watchers.set(projectDir, child);
	}

	/**
	 * Stop watching a project directory.
	 */
	stopWatching(projectDir: string): void {
		const child = this.watchers.get(projectDir);
		if (child) {
			child.kill();
			this.watchers.delete(projectDir);
		}
	}

	/**
	 * Stop all watchers.
	 */
	stopAll(): void {
		for (const child of this.watchers.values()) {
			child.kill();
		}
		this.watchers.clear();
	}

	private handleEvent(projectDir: string, evt: BuildEvent): void {
		const level = getNotificationLevel(this.config);
		const message = formatEventNotification(evt);
		if (!message) return;

		switch (level) {
			case "all":
				this.notify(message);
				break;
			case "milestones":
				if (isMilestone(evt)) this.notify(message);
				break;
			case "errors":
				if (isError(evt)) this.notify(message);
				break;
		}
	}
}

function isMilestone(evt: BuildEvent): boolean {
	return [
		"sprint_complete",
		"build_end",
		"build_audit_done",
		"alignment_complete",
		"directive_received",
		"decision_needed",
		"decision_received",
		"build_paused",
	].includes(evt.type);
}

function isError(evt: BuildEvent): boolean {
	if (evt.type === "build_end" && evt.data?.outcome !== "success") return true;
	if (
		evt.type === "sprint_complete" &&
		evt.data?.status &&
		!evt.data.status.startsWith("PASS")
	)
		return true;
	return false;
}

function formatEventNotification(evt: BuildEvent): string | null {
	switch (evt.type) {
		case "sprint_start":
			return `Starting sprint ${evt.sprint}${evt.data?.name ? `: ${evt.data.name}` : ""}`;

		case "sprint_complete": {
			const status = evt.data?.status ?? "unknown";
			const heals = evt.data?.alignment_attempts ?? "0";
			const duration = evt.data?.duration ?? "";
			const passed = status.startsWith("PASS");
			const base = `Sprint ${evt.sprint} complete: ${passed ? "all checks passed" : "some checks failed"}`;
			const details = [];
			if (heals !== "0") details.push(`${heals} alignment passes`);
			if (duration) details.push(duration);
			return details.length > 0 ? `${base} (${details.join(", ")})` : base;
		}

		case "alignment_complete":
			return `Alignment loop finished for sprint ${evt.sprint}. Attempts: ${evt.data?.attempts ?? "unknown"}, status: ${evt.data?.status ?? "unknown"}`;

		case "audit_complete":
			return `Sprint ${evt.sprint} audit complete${evt.data?.findings_count ? ` (${evt.data.findings_count} findings)` : ""}`;

		case "review_complete":
			return `Sprint ${evt.sprint} review: ${evt.data?.verdict ?? "unknown"}`;

		case "build_audit_done":
			return `Build audit complete${evt.data?.findings_count ? ` (${evt.data.findings_count} findings)` : ""}`;

		case "build_end": {
			const outcome = evt.data?.outcome ?? "unknown";
			return outcome === "success"
				? "Build complete! All sprints passed."
				: `Build ended: ${outcome}`;
		}

		// Layer 1: Steering events
		case "directive_received":
			return `Directive acknowledged for sprint ${evt.sprint}. Preview: ${evt.data?.preview ?? ""}`;

		case "decision_needed":
			return `Build is holding after sprint ${evt.sprint} (${evt.data?.completed_sprint ?? ""}). ${evt.data?.remaining_sprints ?? "?"} sprints remaining. Waiting for your decision.`;

		case "decision_received":
			return `Decision received for sprint ${evt.sprint}. Resuming build.`;

		case "build_paused":
			return `Build paused at sprint ${evt.sprint}, iteration ${evt.data?.iteration ?? "?"}. Work checkpointed. Use restart to resume.`;

		default:
			return null;
	}
}
