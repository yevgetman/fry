package scan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/textutil"
)

const (
	// maxKeyFileBytes caps total bytes of key file content included in the
	// semantic scan prompt to keep it within model context limits.
	maxKeyFileBytes = 50_000

	// maxSingleFileBytes caps a single key file read to prevent one large
	// file from consuming the entire budget.
	maxSingleFileBytes = 10_000

	// maxKeyFiles limits how many key files are included in the prompt.
	maxKeyFiles = 20

	// scanPromptFile is where the assembled scan prompt is written for
	// debugging/inspection.
	scanPromptFile = ".fry/codebase-scan-prompt.md"
)

// SemanticScanOpts configures the semantic scan.
type SemanticScanOpts struct {
	ProjectDir  string
	Snapshot    *StructuralSnapshot
	Engine      engine.Engine
	Model       string // resolved model (Sonnet-class)
	EffortLevel string
	Verbose     bool
}

// RunSemanticScan uses an LLM to produce .fry-config/codebase.md from the structural
// snapshot. It selects key files, assembles a prompt, invokes the engine, and
// writes the result.
func RunSemanticScan(ctx context.Context, opts SemanticScanOpts) error {
	if opts.Engine == nil {
		return fmt.Errorf("semantic scan: engine is required")
	}
	if opts.Snapshot == nil {
		return fmt.Errorf("semantic scan: snapshot is required")
	}

	prompt := assembleSemanticPrompt(opts.Snapshot, opts.ProjectDir)

	// Write prompt for inspection.
	promptPath := filepath.Join(opts.ProjectDir, scanPromptFile)
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		return fmt.Errorf("semantic scan: create dir: %w", err)
	}
	if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
		return fmt.Errorf("semantic scan: write prompt: %w", err)
	}

	codebasePath := filepath.Join(opts.ProjectDir, config.CodebaseFile)
	invocation := fmt.Sprintf(
		"Read and execute ALL instructions in %s. "+
			"Write your analysis to %s. Overwrite the file if it exists. "+
			"Do NOT modify any source code.",
		scanPromptFile, config.CodebaseFile,
	)

	runOpts := engine.RunOpts{
		Model:       opts.Model,
		SessionType: engine.SessionCodebaseScan,
		EffortLevel: opts.EffortLevel,
		WorkDir:     opts.ProjectDir,
	}

	_, _, err := opts.Engine.Run(ctx, invocation, runOpts)
	if err != nil {
		return fmt.Errorf("semantic scan: engine run: %w", err)
	}

	// Verify the output file was created.
	if _, statErr := os.Stat(codebasePath); os.IsNotExist(statErr) {
		return fmt.Errorf("semantic scan: engine did not write %s", config.CodebaseFile)
	}

	// Clean up the transient prompt file.
	_ = os.Remove(promptPath)

	return nil
}

// UpdateCodebaseDoc incrementally updates .fry-config/codebase.md based on a git diff.
// Only called when significant changes are detected (>=5 files or new packages).
func UpdateCodebaseDoc(ctx context.Context, projectDir string, diffSummary string, eng engine.Engine, model string) error {
	codebasePath := filepath.Join(projectDir, config.CodebaseFile)
	existing, err := os.ReadFile(codebasePath)
	if err != nil {
		return fmt.Errorf("update codebase doc: read existing: %w", err)
	}

	prompt := buildUpdatePrompt(string(existing), diffSummary)

	updatePromptFile := ".fry/codebase-update-prompt.md"
	promptPath := filepath.Join(projectDir, updatePromptFile)
	_ = os.MkdirAll(filepath.Dir(promptPath), 0o755)
	_ = os.WriteFile(promptPath, []byte(prompt), 0o644)

	invocation := fmt.Sprintf(
		"Read and execute ALL instructions in %s. "+
			"Update %s in place. Do NOT modify any source code.",
		updatePromptFile, config.CodebaseFile,
	)

	_, _, runErr := eng.Run(ctx, invocation, engine.RunOpts{
		Model:       model,
		SessionType: engine.SessionCodebaseScan,
		WorkDir:     projectDir,
	})

	_ = os.Remove(promptPath)

	if runErr != nil {
		return fmt.Errorf("update codebase doc: engine run: %w", runErr)
	}
	return nil
}

