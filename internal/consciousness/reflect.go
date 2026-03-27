package consciousness

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ReflectionResult is the parsed response from the /reflect endpoint.
type ReflectionResult struct {
	MemoriesConsidered int      `json:"memories_considered"`
	MemoriesIntegrated int      `json:"memories_integrated"`
	MemoriesPruned     int      `json:"memories_pruned"`
	IdentityVersion    int      `json:"identity_version"`
	CommitSHA          string   `json:"commit_sha"`
	Changes            []string `json:"changes"`
	Error              string   `json:"error"`
}

// TriggerReflection sends a POST to /reflect on the consciousness API.
// The server computes memory weights, synthesizes an updated identity.json,
// prunes decayed memories, and commits the result to GitHub.
func TriggerReflection(ctx context.Context, apiURL, apiToken string) (*ReflectionResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL+"/reflect", nil)
	if err != nil {
		return nil, fmt.Errorf("trigger reflection: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("trigger reflection: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("trigger reflection: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return nil, fmt.Errorf("trigger reflection: HTTP %d: %s", resp.StatusCode, snippet)
	}

	var wrapper struct {
		OK    bool             `json:"ok"`
		Stats ReflectionResult `json:"stats"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, fmt.Errorf("trigger reflection: decode response: %w", err)
	}

	if !wrapper.OK {
		return nil, fmt.Errorf("trigger reflection: server returned ok=false")
	}

	return &wrapper.Stats, nil
}
