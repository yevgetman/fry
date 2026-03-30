package scan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/textutil"
)

// Memory represents a single codebase-specific learning persisted in
// .fry/codebase-memories/.
type Memory struct {
	Filename   string
	Confidence string // "high", "medium", "low"
	Source     string // build identifier, e.g. "build-2026-03-29-143857"
	Sprint     int
	Date       string // ISO date, e.g. "2026-03-29"
	Reinforced int    // how many subsequent builds confirmed this
	Body       string // the learning itself
}

// MemoryExtractionOpts configures the post-build memory extraction.
type MemoryExtractionOpts struct {
	ProjectDir   string
	Engine       engine.Engine
	Model        string
	BuildID      string // timestamp-based build identifier
	SprintCount  int
	// Collected build context (all optional).
	Scratchpad    string
	Events        string
	SprintSummary string
	GitDiffStat   string
	AuditFindings string
}

const (
	memoryPromptFile = ".fry/memory-extraction-prompt.md"
	maxMemoryInputBytes = 30_000
)

// ExtractCodebaseMemories runs a cheap LLM call to extract project-specific
// learnings from the build, then saves them as individual memory files.
// Handles deduplication against existing memories.
func ExtractCodebaseMemories(ctx context.Context, opts MemoryExtractionOpts) error {
	if opts.Engine == nil {
		return fmt.Errorf("memory extraction: engine is required")
	}

	memoriesDir := filepath.Join(opts.ProjectDir, config.CodebaseMemoriesDir)
	if err := os.MkdirAll(memoriesDir, 0o755); err != nil {
		return fmt.Errorf("memory extraction: create dir: %w", err)
	}

	// Load existing codebase.md to avoid extracting things already documented.
	codebaseMd, _ := os.ReadFile(filepath.Join(opts.ProjectDir, config.CodebaseFile))

	prompt := buildMemoryExtractionPrompt(opts, string(codebaseMd))

	// Write prompt for inspection.
	promptPath := filepath.Join(opts.ProjectDir, memoryPromptFile)
	_ = os.MkdirAll(filepath.Dir(promptPath), 0o755)
	_ = os.WriteFile(promptPath, []byte(prompt), 0o644)

	output, _, err := opts.Engine.Run(ctx, prompt, engine.RunOpts{
		Model:   opts.Model,
		WorkDir: opts.ProjectDir,
	})
	if err != nil && strings.TrimSpace(output) == "" {
		return fmt.Errorf("memory extraction: engine run: %w", err)
	}

	// Clean up transient prompt file.
	_ = os.Remove(promptPath)

	// Parse the output into individual memories.
	memories := parseMemoryOutput(output, opts.BuildID, opts.SprintCount)
	if len(memories) == 0 {
		return nil
	}

	// Load existing memories for deduplication.
	existing, _ := LoadMemories(memoriesDir)

	// Deduplicate and write new memories.
	nextNum := nextMemoryNumber(existing)
	today := time.Now().Format("2006-01-02")
	written := 0

	for _, m := range memories {
		if idx := findDuplicate(existing, m.Body); idx >= 0 {
			// Reinforce existing memory.
			existing[idx].Reinforced++
			if err := writeMemoryFile(filepath.Join(memoriesDir, existing[idx].Filename), existing[idx]); err != nil {
				continue
			}
			continue
		}

		m.Date = today
		m.Source = opts.BuildID
		m.Filename = fmt.Sprintf("%03d-%s-%s.md", nextNum, today, slugify(m.Body, 40))
		nextNum++

		if err := writeMemoryFile(filepath.Join(memoriesDir, m.Filename), m); err != nil {
			continue
		}
		written++
	}

	return nil
}

// LoadMemories reads all memory files from the given directory.
func LoadMemories(dir string) ([]Memory, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var memories []Memory
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, e.Name()))
		if readErr != nil {
			continue
		}
		m := parseMemoryFile(e.Name(), string(data))
		memories = append(memories, m)
	}
	return memories, nil
}