func buildUpdatePrompt(existingDoc, diffSummary string) string {
	var b strings.Builder
	b.WriteString("# Codebase Document Update\n\n")
	b.WriteString("The following is the current codebase.md document. A build just completed\n")
	b.WriteString("that modified the codebase. Update ONLY the sections affected by the changes.\n")
	b.WriteString("Preserve all other sections exactly as they are.\n\n")
	b.WriteString("## Changes Made\n\n")
	b.WriteString(diffSummary)
	b.WriteString("\n\n## Current Document\n\n")

	doc := existingDoc
	if len(doc) > 20000 {
		doc = textutil.TruncateUTF8(doc, 20000) + "\n... [truncated]"
	}
	b.WriteString(doc)
	b.WriteString("\n\n## Instructions\n\n")
	b.WriteString("Update " + config.CodebaseFile + " to reflect the changes. Only modify sections that\n")
	b.WriteString("are affected. Do NOT rewrite the entire document. If the changes are minor\n")
	b.WriteString("and don't affect any section, make no changes.\n")
	return b.String()
}

// ShouldUpdateCodebaseDoc returns true if the git diff is significant enough
// to warrant an incremental codebase.md update. Counts files changed by
// looking for "diff --git" headers in unified diff output.
func ShouldUpdateCodebaseDoc(diffOutput string) bool {
	if strings.TrimSpace(diffOutput) == "" {
		return false
	}
	fileCount := 0
	for _, line := range strings.Split(diffOutput, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			fileCount++
		}
	}
	return fileCount >= 5
}

