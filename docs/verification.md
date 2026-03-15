# Verification

Fry supports independent verification of each sprint's deliverables. When a `.fry/verification.md` file is present, Fry runs machine-executable checks after the agent signals completion.

## Check Primitives

| Primitive | Example | Passes when |
|---|---|---|
| `@check_file <path>` | `@check_file src/index.ts` | File exists and is non-empty |
| `@check_file_contains <path> <pattern>` | `@check_file_contains package.json "typescript"` | File contains pattern (grep -E) |
| `@check_cmd <command>` | `@check_cmd npm run build` | Command exits 0 |
| `@check_cmd_output <cmd> \| <pattern>` | `@check_cmd_output curl -s /health \| "ok"` | stdout matches pattern |

## Verification File Format

`.fry/verification.md` uses the `@sprint N` block structure:

```markdown
@sprint 1
@check_file go.mod
@check_file_contains go.mod "module myproject"
@check_cmd go build ./...
@check_cmd_output go version | "go1\."

@sprint 2
@check_cmd go test ./...
```

## Custom Verification File

The `@verification` directive in the epic file can override the default path:

```
@verification .fry/checks-phase2.md
```

## Outcome Matrix

| Promise Token | Checks Exist | Checks Pass | Result |
|---|---|---|---|
| Found | Yes | All pass | **PASS** |
| Found | Yes | Some fail | Enters **heal loop** |
| Found | No | N/A | **PASS** |
| Not found | Yes | All pass | **PASS** (verification passed, no promise) |
| Not found | Yes | Some fail | Enters **heal loop** |
| Not found | No | N/A | **FAIL** (no promise after N iters) |

When the heal loop is exhausted, Fry prints recovery commands. Use `fry run --retry` to skip iterations and re-enter the heal loop with a boosted attempt budget (2x normal, minimum 6). See [Self-Healing](self-healing.md) for details.

## Verification for Documents

The same four check primitives work for non-code deliverables in [planning mode](planning-mode.md):

```
@check_file plans/market-analysis.md
@check_file_contains plans/market-analysis.md "## Market Size"
@check_cmd test $(wc -w < plans/market-analysis.md) -ge 500
@check_cmd_output grep -c '^## ' plans/market-analysis.md | ^[5-9]
```

## Output Normalization

`@check_cmd_output` trims leading and trailing whitespace from each line of command output before pattern matching. This prevents platform-specific formatting differences from causing false negatives. For example, macOS `wc -w` outputs `     42` (with leading spaces) while Linux outputs `42`. After trimming, the pattern `^[0-9]+$` matches on both platforms.

If you need to match exact whitespace in command output, use a pattern that accounts for optional whitespace (e.g., `\s*42\s*` instead of `^42$`).

## Verification Reload During Healing

When a heal pass modifies `.fry/verification.md` (e.g., fixing a broken check), Fry re-reads the file before the next verification run. This ensures on-disk edits by the healing agent take effect between attempts, rather than being ignored due to in-memory caching.

## Graceful Degradation

- If `.fry/verification.md` does not exist, Fry falls back to promise-only behavior
- If `fry prepare` fails to generate `.fry/verification.md`, it logs a warning and continues
- If a sprint has no checks defined, it behaves as if no verification file exists for that sprint

## Safety Limits

- **Output cap**: Output from verification checks is capped at 10 MB to prevent unbounded memory growth.
- **Per-check timeout**: Command-based checks (`@check_cmd` and `@check_cmd_output`) are killed after 120 seconds to prevent hanging builds.
- **Diagnostic truncation**: Per-check diagnostic output in heal prompts is truncated to 20 lines for `@check_cmd` failures and 10 lines for `@check_cmd_output` failures.
