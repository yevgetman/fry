// Package confirm implements file-based interactive prompts for agent LLMs.
// Instead of reading from stdin, Fry writes a prompt to .fry/confirm-prompt.json
// and waits for the agent to write a response to .fry/confirm-response.json.
package confirm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yevgetman/fry/internal/config"
)

// PromptType identifies the kind of interactive prompt.
type PromptType string

const (
	PromptTriageConfirm   PromptType = "triage_confirm"
	PromptProjectOverview PromptType = "project_overview"
	PromptExecutiveContext PromptType = "executive_context"
)

// Prompt is the JSON structure written to .fry/confirm-prompt.json.
type Prompt struct {
	Type    PromptType     `json:"type"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
	Options []string       `json:"options"`
}

// Response is the JSON structure read from .fry/confirm-response.json.
type Response struct {
	Action      string         `json:"action"`
	Adjustments map[string]any `json:"adjustments,omitempty"`
}

// WritePrompt atomically writes a prompt to .fry/confirm-prompt.json.
func WritePrompt(projectDir string, p *Prompt) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal confirm prompt: %w", err)
	}
	data = append(data, '\n')
	path := filepath.Join(projectDir, config.ConfirmPromptFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create confirm dir: %w", err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write confirm prompt tmp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename confirm prompt: %w", err)
	}
	return nil
}

// WaitForResponse polls for .fry/confirm-response.json until it appears or
// the context/timeout expires. Returns the parsed response and removes both
// the prompt and response files.
func WaitForResponse(ctx context.Context, projectDir string) (*Response, error) {
	timeout := time.Duration(config.ConfirmTimeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	responsePath := filepath.Join(projectDir, config.ConfirmResponseFile)
	ticker := time.NewTicker(time.Duration(config.ConfirmPollInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			Cleanup(projectDir)
			return nil, fmt.Errorf("timed out waiting for confirm response (%.0fs)", timeout.Seconds())
		case <-ticker.C:
			data, err := os.ReadFile(responsePath)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("read confirm response: %w", err)
			}

			var resp Response
			if err := json.Unmarshal(data, &resp); err != nil {
				return nil, fmt.Errorf("parse confirm response: %w", err)
			}

			Cleanup(projectDir)
			return &resp, nil
		}
	}
}

// Cleanup removes both prompt and response files (best-effort).
func Cleanup(projectDir string) {
	os.Remove(filepath.Join(projectDir, config.ConfirmResponseFile))
	os.Remove(filepath.Join(projectDir, config.ConfirmPromptFile))
}
