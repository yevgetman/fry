# AI Engines

Fry supports three interchangeable AI engines: **OpenAI Codex**, **Claude Code**, and **Ollama**. The engine determines which CLI tool is invoked to execute prompts.

## Supported Engines

### Codex (OpenAI)

The default build engine for software (coding) mode. Requires the Codex CLI:

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
| Sprint execution, Heal, Audit fix, Review, Replan | Standard | Standard | Frontier | Frontier |
| Audit, Audit verify, Build audit (Claude) | Standard | Standard | Standard | Frontier |
| Audit, Audit verify, Build audit (Codex) | Mini | Standard | Frontier | Frontier |
| Build summary | Mini | Mini | Standard | Standard |
| Compaction | Labor | Labor | Labor | Mini |
| Continue analysis | Mini | Mini | Standard | Standard |
| Sanity check | Labor | Labor | Labor | Labor |
| Prepare | Standard | Standard | Standard | **Frontier** |

Empty effort level defaults to "medium" behavior.

## Model Override

Epic directives override the automatic tier selection for their session group:

```
@model opus[1m]                # Overrides sprint execution + heal + compaction + summary
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

This means all Fry features (sprints, verification, healing, review) work identically regardless of which engine is selected.
