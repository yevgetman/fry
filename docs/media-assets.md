# Media Assets

Fry supports an optional `media/` directory at the project root for images, PDFs, fonts, data files, and other assets that your plan references. When present, Fry scans the directory and includes a categorized manifest in every prompt so the AI agent knows what assets are available.

## Usage

```bash
mkdir -p media
cp ~/designs/logo.png media/
cp ~/docs/wireframe.pdf media/
cp ~/fonts/brand-font.woff2 media/
```

Reference the files in your plan:

```markdown
# My Project -- Build Plan

**Branding:**
- Use `media/logo.png` as the header logo and favicon source
- Use `media/brand-font.woff2` as the primary heading font

**UI Reference:**
- See `media/wireframe.pdf` for the page layout
```

During both `fry prepare` and `fry run`, the AI agent receives a manifest of all media files grouped by category, with paths and sizes. The agent can then copy, reference, or embed these assets as instructed.

## Supported File Categories

| Category | Extensions |
|---|---|
| Image | `.png`, `.jpg`, `.jpeg`, `.gif`, `.svg`, `.webp`, `.ico`, `.bmp`, `.tiff`, `.tif` |
| Document | `.pdf`, `.docx`, `.doc`, `.xlsx`, `.xls`, `.pptx`, `.ppt`, `.csv`, `.tsv` |
| Design | `.fig`, `.sketch`, `.psd`, `.ai`, `.xd` |
| Data | `.json`, `.yaml`, `.yml`, `.xml`, `.toml` |
| Video | `.mp4`, `.mov`, `.avi`, `.webm`, `.mkv` |
| Audio | `.mp3`, `.wav`, `.ogg`, `.flac`, `.aac` |
| Font | `.ttf`, `.otf`, `.woff`, `.woff2`, `.eot` |

Files with unrecognized extensions are categorized as "other" and still included in the manifest.

## Manifest Example

When the prompt is assembled, the media section looks like this:

```
# ===== MEDIA ASSETS =====
# The media/ directory contains assets that may be referenced in the plan
# or executive context. Use these files as instructed -- copy them into the
# appropriate locations in the project, reference them in code or documents,
# or use them as design/content inputs.

## Font
- `media/brand-font.woff2` (45.2 KB)

## Image
- `media/logo.png` (12.3 KB)
- `media/icons/arrow.svg` (1.1 KB)

## Document
- `media/wireframe.pdf` (2.1 MB)
```

## Subdirectories

Subdirectories within `media/` are fully supported. Organize assets however you like:

```
media/
  logo.png
  icons/
    arrow.svg
    check.svg
  fonts/
    heading.woff2
    body.woff2
```

## Filtering

The scanner automatically skips:

- **Hidden files and directories** -- names starting with `.` (e.g., `.DS_Store`, `.gitkeep`)
- **Symlinks** -- both file and directory symlinks are skipped for security
- Files that exceed a 10,000-file cap (to prevent runaway scans on very large directories)

## How It Integrates

The media manifest flows through Fry at two levels:

1. **During `fry prepare`** -- the manifest is included in all four generation steps (plan, AGENTS.md, epic, verification). This means the AI can reference media assets when designing sprint prompts and verification checks.

2. **During `fry run`** -- the manifest is injected as prompt Layer 1.25 (between executive context and user directive) for every sprint. The agent sees the full list of available assets each iteration.

## Planning Mode

Media assets work identically in `--planning` mode. Place charts, reference documents, or data files in `media/` and reference them in your plan. The AI planning agent will incorporate them into the deliverable documents written to `output/`.

## Media vs. Supplementary Assets

Fry has two optional asset directories with different purposes:

- **`media/`** -- for binary files (images, fonts, PDFs) that the AI agent needs to **copy or reference by path**. The AI sees a manifest of file paths and sizes, included in both prepare and sprint execution prompts.
- **`assets/`** -- for text reference documents (specs, schemas, requirements) that the AI should **read and understand** during planning. The AI sees the full file contents, included only during prepare. See [Supplementary Assets](supplementary-assets.md).

## Optional

The `media/` directory is entirely optional. If it doesn't exist, Fry behaves exactly as before -- no manifest is generated and no prompt layer is added.
