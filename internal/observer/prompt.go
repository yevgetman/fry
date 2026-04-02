package observer

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/consciousness"
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
	b.WriteString("Respond with a single JSON object. Do NOT include any text outside the JSON.\n\n")
	b.WriteString("**Required fields:**\n\n")
	b.WriteString("- `\"thoughts\"`: Your observations, reflections, and analysis. What did you notice?\n")
	b.WriteString("  What patterns are emerging? What concerns or insights do you have?\n")
	b.WriteString("- `\"scratchpad\"`: Notes for your next wake-up this build. Track ongoing patterns,\n")
	b.WriteString("  hypotheses to verify, things to watch for in subsequent sprints.\n\n")
	b.WriteString("**Optional fields (omit if not needed):**\n\n")
	b.WriteString("- `\"directives\"`: Structured directives for the build system. One per line,\n")
	b.WriteString("  separated by newlines within the string. Format: TYPE: value.\n")
	b.WriteString("  Supported types: WARN, NOTE, SUGGEST.\n")
	b.WriteString("  Example: `\"WARN: alignment loop stuck\\nNOTE: coverage improved\"`\n\n")
	b.WriteString("Example:\n")
	b.WriteString("```json\n")
	b.WriteString("{\n")
	b.WriteString("  \"thoughts\": \"The build is progressing well...\",\n")
	b.WriteString("  \"scratchpad\": \"Watch sprint 2 for test failures...\",\n")
	b.WriteString("  \"directives\": \"NOTE: Sprint 1 used only 3 of 10 iterations\"\n")
	b.WriteString("}\n")
	b.WriteString("```\n")

	return b.String()
}

// observerJSON is the expected JSON structure from the observer.
type observerJSON struct {
	Thoughts   string `json:"thoughts"`
	Scratchpad string `json:"scratchpad"`
	Directives string `json:"directives,omitempty"`
}

// parseObserverResponse extracts structured output from LLM response.
func parseObserverResponse(output string) (*Observation, error) {
	if strings.TrimSpace(output) == "" {
		return &Observation{
			ParseStatus: consciousness.ParseStatusFailed,
			ParseError:  "empty observer output",
		}, fmt.Errorf("empty observer output")
	}

	var parsed observerJSON
	diag, err := textutil.ExtractJSONWithDiagnostics(output, &parsed)
	if err != nil {
		return &Observation{
			ParseStatus: consciousness.ParseStatusFailed,
			ParseError:  err.Error(),
		}, err
	}

	obs := &Observation{
		Thoughts:        strings.TrimSpace(parsed.Thoughts),
		ScratchpadDelta: strings.TrimSpace(parsed.Scratchpad),
	}
	if diag.Repaired {
		obs.ParseStatus = consciousness.ParseStatusRepaired
	} else {
		obs.ParseStatus = consciousness.ParseStatusOK
	}

	if d := strings.TrimSpace(parsed.Directives); d != "" {
		obs.Directives = parseDirectives(d)
	}
	if obs.Thoughts == "" && obs.ScratchpadDelta == "" {
		obs.ParseStatus = consciousness.ParseStatusFailed
		obs.ParseError = "observer JSON did not contain thoughts or scratchpad"
		return obs, fmt.Errorf("observer JSON did not contain thoughts or scratchpad")
	}

	return obs, nil
}

// parseDirectives parses directive lines in the format "TYPE: value".
func parseDirectives(raw string) []consciousness.Directive {
	if raw == "" {
		return nil
	}
	var directives []consciousness.Directive
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
			directives = append(directives, consciousness.Directive{Type: dirType, Value: dirValue})
		}
	}
	return directives
}
