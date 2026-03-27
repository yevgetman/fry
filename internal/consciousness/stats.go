package consciousness

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// PipelineStats is the response from the /stats endpoint.
type PipelineStats struct {
	Memories          MemoryStats    `json:"memories"`
	Builds            BuildStats     `json:"builds"`
	LastTransmutation *string        `json:"last_transmutation"`
	LastReflection    ReflectionInfo `json:"last_reflection"`
}

// MemoryStats contains memory store statistics.
type MemoryStats struct {
	Total               int            `json:"total"`
	ByCategory          map[string]int `json:"by_category"`
	ReflectionThreshold int            `json:"reflection_threshold"`
}

// BuildStats contains build pipeline statistics.
type BuildStats struct {
	GlobalBuildNumber int `json:"global_build_number"`
	TotalSummaries    int `json:"total_summaries"`
	PendingSummaries  int `json:"pending_summaries"`
}

// ReflectionInfo contains the last reflection run details.
type ReflectionInfo struct {
	CompletedAt        *string `json:"completed_at"`
	MemoriesConsidered int     `json:"memories_considered"`
	MemoriesPruned     int     `json:"memories_pruned"`
	IdentityVersion    int     `json:"identity_version"`
	ChangesSummary     string  `json:"changes_summary"`
}

// FetchPipelineStats retrieves consciousness pipeline statistics from the API.
func FetchPipelineStats(ctx context.Context, apiURL string) (*PipelineStats, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL+"/stats", nil)
	if err != nil {
		return nil, fmt.Errorf("fetch pipeline stats: create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch pipeline stats: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("fetch pipeline stats: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return nil, fmt.Errorf("fetch pipeline stats: HTTP %d: %s", resp.StatusCode, snippet)
	}

	var stats PipelineStats
	if err := json.Unmarshal(body, &stats); err != nil {
		return nil, fmt.Errorf("fetch pipeline stats: decode: %w", err)
	}

	return &stats, nil
}

// FormatPipelineStats renders pipeline statistics as human-readable text.
func FormatPipelineStats(stats *PipelineStats) string {
	var b strings.Builder

	b.WriteString("Consciousness Pipeline\n")

	// Memory progress toward reflection threshold
	threshold := stats.Memories.ReflectionThreshold
	if threshold == 0 {
		threshold = 50
	}
	pct := 0
	if threshold > 0 {
		pct = (stats.Memories.Total * 100) / threshold
		if pct > 100 {
			pct = 100
		}
	}
	bar := progressBar(pct, 20)

	if stats.Memories.Total >= threshold {
		fmt.Fprintf(&b, "  Memories:           %d (reflection active)\n", stats.Memories.Total)
	} else {
		fmt.Fprintf(&b, "  Memories:           %d / %d for reflection  %s %d%%\n",
			stats.Memories.Total, threshold, bar, pct)
	}

	fmt.Fprintf(&b, "  Build number:       %d\n", stats.Builds.GlobalBuildNumber)
	fmt.Fprintf(&b, "  Total summaries:    %d\n", stats.Builds.TotalSummaries)

	if stats.Builds.PendingSummaries > 0 {
		fmt.Fprintf(&b, "  Pending summaries:  %d\n", stats.Builds.PendingSummaries)
	}

	// Last transmutation
	if stats.LastTransmutation != nil {
		fmt.Fprintf(&b, "  Last transmutation: %s\n", formatTimestamp(*stats.LastTransmutation))
	} else {
		b.WriteString("  Last transmutation: never\n")
	}

	// Last reflection
	if stats.LastReflection.CompletedAt != nil {
		fmt.Fprintf(&b, "  Last reflection:    %s (v%d, %d memories, %d pruned)\n",
			formatTimestamp(*stats.LastReflection.CompletedAt),
			stats.LastReflection.IdentityVersion,
			stats.LastReflection.MemoriesConsidered,
			stats.LastReflection.MemoriesPruned)
	} else {
		b.WriteString("  Last reflection:    never\n")
	}

	// Category distribution
	if len(stats.Memories.ByCategory) > 0 {
		b.WriteString("\n  Categories:\n    ")

		// Sort categories by count descending
		type catCount struct {
			name  string
			count int
		}
		cats := make([]catCount, 0, len(stats.Memories.ByCategory))
		for name, count := range stats.Memories.ByCategory {
			cats = append(cats, catCount{name, count})
		}
		sort.Slice(cats, func(i, j int) bool {
			return cats[i].count > cats[j].count
		})

		parts := make([]string, len(cats))
		for i, c := range cats {
			parts[i] = fmt.Sprintf("%s: %d", c.name, c.count)
		}
		b.WriteString(strings.Join(parts, "  "))
		b.WriteByte('\n')
	}

	return b.String()
}

func progressBar(pct, width int) string {
	filled := (pct * width) / 100
	if filled > width {
		filled = width
	}
	return strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", width-filled)
}

func formatTimestamp(ts string) string {
	// Trim sub-second precision and Z suffix for readability
	if len(ts) > 19 {
		ts = ts[:19]
	}
	ts = strings.Replace(ts, "T", " ", 1)
	return ts + " UTC"
}
