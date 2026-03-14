# Planning Mode

Fry's execution engine is project-agnostic — the sprint loop, verification runner, and heal loop work identically regardless of whether the output is code or documents.

Pass `--planning` to use planning-domain prompts that generate sprints for producing structured documents instead of code. Use it for business plans, trip planning, research reports, strategic analyses, or any endeavor that requires rigorous, phased document creation.

## Usage

```bash
# Generate and run in planning mode
fry --planning --engine claude

# Generate artifacts only
fry prepare --planning --engine claude
```

## How It Differs from Software Mode

| Aspect | Default (software) | `--planning` |
|---|---|---|
| `.fry/AGENTS.md` | Technology constraints, architecture rules, testing patterns | Domain boundaries, analytical frameworks, document quality standards |
| Sprint phasing | Scaffolding → Schema → Logic → Integration → E2E | Research → Analysis → Strategy → Detailed Planning → Synthesis |
| Sprint deliverables | Source files, configs, tests | Markdown documents, analyses, strategies |
| Verification | Build succeeds, tests pass, files exist | Documents exist, contain required sections, meet minimum depth |

## Quick Start

```bash
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

# Generate and run
fry --planning --engine claude
```

## Output Directory and Naming Convention

In planning mode, all document deliverables are written to `plans/output/`, keeping the `plans/` directory reserved for input files (`executive.md`, `plan.md`).

Output filenames use an ordered, categorized naming convention:

```
{sequence}--{category}--{name}.md
```

- **sequence** — a globally unique number (1, 2, 3...) indicating production order across all sprints
- **category** — a short grouping label (e.g., `research`, `analysis`, `strategy`, `synthesis`)
- **name** — a descriptive kebab-case name

Example directory layout:

```
plans/
  executive.md                              # INPUT: your executive context
  plan.md                                   # INPUT: generated or authored plan
  output/                                   # OUTPUT: all deliverables
    1--research--market-landscape.md
    2--research--competitor-profiles.md
    3--analysis--positioning-options.md
    4--strategy--go-to-market.md
    5--synthesis--executive-summary.md
```

## Verification for Documents

The same four [verification check primitives](verification.md) work for document deliverables:

```
@check_file plans/output/1--research--market-landscape.md
@check_file_contains plans/output/1--research--market-landscape.md "## Market Size"
@check_cmd test $(wc -w < plans/output/1--research--market-landscape.md) -ge 500
@check_cmd_output grep -c '^## ' plans/output/1--research--market-landscape.md | ^[5-9]
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

## When to Use Planning Mode

- Business plans and strategies
- Research reports and literature reviews
- Trip itineraries and travel planning
- Strategic analyses and competitive assessments
- Project proposals and feasibility studies
- Any multi-phase document creation that benefits from structured decomposition
