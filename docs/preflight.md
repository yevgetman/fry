# Preflight Checks

Before executing any sprints, Fry runs a preflight validation phase. All checks must pass or the build is aborted.

## Checks Performed

| Check | Fails when |
|---|---|
| AI engine CLI | `codex` or `claude` binary not on PATH |
| `git` | Not installed |
| `bash` | Not installed |
| `plans/` directory | Missing |
| Input files | Both `plans/plan.md` and `plans/executive.md` are missing (note: `--user-prompt` generates these during prepare, before preflight runs) |
| `.fry/AGENTS.md` existence | Missing |
| `.fry/AGENTS.md` placeholder | First line matches the standard placeholder marker |
| `.fry/AGENTS.md` content | Fewer than 5 lines |
| Docker | Not installed (only when sprints at or after `@docker_from_sprint` are in range) |
| `@require_tool` | Any declared tool not on PATH |
| `@preflight_cmd` | Any custom preflight command exits non-zero |
| Disk space | Warning (not fatal) if less than 2 GB free |

## Required Tools

Declare tools that must be available using `@require_tool` in the epic:

```
@require_tool node
@require_tool docker
@require_tool psql
```

Each tool is checked via PATH lookup. The preflight phase fails if any declared tool is missing.

## Custom Preflight Commands

Run arbitrary validation commands before the build starts using `@preflight_cmd`:

```
@preflight_cmd npm --version
@preflight_cmd docker compose version
@preflight_cmd test -f .env
```

Each command is executed via `bash -c` in the project directory. If any command exits non-zero, the build is aborted.

## Sprint-Scoped Preflight

Before each sprint starts, Fry infers environment prerequisites from the sprint prompt text and checks whether they are satisfied. This catches missing env vars, unavailable Docker, and missing external tools before expensive sprint execution and audit cycles.

**Inferred prerequisites:**
- **Environment variables**: detected via `$VAR`, `${VAR}`, `os.Getenv("VAR")`, `process.env.VAR`, and prose patterns like "env vars like FOO, BAR". Standard system variables (`HOME`, `PATH`, etc.) are excluded.
- **Docker**: detected when the prompt mentions Docker, docker-compose, or testcontainers.
- **External tools**: detected for common tools like Playwright, Cypress, Redis, PostgreSQL, MySQL, Python, Java.

Sprint preflight warnings are non-fatal (the sprint still runs) but are recorded in `build-status.json` under the sprint's `warnings` array. This lets operators immediately see whether audit failures were caused by missing prerequisites rather than real product defects.

## Terminal Output

```
[2026-03-10 11:54:16] Preflight checks passed.
```

If a check fails, Fry prints the specific failure and exits.
