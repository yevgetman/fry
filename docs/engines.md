# AI Engines

Fry supports two interchangeable AI engines: **OpenAI Codex** and **Claude Code**. The engine determines which CLI tool is invoked to execute prompts.

## Supported Engines

### Codex (OpenAI)

The default engine. Requires the Codex CLI:

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
4. Default: `codex`

## Mixing Engines

You can use different engines for different phases. For example, generate the epic with Claude and run the build with Codex:

```bash
fry --prepare-engine claude --engine codex
```

The `--prepare-engine` flag controls which engine is used during `fry prepare` (artifact generation), while `--engine` controls which engine executes the sprints.

### Review Engine

When [dynamic sprint review](sprint-review.md) is enabled, the reviewer session can use a separate engine:

```
@review_engine claude
@review_model claude-sonnet-4-6
```

## Model Override

Override the default model for any engine:

```
@model gpt-4.1              # In the epic file
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
