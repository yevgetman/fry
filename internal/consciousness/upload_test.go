package consciousness

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
)

func TestUploadExperience_Success(t *testing.T) {
	t.Parallel()

	var received UploadPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/ingest", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		err := json.NewDecoder(r.Body).Decode(&received)
		assert.NoError(t, err)

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true,"id":"abc","global_build_number":42}`))
	}))
	defer server.Close()

	record := BuildRecord{
		ID:           "abc",
		Engine:       "claude",
		EffortLevel:  "high",
		TotalSprints: 3,
		Outcome:      "success",
		Summary:      "Build went well.",
	}

	result, err := UploadExperience(context.Background(), server.URL, "test-token", record)
	require.NoError(t, err)
	assert.True(t, result.OK)
	assert.Equal(t, 42, result.GlobalBuildNumber)
	assert.Equal(t, "abc", received.ID)
	assert.Equal(t, "claude", received.BuildMetadata.Engine)
	assert.Equal(t, "Build went well.", received.SummaryText)
}

func TestUploadExperience_Unauthorized(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer server.Close()

	_, err := UploadExperience(context.Background(), server.URL, "bad-token", BuildRecord{ID: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestUploadExperience_ServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal"}`))
	}))
	defer server.Close()

	_, err := UploadExperience(context.Background(), server.URL, "token", BuildRecord{ID: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestUploadExperience_Timeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	_, err := UploadExperience(ctx, server.URL, "token", BuildRecord{ID: "x"})
	require.Error(t, err)
}

func TestUploadExperience_InvalidResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	_, err := UploadExperience(context.Background(), server.URL, "token", BuildRecord{ID: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")
}

func TestUploadExperience_NotOK(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":false}`))
	}))
	defer server.Close()

	_, err := UploadExperience(context.Background(), server.URL, "token", BuildRecord{ID: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ok=false")
}

func TestUploadPayload_Structure(t *testing.T) {
	t.Parallel()

	payload := UploadPayload{
		ID:             "test-id",
		SourceInstance: "abcdef0123456789",
		BuildMetadata: BuildMetadata{
			Engine:       "claude",
			EffortLevel:  "high",
			TotalSprints: 5,
			Outcome:      "success",
		},
		SummaryText: "A summary.",
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))

	assert.Equal(t, "test-id", raw["id"])
	assert.Equal(t, "abcdef0123456789", raw["source_instance"])
	assert.Equal(t, "A summary.", raw["summary_text"])

	meta, ok := raw["build_metadata"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "claude", meta["engine"])
	assert.Equal(t, "high", meta["effort_level"])
	assert.Equal(t, float64(5), meta["total_sprints"])
	assert.Equal(t, "success", meta["outcome"])
}

func TestCachePendingUpload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	record := BuildRecord{ID: "cache-test", Engine: "claude", Summary: "test summary"}

	err := cachePendingUploadToDir(dir, record)
	require.NoError(t, err)

	path := filepath.Join(dir, "pending-cache-test.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var loaded BuildRecord
	require.NoError(t, json.Unmarshal(data, &loaded))
	assert.Equal(t, "cache-test", loaded.ID)
	assert.Equal(t, "test summary", loaded.Summary)
}

func TestCachePendingUpload_Idempotent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	record := BuildRecord{ID: "idem-test", Summary: "v1"}

	require.NoError(t, cachePendingUploadToDir(dir, record))
	record.Summary = "v2"
	require.NoError(t, cachePendingUploadToDir(dir, record))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)

	data, err := os.ReadFile(filepath.Join(dir, "pending-idem-test.json"))
	require.NoError(t, err)

	var loaded BuildRecord
	require.NoError(t, json.Unmarshal(data, &loaded))
	assert.Equal(t, "v2", loaded.Summary)
}

func TestRetryPendingUploads_NoDir(t *testing.T) {
	t.Parallel()

	count := retryPendingUploadsFromDir(context.Background(), "http://unused", "token",
		filepath.Join(t.TempDir(), "nonexistent"))
	assert.Equal(t, 0, count)
}

func TestRetryPendingUploads_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true,"id":"retry-1","global_build_number":1}`))
	}))
	defer server.Close()

	dir := t.TempDir()
	record := BuildRecord{ID: "retry-1", Engine: "claude", Summary: "retry me"}
	require.NoError(t, cachePendingUploadToDir(dir, record))

	count := retryPendingUploadsFromDir(context.Background(), server.URL, "token", dir)
	assert.Equal(t, 1, count)

	// File should be removed
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestRetryPendingUploads_PartialFailure(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"ok":true,"id":"a","global_build_number":1}`))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"fail"}`))
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	require.NoError(t, cachePendingUploadToDir(dir, BuildRecord{ID: "a", Summary: "ok"}))
	require.NoError(t, cachePendingUploadToDir(dir, BuildRecord{ID: "b", Summary: "fail"}))

	count := retryPendingUploadsFromDir(context.Background(), server.URL, "token", dir)
	assert.Equal(t, 1, count)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "pending-b.json", entries[0].Name())
}

func TestRetryPendingUploads_PrunesOld(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	record := BuildRecord{ID: "old-one", Summary: "ancient"}
	require.NoError(t, cachePendingUploadToDir(dir, record))

	// Backdate the file to 8 days ago
	path := filepath.Join(dir, "pending-old-one.json")
	oldTime := time.Now().Add(-8 * 24 * time.Hour)
	require.NoError(t, os.Chtimes(path, oldTime, oldTime))

	// Server should never be called — the file is pruned without upload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not have been called")
	}))
	defer server.Close()

	count := retryPendingUploadsFromDir(context.Background(), server.URL, "token", dir)
	assert.Equal(t, 0, count)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestUploadInBackground_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true,"id":"bg","global_build_number":5}`))
	}))
	defer server.Close()

	record := BuildRecord{ID: "bg", Engine: "claude", Summary: "background test"}
	done := UploadInBackground(server.URL, "token", record, 5*time.Second)

	select {
	case <-done:
		// success
	case <-time.After(10 * time.Second):
		t.Fatal("UploadInBackground did not complete in time")
	}
}

