package preflight

import (
	"regexp"
	"sort"
	"strings"
)

var (
	// Environment variable patterns
	envBashRe    = regexp.MustCompile(`\$\{?([A-Z][A-Z0-9_]{2,})\}?`)
	envGoRe      = regexp.MustCompile(`os\.Getenv\("([A-Z][A-Z0-9_]{2,})"\)`)
	envNodeRe    = regexp.MustCompile(`process\.env\.([A-Z][A-Z0-9_]{2,})`)
	envPythonRe  = regexp.MustCompile(`os\.environ(?:\.get)?\[?"([A-Z][A-Z0-9_]{2,})"`)
	envDotenvRe  = regexp.MustCompile(`(?i)(?:^|\s)([A-Z][A-Z0-9_]{2,})=`)
	envGenericRe = regexp.MustCompile(`(?:env(?:ironment)?\s+var(?:iable)?s?\s+(?:like|such as|including|:)\s*)([A-Z][A-Z0-9_]+(?:\s*,\s*[A-Z][A-Z0-9_]+)*)`)

	// Docker patterns
	dockerRe = regexp.MustCompile(`(?i)docker|testcontainer|docker-compose|docker compose|containerized`)

	// Common tool patterns — tool name mapped to the executable
	toolPatterns = []struct {
		re   *regexp.Regexp
		name string
	}{
		{regexp.MustCompile(`(?i)\bplaywright\b`), "npx"},
		{regexp.MustCompile(`(?i)\bcypress\b`), "npx"},
		{regexp.MustCompile(`(?i)\bredis\b`), "redis-cli"},
		{regexp.MustCompile(`(?i)\bpostgres(?:ql)?\b`), "psql"},
		{regexp.MustCompile(`(?i)\bmysql\b`), "mysql"},
		{regexp.MustCompile(`(?i)\bpython\b`), "python3"},
		{regexp.MustCompile(`(?i)\bjava\b`), "java"},
	}

	// Env vars that are standard system variables, not app-specific prerequisites
	ignoredEnvVars = map[string]bool{
		"HOME": true, "PATH": true, "USER": true, "SHELL": true,
		"TERM": true, "LANG": true, "PWD": true, "TMPDIR": true,
		"EDITOR": true, "GOPATH": true, "GOROOT": true,
		"NODE_ENV": true, "CI": true, "DEBUG": true,
		"NO_COLOR": true, "FORCE_COLOR": true,
	}
)

// inferEnvVars extracts likely environment variable names from text.
func inferEnvVars(text string) []string {
	seen := make(map[string]bool)
	var vars []string

	for _, re := range []*regexp.Regexp{envBashRe, envGoRe, envNodeRe, envPythonRe, envDotenvRe} {
		for _, m := range re.FindAllStringSubmatch(text, -1) {
			name := m[1]
			if !seen[name] && !ignoredEnvVars[name] {
				seen[name] = true
				vars = append(vars, name)
			}
		}
	}

	// Handle comma-separated lists in prose: "env vars like FOO, BAR, BAZ"
	for _, m := range envGenericRe.FindAllStringSubmatch(text, -1) {
		for _, part := range strings.Split(m[1], ",") {
			name := strings.TrimSpace(part)
			if name != "" && !seen[name] && !ignoredEnvVars[name] {
				seen[name] = true
				vars = append(vars, name)
			}
		}
	}

	sort.Strings(vars)
	return vars
}

// inferTools extracts likely required external tools from text.
func inferTools(text string) []string {
	seen := make(map[string]bool)
	var tools []string
	for _, tp := range toolPatterns {
		if tp.re.MatchString(text) && !seen[tp.name] {
			seen[tp.name] = true
			tools = append(tools, tp.name)
		}
	}
	return tools
}