// LoadMemoriesForPrompt loads and formats memories for injection into a prompt,
// respecting the MaxMemoryPromptBytes budget. Returns empty string if no
// memories exist.
func LoadMemoriesForPrompt(projectDir string) string {
	memoriesDir := filepath.Join(projectDir, config.CodebaseMemoriesDir)
	memories, err := LoadMemories(memoriesDir)
	if err != nil || len(memories) == 0 {
		return ""
	}

	// Sort: high confidence first, then most recent, then most reinforced.
	sort.Slice(memories, func(i, j int) bool {
		ci := confidenceRank(memories[i].Confidence)
		cj := confidenceRank(memories[j].Confidence)
		if ci != cj {
			return ci < cj
		}
		if memories[i].Reinforced != memories[j].Reinforced {
			return memories[i].Reinforced > memories[j].Reinforced
		}
		return memories[i].Date > memories[j].Date
	})

	var b strings.Builder
	totalBytes := 0
	for _, m := range memories {
		entry := fmt.Sprintf("- [%s] %s\n", m.Confidence, m.Body)
		if totalBytes+len(entry) > config.MaxMemoryPromptBytes {
			break
		}
		b.WriteString(entry)
		totalBytes += len(entry)
	}
	return b.String()
}

func confidenceRank(c string) int {
	switch strings.ToLower(c) {
	case "high":
		return 0
	case "medium":
		return 1
	case "low":
		return 2
	default:
		return 3
	}
}

func buildMemoryExtractionPrompt(opts MemoryExtractionOpts, codebaseMd string) string {
	var b strings.Builder

	b.WriteString("# Codebase Memory Extraction\n\n")
	b.WriteString("You just completed a build on an existing codebase. Extract codebase-specific\n")
	b.WriteString("learnings that would help future builds on THIS project.\n\n")

	b.WriteString("## What to extract\n\n")
	b.WriteString("- Things about THIS project that would help future builds\n")
	b.WriteString("- Patterns discovered (e.g., \"tests require DATABASE_URL env var\")\n")
	b.WriteString("- Integration points (e.g., \"auth middleware must be registered before routes\")\n")
	b.WriteString("- Gotchas encountered (e.g., \"CI fails if go.sum is not committed\")\n")
	b.WriteString("- Conventions confirmed or discovered\n")
	b.WriteString("- Build process quirks or requirements\n\n")

	b.WriteString("## What NOT to extract\n\n")
	b.WriteString("- Generic programming knowledge\n")
	b.WriteString("- Task-specific details that won't apply to future builds\n")
	b.WriteString("- Information already documented in the codebase description below\n\n")

	b.WriteString("## Output Format\n\n")
	b.WriteString("Output each learning as a block with this exact format:\n\n")
	b.WriteString("```\n")
	b.WriteString("MEMORY: <confidence: high|medium|low>\n")
	b.WriteString("<the learning — 1-3 sentences>\n")
	b.WriteString("---\n")
	b.WriteString("```\n\n")
	b.WriteString("Output ONLY these blocks. No preamble, no summary. If nothing worth\n")
	b.WriteString("remembering was learned, output exactly: NO_MEMORIES\n\n")

	b.WriteString("## Build Context\n\n")

	totalInput := 0
	addSection := func(title, content string) {
		if strings.TrimSpace(content) == "" || totalInput > maxMemoryInputBytes {
			return
		}
		truncated := content
		remaining := maxMemoryInputBytes - totalInput
		if len(truncated) > remaining {
			truncated = textutil.TruncateUTF8(truncated, remaining)
			truncated += "\n... [truncated]\n"
		}
		b.WriteString(fmt.Sprintf("### %s\n\n%s\n\n", title, truncated))
		totalInput += len(truncated)
	}

	addSection("Observer Scratchpad", opts.Scratchpad)
	addSection("Build Events", opts.Events)
	addSection("Sprint Summary", opts.SprintSummary)
	addSection("Git Changes", opts.GitDiffStat)
	addSection("Audit Findings", opts.AuditFindings)

	if len(codebaseMd) > 0 {
		desc := codebaseMd
		if len(desc) > 5000 {
			desc = textutil.TruncateUTF8(desc, 5000) + "\n... [truncated]"
		}
		b.WriteString("### Existing Codebase Description (avoid duplicating this)\n\n")
		b.WriteString(desc)
		b.WriteString("\n\n")
	}

	return b.String()
}

// parseMemoryOutput extracts Memory structs from LLM output.
func parseMemoryOutput(output string, buildID string, sprintCount int) []Memory {
	if strings.TrimSpace(output) == "" || strings.Contains(output, "NO_MEMORIES") {
		return nil
	}

	var memories []Memory
	blocks := strings.Split(output, "---")

	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		// Look for "MEMORY: <confidence>" line.
		lines := strings.Split(block, "\n")
		confidence := ""
		var bodyLines []string

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(strings.ToUpper(trimmed), "MEMORY:") {
				rest := strings.TrimSpace(trimmed[len("MEMORY:"):])
				// Extract confidence from angle brackets or plain text.
				rest = strings.Trim(rest, "<>")
				rest = strings.TrimPrefix(rest, "confidence:")
				rest = strings.TrimSpace(rest)
				switch strings.ToLower(rest) {
				case "high", "medium", "low":
					confidence = strings.ToLower(rest)
				}
			} else if trimmed != "" && trimmed != "```" {
				bodyLines = append(bodyLines, trimmed)
			}
		}

		body := strings.Join(bodyLines, " ")
		if confidence != "" && body != "" {
			memories = append(memories, Memory{
				Confidence: confidence,
				Body:       body,
				Sprint:     sprintCount,
				Source:      buildID,
			})
		}
	}

	return memories
}

