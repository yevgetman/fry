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

## Terminal Output

```
[2026-03-10 11:54:16] Preflight checks passed.
```

If a check fails, Fry prints the specific failure and exits.
