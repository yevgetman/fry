package wake

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yevgetman/fry/internal/wakelog"
)

func writeWakeLogEntries(t *testing.T, dir string, entries []wakelog.Entry) {
	t.Helper()
	path := filepath.Join(dir, "wake_log.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatalf("open wake_log: %v", err)
	}
	defer func() { _ = f.Close() }()
	for _, e := range entries {
		data, _ := json.Marshal(e)
		if _, err := f.Write(data); err != nil {
			t.Fatalf("write entry: %v", err)
		}
		if _, err := f.WriteString("\n"); err != nil {
			t.Fatalf("write newline: %v", err)
		}
	}
}

func TestDetectNoop_NotEnoughHistory(t *testing.T) {
	dir := t.TempDir()
	writeWakeLogEntries(t, dir, []wakelog.Entry{
		{WakeNumber: 1, PromiseTokenFound: false},
		{WakeNumber: 2, PromiseTokenFound: false},
	})
	if err := os.WriteFile(filepath.Join(dir, "supervisor_log.jsonl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	detected, _, err := DetectNoop(dir, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detected {
		t.Error("expected no noop with only 2 entries when n=3")
	}
}

func TestDetectNoop_AllFailed(t *testing.T) {
	dir := t.TempDir()
	writeWakeLogEntries(t, dir, []wakelog.Entry{
		{WakeNumber: 1, PromiseTokenFound: false},
		{WakeNumber: 2, PromiseTokenFound: false},
		{WakeNumber: 3, PromiseTokenFound: false},
	})
	if err := os.WriteFile(filepath.Join(dir, "supervisor_log.jsonl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	detected, reason, err := DetectNoop(dir, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !detected {
		t.Error("expected noop detected when all 3 entries have no promise token")
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}

	// Verify supervisor_log.jsonl got a noop_warning entry
	data, _ := os.ReadFile(filepath.Join(dir, "supervisor_log.jsonl"))
	if !strings.Contains(string(data), "noop_warning") {
		t.Error("expected noop_warning in supervisor_log.jsonl")
	}
}

func TestDetectNoop_OneSuccess_NotNoop(t *testing.T) {
	dir := t.TempDir()
	writeWakeLogEntries(t, dir, []wakelog.Entry{
		{WakeNumber: 1, PromiseTokenFound: false},
		{WakeNumber: 2, PromiseTokenFound: true},
		{WakeNumber: 3, PromiseTokenFound: false},
	})
	if err := os.WriteFile(filepath.Join(dir, "supervisor_log.jsonl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	detected, _, err := DetectNoop(dir, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detected {
		t.Error("expected no noop when one entry has promise token found")
	}
}

func TestDetectNoop_WarnOnlyOnce(t *testing.T) {
	dir := t.TempDir()
	writeWakeLogEntries(t, dir, []wakelog.Entry{
		{WakeNumber: 1, PromiseTokenFound: false},
		{WakeNumber: 2, PromiseTokenFound: false},
		{WakeNumber: 3, PromiseTokenFound: false},
	})
	if err := os.WriteFile(filepath.Join(dir, "supervisor_log.jsonl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	// Call twice; should produce 2 noop_warning entries (dedupe not required by spec)
	_, _, _ = DetectNoop(dir, 3)
	_, _, _ = DetectNoop(dir, 3)

	data, _ := os.ReadFile(filepath.Join(dir, "supervisor_log.jsonl"))
	count := strings.Count(string(data), "noop_warning")
	if count != 2 {
		t.Errorf("expected 2 noop_warning entries on two calls, got %d", count)
	}
}
