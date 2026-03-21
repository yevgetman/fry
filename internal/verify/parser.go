package verify

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	frylog "github.com/yevgetman/fry/internal/log"
)

func ParseVerification(path string) ([]Check, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open verification file: %w", err)
	}
	defer file.Close()

	var checks []Check
	currentSprint := 0
	scanner := bufio.NewScanner(file)
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		line := strings.TrimRight(scanner.Text(), " \t\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		switch {
		case strings.HasPrefix(line, "@sprint "):
			value := strings.TrimSpace(strings.TrimPrefix(line, "@sprint "))
			currentSprint, err = strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("parse verification line %d: invalid sprint number: %w", lineNo, err)
			}
		case strings.HasPrefix(line, "@check_file_contains "):
			if currentSprint == 0 {
				frylog.Log("WARNING: verification line %d: check before any @sprint directive (will never run)", lineNo)
			}
			check, parseErr := parseFileContains(line, currentSprint)
			if parseErr != nil {
				return nil, fmt.Errorf("parse verification line %d: %w", lineNo, parseErr)
			}
			checks = append(checks, check)
		case strings.HasPrefix(line, "@check_file "):
			if currentSprint == 0 {
				frylog.Log("WARNING: verification line %d: check before any @sprint directive (will never run)", lineNo)
			}
			pathValue := strings.TrimSpace(strings.TrimPrefix(line, "@check_file "))
			if pathValue == "" {
				return nil, fmt.Errorf("parse verification line %d: @check_file requires a path", lineNo)
			}
			checks = append(checks, Check{Sprint: currentSprint, Type: CheckFile, Path: pathValue})
		case strings.HasPrefix(line, "@check_cmd_output "):
			if currentSprint == 0 {
				frylog.Log("WARNING: verification line %d: check before any @sprint directive (will never run)", lineNo)
			}
			check, parseErr := parseCmdOutput(line, currentSprint)
			if parseErr != nil {
				return nil, fmt.Errorf("parse verification line %d: %w", lineNo, parseErr)
			}
			checks = append(checks, check)
		case strings.HasPrefix(line, "@check_cmd "):
			if currentSprint == 0 {
				frylog.Log("WARNING: verification line %d: check before any @sprint directive (will never run)", lineNo)
			}
			command := strings.TrimSpace(strings.TrimPrefix(line, "@check_cmd "))
			if command == "" {
				return nil, fmt.Errorf("parse verification line %d: @check_cmd requires a command", lineNo)
			}
			checks = append(checks, Check{Sprint: currentSprint, Type: CheckCmd, Command: command})
		case strings.HasPrefix(line, "@check_test "):
			if currentSprint == 0 {
				frylog.Log("WARNING: verification line %d: check before any @sprint directive (will never run)", lineNo)
			}
			command := strings.TrimSpace(strings.TrimPrefix(line, "@check_test "))
			if command == "" {
				return nil, fmt.Errorf("parse verification line %d: @check_test requires a command", lineNo)
			}
			checks = append(checks, Check{Sprint: currentSprint, Type: CheckTest, Command: command})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan verification file: %w", err)
	}

	return checks, nil
}

func parseFileContains(line string, sprint int) (Check, error) {
	remaining := strings.TrimSpace(strings.TrimPrefix(line, "@check_file_contains "))
	if remaining == "" {
		return Check{}, fmt.Errorf("@check_file_contains requires a path and pattern")
	}

	sep := strings.Index(remaining, " ")
	if sep < 0 {
		return Check{}, fmt.Errorf("@check_file_contains requires a path and pattern")
	}

	path := strings.TrimSpace(remaining[:sep])
	pattern := strings.TrimSpace(remaining[sep+1:])
	if path == "" || pattern == "" {
		return Check{}, fmt.Errorf("@check_file_contains requires a path and pattern")
	}

	return Check{
		Sprint:  sprint,
		Type:    CheckFileContains,
		Path:    path,
		Pattern: unquotePattern(pattern),
	}, nil
}

func parseCmdOutput(line string, sprint int) (Check, error) {
	remaining := strings.TrimSpace(strings.TrimPrefix(line, "@check_cmd_output "))
	parts := strings.SplitN(remaining, " | ", 2)
	if len(parts) != 2 {
		return Check{}, fmt.Errorf("@check_cmd_output requires command and pattern separated by ' | '")
	}

	command := strings.TrimSpace(parts[0])
	pattern := strings.TrimSpace(parts[1])
	if command == "" || pattern == "" {
		return Check{}, fmt.Errorf("@check_cmd_output requires command and pattern separated by ' | '")
	}

	return Check{
		Sprint:  sprint,
		Type:    CheckCmdOutput,
		Command: command,
		Pattern: unquotePattern(pattern),
	}, nil
}

func unquotePattern(s string) string {
	if len(s) >= 2 && strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
		s = s[1 : len(s)-1]
	}

	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case '\\', '"':
				b.WriteByte(s[i+1])
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}

	return b.String()
}