// findDuplicate checks if a memory body is substantially similar to an existing one.
// Returns the index of the duplicate, or -1 if none found.
func findDuplicate(existing []Memory, newBody string) int {
	normalizedNew := normalizeForComparison(newBody)
	for i, m := range existing {
		normalizedExisting := normalizeForComparison(m.Body)
		if normalizedNew == normalizedExisting {
			return i
		}
		// Check for high overlap (>=80% shared words).
		if wordOverlap(normalizedNew, normalizedExisting) >= 0.8 {
			return i
		}
	}
	return -1
}

func normalizeForComparison(s string) string {
	s = strings.ToLower(s)
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func wordOverlap(a, b string) float64 {
	setA := toWordSet(strings.Fields(a))
	setB := toWordSet(strings.Fields(b))
	if len(setA) == 0 || len(setB) == 0 {
		return 0
	}

	shared := 0
	for w := range setA {
		if setB[w] {
			shared++
		}
	}

	// Jaccard: |intersection| / |union|.
	union := len(setA) + len(setB) - shared
	if union == 0 {
		return 0
	}
	return float64(shared) / float64(union)
}

func toWordSet(words []string) map[string]bool {
	s := make(map[string]bool, len(words))
	for _, w := range words {
		s[w] = true
	}
	return s
}

// parseMemoryFile reads a memory file and extracts its frontmatter and body.
func parseMemoryFile(filename, content string) Memory {
	m := Memory{Filename: filename}

	// Parse YAML-like frontmatter between --- delimiters.
	if strings.HasPrefix(content, "---") {
		endIdx := strings.Index(content[3:], "---")
		if endIdx >= 0 {
			frontmatter := content[3 : endIdx+3]
			m.Body = strings.TrimSpace(content[endIdx+6:])

			for _, line := range strings.Split(frontmatter, "\n") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) != 2 {
					continue
				}
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				switch key {
				case "confidence":
					m.Confidence = val
				case "source":
					m.Source = val
				case "sprint":
					fmt.Sscanf(val, "%d", &m.Sprint)
				case "date":
					m.Date = val
				case "reinforced":
					fmt.Sscanf(val, "%d", &m.Reinforced)
				}
			}
		} else {
			m.Body = strings.TrimSpace(content)
		}
	} else {
		m.Body = strings.TrimSpace(content)
	}

	if m.Confidence == "" {
		m.Confidence = "medium"
	}
	return m
}

// writeMemoryFile writes a memory to disk with frontmatter.
func writeMemoryFile(path string, m Memory) error {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("confidence: %s\n", m.Confidence))
	b.WriteString(fmt.Sprintf("source: %s\n", m.Source))
	b.WriteString(fmt.Sprintf("sprint: %d\n", m.Sprint))
	b.WriteString(fmt.Sprintf("date: %s\n", m.Date))
	b.WriteString(fmt.Sprintf("reinforced: %d\n", m.Reinforced))
	b.WriteString("---\n\n")
	b.WriteString(m.Body)
	b.WriteString("\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func nextMemoryNumber(existing []Memory) int {
	max := 0
	for _, m := range existing {
		// Extract number from filename like "001-2026-03-29-slug.md"
		var num int
		if _, err := fmt.Sscanf(m.Filename, "%03d-", &num); err == nil && num > max {
			max = num
		}
	}
	return max + 1
}

func slugify(text string, maxLen int) string {
	words := strings.Fields(strings.ToLower(text))
	var slug []string
	total := 0
	for _, w := range words {
		// Only keep alphanumeric words.
		cleaned := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				return r
			}
			return -1
		}, w)
		if cleaned == "" {
			continue
		}
		if total+len(cleaned)+1 > maxLen {
			break
		}
		slug = append(slug, cleaned)
		total += len(cleaned) + 1
	}
	if len(slug) == 0 {
		return "memory"
	}
	return strings.Join(slug, "-")
}
