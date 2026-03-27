# Planning Mode

Fry's execution engine is project-agnostic — the sprint loop, sanity check runner, and alignment loop work identically regardless of whether the output is code or documents.

Pass `--mode planning` (or the backwards-compatible alias `--planning`) to use planning-domain prompts that generate sprints for producing structured documents instead of code. Use it for business plans, trip planning, research reports, strategic analyses, or any endeavor that requires rigorous, phased document creation.

For human-language content like books, guides, and reports, see [Writing Mode](writing-mode.md) instead.

## Usage

```bash
# Start from just a prompt (no files needed)
fry --mode planning --user-prompt "competitive analysis for entering the EV market"

# Generate and run with existing plan files
fry --mode planning

# Generate artifacts only
fry prepare --mode planning

# Backwards-compatible alias (equivalent to --mode planning)
fry --planning
```

In planning mode, Claude is the default engine for both the prepare and build stages, so `--engine claude` is not required.

## How It Differs from Software Mode

| Aspect | Default (software) | `--planning` |
|---|---|---|
| Default engine (prepare) | Claude | Claude |
| Default engine (build) | Codex | Claude |
| `.fry/AGENTS.md` | Technology constraints, architecture rules, testing patterns | Domain boundaries, analytical frameworks, document quality standards |
| Sprint phasing | Scaffolding → Schema → Logic → Integration → E2E | Research → Analysis → Strategy → Detailed Planning → Synthesis |
| Sprint deliverables | Source files, configs, tests | Markdown documents, analyses, strategies |
| Sanity checks | Build succeeds, tests pass, files exist | Documents exist, contain required sections, meet minimum depth |

## Quick Start

```bash
# Option A: Start from a prompt
fry --mode planning --user-prompt "launch plan for a specialty coffee shop in Portland"

# Option B: Start from a plan file
mkdir -p plans
cat > plans/plan.md << 'EOF'
# Coffee Shop Launch Plan

## Vision
Open a specialty coffee shop in downtown Portland targeting remote workers.

## Key Challenges
- Location selection and lease negotiation
- Menu development and supplier sourcing
- Financial projections and funding strategy
- Marketing and pre-launch buzz
EOF

fry --mode planning
```

## Output Directory and Naming Convention

In planning mode, all document deliverables are written to `output/` at the project root, keeping the `plans/` directory reserved for input files (`executive.md`, `plan.md`).

Output filenames use an ordered, categorized naming convention:

```
{sequence}--{category}--{name}.md
```

- **sequence** — a globally unique number (1, 2, 3...) indicating production order across all sprints
- **category** — a short grouping label (e.g., `research`, `analysis`, `strategy`, `synthesis`)
- **name** — a descriptive kebab-case name

Example directory layout:

```
your-project/
  plans/
    executive.md                              # INPUT: your executive context
    plan.md                                   # INPUT: generated or authored plan
  output/                                     # OUTPUT: all deliverables
    1--research--market-landscape.md
    2--research--competitor-profiles.md
    3--analysis--positioning-options.md
    4--strategy--go-to-market.md
    5--synthesis--executive-summary.md
```

## Sanity Checks for Documents

The same four [sanity check primitives](sanity-checks.md) work for document deliverables:

```
@check_file output/1--research--market-landscape.md
@check_file_contains output/1--research--market-landscape.md "## Market Size"
@check_cmd test $(wc -w < output/1--research--market-landscape.md) -ge 500
@check_cmd_output grep -c '^## ' output/1--research--market-landscape.md | ^[5-9]
```

These checks ensure documents exist, contain required sections, meet minimum word counts, and have sufficient heading structure.

## Using Media Assets in Planning Mode

Place supporting materials (charts, data files, reference PDFs) in the `media/` directory. Reference them in your plan and the AI agent will incorporate them into the deliverable documents.

```
media/
  market-data.csv
  competitor-logos/
    acme.png
    globex.png
```

In your plan: "Use the data in `media/market-data.csv` to inform the market sizing section." See [Media Assets](media-assets.md) for details.

## Using Supplementary Assets in Planning Mode

Place text-based reference documents in the `assets/` directory. Unlike `media/` files (which provide a path manifest), `assets/` files are **read in full** and their contents are injected into the prompts that generate `plans/plan.md` and `.fry/epic.md`.

```
assets/
  prior-analysis.md
  industry-report.txt
  competitor-data.csv
```

In your plan: "Reference the findings in `assets/prior-analysis.md` when developing the competitive positioning section." See [Supplementary Assets](supplementary-assets.md) for details.

## When to Use Planning Mode

- Business plans and strategies
- Research reports and literature reviews
- Trip itineraries and travel planning
- Strategic analyses and competitive assessments
- Project proposals and feasibility studies
- Any multi-phase document creation that benefits from structured decomposition

## Resuming a Planning Build

When a planning build is interrupted or fails, `fry run --continue` automatically restores the `planning` mode from the previous run. There is no need to pass `--mode planning` again:

```bash
fry run --continue                    # auto-detects planning mode
fry run --continue --mode software    # explicit override if needed
```

The mode is persisted to `.fry/build-mode.txt` at the start of every build and read back by `--continue`.

## See Also

- [Writing Mode](writing-mode.md) -- human-language content (books, guides, reports, documentation)
- [Sanity Checks](sanity-checks.md) -- check primitives and document sanity check examples
- [Effort Levels](effort-levels.md) -- sprint count and rigor control
- [Commands](commands.md) -- full CLI reference for `--mode` flag
