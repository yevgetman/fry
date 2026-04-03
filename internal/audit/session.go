package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
)

var (
	claudeSessionIDRe = regexp.MustCompile(`"session_id":"([0-9a-fA-F-]{36})"`)
	codexThreadIDRe   = regexp.MustCompile(`"thread_id":"([0-9a-fA-F-]{36})"`)
)

type sessionContinuity struct {
	engineName  string
	role        string
	path        string
	id          string
	callCount   int
	promptBytes int
	tokenTotal  int
	refreshes   int
}

func newAuditSessionContinuity(projectDir string, sprintNum int, engineName string) *sessionContinuity {
	return newSessionContinuity(engineName, "audit", auditSessionPath(projectDir, sprintNum))
}

func newFixSessionContinuity(projectDir string, sprintNum, cycle int, engineName string) *sessionContinuity {
	return newSessionContinuity(engineName, "fix", fixSessionPath(projectDir, sprintNum, cycle))
}

func newSessionContinuity(engineName, role, path string) *sessionContinuity {
	if !supportsSessionContinuity(engineName) {
		return nil
	}
	return &sessionContinuity{
		engineName: strings.ToLower(strings.TrimSpace(engineName)),
		role:       strings.ToLower(strings.TrimSpace(role)),
		path:       path,
		id:         loadSessionID(path),
	}
}

func supportsSessionContinuity(engineName string) bool {
	switch strings.ToLower(strings.TrimSpace(engineName)) {
	case "claude", "codex":
		return true
	default:
		return false
	}
}

func (s *sessionContinuity) Configure(runOpts *engine.RunOpts) {
	if s == nil || runOpts == nil {
		return
	}
	runOpts.StructuredOutput = true
	if s.id != "" {
		runOpts.SessionID = s.id
	}
}

func (s *sessionContinuity) Capture(output string) {
	if s == nil {
		return
	}
	id := extractSessionID(s.engineName, output)
	if id == "" || id == s.id {
		return
	}
	s.id = id
	_ = persistSessionID(s.path, id)
}

func (s *sessionContinuity) Clear() {
	if s == nil {
		return
	}
	s.id = ""
	s.callCount = 0
	s.promptBytes = 0
	s.tokenTotal = 0
	_ = os.Remove(s.path)
}

type sessionBudget struct {
	MaxCalls       int
	MaxPromptBytes int
	MaxTokens      int
	MaxCarry       int
}

func sessionBudgetForRole(role string) sessionBudget {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "fix":
		return sessionBudget{
			MaxCalls:       config.FixSessionMaxCalls,
			MaxPromptBytes: config.FixSessionMaxPromptBytes,
			MaxTokens:      config.FixSessionMaxTokens,
			MaxCarry:       config.FixSessionMaxCarry,
		}
	default:
		return sessionBudget{
			MaxCalls:       config.AuditSessionMaxCalls,
			MaxPromptBytes: config.AuditSessionMaxPromptBytes,
			MaxTokens:      config.AuditSessionMaxTokens,
			MaxCarry:       config.AuditSessionMaxCarry,
		}
	}
}

func (s *sessionContinuity) MaybeRefresh(carryForwardCount int) string {
	if s == nil || s.id == "" {
		return ""
	}

	budget := sessionBudgetForRole(s.role)
	var reasons []string
	if budget.MaxCalls > 0 && s.callCount >= budget.MaxCalls {
		reasons = append(reasons, fmt.Sprintf("call budget reached (%d)", s.callCount))
	}
	if budget.MaxPromptBytes > 0 && s.promptBytes >= budget.MaxPromptBytes {
		reasons = append(reasons, fmt.Sprintf("prompt budget reached (%d bytes)", s.promptBytes))
	}
	if budget.MaxTokens > 0 && s.tokenTotal >= budget.MaxTokens {
		reasons = append(reasons, fmt.Sprintf("token budget reached (%d tokens)", s.tokenTotal))
	}
	if budget.MaxCarry > 0 && carryForwardCount > budget.MaxCarry {
		reasons = append(reasons, fmt.Sprintf("carry-forward set too large (%d findings)", carryForwardCount))
	}
	if len(reasons) == 0 {
		return ""
	}

	s.refreshes++
	s.Clear()
	return strings.Join(reasons, "; ")
}

func (s *sessionContinuity) RecordCall(promptBytes, tokenTotal int) {
	if s == nil {
		return
	}
	s.callCount++
	s.promptBytes += promptBytes
	s.tokenTotal += tokenTotal
}

func cleanupAuditSessions(projectDir string, sprintNum int) error {
	dir := filepath.Join(projectDir, config.AuditSessionsDir)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	prefix := fmt.Sprintf("sprint%d_", sprintNum)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) || !strings.HasSuffix(entry.Name(), ".session") {
			continue
		}
		if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func auditSessionPath(projectDir string, sprintNum int) string {
	return filepath.Join(projectDir, config.AuditSessionsDir, fmt.Sprintf("sprint%d_audit.session", sprintNum))
}

func fixSessionPath(projectDir string, sprintNum, cycle int) string {
	return filepath.Join(projectDir, config.AuditSessionsDir, fmt.Sprintf("sprint%d_cycle%d_audit-fix.session", sprintNum, cycle))
}

func loadSessionID(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func persistSessionID(path, id string) error {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(id)+"\n"), 0o644)
}

func extractSessionID(engineName, output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}

	var match []string
	switch strings.ToLower(strings.TrimSpace(engineName)) {
	case "claude":
		match = claudeSessionIDRe.FindStringSubmatch(output)
	case "codex":
		match = codexThreadIDRe.FindStringSubmatch(output)
	}
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}
