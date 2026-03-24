package color

import "strings"

// ColorizeLogLine applies color to a timestamped log line based on known patterns.
// It is designed to be passed to log.SetColorize so that only stdout gets colored.
func ColorizeLogLine(line string) string {
	if !Enabled() {
		return line
	}

	// Phase banners: ▶ AGENT, ▶ TRIAGE, ▶ AUDIT, ▶ CONTINUE, ▶ OBSERVER, ▶ BUILD AUDIT, ▶ BUILD SUMMARY, ▶ SUMMARY
	if strings.Contains(line, "▶ ") {
		return colorizeToken(line, "▶ ", Cyan)
	}

	// Sprint start banners
	if strings.Contains(line, "STARTING SPRINT") || strings.Contains(line, "RESUMING SPRINT") {
		return colorizeToken(line, "SPRINT", Bold)
	}
	if strings.Contains(line, "=========") {
		return Colorize(line, Bold)
	}

	// Failure lines — check before PASS since "FAIL" is more urgent
	if containsWord(line, "FAIL") {
		return colorizeToken(line, "FAIL", Red)
	}

	// Success lines
	if containsWord(line, "PASS") {
		return colorizeToken(line, "PASS", Green)
	}

	// Warnings
	if strings.Contains(line, "WARNING") || strings.Contains(line, "⚠") {
		return colorizeToken(line, "WARNING", Yellow)
	}

	// Git checkpoint lines
	if strings.Contains(line, "  GIT:") {
		return Colorize(line, Dim)
	}

	return line
}

// colorizeToken finds the first occurrence of token in line and wraps it with color.
func colorizeToken(line, token string, code Code) string {
	idx := strings.Index(line, token)
	if idx < 0 {
		return line
	}
	return line[:idx] + Colorize(token, code) + line[idx+len(token):]
}

// containsWord checks whether line contains s as a word boundary —
// preceded and followed by a non-letter or at string boundaries.
func containsWord(line, s string) bool {
	idx := 0
	for {
		pos := strings.Index(line[idx:], s)
		if pos < 0 {
			return false
		}
		abs := idx + pos
		before := abs == 0 || !isLetter(line[abs-1])
		after := abs+len(s) >= len(line) || !isLetter(line[abs+len(s)])
		if before && after {
			return true
		}
		idx = abs + len(s)
	}
}

func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
