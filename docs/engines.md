# AI Engines

Fry supports three interchangeable AI engines: **OpenAI Codex**, **Claude Code**, and **Ollama**. The engine determines which CLI tool is invoked to execute prompts.

## Supported Engines

### Codex (OpenAI)

A supported build engine for software (coding) mode. Requires the Codex CLI:

```bash
npm i -g @openai/codex
```

Invocation:
```
echo PROMPT | codex exec --dangerously-bypass-approvals-and-sandbox [--model MODEL] [FLAGS]
```

The prompt is passed via stdin.

### Claude (Anthropic)

Requires the Claude Code CLI:

```bash
npm i -g @anthropic-ai/claude-code
```

Invocation:
```
echo PROMPT | claude -p --dangerously-skip-permissions [--model MODEL] [FLAGS]
```

The prompt is passed via stdin.

### Ollama (Local Models)

Fry supports Ollama for running builds with locally hosted open-source models (Llama 3, Mistral, CodeLlama, Phi-3, Gemma, etc.) without any API keys or cloud dependencies.

**Prerequisites:** Install [Ollama](https://ollama.com) and pull at least one model:

```bash
brew install ollama
ollama pull llama3
```

**Invocation:**
```
echo PROMPT | ollama run <model>
```

The prompt is passed via stdin. The model name is the first positional argument. Use the `@model` directive or `--model` flag to specify the Ollama model name.

**Usage:**
```bash
fry --engine ollama
fry --engine ollama --model codellama
```

**Model validation:** Fry accepts any Ollama model name without static validation. If the model is not locally available, Ollama will error at runtime.

**Tier mapping:** All tiers (frontier, standard, mini, labor) default to `llama3`. Override per-sprint with `@model <model-name>` or globally with `--model`.

## Engine Resolution

The AI engine is resolved with this precedence (highest wins):

1. `--engine` CLI flag
2. `@engine` directive in the epic file
3. `FRY_ENGINE` environment variable
4. Stage-specific default (see below)

### Default Engines by Mode and Stage

| Mode | Prepare Stage | Build Stage |
|---|---|---|
| **Software** (default) | Claude | Claude |
| **Planning** (`--mode planning`) | Claude | Claude |
| **Writing** (`--mode writing`) | Claude | Claude |

Claude is the default engine for all modes and stages. Use `--engine codex` or `--prepare-engine codex` to explicitly select Codex for any stage.

These defaults apply only when no explicit engine is specified via CLI flag, epic directive, or environment variable.

## Mixing Engines

CLI flags take absolute precedence over all other engine settings. For example, use Codex for sprint execution while keeping Claude for preparation:

```bash
fry --engine codex
```

Or use Codex for both stages:

```bash
fry --prepare-engine codex --engine codex
```

The `--prepare-engine` flag controls which engine is used during `fry prepare` (artifact generation), while `--engine` controls which engine executes the sprints. These flags override `@engine` directives in the epic, the `FRY_ENGINE` environment variable, and the default.

### Review Engine

When [dynamic sprint review](sprint-review.md) is enabled, the reviewer session can use a separate engine:

```
@review_engine claude
@review_model claude-sonnet-4-6
```

## Automatic Model Selection (Tier System)

Fry automatically selects the best model for each agent session based on the **engine**, **effort level**, and **session type**. Models are organized into four tiers:

| Tier | Purpose | Claude | Codex | Ollama |
|------|---------|--------|-------|--------|
| **Frontier** | Most capable, highest cost | `opus[1m]` | `gpt-5.4` | `llama3` |
| **Standard** | Strong all-rounder | `sonnet` | `gpt-5.3-codex` | `llama3` |
| **Mini** | Fast, cost-efficient | `haiku` | `gpt-5.4-mini` | `llama3` |
| **Labor** | Cheapest, no reasoning needed | `haiku` | `gpt-5-codex-mini` | `llama3` |

To update models when new ones release, change only the tier mapping table in `internal/engine/models.go`.

### Session × Effort Rules

| Session | low | medium | high | max |
|---------|-----|--------|------|-----|
| Sprint execution, Alignment, Audit fix, Review, Replan | Standard | Standard | Frontier | Frontier |
| Audit, Audit verify, Build audit (Claude) | Standard | Standard | Standard | Frontier |
| Audit, Audit verify, Build audit (Codex) | Mini | Standard | Frontier | Frontier |
| Build summary | Mini | Mini | Standard | Standard |
| Compaction | Labor | Labor | Labor | Mini |
| Continue analysis | Mini | Mini | Standard | Standard |
| Project overview | Labor | Labor | Labor | Labor |
| Prepare | Standard | Standard | Standard | **Frontier** |

Empty effort level defaults to "medium" behavior.

## Model Override

Epic directives override the automatic tier selection for their session group:

```
@model opus[1m]                # Overrides sprint execution + alignment + compaction + summary
@audit_model sonnet            # Overrides audit + audit fix + audit verify + build audit + continue analysis
@review_model claude-sonnet-4-6  # Overrides review + replan
```

The continue analysis session (`--continue`) uses `@audit_model` if set, otherwise falls back to `@model`.

Or via `--model` flag for `fry replan`.

Aliases `@codex_model` and `@codex_flags` are supported for backward compatibility.

## Engine Flags

Pass extra CLI flags to the underlying engine:

```
@engine_flags --full-auto
```

These flags are appended to the engine invocation command.

## Engine Interface

Internally, all three engines implement the same interface:

```go
type Engine interface {
    Run(ctx context.Context, prompt string, opts RunOpts) (output string, exitCode int, err error)
    Name() string
}
```

This means all Fry features (sprints, sanity checks, alignment, review) work identically regardless of which engine is selected.

## Rate-Limit Resilience

All engine calls are automatically wrapped with retry logic that detects rate-limit errors and retries with exponential backoff. This prevents long-running builds from failing due to transient API throttling.

### How It Works

When an engine call fails, Fry inspects the output and error for rate-limit indicators:

- `rate_limit` / `rate limit`
- HTTP `429` status code
- `overloaded` errors
- `too many requests`
- `retry-after: N` (parsed; used as the backoff delay)

If a rate limit is detected, Fry waits and retries. If the output contains a `retry-after` value, that delay is used. Otherwise, exponential backoff kicks in.

### Backoff Sequence

| Attempt | Base Delay | With Jitter (25%) |
|---------|-----------|-------------------|
| 1       | 10s       | 7.5s -- 12.5s     |
| 2       | 20s       | 15s -- 25s        |
| 3       | 40s       | 30s -- 50s        |
| 4       | 80s       | 60s -- 100s       |
| 5       | 120s      | 90s -- 120s (cap) |

Total maximum wait before giving up: ~4.5 minutes.

### Defaults

| Parameter | Value |
|-----------|-------|
| Max retries | 5 |
| Base delay | 10 seconds |
| Max delay | 120 seconds |
| Jitter | 25% |

These are defined in `internal/config/config.go` as `RateLimitMaxRetries`, `RateLimitBaseDelaySec`, `RateLimitMaxDelaySec`, and `RateLimitJitter`.

### Ollama

Ollama runs models locally and does not hit HTTP rate limits. Rate-limit detection is skipped for the Ollama engine.

### Implementation

Rate-limit resilience is implemented as a `ResilientEngine` decorator that wraps any `Engine`. The decorator is transparent -- callers (sprint execution, alignment, audit, review, etc.) are unaware of the retry layer. See `internal/engine/resilient.go` and `internal/engine/ratelimit.go`.
