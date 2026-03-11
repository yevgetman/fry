# Docker Support

fry manages Docker Compose services automatically when configured in the epic file.

## Configuration

Enable Docker support with the `@docker_from_sprint` directive:

```
@docker_from_sprint 2
@docker_ready_cmd docker compose exec -T postgres pg_isready -U myapp
@docker_ready_timeout 30
```

| Directive | Description | Default |
|---|---|---|
| `@docker_from_sprint <N>` | Start docker-compose from sprint N onward | Disabled |
| `@docker_ready_cmd <cmd>` | Custom health check command | Container health status |
| `@docker_ready_timeout <s>` | Health check timeout in seconds | 30 |

## How It Works

1. **Compose version detection** — fry detects Docker Compose V2 (`docker compose`) or V1 (`docker-compose`) automatically
2. **Compose file detection** — checks for `docker-compose.yml` or `compose.yml` in the project directory
3. **Service startup** — runs `docker compose up -d` (or V1 equivalent) before the sprint begins
4. **Readiness check** — waits for services to become healthy before proceeding

## Readiness Checks

### Default Behavior

Without a custom `@docker_ready_cmd`, fry checks that all containers report healthy status — no container should be in "starting" or "unhealthy" state.

### Custom Health Check

With `@docker_ready_cmd`, fry repeatedly runs the specified command until it exits 0 or the timeout is reached:

```
@docker_ready_cmd docker compose exec -T postgres pg_isready -U myapp
@docker_ready_timeout 60
```

## Sprint Scoping

Docker is only required when running sprints at or after the configured sprint number. Earlier sprints can run without Docker installed. This means:

- `fry run epic.md 1 1` with `@docker_from_sprint 2` — Docker not required
- `fry run epic.md 2` with `@docker_from_sprint 2` — Docker required

## Preflight Check

When sprints in the run range are at or after `@docker_from_sprint`, the [preflight checks](preflight.md) verify that Docker is installed on the system before starting the build.