// TestUploadInBackground_DoubleFail verifies that when both UploadExperience
// and CachePendingUpload fail, a warning is emitted to stderr.
func TestUploadInBackground_DoubleFail(t *testing.T) {
	// t.Parallel() omitted: test reassigns os.Stderr which races with other goroutines

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"down"}`))
	}))
	defer server.Close()

	// Redirect HOME to a non-writable directory so CachePendingUpload also fails
	readOnlyDir := filepath.Join(t.TempDir(), "readonly")
	require.NoError(t, os.MkdirAll(readOnlyDir, 0o555))
	t.Cleanup(func() {
		_ = os.Chmod(readOnlyDir, 0o755)
	})
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", readOnlyDir)
	t.Cleanup(func() {
		os.Setenv("HOME", origHome)
	})

	// Capture stderr
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()
	defer w.Close()
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = oldStderr
	})

	record := BuildRecord{ID: "double-fail", Engine: "claude", Summary: "will double fail"}
	done := UploadInBackground(server.URL, "token", record, 5*time.Second)

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("UploadInBackground did not complete in time")
	}

	// Close write-end so ReadAll gets EOF (defer w.Close is a safety net for fatal paths)
	w.Close()
	captured, readErr := io.ReadAll(r)
	require.NoError(t, readErr)

	assert.Contains(t, string(captured), "failed to cache experience upload")
}

func TestUploadInBackground_FailureCaches(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"down"}`))
	}))
	defer server.Close()

	record := BuildRecord{ID: "fail-bg", Engine: "claude", Summary: "will fail"}

	// Override home for cache — we can't easily do this without modifying
	// CachePendingUpload, so we just verify the goroutine completes.
	done := UploadInBackground(server.URL, "token", record, 5*time.Second)

	select {
	case <-done:
		// Goroutine completed — that's what we're testing
	case <-time.After(10 * time.Second):
		t.Fatal("UploadInBackground did not complete in time")
	}
}

func TestUploadEvent_Success(t *testing.T) {
	t.Parallel()

	var received UploadEnvelope
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&received))
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true,"id":"evt","global_build_number":12}`))
	}))
	defer server.Close()

	result, err := UploadEvent(context.Background(), server.URL, "token", UploadEnvelope{
		SchemaVersion: 2,
		ID:            "session-1-0001-checkpoint_summary",
		EventType:     UploadEventCheckpointSummary,
		SessionID:     "session-1",
		Sequence:      1,
		BuildMetadata: BuildMetadata{Engine: "claude", EffortLevel: "high", TotalSprints: 3, Outcome: "failure"},
		SummaryText:   "Checkpoint summary",
	})
	require.NoError(t, err)
	assert.True(t, result.OK)
	assert.Equal(t, UploadEventCheckpointSummary, received.EventType)
	assert.Equal(t, "Checkpoint summary", received.SummaryText)
}

func TestNewLifecycleUpload_FallbackSummary(t *testing.T) {
	t.Parallel()

	// Record with empty summary — session_started events always have empty summary
	record := BuildRecord{
		SessionID:    "test-session",
		Engine:       "claude",
		EffortLevel:  "max",
		TotalSprints: 5,
	}

	env := newLifecycleUpload(record, 0, UploadEventSessionStarted)
	assert.NotEmpty(t, env.SummaryText, "session_started must have non-empty summary_text")
	assert.Contains(t, env.SummaryText, "5 sprints")
	assert.Contains(t, env.SummaryText, "max")
	assert.Contains(t, env.SummaryText, "claude")

	env = newLifecycleUpload(record, 1, UploadEventSessionInterrupted)
	assert.Contains(t, env.SummaryText, "interrupted")

	// Record with actual summary — should use it directly
	record.Summary = "Custom summary text"
	env = newLifecycleUpload(record, 2, UploadEventSessionCompleted)
	assert.Equal(t, "Custom summary text", env.SummaryText)
}

func TestRetryProjectUploads_SuccessAndCounterUpdate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.ConsciousnessUploadQueueDir), 0o755))
	require.NoError(t, writeJSONAtomic(filepath.Join(dir, config.ConsciousnessSessionFile), SessionState{
		SessionID: "session-1",
		Status:    SessionStatusRunning,
	}))
	event := UploadEnvelope{
		SchemaVersion: 2,
		ID:            "session-1-0001-checkpoint_summary",
		EventType:     UploadEventCheckpointSummary,
		SessionID:     "session-1",
		Sequence:      1,
		BuildMetadata: BuildMetadata{Engine: "claude", EffortLevel: "high", TotalSprints: 3, Outcome: "failure"},
		SummaryText:   "Checkpoint summary",
	}
	require.NoError(t, writeJSONAtomic(filepath.Join(dir, config.ConsciousnessUploadQueueDir, eventFilename(event)), event))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true,"id":"evt","global_build_number":13}`))
	}))
	defer server.Close()

	count := RetryProjectUploads(context.Background(), server.URL, "token", dir)
	assert.Equal(t, 1, count)
	assert.Equal(t, 0, countJSONFiles(filepath.Join(dir, config.ConsciousnessUploadQueueDir)))

	state, err := loadSessionState(filepath.Join(dir, config.ConsciousnessSessionFile))
	require.NoError(t, err)
	assert.Equal(t, 1, state.UploadAttempts)
	assert.Equal(t, 1, state.UploadSuccesses)
	assert.Equal(t, 0, state.PendingUploads)
}
