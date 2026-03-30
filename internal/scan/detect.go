package scan

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// projectMarkers are files whose presence indicates an existing software project.
// Each entry is a filename (or glob-like pattern handled as exact match) that
// serves as a high-confidence language/framework indicator.
var projectMarkers = map[string]string{
	// Go
	"go.mod": "Go",
	// JavaScript / TypeScript
	"package.json": "JavaScript/TypeScript",
	// Python
	"pyproject.toml":  "Python",
	"setup.py":        "Python",
	"setup.cfg":       "Python",
	"requirements.txt": "Python",
	"Pipfile":          "Python",
	// Ruby
	"Gemfile": "Ruby",
	// Rust
	"Cargo.toml": "Rust",
	// Java / Kotlin
	"pom.xml":      "Java",
	"build.gradle":  "Java/Kotlin",
	"build.gradle.kts": "Kotlin",
	// C# / .NET
	"*.sln": ".NET", // handled specially in marker check
	// C / C++
	"CMakeLists.txt": "C/C++",
	// Elixir
	"mix.exs": "Elixir",
	// PHP
	"composer.json": "PHP",
	// Swift
	"Package.swift": "Swift",
	// Dart / Flutter
	"pubspec.yaml": "Dart",
}

// minNonDotFiles is the minimum number of non-hidden files that indicates a
// populated project directory (used when no markers or git history are found).
const minNonDotFiles = 10

// IsExistingProject returns true if dir appears to contain an existing software
// project. Detection is purely heuristic — no LLM calls.
//
// A directory is considered an existing project when ANY of these hold:
//   - Git history has more than 1 commit (beyond fry init's initial commit)
//   - A known project marker file exists (go.mod, package.json, Cargo.toml, …)
//   - The directory contains more than minNonDotFiles non-hidden files
func IsExistingProject(ctx context.Context, dir string) bool {
	if hasSignificantGitHistory(ctx, dir) {
		return true
	}
	if hasProjectMarker(dir) {
		return true
	}
	if countNonDotFiles(dir) > minNonDotFiles {
		return true
	}
	return false
}

// hasSignificantGitHistory returns true if the directory is a git repo with
// more than 1 commit.
func hasSignificantGitHistory(ctx context.Context, dir string) bool {
	// Check if .git exists first to avoid running git in non-repos.
	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		return false
	}
	cmd := exec.CommandContext(ctx, "git", "rev-list", "--count", "HEAD")
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return false
	}
	count, err := strconv.Atoi(strings.TrimSpace(stdout.String()))
	if err != nil {
		return false
	}
	return count > 1
}

// hasProjectMarker checks for the presence of known project marker files.
func hasProjectMarker(dir string) bool {
	for marker := range projectMarkers {
		if marker == "*.sln" {
			// Special case: glob for .sln files.
			matches, _ := filepath.Glob(filepath.Join(dir, "*.sln"))
			if len(matches) > 0 {
				return true
			}
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}
	return false
}

// countNonDotFiles counts non-hidden, non-directory entries in the top-level
// directory. It reads at most minNonDotFiles+1 entries to short-circuit.
func countNonDotFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if e.IsDir() {
			continue
		}
		count++
		if count > minNonDotFiles {
			break
		}
	}
	return count
}
