# AI Engines

Fry supports two interchangeable AI engines: **OpenAI Codex** and **Claude Code**. The engine determines which CLI tool is invoked to execute prompts.

## Supported Engines

### Codex (OpenAI)

The default build engine for software (coding) mode. Requires the Codex CLI:

```bash
npm i -g @openai/codex
```

Invocation:
```
codex exec --dangerously-bypass-approvals-and-sandbox [--model MODEL] [FLAGS] PROMPT
```

### Claude (Anthropic)

Requires the Claude Code CLI:

```bash
npm i -g @anthropic-ai/claude-code
```

Invocation:
```
claude -p --dangerously-skip-permissions [--model MODEL] [FLAGS] PROMPT
```

## Engine Resolution

The AI engine is resolved with this precedence (highest wins):

1. `--engine` CLI flag
2. `@engine` directive in the epic file
3. `FRY_ENGINE` environment variable
4. Stage-specific default (see below)

### Default Engines by Mode and Stage

| Mode | Prepare Stage | Build Stage |
|---|---|---|
| **Software** (default) | Claude | Codex |
| **Planning** (`--mode planning`) | Claude | Claude |
| **Writing** (`--mode writing`) | Claude | Claude |

In software mode, Claude is used for artifact generation (prepare) and Codex is used for sprint execution (build). In planning and writing modes, Claude is used for both stages.

These defaults apply only when no explicit engine is specified via CLI flag, epic directive, or environment variable.

## Mixing Engines

You can override the defaults for any stage. For example, use Codex for both preparation and build:

```bash
fry --prepare-engine codex --engine codex
```

The `--prepare-engine` flag controls which engine is used during `fry prepare` (artifact generation), while `--engine` controls which engine executes the sprints.

### Review Engine

When [dynamic sprint review](sprint-review.md) is enabled, the reviewer session can use a separate engine:

```
@review_engine claude
@review_model claude-sonnet-4-6
```

## Automatic Model Selection (Tier System)

Fry automatically selects the best model for each agent session based on the **engine**, **effort level**, and **session type**. Models are organized into four tiers:

| Tier | Purpose | Claude | Codex |
|------|---------|--------|-------|
| **Frontier** | Most capable, highest cost | `opus[1m]` | `gpt-5.4` |
| **Standard** | Strong all-rounder | `sonnet` | `gpt-5.3-codex` |
| **Mini** | Fast, cost-efficient | `haiku` | `gpt-5.4-mini` |
| **Labor** | Cheapest, no reasoning needed | `haiku` | `gpt-5-codex-mini` |

To update models when new ones release, change only the tier mapping table in `internal/engine/models.go`.

### Session × Effort Rules

| Session | low | medium | high | max |
|---------|-----|--------|------|-----|
| Sprint execution, Heal, Audit fix, Review, Replan | Standard | Standard | Frontier | Frontier |
| Audit, Audit verify, Build audit (Claude) | Standard | Standard | Standard | Frontier |
| Audit, Audit verify, Build audit (Codex) | Mini | Standard | Frontier | Frontier |
| Build summary | Mini | Mini | Standard | Standard |
| Compaction | Labor | Labor | Labor | Mini |
| Continue analysis | Mini | Mini | Standard | Standard |
| Sanity check | Labor | Labor | Labor | Labor |
| Prepare | Standard | Standard | Standard | Standard |

Empty effort level defaults to "medium" behavior.

## Model Override

Epic directives override the automatic tier selection for their session group:

```
@model opus[1m]                # Overrides sprint execution + heal + compaction + summary
@audit_model sonnet            # Overrides audit + audit fix + audit verify + build audit
@review_model claude-sonnet-4-6  # Overrides review + replan
```

Or via `--model` flag for `fry replan`.

Aliases `@codex_model` and `@codex_flags` are supported for backward compatibility.

## Engine Flags

Pass extra CLI flags to the underlying engine:

```
@engine_flags --full-auto
```

These flags are appended to the engine invocation command.

## Engine Interface

Internally, both engines implement the same interface:

```go
type Engine interface {
    Run(ctx context.Context, prompt string, opts RunOpts) (output string, exitCode int, err error)
    Name() string
}
```

This means all Fry features (sprints, verification, healing, review) work identically regardless of which engine is selected.
