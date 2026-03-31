package consciousness

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yevgetman/fry/internal/config"
)

// userAgent identifies Fry to Cloudflare so requests are not blocked by
// bot protection (error 1010). Go's default "Go-http-client/1.1" gets flagged.
const userAgent = "fry/" + config.Version

// pendingMaxAge is the maximum age for pending upload files before they are
// pruned. Files older than this are removed without upload attempt.
const pendingMaxAge = 7 * 24 * time.Hour

// UploadPayload is the JSON body sent to the /ingest endpoint.
type UploadPayload struct {
	ID             string        `json:"id"`
	SourceInstance string        `json:"source_instance"`
	BuildMetadata  BuildMetadata `json:"build_metadata"`
	SummaryText    string        `json:"summary_text"`
}

// BuildMetadata contains build-level metadata for the upload payload.
type BuildMetadata struct {
	Engine       string `json:"engine"`
	EffortLevel  string `json:"effort_level"`
	TotalSprints int    `json:"total_sprints"`
	Outcome      string `json:"outcome"`
}

// UploadResult is the response from the /ingest endpoint.
type UploadResult struct {
	OK                bool   `json:"ok"`
	ID                string `json:"id"`
	GlobalBuildNumber int    `json:"global_build_number"`
	Duplicate         bool   `json:"duplicate,omitempty"`
}

// UploadExperience sends a build record to the consciousness API.
func UploadExperience(ctx context.Context, apiURL, apiToken string, record BuildRecord) (*UploadResult, error) {
	payload := UploadPayload{
		ID:             record.ID,
		SourceInstance: InstanceID(),
		BuildMetadata: BuildMetadata{
			Engine:       record.Engine,
			EffortLevel:  record.EffortLevel,
			TotalSprints: record.TotalSprints,
			Outcome:      record.Outcome,
		},
		SummaryText: record.Summary,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("upload experience: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL+"/ingest", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("upload experience: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("User-Agent", userAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload experience: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("upload experience: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(respBody)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return nil, fmt.Errorf("upload experience: HTTP %d: %s", resp.StatusCode, snippet)
	}

	var result UploadResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("upload experience: decode response: %w", err)
	}

	if !result.OK {
		return nil, fmt.Errorf("upload experience: server returned ok=false")
	}

	return &result, nil
}

// CachePendingUpload writes a failed upload's BuildRecord to the pending
// directory for retry on the next build.
func CachePendingUpload(record BuildRecord) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cache pending upload: %w", err)
	}
	dir := filepath.Join(home, config.PendingUploadsDir)
	return cachePendingUploadToDir(dir, record)
}

func cachePendingUploadToDir(dir string, record BuildRecord) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cache pending upload: create dir: %w", err)
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("cache pending upload: marshal: %w", err)
	}

	filename := fmt.Sprintf("pending-%s.json", record.ID)
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("cache pending upload: write: %w", err)
	}

	return nil
}

// RetryPendingUploads reads all files from the pending directory, attempts
// to upload each one, and removes files that succeed. Pending files older
// than 7 days are pruned without upload. Returns the count of successful uploads.
func RetryPendingUploads(ctx context.Context, apiURL, apiToken string) int {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0
	}
	dir := filepath.Join(home, config.PendingUploadsDir)
	return retryPendingUploadsFromDir(ctx, apiURL, apiToken, dir)
}

func retryPendingUploadsFromDir(ctx context.Context, apiURL, apiToken, dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}

	succeeded := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(dir, entry.Name())

		// Prune old files
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if time.Since(info.ModTime()) > pendingMaxAge {
			_ = os.Remove(path)
			continue
		}

		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var record BuildRecord
		if err := json.Unmarshal(data, &record); err != nil {
			// Corrupt file — remove it, it will never succeed
			_ = os.Remove(path)
			continue
		}

		if _, err := UploadExperience(ctx, apiURL, apiToken, record); err != nil {
			continue // leave for next retry
		}

		_ = os.Remove(path)
		succeeded++
	}

	return succeeded
}

// UploadInBackground starts a goroutine that retries pending uploads and
// uploads the current build record. If the upload fails, the record is
// cached locally for retry. The returned channel is closed when the
// goroutine completes.
func UploadInBackground(apiURL, apiToken string, record BuildRecord, timeout time.Duration) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		// Retry any pending uploads from previous builds
		RetryPendingUploads(ctx, apiURL, apiToken)

		// Upload current record
		if _, err := UploadExperience(ctx, apiURL, apiToken, record); err != nil {
			if cacheErr := CachePendingUpload(record); cacheErr != nil {
				fmt.Fprintf(os.Stderr, "fry: warning: failed to cache experience upload: %v\n", cacheErr)
			}
		}
	}()

	return done
}
