package consciousness

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTriggerReflection_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/reflect", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		resp := map[string]interface{}{
			"ok": true,
			"stats": ReflectionResult{
				MemoriesConsidered: 150,
				MemoriesIntegrated: 150,
				MemoriesPruned:     3,
				IdentityVersion:    2,
				CommitSHA:          "abc123",
				Changes:            []string{"Strengthened: correctness value", "Pruned: 3 memories"},
			},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	result, err := TriggerReflection(context.Background(), server.URL, "test-token")
	require.NoError(t, err)
	assert.Equal(t, 150, result.MemoriesConsidered)
	assert.Equal(t, 150, result.MemoriesIntegrated)
	assert.Equal(t, 3, result.MemoriesPruned)
	assert.Equal(t, 2, result.IdentityVersion)
	assert.Equal(t, "abc123", result.CommitSHA)
	assert.Len(t, result.Changes, 2)
}

func TestTriggerReflection_InsufficientMemories(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"ok": true,
			"stats": ReflectionResult{
				MemoriesConsidered: 10,
				MemoriesIntegrated: 0,
				MemoriesPruned:     0,
				IdentityVersion:    0,
				Error:              "insufficient memories: 10 < 50 minimum",
			},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	result, err := TriggerReflection(context.Background(), server.URL, "test-token")
	require.NoError(t, err)
	assert.Equal(t, 10, result.MemoriesConsidered)
	assert.Equal(t, 0, result.MemoriesIntegrated)
	assert.Equal(t, "insufficient memories: 10 < 50 minimum", result.Error)
}

func TestTriggerReflection_Unauthorized(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer server.Close()

	_, err := TriggerReflection(context.Background(), server.URL, "wrong-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 401")
}

func TestTriggerReflection_ServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	_, err := TriggerReflection(context.Background(), server.URL, "test-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
}

func TestTriggerReflection_Timeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Never responds
		select {}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	_, err := TriggerReflection(ctx, server.URL, "test-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trigger reflection:")
}

func TestTriggerReflection_NotOK(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":false}`))
	}))
	defer server.Close()

	_, err := TriggerReflection(context.Background(), server.URL, "test-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ok=false")
}
