package consciousness

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchPipelineStats_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/stats", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"memories": {"total": 23, "by_category": {"process": 8, "testing": 5}, "reflection_threshold": 50},
			"builds": {"global_build_number": 7, "total_summaries": 9, "pending_summaries": 2},
			"last_transmutation": "2026-03-27T03:00:12Z",
			"last_reflection": {"completed_at": null, "memories_considered": 0, "memories_pruned": 0, "identity_version": 0, "changes_summary": ""}
		}`))
	}))
	defer server.Close()

	stats, err := FetchPipelineStats(context.Background(), server.URL)
	require.NoError(t, err)
	assert.Equal(t, 23, stats.Memories.Total)
	assert.Equal(t, 8, stats.Memories.ByCategory["process"])
	assert.Equal(t, 5, stats.Memories.ByCategory["testing"])
	assert.Equal(t, 7, stats.Builds.GlobalBuildNumber)
	assert.Equal(t, 2, stats.Builds.PendingSummaries)
	assert.NotNil(t, stats.LastTransmutation)
	assert.Nil(t, stats.LastReflection.CompletedAt)
}

func TestFetchPipelineStats_ServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal"}`))
	}))
	defer server.Close()

	_, err := FetchPipelineStats(context.Background(), server.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
}

func TestFetchPipelineStats_Timeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := FetchPipelineStats(ctx, server.URL)
	require.Error(t, err)
}

func TestFormatPipelineStats_NoReflection(t *testing.T) {
	t.Parallel()

	transmutation := "2026-03-27T03:00:12Z"
	stats := &PipelineStats{
		Memories: MemoryStats{
			Total:               23,
			ByCategory:          map[string]int{"process": 8, "testing": 5, "healing": 4, "tooling": 3, "planning": 3},
			ReflectionThreshold: 50,
		},
		Builds: BuildStats{
			GlobalBuildNumber: 7,
			TotalSummaries:    9,
			PendingSummaries:  2,
		},
		LastTransmutation: &transmutation,
		LastReflection: ReflectionInfo{
			CompletedAt: nil,
		},
	}

	output := FormatPipelineStats(stats)
	assert.Contains(t, output, "Consciousness Pipeline")
	assert.Contains(t, output, "23 / 50 for reflection")
	assert.Contains(t, output, "46%")
	assert.Contains(t, output, "Build number:       7")
	assert.Contains(t, output, "Pending summaries:  2")
	assert.Contains(t, output, "Last transmutation: 2026-03-27 03:00:12 UTC")
	assert.Contains(t, output, "Last reflection:    never")
	assert.Contains(t, output, "process: 8")
	assert.Contains(t, output, "testing: 5")
}

func TestFormatPipelineStats_WithReflection(t *testing.T) {
	t.Parallel()

	reflectedAt := "2026-03-30T04:00:05Z"
	stats := &PipelineStats{
		Memories: MemoryStats{
			Total:               150,
			ByCategory:          map[string]int{"process": 40, "testing": 30},
			ReflectionThreshold: 50,
		},
		Builds: BuildStats{
			GlobalBuildNumber: 50,
			TotalSummaries:    60,
			PendingSummaries:  0,
		},
		LastTransmutation: nil,
		LastReflection: ReflectionInfo{
			CompletedAt:        &reflectedAt,
			MemoriesConsidered: 150,
			MemoriesPruned:     3,
			IdentityVersion:    2,
		},
	}

	output := FormatPipelineStats(stats)
	assert.Contains(t, output, "150 (reflection active)")
	assert.Contains(t, output, "Last reflection:    2026-03-30 04:00:05 UTC (v2, 150 memories, 3 pruned)")
	assert.Contains(t, output, "Last transmutation: never")
	assert.NotContains(t, output, "Pending summaries")
}

func TestFormatPipelineStats_EmptyState(t *testing.T) {
	t.Parallel()

	stats := &PipelineStats{
		Memories: MemoryStats{
			Total:               0,
			ByCategory:          map[string]int{},
			ReflectionThreshold: 50,
		},
		Builds: BuildStats{},
		LastReflection: ReflectionInfo{
			CompletedAt: nil,
		},
	}

	output := FormatPipelineStats(stats)
	assert.Contains(t, output, "0 / 50 for reflection")
	assert.Contains(t, output, "0%")
	assert.Contains(t, output, "Last transmutation: never")
	assert.Contains(t, output, "Last reflection:    never")
	assert.NotContains(t, output, "Categories:")
}

func TestProgressBar(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "\u2591\u2591\u2591\u2591\u2591", progressBar(0, 5))
	assert.Equal(t, "\u2588\u2588\u2591\u2591\u2591", progressBar(50, 5))
	assert.Equal(t, "\u2588\u2588\u2588\u2588\u2588", progressBar(100, 5))
	assert.Equal(t, "\u2588\u2588\u2588\u2588\u2588", progressBar(150, 5))
}