// assembleSemanticPrompt builds the full prompt for the LLM codebase analysis.
func assembleSemanticPrompt(snap *StructuralSnapshot, projectDir string) string {
	var b strings.Builder

	b.WriteString("# Codebase Analysis Task\n\n")
	b.WriteString("You are analyzing an existing software project. Your job is to produce a comprehensive\n")
	b.WriteString("codebase understanding document. Write your analysis as Markdown.\n\n")
	b.WriteString("## Output Format\n\n")
	b.WriteString("Write the following sections in this exact order:\n\n")
	b.WriteString("```\n")
	b.WriteString("# Codebase: {project name}\n\n")
	b.WriteString("## Summary\n")
	b.WriteString("What this project is, what it does, who it's for. 2-3 sentences.\n\n")
	b.WriteString("## Architecture\n")
	b.WriteString("How the project is organized. Key packages/modules, their responsibilities,\n")
	b.WriteString("and how they relate. Data flow for the primary use case.\n\n")
	b.WriteString("## Tech Stack\n")
	b.WriteString("Languages, frameworks, key dependencies, build tools, test frameworks.\n\n")
	b.WriteString("## Key Files\n")
	b.WriteString("The files that matter most and why. Entry points, core business logic,\n")
	b.WriteString("configuration, database schemas.\n\n")
	b.WriteString("## Conventions\n")
	b.WriteString("Naming patterns, error handling style, test patterns, code organization\n")
	b.WriteString("rules, import grouping. Things an agent must follow to write consistent code.\n\n")
	b.WriteString("## Entry Points\n")
	b.WriteString("Where execution starts. Main binaries, API endpoints, CLI commands,\n")
	b.WriteString("event handlers.\n\n")
	b.WriteString("## Dependencies\n")
	b.WriteString("External services, databases, APIs, third-party libraries that the\n")
	b.WriteString("project relies on and how they're used.\n\n")
	b.WriteString("## Test Structure\n")
	b.WriteString("How tests are organized, what frameworks are used, how to run them,\n")
	b.WriteString("any test utilities or fixtures.\n\n")
	b.WriteString("## Git Activity\n")
	b.WriteString("Recent development focus, frequently modified areas, files that\n")
	b.WriteString("tend to change together.\n\n")
	b.WriteString("## Gotchas\n")
	b.WriteString("Things that aren't obvious from reading individual files. Implicit\n")
	b.WriteString("dependencies, environment requirements, ordering constraints,\n")
	b.WriteString("known quirks.\n")
	b.WriteString("```\n\n")

	// Section: Structural data.
	b.WriteString("---\n\n")
	b.WriteString("## Input Data\n\n")

	// File stats.
	b.WriteString("### Project Statistics\n")
	b.WriteString(fmt.Sprintf("- Total files: %d\n", snap.FileStats.TotalFiles))
	b.WriteString(fmt.Sprintf("- Total directories: %d\n", snap.FileStats.TotalDirs))
	b.WriteString(fmt.Sprintf("- Total size: %s\n", FormatSize(snap.FileStats.TotalSize)))
	b.WriteString("\n")

	// Languages.
	if len(snap.Languages) > 0 {
		b.WriteString("### Detected Languages\n")
		for _, lang := range snap.Languages {
			b.WriteString(fmt.Sprintf("- %s (confidence: %s, evidence: %s)\n", lang.Name, lang.Confidence, lang.Marker))
		}
		b.WriteString("\n")
	}

	// Frameworks.
	if len(snap.Frameworks) > 0 {
		b.WriteString("### Detected Frameworks\n")
		for _, fw := range snap.Frameworks {
			b.WriteString(fmt.Sprintf("- %s\n", fw))
		}
		b.WriteString("\n")
	}

	// Entry points.
	if len(snap.EntryPoints) > 0 {
		b.WriteString("### Entry Points\n")
		for _, ep := range snap.EntryPoints {
			b.WriteString(fmt.Sprintf("- %s\n", ep))
		}
		b.WriteString("\n")
	}

	// Dependencies.
	if len(snap.Dependencies) > 0 {
		b.WriteString("### Dependencies\n")
		for _, dep := range snap.Dependencies {
			if dep.Version != "" {
				b.WriteString(fmt.Sprintf("- %s %s (from %s)\n", dep.Name, dep.Version, dep.Source))
			} else {
				b.WriteString(fmt.Sprintf("- %s (from %s)\n", dep.Name, dep.Source))
			}
		}
		b.WriteString("\n")
	}

	// Test directories.
	if len(snap.TestDirs) > 0 {
		b.WriteString("### Test Directories\n")
		for _, td := range snap.TestDirs {
			b.WriteString(fmt.Sprintf("- %s\n", td))
		}
		b.WriteString("\n")
	}

	// Doc files.
	if len(snap.DocFiles) > 0 {
		b.WriteString("### Documentation Files\n")
		for _, df := range snap.DocFiles {
			b.WriteString(fmt.Sprintf("- %s\n", df))
		}
		b.WriteString("\n")
	}

	// Top directories.
	if len(snap.FileStats.TopDirectories) > 0 {
		b.WriteString("### Top Directories by File Count\n")
		for _, td := range snap.FileStats.TopDirectories {
			b.WriteString(fmt.Sprintf("- %s/ (%d files)\n", td.RelPath, td.FileCount))
		}
		b.WriteString("\n")
	}

	// Extension breakdown.
	if len(snap.FileStats.CountByExt) > 0 {
		b.WriteString("### Files by Extension\n")
		type extCount struct {
			ext   string
			count int
		}
		var exts []extCount
		for ext, count := range snap.FileStats.CountByExt {
			exts = append(exts, extCount{ext, count})
		}
		sort.Slice(exts, func(i, j int) bool { return exts[i].count > exts[j].count })
		for _, ec := range exts {
			b.WriteString(fmt.Sprintf("- %s: %d files\n", ec.ext, ec.count))
		}
		b.WriteString("\n")
	}

	// Git history.
	if snap.GitHistory != nil {
		b.WriteString("### Git History\n")
		b.WriteString(fmt.Sprintf("- Total commits: %d\n", snap.GitHistory.TotalCommits))

		if len(snap.GitHistory.RecentLog) > 0 {
			b.WriteString("\nRecent commits:\n")
			for _, entry := range snap.GitHistory.RecentLog {
				b.WriteString(fmt.Sprintf("  %s\n", entry))
			}
		}

		if len(snap.GitHistory.HotFiles) > 0 {
			b.WriteString("\nMost frequently changed files:\n")
			for _, hf := range snap.GitHistory.HotFiles {
				b.WriteString(fmt.Sprintf("  %s (%d changes)\n", hf.RelPath, hf.ChangeCount))
			}
		}

		if len(snap.GitHistory.TopAuthors) > 0 {
			b.WriteString("\nTop contributors:\n")
			for _, author := range snap.GitHistory.TopAuthors {
				b.WriteString(fmt.Sprintf("  %s\n", author))
			}
		}
		b.WriteString("\n")
	}

	// README content.
	if snap.ReadmeContent != "" {
		b.WriteString("### README Content\n")
		b.WriteString("```\n")
		b.WriteString(snap.ReadmeContent)
		if !strings.HasSuffix(snap.ReadmeContent, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("```\n\n")
	}

	// Key file contents.
	keyFiles := selectKeyFiles(snap, projectDir)
	if len(keyFiles) > 0 {
		b.WriteString("### Key File Contents\n\n")
		for _, kf := range keyFiles {
			b.WriteString(fmt.Sprintf("#### %s\n", kf.relPath))
			b.WriteString("```\n")
			b.WriteString(kf.content)
			if !strings.HasSuffix(kf.content, "\n") {
				b.WriteString("\n")
			}
			b.WriteString("```\n\n")
		}
	}

	return b.String()
}

// keyFile holds a file path and its content for prompt inclusion.
type keyFile struct {
	relPath  string
	content  string
	priority int // lower = higher priority
}

// selectKeyFiles picks the most important files to include in the semantic
// scan prompt, respecting the byte budget.
func selectKeyFiles(snap *StructuralSnapshot, projectDir string) []keyFile {
	var candidates []keyFile

	// Priority 1: Entry points (main files, index files).
	for _, ep := range snap.EntryPoints {
		candidates = append(candidates, keyFile{relPath: ep, priority: 1})
	}

	// Priority 2: Config/manifest files.
	for _, f := range snap.FileTree {
		base := filepath.Base(f.RelPath)
		switch base {
		case "go.mod", "package.json", "Cargo.toml", "pyproject.toml",
			"requirements.txt", "Gemfile", "pom.xml", "build.gradle",
			"tsconfig.json", "webpack.config.js", "vite.config.ts",
			"Dockerfile", "docker-compose.yml", "compose.yml",
			".env.example", "Makefile":
			candidates = append(candidates, keyFile{relPath: f.RelPath, priority: 2})
		}
	}

	// Priority 3: Doc files that aren't README (already in ReadmeContent).
	for _, f := range snap.FileTree {
		base := filepath.Base(f.RelPath)
		switch base {
		case "AGENTS.md", "CLAUDE.md", "CONTRIBUTING.md", "ARCHITECTURE.md":
			candidates = append(candidates, keyFile{relPath: f.RelPath, priority: 3})
		}
	}

	// Priority 4: Type definition files (types.go, models.py, etc.).
	for _, f := range snap.FileTree {
		base := strings.ToLower(filepath.Base(f.RelPath))
		// Skip test files.
		if strings.Contains(base, "_test") || strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") {
			continue
		}
		if strings.Contains(base, "types") || strings.Contains(base, "model") ||
			strings.Contains(base, "schema") || strings.Contains(base, "config") {
			if f.Size > 0 && f.Size < maxSingleFileBytes {
				candidates = append(candidates, keyFile{relPath: f.RelPath, priority: 4})
			}
		}
	}

	// Deduplicate.
	seen := make(map[string]bool)
	var unique []keyFile
	for _, c := range candidates {
		if !seen[c.relPath] {
			seen[c.relPath] = true
			unique = append(unique, c)
		}
	}

	// Sort by priority.
	sort.Slice(unique, func(i, j int) bool {
		if unique[i].priority != unique[j].priority {
			return unique[i].priority < unique[j].priority
		}
		return unique[i].relPath < unique[j].relPath
	})

	// Read files within budget.
	var result []keyFile
	totalBytes := 0
	for _, kf := range unique {
		if len(result) >= maxKeyFiles {
			break
		}
		absPath := filepath.Join(projectDir, kf.relPath)
		data, err := os.ReadFile(absPath)
		if err != nil {
			continue
		}
		content := string(data)

		// Truncate individual files.
		if len(content) > maxSingleFileBytes {
			content = textutil.TruncateUTF8(content, maxSingleFileBytes)
			content += "\n... [truncated]\n"
		}

		if totalBytes+len(content) > maxKeyFileBytes {
			break
		}

		totalBytes += len(content)
		kf.content = content
		result = append(result, kf)
	}

	return result
}
