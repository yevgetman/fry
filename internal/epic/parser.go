package epic

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/yevgetman/fry/internal/config"
)

type parserState int

const (
	stateGlobal parserState = iota
	stateSprintMeta
	stateSprintPrompt
)

func ParseEpic(path string) (*Epic, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open epic file: %w", err)
	}
	defer file.Close()

	ep := &Epic{
		AuditAfterSprint: true, // on by default; use @no_audit to disable
	}
	state := stateGlobal
	scanner := bufio.NewScanner(file)
	lineNo := 0
	var current Sprint
	var promptLines []string
	var maxHealAttemptsSet bool
	var maxFailPercentSet bool

	finalizeSprint := func() {
		if current.Number == 0 {
			return
		}
		current.Prompt = stripPromptBleed(strings.Join(promptLines, "\n"))
		ep.Sprints = append(ep.Sprints, current)
		current = Sprint{}
		promptLines = nil
	}

	for scanner.Scan() {
		lineNo++
		line := strings.TrimRight(scanner.Text(), " \t\r")

		switch state {
		case stateGlobal:
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if directive, value, ok := splitDirective(line); ok {
				switch directive {
				case "@epic":
					ep.Name = value
				case "@engine":
					ep.Engine = value
				case "@docker_from_sprint":
					ep.DockerFromSprint, err = parseIntDirective(directive, value)
				case "@docker_ready_cmd":
					ep.DockerReadyCmd = value
				case "@docker_ready_timeout":
					ep.DockerReadyTimeout, err = parseIntDirective(directive, value)
				case "@require_tool":
					ep.RequiredTools = append(ep.RequiredTools, value)
				case "@preflight_cmd":
					ep.PreflightCmds = append(ep.PreflightCmds, value)
				case "@pre_sprint":
					ep.PreSprintCmd = value
				case "@pre_iteration":
					ep.PreIterationCmd = value
				case "@model", "@codex_model":
					ep.AgentModel = value
				case "@engine_flags", "@codex_flags":
					ep.AgentFlags = value
				case "@verification":
					ep.VerificationFile = value
				case "@max_heal_attempts":
					ep.MaxHealAttempts, err = parseIntDirective(directive, value)
					maxHealAttemptsSet = true
					ep.MaxHealAttemptsSet = true
				case "@max_fail_percent":
					ep.MaxFailPercent, err = parseIntDirective(directive, value)
					maxFailPercentSet = true
					ep.MaxFailPercentSet = true
				case "@compact_with_agent":
					ep.CompactWithAgent = true
				case "@review_between_sprints":
					ep.ReviewBetweenSprints = true
				case "@review_engine":
					ep.ReviewEngine = value
				case "@review_model":
					ep.ReviewModel = value
				case "@max_deviation_scope":
					ep.MaxDeviationScope, err = parseIntDirective(directive, value)
				case "@audit_after_sprint":
					ep.AuditAfterSprint = true
				case "@no_audit":
					ep.AuditAfterSprint = false
				case "@max_audit_iterations":
					ep.MaxAuditIterations, err = parseIntDirective(directive, value)
					ep.MaxAuditIterationsSet = true
				case "@audit_engine":
					ep.AuditEngine = value
				case "@audit_model":
					ep.AuditModel = value
				case "@effort":
					ep.EffortLevel, err = ParseEffortLevel(value)
				case "@sprint":
					current = Sprint{}
					current.Number, err = parseIntDirective(directive, value)
					state = stateSprintMeta
				case "@end":
					state = stateGlobal
				default:
					warnUnknownDirective(line)
				}
				if err != nil {
					return nil, fmt.Errorf("parse epic line %d: %w", lineNo, err)
				}
				continue
			}
		case stateSprintMeta:
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if directive, value, ok := splitDirective(line); ok {
				switch directive {
				case "@name":
					current.Name = value
				case "@max_iterations":
					current.MaxIterations, err = parseIntDirective(directive, value)
				case "@promise":
					current.Promise = value
				case "@max_heal_attempts":
					var heal int
					heal, err = parseIntDirective(directive, value)
					if err == nil {
						current.MaxHealAttempts = &heal
					}
				case "@prompt":
					state = stateSprintPrompt
					promptLines = nil
				case "@end":
					finalizeSprint()
					state = stateGlobal
				default:
					warnUnknownDirective(line)
				}
				if err != nil {
					return nil, fmt.Errorf("parse epic line %d: %w", lineNo, err)
				}
				continue
			}
		case stateSprintPrompt:
			if directive, value, ok := splitDirective(line); ok {
				switch directive {
				case "@sprint":
					finalizeSprint()
					current = Sprint{}
					current.Number, err = parseIntDirective(directive, value)
					if err != nil {
						return nil, fmt.Errorf("parse epic line %d: %w", lineNo, err)
					}
					state = stateSprintMeta
					continue
				case "@end":
					finalizeSprint()
					state = stateGlobal
					continue
				}
			}
			promptLines = append(promptLines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan epic file: %w", err)
	}

	if state == stateSprintPrompt || state == stateSprintMeta {
		finalizeSprint()
	}

	if ep.VerificationFile == "" {
		ep.VerificationFile = config.DefaultVerificationFile
	}
	if ep.DockerReadyTimeout == 0 {
		ep.DockerReadyTimeout = config.DefaultDockerReadyTimeout
	}
	if ep.MaxDeviationScope == 0 {
		ep.MaxDeviationScope = config.DefaultMaxDeviationScope
	}
	if ep.MaxHealAttempts == 0 && !maxHealAttemptsSet {
		ep.MaxHealAttempts = config.DefaultMaxHealAttempts
	}
	if ep.MaxFailPercent == 0 && !maxFailPercentSet {
		ep.MaxFailPercent = config.DefaultMaxFailPercent
	}
	if ep.MaxAuditIterations == 0 && ep.AuditAfterSprint {
		ep.MaxAuditIterations = config.DefaultMaxAuditIterations
	}
	ep.TotalSprints = len(ep.Sprints)

	return ep, nil
}

func splitDirective(line string) (directive string, value string, ok bool) {
	if line == "" || line[0] != '@' {
		return "", "", false
	}
	parts := strings.SplitN(line, " ", 2)
	directive = parts[0]
	if len(parts) == 2 {
		value = strings.TrimSpace(parts[1])
	}
	return directive, value, true
}

func parseIntDirective(name, value string) (int, error) {
	if value == "" {
		return 0, fmt.Errorf("%s requires an integer value", name)
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s requires an integer value: %w", name, err)
	}
	return n, nil
}

func stripPromptBleed(prompt string) string {
	lines := strings.Split(prompt, "\n")
	end := len(lines)
	for end > 0 {
		trimmed := strings.TrimSpace(lines[end-1])
		if trimmed == "" || isMarkdownDivider(trimmed) {
			end--
			continue
		}
		break
	}
	return strings.Join(lines[:end], "\n")
}

func isMarkdownDivider(line string) bool {
	for _, r := range line {
		if r != '#' && r != '=' && r != ' ' {
			return false
		}
	}
	return line != ""
}

func warnUnknownDirective(line string) {
	if isWarnableDirective(line) {
		fmt.Fprintf(os.Stderr, "warning: unrecognized directive: %s\n", line)
	}
}

func isWarnableDirective(line string) bool {
	if len(line) < 2 || line[0] != '@' {
		return false
	}
	return unicode.IsLower(rune(line[1]))
}
