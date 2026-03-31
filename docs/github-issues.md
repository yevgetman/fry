# GitHub Issues

Fry can treat a GitHub issue as the primary task definition for a build.

Use `--gh-issue` with `fry run`, `fry prepare`, or the root `fry` command:

```bash
fry --gh-issue https://github.com/yevgetman/fry/issues/74
fry run --gh-issue https://github.com/yevgetman/fry/issues/74 --engine claude
fry prepare --gh-issue https://github.com/yevgetman/fry/issues/74 --effort high
```

## Requirements

- `gh` must be installed and available on `PATH`
- You must be authenticated for the issue host: `gh auth login`
- The URL must match `https://<host>/<owner>/<repo>/issues/<number>`

Fry validates both the CLI installation and authentication before it fetches the issue.

## What Fry Does

When `--gh-issue` is passed, Fry:

1. Validates the GitHub issue URL
2. Runs `gh auth status --hostname <host>` to confirm authenticated access
3. Fetches the issue with `gh issue view <url> --json ...`
4. Converts the issue title, body, labels, and recent comments into a top-level task prompt
5. Persists the fetched issue context to **`.fry/github-issue.md`**
6. Persists the synthesized prompt to **`.fry/user-prompt.txt`**
7. Continues through the normal triage, prepare, and sprint pipeline

This means GitHub issues participate in the same flow as a normal user prompt:

- Triage sees the issue content
- Complex tasks can bootstrap `plans/executive.md` and `plans/plan.md`
- Generated `.fry/epic.md`, `.fry/AGENTS.md`, and `.fry/verification.md` are derived from the issue
- Sprint prompts inherit the issue-derived directive as Layer 1.5 context

## Interaction with Other Inputs

`--gh-issue` is an alternative to `--user-prompt` and `--user-prompt-file`.

- Do not combine `--gh-issue` with `--user-prompt`
- Do not combine `--gh-issue` with `--user-prompt-file`
- Existing `plans/plan.md` and `plans/executive.md` still take precedence where Fry already uses them

If plan files already exist, the issue-derived prompt is injected as an additional top-level directive instead of replacing those files.

## Dry Runs and Persistence

- `--dry-run` fetches and uses the issue for planning, but does not persist `.fry/user-prompt.txt` or `.fry/github-issue.md`
- Normal runs persist both files so follow-up runs have traceable context

## Stored Artifact

Fry writes the fetched issue context to **`.fry/github-issue.md`** with:

- Issue metadata: URL, repository, issue number, title, state, author, timestamps, labels
- Raw issue body
- The most recent comments (up to the current inclusion window)

This artifact is for transparency and debugging. It is not user-authored input.
