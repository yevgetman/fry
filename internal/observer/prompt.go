package observer

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/textutil"
)

// buildObserverPrompt constructs the prompt for an observer wake-up.
func buildObserverPrompt(opts ObserverOpts, identity, scratchpad string, events []Event) string {
	var b strings.Builder

	// 1. System role preamble
	b.WriteString("# OBSERVER — Metacognitive Build Layer\n\n")
	b.WriteString("You are the Observer — a persistent metacognitive layer within Fry.\n")
	b.WriteString("Your role is to watch builds unfold, notice patterns, and develop insight over time.\n")
	b.WriteString("You do NOT modify source code. You observe, reflect, and record.\n\n")
	b.WriteString(fmt.Sprintf("**Epic:** %s\n", opts.EpicName))
	b.WriteString(fmt.Sprintf("**Sprint:** %d/%d\n", opts.SprintNum, opts.TotalSprints))
	b.WriteString(fmt.Sprintf("**Effort:** %s\n\n", opts.EffortLevel))

	// 2. Current identity document (truncated)
	b.WriteString("## Your Identity\n\n")
	if identity == "" {
		b.WriteString("(No identity document found — you are waking up for the first time.)\n\n")
	} else {
		truncated := identity
		if len(truncated) > config.MaxObserverIdentityBytes {
			truncated = textutil.TruncateUTF8(truncated, config.MaxObserverIdentityBytes) + "\n...(truncated)"
		}
		b.WriteString(truncated)
		b.WriteString("\n\n")
	}

	// 3. Current scratchpad (truncated)
	b.WriteString("## Your Scratchpad (This Build)\n\n")
	if scratchpad == "" {
		b.WriteString("(Empty — this is either the first wake-up this build, or the scratchpad was just reset.)\n\n")
	} else {
		truncated := scratchpad
		if len(truncated) > config.MaxObserverScratchpadBytes {
			truncated = textutil.TruncateUTF8(truncated, config.MaxObserverScratchpadBytes) + "\n...(truncated)"
		}
		b.WriteString(truncated)
		b.WriteString("\n\n")
	}

	// 4. Recent events
	b.WriteString("## Recent Build Events\n\n")
	if len(events) == 0 {
		b.WriteString("(No events recorded yet.)\n\n")
	} else {
		b.WriteString("```jsonl\n")
		for _, evt := range events {
			line, err := json.Marshal(evt)
			if err != nil {
				continue
			}
			b.Write(line)
			b.WriteByte('\n')
		}
		b.WriteString("```\n\n")
	}

	// 5. Wake-point context
	b.WriteString("## Wake-Point Context\n\n")
	switch opts.WakePoint {
	case WakeAfterSprint:
		b.WriteString(fmt.Sprintf("You are waking up **after sprint %d** completed.\n", opts.SprintNum))
		b.WriteString("Reflect on the sprint's execution: iterations, alignment loops, audit findings.\n")
		b.WriteString("Compare what happened with what was expected.\n\n")
	case WakeAfterBuildAudit:
		b.WriteString("You are waking up **after the build-level audit** completed.\n")
		b.WriteString("The entire codebase has been audited. Reflect on cross-cutting findings.\n")
		b.WriteString("Consider patterns across all sprints.\n\n")
	case WakeBuildEnd:
		b.WriteString("You are waking up **at build end** — the final observation point.\n")
		b.WriteString("Synthesize everything: what worked, what struggled, what you would watch for next time.\n\n")
	default:
		b.WriteString(fmt.Sprintf("Wake point: %s\n\n", opts.WakePoint))
	}

	// Include any build data (sorted for deterministic output)
	if len(opts.BuildData) > 0 {
		b.WriteString("### Additional Build Data\n\n")
		keys := make([]string, 0, len(opts.BuildData))
		for k := range opts.BuildData {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(fmt.Sprintf("- **%s:** %s\n", k, opts.BuildData[k]))
		}
		b.WriteString("\n")
	}

	// 6. Output format instructions
	b.WriteString("## Output Format\n\n")
	b.WriteString("Structure your response using these XML-style tags. Output the tags directly — do NOT wrap them in code fences or backticks.\n\n")
	b.WriteString("**Required tags:**\n\n")
	b.WriteString("  <thoughts>\n")
	b.WriteString("  Your observations, reflections, and analysis. What did you notice?\n")
	b.WriteString("  What patterns are emerging? What concerns or insights do you have?\n")
	b.WriteString("  </thoughts>\n\n")
	b.WriteString("  <scratchpad>\n")
	b.WriteString("  Notes for your next wake-up this build. Track ongoing patterns,\n")
	b.WriteString("  hypotheses to verify, things to watch for in subsequent sprints.\n")
	b.WriteString("  </scratchpad>\n\n")
	b.WriteString("**Optional tags (omit entirely if not needed):**\n\n")
	b.WriteString("  <directives>\n")
	b.WriteString("  Structured directives for the build system. One per line.\n")
	b.WriteString("  Format: TYPE: value\n")
	b.WriteString("  Supported types: WARN, NOTE, SUGGEST\n")
	b.WriteString("  Example: WARN: alignment loop on sprint 3 appears stuck on the same error\n")
	b.WriteString("  </directives>\n")

	return b.String()
}

// knownTags lists the tag names the observer response parser recognizes.
var knownTags = []string{"thoughts", "scratchpad", "directives"}

// tagPatterns maps tag names to compiled regexes for extracting their content.
var tagPatterns = buildTagPatterns(knownTags)

func buildTagPatterns(tags []string) map[string]*regexp.Regexp {
	m := make(map[string]*regexp.Regexp, len(tags))
	for _, tag := range tags {
		m[tag] = regexp.MustCompile(`(?s)<` + tag + `>(.*?)</` + tag + `>`)
	}
	return m
}

// parseObserverResponse extracts structured output from LLM response.
func parseObserverResponse(output string) (*Observation, error) {
	if output == "" {
		return &Observation{}, nil
	}

	tags := extractAllTags(output)

	obs := &Observation{}

	thoughts, hasThoughts := tags["thoughts"]
	if hasThoughts {
		obs.Thoughts = strings.TrimSpace(thoughts)
	}

	scratchpad, hasScratchpad := tags["scratchpad"]
	if hasScratchpad {
		obs.ScratchpadDelta = strings.TrimSpace(scratchpad)
	}

	directivesRaw, hasDirectives := tags["directives"]
	if hasDirectives {
		obs.Directives = parseDirectives(strings.TrimSpace(directivesRaw))
	}

	// If no tags were found at all, treat entire output as thoughts (fallback)
	if !hasThoughts && !hasScratchpad && !hasDirectives {
		return &Observation{Thoughts: strings.TrimSpace(output)}, fmt.Errorf("no structured tags found in response")
	}

	return obs, nil
}

// extractAllTags extracts all known XML-style tag contents from the output.
func extractAllTags(output string) map[string]string {
	result := make(map[string]string)
	for tag, pattern := range tagPatterns {
		match := pattern.FindStringSubmatch(output)
		if len(match) == 2 {
			result[tag] = match[1]
		}
	}
	return result
}

// parseDirectives parses directive lines in the format "TYPE: value".
func parseDirectives(raw string) []Directive {
	if raw == "" {
		return nil
	}
	var directives []Directive
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		dirType := strings.TrimSpace(line[:idx])
		dirValue := strings.TrimSpace(line[idx+1:])
		if dirType != "" && dirValue != "" {
			directives = append(directives, Directive{Type: dirType, Value: dirValue})
		}
	}
	return directives
}
