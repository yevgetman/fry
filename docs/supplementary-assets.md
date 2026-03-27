# Supplementary Assets

Fry supports an optional `assets/` directory at the project root for text-based reference documents -- API specifications, requirements docs, design notes, data schemas, and other files whose **content** should inform the plan and epic generation. Unlike `media/` (which provides a manifest of file paths), `assets/` files are **read in full** and their contents are injected into the prompts that generate `plans/plan.md` and `.fry/epic.md`.

## How It Differs from Media

| | `media/` | `assets/` |
|---|---|---|
| **Purpose** | Binary assets (images, fonts, PDFs) | Text reference documents |
| **What the AI sees** | File manifest (paths + sizes) | Full file contents |
| **Used during prepare** | Yes (manifest in all steps) | Yes (contents in Steps 0 and 2) |
| **Used during sprint execution** | Yes (Layer 1.25 in every sprint prompt) | No -- context is baked into plan.md and epic.md |
| **File types** | Any (categorized by extension) | Text files only (allowlisted extensions) |
| **Size limits** | 10,000 file cap | 512 KB per file, 2 MB total, 100 file cap |

Use `media/` for files the build agent needs to **copy or reference by path** (logos, fonts, wireframes). Use `assets/` for documents the AI should **read and understand** when planning (specs, schemas, requirements).

## Usage

```bash
mkdir -p assets
cp ~/docs/api-spec.yaml assets/
cp ~/docs/requirements.md assets/
cp ~/docs/data-model.sql assets/
```

Reference them in your plan or executive context:

```markdown
# My Project -- Build Plan

**API Design:**
- Follow the OpenAPI spec in `assets/api-spec.yaml`
- See `assets/requirements.md` for detailed requirements
- Database schema defined in `assets/data-model.sql`
```

During `fry prepare`, the AI reads each asset file's full contents and uses them to make better-informed decisions when generating the build plan and sprint decomposition.

## Supported File Types

Only text-based files with recognized extensions are included:

| Category | Extensions |
|---|---|
| Documentation | `.md`, `.txt`, `.rst` |
| Data / Config | `.json`, `.yaml`, `.yml`, `.toml`, `.xml`, `.csv`, `.tsv` |
| Web | `.html`, `.htm`, `.css` |
| Code | `.go`, `.py`, `.rb`, `.java`, `.js`, `.ts`, `.jsx`, `.tsx`, `.sql` |
| Shell | `.sh`, `.bash`, `.zsh` |
| Config | `.cfg`, `.ini`, `.conf`, `.properties` |
| Schema | `.proto`, `.graphql`, `.gql` |
| Other | `.log` |

Files with unrecognized extensions are silently skipped. Binary files with text extensions are detected via UTF-8 validation and skipped with a warning.

## How Assets Appear in Prompts

When the prepare prompt is assembled, asset contents are formatted as a clearly delimited section with fenced code blocks:

```
# ===== SUPPLEMENTARY ASSETS =====
# The assets/ directory contains reference documents provided as additional context.
# Use this information to inform your design decisions, architecture, and implementation details.

## File: assets/api-spec.yaml (2.1 KB)
```yaml
openapi: 3.0.0
paths:
  /users:
    get:
      summary: List users
```

## File: assets/requirements.md (1.4 KB)
```markdown
# Requirements
- Must support OAuth 2.0
- Rate limiting at 100 req/min
```
```

Each file includes its relative path, human-readable size, and full content wrapped in a language-tagged fence.

## Size Limits

| Limit | Value | Behavior when exceeded |
|---|---|---|
| Per-file size | 512 KB | File skipped with warning |
| Aggregate total | 2 MB | Remaining files skipped, scan truncated |
| File count | 100 | Remaining files skipped, scan truncated |

These limits prevent prompt bloat. If a file is skipped, a warning is logged so you know to split or trim it.

## Subdirectories

Subdirectories within `assets/` are fully supported:

```
assets/
  api-spec.yaml
  specs/
    auth-flow.md
    data-model.sql
  reference/
    industry-standards.md
```

## Filtering

The scanner automatically skips:

- **Hidden files and directories** -- names starting with `.` (e.g., `.DS_Store`)
- **Symlinks** -- both file and directory symlinks are skipped for security
- **Binary files** -- files with unrecognized extensions, or files that fail UTF-8 validation
- **Oversized files** -- files exceeding the 512 KB per-file limit

## When Assets Are Used

Asset contents are injected into the **prepare** phase only:

| Prepare Step | Receives assets? | Purpose |
|---|---|---|
| Bootstrap (user prompt to executive.md) | Yes | Assets inform executive context generation |
| Step 0 (executive.md to plan.md) | Yes | Assets inform plan generation |
| Step 1 (plan.md to AGENTS.md) | No | AGENTS.md derives from the plan, which already incorporates asset context |
| Step 2 (plan.md to epic.md) | Yes | Assets inform sprint decomposition |
| Step 3 (epic.md to verification.md) | No | Sanity checks derive from plan + epic |

Once `epic.md` is generated, assets are no longer needed -- their context is baked into the plan and epic. During `fry run`, sprint prompts do **not** include asset contents.

## Planning Mode

Supplementary assets work identically in `--planning` mode. Place reference documents, data files, or prior analyses in `assets/` and they will inform the planning document generation.

## Optional

The `assets/` directory is entirely optional. If it doesn't exist, Fry behaves exactly as before -- no asset content is injected and no warnings are shown.
